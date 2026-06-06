package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
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

	// Setup router
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	// Health
	r.GET("/status", routes.Status(store))
	r.GET("/metrics", routes.Metrics(store))

	// Auth
	auth := r.Group("/auth")
	{
		auth.POST("/login", routes.Login(store))
		auth.POST("/sso/mock", routes.SSOMock(store))
	}

	// Admin API — Kong-compatible
	admin := r.Group("/")
	{
		svcs := admin.Group("/services")
		{
			svcs.GET("", routes.ListServices(store))
			svcs.POST("", routes.CreateService(store))
			svcs.GET("/:id", routes.GetService(store))
			svcs.PUT("/:id", routes.UpdateService(store))
			svcs.DELETE("/:id", routes.DeleteService(store))
		}

		rt := admin.Group("/routes")
		{
			rt.GET("", routes.ListRoutes(store))
			rt.POST("", routes.CreateRoute(store))
			rt.GET("/:id", routes.GetRoute(store))
			rt.PUT("/:id", routes.UpdateRoute(store))
			rt.DELETE("/:id", routes.DeleteRoute(store))
		}

		up := admin.Group("/upstreams")
		{
			up.GET("", routes.ListUpstreams(store))
			up.POST("", routes.CreateUpstream(store))
			up.GET("/:id", routes.GetUpstream(store))
			up.PUT("/:id", routes.UpdateUpstream(store))
			up.DELETE("/:id", routes.DeleteUpstream(store))
			up.GET("/:id/targets", routes.ListTargets(store))
			up.POST("/:id/targets", routes.CreateTarget(store))
			up.PUT("/:id/targets/:target_id", routes.UpdateTarget(store))
			up.DELETE("/:id/targets/:target_id", routes.DeleteTarget(store))
		}

		cons := admin.Group("/consumers")
		{
			cons.GET("", routes.ListConsumers(store))
			cons.POST("", routes.CreateConsumer(store))
			cons.GET("/:id", routes.GetConsumer(store))
			cons.PUT("/:id", routes.UpdateConsumer(store))
			cons.DELETE("/:id", routes.DeleteConsumer(store))
		}

		plugs := admin.Group("/plugins")
		{
			plugs.GET("", routes.ListPlugins(store))
			plugs.POST("", routes.CreatePlugin(store))
			plugs.GET("/:id", routes.GetPlugin(store))
			plugs.PUT("/:id", routes.UpdatePlugin(store))
			plugs.DELETE("/:id", routes.DeletePlugin(store))
		}

		// Workspaces
		ws := admin.Group("/workspaces")
		{
			ws.GET("", routes.ListWorkspaces(store))
			ws.POST("", routes.CreateWorkspace(store))
			ws.GET("/:id", routes.GetWorkspace(store))
		}
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
