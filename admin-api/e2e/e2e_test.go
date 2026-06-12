// Cont E2E Test Suite
// Runs against live docker-compose stack (admin-api + postgres + redis)
// Usage: go test -v ./e2e/... or make e2e-test
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const (
	baseURL       = "http://localhost:18081"
	proxyURL      = "http://localhost:18000"
	adminUser     = "admin"
	adminPass     = "admin123"
	editorUser    = "editor1"
	editorPass    = "password123"
	waitTimeout   = 30 * time.Second
	pollInterval  = 2 * time.Second
)

var (
	adminToken   string
	editorToken  string
	testServiceID string
)

// checkServicesUp verifies the API server is reachable
func checkServicesUp(t *testing.T) {
	if err := waitForURL(baseURL+"/health-check", 5*time.Second); err != nil {
		t.Skipf("API server not reachable at %s: %v", baseURL, err)
		return
	}
}

// execCmd runs a command and returns output
func execCmd(name string, args ...string) struct {
	Stdout, Stderr string
} {
	cmd := execCommand(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := struct{ Stdout, Stderr string }{}
	if err == nil {
		result.Stdout = stdout.String()
		result.Stderr = stderr.String()
	}
	return result
}

func execCommand(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

func waitForURL(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 500 {
				return nil
			}
		}
		time.Sleep(pollInterval)
	}
	return fmt.Errorf("URL %s not reachable within %v", url, timeout)
}

// ---- Auth Tests ----

func TestAuthLogin(t *testing.T) {
	checkServicesUp(t)

	body := map[string]interface{}{"username": adminUser, "password": adminPass}
	resp, err := http.Post(baseURL+"/auth/login", "application/json", jsonBody(body))
	if err != nil {
		t.Fatalf("POST /auth/login failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, string(b))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	token, ok := result["token"].(string)
	if !ok || token == "" {
		t.Fatal("No token in login response")
	}
	adminToken = token
	t.Logf("Admin token obtained: %s...", token[:20])
}

func TestAuthMe(t *testing.T) {
	if adminToken == "" {
		t.Skip("No admin token, run TestAuthLogin first")
	}

	req, _ := http.NewRequest("GET", baseURL+"/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /auth/me failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, string(b))
	}

	var me map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&me)
	if me["username"] != adminUser {
		t.Errorf("Expected username %s, got %v", adminUser, me["username"])
	}
	t.Logf("Auth /me OK: role=%v", me["role"])
}

func TestAuthInvalidCredentials(t *testing.T) {
	checkServicesUp(t)

	body := map[string]interface{}{"username": "admin", "password": "wrongpassword"}
	resp, err := http.Post(baseURL+"/auth/login", "application/json", jsonBody(body))
	if err != nil {
		t.Fatalf("POST /auth/login failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 401, got %d: %s", resp.StatusCode, string(b))
	}
	t.Log("Auth invalid credentials OK: 401")
}

func TestAuthEditorLogin(t *testing.T) {
	checkServicesUp(t)

	body := map[string]interface{}{"username": editorUser, "password": editorPass}
	resp, err := http.Post(baseURL+"/auth/login", "application/json", jsonBody(body))
	if err != nil {
		t.Fatalf("POST /auth/login for editor failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Skipf("Editor login failed (user may not exist): %d: %s", resp.StatusCode, string(b))
		return
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	token, ok := result["token"].(string)
	if !ok || token == "" {
		t.Fatal("No token in editor login response")
	}
	editorToken = token
	t.Logf("Editor token obtained: %s...", token[:20])
}

func TestAuthUnauthorizedAccess(t *testing.T) {
	checkServicesUp(t)

	// GET without token
	req, _ := http.NewRequest("GET", baseURL+"/services", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /services without token failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Expected 401 without token, got %d", resp.StatusCode)
	}
	t.Log("Unauthorized access OK: 401 without token")

	// GET with invalid token
	req2, _ := http.NewRequest("GET", baseURL+"/services", nil)
	req2.Header.Set("Authorization", "Bearer invalid-token-xyz")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("GET /services with invalid token failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Expected 401 with invalid token, got %d", resp2.StatusCode)
	}
	t.Log("Invalid token access OK: 401 with invalid token")
}

func TestAuthMePermissions(t *testing.T) {
	if adminToken == "" {
		t.Skip("No admin token")
	}

	req, _ := http.NewRequest("GET", baseURL+"/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /auth/me failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var me map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&me)

	// Check permissions structure
	perms, ok := me["permissions"].(map[string]interface{})
	if !ok {
		t.Fatal("No permissions in /auth/me response")
	}

	// Admin should have full permissions
	for _, entity := range []string{"services", "routes", "consumers", "plugins"} {
		if _, ok := perms[entity]; !ok {
			t.Errorf("Missing permissions for entity %s", entity)
		}
	}
	t.Logf("Auth /me permissions OK: %d entities", len(perms))
}

// ---- Services CRUD ----

func TestServicesCreate(t *testing.T) {
	checkServicesUp(t)
	if adminToken == "" {
		t.Skip("No admin token")
	}

	body := map[string]interface{}{
		"name":     "e2e-test-service",
		"protocol": "http",
		"host":     "localhost",
		"port":     8080,
		"enabled":  true,
	}
	resp, err := adminReq("POST", "/services", adminToken, body)
	if err != nil {
		t.Fatalf("POST /services failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 201, got %d: %s", resp.StatusCode, string(b))
	}

	var svc map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&svc)
	testServiceID, _ = svc["id"].(string)
	t.Logf("Service created: %s", testServiceID)
}

func TestServicesList(t *testing.T) {
	checkServicesUp(t)
	if adminToken == "" {
		t.Skip("No admin token")
	}

	resp, err := adminReq("GET", "/services", adminToken, nil)
	if err != nil {
		t.Fatalf("GET /services failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		Data []interface{} `json:"data"`
		Next string        `json:"next"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode list: %v", err)
	}
	t.Logf("Services list: %d items", len(result.Data))
}

func TestServicesGet(t *testing.T) {
	checkServicesUp(t)
	if testServiceID == "" {
		t.Skip("No test service ID")
	}

	resp, err := adminReq("GET", "/services/"+testServiceID, adminToken, nil)
	if err != nil {
		t.Fatalf("GET /services/:id failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, string(b))
	}

	var svc map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&svc)
	if svc["name"] != "e2e-test-service" {
		t.Errorf("Expected name e2e-test-service, got %v", svc["name"])
	}
}

func TestServicesUpdate(t *testing.T) {
	checkServicesUp(t)
	if testServiceID == "" {
		t.Skip("No test service ID")
	}

	body := map[string]interface{}{"name": "e2e-test-service-updated"}
	resp, err := adminReq("PUT", "/services/"+testServiceID, adminToken, body)
	if err != nil {
		t.Fatalf("PUT /services/:id failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, string(b))
	}

	var svc map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&svc)
	if svc["name"] != "e2e-test-service-updated" {
		t.Errorf("Expected name e2e-test-service-updated, got %v", svc["name"])
	}
}

func TestServicesPatch(t *testing.T) {
	checkServicesUp(t)
	if testServiceID == "" {
		t.Skip("No test service ID")
	}

	body := map[string]interface{}{"enabled": false, "name": "e2e-test-service-patched-" + testServiceID[:8]}
	resp, err := adminReq("PATCH", "/services/"+testServiceID, adminToken, body)
	if err != nil {
		t.Fatalf("PATCH /services/:id failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, string(b))
	}
	t.Log("Service PATCH OK")
}

func TestServicesDelete(t *testing.T) {
	checkServicesUp(t)
	if testServiceID == "" {
		t.Skip("No test service ID")
	}

	resp, err := adminReq("DELETE", "/services/"+testServiceID, adminToken, nil)
	if err != nil {
		t.Fatalf("DELETE /services/:id failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 204, got %d: %s", resp.StatusCode, string(b))
	}
	t.Log("Service deleted OK")
}

// ---- Routes CRUD ----

func TestRoutesCreate(t *testing.T) {
	checkServicesUp(t)
	if adminToken == "" {
		t.Skip("No admin token")
	}

	// Create a service first
	svcName := fmt.Sprintf("e2e-route-test-svc-%d", time.Now().UnixNano())
	body := map[string]interface{}{
		"name":    svcName,
		"host":    "localhost",
		"port":    8080,
		"enabled": true,
	}
	svcResp, err := adminReq("POST", "/services", adminToken, body)
	if err != nil || svcResp.StatusCode != http.StatusCreated {
		t.Skipf("Cannot create service for route test: %v", err)
	}
	defer svcResp.Body.Close()
	var svc map[string]interface{}
	json.NewDecoder(svcResp.Body).Decode(&svc)
	svcID, _ := svc["id"].(string)

	routeBody := map[string]interface{}{
		"name":             fmt.Sprintf("e2e-route-%d", time.Now().UnixNano()),
		"service":          map[string]interface{}{"id": svcID},
		"protocols":         []string{"http"},
		"hosts":            []string{"localhost"},
		"paths":            []string{"/e2e-test"},
		"strip_path":       true,
		"preserve_host":    false,
		"enabled":         true,
	}
	resp, err := adminReq("POST", "/routes", adminToken, routeBody)
	if err != nil {
		t.Fatalf("POST /routes failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 201, got %d: %s", resp.StatusCode, string(b))
	}
	t.Log("Route created OK")
}

func TestRoutesList(t *testing.T) {
	checkServicesUp(t)
	if adminToken == "" {
		t.Skip("No admin token")
	}

	resp, err := adminReq("GET", "/routes", adminToken, nil)
	if err != nil {
		t.Fatalf("GET /routes failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}
	t.Log("Routes list OK")
}

// ---- Upstreams CRUD ----

func TestUpstreamsCreate(t *testing.T) {
	checkServicesUp(t)
	if adminToken == "" {
		t.Skip("No admin token")
	}

	body := map[string]interface{}{
		"name":      fmt.Sprintf("e2e-upstream-%d", time.Now().UnixNano()),
		"algorithm": "roundrobin",
		"enabled":   true,
	}
	resp, err := adminReq("POST", "/upstreams", adminToken, body)
	if err != nil {
		t.Fatalf("POST /upstreams failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 201, got %d: %s", resp.StatusCode, string(b))
	}
	t.Log("Upstream created OK")
}

func TestUpstreamsList(t *testing.T) {
	checkServicesUp(t)
	if adminToken == "" {
		t.Skip("No admin token")
	}

	resp, err := adminReq("GET", "/upstreams", adminToken, nil)
	if err != nil {
		t.Fatalf("GET /upstreams failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}
	t.Log("Upstreams list OK")
}

// ---- Consumers CRUD ----

func TestConsumersCreate(t *testing.T) {
	checkServicesUp(t)
	if adminToken == "" {
		t.Skip("No admin token")
	}

	body := map[string]interface{}{
		"username": fmt.Sprintf("e2e-consumer-%d", time.Now().UnixNano()),
		"enabled":  true,
	}
	resp, err := adminReq("POST", "/consumers", adminToken, body)
	if err != nil {
		t.Fatalf("POST /consumers failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 201, got %d: %s", resp.StatusCode, string(b))
	}
	t.Log("Consumer created OK")
}

func TestConsumersList(t *testing.T) {
	checkServicesUp(t)
	if adminToken == "" {
		t.Skip("No admin token")
	}

	resp, err := adminReq("GET", "/consumers", adminToken, nil)
	if err != nil {
		t.Fatalf("GET /consumers failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}
	t.Log("Consumers list OK")
}

// ---- Plugins ----

func TestPluginsList(t *testing.T) {
	checkServicesUp(t)
	if adminToken == "" {
		t.Skip("No admin token")
	}

	resp, err := adminReq("GET", "/plugins", adminToken, nil)
	if err != nil {
		t.Fatalf("GET /plugins failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}
	t.Log("Plugins list OK")
}

// ---- Workspaces ----

func TestWorkspacesList(t *testing.T) {
	checkServicesUp(t)
	if adminToken == "" {
		t.Skip("No admin token")
	}

	resp, err := adminReq("GET", "/workspaces", adminToken, nil)
	if err != nil {
		t.Fatalf("GET /workspaces failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}
	t.Log("Workspaces list OK")
}

// ---- Health & Metrics ----

func TestHealthCheck(t *testing.T) {
	checkServicesUp(t)

	resp, err := http.Get(baseURL + "/health-check")
	if err != nil {
		t.Fatalf("GET /health-check failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}
	t.Log("Health check OK")
}

func TestMetrics(t *testing.T) {
	checkServicesUp(t)

	resp, err := http.Get(proxyURL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "kong_nginx_requests_total") {
		t.Logf("Metrics body sample: %s", string(body)[:200])
	}
	t.Log("Metrics OK")
}

func TestStatus(t *testing.T) {
	checkServicesUp(t)

	resp, err := http.Get(proxyURL + "/status")
	if err != nil {
		t.Fatalf("GET /status failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}
	t.Log("Status OK")
}

// ---- Helper functions ----

func jsonBody(body map[string]interface{}) io.Reader {
	data, _ := json.Marshal(body)
	return bytes.NewReader(data)
}

func adminReq(method, path, token string, body map[string]interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	return http.DefaultClient.Do(req)
}
