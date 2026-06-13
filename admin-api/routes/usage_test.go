package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestGetOrgUsage_OrgIDRequired(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/usage/org/", nil)
	c.Params = gin.Params{{Key: "org_id", Value: ""}}

	GetOrgUsage(nil)(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if resp["code"] != "BAD_REQUEST" {
		t.Errorf("code: expected BAD_REQUEST, got %v", resp["code"])
	}
}

func TestGetOrgUsage_OrgNotFound(t *testing.T) {
	// Mock store that returns nil org
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/usage/org/test-org", nil)
	c.Params = gin.Params{{Key: "org_id", Value: "test-org"}}

	// Test with nil store (will return not found)
	GetOrgUsage(nil)(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetConsumerUsage_ConsumerIDRequired(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/usage/consumer/", nil)
	c.Params = gin.Params{{Key: "consumer_id", Value: ""}}

	GetConsumerUsage(nil)(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if resp["code"] != "BAD_REQUEST" {
		t.Errorf("code: expected BAD_REQUEST, got %v", resp["code"])
	}
}

func TestOrgUsageResponse_JSON(t *testing.T) {
	resp := OrgUsageResponse{
		OrgID:  "org-001",
		Plan:  "pro",
		Period: "daily",
		Total:  1000,
		Limit:  100000,
		Usage: []HourlyUsageItem{
			{Hour: "2024060100", Count: 100},
			{Hour: "2024060101", Count: 200},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if parsed["org_id"] != "org-001" {
		t.Errorf("org_id: expected org-001, got %v", parsed["org_id"])
	}
	if parsed["plan"] != "pro" {
		t.Errorf("plan: expected pro, got %v", parsed["plan"])
	}
	if parsed["period"] != "daily" {
		t.Errorf("period: expected daily, got %v", parsed["period"])
	}
	if parsed["total"].(float64) != 1000 {
		t.Errorf("total: expected 1000, got %v", parsed["total"])
	}
	if parsed["limit"].(float64) != 100000 {
		t.Errorf("limit: expected 100000, got %v", parsed["limit"])
	}
	usage, ok := parsed["usage"].([]interface{})
	if !ok {
		t.Fatalf("usage: expected []interface{}, got %T", parsed["usage"])
	}
	if len(usage) != 2 {
		t.Errorf("usage length: expected 2, got %d", len(usage))
	}
}

func TestConsumerUsageResponse_JSON(t *testing.T) {
	resp := ConsumerUsageResponse{
		ConsumerID: "consumer-001",
		OrgID:      "org-001",
		Period:     "hourly",
		Total:      500,
		Usage: []HourlyUsageItem{
			{Hour: "2024060112", Count: 50},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if parsed["consumer_id"] != "consumer-001" {
		t.Errorf("consumer_id: expected consumer-001, got %v", parsed["consumer_id"])
	}
	if parsed["org_id"] != "org-001" {
		t.Errorf("org_id: expected org-001, got %v", parsed["org_id"])
	}
	if parsed["period"] != "hourly" {
		t.Errorf("period: expected hourly, got %v", parsed["period"])
	}
}

func TestHourlyUsageItem_JSON(t *testing.T) {
	item := HourlyUsageItem{Hour: "2024060115", Count: 123}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if parsed["hour"] != "2024060115" {
		t.Errorf("hour: expected 2024060115, got %v", parsed["hour"])
	}
	if parsed["count"].(float64) != 123 {
		t.Errorf("count: expected 123, got %v", parsed["count"])
	}
}

func TestUsageEndpoint_QueryParams(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		expectKey string
	}{
		{"default period", "", "period"},
		{"daily period", "?period=daily", "period"},
		{"hourly period", "?period=hourly", "period"},
		{"with start/end", "?start=2024060100&end=2024060123", "start"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/usage/org/test-org"+tt.query, nil)
			c.Params = gin.Params{{Key: "org_id", Value: "test-org"}}

			// Just check request parsing doesn't panic
			_ = c.Request.URL.Query()
		})
	}
}

func TestUsageEndpoint_TimeFormat(t *testing.T) {
	// Verify the time format used by usage endpoint
	now := time.Now()
	format := "2006010215"
	formatted := now.Format(format)

	// Should be 10 characters: YYYYMMDDHH
	if len(formatted) != 10 {
		t.Errorf("time format length: expected 10, got %d", len(formatted))
	}
}
