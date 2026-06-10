package storage

import (
	"encoding/json"
	"time"
)

// ── Kong-compatible entities ───────────────────────────────────────────────

type Service struct {
	ID              string `json:"id"`
	Name            string `json:"name,omitempty"`
	Protocol        string `json:"protocol,omitempty"`
	Host            string `json:"host,omitempty"`
	Port            int    `json:"port,omitempty"`
	Path            string `json:"path,omitempty"`
	URL             string `json:"url,omitempty"`
	Retries         int    `json:"retries,omitempty"`
	ConnectTimeout  int    `json:"connect_timeout,omitempty"`
	ReadTimeout     int    `json:"read_timeout,omitempty"`
	WriteTimeout    int    `json:"write_timeout,omitempty"`
	Enabled         bool   `json:"enabled"`
	CreatedAt       string `json:"created_at,omitempty"`
	UpdatedAt       string `json:"updated_at,omitempty"`
}

// Route wraps ServiceID so JSON serializes as {"service":{"id":"..."}}
type Route struct {
	ID                       string   `json:"id"`
	Name                     string   `json:"name,omitempty"`
	Service                  *ServiceRef `json:"service,omitempty"`
	Protocols                []string `json:"protocols,omitempty"`
	Hosts                    []string `json:"hosts,omitempty"`
	Paths                    []string `json:"paths,omitempty"`
	Methods                  []string `json:"methods,omitempty"`
	StripPath                bool     `json:"strip_path"`
	PreserveHost             bool     `json:"preserve_host"`
	RegexPriority            int      `json:"regex_priority,omitempty"`
	HTTPSRedirectStatusCode int      `json:"https_redirect_status_code,omitempty"`
	ConnectionTimeout        int      `json:"connection_timeout,omitempty"`
	Enabled                  bool     `json:"enabled"`
	CreatedAt                string   `json:"created_at,omitempty"`
	UpdatedAt                string   `json:"updated_at,omitempty"`
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
	ID          string `json:"id"`
	Name        string `json:"name"`
	Algorithm   string `json:"algorithm,omitempty"`
	Slots       int    `json:"slots,omitempty"`
	Healthchecks string `json:"healthchecks,omitempty"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

type Target struct {
	ID         string `json:"id"`
	UpstreamID string `json:"-"`
	Target     string `json:"target,omitempty"`
	Weight     int    `json:"weight,omitempty"`
	Enabled    bool   `json:"enabled"`
	CreatedAt  string `json:"created_at,omitempty"`
}

type Consumer struct {
	ID        string `json:"id"`
	Username  string `json:"username,omitempty"`
	CustomID  string `json:"custom_id,omitempty"`
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
	Name       string          `json:"name"`
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
	Name      string `json:"name"`
	CreatedAt string `json:"created_at,omitempty"`
}

type User struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name,omitempty"`
	Email       string `json:"email,omitempty"`
	Role        string `json:"role"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
	// PasswordHash is never returned via JSON
	PasswordHash string `json:"-"`
}

// ── /status response ────────────────────────────────────────────────────────

type StatusResponse struct {
	Version   string `json:"version"`
	Uptime    int64  `json:"uptime"`
	Memory    struct {
		LuaVMSize   int64 `json:"lua_vms_size"`
		WorkersCount int   `json:"workers_count"`
	} `json:"memory"`
	Database struct {
		Reachable bool `json:"reachable"`
	} `json:"database"`
	Server struct {
		TotalRequests   int64 `json:"total_requests"`
		ConnectionsActive int  `json:"connections_active"`
		ConnectionsAccepted int64 `json:"connections_accepted"`
	} `json:"server"`
	Workers []WorkerStatus `json:"workers"`
}

type WorkerStatus struct {
	Pid               int    `json:"pid"`
	MemoryLuaVMBytes int64  `json:"memory_lua_vm_bytes"`
	ConnectionsActive int    `json:"connections_active"`
	ConnectionsReading int   `json:"connections_reading"`
	ConnectionsWriting int   `json:"connections_writing"`
	ConnectionsWaiting int   `json:"connections_waiting"`
	TotalRequests    int64  `json:"total_requests"`
	Uptime            int64  `json:"uptime"`
}

var StartTime = time.Now()
