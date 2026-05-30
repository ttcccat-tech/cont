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

type Route struct {
	ID                     string   `json:"id"`
	Name                   string   `json:"name,omitempty"`
	ServiceID              string   `json:"service,omitempty"`
	Protocols              []string `json:"protocols,omitempty"`
	Hosts                  []string `json:"hosts,omitempty"`
	Paths                  []string `json:"paths,omitempty"`
	Methods                []string `json:"methods,omitempty"`
	StripPath              bool     `json:"strip_path"`
	PreserveHost           bool     `json:"preserve_host"`
	RegexPriority          int      `json:"regex_priority,omitempty"`
	HTTPSRedirectStatusCode int     `json:"https_redirect_status_code,omitempty"`
	ConnectionTimeout      int      `json:"connection_timeout,omitempty"`
	Enabled                bool     `json:"enabled"`
	CreatedAt              string   `json:"created_at,omitempty"`
	UpdatedAt              string   `json:"updated_at,omitempty"`
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

type Plugin struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	RouteID    string          `json:"route,omitempty"`
	ServiceID  string          `json:"service,omitempty"`
	ConsumerID string          `json:"consumer,omitempty"`
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
