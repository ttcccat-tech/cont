# cont — API Gateway Specification

## Overview

cont is a Kong-compatible API Gateway built on OpenResty (NGINX + lua-nginx-module) with a Go-based Admin API. Goal: full Admin API and Proxy API compatibility with Kong, allowing the existing KongStack frontend (`/kong/*` routes) to work without modification.

## Architecture

```
                    ┌──────────────────────────────────┐
                    │          cont Gateway             │
                    │  ┌────────────────────────────┐   │
Browser / Client ──►│  │   OpenResty (proxy port)    │   │
                    │  │   Lua Plugin Chain          │   │
                    │  │   Route → Service          │   │
                    │  │   → Upstream → Target      │   │
                    │  └────────────────────────────┘   │
                    │  ┌────────────────────────────┐   │
                    │  │   Go Admin API (port 8001)  │   │
                    │  │   Kong Admin API compatible │   │
                    │  └────────────────────────────┘   │
                    └──────────────────────────────────┘
                              │           │
                         Postgres       Redis
                    (config store)  (rate limit,
                                     session cache)
```

## Data Model

### Entities

- **Service**: `id, name, protocol, host, port, path, url, retries, connect_timeout, read_timeout, write_timeout, enabled`
- **Route**: `id, name, service_id, protocols[], hosts[], paths[], methods[], strip_path, preserve_host, regex_priority, https_redirect_status_code, connection_timeout, enabled`
- **Upstream**: `id, name, algorithm (roundrobin|leastconnections|weighted-ip-hash), slots, healthchecks, enabled`
- **Target**: `id, upstream_id, target (ip:port), weight, enabled`
- **Consumer**: `id, username, custom_id, enabled`
- **Plugin**: `id, name, route_id, service_id, consumer_id, config{}, enabled`

### Kong API Compatibility Target

Full compatibility with Kong Admin API v1 (Kong 3.x):

```
GET|POST   /services
GET|PUT|DELETE /services/:id
GET|POST   /routes
GET|PUT|DELETE /routes/:id
GET|POST   /upstreams
GET|PUT|DELETE /upstreams/:id
GET|POST   /upstreams/:upstream_id/targets
GET|PUT|DELETE /upstreams/:upstream_id/targets/:target
GET|POST   /consumers
GET|PUT|DELETE /consumers/:id
GET|POST   /plugins
GET|PUT|DELETE /plugins/:id
GET|POST   /certificates
GET|POST   /snis
GET        /status
GET        /metrics (Prometheus)
GET|POST   /workspaces
```

### Proxy Flow

```
client request
    │
    ▼
[lua-nginx-module: rewrite]
    │  URL normalization, method handling
    ▼
[lua-nginx-module: access]
    │  1. Route matching (by host/path/method)
    │  2. Plugin chain (per-plugin access())
    │  3. Load balancer → select Target
    ▼
[NGINX upstream] ──────────────────────────────────────► upstream target
    │                                                      │
    ▼                                                      │
[lua-nginx-module: header_filter] ◄────────────────────  upstream response
    │  Plugin header_filter() chains
    ▼
[lua-nginx-module: body_filter]
    │  Plugin body_filter() chains
    ▼
[lua-nginx-module: log]
    │  Plugin log() chains, metrics collection
    ▼
client response
```

### Plugin Hooks (in OpenResty phases)

| Phase | Description | NGINX directive |
|-------|-------------|-----------------|
| `init` | Process start (once) | init_by_lua |
| `init_worker` | Each worker spawns | init_worker_by_lua |
| `rewrite` | URL transformation | rewrite_by_lua |
| `access` | Auth, rate-limit check, route resolve | access_by_lua |
| `header_filter` | Modify response headers | header_filter_by_lua |
| `body_filter` | Stream response body | body_filter_by_lua |
| `log` | Logging, metrics | log_by_lua |

### Load Balancing Algorithms

- `roundrobin` — weighted round-robin (default)
- `leastconnections` — least connected
- `weighted-ip-hash` — consistent hashing by client IP

### Health Checks

- Active: HTTP/TCP probes to upstream targets
- Passive: Observe request results (timeouts, errors)
- Automatic target enable/disable based on health

## Technology Stack

| Component | Technology | Reason |
|-----------|-----------|--------|
| Proxy Engine | OpenResty (NGINX + lua-nginx-module) | Request processing + Lua plugin execution |
| Admin API | Go + Gin/Echo | High-performance HTTP, type-safe |
| Config Store | PostgreSQL | Persistent storage for routes/services/etc. |
| Cache/Session | Redis | Rate limit counters, upstream cache, LMDB replacement |
| Metrics | Prometheus | /metrics endpoint compatible with Kong Prometheus plugin |
| Declarative Config | cont.yml | Kong.yml compatible declarative format |

## License

BSD 2-clause (OpenResty/NGINX compatible) — or proprietary commercial license.
