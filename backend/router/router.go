package router

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"zui/server"
	"zui/storage"
)

func RegisterRouter() (*gin.Engine, *storage.Store) {
	engine := gin.Default()
	configureTrustedProxies(engine)
	engine.Use(corsMiddleware())
	engine.Use(securityHeaders())
	engine.Use(rateLimiter(120, time.Minute)) // 120 requests per minute per IP
	registerStaticRoutes(engine)

	store, err := storage.OpenDefaultStore()
	if err != nil {
		log.Fatalf("open sqlite store failed: %v", err)
	}
	authHandler := server.NewAuthHandler(store)
	if err := authHandler.EnsureDefaultAdmin(); err != nil {
		log.Fatalf("init default admin failed: %v", err)
	}
	if err := authHandler.ValidateProductionReadiness(); err != nil {
		log.Fatalf("production security check failed: %v", err)
	}
	inboundHandler := server.NewInboundHandler(store)
	xrayManager := server.NewXrayManager(store)
	dashboardHandler := server.NewDashboardHandler(store, xrayManager, time.Now())
	settingsHandler := server.NewSettingsHandler(store)
	subscriptionHandler := server.NewSubscriptionHandler(store)
	logHandler := server.NewLogHandler(xrayManager)
	toolsHandler := server.NewToolsHandler()
	auditHandler := server.NewAuditHandler(store)
	usersHandler := server.NewUsersHandler(store)
	backupHandler := server.NewBackupHandler(store)

	// Role definitions for RBAC
	viewer := server.RequireRole("viewer", "operator", "admin", "owner")
	operator := server.RequireRole("operator", "admin", "owner")
	admin := server.RequireRole("admin", "owner")

	api := engine.Group("/api/v1")
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok", "version": server.Version})
		})
		api.GET("/public/settings", settingsHandler.Public)
		api.POST("/auth/login", maxBodyBytes(4096), authHandler.Login)
		api.GET("/sub/:token", subscriptionHandler.PublicSubscription)
		api.GET("/sub/:token/*email", subscriptionHandler.PublicSubscription)

		protected := api.Group("")
		protected.Use(authHandler.AuthMiddleware())
		protected.Use(csrfMiddleware())
		{
			// Self-service routes (any authenticated user)
			protected.GET("/auth/me", authHandler.Me)
			protected.POST("/auth/logout", authHandler.Logout)
			protected.POST("/auth/change-username", maxBodyBytes(4096), authHandler.ChangeUsername)
			protected.POST("/auth/change-password", maxBodyBytes(4096), authHandler.ChangePassword)

			// Read-only routes (all roles including viewer)
			protected.GET("/dashboard/summary", viewer, dashboardHandler.Summary)
			protected.GET("/panel/settings", viewer, settingsHandler.Get)
			protected.GET("/subscription/info", viewer, subscriptionHandler.Info)
			protected.GET("/subscription/preview", viewer, subscriptionHandler.Preview)
			protected.GET("/subscription/nodes", viewer, subscriptionHandler.Nodes)
			protected.GET("/logs/xray", viewer, logHandler.Xray)
			protected.GET("/logs/system", viewer, logHandler.System)
			protected.GET("/tools/bbr", viewer, toolsHandler.BBRStatus)
			protected.GET("/audit/logs", viewer, auditHandler.List)

			// Operator routes (operator, admin, owner)
			protected.POST("/subscription/rotate", operator, subscriptionHandler.Rotate)
			protected.POST("/tools/bbr/enable", operator, maxBodyBytes(4096), toolsHandler.BBREnable)
			protected.POST("/tools/speedtest", operator, maxBodyBytes(4096), toolsHandler.Speedtest)
			protected.POST("/xray/start", operator, xrayManager.Start)
			protected.POST("/xray/stop", operator, xrayManager.Stop)
			protected.POST("/xray/restart", operator, xrayManager.Restart)
			protected.POST("/xray/apply", operator, xrayManager.ApplyConfig)
			protected.POST("/xray/stats/sync", operator, xrayManager.SyncUsage)

			// Inbound routes (read for viewer+, write for operator+)
			inbounds := protected.Group("/inbounds")
			{
				inbounds.GET("", viewer, inboundHandler.List)
				inbounds.GET("/:id", viewer, inboundHandler.Get)
				inbounds.POST("", operator, maxBodyBytes(256*1024), inboundHandler.Create)
				inbounds.PUT("/:id", operator, maxBodyBytes(256*1024), inboundHandler.Update)
				inbounds.DELETE("/:id", admin, inboundHandler.Delete)
				inbounds.POST("/:id/clone", operator, inboundHandler.Clone)
				inbounds.POST("/:id/reset-traffic", operator, inboundHandler.ResetTraffic)

				// Client management within inbounds
				inbounds.GET("/:id/clients", viewer, inboundHandler.ListClients)
				inbounds.POST("/:id/clients", operator, maxBodyBytes(4096), inboundHandler.AddClient)
				inbounds.PUT("/:id/clients/:email", operator, maxBodyBytes(4096), inboundHandler.UpdateClient)
				inbounds.DELETE("/:id/clients/:email", operator, inboundHandler.DeleteClient)
			}

			// Xray read routes (viewer+)
			protected.GET("/xray/status", viewer, xrayManager.GetStatus)
			protected.GET("/xray/config", viewer, xrayManager.GetConfig)
			protected.GET("/xray/stats/overview", viewer, xrayManager.StatsOverview)
			protected.GET("/xray/limits/preview", viewer, xrayManager.LimitPreview)
			protected.GET("/xray/online", viewer, xrayManager.OnlineUsers)

			// Xray write routes (operator+)
			protected.PUT("/xray/config", operator, maxBodyBytes(64*1024), xrayManager.UpdateConfig)

			// Admin routes (admin, owner)
			protected.PUT("/panel/settings", admin, maxBodyBytes(16*1024), settingsHandler.Update)
			protected.GET("/users", admin, usersHandler.List)
			protected.POST("/users", admin, maxBodyBytes(4096), usersHandler.Create)
			protected.PUT("/users/:id", admin, maxBodyBytes(4096), usersHandler.Update)
			protected.POST("/users/:id/password", admin, maxBodyBytes(4096), usersHandler.ResetPassword)
			protected.DELETE("/users/:id", admin, usersHandler.Delete)

			// Backup/restore (admin+)
			protected.GET("/backup/download", admin, backupHandler.Download)
			protected.POST("/backup/restore", admin, maxBodyBytes(100*1024*1024), backupHandler.Restore)
		}
	}

	return engine, store
}

func registerStaticRoutes(engine *gin.Engine) {
	frontDir := os.Getenv("ZUI_FRONT_DIR")
	if frontDir == "" {
		frontDir = "../front/dist"
	}
	absFrontDir, err := filepath.Abs(frontDir)
	if err != nil {
		return
	}
	info, err := os.Stat(absFrontDir)
	if err != nil || !info.IsDir() {
		fallbackDir, fallbackErr := filepath.Abs("../front")
		if fallbackErr != nil {
			return
		}
		fallbackInfo, statErr := os.Stat(fallbackDir)
		if statErr != nil || !fallbackInfo.IsDir() {
			return
		}
		absFrontDir = fallbackDir
	}

	serveFile := func(c *gin.Context, p string) {
		cleaned := filepath.Clean(p)
		if cleaned == "." || cleaned == "/" {
			cleaned = "login.html"
		}
		cleaned = strings.TrimPrefix(cleaned, "/")
		candidate := filepath.Join(absFrontDir, cleaned)
		rel, err := filepath.Rel(absFrontDir, candidate)
		if err != nil || strings.HasPrefix(rel, "..") {
			c.Status(http.StatusNotFound)
			return
		}
		if stat, err := os.Stat(candidate); err == nil && !stat.IsDir() {
			c.File(candidate)
			return
		}
		c.File(filepath.Join(absFrontDir, "login.html"))
	}

	engine.GET("/", func(c *gin.Context) {
		serveFile(c, "login.html")
	})
	engine.GET("/login.html", func(c *gin.Context) {
		serveFile(c, "login.html")
	})
	engine.GET("/main.html", func(c *gin.Context) {
		serveFile(c, "main.html")
	})

	engine.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		serveFile(c, c.Request.URL.Path)
	})
}

func corsMiddleware() gin.HandlerFunc {
	allowedOrigins := parseCSVEnv("CORS_ALLOW_ORIGINS", []string{
		"http://127.0.0.1:5500",
		"http://localhost:5500",
		"http://127.0.0.1:8081",
		"http://localhost:8081",
	})
	allowAny := len(allowedOrigins) == 1 && allowedOrigins[0] == "*"
	originSet := map[string]struct{}{}
	for _, one := range allowedOrigins {
		originSet[one] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if allowAny {
			if origin != "" {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				c.Writer.Header().Set("Vary", "Origin")
			} else {
				c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
			}
		} else if origin != "" {
			if _, ok := originSet[origin]; ok {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				c.Writer.Header().Set("Vary", "Origin")
			}
		}

		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// securityHeaders adds standard HTTP security headers to all responses.
func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
		c.Writer.Header().Set("X-Frame-Options", "DENY")
		c.Writer.Header().Set("X-XSS-Protection", "0")
		c.Writer.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Writer.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Writer.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'")
		// Only set HSTS when request is over HTTPS (check forwarded proto)
		if strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") ||
			c.Request.TLS != nil {
			c.Writer.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}

// csrfMiddleware validates Origin/Referer on state-changing requests to prevent CSRF attacks.
func csrfMiddleware() gin.HandlerFunc {
	allowedOrigins := parseCSVEnv("CORS_ALLOW_ORIGINS", []string{
		"http://127.0.0.1:5500",
		"http://localhost:5500",
		"http://127.0.0.1:8081",
		"http://localhost:8081",
	})
	originSet := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		originSet[o] = struct{}{}
	}
	allowAny := len(allowedOrigins) == 1 && allowedOrigins[0] == "*"

	return func(c *gin.Context) {
		method := c.Request.Method
		if method != http.MethodPost && method != http.MethodPut &&
			method != http.MethodDelete && method != http.MethodPatch {
			c.Next()
			return
		}
		origin := c.GetHeader("Origin")
		if origin != "" {
			if allowAny {
				c.Next()
				return
			}
			if _, ok := originSet[origin]; ok {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "origin not allowed"})
			return
		}
		referer := c.GetHeader("Referer")
		if referer == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "missing origin/referer"})
			return
		}
		if allowAny {
			c.Next()
			return
		}
		for _, allowed := range allowedOrigins {
			if strings.HasPrefix(referer, allowed) {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "referer not allowed"})
	}
}

// maxBodyBytes returns middleware that limits the request body size.
func maxBodyBytes(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}

// rateLimiter returns a per-IP sliding window rate limiter middleware.
func rateLimiter(maxRequests int, window time.Duration) gin.HandlerFunc {
	type entry struct {
		count    int
		resetAt  time.Time
	}
	var (
		mu    sync.Mutex
		items = make(map[string]*entry)
	)
	go func() {
		for {
			time.Sleep(window)
			mu.Lock()
			now := time.Now()
			for ip, e := range items {
				if now.After(e.resetAt) {
					delete(items, ip)
				}
			}
			mu.Unlock()
		}
	}()
	return func(c *gin.Context) {
		ip := c.ClientIP()
		mu.Lock()
		e, ok := items[ip]
		now := time.Now()
		if !ok || now.After(e.resetAt) {
			e = &entry{count: 0, resetAt: now.Add(window)}
			items[ip] = e
		}
		e.count++
		count := e.count
		mu.Unlock()
		if count > maxRequests {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded, please try again later",
			})
			return
		}
		c.Next()
	}
}

func configureTrustedProxies(engine *gin.Engine) {
	proxies := parseCSVEnv("TRUSTED_PROXIES", []string{"127.0.0.1", "::1"})
	if len(proxies) == 1 && proxies[0] == "*" {
		if err := engine.SetTrustedProxies(nil); err != nil {
			log.Printf("set trusted proxies failed: %v", err)
		}
		return
	}
	if err := engine.SetTrustedProxies(proxies); err != nil {
		log.Printf("set trusted proxies failed: %v", err)
	}
}

func parseCSVEnv(key string, defaults []string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaults
	}
	parts := strings.Split(raw, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		v := strings.TrimSpace(part)
		if v != "" {
			items = append(items, v)
		}
	}
	if len(items) == 0 {
		return defaults
	}
	return items
}
