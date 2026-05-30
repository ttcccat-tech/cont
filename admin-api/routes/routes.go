package routes

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ttcccat-tech/cont/admin-api/storage"
)

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
			Database: struct{ Reachable bool }{Reachable: true},
		})
	}
}

func Metrics(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prometheus metrics compatible with Kong
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.String(http.StatusOK, `# HELP kong_nginx_requests_total Total number of requests
# TYPE kong_nginx_requests_total counter
kong_nginx_requests_total 0

# HELP kong_nginx_connections_total Number of connections
# TYPE kong_nginx_connections_total gauge
kong_nginx_connections_total{state="active"} 0
kong_nginx_connections_total{state="accepted"} 0
kong_nginx_connections_total{state="handled"} 0
kong_nginx_connections_total{state="reading"} 0
kong_nginx_connections_total{state="writing"} 0
kong_nginx_connections_total{state="waiting"} 0
`)
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
			c.JSON(400, gin.H{"message": err.Error()})
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
			c.JSON(400, gin.H{"message": err.Error()})
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
		c.NoContent(204)
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
			c.JSON(400, gin.H{"message": err.Error()})
			return
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
			c.JSON(400, gin.H{"message": err.Error()})
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
		c.NoContent(204)
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
			c.JSON(400, gin.H{"message": err.Error()})
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
			c.JSON(400, gin.H{"message": err.Error()})
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
		c.NoContent(204)
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
			c.JSON(400, gin.H{"message": err.Error()})
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
			c.JSON(400, gin.H{"message": err.Error()})
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
		c.NoContent(204)
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
			c.JSON(400, gin.H{"message": err.Error()})
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
			c.JSON(400, gin.H{"message": err.Error()})
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
		c.NoContent(204)
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
			c.JSON(400, gin.H{"message": err.Error()})
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
			c.JSON(400, gin.H{"message": err.Error()})
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
		c.NoContent(204)
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
			c.JSON(400, gin.H{"message": err.Error()})
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

// ── Helpers ───────────────────────────────────────────────────────────────

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
