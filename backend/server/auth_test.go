package server

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"zui/storage"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestGenerateAndParseToken(t *testing.T) {
	handler := &AuthHandler{
		secret: []byte("test-secret-key-at-least-32-bytes!"),
	}
	user := storage.User{
		ID:       1,
		Username: "testuser",
		Role:     "admin",
	}
	ttl := time.Hour

	token, expiresAt, err := handler.generateToken(user, ttl)
	if err != nil {
		t.Fatalf("generateToken failed: %v", err)
	}
	if token == "" {
		t.Fatal("token is empty")
	}
	if expiresAt.IsZero() {
		t.Fatal("expiresAt is zero")
	}

	payload, err := handler.parseToken(token)
	if err != nil {
		t.Fatalf("parseToken failed: %v", err)
	}
	if payload.UserID != 1 {
		t.Errorf("UserID = %d, want 1", payload.UserID)
	}
	if payload.Username != "testuser" {
		t.Errorf("Username = %q, want %q", payload.Username, "testuser")
	}
	if payload.Role != "admin" {
		t.Errorf("Role = %q, want %q", payload.Role, "admin")
	}
	if payload.Exp != expiresAt.Unix() {
		t.Errorf("Exp = %d, want %d", payload.Exp, expiresAt.Unix())
	}
}

func TestParseToken_InvalidFormat(t *testing.T) {
	handler := &AuthHandler{
		secret: []byte("test-secret-key-at-least-32-bytes!"),
	}

	_, err := handler.parseToken("no-dot-separator")
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}

func TestParseToken_BadSignature(t *testing.T) {
	handler := &AuthHandler{
		secret: []byte("test-secret-key-at-least-32-bytes!"),
	}
	user := storage.User{ID: 1, Username: "test", Role: "admin"}
	token, _, _ := handler.generateToken(user, time.Hour)

	// Tamper with the token
	tampered := token[:len(token)-2] + "XX"

	_, err := handler.parseToken(tampered)
	if err == nil {
		t.Fatal("expected error for tampered token")
	}
}

func TestParseToken_WrongSecret(t *testing.T) {
	handler1 := &AuthHandler{secret: []byte("test-secret-key-at-least-32-bytes!")}
	handler2 := &AuthHandler{secret: []byte("different-secret-key-at-least-32!")}
	user := storage.User{ID: 1, Username: "test", Role: "admin"}
	token, _, _ := handler1.generateToken(user, time.Hour)

	_, err := handler2.parseToken(token)
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestParseToken_Expired(t *testing.T) {
	handler := &AuthHandler{secret: []byte("test-secret-key-at-least-32-bytes!")}
	user := storage.User{ID: 1, Username: "test", Role: "admin"}
	// Use negative TTL to create an already-expired token
	token, _, _ := handler.generateToken(user, -time.Hour)

	_, err := handler.parseToken(token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestValidatePasswordStrength(t *testing.T) {
	tests := []struct {
		name    string
		pass    string
		wantErr bool
	}{
		{"valid", "Str0ngPass!1", false},
		{"too short", "Sh0rt!", true},
		{"no uppercase", "lowercase123!", true},
		{"no lowercase", "UPPERCASE123!", true},
		{"no digit", "NoDigitsHere!", true},
		{"empty", "", true},
		{"minimum valid", "Abcdefghij1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePasswordStrength(tt.pass)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePasswordStrength(%q) error = %v, wantErr %v", tt.pass, err, tt.wantErr)
			}
		})
	}
}

func TestGenerateRandomPassword(t *testing.T) {
	p1 := generateRandomPassword(16)
	p2 := generateRandomPassword(16)

	if len(p1) != 16 {
		t.Errorf("password length = %d, want 16", len(p1))
	}
	if p1 == p2 {
		t.Error("two random passwords should not be equal")
	}
}

func TestLoginRateLimiter(t *testing.T) {
	handler := &AuthHandler{
		loginAttempts: map[string]loginAttempt{},
		maxFailures:   3,
		failWindow:    time.Minute,
		lockDuration:  time.Minute * 5,
	}
	ip := "192.168.1.1"

	// Should not be blocked initially
	if _, blocked := handler.loginBlocked(ip); blocked {
		t.Fatal("should not be blocked initially")
	}

	// Record failures up to limit
	for i := 0; i < 3; i++ {
		retryAfter, blocked := handler.recordLoginFailure(ip)
		if i < 2 && blocked {
			t.Fatalf("should not be blocked after %d failures", i+1)
		}
		if i == 2 {
			if !blocked {
				t.Fatal("should be blocked after 3 failures")
			}
			if retryAfter <= 0 {
				t.Fatal("retryAfter should be positive")
			}
		}
	}

	// Should be blocked now
	if _, blocked := handler.loginBlocked(ip); !blocked {
		t.Fatal("should be blocked after max failures")
	}

	// Clear and verify
	handler.clearLoginFailure(ip)
	if _, blocked := handler.loginBlocked(ip); blocked {
		t.Fatal("should not be blocked after clear")
	}
}

func TestRequireRole(t *testing.T) {
	tests := []struct {
		name       string
		allowed    []string
		userRole   string
		wantStatus int
	}{
		{"admin allows admin", []string{"admin", "owner"}, "admin", 200},
		{"admin rejects viewer", []string{"admin", "owner"}, "viewer", 403},
		{"viewer allows viewer", []string{"viewer", "operator", "admin", "owner"}, "viewer", 200},
		{"operator allows admin", []string{"operator", "admin", "owner"}, "admin", 200},
		{"operator rejects viewer", []string{"operator", "admin", "owner"}, "viewer", 403},
		{"empty role rejected", []string{"admin"}, "", 403},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			_, r := gin.CreateTestContext(w)

			// Inject role via a fake auth middleware before RequireRole
			r.GET("/test",
				func(c *gin.Context) {
					c.Set("auth.role", tt.userRole)
					c.Next()
				},
				RequireRole(tt.allowed...),
				func(c *gin.Context) {
					c.Status(200)
				},
			)

			req := httptest.NewRequest("GET", "/test", nil)
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestIntEnv(t *testing.T) {
	t.Setenv("TEST_INT_ENV", "42")
	if got := intEnv("TEST_INT_ENV", 10); got != 42 {
		t.Errorf("intEnv = %d, want 42", got)
	}
	if got := intEnv("TEST_INT_MISSING", 10); got != 10 {
		t.Errorf("intEnv default = %d, want 10", got)
	}
	t.Setenv("TEST_INT_BAD", "abc")
	if got := intEnv("TEST_INT_BAD", 10); got != 10 {
		t.Errorf("intEnv bad = %d, want 10", got)
	}
}

func TestBoolEnv(t *testing.T) {
	tests := []struct {
		val      string
		fallback bool
		want     bool
	}{
		{"1", false, true},
		{"true", false, true},
		{"yes", false, true},
		{"0", true, false},
		{"false", true, false},
		{"no", true, false},
		{"", true, true},
		{"", false, false},
	}
	for _, tt := range tests {
		t.Setenv("TEST_BOOL_ENV", tt.val)
		if got := boolEnv("TEST_BOOL_ENV", tt.fallback); got != tt.want {
			t.Errorf("boolEnv(%q, %v) = %v, want %v", tt.val, tt.fallback, got, tt.want)
		}
	}
}

func TestToSafeInt64(t *testing.T) {
	tests := []struct {
		input any
		want  int64
		ok    bool
	}{
		{int64(42), 42, true},
		{int(7), 7, true},
		{float64(3.0), 3, true},
		{"bad", 0, false},
		{nil, 0, false},
	}
	for _, tt := range tests {
		got, ok := toInt64(tt.input)
		if ok != tt.ok || got != tt.want {
			t.Errorf("toInt64(%v) = (%d, %v), want (%d, %v)", tt.input, got, ok, tt.want, tt.ok)
		}
	}
}
