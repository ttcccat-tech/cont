# SPEC-usage-counter: Redis Hourly Usage Counter

## 背景
`SPEC-v2.0` 要求每次 Proxy 請求寫入 Redis，格式：`cont:usage:{org_id}:{YYYYMMDDHH}` → increment counter。

目前 `rate-limiting-advanced/handler.lua` 的 plan-quota 檢查依賴 `current_usage`（來自 `/internal/plan-quota/:consumer_id`），但背後的 Redis counter 尚未實作，導致 `current_usage` 永遠是 0，超限阻擋完全失效。

## 目標
- 每次 Proxy 請求寫入 Redis sorted set 或 string counter
- Key 格式：`cont:usage:{org_id}:{YYYYMMDDHH}`（每小時一分桶）
- 提供 `IncrUsage(org_id, consumer_id, route_id, service_id, latency, status_code)` 函數
- `admin-api` 提供 `GET /usage/org/:id` 和 `GET /usage/consumer/:id`

## Scope
### In-scope
- `proxy/lua/cont/access.lua`：每次請求尾端呼叫 `incr_usage()`
- `proxy/lua/cont/plugins/rate-limiting-advanced/handler.lua`：呼叫 `GET /internal/usage/incr` 寫入 Redis
- `admin-api/routes/usage.go`：`POST /internal/usage/incr` endpoint
- `admin-api/storage/usage.go`：`IncrUsage()` 寫入 Redis (`HSET` + `EXPIRE`)
- `admin-api/routes/usage.go`：`GET /usage/org/:org_id` 查詢每小時 counter
- `admin-api/routes/usage.go`：`GET /usage/consumer/:consumer_id` 查詢消費者用量
- Redis key：`cont:usage:{org_id}:{YYYYMMDDHH}`（string counter）

### Out-of-scope
- 不實作 Prometheus/Grafana 匯出（預計在 2.1）
- 不實作 Prometheus format endpoint

## 驗收標準
1. `POST /internal/usage/incr` 成功寫入 Redis，回傳 `{success: true}`
2. `GET /usage/org/:org_id` 返回该 org 當天每小時 counter 陣列
3. `GET /usage/consumer/:consumer_id` 返回该 consumer 的用量
4. Docker build `--no-cache` admin-api 成功
5. Docker build `--no-cache` cont-proxy 成功
6. containers 重啟後 Redis counter 依然遞增

## Tasks

### TASK-UC-1: storage/usage.go — IncrUsage Redis write
- 完成定義：`IncrUsage(ctx, org_id, consumer_id, route_id, service_id, latency, status_code)` 寫入 `HSET cont:usage:{org_id}:{YYYYMMDDHH} {consumer_id}:{route_id} {json}` 並 `EXPIRE 86400`
- File: `admin-api/storage/usage.go` (new)

### TASK-UC-2: routes/usage.go — POST /internal/usage/incr endpoint
- 完成定義：`POST /internal/usage/incr` 接收 JSON body，呼叫 `IncrUsage()`，回傳 `{"success":true}`
- File: `admin-api/routes/usage.go`

### TASK-UC-3: routes/usage.go — GET /usage/org/:org_id endpoint
- 完成定義：`GET /usage/org/:org_id` 查詢 Redis，返回 `{"org_id":"X","date":"YYYY-MM-DD","hourly":[{"hour":10,"count":42},...]}` 格式

### TASK-UC-4: routes/usage.go — GET /usage/consumer/:consumer_id endpoint
- 完成定義：`GET /usage/consumer/:consumer_id` 返回该 consumer 的用量摘要

### TASK-UC-5: access.lua — 每次請求呼叫 incr usage
- 完成定義：在 `access.lua` 請求處理尾端（`access_by_lua_block` 结束时），呼叫 `ngx.location.capture("/__cont_api_internal__/internal/usage/incr", {..})`，不阻擋主要請求路徑

### TASK-UC-6: Docker build --no-cache admin-api
- 完成定義：`docker compose build --no-cache admin-api` 成功

### TASK-UC-7: Docker build --no-cache cont-proxy
- 完成定義：`docker compose build --no-cache cont-proxy` 成功

### TASK-UC-8: Containers restart + smoke test
- 完成定義：兩個 container 重啟，Redis counter 在新請求後遞增
