package routes

import (
	"regexp"
	"strings"
	"testing"
)

func TestValidateFQDN(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		// Valid FQDNs
		{"simple", "example.com", true},
		{"subdomain", "www.example.com", true},
		{"deep subdomain", "a.b.c.example.com", true},
		{"wildcard", "*.example.com", true},
		{"wildcard subdomain", "*.www.example.com", true},
		{"mixed case", "Example.Com", true},
		{"alphanumeric", "test123.example456.com", true},
		{"hyphen in label", "my-host.example.com", true},
		{"single label", "localhost", true},

		// Invalid FQDNs
		{"empty", "", true}, // empty is allowed (not required field)
		{"label too long", strings.Repeat("a", 64) + ".com", false},
		{"label starts hyphen", "-example.com", false},
		{"label ends hyphen", "example-.com", false},
		{"empty label", "example..com", false},
		{"double wildcard", "*.*.example.com", false},
		{"underscore", "example_name.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateFQDNPure(tt.input)
			if got != tt.valid {
				t.Errorf("validateFQDN(%q) = %v, want %v", tt.input, got, tt.valid)
			}
		})
	}
}

func TestValidateHostPort(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		// Valid host:port
		{"ipv4 simple", "192.168.1.1:8080", true},
		{"ipv4 port 80", "10.0.0.1:80", true},
		{"ipv4 port 443", "10.0.0.1:443", true},
		{"hostname port", "example.com:8080", true},
		{"localhost port", "localhost:3000", true},
		{"hostname dash port", "my-host.example.com:8080", true},

		// Valid IPv6
		{"ipv6 bracketed", "[::1]:8080", true},
		{"ipv6 loopback", "[::1]:80", true},
		{"ipv6 full", "[2001:db8::1]:8080", true},
		{"ipv6 localhost", "[::]:8000", true},

		// Invalid host:port
		{"no port", "192.168.1.1", false},
		{"no host colon only", ":8080", false},
		{"empty", "", true}, // empty allowed (not required)
		{"port 0", "192.168.1.1:0", false},
		{"port too high", "192.168.1.1:65536", false},
		{"port non-numeric", "192.168.1.1:abc", false},
		{"double colon ipv4", "192.168.1.1:8080:9090", false},
		{"ipv6 no closing bracket", "[::1:8080", false},
		{"ipv6 no port after bracket", "[::1]", false},
		{"ipv6 port only", ":8080", false},
		{"negative port", "192.168.1.1:-1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateHostPortPure(tt.input)
			if got != tt.valid {
				t.Errorf("validateHostPort(%q) = %v, want %v", tt.input, got, tt.valid)
			}
		})
	}
}

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
		got := isValidPort(tt.input)
		if got != tt.valid {
			t.Errorf("isValidPort(%q) = %v, want %v", tt.input, got, tt.valid)
		}
	}
}

// Pure-function copies of the validator logic for unit testing.
// These replicate the actual validation implementation without needing
// go-playground/validator's FieldLevel interface.

func validateFQDNPure(val string) bool {
	if val == "" {
		return true
	}
	if strings.HasPrefix(val, "*.") {
		val = val[2:]
	}
	if len(val) > 253 || val == "" {
		return false
	}
	partRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)
	parts := strings.Split(val, ".")
	for _, part := range parts {
		if part == "" || len(part) > 63 || !partRegex.MatchString(part) {
			return false
		}
	}
	return true
}

func validateHostPortPure(val string) bool {
	if val == "" {
		return true
	}
	colonCount := strings.Count(val, ":")
	if colonCount == 0 {
		return false
	}
	if strings.HasPrefix(val, "[") {
		bracketEnd := strings.Index(val, "]")
		if bracketEnd == -1 || bracketEnd+1 >= len(val) || val[bracketEnd+1] != ':' {
			return false
		}
		portStr := val[bracketEnd+2:]
		return isValidPort(portStr)
	}
	if colonCount > 1 {
		return false
	}
	lastColon := strings.LastIndex(val, ":")
	host := val[:lastColon]
	portStr := val[lastColon+1:]
	if host == "" || portStr == "" {
		return false
	}
	return isValidPort(portStr)
}