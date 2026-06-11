# cont — API Gateway

Cloud-native API Gateway with Kong-compatible Admin API and Proxy API, built on OpenResty (NGINX + lua-nginx-module) and Go.

## Quick Start

```bash
# Start all services
make up

# Check status
curl http://localhost:8000/status

# Prometheus metrics
curl http://localhost:8000/metrics

# Admin API (Kong-compatible)
curl http://localhost:8001/services
```

## Default Credentials

| User | Password | Role |
|------|----------|------|
| admin | admin123 | admin |
| user | (set at creation) | editor |

> **Note:** Admin password is set via `bcrypt` hash in the database. Default password `admin123` is used for initial setup. Change it after first login.

## Architecture

```
Browser/Client
      │
      ▼
┌─────────────────────────────────────┐
│   OpenResty Proxy (port 8000)        │
│   Lua Plugin Chain                   │
│   Route → Service → Upstream → Target│
└─────────────────────────────────────┘
      │                    │
      ▼                    ▼
┌─────────────┐     ┌──────────┐
│ Admin API   │     │ Upstream │
│ Go+Gin 8001 │     │ Targets  │
└─────────────┘     └──────────┘
      │                    │
      ▼                    ▼
┌─────────────┐     ┌──────────┐
│ PostgreSQL  │     │  Redis   │
│ (config)    │     │(rate limit│
└─────────────┘     │ /cache)  │
                    └──────────┘
```

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Proxy Engine | OpenResty (NGINX + lua-nginx-module) |
| Admin API | Go 1.22 + Gin |
| Config Store | PostgreSQL 16 |
| Cache/Rate Limit | Redis 7 |
| Metrics | Prometheus (Kong-compatible) |

## Development

```bash
# Build admin API
make admin-build

# Run admin API locally (requires Go 1.22+)
make admin-run

# View logs
make logs
```

## Admin API Endpoints

Full Kong Admin API v1 compatibility:

```
GET|POST   /services
GET|PUT|DELETE /services/:id
GET|POST   /routes
GET|PUT|DELETE /routes/:id
GET|POST   /upstreams
GET|PUT|DELETE /upstreams/:id
GET|POST   /upstreams/:id/targets
GET|PUT|DELETE /upstreams/:id/targets/:target_id
GET|POST   /consumers
GET|PUT|DELETE /consumers/:id
GET|POST   /plugins
GET|PUT|DELETE /plugins/:id
GET        /status
GET        /metrics
```

## License

BSD 2-Clause / NGINX Compatible
