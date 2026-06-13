package routes

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/ttcccat-tech/cont/admin-api/storage"
)

// OAuthProvider represents an OAuth2 provider configuration
type OAuthProvider struct {
	Provider      string `json:"provider"`
	ClientID      string `json:"client_id"`
	ClientSecret  string `json:"-"`
	IssuerURL     string `json:"issuer_url,omitempty"`
	AuthURL       string `json:"authorization_url"`
	TokenURL      string `json:"token_url"`
	UserInfoURL   string `json:"userinfo_url"`
	JWKSURL       string `json:"jwks_url,omitempty"`
	Scopes        string `json:"scopes"`
	Enabled       bool   `json:"enabled"`
}

// OAuthState represents a temporary OAuth state for CSRF protection
type OAuthState struct {
	State       string    `json:"state"`
	Provider    string    `json:"provider"`
	RedirectURI string    `json:"redirect_uri,omitempty"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// OAuthUserInfo represents user info from the identity provider
type OAuthUserInfo struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture,omitempty"`
}

// ListOAuthProviders returns all configured OAuth providers
func ListOAuthProviders(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		providers, err := store.ListOAuthProviders()
		if err != nil {
			internalError(c)
			return
		}
		// Strip secrets for list response
		var public []map[string]interface{}
		for _, p := range providers {
			public = append(public, map[string]interface{}{
				"provider":          p.Provider,
				"client_id":         p.ClientID,
				"issuer_url":        p.IssuerURL,
				"authorization_url": p.AuthorizationURL,
				"token_url":         p.TokenURL,
				"userinfo_url":      p.UserInfoURL,
				"jwks_url":          p.JWKSURL,
				"scopes":            p.Scopes,
				"enabled":           p.Enabled,
			})
		}
		c.JSON(200, public)
	}
}

// GetOAuthProvider returns a single OAuth provider config
func GetOAuthProvider(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		provider := c.Param("provider")
		p, err := store.GetOAuthProvider(provider)
		if err != nil {
			notFound(c, "provider not found")
			return
		}
		c.JSON(200, p)
	}
}

// CreateOAuthProvider creates a new OAuth provider
func CreateOAuthProvider(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input struct {
			Provider         string `json:"provider" binding:"required"`
			ClientID         string `json:"client_id" binding:"required"`
			ClientSecret     string `json:"client_secret" binding:"required"`
			IssuerURL        string `json:"issuer_url"`
			AuthorizationURL string `json:"authorization_url"`
			TokenURL         string `json:"token_url" binding:"required"`
			UserInfoURL      string `json:"userinfo_url"`
			JWKSURL          string `json:"jwks_url"`
			Scopes           string `json:"scopes"`
			Enabled          bool   `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			badRequestMsg(c, err.Error())
			return
		}
		p := &storage.OAuthProviderModel{
			Provider:         input.Provider,
			ClientID:         input.ClientID,
			ClientSecret:     input.ClientSecret,
			IssuerURL:        input.IssuerURL,
			AuthorizationURL: input.AuthorizationURL,
			TokenURL:         input.TokenURL,
			UserInfoURL:      input.UserInfoURL,
			JWKSURL:          input.JWKSURL,
			Scopes:           input.Scopes,
			Enabled:          input.Enabled,
		}
		created, err := store.CreateOAuthProvider(p)
		if err != nil {
			badRequestMsg(c, err.Error())
			return
		}
		actor, _ := c.Get("username")
		actorStr := ""
		if actor != nil {
			actorStr = actor.(string)
		}
		store.CreateAuditLog(&storage.AuditLog{
			AuditType:     "create",
			TargetType:    "OAuthProvider",
			TargetID:      created.Provider,
			ActorUsername: actorStr,
			Description:   "Created OAuth provider: " + created.Provider,
		})
		c.JSON(201, created)
	}
}

// UpdateOAuthProvider updates an existing OAuth provider
func UpdateOAuthProvider(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		provider := c.Param("provider")
		var input struct {
			ClientID         string `json:"client_id" binding:"required"`
			ClientSecret     string `json:"client_secret"` // optional — empty means don't update
			IssuerURL        string `json:"issuer_url"`
			AuthorizationURL string `json:"authorization_url"`
			TokenURL         string `json:"token_url" binding:"required"`
			UserInfoURL      string `json:"userinfo_url"`
			JWKSURL          string `json:"jwks_url"`
			Scopes           string `json:"scopes"`
			Enabled          bool   `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			badRequestMsg(c, err.Error())
			return
		}
		p := &storage.OAuthProviderModel{
			ClientID:         input.ClientID,
			ClientSecret:     input.ClientSecret,
			IssuerURL:        input.IssuerURL,
			AuthorizationURL: input.AuthorizationURL,
			TokenURL:         input.TokenURL,
			UserInfoURL:      input.UserInfoURL,
			JWKSURL:          input.JWKSURL,
			Scopes:           input.Scopes,
			Enabled:          input.Enabled,
		}
		updated, err := store.UpdateOAuthProvider(provider, p)
		if err != nil {
			notFound(c, "provider not found")
			return
		}
		actor, _ := c.Get("username")
		actorStr := ""
		if actor != nil {
			actorStr = actor.(string)
		}
		store.CreateAuditLog(&storage.AuditLog{
			AuditType:     "update",
			TargetType:    "OAuthProvider",
			TargetID:      provider,
			ActorUsername: actorStr,
			Description:   "Updated OAuth provider: " + provider,
		})
		c.JSON(200, updated)
	}
}

// DeleteOAuthProvider deletes an OAuth provider
func DeleteOAuthProvider(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		provider := c.Param("provider")
		if err := store.DeleteOAuthProvider(provider); err != nil {
			notFound(c, "provider not found")
			return
		}
		actor, _ := c.Get("username")
		actorStr := ""
		if actor != nil {
			actorStr = actor.(string)
		}
		store.CreateAuditLog(&storage.AuditLog{
			AuditType:     "delete",
			TargetType:    "OAuthProvider",
			TargetID:      provider,
			ActorUsername: actorStr,
			Description:   "Deleted OAuth provider: " + provider,
		})
		c.Status(204)
	}
}

// InitiateOAuth starts the OAuth2 authorization code flow
func InitiateOAuth(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		provider := c.Param("provider")
		redirectURI := c.Query("redirect_uri")
		db := store.DB()

		// Load provider config
		var p OAuthProvider
		err := db.QueryRow(`
			SELECT provider, client_id, client_secret, authorization_url,
			       token_url, userinfo_url, scopes, enabled
			FROM oauth_providers WHERE provider = $1 AND enabled = true`, provider,
		).Scan(&p.Provider, &p.ClientID, &p.ClientSecret, &p.AuthURL,
			&p.TokenURL, &p.UserInfoURL, &p.Scopes, &p.Enabled)
		if err != nil {
			notFound(c, "provider not found or disabled")
			return
		}

		// Generate state (CSRF token)
		stateBytes := make([]byte, 32)
		if _, err := rand.Read(stateBytes); err != nil {
			internalError(c)
			return
		}
		state := base64.URLEncoding.EncodeToString(stateBytes)

		// Store state in DB (expires in 10 minutes)
		expiresAt := time.Now().Add(10 * time.Minute)
		_, err = db.Exec(`
			INSERT INTO oauth_states (state, provider, redirect_uri, expires_at)
			VALUES ($1, $2, $3, $4) ON CONFLICT (state) DO UPDATE
			SET provider=$2, redirect_uri=$3, expires_at=$4`,
			state, provider, redirectURI, expiresAt)
		if err != nil {
			internalError(c)
			return
		}

		// Build authorization URL
		authURL := p.AuthURL
		if authURL == "" {
			authURL = "https://accounts.google.com/o/oauth2/v2/auth"
		}
		callbackURL := getOAuthCallbackURL(c, provider)
		scopes := p.Scopes
		if scopes == "" {
			scopes = "openid email profile"
		}

		params := url.Values{}
		params.Set("client_id", p.ClientID)
		params.Set("redirect_uri", callbackURL)
		params.Set("response_type", "code")
		params.Set("scope", scopes)
		params.Set("state", state)
		params.Set("access_type", "online")
		params.Set("prompt", "select_account")

		// Audit log: OAuth initiation (login attempt)
		store.CreateAuditLog(&storage.AuditLog{
			AuditType:     "login",
			TargetType:    "OAuthProvider",
			TargetID:      provider,
			ActorUsername: "anonymous",
			Description:   "OAuth login initiated with " + provider,
		})

		c.Redirect(http.StatusFound, authURL+"?"+params.Encode())
	}
}

// HandleOAuthCallback processes the OAuth2 callback (authorization code exchange)
func HandleOAuthCallback(store *storage.Store, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		provider := c.Param("provider")
		code := c.Query("code")
		state := c.Query("state")
		errorParam := c.Query("error")
		db := store.DB()

		if errorParam != "" {
			badRequestWithDetails(c, "oauth_error", errorParam)
			return
		}

		if code == "" || state == "" {
			badRequestMsg(c, "missing code or state")
			return
		}

		// Validate state
		var storedState OAuthState
		err := db.QueryRow(`
			SELECT state, provider, redirect_uri, expires_at
			FROM oauth_states WHERE state = $1`, state,
		).Scan(&storedState.State, &storedState.Provider, &storedState.RedirectURI, &storedState.ExpiresAt)
		if err != nil || storedState.Provider != provider {
			badRequestMsg(c, "invalid state")
			return
		}
		if time.Now().After(storedState.ExpiresAt) {
			badRequestMsg(c, "state expired")
			return
		}

		// Delete state (one-time use)
		db.Exec("DELETE FROM oauth_states WHERE state = $1", state)

		// Load provider config
		var p OAuthProvider
		err = db.QueryRow(`
			SELECT provider, client_id, client_secret, token_url, userinfo_url
			FROM oauth_providers WHERE provider = $1 AND enabled = true`, provider,
		).Scan(&p.Provider, &p.ClientID, &p.ClientSecret, &p.TokenURL, &p.UserInfoURL)
		if err != nil {
			notFound(c, "provider not found")
			return
		}

		// Exchange code for tokens
		callbackURL := getOAuthCallbackURL(c, provider)
		tokenData, err := exchangeCodeForToken(p, code, callbackURL)
		if err != nil {
			badGateway(c, "token exchange failed", err.Error())
			return
		}

		// Fetch user info
		userInfo, err := fetchUserInfo(p, tokenData.AccessToken)
		if err != nil {
			badGateway(c, "failed to fetch user info", err.Error())
			return
		}

		// Find or create user
		user, err := store.GetOrCreateOAuthUser(provider, userInfo.Subject, userInfo.Email, userInfo.Name)
		if err != nil {
			internalError(c)
			return
		}

		// Audit log: OAuth login success
		store.CreateAuditLog(&storage.AuditLog{
			AuditType:     "login",
			TargetType:    "User",
			TargetID:      user.ID,
			ActorUsername: user.Username,
			Description:   "OAuth login successful via " + provider,
		})

		// Generate JWT
		token, err := generateOAuthJWT(user, jwtSecret)
		if err != nil {
			internalError(c)
			return
		}

		// Redirect with token if redirect_uri was provided
		if storedState.RedirectURI != "" {
			redirectURL, _ := url.Parse(storedState.RedirectURI)
			q := redirectURL.Query()
			q.Set("token", token)
			redirectURL.RawQuery = q.Encode()
			c.Redirect(http.StatusFound, redirectURL.String())
			return
		}

		// Otherwise return JSON
		c.JSON(200, gin.H{
			"token": token,
			"user":  user,
		})
	}
}

// ── Token Exchange ─────────────────────────────────────────────────────────

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken     string `json:"id_token,omitempty"`
	ExpiresIn   int    `json:"expires_in,omitempty"`
	Scope       string `json:"scope,omitempty"`
}

func exchangeCodeForToken(p OAuthProvider, code, callbackURL string) (*tokenResponse, error) {
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", p.ClientID)
	data.Set("client_secret", p.ClientSecret)
	data.Set("redirect_uri", callbackURL)
	data.Set("grant_type", "authorization_code")

	resp, err := http.PostForm(p.TokenURL, data)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var token tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}
	return &token, nil
}

func fetchUserInfo(p OAuthProvider, accessToken string) (*OAuthUserInfo, error) {
	userInfoURL := p.UserInfoURL
	if userInfoURL == "" {
		userInfoURL = "https://openidconnect.googleapis.com/v1/userinfo"
	}

	req, _ := http.NewRequest("GET", userInfoURL, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("userinfo request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("userinfo endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var userInfo OAuthUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode userinfo: %w", err)
	}
	return &userInfo, nil
}

// ── JWT Generation ─────────────────────────────────────────────────────────

func generateOAuthJWT(user *storage.User, jwtSecret string) (string, error) {
	claims := jwt.MapClaims{
		"sub":      user.ID,
		"username": user.Username,
		"role":     user.Role,
		"email":    user.Email,
		"provider": "oauth",
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
		"iat":      time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtSecret))
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func getOAuthCallbackURL(c *gin.Context, provider string) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	host := c.Request.Host
	return fmt.Sprintf("%s://%s/auth/%s/callback", scheme, host, provider)
}
