package storage

import (
	"encoding/json"
	"testing"
)

func TestRoute_UnmarshalJSON_IDFormat(t *testing.T) {
	input := `{"id":"route-001","name":"test-route","service":{"id":"svc-001"}}`
	var r Route
	if err := json.Unmarshal([]byte(input), &r); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if r.ID != "route-001" {
		t.Errorf("expected ID route-001, got %s", r.ID)
	}
	if r.GetServiceID() != "svc-001" {
		t.Errorf("expected service.id svc-001, got %s", r.GetServiceID())
	}
	if r.GetServiceName() != "" {
		t.Errorf("expected empty service name, got %s", r.GetServiceName())
	}
}

func TestRoute_UnmarshalJSON_NameFormat(t *testing.T) {
	input := `{"id":"route-002","name":"test-route","service":{"name":"my-service"}}`
	var r Route
	if err := json.Unmarshal([]byte(input), &r); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if r.ID != "route-002" {
		t.Errorf("expected ID route-002, got %s", r.ID)
	}
	if r.GetServiceID() != "" {
		t.Errorf("expected empty service ID for name format, got %s", r.GetServiceID())
	}
	if r.GetServiceName() != "my-service" {
		t.Errorf("expected service.name my-service, got %s", r.GetServiceName())
	}
}

func TestRoute_UnmarshalJSON_StringService(t *testing.T) {
	input := `{"id":"route-003","service":"svc-string-format"}`
	var r Route
	if err := json.Unmarshal([]byte(input), &r); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if r.GetServiceID() != "svc-string-format" {
		t.Errorf("expected service.id svc-string-format, got %s", r.GetServiceID())
	}
}

func TestRoute_UnmarshalJSON_NilService(t *testing.T) {
	input := `{"id":"route-004","name":"no-service-route"}`
	var r Route
	if err := json.Unmarshal([]byte(input), &r); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if r.GetServiceID() != "" {
		t.Errorf("expected empty service ID, got %s", r.GetServiceID())
	}
	if r.GetServiceName() != "" {
		t.Errorf("expected empty service name, got %s", r.GetServiceName())
	}
}

func TestRoute_GetServiceID_NilService(t *testing.T) {
	var r Route
	if id := r.GetServiceID(); id != "" {
		t.Errorf("expected empty string for nil service, got %s", id)
	}
}

func TestRoute_GetServiceName_NilService(t *testing.T) {
	var r Route
	if name := r.GetServiceName(); name != "" {
		t.Errorf("expected empty string for nil service, got %s", name)
	}
}

func TestServiceRef_JSON(t *testing.T) {
	// Test ServiceRef marshaling
	ref := ServiceRef{ID: "test-id", Name: "test-name"}
	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var back ServiceRef
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if back.ID != "test-id" || back.Name != "test-name" {
		t.Errorf("expected id=test-id name=test-name, got id=%s name=%s", back.ID, back.Name)
	}
}

func TestService_JSON_Marshal(t *testing.T) {
	svc := Service{
		ID:   "svc-001",
		Name: "test-svc",
		Protocol: "http",
		Host: "example.com",
		Port: 80,
		Enabled: true,
	}
	data, err := json.Marshal(svc)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var back Service
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if back.ID != "svc-001" || back.Host != "example.com" {
		t.Errorf("unexpected: %+v", back)
	}
}

func TestConsumer_JSON(t *testing.T) {
	c := Consumer{
		ID:       "cons-001",
		Username: "alice",
		CustomID: "custom-123",
		Enabled:  true,
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	// PasswordHash should be omitted
	var back map[string]interface{}
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if _, ok := back["password_hash"]; ok {
		t.Error("password_hash should not be in JSON output")
	}
	if back["username"] != "alice" {
		t.Errorf("expected username alice, got %v", back["username"])
	}
}

func TestPlugin_JSON(t *testing.T) {
	p := Plugin{
		ID:      "plugin-001",
		Name:    "rate-limiting",
		Enabled: true,
		Config:  json.RawMessage(`{"minute":100}`),
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var back Plugin
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if back.Name != "rate-limiting" {
		t.Errorf("expected name rate-limiting, got %s", back.Name)
	}
}

func TestUpstream_JSON(t *testing.T) {
	u := Upstream{
		ID:        "upstream-001",
		Name:      "lb-backend",
		Algorithm: "round-robin",
		Slots:     1000,
		Enabled:   true,
	}
	data, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var back Upstream
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if back.Name != "lb-backend" || back.Algorithm != "round-robin" {
		t.Errorf("unexpected: %+v", back)
	}
}

func TestWorkspace_JSON(t *testing.T) {
	w := Workspace{
		ID:   "ws-001",
		Name: "production",
	}
	data, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var back Workspace
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if back.Name != "production" {
		t.Errorf("unexpected: %+v", back)
	}
}

func TestUser_JSON_PasswordHashOmitted(t *testing.T) {
	u := User{
		ID:           "user-001",
		Username:     "admin",
		DisplayName:  "Admin User",
		Email:        "admin@example.com",
		Role:         "admin",
		Enabled:      true,
		PasswordHash: "secret-hash-should-not-leak",
	}
	data, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var back map[string]interface{}
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if _, ok := back["password_hash"]; ok {
		t.Error("password_hash must not be serialized to JSON")
	}
	if back["username"] != "admin" {
		t.Errorf("expected username admin, got %v", back["username"])
	}
}