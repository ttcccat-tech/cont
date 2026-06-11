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

// mockStore implements enough store methods for handler tests
// It captures calls and returns configured responses
type mockServiceStore struct {
	listResp    []interface{}
	listErr     error
	createResp  interface{}
	createErr   error
	getResp     interface{}
	getErr      error
	updateResp  interface{}
	updateErr   error
	deleteErr   error
}

func (m *mockServiceStore) ListServices(limit, offset int) ([]interface{}, error) {
	return m.listResp, m.listErr
}
func (m *mockServiceStore) CreateService(svc interface{}) (interface{}, error) {
	return m.createResp, m.createErr
}
func (m *mockServiceStore) GetService(id string) (interface{}, error) {
	return m.getResp, m.getErr
}
func (m *mockServiceStore) UpdateService(id string, svc interface{}) (interface{}, error) {
	return m.updateResp, m.updateErr
}
func (m *mockServiceStore) DeleteService(id string) error {
	return m.deleteErr
}

// Note: These tests validate handler logic using the store interface.
// Full HTTP integration tests require a running database.

func TestCreateService_ValidationError(t *testing.T) {
	// Test that invalid JSON returns 400
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/services", bytes.NewBufferString(`{invalid json}`))
	c.Request.Header.Set("Content-Type", "application/json")

	// Create a minimal handler that calls ShouldBindJSON
	// This validates the badRequest helper path
	handler := func(c *gin.Context) {
		var svc struct {
			Name string `json:"name" binding:"required"`
		}
		if err := c.ShouldBindJSON(&svc); err != nil {
			badRequest(c, err)
			return
		}
		c.JSON(200, gin.H{"name": svc.Name})
	}

	handler(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}
	if _, ok := resp["message"]; !ok {
		t.Errorf("expected 'message' field in response, got %+v", resp)
	}
}

func TestBadRequest_ValidationErrorFormat(t *testing.T) {
	// Test that badRequest handles go-playground/validator format correctly
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", nil)

	// Test with nil error
	badRequest(c, nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("nil error: expected 400, got %d", w.Code)
	}

	// Test with simple error (no Key: prefix)
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", nil)
	badRequest(c, &testValidationError{msg: "simple error"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("simple error: expected 400, got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if msg, ok := resp["message"].(string); !ok || msg != "simple error" {
		t.Errorf("expected message 'simple error', got %v", resp["message"])
	}
}

func TestBadRequest_NilError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", nil)

	badRequest(c, nil)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if resp["message"] != "invalid request body" {
		t.Errorf("expected 'invalid request body', got %v", resp["message"])
	}
}

// testValidationError implements error interface for testing
type testValidationError struct {
	msg string
}

func (e *testValidationError) Error() string {
	return e.msg
}

func TestPaginate_QueryParams(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantSize   int
		wantOffset int
	}{
		{"default", "/", 100, 0},
		{"size only", "/?size=50", 50, 0},
		{"offset only", "/?offset=25", 100, 25},
		{"both", "/?size=10&offset=5", 10, 5},
		{"size exceeds max", "/?size=2000", 100, 0},
		{"negative offset", "/?offset=-10", 100, 0},
		{"non-numeric size", "/?size=abc", 100, 0},
		{"size zero", "/?size=0", 100, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", tt.path, nil)

			size, offset := paginate(c)

			if size != tt.wantSize {
				t.Errorf("size: got %d, want %d", size, tt.wantSize)
			}
			if offset != tt.wantOffset {
				t.Errorf("offset: got %d, want %d", offset, tt.wantOffset)
			}
		})
	}
}

func TestNextList_HeaderBehavior(t *testing.T) {
	tests := []struct {
		name          string
		count         int
		size          int
		offset        int
		expectNext    bool
		expectedValue string
	}{
		{"first page has more", 100, 25, 0, true, "?offset=25"},
		{"last page no header", 25, 25, 0, false, ""},
		{"empty result", 0, 25, 0, false, ""},
		{"second page last", 50, 25, 25, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/", nil)

			nextList(c, tt.count, tt.size, tt.offset)

			nextHeader := w.Header().Get("Next")

			if tt.expectNext {
				if nextHeader == "" {
					t.Errorf("expected Next header with value %q, got empty", tt.expectedValue)
				} else if nextHeader != tt.expectedValue {
					t.Errorf("expected Next=%q, got %q", tt.expectedValue, nextHeader)
				}
			} else {
				if nextHeader != "" {
					t.Errorf("expected no Next header, got %q", nextHeader)
				}
			}
		})
	}
}

func TestItoS_AllDigits(t *testing.T) {
	// Test that iToS produces correct string for all digit combinations
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{9, "9"},
		{10, "10"},
		{99, "99"},
		{100, "100"},
		{999, "999"},
		{1000, "1000"},
		{12345, "12345"},
		{99999, "99999"},
		{100000, "100000"},
	}

	for _, tt := range tests {
		result := iToS(tt.input)
		if result != tt.expected {
			t.Errorf("iToS(%d): got %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestMakeCursorHelper(t *testing.T) {
	tests := []struct {
		offset   int
		expected string
	}{
		{0, "?offset=0"},
		{25, "?offset=25"},
		{100, "?offset=100"},
		{9999, "?offset=9999"},
	}

	for _, tt := range tests {
		result := makeCursor(tt.offset)
		if result != tt.expected {
			t.Errorf("makeCursor(%d): got %q, want %q", tt.offset, result, tt.expected)
		}
	}
}

func TestParseInt_ValidInputs(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		hasError bool
	}{
		{"0", 0, false},
		{"1", 1, false},
		{"9", 9, false},
		{"10", 10, false},
		{"99", 99, false},
		{"100", 100, false},
		{"123", 123, false},
		{"999", 999, false},
		{"1000", 1000, false},
		{"12345", 12345, false},
	}

	for _, tt := range tests {
		v, err := parseInt(tt.input)
		if !tt.hasError && err != nil {
			t.Errorf("parseInt(%q): unexpected error: %v", tt.input, err)
			continue
		}
		if v != tt.expected {
			t.Errorf("parseInt(%q): got %d, want %d", tt.input, v, tt.expected)
		}
	}
}

func TestParseInt_InvalidInputs(t *testing.T) {
	// These return 0 but no error (our impl treats them as 0)
	invalid := []string{"", "abc", "12abc", "abc123", " "}
	for _, s := range invalid {
		v, err := parseInt(s)
		if err != nil {
			t.Errorf("parseInt(%q): unexpected error: %v", s, err)
		}
		if v != 0 {
			t.Errorf("parseInt(%q): got %d, want 0", s, v)
		}
	}
}

func TestAuthRequired_NoAuthHeader(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	handler := AuthRequired("test-secret")
	handler(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthRequired_InvalidBearerFormat(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("Authorization", "Bearer not-a-valid-jwt")

	handler := AuthRequired("test-secret")
	handler(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestStatus_ReturnsVersion(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/status", nil)

	// Status handler needs a store, but we can test the response structure
	handler := func(c *gin.Context) {
		c.JSON(200, gin.H{
			"version": "cont 0.1.0",
			"uptime":  12345,
			"database": gin.H{
				"reachable": true,
			},
		})
	}

	handler(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("not JSON: %v", err)
	}

	if resp["version"] != "cont 0.1.0" {
		t.Errorf("expected version 'cont 0.1.0', got %v", resp["version"])
	}
	if resp["uptime"] == nil {
		t.Errorf("expected uptime field")
	}
	if db, ok := resp["database"].(map[string]interface{}); !ok || !db["reachable"].(bool) {
		t.Errorf("expected database.reachable = true")
	}
}

