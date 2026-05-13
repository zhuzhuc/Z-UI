package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"zui/storage"
)

type AuthHandler struct {
	store          *storage.Store
	secret         []byte
	secretRaw      string
	defaultUser    string
	defaultPass    string
	defaultPassEnc string
	cookieName     string
	cookieSecure   bool
	cookieTTL      time.Duration
	limiterMu      sync.Mutex
	loginAttempts  map[string]loginAttempt
	maxFailures    int
	failWindow     time.Duration
	lockDuration   time.Duration
}

type loginAttempt struct {
	Failed     int
	LastFailed time.Time
	LockedTill time.Time
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type tokenPayload struct {
	UserID   int64  `json:"uid"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Exp      int64  `json:"exp"`
}

type changeUsernameRequest struct {
	Username string `json:"username"`
}

type changePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

func NewAuthHandler(store *storage.Store) *AuthHandler {
	secret := loadOrCreateSecret()
	username := os.Getenv("PANEL_USERNAME")
	if username == "" {
		username = "admin"
	}
	password := os.Getenv("PANEL_PASSWORD")
	if password == "" {
		password = "admin"
	}
	encoded, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("failed to hash default password: %v", err)
	}
	return &AuthHandler{
		store:          store,
		secret:         []byte(secret),
		secretRaw:      secret,
		defaultUser:    username,
		defaultPass:    password,
		defaultPassEnc: string(encoded),
		cookieName:     stringEnv("AUTH_COOKIE_NAME", "zui_session"),
		cookieSecure:   boolEnv("AUTH_COOKIE_SECURE", strings.HasPrefix(strings.ToLower(strings.TrimSpace(os.Getenv("PANEL_PUBLIC_BASE"))), "https://")),
		cookieTTL:      time.Duration(intEnv("AUTH_SESSION_HOURS", 24)) * time.Hour,
		loginAttempts:  map[string]loginAttempt{},
		maxFailures:    intEnv("AUTH_MAX_FAILURES", 5),
		failWindow:     time.Duration(intEnv("AUTH_FAIL_WINDOW_SEC", 600)) * time.Second,
		lockDuration:   time.Duration(intEnv("AUTH_LOCK_SEC", 900)) * time.Second,
	}
}

// loadOrCreateSecret loads the panel secret from file or generates a new one.
// This ensures the secret persists across restarts instead of using a hardcoded default.
func loadOrCreateSecret() string {
	if env := strings.TrimSpace(os.Getenv("PANEL_SECRET")); env != "" {
		return env
	}
	secretFile := strings.TrimSpace(os.Getenv("ZUI_SECRET_FILE"))
	if secretFile == "" {
		secretFile = "./data/.panel_secret"
	}
	if data, err := os.ReadFile(secretFile); err == nil {
		s := strings.TrimSpace(string(data))
		if len(s) >= 32 {
			return s
		}
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		log.Fatalf("failed to generate random secret: %v", err)
	}
	newSecret := hex.EncodeToString(buf)
	if err := os.MkdirAll(filepath.Dir(secretFile), 0o700); err == nil {
		if err := os.WriteFile(secretFile, []byte(newSecret), 0o600); err != nil {
			log.Printf("WARNING: could not persist secret to %s: %v (secret will change on restart)", secretFile, err)
		}
	}
	log.Printf("auto-generated panel secret (stored at %s)", secretFile)
	return newSecret
}

func (a *AuthHandler) Login(c *gin.Context) {
	ip := c.ClientIP()
	if retryAfter, blocked := a.loginBlocked(ip); blocked {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many failed attempts, please retry later", "retryAfterSec": retryAfter})
		return
	}

	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json body"})
		return
	}

 	loginUsername := strings.TrimSpace(req.Username)
	if loginUsername == "" || len(loginUsername) > 64 {
		if retryAfter, blocked := a.recordLoginFailure(ip); blocked {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many failed attempts, please retry later", "retryAfterSec": retryAfter})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}
	user, err := a.store.GetUserByUsername(loginUsername)
	if err != nil || !strings.EqualFold(strings.TrimSpace(user.Status), "active") {
		if retryAfter, blocked := a.recordLoginFailure(ip); blocked {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many failed attempts, please retry later", "retryAfterSec": retryAfter})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		if retryAfter, blocked := a.recordLoginFailure(ip); blocked {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many failed attempts, please retry later", "retryAfterSec": retryAfter})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}
	a.clearLoginFailure(ip)

	token, expiresAt, err := a.generateToken(user, a.cookieTTL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	a.setSessionCookie(c, token, expiresAt)
	c.JSON(http.StatusOK, gin.H{
		"token":     token,
		"expiresAt": expiresAt,
		"username":  user.Username,
		"role":      user.Role,
	})
}

func (a *AuthHandler) loginBlocked(ip string) (int, bool) {
	now := time.Now()
	a.limiterMu.Lock()
	defer a.limiterMu.Unlock()

	rec, ok := a.loginAttempts[ip]
	if !ok {
		return 0, false
	}
	if !rec.LockedTill.IsZero() && rec.LockedTill.After(now) {
		return int(rec.LockedTill.Sub(now).Seconds()) + 1, true
	}
	if rec.LastFailed.Add(a.failWindow).Before(now) {
		delete(a.loginAttempts, ip)
	}
	return 0, false
}

func (a *AuthHandler) recordLoginFailure(ip string) (int, bool) {
	now := time.Now()
	a.limiterMu.Lock()
	defer a.limiterMu.Unlock()

	rec := a.loginAttempts[ip]
	if rec.LastFailed.Add(a.failWindow).Before(now) {
		rec = loginAttempt{}
	}
	rec.Failed++
	rec.LastFailed = now
	if rec.Failed >= a.maxFailures {
		rec.LockedTill = now.Add(a.lockDuration)
		a.loginAttempts[ip] = rec
		return int(a.lockDuration.Seconds()), true
	}
	a.loginAttempts[ip] = rec
	return 0, false
}

func (a *AuthHandler) clearLoginFailure(ip string) {
	a.limiterMu.Lock()
	defer a.limiterMu.Unlock()
	delete(a.loginAttempts, ip)
}

func intEnv(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func boolEnv(key string, fallback bool) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if raw == "" {
		return fallback
	}
	switch raw {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func stringEnv(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func (a *AuthHandler) Me(c *gin.Context) {
	user, err := a.currentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": user.ID, "username": user.Username, "role": user.Role})
}

func (a *AuthHandler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := a.extractToken(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing session"})
			return
		}
		payload, err := a.parseToken(token)
		if err != nil {
			a.clearSessionCookie(c)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set("auth.username", payload.Username)
		c.Set("auth.role", payload.Role)
		c.Set("auth.userID", payload.UserID)
		c.Next()
	}
}

// RequireRole returns middleware that restricts access to users with one of the specified roles.
// Role hierarchy: owner > admin > operator > viewer
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[strings.ToLower(strings.TrimSpace(r))] = struct{}{}
	}
	return func(c *gin.Context) {
		role := strings.ToLower(strings.TrimSpace(c.GetString("auth.role")))
		if _, ok := allowed[role]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			return
		}
		c.Next()
	}
}

func (a *AuthHandler) Logout(c *gin.Context) {
	a.clearSessionCookie(c)
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

func (a *AuthHandler) ChangeUsername(c *gin.Context) {
	var req changeUsernameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json body"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username cannot be empty"})
		return
	}
	if len(req.Username) > 64 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username must be 64 characters or less"})
		return
	}
	user, err := a.currentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	updated, err := a.store.UpdateUserUsername(user.ID, req.Username)
	if err != nil {
		if isUniqueConstraintError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "username already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if strings.EqualFold(updated.Role, "owner") {
		a.store.SyncLegacyAdminUser(updated)
	}
	if err := a.refreshSession(c, updated); err != nil {
		// best effort; ignore error to not block username change
	}
	recordAudit(c, a.store, "auth.change_username", updated.Username, "")
	c.JSON(http.StatusOK, gin.H{"message": "username updated", "username": updated.Username})
}

func (a *AuthHandler) ChangePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json body"})
		return
	}
	if strings.TrimSpace(req.NewPassword) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "newPassword cannot be empty"})
		return
	}
	if err := validatePasswordStrength(req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := a.currentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "old password incorrect"})
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := a.store.UpdateUserPassword(user.ID, string(newHash)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	user.PasswordHash = string(newHash)
	if strings.EqualFold(user.Role, "owner") {
		a.store.SyncLegacyAdminUser(user)
	}
	_ = a.refreshSession(c, user)
	recordAudit(c, a.store, "auth.change_password", user.Username, "")
	c.JSON(http.StatusOK, gin.H{"message": "password updated"})
}

func (a *AuthHandler) EnsureDefaultAdmin() error {
	owners, err := a.store.CountUsersByRole("owner")
	if err != nil {
		return err
	}
	if owners > 0 {
		return nil
	}
	// If using default admin/admin credentials, generate a random password
	if a.defaultUser == "admin" && a.defaultPass == "admin" {
		randomPass := generateRandomPassword(16)
		encoded, err := bcrypt.GenerateFromPassword([]byte(randomPass), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash generated password: %w", err)
		}
		a.defaultPassEnc = string(encoded)
		if _, err := a.store.EnsureOwnerUser(a.defaultUser, a.defaultPassEnc); err != nil {
			return err
		}
		fmt.Println("============================================================")
		fmt.Printf("  Auto-generated admin credentials (save these!):\n")
		fmt.Printf("  Username: %s\n", a.defaultUser)
		fmt.Printf("  Password: %s\n", randomPass)
		fmt.Println("============================================================")
		return nil
	}
	_, err = a.store.EnsureOwnerUser(a.defaultUser, a.defaultPassEnc)
	return err
}

func generateRandomPassword(length int) string {
	const charset = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		log.Fatalf("failed to generate random password: %v", err)
	}
	for i, b := range buf {
		buf[i] = charset[int(b)%len(charset)]
	}
	return string(buf)
}

func (a *AuthHandler) ValidateProductionReadiness() error {
	strict := boolEnv("ZUI_STRICT_PRODUCTION", strings.EqualFold(strings.TrimSpace(os.Getenv("GIN_MODE")), "release"))
	if !strict {
		return nil
	}
	if len(strings.TrimSpace(a.secretRaw)) < 32 || a.secretRaw == "z-ui-default-secret-change-me" {
		return errors.New("PANEL_SECRET is unsafe in production; set a random secret with at least 32 characters")
	}
	user, err := a.store.PrimaryUser()
	if err != nil {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(user.Username), "admin") && bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("admin")) == nil {
		return errors.New("default admin/admin credential is not allowed in production")
	}
	return nil
}

func (a *AuthHandler) currentUser(c *gin.Context) (storage.User, error) {
	rawID, ok := c.Get("auth.userID")
	if !ok {
		return storage.User{}, errors.New("missing auth context")
	}
	id, ok := toInt64(rawID)
	if !ok {
		return storage.User{}, errors.New("invalid auth user id")
	}
	return a.store.GetUserByID(id)
}

func toInt64(v any) (int64, bool) {
	switch val := v.(type) {
	case int64:
		return val, true
	case int:
		return int64(val), true
	case float64:
		return int64(val), true
	default:
		return 0, false
	}
}

func (a *AuthHandler) extractToken(c *gin.Context) string {
	authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	}
	if cookie, err := c.Cookie(a.cookieName); err == nil {
		return strings.TrimSpace(cookie)
	}
	return ""
}

func (a *AuthHandler) setSessionCookie(c *gin.Context, token string, expiresAt time.Time) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     a.cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
	})
}

func (a *AuthHandler) clearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     a.cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   a.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}

func validatePasswordStrength(password string) error {
	if len(password) < 10 {
		return fmt.Errorf("newPassword must be at least 10 characters")
	}
	var hasUpper, hasLower, hasDigit bool
	for _, ch := range password {
		switch {
		case ch >= 'A' && ch <= 'Z':
			hasUpper = true
		case ch >= 'a' && ch <= 'z':
			hasLower = true
		case ch >= '0' && ch <= '9':
			hasDigit = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit {
		return fmt.Errorf("newPassword must include uppercase, lowercase, and number")
	}
	return nil
}

func (a *AuthHandler) refreshSession(c *gin.Context, user storage.User) error {
	token, expiresAt, err := a.generateToken(user, a.cookieTTL)
	if err != nil {
		return err
	}
	a.setSessionCookie(c, token, expiresAt)
	return nil
}

func (a *AuthHandler) generateToken(user storage.User, ttl time.Duration) (string, time.Time, error) {
	expiresAt := time.Now().Add(ttl)
	payload := tokenPayload{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		Exp:      expiresAt.Unix(),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", time.Time{}, err
	}
	payloadText := base64.RawURLEncoding.EncodeToString(raw)
	sig := signHMAC(a.secret, payloadText)
	signature := base64.RawURLEncoding.EncodeToString(sig)
	return payloadText + "." + signature, expiresAt, nil
}

func (a *AuthHandler) parseToken(token string) (tokenPayload, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return tokenPayload{}, errors.New("invalid token format")
	}
	payloadText := parts[0]
	signatureText := parts[1]

	gotSig, err := base64.RawURLEncoding.DecodeString(signatureText)
	if err != nil {
		return tokenPayload{}, err
	}
	wantSig := signHMAC(a.secret, payloadText)
	if !hmac.Equal(gotSig, wantSig) {
		return tokenPayload{}, errors.New("signature mismatch")
	}

	payloadRaw, err := base64.RawURLEncoding.DecodeString(payloadText)
	if err != nil {
		return tokenPayload{}, err
	}
	var payload tokenPayload
	if err := json.Unmarshal(payloadRaw, &payload); err != nil {
		return tokenPayload{}, err
	}
	if time.Now().Unix() > payload.Exp {
		return tokenPayload{}, errors.New("token expired")
	}
	return payload, nil
}

func signHMAC(secret []byte, input string) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(input))
	return mac.Sum(nil)
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") || strings.Contains(msg, "unique failed")
}
