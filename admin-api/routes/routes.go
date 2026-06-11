package routes

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
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
		if orgID, ok := claims["org_id"].(string); ok {
			c.Set("org_id", orgID)
		}
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

// getOrgID extracts org_id from the request context.
// Admin role bypasses org_id filtering (returns "" to see all orgs).
func getOrgID(c *gin.Context) string {
	if role, _ := c.Get("role"); role == "admin" {
		return ""
	}
	if orgID, ok := c.Get("org_id"); ok {
		if s, ok := orgID.(string); ok {
			return s
		}
	}
	return ""
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
		orgID := getOrgID(c)
		rows, err := store.ListServices(orgID, size, offset)
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
		orgID := getOrgID(c)
		if orgID != "" {
			s.OrgID = orgID
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
		orgID := getOrgID(c)
		s, err := store.GetService(c.Param("id"), orgID)
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
		orgID := getOrgID(c)
		result, err := store.UpdateService(c.Param("id"), orgID, &s)
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
		orgID := getOrgID(c)
		if err := store.DeleteService(c.Param("id"), orgID); err != nil {
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

func GetUpstreamHealth(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		upstreamID := c.Param("id")
		upstream, err := store.GetUpstream(upstreamID)
		if err == sql.ErrNoRows {
			c.JSON(404, gin.H{"message": "upstream not found"})
			return
		}
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}

		targets, err := store.ListTargetsByUpstream(upstreamID)
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}

		ctx := context.Background()
		healthStatuses, _ := store.Redis().GetTargetHealthStatuses(ctx, upstreamID)

		type TargetHealth struct {
			ID       string `json:"id"`
			Target   string `json:"target"`
			Weight   int    `json:"weight"`
			Enabled  bool   `json:"enabled"`
			Healthy  bool   `json:"healthy"`
			Port     int    `json:"port"`
			Host     string `json:"host"`
		}

		targetHealths := make([]TargetHealth, 0, len(targets))
		for _, t := range targets {
			healthy := healthStatuses[t.Target] != true
			port := 80
			host := t.Target
			if idx := strings.LastIndex(t.Target, ":"); idx > 0 {
				host = t.Target[:idx]
				if p, err := strconv.Atoi(t.Target[idx+1:]); err == nil {
					port = p
				}
			}
			targetHealths = append(targetHealths, TargetHealth{
				ID:      t.ID,
				Target:  t.Target,
				Weight:  t.Weight,
				Enabled: t.Enabled,
				Healthy: healthy,
				Port:    port,
				Host:    host,
			})
		}

		c.JSON(200, gin.H{
			"upstream_id":   upstream.ID,
			"upstream_name": upstream.Name,
			"algorithm":     upstream.Algorithm,
			"enabled":       upstream.Enabled,
			"targets":        targetHealths,
		})
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

// ── Consumer Credentials ──────────────────────────────────────────────────

func ListCredentials(store *storage.Store, credentialType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		consumerID := c.Param("id")
		// Verify consumer exists
		if _, err := store.GetConsumer(consumerID); err != nil {
			if err == sql.ErrNoRows {
				c.JSON(404, gin.H{"message": "consumer not found"})
				return
			}
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		rows, err := store.ListConsumerCredentials(consumerID, credentialType)
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		// Return API-safe responses (no secrets)
		resp := make([]storage.CredentialResponse, len(rows))
		for i, r := range rows {
			resp[i] = r.ToResponse()
		}
		c.JSON(200, gin.H{"data": resp})
	}
}

func CreateCredential(store *storage.Store, credentialType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		consumerID := c.Param("id")
		// Verify consumer exists
		if _, err := store.GetConsumer(consumerID); err != nil {
			if err == sql.ErrNoRows {
				c.JSON(404, gin.H{"message": "consumer not found"})
				return
			}
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		var req struct {
			Key    string `json:"key" binding:"required"`
			Secret string `json:"secret,omitempty"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			badRequest(c, err)
			return
		}
		if req.Key == "" {
			c.JSON(400, gin.H{"message": "key is required"})
			return
		}
		cred := &storage.ConsumerCredential{
			ConsumerID:     consumerID,
			CredentialType: credentialType,
			Key:            req.Key,
			Secret:         req.Secret,
			Enabled:        true,
		}
		result, err := store.CreateConsumerCredential(cred)
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(201, result.ToResponse())
	}
}

func DeleteCredential(store *storage.Store, credentialType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		consumerID := c.Param("id")
		credID := c.Param("credId")
		if err := store.DeleteConsumerCredential(consumerID, credentialType, credID); err != nil {
			if err == sql.ErrNoRows {
				c.JSON(404, gin.H{"message": "credential not found"})
				return
			}
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// ValidateCredential is an internal endpoint for proxy auth validation
func ValidateCredential(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		credType := c.Param("type")
		key := c.Param("key")
		if key == "" {
			c.JSON(401, gin.H{"message": "missing key"})
			return
		}
		cred, err := store.GetConsumerCredentialByKey(credType, key)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(401, gin.H{"message": "invalid credentials"})
				return
			}
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"consumer_id": cred.ConsumerID})
	}
}

// ValidateJWT is an internal endpoint for proxy JWT token validation
// Called by the Cont proxy's access.lua during jwt-auth plugin access phase
func ValidateJWT(store *storage.Store, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := c.Param("token")
		if tokenStr == "" {
			c.JSON(401, gin.H{"message": "missing token"})
			return
		}

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
			c.JSON(401, gin.H{"message": "invalid or expired token"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(401, gin.H{"message": "invalid token claims"})
			return
		}

		// Extract user_id from "sub" claim
		userID, _ := claims["sub"].(string)
		consumerID := userID // consumer_id == user_id in Cont

		c.JSON(200, gin.H{
			"consumer_id": consumerID,
			"user_id":     userID,
		})
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
		userID, _ := c.Get("user_id")
		role, _ := c.Get("role")
		wsID := c.Param("id")

		// Check if user has access to this workspace
		wsRole, err := store.GetUserWorkspaceRole(userID.(string), wsID)
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		// Global admin bypasses workspace-level access checks
		if role.(string) != "admin" && wsRole == "" {
			c.JSON(404, gin.H{"message": "workspace not found"})
			return
		}

		w, err := store.GetWorkspace(wsID)
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

// ListMyWorkspaces returns workspaces accessible to the current user
func ListMyWorkspaces(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		role, _ := c.Get("role")

		// Admin sees all workspaces
		if role.(string) == "admin" {
			rows, err := store.ListWorkspaces()
			if err != nil {
				c.JSON(500, gin.H{"message": err.Error()})
				return
			}
			c.JSON(200, gin.H{"data": rows, "next": ""})
			return
		}

		// Non-admin: only workspaces they are explicitly assigned to
		workspaces, err := store.ListUserWorkspaces(userID.(string))
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"data": workspaces, "next": ""})
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

// SetUserWorkspace assigns a user to a workspace with a specific role
func SetUserWorkspace(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			UserID string `json:"user_id" binding:"required"`
			Role   string `json:"role" binding:"required,oneof=viewer editor admin"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			badRequest(c, err)
			return
		}
		wsID := c.Param("id")
		if err := store.SetUserWorkspace(req.UserID, wsID, req.Role); err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"message": "user workspace assignment updated"})
	}
}

// RemoveUserWorkspace removes a user's access to a workspace
func RemoveUserWorkspace(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Param("userId")
		wsID := c.Param("id")
		if err := store.RemoveUserWorkspace(userID, wsID); err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// GetUserWorkspaces returns all workspace assignments for a specific user
func GetUserWorkspaces(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Param("userId")
		workspaces, err := store.ListUserWorkspaces(userID)
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"data": workspaces})
	}
}

// ListWorkspaceUsers returns all users assigned to a workspace
func ListWorkspaceUsers(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		wsID := c.Param("id")
		users, err := store.ListWorkspaceUsers(wsID)
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"data": users})
	}
}

// RequireWorkspacePermission middleware checks if user has access to a specific workspace
// entity: the resource entity being accessed (services, routes, consumers, plugins, upstreams, targets)
// write: whether this is a write operation
func RequireWorkspacePermission(store *storage.Store, entity string, write bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "user_id not found in token"})
			return
		}
		role, _ := c.Get("role")
		roleStr := role.(string)

		// Global admin bypasses workspace-level access checks
		if roleStr == "admin" {
			c.Next()
			return
		}

		// Get workspace ID from query param or route param
		workspaceID := c.Query("workspace")
		if workspaceID == "" {
			workspaceID = c.Param("workspace")
		}

		// If no workspace specified, fall back to the first assigned workspace
		if workspaceID == "" {
			workspaces, err := store.ListUserWorkspaces(userID.(string))
			if err != nil || len(workspaces) == 0 {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "no workspace access"})
				return
			}
			workspaceID = workspaces[0].WorkspaceID
		}

		wsRole, err := store.GetUserWorkspaceRole(userID.(string), workspaceID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if wsRole == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "workspace access denied"})
			return
		}

		// Map workspace role to permission level for this entity
		// viewer -> level 1 (read-only), editor -> level 2 (read+write), admin -> level 3 (full)
		wsLevel := 0
		switch wsRole {
		case "viewer":
			wsLevel = 1
		case "editor":
			wsLevel = 2
		case "admin":
			wsLevel = 3
		}

		// For write operations, require editor or admin workspace role
		if write && wsLevel < 2 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "write permission denied for " + entity})
			return
		}

		// For delete operations, require admin workspace role
		if entity == "delete" && wsLevel < 3 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "delete permission denied"})
			return
		}

		// Store workspace ID in context for downstream handlers
		c.Set("workspace_id", workspaceID)
		c.Set("workspace_role", wsRole)
		c.Next()
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

// Login rate limit constants
const (
	LoginMaxAttempts     = 5       // max failed attempts before lockout
	LoginWindowSeconds   = 60      // time window for max attempts (seconds)
	LoginLockoutSeconds  = 300     // lockout duration after max failed attempts (5 minutes)
)

func Login(store *storage.Store, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "invalid request"})
			return
		}
		// Get client IP for attempt tracking
		clientIP := c.ClientIP()

		// Check lockout (user-based)
		locked, err := store.IsLockedOut(req.Username, LoginMaxAttempts, LoginWindowSeconds)
		if err == nil && locked {
			c.JSON(429, gin.H{"error": "too many failed login attempts, account temporarily locked"})
			return
		}

		user, err := store.GetUserByUsername(req.Username)
		if err != nil || user == nil {
			// Record failed attempt even for unknown user
			store.RecordFailedLogin(req.Username, clientIP)
			c.JSON(401, gin.H{"error": "invalid credentials"})
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
			store.RecordFailedLogin(req.Username, clientIP)
			c.JSON(401, gin.H{"error": "invalid credentials"})
			return
		}

		// Successful login: clear failed attempts
		store.ClearFailedLogins(req.Username)

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

// SendOTP sends a one-time password to the given email
func SendOTP(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Email   string `json:"email" binding:"required,email"`
			Purpose string `json:"purpose" binding:"required,oneof=register reset-password"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "invalid request: " + err.Error()})
			return
		}

		// Generate 6-digit OTP
		code := ""
		for i := 0; i < 6; i++ {
			code += fmt.Sprintf("%d", (time.Now().UnixNano()/int64(i+1))%10)
		}
		// Simple 6-digit numeric code using crypto/rand
		b := make([]byte, 3)
		rand.Read(b)
		code = fmt.Sprintf("%06d", int(b[0])*256+int(b[1])*16+int(b[2]))

		// Store OTP (expires in 10 minutes)
		otp, err := store.CreateOTP(req.Email, code, req.Purpose, 10)
		if err != nil {
			c.JSON(500, gin.H{"error": "failed to create OTP"})
			return
		}

		// TODO: In production, send code via email/SMS. For now, log it.
		// In dev mode, the code is returned so we can test easily.
		log.Printf("[OTP] To %s (purpose=%s): %s", req.Email, req.Purpose, code)

		c.JSON(200, gin.H{
			"message":   "verification code sent",
			"otp_id":    otp.ID,
			"expires_in": 600, // seconds
			// NOTE: In dev mode only — remove in production
			"code": code,
		})
	}
}

// VerifyOTP verifies the OTP and creates user+organization if registration
func VerifyOTP(store *storage.Store, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Email       string `json:"email" binding:"required,email"`
			Code        string `json:"code" binding:"required,len=6"`
			Purpose     string `json:"purpose" binding:"required,oneof=register reset-password"`
			Password    string `json:"password,omitempty"`  // required for register
			Username    string `json:"username,omitempty"`  // required for register
			DisplayName string `json:"display_name,omitempty"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "invalid request: " + err.Error()})
			return
		}

		// Find and validate OTP
		otp, err := store.GetOTP(req.Email, req.Code, req.Purpose)
		if err != nil || otp == nil {
			c.JSON(400, gin.H{"error": "invalid or expired verification code"})
			return
		}

		if req.Purpose == "register" {
			// Validate registration fields
			if req.Username == "" || req.Password == "" {
				c.JSON(400, gin.H{"error": "username and password are required for registration"})
				return
			}
			if len(req.Password) < 6 {
				c.JSON(400, gin.H{"error": "password must be at least 6 characters"})
				return
			}

			// Check if username already exists
			existing, _ := store.GetUserByUsername(req.Username)
			if existing != nil {
				c.JSON(409, gin.H{"error": "username already taken"})
				return
			}

			// Create organization first
			orgName := req.Username + "-org" // default org name
			org := &storage.Organization{Name: orgName, Plan: "free"}
			org, err = store.CreateOrganization(org)
			if err != nil {
				c.JSON(500, gin.H{"error": "failed to create organization"})
				return
			}

			// Hash password
			hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
			if err != nil {
				c.JSON(500, gin.H{"error": "failed to hash password"})
				return
			}

			// Create user
			displayName := req.DisplayName
			if displayName == "" {
				displayName = req.Username
			}
			user := &storage.User{
				Username:    req.Username,
				PasswordHash: string(hash),
				DisplayName: displayName,
				Email:       req.Email,
				Role:        "admin", // First user is admin of their org
				OrgID:       org.ID,
				Enabled:     true,
			}
			user, err = store.CreateUser(user)
			if err != nil {
				c.JSON(500, gin.H{"error": "failed to create user: " + err.Error()})
				return
			}

			// Mark OTP as verified
			store.MarkOTPVerified(otp.ID)

			// Generate JWT
			token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"sub":      user.ID,
				"username": user.Username,
				"role":     user.Role,
				"exp":      time.Now().Add(24 * time.Hour).Unix(),
				"iat":      time.Now().Unix(),
			})
			tokenStr, _ := token.SignedString([]byte(jwtSecret))

			c.JSON(201, gin.H{
				"token": tokenStr,
				"user": gin.H{
					"id":           user.ID,
					"username":     user.Username,
					"display_name": user.DisplayName,
					"email":        user.Email,
					"role":         user.Role,
					"org_id":       user.OrgID,
				},
				"organization": gin.H{
					"id":   org.ID,
					"name": org.Name,
					"plan": org.Plan,
				},
			})
		} else {
			c.JSON(400, gin.H{"error": "reset-password not yet implemented"})
		}
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
	entities := []string{"services", "routes", "plugins", "consumers", "upstreams", "targets", "workspaces", "users", "groups"}
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
		userID, _ := c.Get("user_id")
		userIDStr := ""
		if userID != nil {
			userIDStr = userID.(string)
		}
		actor, _ := c.Get("username")
		actorStr := ""
		if actor != nil {
			actorStr = actor.(string)
		}
		store.CreateAuditLog(&storage.AuditLog{
			AuditType:     "create",
			TargetType:    "User",
			TargetID:      userIDStr,
			ActorUserID:   userIDStr,
			ActorUsername: actorStr,
			Description:   "Created user: " + user.Username,
		})
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
		userID, _ := c.Get("user_id")
		userIDStr := ""
		if userID != nil {
			userIDStr = userID.(string)
		}
		actor, _ := c.Get("username")
		actorStr := ""
		if actor != nil {
			actorStr = actor.(string)
		}
		store.CreateAuditLog(&storage.AuditLog{
			AuditType:     "update",
			TargetType:    "User",
			TargetID:      userIDStr,
			ActorUserID:   userIDStr,
			ActorUsername: actorStr,
			Description:   "Updated user: " + existing.Username,
		})
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
		userID, _ := c.Get("user_id")
		userIDStr := ""
		if userID != nil {
			userIDStr = userID.(string)
		}
		actor, _ := c.Get("username")
		actorStr := ""
		if actor != nil {
			actorStr = actor.(string)
		}
		store.CreateAuditLog(&storage.AuditLog{
			AuditType:     "delete",
			TargetType:    "User",
			TargetID:      userIDStr,
			ActorUserID:   userIDStr,
			ActorUsername: actorStr,
			Description:   "Deleted user: " + existing.Username,
		})
		c.JSON(204, nil)
	}
}

// ── Auth Groups ─────────────────────────────────────────────────────────────

type UpdateAuthGroupRequest struct {
	Name        string            `json:"name" binding:"omitempty,max=255"`
	Label       string            `json:"label" binding:"omitempty"`
	Description string            `json:"description,omitempty"`
	Permissions []storage.PermissionEntry `json:"permissions,omitempty"`
}

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
		userID, _ := c.Get("user_id")
		userIDStr := ""
		if userID != nil {
			userIDStr = userID.(string)
		}
		actor, _ := c.Get("username")
		actorStr := ""
		if actor != nil {
			actorStr = actor.(string)
		}
		store.CreateAuditLog(&storage.AuditLog{
			AuditType:     "create",
			TargetType:    "AuthGroup",
			TargetID:      created.ID,
			ActorUserID:   userIDStr,
			ActorUsername: actorStr,
			Description:   "Created AuthGroup: " + created.Name,
		})
		c.JSON(201, created)
	}
}

func GetAuthGroup(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		groupID := resolveGroupID(store, id)
		g, err := store.GetAuthGroup(groupID)
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
		groupID := resolveGroupID(store, id)
		var req UpdateAuthGroupRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			badRequest(c, err)
			return
		}
		// Fetch existing to merge
		existing, err := store.GetAuthGroup(groupID)
		if err != nil {
			c.JSON(404, gin.H{"message": "group not found"})
			return
		}
		// Apply partial updates
		if req.Name != "" {
			existing.Name = req.Name
		}
		if req.Label != "" {
			existing.Label = req.Label
		}
		existing.Description = req.Description
		if req.Permissions != nil {
			existing.Permissions = req.Permissions
		}
		updated, err := store.UpdateAuthGroup(groupID, existing)
		if err != nil {
			if isUniqueViolation(err) {
				c.JSON(409, gin.H{"message": "group name already exists"})
				return
			}
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		userID, _ := c.Get("user_id")
		userIDStr := ""
		if userID != nil {
			userIDStr = userID.(string)
		}
		actor, _ := c.Get("username")
		actorStr := ""
		if actor != nil {
			actorStr = actor.(string)
		}
		store.CreateAuditLog(&storage.AuditLog{
			AuditType:     "update",
			TargetType:    "AuthGroup",
			TargetID:      groupID,
			ActorUserID:   userIDStr,
			ActorUsername: actorStr,
			Description:   "Updated AuthGroup: " + existing.Name,
		})
		c.JSON(200, updated)
	}
}

func DeleteAuthGroup(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		groupID := resolveGroupID(store, id)
		existing, _ := store.GetAuthGroup(groupID)
		if existing == nil {
			c.JSON(404, gin.H{"message": "group not found"})
			return
		}
		if err := store.DeleteAuthGroup(groupID); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		userID, _ := c.Get("user_id")
		userIDStr := ""
		if userID != nil {
			userIDStr = userID.(string)
		}
		actor, _ := c.Get("username")
		actorStr := ""
		if actor != nil {
			actorStr = actor.(string)
		}
		store.CreateAuditLog(&storage.AuditLog{
			AuditType:     "delete",
			TargetType:    "AuthGroup",
			TargetID:      groupID,
			ActorUserID:   userIDStr,
			ActorUsername: actorStr,
			Description:   "Deleted AuthGroup: " + existing.Name,
		})
		c.JSON(204, nil)
	}
}

// ── Auth Group Members ───────────────────────────────────────────────────────

type GroupMembersRequest struct {
	UserIDs []string `json:"user_ids"`
}

// resolveGroupID resolves a group id that could be a UUID or a name.
// Returns the UUID on success, or the original string if not found.
func resolveGroupID(store *storage.Store, id string) string {
	// Try as UUID first
	g, err := store.GetAuthGroup(id)
	if err == nil && g != nil {
		return g.ID
	}
	// Fall back to name lookup
	g2, err := store.GetAuthGroupByName(id)
	if err == nil && g2 != nil {
		return g2.ID
	}
	return id
}

func GetGroupMembers(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		groupID := resolveGroupID(store, id)
		ids, err := store.ListGroupMembers(groupID)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		type MemberInfo struct {
			ID          string `json:"id"`
			Username    string `json:"username"`
			DisplayName string `json:"display_name"`
			Email       string `json:"email"`
			Role        string `json:"role"`
		}
		members := []MemberInfo{}
		for _, uid := range ids {
			u, err := store.GetUser(uid)
			if err == nil && u != nil {
				members = append(members, MemberInfo{
					ID:          u.ID,
					Username:    u.Username,
					DisplayName: u.DisplayName,
					Email:       u.Email,
					Role:        u.Role,
				})
			}
		}
		c.JSON(200, gin.H{"members": members})
	}
}

func SetGroupMembers(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		groupID := resolveGroupID(store, id)
		var req GroupMembersRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "invalid request"})
			return
		}
		if err := store.SetGroupMembers(groupID, req.UserIDs); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"message": "members updated"})
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
		size, offset := paginate(c)
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

type UpdateAlertRuleRequest struct {
	Name                 string  `json:"name" binding:"omitempty,max=255"`
	Description          string  `json:"description,omitempty"`
	MetricType           string  `json:"metric_type" binding:"omitempty,oneof=error_rate latency"`
	ServiceName          string  `json:"service_name"`
	ThresholdValue       float64 `json:"threshold_value"`
	Operator             string  `json:"operator" binding:"omitempty,oneof=> < >= <= =="`
	DurationSeconds      int     `json:"duration_seconds" binding:"omitempty,min=1"`
	Enabled              *bool   `json:"enabled"`
	NotificationChannels string  `json:"notification_channels,omitempty"`
	SlackWebhookURL      string  `json:"slack_webhook_url,omitempty"`
	EmailWebhookURL      string  `json:"email_webhook_url,omitempty"`
	DiscordWebhookURL    string  `json:"discord_webhook_url,omitempty"`
	AlertSuppressSeconds int     `json:"alert_suppress_seconds"`
}

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
		var req UpdateAlertRuleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			badRequest(c, err)
			return
		}
		existing, err := store.GetAlertRule(id)
		if err != nil {
			c.JSON(404, gin.H{"message": "alert rule not found"})
			return
		}
		if req.Name != "" {
			existing.Name = req.Name
		}
		existing.Description = req.Description
		if req.MetricType != "" {
			existing.MetricType = req.MetricType
		}
		existing.ServiceName = req.ServiceName
		if req.ThresholdValue != 0 {
			existing.ThresholdValue = req.ThresholdValue
		}
		if req.Operator != "" {
			existing.Operator = req.Operator
		}
		if req.DurationSeconds != 0 {
			existing.DurationSeconds = req.DurationSeconds
		}
		if req.Enabled != nil {
			existing.Enabled = *req.Enabled
		}
		existing.NotificationChannels = req.NotificationChannels
		existing.SlackWebhookURL = req.SlackWebhookURL
		existing.EmailWebhookURL = req.EmailWebhookURL
		existing.DiscordWebhookURL = req.DiscordWebhookURL
		existing.AlertSuppressSeconds = req.AlertSuppressSeconds
		updated, err := store.UpdateAlertRule(id, existing)
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

type UpdateAPIKeyReq struct {
	KeyName       string `json:"key_name" binding:"omitempty,max=255"`
	ConsumerName  string `json:"consumer_name"`
	Description   string `json:"description,omitempty"`
	Status        string `json:"status" binding:"omitempty,oneof=pending approved rejected"`
	ReviewedBy    string `json:"reviewed_by,omitempty"`
}

// ApproveAPIKey approves an API key request and returns the generated key
func ApproveAPIKey(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		reviewer, _ := c.Get("username")
		reviewerStr := ""
		if reviewer != nil {
			reviewerStr = reviewer.(string)
		}
		reviewerID, _ := c.Get("user_id")
		reviewerIDStr := ""
		if reviewerID != nil {
			reviewerIDStr = reviewerID.(string)
		}
		existing, err := store.GetAPIKeyRequest(id)
		if err != nil || existing == nil {
			c.JSON(404, gin.H{"message": "API key request not found"})
			return
		}
		if existing.Status != "pending" {
			c.JSON(400, gin.H{"message": "API key request is not pending"})
			return
		}

		// Generate a secure random API key
		generatedKey := generateSecureKey(32)

		// Get or create consumer by consumer_name
		consumerName := existing.ConsumerName
		if consumerName == "" {
			consumerName = existing.KeyName + "-consumer"
		}
		var consumerID string
		consumers, _ := store.ListConsumers(100, 0)
		for _, con := range consumers {
			if con.Username == consumerName {
				consumerID = con.ID
				break
			}
		}
		if consumerID == "" {
			// Create new consumer
			newCon := &storage.Consumer{Username: consumerName}
			createdCon, err := store.CreateConsumer(newCon)
			if err != nil {
				c.JSON(500, gin.H{"error": "failed to create consumer: " + err.Error()})
				return
			}
			consumerID = createdCon.ID
		}

		// Create key-auth credential for the consumer
		cred := &storage.ConsumerCredential{
			ConsumerID:     consumerID,
			CredentialType: "key-auth",
			Key:            generatedKey,
			Secret:         "",
			Enabled:        true,
		}
		_, err = store.CreateConsumerCredential(cred)
		if err != nil {
			c.JSON(500, gin.H{"error": "failed to create credential: " + err.Error()})
			return
		}

		// Update request status and store the generated key
		existing.Status = "approved"
		existing.ReviewedBy = reviewerStr
		existing.GeneratedKey = generatedKey
		// Store the actual key in key_value for display to user (only after approval)
		existing.KeyValue = generatedKey
		updated, err := store.UpdateAPIKeyRequest(id, existing)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		// Write audit log
		store.CreateAuditLog(&storage.AuditLog{
			AuditType:     "update",
			TargetType:    "APIKeyRequest",
			TargetID:      id,
			ActorUserID:   reviewerIDStr,
			ActorUsername: reviewerStr,
			Description:   "Approved API key request: " + existing.KeyName + " (key generated for consumer: " + consumerName + ")",
		})
		// Send notification (non-blocking)
		go SendAPIKeyApprovalNotification(store, existing, reviewerStr, "approved")
		c.JSON(200, updated)
	}
}

// RejectAPIKey rejects an API key request
func RejectAPIKey(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		reviewer, _ := c.Get("username")
		reviewerStr := ""
		if reviewer != nil {
			reviewerStr = reviewer.(string)
		}
		reviewerID, _ := c.Get("user_id")
		reviewerIDStr := ""
		if reviewerID != nil {
			reviewerIDStr = reviewerID.(string)
		}
		existing, err := store.GetAPIKeyRequest(id)
		if err != nil || existing == nil {
			c.JSON(404, gin.H{"message": "API key request not found"})
			return
		}
		existing.Status = "rejected"
		existing.ReviewedBy = reviewerStr
		updated, err := store.UpdateAPIKeyRequest(id, existing)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		// Write audit log
		store.CreateAuditLog(&storage.AuditLog{
			AuditType:    "update",
			TargetType:   "APIKeyRequest",
			TargetID:     id,
			ActorUserID:  reviewerIDStr,
			ActorUsername: reviewerStr,
			Description:  "Rejected API key request: " + existing.KeyName,
		})
		// Send notification (non-blocking)
		go SendAPIKeyApprovalNotification(store, existing, reviewerStr, "rejected")
		c.JSON(200, updated)
	}
}

// ListMyAPIKeyRequests returns API key requests for the current user
func ListMyAPIKeyRequests(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		userIDStr := ""
		if userID != nil {
			userIDStr = userID.(string)
		}
		username, _ := c.Get("username")
		usernameStr := ""
		if username != nil {
			usernameStr = username.(string)
		}
		allReqs, err := store.ListAPIKeyRequests()
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		// Filter to requests by this user
		var mine []storage.APIKeyRequest
		for _, r := range allReqs {
			if r.ApplicantUserID == userIDStr || r.ApplicantUsername == usernameStr {
				mine = append(mine, r)
			}
		}
		c.JSON(200, mine)
	}
}

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
		// Set applicant from auth context
		if uid, ok := c.Get("user_id"); ok {
			r.ApplicantUserID = uid.(string)
		}
		if uname, ok := c.Get("username"); ok {
			r.ApplicantUsername = uname.(string)
		}
		created, err := store.CreateAPIKeyRequest(&r)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		actor, _ := c.Get("username")
		actorStr := ""
		if actor != nil {
			actorStr = actor.(string)
		}
		store.CreateAuditLog(&storage.AuditLog{
			AuditType:     "create",
			TargetType:    "APIKeyRequest",
			TargetID:      strconv.FormatInt(created.ID, 10),
			ActorUsername: actorStr,
			Description:   "Created API key request: " + created.KeyName,
		})
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
		var req UpdateAPIKeyReq
		if err := c.ShouldBindJSON(&req); err != nil {
			badRequest(c, err)
			return
		}
		existing, err := store.GetAPIKeyRequest(id)
		if err != nil {
			c.JSON(404, gin.H{"message": "API key request not found"})
			return
		}
		if req.KeyName != "" {
			existing.KeyName = req.KeyName
		}
		existing.ConsumerName = req.ConsumerName
		existing.Description = req.Description
		if req.Status != "" {
			existing.Status = req.Status
		}
		if req.ReviewedBy != "" {
			existing.ReviewedBy = req.ReviewedBy
		}
		updated, err := store.UpdateAPIKeyRequest(id, existing)
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
		userID, _ := c.Get("user_id")
		userIDStr := ""
		if userID != nil {
			userIDStr = userID.(string)
		}
		actor, _ := c.Get("username")
		actorStr := ""
		if actor != nil {
			actorStr = actor.(string)
		}
		store.CreateAuditLog(&storage.AuditLog{
			AuditType:     "delete",
			TargetType:    "APIKeyRequest",
			TargetID:      id,
			ActorUserID:   userIDStr,
			ActorUsername: actorStr,
			Description:   "Deleted API key request: " + existing.KeyName,
		})
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

// ── Notification Helpers ─────────────────────────────────────────────────────

// notifyWebhook sends a POST request to a webhook URL with the given payload.
// Returns silently if the URL is empty or the request fails (non-blocking).
// generateSecureKey generates a cryptographically secure random key
func generateSecureKey(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func notifyWebhook(webhookURL string, payload map[string]interface{}) {
	if webhookURL == "" {
		return
	}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return
	}
	resp.Body.Close()
}

// SendAPIKeyApprovalNotification sends Slack/Email notifications when an API key request is approved or rejected.
func SendAPIKeyApprovalNotification(store *storage.Store, keyReq *storage.APIKeyRequest, approvedBy, status string) {
	// Fetch applicant's email from user record
	var applicantEmail string
	applicant, _ := store.GetUser(keyReq.ApplicantUserID)
	if applicant != nil {
		applicantEmail = applicant.Email
	}

	// Build Slack message
	slackText := fmt.Sprintf("🔑 API Key Request %s: *%s*\n• Key Name: %s\n• Consumer: %s\n• Status: %s\n• Reviewed by: %s",
		strings.ToUpper(status), keyReq.KeyName, keyReq.KeyName, keyReq.ConsumerName, status, approvedBy)
	if status == "approved" && keyReq.KeyValue != "" {
		slackText += fmt.Sprintf("\n• Your API Key: `%s`", keyReq.KeyValue)
	}
	if keyReq.Reason != "" {
		slackText += fmt.Sprintf("\n• Reason: %s", keyReq.Reason)
	}
	slackPayload := map[string]interface{}{"text": slackText}

	// Build email payload (generic JSON — can be consumed by any email automation service)
	emailBody := fmt.Sprintf("Your API key request '%s' for consumer '%s' has been %s by %s.",
		keyReq.KeyName, keyReq.ConsumerName, status, approvedBy)
	if status == "approved" && keyReq.KeyValue != "" {
		emailBody += fmt.Sprintf("\n\nYour API Key:\n%s\n\nPlease keep this key secure and do not share it.", keyReq.KeyValue)
	}
	if keyReq.Reason != "" {
		emailBody += fmt.Sprintf("\n\nRequested Reason: %s", keyReq.Reason)
	}
	emailPayload := map[string]interface{}{
		"subject": fmt.Sprintf("API Key Request [%s] %s", strings.ToUpper(status), keyReq.KeyName),
		"to":      applicantEmail,
		"body":    emailBody,
	}

	// Try to send via configured webhook URLs (from alert rules settings or env)
	// Slack: use SLACK_WEBHOOK_URL env var if set, otherwise skip
	if slackURL := os.Getenv("SLACK_WEBHOOK_URL"); slackURL != "" {
		notifyWebhook(slackURL, slackPayload)
	}

	// Email: use EMAIL_WEBHOOK_URL env var (e.g., a Mailgun/SendGrid relay endpoint)
	if emailURL := os.Getenv("EMAIL_WEBHOOK_URL"); emailURL != "" && applicantEmail != "" {
		notifyWebhook(emailURL, emailPayload)
	}
}
