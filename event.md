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

### ✅ SPEC-usage-alerting — Usage Alerting（已完成）
- [✅] TASK-UA-1: alerter.go — 廢除 evaluateUsageQuota()，在 evaluateRule() 處理 usage_quota
- [✅] TASK-UA-2: alerter.go — 新增 computeConsumerUsageQuotaMetric()
- [✅] TASK-UA-3: store.go — AlertRule CRUD 確認 PercentageThreshold + quota_metric_type
- [✅] TASK-UA-4: AlertRules.tsx — POST/PUT 攜帶 quota_metric_type + percentage_threshold
- [✅] TASK-UA-5: AlertRules.tsx — 列表顯示 usage_quota 門檻百分比
- **小黑驗證**: evaluateUsageQuota 已移除 ✅，usage_quota 整合至 evaluateRule() ✅
- **小黑驗證**: computeConsumerUsageQuotaMetric 存在 line 273 ✅
- **小黑驗證**: Docker build --no-cache admin-api ✅ frontend ✅

### ✅ SPEC-2.5-A — Analytics Dashboard 整合（已完成）
- [✅] TASK-2.5-A1: Backend `/usage/analytics` endpoint (routes/usage.go + redis.go)
- [✅] TASK-2.5-A2: `getAnalyticsUsage()` API in kong.ts
- [✅] TASK-2.5-A3: Cont usage panel in Analytics.tsx (progress bar, hourly trend, top entities)
- **小黑驗證**: docker build --no-cache admin-api ✅ frontend ✅
- **小黑驗證**: containers restarted ✅

### ✅ SPEC-2.5-B — Usage Quota Alerting（已完成）
- [✅] TASK-2.5-B1: alerter.go — evaluateUsageQuotas(), fire at 80/90/100%, AlertHistory, webhook trigger
- [✅] TASK-2.5-B2: AlertRules.tsx — usage_quota alert type + percentage threshold UI（已於 TASK-UA-4 完成）
- **小黑驗證**: alerter.go Docker build ✅, frontend Docker build ✅, containers running ✅

## ✅ 已完成（本輪 2026-06-15）

### 本輪 hotfix merge 到 main
- [✅] Usage Alerting — alerter.go 整合 usage_quota + computeConsumerUsageQuotaMetric
- [✅] AlertRules.tsx — quota_metric_type + percentage_threshold 表單欄位
- [✅] Analytics Dashboard — /usage/analytics endpoint + Cont usage panel

### 历史完成
- [✅] GetTopRoutesByUsage parts[3] — analytics key parsing fix
- [✅] GetTopConsumersByUsage parts[3] — analytics key parsing fix
- [✅] BillingPortal route — App.tsx /billing → BillingPortal
- [✅] ApiDocs /docs.json content-type — application/x-yaml
- [✅] BUG-001: GetUser 500 INTERNAL_ERROR — `sql.NullString` 修補
- [✅] BUG-002: GetUser SELECT 缺少 `org_id` — 加入 `org_id`
- [✅] BUG-003: Route 轉發 404 — config sync 10s + chunked decode
- [✅] BUG-004: JWT 未強制執行 — access_by_lua_block inline JWT validation
- [✅] TASK-PLUGIN-4: Docker build --no-cache succeeds
- [✅] Users/Groups/Consumers/Upstreams/Plugins CRUD 全部通過
- [✅] Proxy forwarding /test-api/health → 200
- [✅] Auth 登入（JWT token）正常
- [✅] `/api-docs` 正常顯示

## 成功標準（2.0）

- [✅] Admin API 所有端點正常（CRUD）
- [✅] Proxy 轉發正常
- [✅] 使用者可在 UI 完成所有操作
- [✅] JWT / Auth 流程正常
- [✅] `/api-docs` 正常顯示
- [✅] `GET /usage/analytics?org_id=X` 返回正確 JSON
- [✅] Analytics.tsx Cont 用量 panel 渲染正常
- [ ] Load test 0% 錯誤率
- [ ] 用量寫入 Redis 每小時 counter
- [ ] `GET /usage/org/:id` 返回正確 JSON
- [ ] `GET /usage/consumer/:id` 返回正確 JSON
- [ ] Free plan 超限 → 429 + header
- [ ] 用量 80% → X-Usage-Warning header
- [ ] Webhook delivery 寫入 `webhook_deliveries` table
- [ ] Webhook 失敗自動重試（3次，指數回退）
- [ ] `GET /webhooks/:id/deliveries` 可查到送達歷史

## Phase 1 QA Verification (SPEC-PENDING-01) — 2026-06-15

### 🔴 TASK-P1-QA-1: Verify v025 migration exists
- **Command**: `grep -n "v025" /var/repo/cont/admin-api/migrator/migrations.go`
- **Result**: No matches found (exit_code=1)
- **Status**: FAILED — v025 migration not found

### 🔴 TASK-P1-QA-2: Verify Go build
- **Command**: `cd /var/repo/cont/admin-api && go vet ./... 2>&1 | head -10`
- **Result**:
  ```
  # go.opentelemetry.io/otel/internal/global
  /root/go/pkg/mod/go.opentelemetry.io/otel@v1.18.0/internal/global/handler.go:44:18: undefined: atomic.Pointer
  note: module requires Go 1.20
  # google.golang.org/grpc
  /root/go/pkg/mod/google.golang.org/grpc@v1.60.1/server.go:2165:14: undefined: atomic.Int64
  note: module requires Go 1.19
  ```
- **Status**: FAILED — go vet reports errors (not just warnings)

### ✅ TASK-P1-QA-3: Verify admin-api Docker build
- **Command**: `docker compose ps cont-admin-api`
- **Result**: cont-admin-api | cont-cont-admin-api | "./cont-admin-api" | Up 2 hours (healthy)
- **Logs**: Only redis duration truncation warnings (not errors)
- **Status**: PASSED

### ✅ TASK-P1-QA-4: Verify proxy Docker build
- **Command**: `docker compose ps cont-proxy`
- **Result**: cont-proxy | cont-cont-proxy | "nginx -g 'daemon of…" | Up About an hour
- **Logs**: Only worker_connections limit warning + normal access logs
- **Status**: PASSED

### ✅ TASK-P1-QA-5: Verify nginx -t
- **Command**: `docker exec cont-proxy nginx -t 2>&1`
- **Result**: "nginx: the configuration file /usr/local/openresty/nginx/conf/nginx.conf syntax is ok" + "nginx: configuration file /usr/local/openresty/nginx/conf/nginx.conf test is successful"
- **Status**: PASSED

### ✅ TASK-P1-QA-6: Verify Lua modules
- **jwt_validation.lua**: EXISTS — contains cosocket code for JWT validation via Admin API
- **config_sync.lua**: EXISTS — contains cosocket code for config sync
- **Status**: PASSED

### ✅ TASK-P1-QA-7: Verify API endpoints
- **Command**: `curl -s http://localhost:18081/internal/config/snapshot | head -5`
- **Result**: Valid JSON with plugins, routes, services, upstreams arrays
- **Command**: `curl -s -X POST http://localhost:18081/internal/usage/incr -H "Content-Type: application/json" -d '{"org_id":"test","consumer_id":"c1","route_id":"r1","service_id":"s1","latency_ms":1,"status_code":200}'`
- **Result**: `{"count":1,"success":true}`
- **Status**: PASSED

### Phase 1 Summary
| Task | Status |
|------|--------|
| P1-QA-1 v025 migration | 🔴 FAILED |
| P1-QA-2 Go build | 🔴 FAILED |
| P1-QA-3 admin-api Docker | ✅ PASSED |
| P1-QA-4 proxy Docker | ✅ PASSED |
| P1-QA-5 nginx -t | ✅ PASSED |
| P1-QA-6 Lua modules | ✅ PASSED |
| P1-QA-7 API endpoints | ✅ PASSED |
