# SPEC-BUG-PROXY-UPSTREAM-503 — Proxy 503 Service Unavailable

## Background
- **發現時間**: 2026-06-17 01:18 UTC
- **API**: GET /{route_path}/health via Gateway (port 18000)
- **預期**: 200 OK
- **實際**: 503 Service Unavailable

## Root Cause（小黑初步分析）
- QA 提及「已知根因 — GetProxyRuntimeConfig 的 targetsMap 當 upstream 無 targets 時 map entry 為 nil」
- 這與 2026-06-16 修復的 BUG-PROXY-SERVICE-NIL-UPSTREAM 相同症狀
- 可能是 regression（develop branch 有新 commit 未 merge 到 main）或
  某些 upstream 根本沒有 targets（targetsMap entry 從未被創建）

## 驗證計畫
1. 先確認 `/test-api/health` 是否真的 503（重現 bug）
2. 檢查 `GET /internal/config/snapshot` 的 targets map
3. 確認 routes/routes.go:1375 的 `targetsMap[u.ID] = []ProxyTarget{}` 是否仍在
4. 如果是 regression，確認 nginx.conf 的 nil guard 是否仍在

## Scope
### In-scope
- 確認 targetsMap 初始化邏輯存在且正確
- 確認 nginx.conf nil guard 存在且正確
- 如果 regression，重新修復並驗證

### Out-of-scope
- 不改 Lua 邏輯（只做 nil guard 檢查）
- 不改 Go targetsMap 初始化邏輯（只需確認存在）

## Acceptance Criteria
1. `curl http://localhost:18000/test-api/health` → 200 OK
2. `GET /internal/config/snapshot` targets map 無 nil entries（全部是 [] 而非 null）
3. Docker build --no-cache cont-proxy successful
4. Container healthy

## Tasks
- [ ] TASK-503-FIX-1: 確認 targetsMap 初始化邏輯存在（routes/routes.go:1375）
- [ ] TASK-503-FIX-2: 確認 nginx.conf nil guard 存在
- [ ] TASK-503-FIX-3: 重新 build + 重啟 cont-proxy
- [ ] TASK-503-FIX-4: Smoke test — `GET /test-api/health` → 200