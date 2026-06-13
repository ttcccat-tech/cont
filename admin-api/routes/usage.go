package routes

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ttcccat-tech/cont/admin-api/storage"
)

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

		org, err := store.GetOrganization(orgID)
		if err != nil || org == nil {
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "organization not found"})
			return
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
		plan, err := store.GetPlanByName(org.Plan)
		if err != nil || plan == nil {
			plan = &storage.Plan{RequestLimit: 100000, WorkspaceLimit: 3, UserLimit: 5}
		}

		c.JSON(http.StatusOK, OrgUsageResponse{
			OrgID:  orgID,
			Plan:   org.Plan,
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
		consumer, err := store.GetConsumer(consumerID)
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
			OrgID:      consumer.OrganizationID,
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

		c.JSON(http.StatusOK, UsageSummaryResponse{
			StartHour:     startHour,
			EndHour:       endHour,
			TotalRequests: total,
			TopOrgs:       topOrgs,
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