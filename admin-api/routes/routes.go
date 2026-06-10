package routes

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/ttcccat-tech/cont/admin-api/storage"
	"golang.org/x/crypto/bcrypt"
)

// AuthRequired returns a gin middleware that validates JWT tokens
func AuthRequired(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			authHeader = c.GetHeader("Kong-Admin-Token")
		}
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(jwtSecret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
			return
		}
		c.Set("user_id", claims["sub"])
		c.Set("username", claims["username"])
		c.Set("role", claims["role"])
		c.Next()
	}
}

// ── Helpers ────────────────────────────────────────────────────────────────

func paginate(c *gin.Context) (int, int) {
	size := 100
	offset := 0
	if s := c.Query("size"); s != "" {
		if ps, err := parseInt(s); err == nil && ps > 0 && ps <= 1000 {
			size = ps
		}
	}
	if o := c.Query("offset"); o != "" {
		if po, err := parseInt(o); err == nil && po >= 0 {
			offset = po
		}
	}
	return size, offset
}

func parseInt(s string) (int, error) {
	var v int
	_, err := parseIntFmt(s, &v)
	return v, err
}

func parseIntFmt(s string, v *int) (int, error) {
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, nil
		}
		n = n*10 + int(ch-'0')
	}
	*v = n
	return n, nil
}

func nextList(c *gin.Context, count int, size, offset int) {
	if offset+size < count {
		c.Header("Next", makeCursor(offset+size))
	}
}

// badRequest sends a 400 with a structured validation error message
func badRequest(c *gin.Context, err error) {
	if err == nil {
		c.JSON(400, gin.H{"message": "invalid request body"})
		return
	}
	msg := err.Error()
	if strings.HasPrefix(msg, "Key:") {
		var fields []string
		errStr := msg
		for {
			idx := strings.Index(errStr, "Error:FieldLevel")
			if idx == -1 {
				break
			}
			rest := errStr[idx+len("Error:FieldLevel"):]
			spaceIdx := strings.Index(rest, " ")
			if spaceIdx > 0 {
				field := rest[:spaceIdx]
				if field != "" {
					fields = append(fields, field)
				}
			}
			if len(rest) > 50 {
				errStr = rest[50:]
			} else {
				break
			}
		}
		if len(fields) > 0 {
			c.JSON(400, gin.H{"message": "validation failed", "errors": fields})
			return
		}
	}
	c.JSON(400, gin.H{"message": msg})
}

func makeCursor(offset int) string {
	return "?offset=" + iToS(offset)
}

func iToS(v int) string {
	if v == 0 {
		return "0"
	}
	b := make([]byte, 0, 10)
	for v > 0 {
		b = append(b, byte('0'+v%10))
		v /= 10
	}
	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return string(b)
}

// ── Status ────────────────────────────────────────────────────────────────

func Status(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, storage.StatusResponse{
			Version: "cont 0.1.0",
			Uptime:  int64(storage.StartTime.Second()),
			Database: struct {
				Reachable bool `json:"reachable"`
			}{Reachable: true},
		})
	}
}

func Metrics() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// ── Services ──────────────────────────────────────────────────────────────

func ListServices(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		size, offset := paginate(c)
		rows, err := store.ListServices(size, offset)
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"data": rows, "next": ""})
	}
}

func CreateService(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var s storage.Service
		if err := c.ShouldBindJSON(&s); err != nil {
badRequest(c, err)
			return
		}
		result, err := store.CreateService(&s)
		if err != nil {
			if isUniqueViolation(err) {
				c.JSON(409, gin.H{"message": "service already exists"})
				return
			}
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(201, result)
	}
}

func GetService(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		s, err := store.GetService(c.Param("id"))
		if err == sql.ErrNoRows {
			c.JSON(404, gin.H{"message": "service not found"})
			return
		}
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, s)
	}
}

func UpdateService(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var s storage.Service
		if err := c.ShouldBindJSON(&s); err != nil {
badRequest(c, err)
			return
		}
		result, err := store.UpdateService(c.Param("id"), &s)
		if err == sql.ErrNoRows {
			c.JSON(404, gin.H{"message": "service not found"})
			return
		}
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, result)
	}
}

func DeleteService(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := store.DeleteService(c.Param("id")); err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// ── Routes ────────────────────────────────────────────────────────────────

func ListRoutes(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		size, offset := paginate(c)
		rows, err := store.ListRoutes(size, offset)
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"data": rows, "next": ""})
	}
}

func CreateRoute(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var r storage.Route
		if err := c.ShouldBindJSON(&r); err != nil {
badRequest(c, err)
			return
		}
		// Resolve service.name → service.id if service_id is empty
		if r.Service != nil && r.Service.ID == "" && r.GetServiceName() != "" {
			svc, err := store.GetServiceByName(r.GetServiceName())
			if err != nil {
				c.JSON(400, gin.H{"message": "service not found: " + r.GetServiceName()})
				return
			}
			if svc == nil {
				c.JSON(400, gin.H{"message": "service not found: " + r.GetServiceName()})
				return
			}
			r.Service.ID = svc.ID
		}
		result, err := store.CreateRoute(&r)
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(201, result)
	}
}

func GetRoute(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		r, err := store.GetRoute(c.Param("id"))
		if err == sql.ErrNoRows {
			c.JSON(404, gin.H{"message": "route not found"})
			return
		}
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, r)
	}
}

func UpdateRoute(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var r storage.Route
		if err := c.ShouldBindJSON(&r); err != nil {
badRequest(c, err)
			return
		}
		result, err := store.UpdateRoute(c.Param("id"), &r)
		if err == sql.ErrNoRows {
			c.JSON(404, gin.H{"message": "route not found"})
			return
		}
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, result)
	}
}

func DeleteRoute(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := store.DeleteRoute(c.Param("id")); err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// ── Upstreams ─────────────────────────────────────────────────────────────

func ListUpstreams(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		size, offset := paginate(c)
		rows, err := store.ListUpstreams(size, offset)
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"data": rows, "next": ""})
	}
}

func CreateUpstream(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var u storage.Upstream
		if err := c.ShouldBindJSON(&u); err != nil {
badRequest(c, err)
			return
		}
		result, err := store.CreateUpstream(&u)
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(201, result)
	}
}

func GetUpstream(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, err := store.GetUpstream(c.Param("id"))
		if err == sql.ErrNoRows {
			c.JSON(404, gin.H{"message": "upstream not found"})
			return
		}
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, u)
	}
}

func UpdateUpstream(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var u storage.Upstream
		if err := c.ShouldBindJSON(&u); err != nil {
badRequest(c, err)
			return
		}
		result, err := store.UpdateUpstream(c.Param("id"), &u)
		if err == sql.ErrNoRows {
			c.JSON(404, gin.H{"message": "upstream not found"})
			return
		}
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, result)
	}
}

func DeleteUpstream(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := store.DeleteUpstream(c.Param("id")); err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// ── Targets ───────────────────────────────────────────────────────────────

func ListTargets(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := store.ListTargetsByUpstream(c.Param("id"))
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"data": rows, "next": ""})
	}
}

func CreateTarget(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var t storage.Target
		if err := c.ShouldBindJSON(&t); err != nil {
badRequest(c, err)
			return
		}
		t.UpstreamID = c.Param("id")
		result, err := store.CreateTarget(&t)
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(201, result)
	}
}

func UpdateTarget(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var t storage.Target
		if err := c.ShouldBindJSON(&t); err != nil {
badRequest(c, err)
			return
		}
		result, err := store.UpdateTarget(c.Param("id"), c.Param("target_id"), &t)
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, result)
	}
}

func DeleteTarget(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := store.DeleteTarget(c.Param("id"), c.Param("target_id")); err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// ── Consumers ─────────────────────────────────────────────────────────────

func ListConsumers(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		size, offset := paginate(c)
		rows, err := store.ListConsumers(size, offset)
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"data": rows, "next": ""})
	}
}

func CreateConsumer(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var con storage.Consumer
		if err := c.ShouldBindJSON(&con); err != nil {
badRequest(c, err)
			return
		}
		result, err := store.CreateConsumer(&con)
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(201, result)
	}
}

func GetConsumer(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		con, err := store.GetConsumer(c.Param("id"))
		if err == sql.ErrNoRows {
			c.JSON(404, gin.H{"message": "consumer not found"})
			return
		}
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, con)
	}
}

func UpdateConsumer(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var con storage.Consumer
		if err := c.ShouldBindJSON(&con); err != nil {
badRequest(c, err)
			return
		}
		result, err := store.UpdateConsumer(c.Param("id"), &con)
		if err == sql.ErrNoRows {
			c.JSON(404, gin.H{"message": "consumer not found"})
			return
		}
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, result)
	}
}

func DeleteConsumer(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := store.DeleteConsumer(c.Param("id")); err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// ── Plugins ────────────────────────────────────────────────────────────────

func ListPlugins(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		size, offset := paginate(c)
		rows, err := store.ListPlugins(size, offset)
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"data": rows, "next": ""})
	}
}

func CreatePlugin(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var p storage.Plugin
		if err := c.ShouldBindJSON(&p); err != nil {
badRequest(c, err)
			return
		}
		result, err := store.CreatePlugin(&p)
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(201, result)
	}
}

func GetPlugin(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, err := store.GetPlugin(c.Param("id"))
		if err == sql.ErrNoRows {
			c.JSON(404, gin.H{"message": "plugin not found"})
			return
		}
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, p)
	}
}

func UpdatePlugin(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var p storage.Plugin
		if err := c.ShouldBindJSON(&p); err != nil {
badRequest(c, err)
			return
		}
		result, err := store.UpdatePlugin(c.Param("id"), &p)
		if err == sql.ErrNoRows {
			c.JSON(404, gin.H{"message": "plugin not found"})
			return
		}
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, result)
	}
}

func DeletePlugin(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := store.DeletePlugin(c.Param("id")); err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// ── Workspaces ─────────────────────────────────────────────────────────────

func ListWorkspaces(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := store.ListWorkspaces()
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"data": rows, "next": ""})
	}
}

func CreateWorkspace(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var w storage.Workspace
		if err := c.ShouldBindJSON(&w); err != nil {
badRequest(c, err)
			return
		}
		result, err := store.CreateWorkspace(&w)
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(201, result)
	}
}

func GetWorkspace(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		w, err := store.GetWorkspace(c.Param("id"))
		if err == sql.ErrNoRows {
			c.JSON(404, gin.H{"message": "workspace not found"})
			return
		}
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, w)
	}
}

func UpdateWorkspace(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var w storage.Workspace
		if err := c.ShouldBindJSON(&w); err != nil {
badRequest(c, err)
			return
		}
		result, err := store.UpdateWorkspace(c.Param("id"), &w)
		if err == sql.ErrNoRows {
			c.JSON(404, gin.H{"message": "workspace not found"})
			return
		}
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, result)
	}
}

func DeleteWorkspace(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := store.DeleteWorkspace(c.Param("id")); err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// ── Auth ─────────────────────────────────────────────────────────────────────

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token       string         `json:"token"`
	User        UserInfo       `json:"user"`
	Permissions map[string]any `json:"permissions"`
}

type UserInfo struct {
	ID          string           `json:"id"`
	Username    string           `json:"username"`
	DisplayName string           `json:"display_name"`
	Email       string           `json:"email"`
	Groups      []map[string]any `json:"groups"`
	CreatedAt   string           `json:"created_at"`
}

// Demo users: admin/admin123 (full), user/user123 (limited)
var demoUsers = map[string]struct {
	Password    string
	DisplayName string
	Email       string
	Groups      []map[string]any
	Level       int
}{
	"admin": {
		Password:    "admin123",
		DisplayName: "Administrator",
		Email:       "admin@cont.local",
		Groups:      []map[string]any{{"name": "admin", "label": "Administrators"}},
		Level:       3,
	},
	"user": {
		Password:    "user123",
		DisplayName: "Regular User",
		Email:       "user@cont.local",
		Groups:      []map[string]any{{"name": "users", "label": "Users"}},
		Level:       1,
	},
}

func Login(store *storage.Store, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "invalid request"})
			return
		}
		user, err := store.GetUserByUsername(req.Username)
		if err != nil || user == nil {
			c.JSON(401, gin.H{"error": "invalid credentials"})
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
			c.JSON(401, gin.H{"error": "invalid credentials"})
			return
		}
		// Generate JWT
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub":      user.ID,
			"username": user.Username,
			"role":     user.Role,
			"exp":      time.Now().Add(24 * time.Hour).Unix(),
			"iat":      time.Now().Unix(),
		})
		tokenStr, err := token.SignedString([]byte(jwtSecret))
		if err != nil {
			c.JSON(500, gin.H{"error": "failed to generate token"})
			return
		}
		c.JSON(200, LoginResponse{
			Token: tokenStr,
			User: UserInfo{
				ID:          user.ID,
				Username:    user.Username,
				DisplayName: user.DisplayName,
				Email:       user.Email,
				Groups:      []map[string]any{{"name": user.Role, "label": strings.Title(user.Role)}},
				CreatedAt:   user.CreatedAt,
			},
			Permissions: buildPermissions(user.Role),
		})
	}
}

// GetMe returns the current authenticated user's info and permissions
func GetMe(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		username, _ := c.Get("username")
		role, _ := c.Get("role")
		roleStr := role.(string)
		c.JSON(200, gin.H{
			"id":           userID,
			"username":     username,
			"role":         role,
			"permissions":  buildPermissions(roleStr),
		})
	}
}

func levelFromRole(role string) int {
	switch role {
	case "admin":
		return 3
	case "editor":
		return 2
	default:
		return 1
	}
}

// buildPermissions returns a permissions map that accurately reflects what the
// user can actually do with each entity, based on the real RBAC rules (CanWrite/CanRead).
// This is what we return to the frontend so it can show/hide buttons correctly.
func buildPermissions(role string) map[string]any {
	entities := []string{"services", "routes", "plugins", "consumers", "upstreams", "targets", "workspaces", "users"}
	perms := make(map[string]any)
	for _, e := range entities {
		canR := storage.CanRead(role, e)
		canW := storage.CanWrite(role, e)
		canD := storage.CanDelete(role, e)

		lvl := 0
		if canR {
			lvl = 1
		}
		if canW {
			lvl = 2
		}
		if canD {
			lvl = 3
		}

		mode := "r"
		if canW && canD {
			mode = "rwd"
		} else if canW {
			mode = "rw"
		} else if canR {
			mode = "r"
		}

		perms[e] = map[string]any{"mode": mode, "level": lvl}
	}
	return perms
}

func SSOMock(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, LoginResponse{
			Token: "cont-sso-mock-token-admin",
			User: UserInfo{
				ID:          "sso-admin-001",
				Username:    "sso_admin",
				DisplayName: "SSO Admin",
				Email:       "sso@cont.local",
				Groups:      []map[string]any{{"name": "admin", "label": "Administrators"}},
				CreatedAt:   "2026-01-01T00:00:00Z",
			},
			Permissions: map[string]any{
				"services":  map[string]any{"mode": "rw", "level": 3},
				"routes":    map[string]any{"mode": "rw", "level": 3},
				"plugins":   map[string]any{"mode": "rw", "level": 3},
				"consumers": map[string]any{"mode": "rw", "level": 3},
				"upstreams": map[string]any{"mode": "rw", "level": 3},
				"workspace": map[string]any{"mode": "rw", "level": 3},
			},
		})
	}
}

// RequireRole returns a gin middleware that checks if user has required role
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("role")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "role not found in token"})
			return
		}
		roleStr, ok := userRole.(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid role format"})
			return
		}
		for _, r := range roles {
			if roleStr == r {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
	}
}

// RequirePermission returns a gin middleware that checks if user has permission for an entity
func RequirePermission(entity string, write bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("role")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "role not found in token"})
			return
		}
		roleStr, ok := userRole.(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid role format"})
			return
		}
		if write {
			if !storage.CanWrite(roleStr, entity) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "write permission denied for " + entity})
				return
			}
		} else {
			if !storage.CanRead(roleStr, entity) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "read permission denied for " + entity})
				return
			}
		}
		c.Next()
	}
}

// ── Roles ─────────────────────────────────────────────────────────────────────

type RoleInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

func ListRoles() gin.HandlerFunc {
	return func(c *gin.Context) {
		roles := []RoleInfo{
			{
				Name:        "admin",
				Description: "Full CRUD access to all entities",
				Permissions: []string{"services:rw", "routes:rw", "plugins:rw", "consumers:rw", "upstreams:rw", "targets:rw", "workspaces:rw", "users:rw"},
			},
			{
				Name:        "editor",
				Description: "CRUD services/routes/consumers, read plugins/upstreams",
				Permissions: []string{"services:rw", "routes:rw", "plugins:r", "consumers:rw", "upstreams:r", "targets:rw", "workspaces:r", "users:r"},
			},
			{
				Name:        "viewer",
				Description: "Read-only access to all entities",
				Permissions: []string{"services:r", "routes:r", "plugins:r", "consumers:r", "upstreams:r", "targets:r", "workspaces:r", "users:r"},
			},
		}
		c.JSON(200, gin.H{"data": roles})
	}
}

func GetRolePermissions() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.Param("role")
		p := storage.GetPermissions(role)
		c.JSON(200, gin.H{"role": role, "permissions": p})
	}
}

// ── Helpers ────────────────────────────────────────────────────────────────

func isUniqueViolation(err error) bool {
	return err != nil && (contains(err.Error(), "unique") || contains(err.Error(), "23505"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ── Users CRUD ─────────────────────────────────────────────────────────

func ListUsers(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		users, err := store.ListUsers()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, users)
	}
}

func GetUser(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		user, err := store.GetUser(id)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if user == nil {
			c.JSON(404, gin.H{"error": "user not found"})
			return
		}
		c.JSON(200, user)
	}
}

func CreateUser(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Username    string `json:"username" binding:"required"`
			Password    string `json:"password"`
			DisplayName string `json:"display_name"`
			Email       string `json:"email"`
			Role        string `json:"role" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
badRequest(c, err)
			return
		}
		password := req.Password
		if password == "" {
			password = "ChangeMe123"
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(500, gin.H{"error": "failed to hash password"})
			return
		}
		user, err := store.CreateUser(&storage.User{
			Username:    req.Username,
			PasswordHash: string(hash),
			DisplayName: req.DisplayName,
			Email:       req.Email,
			Role:        req.Role,
			Enabled:     true,
		})
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(201, user)
	}
}

func UpdateUser(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			DisplayName string `json:"display_name"`
			Email       string `json:"email"`
			Role        string `json:"role"`
			Enabled     *bool  `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "invalid request"})
			return
		}
		existing, _ := store.GetUser(id)
		if existing == nil {
			c.JSON(404, gin.H{"error": "user not found"})
			return
		}
		if req.DisplayName != "" {
			existing.DisplayName = req.DisplayName
		}
		if req.Email != "" {
			existing.Email = req.Email
		}
		if req.Role != "" {
			existing.Role = req.Role
		}
		if req.Enabled != nil {
			existing.Enabled = *req.Enabled
		}
		if err := store.UpdateUser(id, existing); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		updated, _ := store.GetUser(id)
		c.JSON(200, updated)
	}
}

func DeleteUser(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		existing, _ := store.GetUser(id)
		if existing == nil {
			c.JSON(404, gin.H{"error": "user not found"})
			return
		}
		if err := store.DeleteUser(id); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(204, nil)
	}
}

// ── Auth Groups ─────────────────────────────────────────────────────────────

func ListAuthGroups(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		groups, err := store.ListAuthGroups()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if groups == nil {
			groups = []storage.AuthGroup{}
		}
		c.JSON(200, groups)
	}
}

func CreateAuthGroup(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var g storage.AuthGroup
		if err := c.ShouldBindJSON(&g); err != nil {
			badRequest(c, err)
			return
		}
		created, err := store.CreateAuthGroup(&g)
		if err != nil {
			if isUniqueViolation(err) {
				c.JSON(409, gin.H{"message": "group name already exists"})
				return
			}
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(201, created)
	}
}

func GetAuthGroup(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		g, err := store.GetAuthGroup(id)
		if err != nil {
			c.JSON(404, gin.H{"message": "group not found"})
			return
		}
		c.JSON(200, g)
	}
}

func UpdateAuthGroup(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var g storage.AuthGroup
		if err := c.ShouldBindJSON(&g); err != nil {
			badRequest(c, err)
			return
		}
		updated, err := store.UpdateAuthGroup(id, &g)
		if err != nil {
			if isUniqueViolation(err) {
				c.JSON(409, gin.H{"message": "group name already exists"})
				return
			}
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, updated)
	}
}

func DeleteAuthGroup(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		existing, _ := store.GetAuthGroup(id)
		if existing == nil {
			c.JSON(404, gin.H{"message": "group not found"})
			return
		}
		if err := store.DeleteAuthGroup(id); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(204, nil)
	}
}

// ── Resources ───────────────────────────────────────────────────────────────

func ListResources(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		resources, err := store.ListResources()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if resources == nil {
			resources = []storage.Resource{}
		}
		c.JSON(200, gin.H{"resources": resources})
	}
}

// ── Audit Logs ──────────────────────────────────────────────────────────────

func ListAuditLogs(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, size := paginate(c)
		_, offset := paginate(c)
		logs, err := store.ListAuditLogs(size, offset)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if logs == nil {
			logs = []storage.AuditLog{}
		}
		c.JSON(200, logs)
	}
}

// ── Alert Rules ─────────────────────────────────────────────────────────────

func ListAlertRules(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		rules, err := store.ListAlertRules()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if rules == nil {
			rules = []storage.AlertRule{}
		}
		c.JSON(200, rules)
	}
}

func CreateAlertRule(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var r storage.AlertRule
		if err := c.ShouldBindJSON(&r); err != nil {
			badRequest(c, err)
			return
		}
		created, err := store.CreateAlertRule(&r)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(201, created)
	}
}

func GetAlertRule(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		r, err := store.GetAlertRule(id)
		if err != nil {
			c.JSON(404, gin.H{"message": "alert rule not found"})
			return
		}
		c.JSON(200, r)
	}
}

func UpdateAlertRule(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var r storage.AlertRule
		if err := c.ShouldBindJSON(&r); err != nil {
			badRequest(c, err)
			return
		}
		updated, err := store.UpdateAlertRule(id, &r)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, updated)
	}
}

func DeleteAlertRule(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		existing, _ := store.GetAlertRule(id)
		if existing == nil {
			c.JSON(404, gin.H{"message": "alert rule not found"})
			return
		}
		if err := store.DeleteAlertRule(id); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(204, nil)
	}
}

// ── API Key Requests ────────────────────────────────────────────────────────

func ListAPIKeyRequests(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		reqs, err := store.ListAPIKeyRequests()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if reqs == nil {
			reqs = []storage.APIKeyRequest{}
		}
		c.JSON(200, reqs)
	}
}

func CreateAPIKeyRequest(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var r storage.APIKeyRequest
		if err := c.ShouldBindJSON(&r); err != nil {
			badRequest(c, err)
			return
		}
		if r.Status == "" {
			r.Status = "pending"
		}
		created, err := store.CreateAPIKeyRequest(&r)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(201, created)
	}
}

func GetAPIKeyRequest(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		r, err := store.GetAPIKeyRequest(id)
		if err != nil {
			c.JSON(404, gin.H{"message": "API key request not found"})
			return
		}
		c.JSON(200, r)
	}
}

func UpdateAPIKeyRequest(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var r storage.APIKeyRequest
		if err := c.ShouldBindJSON(&r); err != nil {
			badRequest(c, err)
			return
		}
		updated, err := store.UpdateAPIKeyRequest(id, &r)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, updated)
	}
}

func DeleteAPIKeyRequest(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		existing, _ := store.GetAPIKeyRequest(id)
		if existing == nil {
			c.JSON(404, gin.H{"message": "API key request not found"})
			return
		}
		if err := store.DeleteAPIKeyRequest(id); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(204, nil)
	}
}

// ── Config Snapshots ────────────────────────────────────────────────────────

func ListConfigSnapshots(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		snaps, err := store.ListConfigSnapshots()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if snaps == nil {
			snaps = []storage.ConfigSnapshot{}
		}
		c.JSON(200, snaps)
	}
}

func CreateConfigSnapshot(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var sn storage.ConfigSnapshot
		if err := c.ShouldBindJSON(&sn); err != nil {
			badRequest(c, err)
			return
		}
		created, err := store.CreateConfigSnapshot(&sn)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(201, created)
	}
}

func DeleteConfigSnapshot(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		existing, _ := store.GetConfigSnapshot(id)
		if existing == nil {
			c.JSON(404, gin.H{"message": "snapshot not found"})
			return
		}
		if err := store.DeleteConfigSnapshot(id); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(204, nil)
	}
}

// ── Health & Config Check ───────────────────────────────────────────────────

func HealthCheck(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Simple liveness: DB and Redis ping
		healthy := true
		var errors []string
		if err := store.Ping(); err != nil {
			healthy = false
			errors = append(errors, "db: "+err.Error())
		}
		if err := store.PingRedis(); err != nil {
			healthy = false
			errors = append(errors, "redis: "+err.Error())
		}
		if healthy {
			c.JSON(200, gin.H{"status": "healthy"})
		} else {
			c.JSON(503, gin.H{"status": "unhealthy", "errors": errors})
		}
	}
}

func ConfigCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Return current config version / status
		c.JSON(200, gin.H{
			"version": "1.0.0",
			"build":   "cont-admin-api",
		})
	}
}
