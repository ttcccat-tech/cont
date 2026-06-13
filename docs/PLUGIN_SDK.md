# Cont Plugin SDK

This document describes how to develop custom plugins for the Cont API Gateway.

## Plugin Architecture

Cont plugins are Lua modules that run in the OpenResty/nginx request lifecycle:

```
access_by_lua (authentication + route matching)
    ↓
access phase (per-plugin .access())
    ↓
upstream target selection
    ↓
pre_proxy phase (per-plugin .pre_proxy()) [optional]
    ↓
proxy_to_upstream
    ↓
log phase (per-plugin .log()) [optional]
```

## Plugin Registry

All available plugins and their schemas are registered at:

```
GET /__cont_api_internal__/internal/plugin-registry
```

Response:
```json
{
  "plugins": [
    {
      "name": "rate-limiting-advanced",
      "version": "0.2.0",
      "label": "Advanced Rate Limiting",
      "description": "...",
      "access_phase": true,
      "log_phase": false,
      "pre_proxy": false,
      "config_schema": { ... }
    }
  ]
}
```

## Built-in Plugins

| Name | Access | Log | PreProxy | Description |
|------|--------|-----|----------|-------------|
| `rate-limiting-advanced` | ✅ | ❌ | ❌ | Redis sliding window + local fallback |
| `proxy-cache-advanced` | ✅ | ✅ | ❌ | Redis/local response caching |
| `circuit-breaker` | ✅ | ❌ | ✅ | CLOSED/OPEN/HALF_OPEN state machine |
| `usage-tracking` | ❌ | ✅ | ❌ | Request count/latency tracking |
| `rate-limiting-basic` | ✅ | ❌ | ❌ | Simple shared-dict rate limiting |

## Writing a Custom Plugin

### 1. Create the plugin directory

```
proxy/lua/cont/plugins/<my-plugin>/handler.lua
```

### 2. Implement the handler module

```lua
-- plugins/my-plugin/handler.lua
local _M = { version = "0.1.0" }

-- Optional: access phase handler
-- Called after route matching and auth checks
function _M.access(self, plugin)
    local cfg = plugin.config or {}
    local limit = cfg.limit or 100

    -- ngx.ctx.authenticated_consumer_id is set if consumer is authenticated
    local consumer_id = ngx.ctx.authenticated_consumer_id
    local identifier = consumer_id or ngx.var.remote_addr

    -- ... your logic ...

    if over_limit then
        ngx.header["Retry-After"] = "60"
        ngx.header["Content-Type"] = "application/json"
        ngx.status = 429
        ngx.say('{"message":"Rate limit exceeded","error":"Too Many Requests","statusCode":429}')
        return ngx.exit(429)
    end
end

-- Optional: pre_proxy callback
-- Called after upstream target is selected, before proxy
-- Receives the plugin instance and upstream_id
function _M.pre_proxy(self, plugin, upstream_id)
    local cfg = plugin.config or {}
    -- ... your logic ...
end

-- Optional: log phase handler
-- Called after the response is received from upstream
function _M.log(self, plugin)
    local cfg = plugin.config or {}
    local latency = ngx.ctx.request_start_time
        and (ngx.now() * 1000 - ngx.ctx.request_start_time)
        or 0
    -- ... your logic ...
end

return _M
```

### 3. Register the plugin schema (backend)

Add to `admin-api/storage/plugin_registry.go` in `BuiltInPlugins()`:

```go
{
    Name:        "my-plugin",
    Version:     "0.1.0",
    Label:       "My Custom Plugin",
    Description: "Does something useful",
    AccessPhase: true,
    ConfigSchema: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "limit": map[string]interface{}{
                "type":        "integer",
                "minimum":     1,
                "default":     100,
                "description": "requests per minute",
            },
        },
    },
},
```

### 4. Enable the plugin via Admin API

```bash
# Create a plugin instance
POST /plugins
{
  "name": "my-plugin",
  "route": { "id": "<route-id>" },
  "config": {
    "limit": 50
  },
  "enabled": true
}
```

## Plugin Config Object

When a plugin is active, `plugin.config` contains the JSON config from the database:

```lua
function _M.access(self, plugin)
    local cfg = plugin.config or {}
    local my_setting = cfg.my_setting or "default"
end
```

The `plugin` object also has:
- `plugin.id` — unique plugin instance ID
- `plugin.name` — plugin type name (e.g. `rate-limiting-advanced`)
- `plugin.route_id` — attached route ID (or nil)
- `plugin.service_id` — attached service ID (or nil)
- `plugin.consumer_id` — attached consumer ID (or nil)

## Context Variables (ngx.ctx)

These are set by access.lua before plugin access() runs:

| Variable | Type | Description |
|----------|------|-------------|
| `ngx.ctx.authenticated_consumer_id` | string | authenticated consumer UUID |
| `ngx.ctx.authenticated_user_id` | string | authenticated user UUID |
| `ngx.ctx.request_start_time` | number | request start time in ms (set in access phase) |
| `ngx.ctx.matched_route` | table | matched route object |
| `ngx.ctx.route_id` | string | matched route ID |
| `ngx.ctx.service_id` | string | target service ID |
| `ngx.ctx.upstream_target` | string | resolved upstream target (host:port) |
| `ngx.ctx.grpc_web` | bool | true if gRPC-Web request |

## Plugin Execution Order

Plugins run in the order they appear in `_G.cont.plugins` (sorted by `created_at`).
Auth plugins (jwt, key-auth, basic-auth, hmac-auth) are run separately before other plugins.

## Shared Memory

Cont proxy exposes these shared dicts (defined in nginx.conf):

| Dict Name | Purpose |
|-----------|---------|
| `cont_circuit_breaker_config` | CB config per upstream (JSON) |
| `cont_circuit_breaker_state` | CB state counters per upstream |
| `cont_rate_limit` | local rate limit counters |

## Internal Admin API (proxy → backend)

Proxy uses these internal endpoints (no auth, called via ngx.location.capture):

| Endpoint | Purpose |
|----------|---------|
| `GET /__cont_api_internal__/internal/plugins` | list enabled plugin instances |
| `GET /__cont_api_internal__/internal/plugin-registry` | plugin schemas |
| `GET /__cont_api_internal__/internal/config/snapshot` | full runtime config |
| `GET /__cont_api_internal__/internal/circuit-breaker-configs` | CB configs |
| `GET /__cont_api_internal__/internal/plan-quota/:id` | consumer quota |
| `GET /__cont_api_internal__/internal/validate-cred/:type/:key` | credential validation |
| `GET /__cont_api_internal__/internal/validate-jwt/:token` | JWT validation |
