package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/ttcccat-tech/cont/admin-api/routes"
	"github.com/ttcccat-tech/cont/admin-api/storage"
)

func main() {
	// Init storage
	db, err := storage.NewPostgres(os.Getenv("POSTGRES_URL"))
	if err != nil {
		log.Fatalf("Failed to connect to postgres: %v", err)
	}
	defer db.Close()

	if err := storage.RunMigrations(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	rdb := storage.NewRedis(os.Getenv("REDIS_URL"))

	// Wire up store
	store := storage.NewStore(db, rdb)

	// Seed default users
	if err := store.SeedDefaultUsers(); err != nil {
		log.Printf("Warning: failed to seed default users: %v", err)
	}

	// Seed default plans (Stripe billing)
	if err := store.InitDefaultPlans(); err != nil {
		log.Printf("Warning: failed to init default plans: %v", err)
	}

	// JWT secret
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "cont-dev-secret-change-in-production"
		log.Printf("WARNING: JWT_SECRET not set, using default. DO NOT use in production.")
	}

	// Setup router
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	// Health + Metrics
	r.GET("/status", routes.Status(store))
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Swagger docs (public, before auth)
	r.GET("/docs", func(c *gin.Context) {
		c.Header("Content-Type", "application/x-yaml")
		c.File("docs/swagger.yaml")
	})
	r.GET("/docs.json", func(c *gin.Context) {
		c.Header("Content-Type", "application/json")
		c.File("docs/swagger.yaml")
	})

	// Auth
	auth := r.Group("/auth")
	{
		auth.POST("/login", routes.Login(store, jwtSecret))
		auth.POST("/register/send-otp", routes.SendOTP(store))
		auth.POST("/register/verify-otp", routes.VerifyOTP(store, jwtSecret))
		auth.GET("/me", routes.AuthRequired(jwtSecret), routes.GetMe(jwtSecret))
		// OAuth2/OIDC SSO
		auth.GET("/oauth/providers", routes.ListOAuthProviders(store))
		auth.GET("/:provider", routes.InitiateOAuth(store))
		auth.GET("/:provider/callback", routes.HandleOAuthCallback(store, jwtSecret))
	}

	// Internal endpoint for proxy auth validation (public, no auth)
	r.GET("/internal/validate-cred/:type/:key", routes.ValidateCredential(store))
	r.GET("/internal/validate-jwt/:token", routes.ValidateJWT(store, jwtSecret))
	r.GET("/internal/plugins", routes.ListInternalPlugins(store))

	// Admin API — Kong-compatible (auth protected)
	admin := r.Group("/")
	admin.Use(routes.AuthRequired(jwtSecret))
	{
		svcs := admin.Group("/services")
		{
			svcs.GET("", routes.RequirePermission(store, "services", false), routes.ListServices(store))
			svcs.POST("", routes.RequirePermission(store, "services", true), routes.CreateService(store))
			svcs.GET("/:id", routes.RequirePermission(store, "services", false), routes.GetService(store))
			svcs.PUT("/:id", routes.RequirePermission(store, "services", true), routes.UpdateService(store))
			svcs.PATCH("/:id", routes.RequirePermission(store, "services", true), routes.UpdateService(store))
			svcs.DELETE("/:id", routes.RequirePermission(store, "services", true), routes.DeleteService(store))
		}

		rt := admin.Group("/routes")
		{
			rt.GET("", routes.RequirePermission(store, "routes", false), routes.ListRoutes(store))
			rt.POST("", routes.RequirePermission(store, "routes", true), routes.CreateRoute(store))
			rt.GET("/:id", routes.RequirePermission(store, "routes", false), routes.GetRoute(store))
			rt.PUT("/:id", routes.RequirePermission(store, "routes", true), routes.UpdateRoute(store))
			rt.PATCH("/:id", routes.RequirePermission(store, "routes", true), routes.UpdateRoute(store))
			rt.DELETE("/:id", routes.RequirePermission(store, "routes", true), routes.DeleteRoute(store))
		}

		up := admin.Group("/upstreams")
		{
			up.GET("", routes.RequirePermission(store, "upstreams", false), routes.ListUpstreams(store))
			up.POST("", routes.RequirePermission(store, "upstreams", true), routes.CreateUpstream(store))
			up.GET("/:id", routes.RequirePermission(store, "upstreams", false), routes.GetUpstream(store))
			up.PUT("/:id", routes.RequirePermission(store, "upstreams", true), routes.UpdateUpstream(store))
			up.PATCH("/:id", routes.RequirePermission(store, "upstreams", true), routes.UpdateUpstream(store))
			up.DELETE("/:id", routes.RequirePermission(store, "upstreams", true), routes.DeleteUpstream(store))
			up.GET("/:id/health", routes.RequirePermission(store, "upstreams", false), routes.GetUpstreamHealth(store))
			up.GET("/:id/targets", routes.RequirePermission(store, "targets", false), routes.ListTargets(store))
			up.POST("/:id/targets", routes.RequirePermission(store, "targets", true), routes.CreateTarget(store))
			up.PUT("/:id/targets/:target_id", routes.RequirePermission(store, "targets", true), routes.UpdateTarget(store))
			up.PATCH("/:id/targets/:target_id", routes.RequirePermission(store, "targets", true), routes.UpdateTarget(store))
			up.DELETE("/:id/targets/:target_id", routes.RequirePermission(store, "targets", true), routes.DeleteTarget(store))
		}

		cons := admin.Group("/consumers")
		{
			cons.GET("", routes.RequirePermission(store, "consumers", false), routes.ListConsumers(store))
			cons.POST("", routes.RequirePermission(store, "consumers", true), routes.CreateConsumer(store))
			cons.GET("/:id", routes.RequirePermission(store, "consumers", false), routes.GetConsumer(store))
			cons.PUT("/:id", routes.RequirePermission(store, "consumers", true), routes.UpdateConsumer(store))
			cons.PATCH("/:id", routes.RequirePermission(store, "consumers", true), routes.UpdateConsumer(store))
			cons.DELETE("/:id", routes.RequirePermission(store, "consumers", true), routes.DeleteConsumer(store))
			// Consumer credentials: /consumers/:id/key-auth/credentials, etc.
			cred := cons.Group("/:id")
			cred.GET("/key-auth/credentials", routes.RequirePermission(store, "consumers", false), routes.ListCredentials(store, "key-auth"))
			cred.POST("/key-auth/credentials", routes.RequirePermission(store, "consumers", true), routes.CreateCredential(store, "key-auth"))
			cred.DELETE("/key-auth/credentials/:credId", routes.RequirePermission(store, "consumers", true), routes.DeleteCredential(store, "key-auth"))
			cred.GET("/basic-auth/credentials", routes.RequirePermission(store, "consumers", false), routes.ListCredentials(store, "basic-auth"))
			cred.POST("/basic-auth/credentials", routes.RequirePermission(store, "consumers", true), routes.CreateCredential(store, "basic-auth"))
			cred.DELETE("/basic-auth/credentials/:credId", routes.RequirePermission(store, "consumers", true), routes.DeleteCredential(store, "basic-auth"))
			cred.GET("/hmac-auth/credentials", routes.RequirePermission(store, "consumers", false), routes.ListCredentials(store, "hmac-auth"))
			cred.POST("/hmac-auth/credentials", routes.RequirePermission(store, "consumers", true), routes.CreateCredential(store, "hmac-auth"))
			cred.DELETE("/hmac-auth/credentials/:credId", routes.RequirePermission(store, "consumers", true), routes.DeleteCredential(store, "hmac-auth"))
		}

		plugs := admin.Group("/plugins")
		{
			plugs.GET("", routes.RequirePermission(store, "plugins", false), routes.ListPlugins(store))
			plugs.POST("", routes.RequirePermission(store, "plugins", true), routes.CreatePlugin(store))
			plugs.GET("/:id", routes.RequirePermission(store, "plugins", false), routes.GetPlugin(store))
			plugs.PUT("/:id", routes.RequirePermission(store, "plugins", true), routes.UpdatePlugin(store))
			plugs.PATCH("/:id", routes.RequirePermission(store, "plugins", true), routes.UpdatePlugin(store))
			plugs.DELETE("/:id", routes.RequirePermission(store, "plugins", true), routes.DeletePlugin(store))
		}

		// Workspaces
		ws := admin.Group("/workspaces")
		{
			ws.GET("", routes.RequirePermission(store, "workspaces", false), routes.ListWorkspaces(store))
			ws.POST("", routes.RequirePermission(store, "workspaces", true), routes.CreateWorkspace(store))
			ws.GET("/mine", routes.ListMyWorkspaces(store))
			ws.GET("/:id", routes.RequirePermission(store, "workspaces", false), routes.GetWorkspace(store))
			ws.PUT("/:id", routes.RequirePermission(store, "workspaces", true), routes.UpdateWorkspace(store))
			ws.PATCH("/:id", routes.RequirePermission(store, "workspaces", true), routes.UpdateWorkspace(store))
			ws.DELETE("/:id", routes.RequirePermission(store, "workspaces", true), routes.DeleteWorkspace(store))
			// Workspace user assignment management
			ws.GET("/:id/users", routes.RequirePermission(store, "workspaces", false), routes.ListWorkspaceUsers(store))
			ws.PUT("/:id/users", routes.RequirePermission(store, "workspaces", true), routes.SetUserWorkspace(store))
			ws.DELETE("/:id/users/:userId", routes.RequirePermission(store, "workspaces", true), routes.RemoveUserWorkspace(store))
			ws.GET("/users/:userId", routes.RequirePermission(store, "workspaces", false), routes.GetUserWorkspaces(store))
		}

		// Roles (RBAC)
		admin.GET("/roles", routes.ListRoles())
		admin.GET("/roles/:role/permissions", routes.GetRolePermissions())

		// Users (admin only)
		users := admin.Group("/users")
		{
			users.GET("", routes.RequirePermission(store, "users", false), routes.ListUsers(store))
			users.POST("", routes.RequirePermission(store, "users", true), routes.CreateUser(store))
			users.GET("/:id", routes.RequirePermission(store, "users", false), routes.GetUser(store))
			users.PUT("/:id", routes.RequirePermission(store, "users", true), routes.UpdateUser(store))
			users.PATCH("/:id", routes.RequirePermission(store, "users", true), routes.UpdateUser(store))
			users.DELETE("/:id", routes.RequirePermission(store, "users", true), routes.DeleteUser(store))
			users.GET("/:id/resource-permissions", routes.RequirePermission(store, "users", false), routes.ListUserResourcePermissions(store))
			users.PUT("/:id/resource-permissions", routes.RequirePermission(store, "users", true), routes.SetUserResourcePermissions(store))
		}

		// Auth Groups
		groups := admin.Group("/groups")
		{
			groups.GET("", routes.RequirePermission(store, "groups", false), routes.ListAuthGroups(store))
			groups.POST("", routes.RequirePermission(store, "groups", true), routes.CreateAuthGroup(store))
			groups.GET("/:id", routes.RequirePermission(store, "groups", false), routes.GetAuthGroup(store))
			groups.PUT("/:id", routes.RequirePermission(store, "groups", true), routes.UpdateAuthGroup(store))
			groups.PATCH("/:id", routes.RequirePermission(store, "groups", true), routes.UpdateAuthGroup(store))
			groups.DELETE("/:id", routes.RequirePermission(store, "groups", true), routes.DeleteAuthGroup(store))
			groups.GET("/:id/members", routes.RequirePermission(store, "groups", false), routes.GetGroupMembers(store))
			groups.PUT("/:id/members", routes.RequirePermission(store, "groups", true), routes.SetGroupMembers(store))
			groups.GET("/:id/resource-permissions", routes.RequirePermission(store, "groups", false), routes.ListGroupResourcePermissions(store))
			groups.PUT("/:id/resource-permissions", routes.RequirePermission(store, "groups", true), routes.SetGroupResourcePermissions(store))
		}

		// Resources (full CRUD + resource-level RBAC)
		admin.GET("/resources", routes.RequirePermission(store, "resources", false), routes.ListResources(store))
		admin.GET("/resources/:id", routes.RequirePermission(store, "resources", false), routes.GetResource(store))
		admin.POST("/resources", routes.RequirePermission(store, "resources", true), routes.CreateResource(store))
		admin.DELETE("/resources/:id", routes.RequirePermission(store, "resources", true), routes.DeleteResource(store))

		// Audit Logs
		admin.GET("/audit", routes.RequirePermission(store, "groups", false), routes.ListAuditLogs(store))

		// Alert Rules
		alerts := admin.Group("/alerts")
		{
			alerts.GET("/rules", routes.ListAlertRules(store))
			alerts.POST("/rules", routes.CreateAlertRule(store))
			alerts.GET("/rules/:id", routes.GetAlertRule(store))
			alerts.PUT("/rules/:id", routes.UpdateAlertRule(store))
			alerts.PATCH("/rules/:id", routes.UpdateAlertRule(store))
			alerts.DELETE("/rules/:id", routes.DeleteAlertRule(store))
		}

		// API Key Requests
		apikeys := admin.Group("/api-keys")
		{
			apikeys.GET("", routes.RequirePermission(store, "groups", false), routes.ListAPIKeyRequests(store))
			apikeys.POST("", routes.CreateAPIKeyRequest(store))
			apikeys.GET("/:id", routes.RequirePermission(store, "groups", false), routes.GetAPIKeyRequest(store))
			apikeys.PUT("/:id", routes.RequirePermission(store, "groups", true), routes.UpdateAPIKeyRequest(store))
			apikeys.PATCH("/:id", routes.RequirePermission(store, "groups", true), routes.UpdateAPIKeyRequest(store))
			apikeys.DELETE("/:id", routes.RequirePermission(store, "groups", true), routes.DeleteAPIKeyRequest(store))
			apikeys.PUT("/:id/approve", routes.RequirePermission(store, "groups", true), routes.ApproveAPIKey(store))
			apikeys.PUT("/:id/reject", routes.RequirePermission(store, "groups", true), routes.RejectAPIKey(store))
			apikeys.GET("/mine", routes.ListMyAPIKeyRequests(store))
		}

		// Config Snapshots
		admin.GET("/config/snapshots", routes.ListConfigSnapshots(store))
		admin.POST("/config/snapshots", routes.CreateConfigSnapshot(store))
		admin.GET("/config/snapshots/:id", routes.GetConfigSnapshot(store))
		admin.DELETE("/config/snapshots/:id", routes.DeleteConfigSnapshot(store))
		admin.GET("/config/snapshots/diff", routes.DiffConfigSnapshots(store))
		admin.POST("/config/snapshots/:id/rollback", routes.RollbackConfigSnapshot(store))

	// Health & Config Check (public, no auth)
	r.GET("/health-check", routes.HealthCheck(store))
	r.GET("/config-check", routes.ConfigCheck())

	// Admin API — Kong-compatible (auth protected)
		admin.POST("/crypto/rsa-keypair", routes.GenerateRSAKeyPair)

		// Billing / Stripe
		frontendBaseURL := os.Getenv("FRONTEND_BASE_URL")
		if frontendBaseURL == "" {
			frontendBaseURL = "http://localhost:5173"
		}
		admin.GET("/billing/plans", routes.ListPlans(store))
		admin.GET("/billing/subscription", routes.GetSubscription(store))
		admin.POST("/billing/checkout", routes.CreateCheckoutSession(store, frontendBaseURL))
		admin.POST("/billing/portal", routes.CreatePortalSession(store, frontendBaseURL))
		admin.GET("/billing/subscriptions", routes.ListSubscriptions(store))
	}

	// Stripe Webhook — public (Stripe signs with secret, no JWT auth)
	r.POST("/webhooks/stripe", routes.HandleStripeWebhook(store, os.Getenv("STRIPE_WEBHOOK_SECRET")))

	port := os.Getenv("ADMIN_PORT")
	if port == "" {
		port = "8001"
	}
	log.Printf("cont Admin API listening on :%s", port)
	http.ListenAndServe(":"+port, r)
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Kong-Admin-Token")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
