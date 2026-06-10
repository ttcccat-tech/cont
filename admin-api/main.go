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
		auth.GET("/me", routes.AuthRequired(jwtSecret), routes.GetMe(jwtSecret))
		// SSO endpoints can be added here for OAuth2/OIDC providers
	}

	// Admin API — Kong-compatible (auth protected)
	admin := r.Group("/")
	admin.Use(routes.AuthRequired(jwtSecret))
	{
		svcs := admin.Group("/services")
		{
			svcs.GET("", routes.RequirePermission("services", false), routes.ListServices(store))
			svcs.POST("", routes.RequirePermission("services", true), routes.CreateService(store))
			svcs.GET("/:id", routes.RequirePermission("services", false), routes.GetService(store))
			svcs.PUT("/:id", routes.RequirePermission("services", true), routes.UpdateService(store))
			svcs.PATCH("/:id", routes.RequirePermission("services", true), routes.UpdateService(store))
			svcs.DELETE("/:id", routes.RequirePermission("services", true), routes.DeleteService(store))
		}

		rt := admin.Group("/routes")
		{
			rt.GET("", routes.RequirePermission("routes", false), routes.ListRoutes(store))
			rt.POST("", routes.RequirePermission("routes", true), routes.CreateRoute(store))
			rt.GET("/:id", routes.RequirePermission("routes", false), routes.GetRoute(store))
			rt.PUT("/:id", routes.RequirePermission("routes", true), routes.UpdateRoute(store))
			rt.PATCH("/:id", routes.RequirePermission("routes", true), routes.UpdateRoute(store))
			rt.DELETE("/:id", routes.RequirePermission("routes", true), routes.DeleteRoute(store))
		}

		up := admin.Group("/upstreams")
		{
			up.GET("", routes.RequirePermission("upstreams", false), routes.ListUpstreams(store))
			up.POST("", routes.RequirePermission("upstreams", true), routes.CreateUpstream(store))
			up.GET("/:id", routes.RequirePermission("upstreams", false), routes.GetUpstream(store))
			up.PUT("/:id", routes.RequirePermission("upstreams", true), routes.UpdateUpstream(store))
			up.PATCH("/:id", routes.RequirePermission("upstreams", true), routes.UpdateUpstream(store))
			up.DELETE("/:id", routes.RequirePermission("upstreams", true), routes.DeleteUpstream(store))
			up.GET("/:id/targets", routes.RequirePermission("targets", false), routes.ListTargets(store))
			up.POST("/:id/targets", routes.RequirePermission("targets", true), routes.CreateTarget(store))
			up.PUT("/:id/targets/:target_id", routes.RequirePermission("targets", true), routes.UpdateTarget(store))
			up.PATCH("/:id/targets/:target_id", routes.RequirePermission("targets", true), routes.UpdateTarget(store))
			up.DELETE("/:id/targets/:target_id", routes.RequirePermission("targets", true), routes.DeleteTarget(store))
		}

		cons := admin.Group("/consumers")
		{
			cons.GET("", routes.RequirePermission("consumers", false), routes.ListConsumers(store))
			cons.POST("", routes.RequirePermission("consumers", true), routes.CreateConsumer(store))
			cons.GET("/:id", routes.RequirePermission("consumers", false), routes.GetConsumer(store))
			cons.PUT("/:id", routes.RequirePermission("consumers", true), routes.UpdateConsumer(store))
			cons.PATCH("/:id", routes.RequirePermission("consumers", true), routes.UpdateConsumer(store))
			cons.DELETE("/:id", routes.RequirePermission("consumers", true), routes.DeleteConsumer(store))
		}

		plugs := admin.Group("/plugins")
		{
			plugs.GET("", routes.RequirePermission("plugins", false), routes.ListPlugins(store))
			plugs.POST("", routes.RequirePermission("plugins", true), routes.CreatePlugin(store))
			plugs.GET("/:id", routes.RequirePermission("plugins", false), routes.GetPlugin(store))
			plugs.PUT("/:id", routes.RequirePermission("plugins", true), routes.UpdatePlugin(store))
			plugs.PATCH("/:id", routes.RequirePermission("plugins", true), routes.UpdatePlugin(store))
			plugs.DELETE("/:id", routes.RequirePermission("plugins", true), routes.DeletePlugin(store))
		}

		// Workspaces
		ws := admin.Group("/workspaces")
		{
			ws.GET("", routes.RequirePermission("workspaces", false), routes.ListWorkspaces(store))
			ws.POST("", routes.RequirePermission("workspaces", true), routes.CreateWorkspace(store))
			ws.GET("/:id", routes.RequirePermission("workspaces", false), routes.GetWorkspace(store))
			ws.PUT("/:id", routes.RequirePermission("workspaces", true), routes.UpdateWorkspace(store))
			ws.PATCH("/:id", routes.RequirePermission("workspaces", true), routes.UpdateWorkspace(store))
			ws.DELETE("/:id", routes.RequirePermission("workspaces", true), routes.DeleteWorkspace(store))
		}

		// Roles (RBAC)
		admin.GET("/roles", routes.ListRoles())
		admin.GET("/roles/:role/permissions", routes.GetRolePermissions())

		// Users (admin only)
		users := admin.Group("/users")
		{
			users.GET("", routes.RequirePermission("users", false), routes.ListUsers(store))
			users.POST("", routes.RequirePermission("users", true), routes.CreateUser(store))
			users.GET("/:id", routes.RequirePermission("users", false), routes.GetUser(store))
			users.PUT("/:id", routes.RequirePermission("users", true), routes.UpdateUser(store))
			users.PATCH("/:id", routes.RequirePermission("users", true), routes.UpdateUser(store))
			users.DELETE("/:id", routes.RequirePermission("users", true), routes.DeleteUser(store))
		}

		// Auth Groups
		groups := admin.Group("/groups")
		{
			groups.GET("", routes.ListAuthGroups(store))
			groups.POST("", routes.CreateAuthGroup(store))
			groups.GET("/:id", routes.GetAuthGroup(store))
			groups.PUT("/:id", routes.UpdateAuthGroup(store))
			groups.PATCH("/:id", routes.UpdateAuthGroup(store))
			groups.DELETE("/:id", routes.DeleteAuthGroup(store))
		}

		// Resources
		admin.GET("/resources", routes.ListResources(store))

		// Audit Logs
		admin.GET("/audit", routes.ListAuditLogs(store))

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
			apikeys.GET("", routes.ListAPIKeyRequests(store))
			apikeys.POST("", routes.CreateAPIKeyRequest(store))
			apikeys.GET("/:id", routes.GetAPIKeyRequest(store))
			apikeys.PUT("/:id", routes.UpdateAPIKeyRequest(store))
			apikeys.PATCH("/:id", routes.UpdateAPIKeyRequest(store))
			apikeys.DELETE("/:id", routes.DeleteAPIKeyRequest(store))
		}

		// Config Snapshots
		admin.GET("/snapshots", routes.ListConfigSnapshots(store))
		admin.POST("/snapshots", routes.CreateConfigSnapshot(store))
		admin.DELETE("/snapshots/:id", routes.DeleteConfigSnapshot(store))

		// Health & Config Check (for HealthPortal)
		admin.GET("/health-check", routes.HealthCheck(store))
		admin.GET("/config-check", routes.ConfigCheck())
	}

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
