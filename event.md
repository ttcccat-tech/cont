# Cont 2.0 Development Event Log

## Active Session
- Date: 2026-06-15
- PM: 小黑
- Scope: 2.0 用量追蹤 + 超限阻擋 + Webhook 可靠化

## Status
- 🔴 = blocked/bug 需要修
- 🟡 = 預計優化（也是正式待辦）
- ✅ = 完成
- ⏳ = in progress

## Tasks

### 🔴 SPEC-ANALYTICS-01 — Analytics Key Parsing Bug
- [✅] TASK-ANALYTICS-1: Fix `GetTopRoutesByUsage` key parsing — `parts[2]` → `parts[3]`
- [✅] TASK-ANALYTICS-2: Fix `GetTopConsumersByUsage` key parsing — `parts[2]` → `parts[3]`
- **小黑驗證**: code review `redis.go` lines 299, 357 — 兩處均已使用 `parts[3]`
- **小黑驗證**: Docker build --no-cache proxy ✅ admin-api ✅

### 🔴 SPEC-BLACKSCREEN-01 — Frontend 頁面修復
- [✅] TASK-1: Fix Users.tsx `d.map is not a function` (commit `e85ceb37`)
- [✅] TASK-2: Fix AlertRules.tsx blank page (commit `f55927a2`)
- [✅] TASK-3: Fix Billing.tsx route — import BillingPortal, use it instead of Settings
- [✅] TASK-4: /config-snapshots route — 已正確存在
- [✅] TASK-5: Fix ApiDocs.tsx load failure — `/docs.json` content-type 改為 `application/x-yaml`
- [✅] TASK-6: Sidebar 工作區導航 — 已正確連結到 `/workspaces`
- **小黑驗證**: curl localhost:18082/billing → 200 ✅
- **小黑驗證**: curl localhost:18082/config-snapshots → 200 ✅
- **小黑驗證**: curl localhost:18082/api-docs → 200 ✅ (Swagger UI loads)
- **小黑驗證**: curl localhost:18081/docs.json → swagger.yaml + Content-Type: application/x-yaml ✅

### 🟡 SPEC-usage-alerting — Usage Alerting（待處理）
- 6 tasks，見 SPEC-usage-alerting.md

### 🟡 SPEC-2.5-A — Analytics Dashboard 整合（待處理）
- Backend `GET /usage/analytics` endpoint
- Frontend Cont 用量 panel
- 見 SPEC-2.5-A.md

### 🟡 SPEC-2.5-B — Usage Quota Alerting（待處理）
- alerter.go 整合 usage quota check
- 見 SPEC-2.5-B.md

## ✅ 已完成（本輪 2026-06-15）

### 本輪 hotfix merge 到 main
- [✅] GetTopRoutesByUsage parts[3] — analytics key parsing fix
- [✅] GetTopConsumersByUsage parts[3] — analytics key parsing fix
- [✅] BillingPortal route — App.tsx /billing → BillingPortal
- [✅] ApiDocs /docs.json content-type — application/x-yaml

### 历史完成
- [✅] BUG-001: GetUser 500 INTERNAL_ERROR — `sql.NullString` 修補
- [✅] BUG-002: GetUser SELECT 缺少 `org_id` — 加入 `org_id`
- [✅] BUG-003: Route 轉發 404 — config sync 10s + chunked decode
- [✅] BUG-004: JWT 未強制執行 — access_by_lua_block inline JWT validation
- [✅] TASK-PLUGIN-4: Docker build --no-cache succeeds
- [✅] Users/Groups/Consumers/Upstreams/Plugins CRUD 全部通過
- [✅] Proxy forwarding /test-api/health → 200
- [✅] Auth 登入（JWT token）正常

## 成功標準（2.0）

- [✅] Admin API 所有端點正常（CRUD）
- [✅] Proxy 轉發正常
- [✅] 使用者可在 UI 完成所有操作
- [✅] JWT / Auth 流程正常
- [✅] `/api-docs` 正常顯示
- [ ] Load test 0% 錯誤率
- [ ] 用量寫入 Redis 每小時 counter
- [ ] `GET /usage/org/:id` 返回正確 JSON
- [ ] `GET /usage/consumer/:id` 返回正確 JSON
- [ ] Free plan 超限 → 429 + header
- [ ] 用量 80% → X-Usage-Warning header
- [ ] Webhook delivery 寫入 `webhook_deliveries` table
- [ ] Webhook 失敗自動重試（3次，指數回退）
- [ ] `GET /webhooks/:id/deliveries` 可查到送達歷史
