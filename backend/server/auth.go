package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
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
	defaultUser    string
	defaultPass    string
	defaultPassEnc string
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
	Username string `json:"username"`
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
	secret := os.Getenv("PANEL_SECRET")
	if secret == "" {
		secret = "z-ui-default-secret-change-me"
	}
	username := os.Getenv("PANEL_USERNAME")
	if username == "" {
		username = "admin"
	}
	password := os.Getenv("PANEL_PASSWORD")
	if password == "" {
		password = "admin"
	}
	encoded, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return &AuthHandler{
		store:          store,
		secret:         []byte(secret),
		defaultUser:    username,
		defaultPass:    password,
		defaultPassEnc: string(encoded),
		loginAttempts:  map[string]loginAttempt{},
		maxFailures:    intEnv("AUTH_MAX_FAILURES", 5),
		failWindow:     time.Duration(intEnv("AUTH_FAIL_WINDOW_SEC", 600)) * time.Second,
		lockDuration:   time.Duration(intEnv("AUTH_LOCK_SEC", 900)) * time.Second,
	}
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

	username, hash, err := a.getActiveCredentials()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	loginUsername := strings.TrimSpace(req.Username)
	if loginUsername != username || bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		if retryAfter, blocked := a.recordLoginFailure(ip); blocked {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many failed attempts, please retry later", "retryAfterSec": retryAfter})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}
	a.clearLoginFailure(ip)

	token, expiresAt, err := a.generateToken(loginUsername, 24*time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token":     token,
		"expiresAt": expiresAt,
		"username":  loginUsername,
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

func (a *AuthHandler) Me(c *gin.Context) {
	username, ok := c.Get("auth.username")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"username": username})
}

func (a *AuthHandler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")
		payload, err := a.parseToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set("auth.username", payload.Username)
		c.Next()
	}
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

	settings, err := a.store.GetPanelSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	trimmedUsername := strings.TrimSpace(req.Username)
	settings.AdminUsername = trimmedUsername
	if strings.TrimSpace(settings.AdminPasswordHash) == "" {
		settings.AdminPasswordHash = a.defaultPassEnc
	}
	if _, err := a.store.UpdatePanelSettings(settings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	recordAudit(c, a.store, "auth.change_username", trimmedUsername, "")

	c.JSON(http.StatusOK, gin.H{"message": "username updated", "username": trimmedUsername})
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

	username, hash, err := a.getActiveCredentials()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = username
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.OldPassword)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "old password incorrect"})
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	settings, err := a.store.GetPanelSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	settings.AdminPasswordHash = string(newHash)
	if strings.TrimSpace(settings.AdminUsername) == "" {
		settings.AdminUsername = a.defaultUser
	}
	if _, err := a.store.UpdatePanelSettings(settings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	recordAudit(c, a.store, "auth.change_password", settings.AdminUsername, "")

	c.JSON(http.StatusOK, gin.H{"message": "password updated"})
}

func (a *AuthHandler) EnsureDefaultAdmin() error {
	username, hash, err := a.store.GetAdminCredentials()
	if err != nil {
		return err
	}
	if strings.TrimSpace(username) != "" && strings.TrimSpace(hash) != "" {
		return nil
	}
	return a.store.SetAdminCredentials(a.defaultUser, a.defaultPassEnc)
}

func (a *AuthHandler) getActiveCredentials() (username string, hash string, err error) {
	username, hash, err = a.store.GetAdminCredentials()
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(username) == "" || strings.TrimSpace(hash) == "" {
		if err := a.store.SetAdminCredentials(a.defaultUser, a.defaultPassEnc); err != nil {
			return "", "", err
		}
		return a.defaultUser, a.defaultPassEnc, nil
	}
	return username, hash, nil
}

func (a *AuthHandler) generateToken(username string, ttl time.Duration) (string, time.Time, error) {
	expiresAt := time.Now().Add(ttl)
	payload := tokenPayload{Username: username, Exp: expiresAt.Unix()}
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
