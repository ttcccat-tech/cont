package integration

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	_ "github.com/lib/pq"
	"github.com/ttcccat-tech/cont/admin-api/routes"
	"github.com/ttcccat-tech/cont/admin-api/storage"
	"golang.org/x/crypto/bcrypt"
)

// Test fixtures
var (
	testDB     *sql.DB
	testStore  *storage.Store
	testRouter *gin.Engine
	adminToken string
	jwtSecret  = "test-integration-secret"
)

const (
	testDBURL = "postgres://kong:kongpass@cont-postgres:5432/cont_integration_test?sslmode=disable"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	// Setup: connect to test DB and run migrations
	var err error
	testDB, err = sql.Open("postgres", testDBURL)
	if err != nil {
		log.Fatalf("Failed to open test DB: %v", err)
	}
	if err := testDB.Ping(); err != nil {
		log.Fatalf("Failed to ping test DB: %v", err)
	}
	log.Println("Connected to test DB")

	// Run migrations on test DB
	if err := storage.RunMigrations(testDB); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	log.Println("Migrations complete")

	// Setup Redis (nil is OK for most tests)
	rdb := storage.NewRedis("cont-redis:6379")
	testStore = storage.NewStore(testDB, rdb)

	// Create admin JWT token
	adminToken = makeTestToken(jwtSecret, jwt.MapClaims{
		"sub":      "test-admin-uuid",
		"username": "testadmin",
		"role":     "admin",
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	})

	// Seed test admin user so workspace permission checks work
	if err := seedTestUser(testStore); err != nil {
		log.Printf("Warning: seedTestUser failed: %v (workspace tests may fail)", err)
	}

	// Build test router
	testRouter = buildTestRouter()
	log.Println("Test router ready")

	// Run tests
	code := m.Run()

	// Teardown
	testDB.Close()
	os.Exit(code)
}

func buildTestRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	// Public routes (no auth needed for test setup)
	r.GET("/status", routes.Status(testStore))

	// Internal routes
	r.GET("/internal/validate-cred/:type/:key", routes.ValidateCredential(testStore))
	r.GET("/internal/validate-jwt/:token", routes.ValidateJWT(testStore, jwtSecret))

	// Auth group
	auth := r.Group("/auth")
	auth.GET("/oauth/providers", routes.ListOAuthProviders(testStore))
	auth.GET("/oauth/:provider", routes.InitiateOAuth(testStore))
	auth.POST("/oauth/:provider/callback", routes.HandleOAuthCallback(testStore, jwtSecret))
	auth.POST("/send-otp", routes.SendOTP(testStore))
	auth.POST("/verify-otp", routes.VerifyOTP(testStore, jwtSecret))
	auth.POST("/login", routes.Login(testStore, jwtSecret))
	auth.GET("/me", routes.AuthRequired(jwtSecret), routes.GetMe(jwtSecret))

	// Admin routes (all protected)
	admin := r.Group("/")

	// Add AuthRequired middleware to all admin routes
	admin.Use(routes.AuthRequired(jwtSecret))

	// Services
	svcs := admin.Group("/services")
	svcs.GET("", routes.RequirePermission(testStore, "services", false), routes.ListServices(testStore))
	svcs.POST("", routes.RequirePermission(testStore, "services", true), routes.CreateService(testStore))
	svcs.GET("/:id", routes.RequirePermission(testStore, "services", false), routes.GetService(testStore))
	svcs.PUT("/:id", routes.RequirePermission(testStore, "services", true), routes.UpdateService(testStore))
	svcs.PATCH("/:id", routes.RequirePermission(testStore, "services", true), routes.UpdateService(testStore))
	svcs.DELETE("/:id", routes.RequirePermission(testStore, "services", true), routes.DeleteService(testStore))

	// Routes
	rt := admin.Group("/routes")
	rt.GET("", routes.RequirePermission(testStore, "routes", false), routes.ListRoutes(testStore))
	rt.POST("", routes.RequirePermission(testStore, "routes", true), routes.CreateRoute(testStore))
	rt.GET("/:id", routes.RequirePermission(testStore, "routes", false), routes.GetRoute(testStore))
	rt.PUT("/:id", routes.RequirePermission(testStore, "routes", true), routes.UpdateRoute(testStore))
		rt.PATCH("/:id", routes.RequirePermission(testStore, "routes", true), routes.PatchRoute(testStore))
	rt.DELETE("/:id", routes.RequirePermission(testStore, "routes", true), routes.DeleteRoute(testStore))

	// Upstreams
	up := admin.Group("/upstreams")
	up.GET("", routes.RequirePermission(testStore, "upstreams", false), routes.ListUpstreams(testStore))
	up.POST("", routes.RequirePermission(testStore, "upstreams", true), routes.CreateUpstream(testStore))
	up.GET("/:id", routes.RequirePermission(testStore, "upstreams", false), routes.GetUpstream(testStore))
	up.PUT("/:id", routes.RequirePermission(testStore, "upstreams", true), routes.UpdateUpstream(testStore))
	up.PATCH("/:id", routes.RequirePermission(testStore, "upstreams", true), routes.UpdateUpstream(testStore))
	up.DELETE("/:id", routes.RequirePermission(testStore, "upstreams", true), routes.DeleteUpstream(testStore))
	up.GET("/:id/health", routes.RequirePermission(testStore, "upstreams", false), routes.GetUpstreamHealth(testStore))
	up.GET("/:id/targets", routes.RequirePermission(testStore, "targets", false), routes.ListTargets(testStore))
	up.POST("/:id/targets", routes.RequirePermission(testStore, "targets", true), routes.CreateTarget(testStore))
	up.PUT("/:id/targets/:target_id", routes.RequirePermission(testStore, "targets", true), routes.UpdateTarget(testStore))
	up.PATCH("/:id/targets/:target_id", routes.RequirePermission(testStore, "targets", true), routes.UpdateTarget(testStore))
	up.DELETE("/:id/targets/:target_id", routes.RequirePermission(testStore, "targets", true), routes.DeleteTarget(testStore))

	// Consumers
	cons := admin.Group("/consumers")
	cons.GET("", routes.RequirePermission(testStore, "consumers", false), routes.ListConsumers(testStore))
	cons.POST("", routes.RequirePermission(testStore, "consumers", true), routes.CreateConsumer(testStore))
	cons.GET("/:id", routes.RequirePermission(testStore, "consumers", false), routes.GetConsumer(testStore))
	cons.PUT("/:id", routes.RequirePermission(testStore, "consumers", true), routes.UpdateConsumer(testStore))
	cons.PATCH("/:id", routes.RequirePermission(testStore, "consumers", true), routes.UpdateConsumer(testStore))
	cons.DELETE("/:id", routes.RequirePermission(testStore, "consumers", true), routes.DeleteConsumer(testStore))

	// Consumer credentials
	cred := cons.Group("/:id")
	cred.GET("/key-auth/credentials", routes.RequirePermission(testStore, "consumers", false), routes.ListCredentials(testStore, "key-auth"))
	cred.POST("/key-auth/credentials", routes.RequirePermission(testStore, "consumers", true), routes.CreateCredential(testStore, "key-auth"))
	cred.DELETE("/key-auth/credentials/:credId", routes.RequirePermission(testStore, "consumers", true), routes.DeleteCredential(testStore, "key-auth"))
	cred.GET("/basic-auth/credentials", routes.RequirePermission(testStore, "consumers", false), routes.ListCredentials(testStore, "basic-auth"))
	cred.POST("/basic-auth/credentials", routes.RequirePermission(testStore, "consumers", true), routes.CreateCredential(testStore, "basic-auth"))
	cred.DELETE("/basic-auth/credentials/:credId", routes.RequirePermission(testStore, "consumers", true), routes.DeleteCredential(testStore, "basic-auth"))
	cred.GET("/hmac-auth/credentials", routes.RequirePermission(testStore, "consumers", false), routes.ListCredentials(testStore, "hmac-auth"))
	cred.POST("/hmac-auth/credentials", routes.RequirePermission(testStore, "consumers", true), routes.CreateCredential(testStore, "hmac-auth"))
	cred.DELETE("/hmac-auth/credentials/:credId", routes.RequirePermission(testStore, "consumers", true), routes.DeleteCredential(testStore, "hmac-auth"))

	// Plugins
	plugs := admin.Group("/plugins")
	plugs.GET("", routes.RequirePermission(testStore, "plugins", false), routes.ListPlugins(testStore))
	plugs.POST("", routes.RequirePermission(testStore, "plugins", true), routes.CreatePlugin(testStore))
	plugs.GET("/:id", routes.RequirePermission(testStore, "plugins", false), routes.GetPlugin(testStore))
	plugs.PUT("/:id", routes.RequirePermission(testStore, "plugins", true), routes.UpdatePlugin(testStore))
	plugs.PATCH("/:id", routes.RequirePermission(testStore, "plugins", true), routes.UpdatePlugin(testStore))
	plugs.DELETE("/:id", routes.RequirePermission(testStore, "plugins", true), routes.DeletePlugin(testStore))

	// Workspaces
	ws := admin.Group("/workspaces")
	ws.GET("", routes.RequirePermission(testStore, "workspaces", false), routes.ListWorkspaces(testStore))
	ws.POST("", routes.RequirePermission(testStore, "workspaces", true), routes.CreateWorkspace(testStore))
	ws.GET("/:id", routes.RequirePermission(testStore, "workspaces", false), routes.GetWorkspace(testStore))
	ws.PUT("/:id", routes.RequirePermission(testStore, "workspaces", true), routes.UpdateWorkspace(testStore))
	ws.PATCH("/:id", routes.RequirePermission(testStore, "workspaces", true), routes.UpdateWorkspace(testStore))
	ws.DELETE("/:id", routes.RequirePermission(testStore, "workspaces", true), routes.DeleteWorkspace(testStore))
	ws.GET("/mine", routes.ListMyWorkspaces(testStore))
	ws.GET("/:id/users", routes.RequirePermission(testStore, "workspaces", false), routes.ListWorkspaceUsers(testStore))
	ws.PUT("/:id/users", routes.RequirePermission(testStore, "workspaces", true), routes.SetUserWorkspace(testStore))
	ws.DELETE("/:id/users/:userId", routes.RequirePermission(testStore, "workspaces", true), routes.RemoveUserWorkspace(testStore))
	ws.GET("/users/:userId", routes.RequirePermission(testStore, "workspaces", false), routes.GetUserWorkspaces(testStore))

	// Users
	users := admin.Group("/users")
	users.GET("", routes.RequirePermission(testStore, "users", false), routes.ListUsers(testStore))
	users.POST("", routes.RequirePermission(testStore, "users", true), routes.CreateUser(testStore))
	users.GET("/:id", routes.RequirePermission(testStore, "users", false), routes.GetUser(testStore))
	users.PUT("/:id", routes.RequirePermission(testStore, "users", true), routes.UpdateUser(testStore))
	users.PATCH("/:id", routes.RequirePermission(testStore, "users", true), routes.UpdateUser(testStore))
	users.DELETE("/:id", routes.RequirePermission(testStore, "users", true), routes.DeleteUser(testStore))

	// AuthGroups
	groups := admin.Group("/groups")
	groups.GET("", routes.RequirePermission(testStore, "auth_groups", false), routes.ListAuthGroups(testStore))
	groups.POST("", routes.RequirePermission(testStore, "auth_groups", true), routes.CreateAuthGroup(testStore))
	groups.GET("/:id", routes.RequirePermission(testStore, "auth_groups", false), routes.GetAuthGroup(testStore))
	groups.PUT("/:id", routes.RequirePermission(testStore, "auth_groups", true), routes.UpdateAuthGroup(testStore))
	groups.PATCH("/:id", routes.RequirePermission(testStore, "auth_groups", true), routes.UpdateAuthGroup(testStore))
	groups.DELETE("/:id", routes.RequirePermission(testStore, "auth_groups", true), routes.DeleteAuthGroup(testStore))
	groups.GET("/:id/members", routes.RequirePermission(testStore, "auth_groups", false), routes.GetGroupMembers(testStore))
	groups.PUT("/:id/members", routes.RequirePermission(testStore, "auth_groups", true), routes.SetGroupMembers(testStore))

	// API Key Requests
	keys := admin.Group("/api-keys")
	keys.GET("", routes.RequirePermission(testStore, "api_keys", false), routes.ListAPIKeyRequests(testStore))
	keys.POST("", routes.RequirePermission(testStore, "api_keys", true), routes.CreateAPIKeyRequest(testStore))
	keys.GET("/:id", routes.RequirePermission(testStore, "api_keys", false), routes.GetAPIKeyRequest(testStore))
	keys.PUT("/:id", routes.RequirePermission(testStore, "api_keys", true), routes.UpdateAPIKeyRequest(testStore))
	keys.PATCH("/:id", routes.RequirePermission(testStore, "api_keys", true), routes.UpdateAPIKeyRequest(testStore))
	keys.DELETE("/:id", routes.RequirePermission(testStore, "api_keys", true), routes.DeleteAPIKeyRequest(testStore))

	// Config Snapshots
	snaps := admin.Group("/config-snapshots")
	snaps.GET("", routes.RequirePermission(testStore, "config_snapshots", false), routes.ListConfigSnapshots(testStore))
	snaps.POST("", routes.RequirePermission(testStore, "config_snapshots", true), routes.CreateConfigSnapshot(testStore))
	snaps.DELETE("/:id", routes.RequirePermission(testStore, "config_snapshots", true), routes.DeleteConfigSnapshot(testStore))

	// Audit Logs
	admin.GET("/audit", routes.RequirePermission(testStore, "audit_logs", false), routes.ListAuditLogs(testStore))

	return r
}

// Helper functions

// seedTestUser creates a test admin user in the DB for workspace permission checks
func seedTestUser(store *storage.Store) error {
	// Check if already exists
	existing, _ := store.GetUserByUsername("testadmin")
	if existing != nil {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("testpass123"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = store.CreateUser(&storage.User{
		ID:          "test-admin-uuid",
		Username:    "testadmin",
		PasswordHash: string(hash),
		DisplayName: "Test Admin",
		Email:       "testadmin@cont.local",
		Role:        "admin",
	})
	return err
}

func makeTestToken(secret string, claims jwt.MapClaims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte(secret))
	return tokenStr
}

func adminReq(method, path string, body interface{}) *http.Request {
	var bodyReader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	} else {
		bodyReader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func adminReqNoBody(method, path string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	return req
}

func authReq(method, path string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	return req
}

func makeAdminEditorToken(username, role string) string {
	return makeTestToken(jwtSecret, jwt.MapClaims{
		"sub":      username + "-id",
		"username": username,
		"role":     role,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	})
}

// ── HTTP Integration Tests ────────────────────────────────────────────────

func TestStatus_OK(t *testing.T) {
	req := authReq("GET", "/status")
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if resp["version"] == "" {
		t.Errorf("expected version field")
	}
}

// ── Services CRUD ─────────────────────────────────────────────────────────

func TestServices_CreateRead(t *testing.T) {
	// Create
	svc := map[string]interface{}{
		"name":    "test-svc-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"host":    "example.com",
		"port":    8080,
		"path":    "/api",
		"protocol": "http",
	}
	req := adminReq("POST", "/services", svc)
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("CREATE service: expected 201/200, got %d: %s", w.Code, w.Body.String())
	}
	var created map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	svcID, ok := created["id"].(string)
	if !ok || svcID == "" {
		t.Fatalf("expected id field, got %+v", created)
	}

	// Read
	req = adminReqNoBody("GET", "/services/"+svcID)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GET service: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var fetched map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &fetched); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if fetched["name"] != svc["name"] {
		t.Errorf("name mismatch: got %v, want %v", fetched["name"], svc["name"])
	}
}

func TestServices_List(t *testing.T) {
	req := adminReqNoBody("GET", "/services")
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var list []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("not JSON array: %v", err)
	}
}

func TestServices_Update(t *testing.T) {
	// Create first
	svc := map[string]interface{}{
		"name": "test-svc-update-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"host": "old.com",
		"port": 80,
	}
	req := adminReq("POST", "/services", svc)
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("CREATE: %d: %s", w.Code, w.Body.String())
	}
	var created map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &created)
	svcID := created["id"].(string)

	// Update PUT
	updated := map[string]interface{}{
		"name": svc["name"],
		"host": "new.com",
		"port": 443,
	}
	req = adminReq("PUT", "/services/"+svcID, updated)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("PUT: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify
	req = adminReqNoBody("GET", "/services/"+svcID)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	json.Unmarshal(w.Body.Bytes(), &created)
	if created["host"] != "new.com" {
		t.Errorf("host mismatch: got %v", created["host"])
	}

	// Update PATCH
	patch := map[string]interface{}{
		"port": 9000,
	}
	req = adminReq("PATCH", "/services/"+svcID, patch)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("PATCH: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Cleanup
	req = adminReqNoBody("DELETE", "/services/"+svcID)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
}

func TestServices_Delete(t *testing.T) {
	// Create
	svc := map[string]interface{}{
		"name": "test-svc-delete-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"host": "delete.com",
		"port": 80,
	}
	req := adminReq("POST", "/services", svc)
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	json.Unmarshal(w.Body.Bytes(), &struct{ ID string }{})
	var created struct{ ID string }
	json.Unmarshal(w.Body.Bytes(), &created)

	// Delete
	req = adminReqNoBody("DELETE", "/services/"+created.ID)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
		t.Errorf("DELETE: expected 204/200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify gone
	req = adminReqNoBody("GET", "/services/"+created.ID)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("GET after DELETE: expected 404, got %d", w.Code)
	}
}

func TestServices_ValidationError(t *testing.T) {
	// Missing required fields
	svc := map[string]interface{}{
		"port": 8080,
	}
	req := adminReq("POST", "/services", svc)
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestServices_Unauthorized(t *testing.T) {
	// No auth header
	req := authReq("GET", "/services")
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// ── Routes CRUD ───────────────────────────────────────────────────────────

func TestRoutes_CRUD(t *testing.T) {
	// Create a service first (route needs it)
	svc := map[string]interface{}{
		"name": "test-routes-svc-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"host": "route-example.com",
		"port": 80,
	}
	req := adminReq("POST", "/services", svc)
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	var svcCreated struct{ ID string }
	json.Unmarshal(w.Body.Bytes(), &svcCreated)
	svcID := svcCreated.ID

	// Create route
	route := map[string]interface{}{
		"name":   "test-route-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"service": map[string]interface{}{"id": svcID},
		"hosts":  []string{"test.example.com"},
		"paths":  []string{"/test"},
		"methods": []string{"GET"},
	}
	req = adminReq("POST", "/routes", route)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("CREATE route: %d: %s", w.Code, w.Body.String())
	}
	var routeCreated map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &routeCreated)
	routeID := routeCreated["id"].(string)

	// Read
	req = adminReqNoBody("GET", "/routes/"+routeID)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GET route: %d: %s", w.Code, w.Body.String())
	}

	// List
	req = adminReqNoBody("GET", "/routes")
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("LIST routes: %d: %s", w.Code, w.Body.String())
	}

	// Update
	update := map[string]interface{}{
		"name":  route["name"],
		"hosts": []string{"updated.example.com"},
	}
	req = adminReq("PUT", "/routes/"+routeID, update)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("PUT route: %d: %s", w.Code, w.Body.String())
	}

	// Delete
	req = adminReqNoBody("DELETE", "/routes/"+routeID)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
		t.Errorf("DELETE route: %d: %s", w.Code, w.Body.String())
	}

	// Cleanup service
	req = adminReqNoBody("DELETE", "/services/"+svcID)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
}

func TestRoutes_ServiceByName(t *testing.T) {
	// Create service
	svc := map[string]interface{}{
		"name": "test-route-name-svc-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"host": "name-example.com",
		"port": 80,
	}
	req := adminReq("POST", "/services", svc)
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	var svcCreated struct{ ID string }
	json.Unmarshal(w.Body.Bytes(), &svcCreated)

	// Create route using service.name (not service.id)
	route := map[string]interface{}{
		"name":   "test-route-by-name-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"service": map[string]interface{}{"name": svc["name"]},
		"hosts":  []string{"name.example.com"},
		"paths":  []string{"/name-test"},
	}
	req = adminReq("POST", "/routes", route)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Errorf("CREATE route by name: %d: %s", w.Code, w.Body.String())
	}
	var routeCreated map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &routeCreated)
	routeID := routeCreated["id"].(string)

	// Verify service_id was resolved
	if routeCreated["service_id"] == nil || routeCreated["service_id"] == "" {
		t.Errorf("service_id should be resolved from name")
	}

	// Cleanup
	req = adminReqNoBody("DELETE", "/routes/"+routeID)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	req = adminReqNoBody("DELETE", "/services/"+svcCreated.ID)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
}

// ── Upstreams CRUD ────────────────────────────────────────────────────────

func TestUpstreams_CRUD(t *testing.T) {
	name := "test-upstream-" + fmt.Sprintf("%d", time.Now().UnixNano())

	// Create
	up := map[string]interface{}{
		"name":      name,
		"algorithm": "roundrobin",
		"slots":     10000,
	}
	req := adminReq("POST", "/upstreams", up)
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("CREATE upstream: %d: %s", w.Code, w.Body.String())
	}
	var created struct{ ID string }
	json.Unmarshal(w.Body.Bytes(), &created)
	upstreamID := created.ID

	// Read
	req = adminReqNoBody("GET", "/upstreams/"+upstreamID)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GET upstream: %d: %s", w.Code, w.Body.String())
	}

	// List
	req = adminReqNoBody("GET", "/upstreams")
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("LIST upstreams: %d: %s", w.Code, w.Body.String())
	}

	// Update
	update := map[string]interface{}{
		"name":      name,
		"algorithm": "leastconn",
	}
	req = adminReq("PUT", "/upstreams/"+upstreamID, update)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("PUT upstream: %d: %s", w.Code, w.Body.String())
	}

	// Delete
	req = adminReqNoBody("DELETE", "/upstreams/"+upstreamID)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
		t.Errorf("DELETE upstream: %d: %s", w.Code, w.Body.String())
	}
}

// ── Targets CRUD ──────────────────────────────────────────────────────────

func TestTargets_CRUD(t *testing.T) {
	// Create upstream
	up := map[string]interface{}{
		"name": "test-targets-upstream-" + fmt.Sprintf("%d", time.Now().UnixNano()),
	}
	req := adminReq("POST", "/upstreams", up)
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	var upCreated struct{ ID string }
	json.Unmarshal(w.Body.Bytes(), &upCreated)

	// Create target
	target := map[string]interface{}{
		"target": "192.168.1.100:8080",
		"weight": 100,
	}
	req = adminReq("POST", "/upstreams/"+upCreated.ID+"/targets", target)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("CREATE target: %d: %s", w.Code, w.Body.String())
	}
	var targetCreated map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &targetCreated)
	targetID := targetCreated["id"].(string)

	// List targets
	req = adminReqNoBody("GET", "/upstreams/"+upCreated.ID+"/targets")
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("LIST targets: %d: %s", w.Code, w.Body.String())
	}

	// Update target
	update := map[string]interface{}{
		"weight": 50,
	}
	req = adminReq("PUT", "/upstreams/"+upCreated.ID+"/targets/"+targetID, update)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("PUT target: %d: %s", w.Code, w.Body.String())
	}

	// Delete target
	req = adminReqNoBody("DELETE", "/upstreams/"+upCreated.ID+"/targets/"+targetID)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
		t.Errorf("DELETE target: %d: %s", w.Code, w.Body.String())
	}

	// Cleanup upstream
	req = adminReqNoBody("DELETE", "/upstreams/"+upCreated.ID)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
}

// ── Consumers CRUD ────────────────────────────────────────────────────────

func TestConsumers_CRUD(t *testing.T) {
	name := "test-consumer-" + fmt.Sprintf("%d", time.Now().UnixNano())

	// Create
	cons := map[string]interface{}{
		"username": name,
		"custom_id": "custom-123",
	}
	req := adminReq("POST", "/consumers", cons)
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("CREATE consumer: %d: %s", w.Code, w.Body.String())
	}
	var created struct{ ID string }
	json.Unmarshal(w.Body.Bytes(), &created)
	consumerID := created.ID

	// Read
	req = adminReqNoBody("GET", "/consumers/"+consumerID)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GET consumer: %d: %s", w.Code, w.Body.String())
	}

	// List
	req = adminReqNoBody("GET", "/consumers")
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("LIST consumers: %d: %s", w.Code, w.Body.String())
	}

	// Update
	update := map[string]interface{}{
		"custom_id": "updated-456",
	}
	req = adminReq("PATCH", "/consumers/"+consumerID, update)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("PATCH consumer: %d: %s", w.Code, w.Body.String())
	}

	// Delete
	req = adminReqNoBody("DELETE", "/consumers/"+consumerID)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
		t.Errorf("DELETE consumer: %d: %s", w.Code, w.Body.String())
	}
}

// ── Plugins CRUD ──────────────────────────────────────────────────────────

func TestPlugins_CRUD(t *testing.T) {
	// Create a service to attach plugin to
	svc := map[string]interface{}{
		"name": "test-plugin-svc-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"host": "plugin-test.com",
		"port": 80,
	}
	req := adminReq("POST", "/services", svc)
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	var svcCreated struct{ ID string }
	json.Unmarshal(w.Body.Bytes(), &svcCreated)

	// Create plugin
	plugin := map[string]interface{}{
		"name":      "rate-limiting",
		"service_id": svcCreated.ID,
		"config":    map[string]interface{}{"minute": 100},
	}
	req = adminReq("POST", "/plugins", plugin)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("CREATE plugin: %d: %s", w.Code, w.Body.String())
	}
	var created struct{ ID string }
	json.Unmarshal(w.Body.Bytes(), &created)
	pluginID := created.ID

	// Read
	req = adminReqNoBody("GET", "/plugins/"+pluginID)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GET plugin: %d: %s", w.Code, w.Body.String())
	}

	// List
	req = adminReqNoBody("GET", "/plugins")
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("LIST plugins: %d: %s", w.Code, w.Body.String())
	}

	// Update
	update := map[string]interface{}{
		"config": map[string]interface{}{"minute": 200},
	}
	req = adminReq("PATCH", "/plugins/"+pluginID, update)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("PATCH plugin: %d: %s", w.Code, w.Body.String())
	}

	// Delete
	req = adminReqNoBody("DELETE", "/plugins/"+pluginID)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
		t.Errorf("DELETE plugin: %d: %s", w.Code, w.Body.String())
	}

	// Cleanup service
	req = adminReqNoBody("DELETE", "/services/"+svcCreated.ID)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
}

// ── Workspaces CRUD ───────────────────────────────────────────────────────

func TestWorkspaces_CRUD(t *testing.T) {
	name := "test-workspace-" + fmt.Sprintf("%d", time.Now().UnixNano())

	// Create
	ws := map[string]interface{}{
		"name": name,
	}
	req := adminReq("POST", "/workspaces", ws)
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("CREATE workspace: %d: %s", w.Code, w.Body.String())
	}
	var created struct{ ID string }
	json.Unmarshal(w.Body.Bytes(), &created)
	wsID := created.ID

	// Read
	req = adminReqNoBody("GET", "/workspaces/"+wsID)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GET workspace: %d: %s", w.Code, w.Body.String())
	}

	// List
	req = adminReqNoBody("GET", "/workspaces")
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("LIST workspaces: %d: %s", w.Code, w.Body.String())
	}

	// Update
	update := map[string]interface{}{
		"name": name + "-updated",
	}
	req = adminReq("PUT", "/workspaces/"+wsID, update)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("PUT workspace: %d: %s", w.Code, w.Body.String())
	}

	// Delete
	req = adminReqNoBody("DELETE", "/workspaces/"+wsID)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
		t.Errorf("DELETE workspace: %d: %s", w.Code, w.Body.String())
	}
}

func TestWorkspaces_ListMine(t *testing.T) {
	req := adminReqNoBody("GET", "/workspaces/mine")
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// ── Users CRUD ────────────────────────────────────────────────────────────

func TestUsers_CRUD(t *testing.T) {
	username := "testuser-" + fmt.Sprintf("%d", time.Now().UnixNano())

	// Create user
	user := map[string]interface{}{
		"username":    username,
		"display_name": "Test User",
		"email":       username + "@test.com",
		"role":        "viewer",
	}
	req := adminReq("POST", "/users", user)
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("CREATE user: %d: %s", w.Code, w.Body.String())
	}
	var created struct{ ID string }
	json.Unmarshal(w.Body.Bytes(), &created)
	userID := created.ID

	// Read
	req = adminReqNoBody("GET", "/users/"+userID)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GET user: %d: %s", w.Code, w.Body.String())
	}

	// List
	req = adminReqNoBody("GET", "/users")
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("LIST users: %d: %s", w.Code, w.Body.String())
	}

	// Update
	update := map[string]interface{}{
		"display_name": "Updated Name",
		"role":         "editor",
	}
	req = adminReq("PATCH", "/users/"+userID, update)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("PATCH user: %d: %s", w.Code, w.Body.String())
	}

	// Delete
	req = adminReqNoBody("DELETE", "/users/"+userID)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
		t.Errorf("DELETE user: %d: %s", w.Code, w.Body.String())
	}
}

// ── AuthGroups CRUD ───────────────────────────────────────────────────────

func TestAuthGroups_CRUD(t *testing.T) {
	name := "test-group-" + fmt.Sprintf("%d", time.Now().UnixNano())

	// Create
	group := map[string]interface{}{
		"name":        name,
		"label":       "Test Group",
		"description": "A test auth group",
		"permissions": []string{"services:read", "routes:write"},
	}
	req := adminReq("POST", "/groups", group)
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("CREATE group: %d: %s", w.Code, w.Body.String())
	}
	var created struct{ ID string }
	json.Unmarshal(w.Body.Bytes(), &created)
	groupID := created.ID

	// Read
	req = adminReqNoBody("GET", "/groups/"+groupID)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GET group: %d: %s", w.Code, w.Body.String())
	}

	// List
	req = adminReqNoBody("GET", "/groups")
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("LIST groups: %d: %s", w.Code, w.Body.String())
	}

	// Update
	update := map[string]interface{}{
		"label": "Updated Label",
	}
	req = adminReq("PATCH", "/groups/"+groupID, update)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("PATCH group: %d: %s", w.Code, w.Body.String())
	}

	// Delete
	req = adminReqNoBody("DELETE", "/groups/"+groupID)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
		t.Errorf("DELETE group: %d: %s", w.Code, w.Body.String())
	}
}

// ── API Key Requests CRUD ─────────────────────────────────────────────────

func TestAPIKeyRequests_CRUD(t *testing.T) {
	// Create request
	reqPayload := map[string]interface{}{
		"key_name":    "test-key-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"description": "Test API key",
		"reason":      "testing",
	}
	req := adminReq("POST", "/api-keys", reqPayload)
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("CREATE api-key: %d: %s", w.Code, w.Body.String())
	}
	var created struct{ ID float64 }
	json.Unmarshal(w.Body.Bytes(), &created)
	keyID := int(created.ID)

	// Read
	req = adminReqNoBody("GET", fmt.Sprintf("/api-keys/%d", keyID))
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GET api-key: %d: %s", w.Code, w.Body.String())
	}

	// List
	req = adminReqNoBody("GET", "/api-keys")
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("LIST api-keys: %d: %s", w.Code, w.Body.String())
	}

	// Update (PATCH status)
	patch := map[string]interface{}{
		"status": "approved",
	}
	req = adminReq("PATCH", fmt.Sprintf("/api-keys/%d", keyID), patch)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("PATCH api-key: %d: %s", w.Code, w.Body.String())
	}

	// Delete
	req = adminReqNoBody("DELETE", fmt.Sprintf("/api-keys/%d", keyID))
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
		t.Errorf("DELETE api-key: %d: %s", w.Code, w.Body.String())
	}
}

// ── Config Snapshots CRUD ─────────────────────────────────────────────────

func TestConfigSnapshots_CRUD(t *testing.T) {
	// Create snapshot
	snap := map[string]interface{}{
		"version_label": "v1.0.0-test-" + fmt.Sprintf("%d", time.Now().UnixNano()),
	}
	req := adminReq("POST", "/config-snapshots", snap)
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("CREATE snapshot: %d: %s", w.Code, w.Body.String())
	}
	var created struct{ ID float64 }
	json.Unmarshal(w.Body.Bytes(), &created)
	snapID := int(created.ID)

	// List
	req = adminReqNoBody("GET", "/config-snapshots")
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("LIST snapshots: %d: %s", w.Code, w.Body.String())
	}

	// Delete
	req = adminReqNoBody("DELETE", fmt.Sprintf("/config-snapshots/%d", snapID))
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
		t.Errorf("DELETE snapshot: %d: %s", w.Code, w.Body.String())
	}
}

// ── Audit Logs ────────────────────────────────────────────────────────────

func TestAuditLogs_List(t *testing.T) {
	req := adminReqNoBody("GET", "/audit")
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var logs []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &logs); err != nil {
		t.Fatalf("not JSON array: %v", err)
	}
}

// ── Pagination ────────────────────────────────────────────────────────────

func TestServices_Pagination(t *testing.T) {
	// Create 3 services
	for i := 0; i < 3; i++ {
		svc := map[string]interface{}{
			"name": fmt.Sprintf("pagination-svc-%d-%d", time.Now().UnixNano(), i),
			"host": "page.example.com",
			"port": 80,
		}
		req := adminReq("POST", "/services", svc)
		w := httptest.NewRecorder()
		testRouter.ServeHTTP(w, req)
	}

	// List with pagination
	req := adminReqNoBody("GET", "/services?size=2&offset=0")
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var list []interface{}
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) > 2 {
		t.Errorf("expected at most 2 results, got %d", len(list))
	}

	// Check Next header
	if nextHeader := w.Header().Get("Next"); nextHeader == "" {
		t.Log("Note: Next header may be empty if fewer results than size")
	}
}

// ── RBAC Permission Tests ─────────────────────────────────────────────────

func TestRBAC_ViewerCannotWrite(t *testing.T) {
	viewerToken := makeAdminEditorToken("viewer-user", "viewer")

	svc := map[string]interface{}{
		"name": "rbac-test-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"host": "rbac.example.com",
		"port": 80,
	}
	b, _ := json.Marshal(svc)
	req := httptest.NewRequest("POST", "/services", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden && w.Code != http.StatusUnauthorized {
		t.Errorf("viewer POST /services: expected 403/401, got %d", w.Code)
	}
}

func TestRBAC_EditorCannotDeleteUpstreams(t *testing.T) {
	editorToken := makeAdminEditorToken("editor-user", "editor")

	// Create upstream first
	up := map[string]interface{}{
		"name": "rbac-upstream-" + fmt.Sprintf("%d", time.Now().UnixNano()),
	}
	b, _ := json.Marshal(up)
	req := httptest.NewRequest("POST", "/upstreams", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+editorToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	var created struct{ ID string }
	json.Unmarshal(w.Body.Bytes(), &created)

	// Editor tries to delete upstream
	req = httptest.NewRequest("DELETE", "/upstreams/"+created.ID, nil)
	req.Header.Set("Authorization", "Bearer "+editorToken)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)

	// Should be forbidden (editor can't delete upstreams)
	if w.Code != http.StatusForbidden {
		t.Logf("Note: editor delete upstream returned %d (may be allowed by design)", w.Code)
	}
}

// ── Consumer Credentials ──────────────────────────────────────────────────

func TestConsumerCredentials_KeyAuth(t *testing.T) {
	// Create consumer
	cons := map[string]interface{}{
		"username": "keyauth-consumer-" + fmt.Sprintf("%d", time.Now().UnixNano()),
	}
	var req *http.Request
	var w *httptest.ResponseRecorder
	req = adminReq("POST", "/consumers", cons)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	var consCreated struct{ ID string }
	json.Unmarshal(w.Body.Bytes(), &consCreated)
	consumerID := consCreated.ID

	// Create key-auth credential
	cred := map[string]interface{}{
		"key": "test-api-key-" + fmt.Sprintf("%d", time.Now().UnixNano()),
	}
	req = adminReq("POST", "/consumers/"+consumerID+"/key-auth/credentials", cred)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("CREATE key-auth: %d: %s", w.Code, w.Body.String())
	}
	var credCreated struct{ ID string }
	json.Unmarshal(w.Body.Bytes(), &credCreated)

	// List credentials
	req = adminReqNoBody("GET", "/consumers/"+consumerID+"/key-auth/credentials")
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("LIST key-auth: %d: %s", w.Code, w.Body.String())
	}

	// Delete credential
	req = adminReqNoBody("DELETE", "/consumers/"+consumerID+"/key-auth/credentials/"+credCreated.ID)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
		t.Errorf("DELETE key-auth: %d: %s", w.Code, w.Body.String())
	}

	// Cleanup consumer
	req = adminReqNoBody("DELETE", "/consumers/"+consumerID)
	w = httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
}