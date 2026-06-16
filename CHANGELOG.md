# Cont Changelog

All notable changes to the Cont API Gateway project are documented here.

## [2.1.0] — 2026-06-16

### Fixed

- **BUG-PROXY-502**: Proxy forwarding returned 502 Bad Gateway for newly created routes. Root cause: `nginx.conf` route priority logic used `>` instead of `>=`, causing the last-route-in-array to incorrectly win when priorities were equal. Fix: changed `priority > highest_priority` to `priority >= highest_priority` so the first matching route is selected at equal priority. Affects all routes with `priority=0` (default). Verified: `GET /test-api/health` → 200, upstream=`192.168.1.202:3010`.
- **BUG-PROXY-UPSTREAM-WRONG**: Proxy forwarded to wrong upstream (`final.com:80`). Root cause: `nginx.conf` route matching selected the wrong route when multiple routes had equal priority. Secondary fix: `config_sync.lua` converted services from array to dict keyed by `service.id` for O(1) lookup.
- **BUG-PROXY-500-LUA-FFI**: Proxy returned 500 Lua FFI crash — `attempt to get length of local 'targets' (a userdata value)`. Root cause: FFI cdata cannot use `#` operator in LuaJIT. Fix: replaced `#targets` with `next(targets)` to check non-empty table.
- **BUG-PROXY-GLOBAL-JWT**: Proxy enforced JWT on all routes globally (even routes with no auth plugin), returning 401 on routes that should be open. Root cause: `nginx.conf` `is_global = (not p.route_id and not p.service_id)` incorrectly flagged all routes as requiring JWT. Fix: removed `is_global` condition; JWT enforcement only applies to routes/services with explicitly attached JWT plugins.
- **BUG-JWT-CREDENTIAL-REGRESSION-3**: `POST /consumers/{id}/jwt/credentials` returned 404. Verified as QA endpoint error (should be `/jwt/credentials` not `/jwt`). Code was correct.
- **BUG-JWT-CREDENTIAL-REGRESSION-2**: JWT credential API regression (404). Root cause: container state issue; code correct. Fix: `docker compose build --no-cache cont-admin-api` + restart.
- **BUG-JWT-CREDENTIAL**: `POST /consumers/{id}/jwt/credentials` returned 404. Root cause: handler not registered. Fix: registered `/consumers/:id/jwt/credentials` route in `main.go`.
- **BUG-Services-Update**: `PUT /services/{id}` returned INTERNAL_ERROR. Root cause: `upstream_id` UUID validation + handler returned 400 for invalid upstream_id. Fix: store.go upstream_id UUID validation + routes.go handler returns 400 for invalid upstream_id.
- **BUG-Routes-Update**: `PUT /routes/{id}` returned INTERNAL_ERROR. Root cause: `store.go UpdateRoute` args/setClauses misalignment. Fix: completely rewritten args/setClauses alignment logic.
- **BUG-Upstreams-Update**: `PUT /upstreams/{id}` cleared the `name` field to empty string on partial update. Root cause: `name=$2` directly overwrote without COALESCE. Fix: `name=COALESCE(NULLIF($2,''), name)` — empty string preserves existing value.
- **BUG-REGRESSION-UE-1**: `POST /internal/usage/incr` returned success but Redis DBSIZE stayed 0. Root cause: all `pipe.Expire(ctx, key, 62*24*60*60)` missing `*time.Second` — value `62*24*60*60 = 5356800` interpreted as nanoseconds → go-redis truncated to 1 second TTL. Fix: `62*24*60*60*time.Second` (62 days), all 5 Expire calls patched.
- **BUG-CreateRoute-service_id-empty-string**: `POST /routes` (without service) returned INTERNAL_ERROR — `invalid input syntax for type uuid: ""`. Root cause: `store.go CreateRoute` passed empty string `""` for nil service_id, but PostgreSQL UUID column rejects empty string. Fix: when `service_id == ""`, pass `nil` (Go `interface{}`) so PostgreSQL receives NULL.
- **BUG-GetUser-500**: `GET /users/{id}` returned 500 INTERNAL_ERROR. Root cause: `last_login_at` was `NULL` in DB but scanned into `string` type (cannot be NULL). Fix: `sql.NullString` in store.go.
- **BUG-GetUser-missing-org_id**: `GET /users/{id}` returned 500. Root cause: SELECT query missing `org_id` column but Scan attempted to read it. Fix: added `org_id` to SELECT and Scan arguments.
- **BUG-ConfigSync-chunked**: Config sync failed to decode chunked transfer encoding, causing routes to not appear in proxy config. Fix: `nginx.conf` periodic timer now handles chunked responses from Admin API.
- **BUG-nginx-cont_trace_id-undeclared**: `ngx.var.cont_trace_id` was referenced but never declared, causing Lua errors. Fix: added `set $cont_trace_id '';` in nginx.conf http block.

### Added

- **SPEC-default-org-usage**: Default org (zero-UUID) usage tracking now correctly queries Redis `GetMonthlyUsage` instead of hardcoding `current_usage: 0`. `GetOrgUsage` now has a fast path for zero-UUID org that skips DB lookup. `UsageByTimeRange` fixed Redis string→int64 parsing + hour calculation bug.
- **SPEC-usage-alerting**: Alert rules now support `usage_quota` metric type. `evaluateUsageQuota()` integrated into `evaluateRule()`. `computeConsumerUsageQuotaMetric()` added to alerter. Alert fires at 80/90/100% thresholds.
- **SPEC-2.5-A**: Analytics Dashboard — `/usage/analytics` endpoint + Cont usage panel in frontend (progress bar, hourly trend, top entities).
- **SPEC-2.5-B**: Usage Quota Alerting — `evaluateUsageQuotas()`, fire at 80/90/100%, AlertHistory, webhook trigger.
- **SPEC-webhooks**: Webhook reliable delivery with v027 migration (`webhook_subscriptions` + `webhook_deliveries` tables). Worker pool size 10, exponential backoff 1s→5s→30s, max 3 attempts. `TriggerWebhook` + `FireWebhooks` integrated into alerter.
- **Prometheus metrics**: DB/Redis pool metrics exposed at `/metrics`. Pool metrics use polling Gauge approach for reliability.
- **OpenTelemetry tracing**: OTLP exporter support added for distributed tracing.
- **gRPC support**: Cont gRPC Protocol Support Phase 1 — `upstream_id/service_id` UUID type cast fix, X-GRPC-Web header forwarding.
- **Circuit Breaker**: Plugin state machine (CLOSED/OPEN/HALF_OPEN) implemented.
- **Plugin SDK**: Schema registry + worker sync for plugin management.
- **Plugin Gallery**: Frontend UI to browse/install plugin types from registry.
- **Database Migration System**: Versioned migration system with rollback support + CLI tool.
- **Webhook Deliveries Dashboard**: Frontend UI for webhook delivery history + Dead Letter Queue (DLQ) display.

### Changed

- **Route priority logic**: `nginx.conf` `priority >` → `priority >=` — first route wins at equal priority (previously last route won).
- **Services storage**: Converted from array to dict keyed by `service.id` for O(1) lookup in `config_sync.lua`.
- **Config sync interval**: Reduced from 30s to 10s for faster propagation of route/service changes.
- **JWT enforcement**: Only enforced on routes/services with explicitly attached JWT plugins (not globally).
- **Periodic sync**: Handles chunked transfer encoding from Admin API correctly.

### Verified (No Code Changes Required)

- **BUG-JWT-CREDENTIAL-REGRESSION-3**: QA used wrong endpoint (`/jwt` vs `/jwt/credentials`). Code correct.
- **BUG-JWT-CREDENTIAL-REGRESSION**: Container state issue, code correct. Resolved by rebuild.

---

## [2.0.0] — 2026-06-15

### Core Gateway

- Admin API (Go + PostgreSQL) on port 18081
- Proxy (OpenResty + Lua) on port 18000
- Frontend (Docker) on port 18082
- Redis for session/cache, PostgreSQL for persistent storage

### Features

- Users, Groups, Consumers, Upstreams, Services, Routes, Plugins CRUD
- JWT authentication with credential management per consumer
- Rate limiting, CORS, proxy cache, JWT, key-auth default plugins
- Usage tracking with hourly Redis counters
- Free plan quota enforcement (429 when exceeded)
- `X-Usage-Warning` header at 80%, `X-Plan-Quota-Limit/Remaining` headers
- Webhook delivery with retry (exponential backoff, 3 attempts)
- `/api-docs` OpenAPI documentation endpoint
- Config sync from Admin API to proxy every 10s

### Bug Fixes (2.0 era)

- IncrUsage Redis write silent failure (TTL bug)
- GetUser 500 (NULL scanning)
- GetUser missing org_id
- Services/Upstreams/Routes partial update field clearing
- JWT credential API registration
