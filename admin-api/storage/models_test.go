package storage

import (
	"encoding/json"
	"strings"
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

// ── SanitizeString ─────────────────────────────────────────────────────────

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{"plain text unchanged", "hello world", "hello world"},
		{"whitespace trimmed", "  hello  ", "hello"},
		{"tab preserved", "hello\tworld", "hello\tworld"},
		{"newline preserved", "hello\nworld", "hello\nworld"},
		{"cr preserved", "hello\rworld", "hello\rworld"},
		{"control chars stripped", "hello\x00world", "helloworld"},
		{"bell stripped", "hello\x07world", "helloworld"},
		{"null byte stripped", "a\x00b", "ab"},
		{"leading+trailing trimmed (internal whitespace preserved)", "  hello\tworld\n\r  ", "hello\tworld"},
		{"empty", "", ""},
		{"only whitespace", "   \t\n\r   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeString(tt.input)
			if got != tt.expect {
				t.Errorf("SanitizeString(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}

// ── IsValidTarget ──────────────────────────────────────────────────────────

func TestIsValidTarget(t *testing.T) {
	tests := []struct {
		name   string
		target string
		valid  bool
	}{
		// Valid host:port
		{"ipv4 with port", "192.168.1.1:8080", true},
		{"hostname with port", "example.com:8080", true},
		{"localhost with port", "localhost:3000", true},
		{"port 80", "10.0.0.1:80", true},
		{"port 443", "10.0.0.1:443", true},
		// Valid IPv6
		{"ipv6 bracketed", "[::1]:8080", true},
		{"ipv6 loopback", "[::1]:80", true},
		{"ipv6 full", "[2001:db8::1]:8080", true},
		{"ipv6 localhost", "[::]:8000", true},
		// Invalid
		{"no port", "192.168.1.1", false},
		{"empty", "", false},
		{"port 0", "192.168.1.1:0", false},
		{"port too high", "192.168.1.1:65536", false},
		{"port non-numeric", "192.168.1.1:abc", false},
		{"double colon ipv4", "192.168.1.1:8080:9090", false},
		{"ipv6 no closing bracket", "[::1:8080", false},
		{"ipv6 no port after bracket", "[::1]", false},
		{"empty host", ":8080", false},
		{"empty port", "192.168.1.1:", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidTarget(tt.target)
			if got != tt.valid {
				t.Errorf("IsValidTarget(%q) = %v, want %v", tt.target, got, tt.valid)
			}
		})
	}
}

// ── isValidPort ────────────────────────────────────────────────────────────

func TestIsValidPort(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"1", true},
		{"80", true},
		{"443", true},
		{"8080", true},
		{"3000", true},
		{"65535", true},
		{"0", false},
		{"65536", false},
		{"-1", false},
		{"", false},
		{"80a", false},
		{"80abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isValidPort(tt.input)
			if got != tt.valid {
				t.Errorf("isValidPort(%q) = %v, want %v", tt.input, got, tt.valid)
			}
		})
	}
}

// ── IsValidHostname ─────────────────────────────────────────────────────────

func TestIsValidHostname(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		// Valid
		{"simple", "example.com", true},
		{"subdomain", "www.example.com", true},
		{"deep subdomain", "a.b.c.example.com", true},
		{"alphanumeric", "test123.example456.com", true},
		{"hyphen in label", "my-host.example.com", true},
		{"single label", "localhost", true},
		{"mixed case", "Example.Com", true},
		{"label exactly 63 chars (max valid)", strings.Repeat("a", 63) + ".com", true},
		{"label 64 chars (invalid)", strings.Repeat("a", 64) + ".com", false},
		// Invalid
		{"empty", "", false},
		{"label starts hyphen", "-example.com", false},
		{"label ends hyphen", "example-.com", false},
		{"empty label", "example..com", false},
		{"underscore", "example_name.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidHostname(tt.input)
			if got != tt.valid {
				t.Errorf("IsValidHostname(%q) = %v, want %v", tt.input, got, tt.valid)
			}
		})
	}
}

// ── ConsumerCredential.ToResponse ─────────────────────────────────────────

func TestConsumerCredential_ToResponse(t *testing.T) {
	cc := ConsumerCredential{
		ID:             "cred-001",
		ConsumerID:     "cons-001",
		CredentialType: "key-auth",
		Key:            "xKq3J9vNm2P",
		Secret:         "super-secret-bcrypt-hash",
		Enabled:        true,
	}
	resp := cc.ToResponse()
	if resp.Key != "xKq3J9vNm2P" {
		t.Errorf("expected key xKq3J9vNm2P, got %s", resp.Key)
	}
	if resp.ConsumerID != "cons-001" {
		t.Errorf("expected consumer_id cons-001, got %s", resp.ConsumerID)
	}
	if resp.CredentialType != "key-auth" {
		t.Errorf("expected credential_type key-auth, got %s", resp.CredentialType)
	}
	if resp.Enabled != true {
		t.Errorf("expected enabled true, got %v", resp.Enabled)
	}
	// Verify secret is NOT in the response (ToResponse hides it)
	data, _ := json.Marshal(resp)
	var back map[string]interface{}
	json.Unmarshal(data, &back)
	if _, ok := back["secret"]; ok {
		t.Error("secret must not appear in CredentialResponse JSON")
	}
}
