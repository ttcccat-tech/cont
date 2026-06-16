package routes

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ttcccat-tech/cont/admin-api/storage"
)

// ── POST /internal/usage/incr ──────────────────────────────────────────────────

type IncrUsageRequest struct {
	OrgID      string `json:"org_id" binding:"required"`
	ConsumerID string `json:"consumer_id"`
	RouteID    string `json:"route_id"`
	ServiceID  string `json:"service_id"`
	LatencyMs  int64  `json:"latency_ms"`
	StatusCode int    `json:"status_code"`
}

func IncrUsage(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req IncrUsageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "message": err.Error()})
			return
		}
		if req.OrgID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "message": "org_id required"})
			return
		}

		count, err := store.Redis().IncrUsage(
			c.Request.Context(),
			req.OrgID,
			req.ConsumerID,
			req.RouteID,
			req.ServiceID,
			req.LatencyMs,
			req.StatusCode,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "count": count})
	}
}

// ── GET /usage/org/:org_id ─────────────────────────────────────────────────────

type OrgUsageResponse struct {
	OrgID    string          `json:"org_id"`
	Plan     string          `json:"plan"`
	Period   string          `json:"period"` // "daily" or "hourly"
	Total    int64           `json:"total"`
	Limit    int64           `json:"limit"`  // -1 = unlimited
	Usage    []HourlyUsageItem `json:"usage"`
}

type HourlyUsageItem struct {
	Hour  string `json:"hour"`
	Count int64  `json:"count"`
}

func GetOrgUsage(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		orgID := c.Param("org_id")
		if orgID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "message": "org_id required"})
			return
		}

		// Parse time range from query params
		startHour := c.DefaultQuery("start", time.Now().Add(-24*time.Hour).Format("2006010215"))
		endHour := c.DefaultQuery("end", time.Now().Format("2006010215"))
		period := c.DefaultQuery("period", "daily")

		var orgPlan string
		if orgID == "00000000-0000-0000-0000-000000000000" {
			// Default org — no DB record, use free plan
			orgPlan = "default"
		} else {
			org, err := store.GetOrganization(orgID)
			if err != nil || org == nil {
				c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "organization not found"})
				return
			}
			orgPlan = org.Plan
		}

		hourlyData, err := store.Redis().GetOrgUsageByHour(c.Request.Context(), orgID, startHour, endHour)
		if err != nil {
			hourlyData = []storage.HourlyUsage{}
		}

		// Convert to response format
		usage := make([]HourlyUsageItem, len(hourlyData))
		var total int64
		for i, h := range hourlyData {
			usage[i] = HourlyUsageItem{Hour: h.Hour, Count: h.Count}
			total += h.Count
		}

		// Get plan limits
		plan, err := store.GetPlanByName(orgPlan)
		if err != nil || plan == nil {
			plan = &storage.Plan{RequestLimit: 100000, WorkspaceLimit: 3, UserLimit: 5}
		}

		c.JSON(http.StatusOK, OrgUsageResponse{
			OrgID:  orgID,
			Plan:   orgPlan,
			Period: period,
			Total:  total,
			Limit:  plan.RequestLimit,
			Usage:  usage,
		})
	}
}

// ── GET /usage/consumer/:consumer_id ──────────────────────────────────────────

type ConsumerUsageResponse struct {
	ConsumerID string          `json:"consumer_id"`
	OrgID      string          `json:"org_id"`
	Period     string          `json:"period"`
	Total      int64           `json:"total"`
	Usage      []HourlyUsageItem `json:"usage"`
}

func GetConsumerUsage(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		consumerID := c.Param("consumer_id")
		if consumerID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "message": "consumer_id required"})
			return
		}

		startHour := c.DefaultQuery("start", time.Now().Add(-24*time.Hour).Format("2006010215"))
		endHour := c.DefaultQuery("end", time.Now().Format("2006010215"))
		period := c.DefaultQuery("period", "daily")

		// Look up consumer to get org_id
		consumer, err := store.GetConsumer(consumerID, "")
		if err != nil || consumer == nil {
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "consumer not found"})
			return
		}

		hourlyData, err := store.Redis().GetConsumerUsageByHour(c.Request.Context(), consumerID, startHour, endHour)
		if err != nil {
			hourlyData = []storage.HourlyUsage{}
		}

		usage := make([]HourlyUsageItem, len(hourlyData))
		var total int64
		for i, h := range hourlyData {
			usage[i] = HourlyUsageItem{Hour: h.Hour, Count: h.Count}
			total += h.Count
		}

		c.JSON(http.StatusOK, ConsumerUsageResponse{
			ConsumerID: consumerID,
			OrgID:      consumer.OrgID,
			Period:     period,
			Total:      total,
			Usage:      usage,
		})
	}
}

// ── GET /usage/summary ─────────────────────────────────────────────────────────

type UsageSummaryResponse struct {
	StartHour   string          `json:"start_hour"`
	EndHour     string          `json:"end_hour"`
	TotalRequests int64         `json:"total_requests"`
	TopOrgs     []OrgUsageItem  `json:"top_orgs"`
}

type OrgUsageItem struct {
	OrgID string `json:"org_id"`
	Count int64  `json:"count"`
}

func GetUsageSummary(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		startHour := c.DefaultQuery("start", time.Now().Add(-24*time.Hour).Format("2006010215"))
		endHour := c.DefaultQuery("end", time.Now().Format("2006010215"))
		limitStr := c.DefaultQuery("limit", "10")
		limit, _ := strconv.ParseInt(limitStr, 10, 64)
		if limit <= 0 || limit > 100 {
			limit = 10
		}

		topOrgs, err := store.Redis().GetTopOrgsByUsage(c.Request.Context(), startHour, endHour, int(limit))
		if err != nil {
			topOrgs = []struct {
				OrgID string `json:"org_id"`
				Count int64  `json:"count"`
			}{}
		}

		var total int64
		for _, o := range topOrgs {
			total += o.Count
		}

		// Convert to OrgUsageItem
		orgUsageItems := make([]OrgUsageItem, len(topOrgs))
		for i, o := range topOrgs {
			orgUsageItems[i] = OrgUsageItem{OrgID: o.OrgID, Count: o.Count}
		}

		c.JSON(http.StatusOK, UsageSummaryResponse{
			StartHour:     startHour,
			EndHour:       endHour,
			TotalRequests: total,
			TopOrgs:       orgUsageItems,
		})
	}
}

// Helper function to calculate percentage
func calcPercent(used, limit int64) float64 {
	if limit <= 0 {
		return 0
	}
	return math.Round(float64(used)/float64(limit)*10000) / 100
}

// ── GET /usage/analytics ───────────────────────────────────────────────────────
// Returns aggregated analytics for the current org's usage dashboard:
// monthly total, plan quota, usage percentage, hourly trend,
// top routes, and top consumers.
type AnalyticsResponse struct {
	OrgID            string              `json:"org_id"`
	Plan             string              `json:"plan"`
	MonthlyTotal     int64               `json:"monthly_total"`
	PlanQuota        int64               `json:"plan_quota"`
	UsagePercentage  float64             `json:"usage_percentage"`
	HourlyTrend      []HourlyUsageItem   `json:"hourly_trend"`
	TopRoutes        []RouteUsageItem    `json:"top_routes"`
	TopConsumers     []ConsumerUsageItem `json:"top_consumers"`
}

type RouteUsageItem struct {
	RouteID string `json:"route_id"`
	Count   int64  `json:"count"`
}

type ConsumerUsageItem struct {
	ConsumerID string `json:"consumer_id"`
	Count      int64  `json:"count"`
}

// getAnalyticsOrgID resolves the org_id for analytics from query param or JWT claim.
// Query param takes precedence (supports GET /usage/analytics?org_id=X).
func getAnalyticsOrgID(c *gin.Context) string {
	// Query param takes precedence per acceptance criteria
	if orgID := c.Query("org_id"); orgID != "" {
		return orgID
	}
	if orgID, ok := c.Get("org_id"); ok {
		if s, ok := orgID.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

func GetAnalyticsUsage(store *storage.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		orgID := getAnalyticsOrgID(c)
		if orgID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "message": "org_id required"})
			return
		}

		// Look up org only if not the zero UUID (admin default org has no DB record)
		var orgPlan string
		if orgID != "00000000-0000-0000-0000-000000000000" {
			org, err := store.GetOrganization(orgID)
			if err != nil || org == nil {
				c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "organization not found"})
				return
			}
			orgPlan = org.Plan
		} else {
			// Admin default org — use default plan
			orgPlan = "default"
		}

		plan, err := store.GetPlanByName(orgPlan)
		if err != nil || plan == nil {
			plan = &storage.Plan{RequestLimit: 100000, WorkspaceLimit: 3, UserLimit: 5}
		}

		// Monthly total: sum all hourly buckets from 1st of month to now
		monthlyTotal, err := store.Redis().GetMonthlyUsage(c.Request.Context(), orgID)
		if err != nil {
			monthlyTotal = 0
		}

		// Hourly trend: last 7 days (168 hours)
		startHour := time.Now().Add(-7 * 24 * time.Hour).Format("2006010215")
		endHour := time.Now().Format("2006010215")
		hourlyData, _ := store.Redis().GetOrgUsageByHour(c.Request.Context(), orgID, startHour, endHour)
		hourlyTrend := make([]HourlyUsageItem, len(hourlyData))
		for i, h := range hourlyData {
			hourlyTrend[i] = HourlyUsageItem{Hour: h.Hour, Count: h.Count}
		}

		// Top routes
		topRoutesRaw, _ := store.Redis().GetTopRoutesByUsage(c.Request.Context(), startHour, endHour, 5)
		topRoutes := make([]RouteUsageItem, len(topRoutesRaw))
		for i, r := range topRoutesRaw {
			topRoutes[i] = RouteUsageItem{RouteID: r.RouteID, Count: r.Count}
		}

		// Top consumers
		topConsumersRaw, _ := store.Redis().GetTopConsumersByUsage(c.Request.Context(), startHour, endHour, 5)
		topConsumers := make([]ConsumerUsageItem, len(topConsumersRaw))
		for i, c := range topConsumersRaw {
			topConsumers[i] = ConsumerUsageItem{ConsumerID: c.ConsumerID, Count: c.Count}
		}

		usagePercent := calcPercent(monthlyTotal, plan.RequestLimit)

		c.JSON(http.StatusOK, AnalyticsResponse{
			OrgID:           orgID,
			Plan:            orgPlan,
			MonthlyTotal:    monthlyTotal,
			PlanQuota:       plan.RequestLimit,
			UsagePercentage: usagePercent,
			HourlyTrend:     hourlyTrend,
			TopRoutes:       topRoutes,
			TopConsumers:    topConsumers,
		})
	}
}