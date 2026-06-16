# SPEC-default-org-usage: Default Org Usage Tracking + Anonymous Quota Enforcement

## 背景

2.0 系統有兩種請求：
1. **Authenticated requests**：有 JWT token，consumer 有 org_id
2. **Anonymous requests**：無 JWT token，org_id = `00000000-0000-0000-0000-000000000000`（zero UUID）

目前 `POST /internal/usage/incr` 可以正確寫入 default org 的 Redis counter（`cont:usage:00000000...:YYYYMMDDHH`），但 two 個關鍵 API 失敗：

1. **`GET /internal/plan-quota/default`** → hardcoded `current_usage: 0`，從不查詢 Redis
   - 導致 Anonymous requests 永遠不會觸發 429 超限阻擋
   - Free plan 超限完全失效

2. **`GET /usage/org/00000000-0000-0000-0000-000000000000`** → 404 NOT_FOUND
   - 因為 `store.GetOrganization()` 查不到 zero-UUID 的記錄
   - Analytics dashboard 無法顯示 default org 的用量

## 目標

1. `GET /internal/plan-quota/default` 的 `current_usage` 從 Redis 即時計算（呼叫 `GetMonthlyUsage`）
2. `GET /usage/org/:org_id` 對 zero-UUID default org 回傳真實 Redis 用量（不走 DB lookup）
3. Anonymous requests 在 Free plan 超限時正確回 429 + header
4. 用量 80% 時正確出現 `X-Usage-Warning` header

## Scope

### In-scope
- `routes/routes.go` → `GetDefaultPlanQuota`：對 zero-UUID org 呼叫 `GetMonthlyUsage`
- `routes/usage.go` → `GetOrgUsage`：對 zero-UUID org 跳過 DB lookup，直接查 Redis
- `GET /internal/plan-quota/default` 應該與 `GetPlanQuota` 邏輯一致（除了不回傳 consumer-specific 資料）

### Out-of-scope
- 不修改 IncrUsage 邏輯（已正確）
- 不修改 GetPlanQuota（已正確）
- 不實作新的 storage 函數

## 驗收標準

1. `POST /internal/usage/incr` 寫入 `org_id=00000000...` → Redis 有 key ✅（已存在）
2. `GET /internal/plan-quota/default` → `current_usage > 0`（在寫入後）
3. `GET /usage/org/00000000-0000-0000-0000-000000000000` → 200 + 正確 JSON（不走 DB）
4. Anonymous request（無 auth）達到 1000 請求 → 429 + `X-RateLimit-Limit-Reached: true`
5. Anonymous request 用量 >= 80% → `X-Usage-Warning` header 出現
6. Docker build --no-cache admin-api 成功
7. Docker build --no-cache cont-proxy 成功
8. Containers 重啟後依然正常

## Tasks

### TASK-DU-1: Fix GetDefaultPlanQuota to query Redis for current_usage
- **File**: `admin-api/routes/routes.go`
- **完成定義**: `GetDefaultPlanQuota` 對 zero-UUID org 呼叫 `store.Redis().GetMonthlyUsage(ctx, "00000000-0000-0000-0000-000000000000")`，而非 hardcoded 0
- **具體修改**:
  - 將 `GetDefaultPlanQuota` 的 `current_usage` 從 hardcoded `0` 改為呼叫 `store.Redis().GetMonthlyUsage(ctx, "00000000-0000-0000-0000-000000000000")`
  - 保持 `request_limit: 1000`, `plan_name: "free"` 不變

### TASK-DU-2: Fix GetOrgUsage for zero-UUID default org
- **File**: `admin-api/routes/usage.go`
- **完成定義**: `GetOrgUsage` 遇到 `orgID == "00000000-0000-0000-0000-000000000000"` 時，跳過 `store.GetOrganization()` DB lookup，直接查 Redis
- **具體修改**:
  - 在 `GetOrgUsage` 的 org lookup 段落，加入 `if orgID == "00000000-0000-0000-0000-000000000000"` 的快速路徑
  - 設 `orgPlan = "default"`, `plan = &Plan{RequestLimit: 1000, ...}`

### TASK-DU-3: Docker build --no-cache admin-api
- **完成定義**: `docker compose build --no-cache cont-admin-api` 成功，container restart 後 healthy

### TASK-DU-4: Docker build --no-cache cont-proxy
- **完成定義**: `docker compose build --no-cache cont-proxy` 成功，container restart 後 healthy

### TASK-DU-5: Smoke test — default org quota + usage
- **完成定義**:
  1. `POST /internal/usage/incr` 寫入 `org_id=00000000...` → Redis 有 key
  2. `GET /internal/plan-quota/default` → `current_usage > 0`
  3. `GET /usage/org/00000000-0000-0000-0000-000000000000` → 200 + valid JSON
  4. Anonymous proxy request (no JWT) → 200 (under limit)
