package storage

import "encoding/json"

// BuiltInPlugins returns the registry of all built-in Cont plugins with their schemas.
// This is used by GET /internal/plugin-registry to inform the proxy which plugins
// are available and how they should be invoked.
func BuiltInPlugins() []PluginSchema {
	return []PluginSchema{
		{
			Name:        "rate-limiting-advanced",
			Version:     "0.2.0",
			Label:       "Advanced Rate Limiting",
			Description: "Redis sliding window + local fallback rate limiting with plan quota enforcement",
			AccessPhase: true,
			ConfigSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"minute":        map[string]interface{}{"type": "integer", "minimum": 0, "description": "requests per minute (0=disabled)"},
					"hour":          map[string]interface{}{"type": "integer", "minimum": 0, "description": "requests per hour (0=disabled)"},
					"day":           map[string]interface{}{"type": "integer", "minimum": 0, "description": "requests per day (0=disabled)"},
					"second":        map[string]interface{}{"type": "integer", "minimum": 0, "description": "requests per second (0=disabled)"},
					"burst":         map[string]interface{}{"type": "integer", "minimum": 0, "description": "burst allowance"},
					"policy":        map[string]interface{}{"type": "string", "enum": []interface{}{"local", "redis"}, "default": "local", "description": "storage backend"},
					"redis_host":    map[string]interface{}{"type": "string", "default": "cont-redis"},
					"redis_port":    map[string]interface{}{"type": "integer", "default": 6379},
					"redis_password": map[string]interface{}{"type": "string"},
					"redis_database": map[string]interface{}{"type": "integer", "default": 0},
					"redis_timeout": map[string]interface{}{"type": "integer", "default": 500, "description": "connect timeout in ms"},
				},
			},
		},
		{
			Name:        "proxy-cache-advanced",
			Version:     "0.1.0",
			Label:       "Advanced Proxy Cache",
			Description: "Redis/local in-memory response caching with X-Cache-Status headers",
			AccessPhase: true,
			LogPhase:    true,
			ConfigSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"ttl":          map[string]interface{}{"type": "integer", "minimum": 1, "default": 300, "description": "cache TTL in seconds"},
					"memory_cache": map[string]interface{}{"type": "string", "enum": []interface{}{"redis", "local"}, "default": "local"},
					"redis_host":   map[string]interface{}{"type": "string", "default": "cont-redis"},
					"redis_port":   map[string]interface{}{"type": "integer", "default": 6379},
					"redis_password": map[string]interface{}{"type": "string"},
					"redis_database": map[string]interface{}{"type": "integer", "default": 0},
					"cache_by":     map[string]interface{}{"type": "string", "enum": []interface{}{"status", "all"}, "default": "status", "description": "which responses to cache"},
					"status_codes": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "integer"}, "default": []interface{}{200, 301, 404}},
				},
			},
		},
		{
			Name:        "circuit-breaker",
			Version:     "0.1.0",
			Label:       "Circuit Breaker",
			Description: "Upstream health-driven circuit breaker with CLOSED/OPEN/HALF_OPEN state machine",
			AccessPhase: true,
			PreProxy:    true,
			ConfigSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"trip_threshold":        map[string]interface{}{"type": "integer", "minimum": 1, "default": 5, "description": "consecutive failures to trip"},
					"recovery_timeout":      map[string]interface{}{"type": "integer", "minimum": 1, "default": 30, "description": "seconds before half-open probe"},
					"half_open_max_requests": map[string]interface{}{"type": "integer", "minimum": 1, "default": 3},
					"half_open_success_rate": map[string]interface{}{"type": "number", "minimum": 0, "maximum": 100, "default": 50, "description": "% success to close"},
				},
			},
		},
		{
			Name:        "usage-tracking",
			Version:     "0.1.0",
			Label:       "Usage Tracking",
			Description: "Tracks request count/latency per consumer for billing and quota",
			LogPhase:    true,
			ConfigSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"report_interval": map[string]interface{}{"type": "integer", "minimum": 1, "default": 3600, "description": "aggregation interval in seconds"},
				},
			},
		},
		{
			Name:        "rate-limiting-basic",
			Version:     "0.1.0",
			Label:       "Basic Rate Limiting",
			Description: "Simple local shared-dict rate limiting (lightweight alternative)",
			AccessPhase: true,
			ConfigSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"minute": map[string]interface{}{"type": "integer", "minimum": 0, "default": 100},
					"hour":   map[string]interface{}{"type": "integer", "minimum": 0, "default": 1000},
					"day":    map[string]interface{}{"type": "integer", "minimum": 0, "default": 10000},
				},
			},
		},
	}
}

// GetPluginSchema returns the schema for a named plugin, or nil if not found
func GetPluginSchema(name string) *PluginSchema {
	for _, p := range BuiltInPlugins() {
		if p.Name == name {
			return &p
		}
	}
	return nil
}

// MarshalJSON implements json.Marshaler so Plugin.Schema is populated from registry
func (p Plugin) MarshalJSON() ([]byte, error) {
	type Alias Plugin
	schema := GetPluginSchema(p.Name)
	return json.Marshal(&struct {
		*Alias
		Schema *PluginSchema `json:"schema,omitempty"`
	}{
		Alias:  (*Alias)(&p),
		Schema: schema,
	})
}
