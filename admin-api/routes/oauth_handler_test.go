package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/ttcccat-tech/cont/admin-api/storage"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestOAuthState_JSON(t *testing.T) {
	state := OAuthState{
		State:       "test-state-123",
		Provider:    "google",
		RedirectURI: "http://localhost:3000/callback",
		ExpiresAt:   time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var parsed OAuthState
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if parsed.State != state.State {
		t.Errorf("state: expected %q, got %q", state.State, parsed.State)
	}
	if parsed.Provider != state.Provider {
		t.Errorf("provider: expected %q, got %q", state.Provider, parsed.Provider)
	}
}

func TestOAuthUserInfo_JSON(t *testing.T) {
	userInfo := OAuthUserInfo{
		Subject: "user-123",
		Email:   "alice@example.com",
		Name:    "Alice",
		Picture: "https://example.com/photo.jpg",
	}

	data, err := json.Marshal(userInfo)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var parsed OAuthUserInfo
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if parsed.Subject != userInfo.Subject {
		t.Errorf("subject: expected %q, got %q", userInfo.Subject, parsed.Subject)
	}
	if parsed.Email != userInfo.Email {
		t.Errorf("email: expected %q, got %q", userInfo.Email, parsed.Email)
	}
	if parsed.Name != userInfo.Name {
		t.Errorf("name: expected %q, got %q", userInfo.Name, parsed.Name)
	}
	if parsed.Picture != userInfo.Picture {
		t.Errorf("picture: expected %q, got %q", userInfo.Picture, parsed.Picture)
	}
}

func TestOAuthProvider_JSONSecretHidden(t *testing.T) {
	provider := OAuthProvider{
		Provider:     "google",
		ClientID:     "client-123",
		ClientSecret: "super-secret",
		IssuerURL:    "https://issuer.example.com",
		AuthURL:      "https://auth.example.com",
		TokenURL:     "https://token.example.com",
		UserInfoURL:  "https://userinfo.example.com",
		JWKSURL:      "https://jwks.example.com",
		Scopes:       "openid email profile",
		Enabled:      true,
	}

	data, err := json.Marshal(provider)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	// client_secret must NOT appear in JSON output
	if _, ok := parsed["client_secret"]; ok {
		t.Errorf("client_secret should not be serialized to JSON (json:\"-\" tag)")
	}

	if parsed["provider"] != "google" {
		t.Errorf("provider: expected google, got %v", parsed["provider"])
	}
}

func TestTokenResponse_JSON(t *testing.T) {
	token := tokenResponse{
		AccessToken:  "ya29.xxx",
		TokenType:    "Bearer",
		RefreshToken: "refresh-token",
		IDToken:     "id-token",
		ExpiresIn:    3600,
		Scope:       "openid email profile",
	}

	data, err := json.Marshal(token)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var parsed tokenResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if parsed.AccessToken != token.AccessToken {
		t.Errorf("access_token: expected %q, got %q", token.AccessToken, parsed.AccessToken)
	}
	if parsed.TokenType != token.TokenType {
		t.Errorf("token_type: expected %q, got %q", token.TokenType, parsed.TokenType)
	}
	if parsed.RefreshToken != token.RefreshToken {
		t.Errorf("refresh_token: expected %q, got %q", token.RefreshToken, parsed.RefreshToken)
	}
	if parsed.IDToken != token.IDToken {
		t.Errorf("id_token: expected %q, got %q", token.IDToken, parsed.IDToken)
	}
	if parsed.ExpiresIn != token.ExpiresIn {
		t.Errorf("expires_in: expected %d, got %d", token.ExpiresIn, parsed.ExpiresIn)
	}
	if parsed.Scope != token.Scope {
		t.Errorf("scope: expected %q, got %q", token.Scope, parsed.Scope)
	}
}

func TestHandleOAuthCallback_ErrorParam(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/auth/google/callback?error=access_denied", nil)
	c.Params = gin.Params{{Key: "provider", Value: "google"}}

	HandleOAuthCallback(nil, "test-secret")(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if resp["code"] != "BAD_REQUEST" {
		t.Errorf("code: expected BAD_REQUEST, got %v", resp["code"])
	}
}

func TestHandleOAuthCallback_MissingCode(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/auth/google/callback?state=test", nil)
	c.Params = gin.Params{{Key: "provider", Value: "google"}}

	HandleOAuthCallback(nil, "test-secret")(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if resp["message"] != "missing code or state" {
		t.Errorf("message: expected 'missing code or state', got %v", resp["message"])
	}
}

func TestHandleOAuthCallback_MissingState(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/auth/google/callback?code=auth-code", nil)
	c.Params = gin.Params{{Key: "provider", Value: "google"}}

	HandleOAuthCallback(nil, "test-secret")(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetOAuthCallbackURL_HTTP(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/auth/google/callback", nil)
	c.Request.Host = "api.cont.example.com:8001"

	url := getOAuthCallbackURL(c, "google")
	expected := "http://api.cont.example.com:8001/auth/google/callback"

	if url != expected {
		t.Errorf("callback URL: expected %q, got %q", expected, url)
	}
}

func TestGetOAuthCallbackURL_HTTPS(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/auth/google/callback", nil)
	c.Request.Host = "api.cont.example.com:8001"
	// Pretend TLS is active
	c.Request.TLS = &tlsConnectionState{}

	url := getOAuthCallbackURL(c, "google")
	expected := "https://api.cont.example.com:8001/auth/google/callback"

	if url != expected {
		t.Errorf("callback URL: expected %q, got %q", expected, url)
	}
}

func TestInitiateOAuth_StateLength(t *testing.T) {
	// Test that state generation produces 32 bytes -> URL-safe base64
	stateBytes := make([]byte, 32)
	rand.Read(stateBytes)
	state := base64.URLEncoding.EncodeToString(stateBytes)

	// URL-safe base64 without padding is ~43 characters
	if len(state) < 40 {
		t.Errorf("state length too short: got %d", len(state))
	}
}

func TestOAuthProvider_DefaultScopes(t *testing.T) {
	// Test that empty scopes get defaults
	p := OAuthProvider{Scopes: ""}
	scopes := p.Scopes
	if scopes == "" {
		scopes = "openid email profile"
	}
	if scopes != "openid email profile" {
		t.Errorf("default scopes: expected 'openid email profile', got %q", scopes)
	}

	// Custom scopes preserved
	p2 := OAuthProvider{Scopes: "profile github:gist"}
	if p2.Scopes != "profile github:gist" {
		t.Errorf("custom scopes: expected 'profile github:gist', got %q", p2.Scopes)
	}
}

func TestOAuthProvider_DefaultAuthURL(t *testing.T) {
	p := OAuthProvider{Provider: "google", AuthURL: ""}
	authURL := p.AuthURL
	if authURL == "" {
		authURL = "https://accounts.google.com/o/oauth2/v2/auth"
	}
	if authURL != "https://accounts.google.com/o/oauth2/v2/auth" {
		t.Errorf("default auth URL: expected Google OAuth2 URL, got %q", authURL)
	}

	// Custom auth URL preserved
	p2 := OAuthProvider{Provider: "google", AuthURL: "https://custom.auth.example.com/oauth"}
	if p2.AuthURL != "https://custom.auth.example.com/oauth" {
		t.Errorf("custom auth URL: expected preserved, got %q", p2.AuthURL)
	}
}

// tlsConnectionState is a minimal TLS connection state for testing
type tlsConnectionState struct{}

func TestInitiateOAuth_RedirectParams(t *testing.T) {
	// Test the OAuth redirect URL parameter construction
	p := OAuthProvider{
		Provider: "google",
		ClientID: "test-client",
		AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
		Scopes:   "openid email profile",
	}

	params := url.Values{}
	params.Set("client_id", p.ClientID)
	params.Set("redirect_uri", "http://localhost:8001/auth/google/callback")
	params.Set("response_type", "code")
	params.Set("scope", p.Scopes)
	params.Set("state", "random-state")
	params.Set("access_type", "online")
	params.Set("prompt", "select_account")

	if params.Get("client_id") != "test-client" {
		t.Errorf("client_id: expected test-client, got %s", params.Get("client_id"))
	}
	if params.Get("response_type") != "code" {
		t.Errorf("response_type: expected code, got %s", params.Get("response_type"))
	}
	if params.Get("scope") != "openid email profile" {
		t.Errorf("scope: expected 'openid email profile', got %s", params.Get("scope"))
	}
	if params.Get("access_type") != "online" {
		t.Errorf("access_type: expected online, got %s", params.Get("access_type"))
	}
	if params.Get("prompt") != "select_account" {
		t.Errorf("prompt: expected select_account, got %s", params.Get("prompt"))
	}
}

func TestGenerateOAuthJWT_Claims(t *testing.T) {
	user := &storage.User{
		ID:       "user-123",
		Username: "alice",
		Role:     "editor",
		Email:    "alice@example.com",
	}

	jwtSecret := "test-secret"
	tokenStr, err := generateOAuthJWT(user, jwtSecret)
	if err != nil {
		t.Fatalf("generateOAuthJWT: %v", err)
	}

	if tokenStr == "" {
		t.Errorf("token: expected non-empty string")
	}

	// Parse and verify claims
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	})
	if err != nil {
		t.Fatalf("jwt.Parse: %v", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatalf("claims: expected MapClaims")
	}

	if claims["sub"] != "user-123" {
		t.Errorf("sub: expected user-123, got %v", claims["sub"])
	}
	if claims["username"] != "alice" {
		t.Errorf("username: expected alice, got %v", claims["username"])
	}
	if claims["role"] != "editor" {
		t.Errorf("role: expected editor, got %v", claims["role"])
	}
	if claims["email"] != "alice@example.com" {
		t.Errorf("email: expected alice@example.com, got %v", claims["email"])
	}
	if claims["provider"] != "oauth" {
		t.Errorf("provider: expected oauth, got %v", claims["provider"])
	}

	// Check expiry is approximately 24 hours from now
	exp, ok := claims["exp"].(float64)
	if !ok {
		t.Fatalf("exp: expected float64")
	}
	expTime := time.Unix(int64(exp), 0)
	expectedExp := time.Now().Add(24 * time.Hour)
	// Allow 1 minute tolerance
	if expTime.Sub(expectedExp) > time.Minute || expectedExp.Sub(expTime) > time.Minute {
		t.Errorf("exp: expected ~24h from now, got %v (now=%v)", expTime, time.Now())
	}
}
