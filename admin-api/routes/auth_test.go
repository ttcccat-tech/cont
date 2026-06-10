package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func makeTestToken(secret string, claims jwt.MapClaims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte(secret))
	return tokenStr
}

func TestAuthRequired_MissingHeader(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	handler := AuthRequired("test-secret")
	handler(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestAuthRequired_KongAdminToken(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("Kong-Admin-Token", "Bearer invalid-token")

	handler := AuthRequired("test-secret")
	handler(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestAuthRequired_InvalidToken(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("Authorization", "Bearer invalid-token")

	handler := AuthRequired("test-secret")
	handler(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestAuthRequired_ExpiredToken(t *testing.T) {
	secret := "test-secret"
	claims := jwt.MapClaims{
		"sub":      "user-001",
		"username": "alice",
		"role":     "admin",
		"exp":      time.Now().Add(-1 * time.Hour).Unix(),
	}
	tokenStr := makeTestToken(secret, claims)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("Authorization", "Bearer "+tokenStr)

	handler := AuthRequired(secret)
	handler(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestAuthRequired_ValidToken(t *testing.T) {
	secret := "test-secret"
	claims := jwt.MapClaims{
		"sub":      "user-001",
		"username": "alice",
		"role":     "admin",
		"exp":      time.Now().Add(1 * time.Hour).Unix(),
	}
	tokenStr := makeTestToken(secret, claims)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("Authorization", "Bearer "+tokenStr)

	handler := AuthRequired(secret)
	handler(c)

	if w.Code != 0 && w.Code != http.StatusOK {
		// After c.Next(), code stays at 0 (no response sent yet)
		// Check that user_id was set
	}
	if c.GetString("user_id") != "user-001" {
		t.Errorf("expected user_id user-001, got %s", c.GetString("user_id"))
	}
	if c.GetString("username") != "alice" {
		t.Errorf("expected username alice, got %s", c.GetString("username"))
	}
	if c.GetString("role") != "admin" {
		t.Errorf("expected role admin, got %s", c.GetString("role"))
	}
}

func TestAuthRequired_WrongSecret(t *testing.T) {
	claims := jwt.MapClaims{
		"sub": "user-001",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	}
	tokenStr := makeTestToken("correct-secret", claims)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("Authorization", "Bearer "+tokenStr)

	handler := AuthRequired("wrong-secret")
	handler(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestAuthRequired_BearerPrefixOptional(t *testing.T) {
	claims := jwt.MapClaims{
		"sub":      "user-001",
		"username": "alice",
		"role":     "admin",
		"exp":      time.Now().Add(1 * time.Hour).Unix(),
	}
	tokenStr := makeTestToken("test-secret", claims)

	// Kong admin API accepts tokens without Bearer prefix
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("Authorization", tokenStr)

	handler := AuthRequired("test-secret")
	handler(c)

	// Should succeed — Bearer prefix is optional in Kong-compatible API
	if c.GetString("user_id") != "user-001" {
		t.Errorf("expected user_id user-001, got %s", c.GetString("user_id"))
	}
}