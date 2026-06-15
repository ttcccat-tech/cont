# SPEC-usage-tracking-fix

## 背景

`GET /usage/org/:id` 和 `GET /usage/consumer/:id` endpoint 已實作（main.go:339-340），但 API 返回 `total: 0`，且 Redis 無任何 `cont:usage:*` key。

根因：TASK-UC-5 的 `ngx.location.capture_multi` 沒有傳 `org_id`，且只送 query args 而非 JSON body。

## 目標

修復 proxy Lua 的用量追蹤 call，使 Redis 每小時 counter 能正確寫入。

## Scope

### In-scope
- 修復 `access.lua` TASK-UC-5：加入 `org_id` 到 `/internal/usage/incr` 請求
- 確保 `/internal/usage/incr` 能接收並處理請求（query args 或 JSON body）
- 驗證 Redis 出現 `cont:usage:{org_id}:{YYYYMMDDHH}` key
- 驗證 `GET /usage/org/:id` 返回非零 total（或至少 handler 正常運作）

### Out-of-scope
- 不改 `IncrementUsageDetailed` 邏輯（已存在）
- 不新增 API endpoint（已存在）
- 不實作 Free plan 超限 429 阻擋（另一個 issue）

## 驗收標準

1. `curl http://localhost:18081/usage/org/{org_id}` 返回 200 + 正確 JSON schema（`org_id`, `plan`, `total`, `limit`, `usage[]`）
2. `curl http://localhost:18081/usage/consumer/{consumer_id}` 返回 200 + 正確 JSON schema（`consumer_id`, `org_id`, `total`, `usage[]`）
3. 手動觸發 `POST /internal/usage/incr`（帶 org_id）後，Redis 出現 `cont:usage:{org_id}:{YYYYMMDDHH}` key，TTL > 0
4. Docker build --no-cache cont-proxy 成功
5. 重啟後 container 正常運行
