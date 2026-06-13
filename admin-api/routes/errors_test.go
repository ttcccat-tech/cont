package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestBadRequestMsg(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	badRequestMsg(c, "test message")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if resp.Code != ErrCodeBadRequest {
		t.Errorf("code: expected %s, got %s", ErrCodeBadRequest, resp.Code)
	}
	if resp.Message != "test message" {
		t.Errorf("message: expected 'test message', got %q", resp.Message)
	}
	if resp.Details != nil {
		t.Errorf("details: expected nil, got %v", resp.Details)
	}
}

func TestBadRequestWithDetails(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	details := map[string]string{"field": "name", "reason": "required"}
	badRequestWithDetails(c, "validation failed", details)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if resp.Code != ErrCodeBadRequest {
		t.Errorf("code: expected %s, got %s", ErrCodeBadRequest, resp.Code)
	}
	if resp.Message != "validation failed" {
		t.Errorf("message: expected 'validation failed', got %q", resp.Message)
	}
	d, ok := resp.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("details: expected map, got %T", resp.Details)
	}
	if d["field"] != "name" {
		t.Errorf("details.field: expected 'name', got %v", d["field"])
	}
}

func TestBadRequestValidation(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	errs := []string{"name is required", "email invalid"}
	badRequestValidation(c, "validation failed", errs)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if resp["code"] != ErrCodeValidation {
		t.Errorf("code: expected %s, got %v", ErrCodeValidation, resp["code"])
	}
	if resp["message"] != "validation failed" {
		t.Errorf("message: expected 'validation failed', got %v", resp["message"])
	}
	errList, ok := resp["errors"].([]interface{})
	if !ok {
		t.Fatalf("errors: expected []interface{}, got %T", resp["errors"])
	}
	if len(errList) != 2 {
		t.Errorf("errors length: expected 2, got %d", len(errList))
	}
}

func TestUnauthorized(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	unauthorized(c, "invalid token")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if resp.Code != ErrCodeUnauthorized {
		t.Errorf("code: expected %s, got %s", ErrCodeUnauthorized, resp.Code)
	}
	if resp.Message != "invalid token" {
		t.Errorf("message: expected 'invalid token', got %q", resp.Message)
	}
}

func TestForbidden(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	forbidden(c, "access denied")

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if resp.Code != ErrCodeForbidden {
		t.Errorf("code: expected %s, got %s", ErrCodeForbidden, resp.Code)
	}
	if resp.Message != "access denied" {
		t.Errorf("message: expected 'access denied', got %q", resp.Message)
	}
}

func TestNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	notFound(c, "service not found")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if resp.Code != ErrCodeNotFound {
		t.Errorf("code: expected %s, got %s", ErrCodeNotFound, resp.Code)
	}
	if resp.Message != "service not found" {
		t.Errorf("message: expected 'service not found', got %q", resp.Message)
	}
}

func TestConflict(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	conflict(c, "service already exists")

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if resp.Code != ErrCodeConflict {
		t.Errorf("code: expected %s, got %s", ErrCodeConflict, resp.Code)
	}
	if resp.Message != "service already exists" {
		t.Errorf("message: expected 'service already exists', got %q", resp.Message)
	}
}

func TestBadGateway(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		details  []string
		wantCode int
	}{
		{"no details", "upstream error", nil, http.StatusBadGateway},
		{"with details", "upstream error", []string{"connection refused"}, http.StatusBadGateway},
		{"multiple details uses first", "upstream error", []string{"first", "second"}, http.StatusBadGateway},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/", nil)

			if len(tt.details) > 0 {
				badGateway(c, tt.message, tt.details[0])
			} else {
				badGateway(c, tt.message)
			}

			if w.Code != tt.wantCode {
				t.Errorf("expected %d, got %d", tt.wantCode, w.Code)
			}

			var resp ErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("not JSON: %v", err)
			}
			if resp.Code != ErrCodeBadGateway {
				t.Errorf("code: expected %s, got %s", ErrCodeBadGateway, resp.Code)
			}
			if resp.Message != tt.message {
				t.Errorf("message: expected %q, got %q", tt.message, resp.Message)
			}
			if len(tt.details) > 0 && resp.Details != tt.details[0] {
				t.Errorf("details: expected %q, got %v", tt.details[0], resp.Details)
			}
		})
	}
}

func TestInvalidJSON(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	invalidJSON(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if resp.Code != ErrCodeInvalidJSON {
		t.Errorf("code: expected %s, got %s", ErrCodeInvalidJSON, resp.Code)
	}
	if resp.Message != "invalid JSON body" {
		t.Errorf("message: expected 'invalid JSON body', got %q", resp.Message)
	}
}

func TestMissingField(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	missingField(c, "name")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if resp.Code != ErrCodeMissingField {
		t.Errorf("code: expected %s, got %s", ErrCodeMissingField, resp.Code)
	}
	if resp.Message != "missing required field: name" {
		t.Errorf("message: expected 'missing required field: name', got %q", resp.Message)
	}
}

func TestAlreadyExists(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	alreadyExists(c, "service:test-service")

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if resp.Code != ErrCodeAlreadyExists {
		t.Errorf("code: expected %s, got %s", ErrCodeAlreadyExists, resp.Code)
	}
	if resp.Message != "service:test-service already exists" {
		t.Errorf("message: expected 'service:test-service already exists', got %q", resp.Message)
	}
}

func TestInternalError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	internalError(c)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if resp.Code != ErrCodeInternal {
		t.Errorf("code: expected %s, got %s", ErrCodeInternal, resp.Code)
	}
	if resp.Message != "internal server error" {
		t.Errorf("message: expected 'internal server error', got %q", resp.Message)
	}
}

func TestInternalErrorWithLog(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	internalErrorWithLog(c, nil)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if resp.Code != ErrCodeInternal {
		t.Errorf("code: expected %s, got %s", ErrCodeInternal, resp.Code)
	}
	if resp.Message != "internal server error" {
		t.Errorf("message: expected 'internal server error', got %q", resp.Message)
	}
}

func TestErrorCodes(t *testing.T) {
	codes := []string{
		ErrCodeBadRequest,
		ErrCodeUnauthorized,
		ErrCodeForbidden,
		ErrCodeNotFound,
		ErrCodeConflict,
		ErrCodeInternal,
		ErrCodeValidation,
		ErrCodeBadGateway,
		ErrCodeInvalidJSON,
		ErrCodeMissingField,
		ErrCodeAlreadyExists,
	}
	expected := []string{
		"BAD_REQUEST",
		"UNAUTHORIZED",
		"FORBIDDEN",
		"NOT_FOUND",
		"CONFLICT",
		"INTERNAL_ERROR",
		"VALIDATION_ERROR",
		"BAD_GATEWAY",
		"INVALID_JSON",
		"MISSING_FIELD",
		"ALREADY_EXISTS",
	}
	for i, code := range codes {
		if code != expected[i] {
			t.Errorf("error code[%d]: expected %s, got %s", i, expected[i], code)
		}
	}
}
