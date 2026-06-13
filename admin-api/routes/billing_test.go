package routes

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestCreateCheckoutSession_StripeNotConfigured(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/billing/checkout", nil)
	c.Set("org_id", "test-org")

	CreateCheckoutSession(nil, "http://localhost:3000")(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	msg, ok := resp["message"].(string)
	if !ok {
		t.Fatalf("message field missing or not string: %T", resp["message"])
	}
	if msg == "" {
		t.Errorf("message should not be empty")
	}
}

func TestCreateCheckoutSession_OrgIDRequired(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/billing/checkout", nil)
	// org_id not set

	CreateCheckoutSession(nil, "http://localhost:3000")(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetSubscription_OrgIDRequired(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/billing/subscription", nil)
	c.Set("org_id", "")

	GetSubscription(nil)(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetPlan_NotConfigured(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/billing/plans", nil)

	GetPlans(nil)(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	msg, ok := resp["message"].(string)
	if !ok {
		t.Fatalf("message field missing or not string: %T", resp["message"])
	}
}

func TestHandleStripeWebhook_StripeNotConfigured(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/webhooks/stripe", nil)

	HandleStripeWebhook(nil, "")(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	msg, ok := resp["message"].(string)
	if !ok {
		t.Fatalf("message field missing or not string: %T", resp["message"])
	}
}

func TestHandleStripeWebhook_InvalidJSON(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/webhooks/stripe", bytes.NewBufferString("not json"))
	c.Request.Header.Set("Content-Type", "application/json")

	// Even when stripeEnabled is false, this should return 400 for invalid JSON
	HandleStripeWebhook(nil, "")(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleStripeWebhook_MissingBody(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/webhooks/stripe", nil)
	// No body

	HandleStripeWebhook(nil, "")(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestBillingPortal_StripeNotConfigured(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/billing/portal", nil)
	c.Set("org_id", "test-org")

	CreatePortalSession(nil, "http://localhost:3000")(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestBillingPortal_OrgIDRequired(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/billing/portal", nil)
	// org_id not set

	CreatePortalSession(nil, "http://localhost:3000")(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestBillingPortal_MissingBody(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/billing/portal", nil)
	c.Set("org_id", "test-org")

	CreatePortalSession(nil, "http://localhost:3000")(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestBillingPortal_InvalidJSON(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/billing/portal", bytes.NewBufferString(`{invalid`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("org_id", "test-org")

	CreatePortalSession(nil, "http://localhost:3000")(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateCheckoutSession_InvalidJSON(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/billing/checkout", bytes.NewBufferString(`{invalid`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("org_id", "test-org")

	CreateCheckoutSession(nil, "http://localhost:3000")(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateCheckoutSession_InvalidPlan(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/billing/checkout", bytes.NewBufferString(`{"plan_name":"invalid","billing_cycle":"monthly"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("org_id", "test-org")

	CreateCheckoutSession(nil, "http://localhost:3000")(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateCheckoutSession_InvalidBillingCycle(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/billing/checkout", bytes.NewBufferString(`{"plan_name":"pro","billing_cycle":"weekly"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("org_id", "test-org")

	CreateCheckoutSession(nil, "http://localhost:3000")(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
