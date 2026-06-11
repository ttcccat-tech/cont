package storage

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

// ── Auth Groups & Resources ────────────────────────────────────────────────

type PermissionEntry struct {
	ResourceID string `json:"resource_id"`
	Mode       string `json:"mode"` // deny, read, write
}

type AuthGroup struct {
	ID          string             `json:"id"`
	Name        string             `json:"name" binding:"required,max=255"`
	Label       string             `json:"label"`
	Description string             `json:"description,omitempty"`
	Permissions []PermissionEntry  `json:"permissions,omitempty"`
	CreatedAt   string             `json:"created_at,omitempty"`
	UpdatedAt   string             `json:"updated_at,omitempty"`
}

type Resource struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type,omitempty"`
}

// ── Audit Log ──────────────────────────────────────────────────────────────

type AuditLog struct {
	ID           int64  `json:"id"`
	AuditType    string `json:"audit_type"`    // create, update, delete
	TargetType   string `json:"target_type"`   // service, route, consumer, etc.
	TargetID     string `json:"target_id"`
	ActorUserID  string `json:"actor_user_id"`
	ActorUsername string `json:"actor_username"`
	Description  string `json:"description"`
	CreatedAt    string `json:"created_at"`
}

// ── Alert Rules ────────────────────────────────────────────────────────────

type AlertRule struct {
	ID                  int64   `json:"id"`
	Name                string  `json:"name" binding:"required,max=255"`
	Description         string  `json:"description,omitempty"`
	MetricType          string  `json:"metric_type" binding:"required,oneof=error_rate latency"`
	ServiceName         string  `json:"service_name"`
	ThresholdValue      float64 `json:"threshold_value"`
	Operator            string  `json:"operator" binding:"required,oneof=> < >= <= =="`
	DurationSeconds     int     `json:"duration_seconds" binding:"min=1"`
	Enabled             bool    `json:"enabled"`
	NotificationChannels string `json:"notification_channels,omitempty"`
	SlackWebhookURL     string  `json:"slack_webhook_url,omitempty"`
	EmailWebhookURL     string  `json:"email_webhook_url,omitempty"`
	DiscordWebhookURL   string  `json:"discord_webhook_url,omitempty"`
	AlertSuppressSeconds int    `json:"alert_suppress_seconds"`
	CreatedAt           string  `json:"created_at,omitempty"`
	UpdatedAt           string  `json:"updated_at,omitempty"`
}

// ── API Key Requests ───────────────────────────────────────────────────────

type APIKeyRequest struct {
	ID                 int64  `json:"id"`
	KeyName            string `json:"key_name" binding:"required,max=255"`
	ConsumerName       string `json:"consumer_name"`
	Description        string `json:"description,omitempty"`
	Status             string `json:"status" binding:"omitempty,oneof=pending approved rejected"`
	ApplicantUserID    string `json:"applicant_user_id"`
	ApplicantUsername  string `json:"applicant_username"`
	ReviewedBy         string `json:"reviewed_by,omitempty"`
	ReviewedAt         string `json:"reviewed_at,omitempty"`
	CreatedAt          string `json:"created_at,omitempty"`
	UpdatedAt          string `json:"updated_at,omitempty"`
}

// ── Config Snapshots ───────────────────────────────────────────────────────

type ConfigSnapshot struct {
	ID           int64   `json:"id"`
	VersionLabel string  `json:"version_label"`
	DiffFromPrev *string `json:"diff_from_prev,omitempty"`
	ActorUserID  string  `json:"actor_user_id"`
	ActorUsername string `json:"actor_username"`
	CreatedAt    string  `json:"created_at"`
}

// ── Kong-compatible entities ───────────────────────────────────────────────

type Service struct {
	ID              string `json:"id"`
	Name            string `json:"name,omitempty" binding:"omitempty,max=255"`
	Protocol        string `json:"protocol,omitempty" binding:"omitempty,oneof=http https tcp tls udp grpc grpcs"`
	Host            string `json:"host,omitempty"`
	Port            int    `json:"port,omitempty" binding:"omitempty,min=1,max=65535"`
	Path            string `json:"path,omitempty" binding:"omitempty,max=8192"`
	URL             string `json:"url,omitempty" binding:"omitempty,url"`
	Retries         int    `json:"retries,omitempty" binding:"omitempty,min=0,max=100"`
	ConnectTimeout  int    `json:"connect_timeout,omitempty" binding:"omitempty,min=0,max=600000"`
	ReadTimeout     int    `json:"read_timeout,omitempty" binding:"omitempty,min=0,max=600000"`
	WriteTimeout    int    `json:"write_timeout,omitempty" binding:"omitempty,min=0,max=600000"`
	Enabled         bool   `json:"enabled"`
	CreatedAt       string `json:"created_at,omitempty"`
	UpdatedAt       string `json:"updated_at,omitempty"`
}

// Route wraps ServiceID so JSON serializes as {"service":{"id":"..."}}
type Route struct {
	ID                      string   `json:"id"`
	Name                    string   `json:"name,omitempty" binding:"omitempty,max=255"`
	Service                 *ServiceRef `json:"service,omitempty"`
	Protocols               []string `json:"protocols,omitempty" binding:"omitempty,min=1,dive,oneof=http https tcp tls grpc grpcs"`
	Hosts                   []string `json:"hosts,omitempty" binding:"omitempty,min=1,dive,fqdn"`
	Paths                   []string `json:"paths,omitempty" binding:"omitempty,min=1,dive,starts_with=/"`
	Methods                 []string `json:"methods,omitempty" binding:"omitempty,min=1,dive,oneof=GET POST PUT PATCH DELETE HEAD OPTIONS"`
	StripPath               bool     `json:"strip_path"`
	PreserveHost            bool     `json:"preserve_host"`
	RegexPriority           int      `json:"regex_priority,omitempty" binding:"omitempty,min=0"`
	HTTPSRedirectStatusCode int      `json:"https_redirect_status_code,omitempty" binding:"omitempty,oneof=301 302 307 308"`
	ConnectionTimeout       int      `json:"connection_timeout,omitempty" binding:"omitempty,min=0,max=600000"`
	Enabled                 bool     `json:"enabled"`
	CreatedAt               string   `json:"created_at,omitempty"`
	UpdatedAt               string   `json:"updated_at,omitempty"`
}

// ServiceRef is used for JSON serialize/deserialize of {"service":{"id":"..."}} or {"service":{"name":"..."}}
type ServiceRef struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// GetServiceID returns the service ID for SQL writes
func (r *Route) GetServiceID() string {
	if r.Service == nil {
		return ""
	}
	return r.Service.ID
}

// GetServiceName returns the service name for lookup
func (r *Route) GetServiceName() string {
	if r.Service == nil {
		return ""
	}
	return r.Service.Name
}

// UnmarshalJSON converts {"service":"uuid"} or {"service":{"id":"uuid"}} to ServiceRef
func (r *Route) UnmarshalJSON(data []byte) error {
	type routeAlias Route
	aux := struct {
		Service interface{} `json:"service"`
		*routeAlias
	}{
		routeAlias: (*routeAlias)(r),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	switch v := aux.Service.(type) {
	case string:
		r.Service = &ServiceRef{ID: v}
	case map[string]interface{}:
		if id, ok := v["id"].(string); ok && id != "" {
			r.Service = &ServiceRef{ID: id}
		} else if name, ok := v["name"].(string); ok && name != "" {
			// Store service.name for later resolution in CreateRoute handler
			r.Service = &ServiceRef{Name: name}
		}
	case nil:
		r.Service = nil
	}
	return nil
}

type Upstream struct {
	ID           string `json:"id"`
	Name         string `json:"name" binding:"required,max=255"`
	Algorithm    string `json:"algorithm,omitempty" binding:"omitempty,oneof=round-robin consistent-hashing least-connections"`
	Slots        int    `json:"slots,omitempty" binding:"omitempty,min=10,max=65536"`
	Healthchecks string `json:"healthchecks,omitempty"`
	Enabled      bool   `json:"enabled"`
	CreatedAt    string `json:"created_at,omitempty"`
	UpdatedAt    string `json:"updated_at,omitempty"`
}

type Target struct {
	ID         string `json:"id"`
	UpstreamID string `json:"-"`
	Target     string `json:"target,omitempty" binding:"required"`
	Weight     int    `json:"weight,omitempty" binding:"omitempty,min=0,max=1000"`
	Enabled    bool   `json:"enabled"`
	CreatedAt  string `json:"created_at,omitempty"`
}

type Consumer struct {
	ID        string `json:"id"`
	Username  string `json:"username,omitempty" binding:"required,max=255"`
	CustomID  string `json:"custom_id,omitempty" binding:"omitempty,max=255"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// PluginScope holds the id of the entity this plugin is attached to
type PluginScope struct {
	ID string `json:"id"`
}

type Plugin struct {
	ID         string          `json:"id"`
	Name       string          `json:"name" binding:"required,max=255"`
	Route      *PluginScope    `json:"route,omitempty"`
	Service    *PluginScope    `json:"service,omitempty"`
	Consumer   *PluginScope    `json:"consumer,omitempty"`
	Config     json.RawMessage `json:"config,omitempty"`
	Enabled    bool            `json:"enabled"`
	CreatedAt  string          `json:"created_at,omitempty"`
	UpdatedAt  string          `json:"updated_at,omitempty"`
}

type Workspace struct {
	ID        string `json:"id"`
	Name      string `json:"name" binding:"required,max=255"`
	CreatedAt string `json:"created_at,omitempty"`
}

type User struct {
	ID          string `json:"id"`
	Username    string `json:"username" binding:"required,max=255"`
	DisplayName string `json:"display_name,omitempty" binding:"omitempty,max=255"`
	Email       string `json:"email,omitempty" binding:"omitempty,email"`
	Role        string `json:"role" binding:"required,oneof=admin editor viewer"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
	// PasswordHash is never returned via JSON
	PasswordHash string         `json:"-"`
	Groups       []UserGroupRef `json:"groups,omitempty"`
}

// UserGroupRef is the group membership info returned with User
type UserGroupRef struct {
	Name  string `json:"name"`
	Label string `json:"label,omitempty"`
}

// ── /status response ────────────────────────────────────────────────────────

type StatusResponse struct {
	Version string `json:"version"`
	Uptime  int64  `json:"uptime"`
	Memory  struct {
		LuaVMSize    int64 `json:"lua_vms_size"`
		WorkersCount int   `json:"workers_count"`
	} `json:"memory"`
	Database struct {
		Reachable bool `json:"reachable"`
	} `json:"database"`
	Server struct {
		TotalRequests       int64 `json:"total_requests"`
		ConnectionsActive   int   `json:"connections_active"`
		ConnectionsAccepted int64 `json:"connections_accepted"`
	} `json:"server"`
	Workers []WorkerStatus `json:"workers"`
}

type WorkerStatus struct {
	Pid                int   `json:"pid"`
	MemoryLuaVMBytes   int64 `json:"memory_lua_vm_bytes"`
	ConnectionsActive  int   `json:"connections_active"`
	ConnectionsReading int   `json:"connections_reading"`
	ConnectionsWriting int   `json:"connections_writing"`
	ConnectionsWaiting int   `json:"connections_waiting"`
	TotalRequests      int64 `json:"total_requests"`
	Uptime             int64 `json:"uptime"`
}

var StartTime = time.Now()

// ── Validation helpers ─────────────────────────────────────────────────────

// SanitizeString trims whitespace and rejects control characters
func SanitizeString(s string) string {
	s = strings.TrimSpace(s)
	// Remove control characters (ASCII < 0x20 except TAB/LF/CR)
	var result strings.Builder
	for _, r := range s {
		if r >= 0x20 || r == '\t' || r == '\n' || r == '\r' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// IsValidTarget checks that a target string is host:port or [IPv6]:port
func IsValidTarget(target string) bool {
	if target == "" {
		return false
	}
	colonCount := strings.Count(target, ":")
	if colonCount == 0 {
		return false
	}
	// IPv6 format: [::1]:8080
	if strings.HasPrefix(target, "[") {
		bracketEnd := strings.Index(target, "]")
		if bracketEnd == -1 || bracketEnd+1 >= len(target) || target[bracketEnd+1] != ':' {
			return false
		}
		portStr := target[bracketEnd+2:]
		return isValidPort(portStr)
	}
	// If multiple colons and not IPv6, reject (not a valid host:port)
	if colonCount > 1 {
		return false
	}
	lastColon := strings.LastIndex(target, ":")
	host := target[:lastColon]
	portStr := target[lastColon+1:]
	if host == "" || portStr == "" {
		return false
	}
	return isValidPort(portStr)
}

func isValidPort(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	var port int
	for _, c := range s {
		port = port*10 + int(c-'0')
		if port > 65535 {
			return false
		}
	}
	return port >= 1
}

// IsValidHostname checks that a string is a valid hostname
func IsValidHostname(val string) bool {
	if val == "" {
		return false
	}
	if len(val) > 253 {
		return false
	}
	partRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)
	parts := strings.Split(val, ".")
	for _, part := range parts {
		if part == "" || len(part) > 63 {
			return false
		}
		if !partRegex.MatchString(part) {
			return false
		}
	}
	return true
}