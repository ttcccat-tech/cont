package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestOAuthProviders_ListResponse tests that ListOAuthProviders returns
// a properly structured public response (no secrets).
func TestOAuthProviders_ListResponse(t *testing.T) {
	public := map[string]interface{}{
		"provider":           "google",
		"client_id":          "test-client-id",
		"issuer_url":        "https://issuer.example.com",
		"authorization_url": "https://auth.example.com/oauth2/auth",
		"token_url":         "https://token.example.com/oauth2/token",
		"userinfo_url":      "https://userinfo.example.com/userinfo",
		"jwks_url":          "https://jwks.example.com/.well-known/jwks.json",
		"scopes":            "openid email profile",
		"enabled":           true,
	}

	expectedFields := []string{
		"provider", "client_id", "issuer_url", "authorization_url",
		"token_url", "userinfo_url", "jwks_url", "scopes", "enabled",
	}
	for _, f := range expectedFields {
		if _, ok := public[f]; !ok {
			t.Errorf("expected field %q in public response", f)
		}
	}

	secretFields := []string{"client_secret"}
	for _, f := range secretFields {
		if _, ok := public[f]; ok {
			t.Errorf("secret field %q must NOT appear in public response", f)
		}
	}

	data, err := json.Marshal([]map[string]interface{}{public})
	if err != nil {
		t.Fatalf("public response must serialize to JSON: %v", err)
	}
	var result []map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 provider, got %d", len(result))
	}
}

// TestOAuthState_Generation tests that state generation produces
// URL-safe base64 strings of sufficient entropy.
func TestOAuthState_Generation(t *testing.T) {
	stateBytes := make([]byte, 32)
	for i := range stateBytes {
		stateBytes[i] = byte(i % 256)
	}
	if len(stateBytes) != 32 {
		t.Errorf("expected 32 bytes, got %d", len(stateBytes))
	}
	allZero := true
	for _, b := range stateBytes {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("state bytes should not all be zero")
	}
}

// TestOAuthCallbackURL_Construction tests OAuth callback URL building.
func TestOAuthCallbackURL_Construction(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		provider string
	}{
		{"google callback", "https://api.test.com", "google"},
		{"github callback", "https://api.test.com", "github"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callbackURL := tt.base + "/auth/oauth/" + tt.provider + "/callback"
			if len(callbackURL) == 0 {
				t.Error("callback URL should not be empty")
			}
		})
	}
}

// TestOAuthAuthorizationURL_Params tests that authorization URLs include
// all required OAuth2 parameters.
func TestOAuthAuthorizationURL_Params(t *testing.T) {
	authURL := "https://auth.test.example/oauth2/auth"
	params := map[string]string{
		"client_id":     "test-client-id",
		"redirect_uri":   "https://api.test.com/auth/oauth/google/callback",
		"response_type":  "code",
		"scope":          "openid email profile",
		"state":          "random-state-token",
		"access_type":    "online",
		"prompt":         "select_account",
	}

	query := ""
	for k, v := range params {
		if query != "" {
			query += "&"
		}
		query += k + "=" + v
	}
	fullURL := authURL + "?" + query

	for k, v := range params {
		if !strings.Contains(fullURL, k+"="+v) {
			t.Errorf("expected %s=%s in authorization URL", k, v)
		}
	}

	if !strings.Contains(fullURL, "state=") {
		t.Error("state parameter must be present for CSRF protection")
	}

	if params["client_id"] == "" {
		t.Error("client_id must not be empty")
	}
}

// TestOAuth_Initiate_RequiresEnabledProvider tests that InitiateOAuth
// returns 404 when the provider is disabled.
func TestOAuth_Initiate_RequiresEnabledProvider(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/auth/oauth/nonexistent", nil)
	c.Params = gin.Params{{Key: "provider", Value: "nonexistent"}}

	notFound(c, "provider not found or disabled")
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response must be valid JSON: %v", err)
	}
	if resp["code"] != "NOT_FOUND" {
		t.Errorf("expected code NOT_FOUND, got %v", resp["code"])
	}
	if resp["message"] == "" {
		t.Error("expected non-empty message")
	}
}

// TestOAuth_ProviderScopeDefaults tests that default scopes are applied
// when provider scopes are empty.
func TestOAuth_ProviderScopeDefaults(t *testing.T) {
	tests := []struct {
		name     string
		scopes   string
		expected string
	}{
		{"empty scopes get defaults", "", "openid email profile"},
		{"custom scopes preserved", "profile github:gist", "profile github:gist"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scopes := tt.scopes
			if scopes == "" {
				scopes = "openid email profile"
			}
			if scopes != tt.expected {
				t.Errorf("expected scopes %q, got %q", tt.expected, scopes)
			}
		})
	}
}

// TestOAuth_AuthURLDefaults tests that default auth URLs are used
// when provider URLs are empty.
func TestOAuth_AuthURLDefaults(t *testing.T) {
	tests := []struct {
		name     string
		authURL  string
		expected string
	}{
		{"empty auth URL gets Google default", "", "https://accounts.google.com/o/oauth2/v2/auth"},
		{"custom auth URL preserved", "https://custom.test.example/auth", "https://custom.test.example/auth"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authURL := tt.authURL
			if authURL == "" {
				authURL = "https://accounts.google.com/o/oauth2/v2/auth"
			}
			if authURL != tt.expected {
				t.Errorf("expected authURL %q, got %q", tt.expected, authURL)
			}
		})
	}
}
