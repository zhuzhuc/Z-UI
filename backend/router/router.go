package router

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"zui/server"
	"zui/storage"
)

func RegisterRouter() *gin.Engine {
	engine := gin.Default()
	configureTrustedProxies(engine)
	engine.Use(corsMiddleware())
	registerStaticRoutes(engine)

	store, err := storage.OpenDefaultStore()
	if err != nil {
		log.Fatalf("open sqlite store failed: %v", err)
	}
	authHandler := server.NewAuthHandler(store)
	if err := authHandler.EnsureDefaultAdmin(); err != nil {
		log.Fatalf("init default admin failed: %v", err)
	}
	inboundHandler := server.NewInboundHandler(store)
	xrayManager := server.NewXrayManager(store)
	dashboardHandler := server.NewDashboardHandler(store, xrayManager, time.Now())
	settingsHandler := server.NewSettingsHandler(store)
	subscriptionHandler := server.NewSubscriptionHandler(store)
	logHandler := server.NewLogHandler(xrayManager)
	toolsHandler := server.NewToolsHandler()
	auditHandler := server.NewAuditHandler(store)

	api := engine.Group("/api/v1")
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})
		api.POST("/auth/login", authHandler.Login)
		api.GET("/sub/:token", subscriptionHandler.PublicSubscription)

		protected := api.Group("")
		protected.Use(authHandler.AuthMiddleware())
		{
			protected.GET("/auth/me", authHandler.Me)
			protected.POST("/auth/change-username", authHandler.ChangeUsername)
			protected.POST("/auth/change-password", authHandler.ChangePassword)
			protected.GET("/dashboard/summary", dashboardHandler.Summary)
			protected.GET("/panel/settings", settingsHandler.Get)
			protected.PUT("/panel/settings", settingsHandler.Update)
			protected.GET("/subscription/info", subscriptionHandler.Info)
			protected.POST("/subscription/rotate", subscriptionHandler.Rotate)
			protected.GET("/subscription/preview", subscriptionHandler.Preview)
			protected.GET("/subscription/nodes", subscriptionHandler.Nodes)
			protected.GET("/logs/xray", logHandler.Xray)
			protected.GET("/logs/system", logHandler.System)
			protected.GET("/tools/bbr", toolsHandler.BBRStatus)
			protected.POST("/tools/bbr/enable", toolsHandler.BBREnable)
			protected.POST("/tools/speedtest", toolsHandler.Speedtest)
			protected.GET("/audit/logs", auditHandler.List)

			inbounds := protected.Group("/inbounds")
			{
				inbounds.GET("", inboundHandler.List)
				inbounds.GET("/:id", inboundHandler.Get)
				inbounds.POST("", inboundHandler.Create)
				inbounds.PUT("/:id", inboundHandler.Update)
				inbounds.DELETE("/:id", inboundHandler.Delete)
			}

			xray := protected.Group("/xray")
			{
				xray.GET("/status", xrayManager.GetStatus)
				xray.POST("/start", xrayManager.Start)
				xray.POST("/stop", xrayManager.Stop)
				xray.POST("/restart", xrayManager.Restart)
				xray.POST("/apply", xrayManager.ApplyConfig)
				xray.GET("/config", xrayManager.GetConfig)
				xray.PUT("/config", xrayManager.UpdateConfig)
				xray.GET("/stats/overview", xrayManager.StatsOverview)
				xray.POST("/stats/sync", xrayManager.SyncUsage)
				xray.GET("/limits/preview", xrayManager.LimitPreview)
			}
		}
	}

	return engine
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
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		} else if origin != "" {
			if _, ok := originSet[origin]; ok {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				c.Writer.Header().Set("Vary", "Origin")
			}
		}

		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
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
