# SPEC-usage-enforcement: Usage Enforcement + 80% Warning

## 背景

2.0 核心功能之一：**超限阻擋**（Free plan 達到 quota 時回 429）與 **80% warning header**。

目前 `rate-limiting-advanced/handler.lua` 的 Plan Quota Check（lines 162-206）邏輯正確，但依賴 `GET /internal/plan-quota/:consumer_id` 返回的 `current_usage`。若 `current_usage` 永遠是 0（因為 Redis counter 未正確寫入），則超限阻擋完全失效。

另外，`access.lua` 的 `POST /internal/usage/incr` 呼叫存在，但 **`POST /internal/usage/incr` 返回 success 但 Redis 沒有寫入**（Redis DBSIZE=0）。

## 目標

1. 確保每次 Proxy 請求的用量寫入 Redis counter（`cont:usage:{org_id}:{YYYYMMDDHH}`）
2. `GET /internal/plan-quota/:consumer_id` 的 `current_usage` 能正確從 Redis 計算當月總用量
3. Free plan 超限（`current_usage >= request_limit`）→ 429 + `X-RateLimit-Limit-Reached: true`
4. 用量 80% → `X-Usage-Warning` header

## Scope

### In-scope
- 確認 `POST /internal/usage/incr` 正確寫入 Redis（`IncrUsage` function in `storage/usage.go`）
- `storage/usage.go` → `IncrUsage()` 確認 pipeline 正確執行，Redis 有 keys
- `routes/routes.go` → `GetPlanQuota()` 正確查詢 Redis 當月用量（呼叫 `GetMonthlyUsage`）
- `handler.lua` 的 80% warning（line 188-190）已存在且正確
- `handler.lua` 的 429 超限阻擋（line 192-204）已存在且正確
- Docker build --no-cache admin-api + cont-proxy
- Containers 重啟後依然正常

### Out-of-scope
- 不實作新的 plan quota 表（已存在 plans table）
- 不實作 billing cycle 计算（不在 2.0 範圍）
- 不實作 Webhook retry（單獨的 issue）

## 驗收標準

1. `POST /internal/usage/incr` 後 Redis 出現 `cont:usage:{org_id}:{YYYYMMDDHH}` key，`DBSIZE > 0`
2. `GET /internal/plan-quota/:consumer_id` 返回的 `current_usage` 非零（如果有用量記錄）
3. Free plan 超限（current_usage >= 1000）→ `ngx.status = 429` + `X-RateLimit-Limit-Reached: true`
4. 用量 >= 80% → `X-Usage-Warning: XX%` header 出現在 response
5. Docker build --no-cache admin-api 成功
6. Docker build --no-cache cont-proxy 成功
7. Containers 重啟後功能正常

## Tasks

### 🔴 TASK-UE-1: Fix IncrUsage Redis write (root cause) — ✅ FIXED 2026-06-16
- **完成定義**: `POST /internal/usage/incr` 後 Redis DBSIZE > 0，且 `cont:usage:{org_id}:{YYYYMMDDHH}` key 存在
- **根因分析**: `storage/usage.go` 的 IncrUsage 代碼正確，但 `docker compose` 啟動的 container 名稱是 `cont-admin-api-test`（覆寫了 docker-compose.yml 的 `container_name: cont-admin-api`），導致 `docker compose up cont-admin-api` 啟動了另一個 instance，真正的 service 從未重啟
- **修補**:
  1. `docker stop cont-admin-api-test && docker rm cont-admin-api-test` — 移除舊 container
  2. `docker compose up -d cont-admin-api` — 以正確名稱啟動 service
  3. 驗證 `POST /internal/usage/incr` → Redis 出現 `cont:usage:test-final:2026061517` key ✅
  4. Docker build `--no-cache` 成功 ✅
- **小黑驗證**: Redis DBSIZE > 0，KEY 存在 ✅

### ✅ TASK-UE-2: Verify GetPlanQuota current_usage (inherited)
- `handler.lua` line 178: `local current_usage = tonumber(data.current_usage) or 0`
- 依賴 TASK-UE-1 的 Redis counter 正確寫入
- 當用量 counter 正確時，GetPlanQuota 的 GetMonthlyUsage 會正確計算

### ✅ TASK-UE-3: 80% Warning header (already implemented)
- `handler.lua` lines 186-190 已實作 X-Usage-Warning
- 需要 TASK-UE-1 完成後才能實際觸發（current_usage 需要非零）

### ✅ TASK-UE-4: 429 Over-limit blocking (already implemented)
- `handler.lua` lines 192-204 已實作 429 + header
- 需要 TASK-UE-1 完成後才能實際觸發

### ✅ TASK-UE-5: Docker build --no-cache admin-api — ✅ DONE 2026-06-16
- **完成定義**: `docker compose build --no-cache cont-admin-api` 成功，container restart 後 healthy

### TASK-UE-6: Docker build --no-cache cont-proxy
- **完成定義**: `docker compose build --no-cache cont-proxy` 成功，container restart 後 healthy

### TASK-UE-7: Smoke test full flow
- **完成定義**:
  1. `POST /internal/usage/incr` → Redis 有 key
  2. `GET /internal/plan-quota/default` → `current_usage > 0`（after step 1）
  3. Free plan 超限 scenario 測試（mock current_usage = 1000, request_limit = 1000）→ 429
