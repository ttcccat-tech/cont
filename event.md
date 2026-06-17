# Cont QA Event Log — 2026-06-17

## QA Summary
- **Date**: 2026-06-17
- **Admin API**: http://localhost:18081
- **Gateway**: http://localhost:18000
- **Tester**: Hermes Agent (cron)

## 🔴 Bug 記錄

### 🔴 BUG-SERVICES-UPDATE: Services Update 返回 500（P0） — 🔴 QA驗證失敗（2026-06-17）
- **API**: PUT /services/{id}
- **預期**: 200
- **實際**: 500 Internal Server Error
- **原因**: org_id WHERE clause 需加 COALESCE（fix 已 commit 在 store.go 但容器未重建）
- **修補方向**: docker compose build --no-cache cont-admin-api && 重啟容器
- **驗證**: QA Phase 6.4
- **嚴重程度**: P0（功能阻斷 — Service 無法更新）
- **驗證記錄**: curl PUT 返回 INTERNAL_ERROR (500)
- **待做**: 重建 cont-admin-api 容器使 fix 生效

### 🔴 BUG-ROUTES-UPDATE: Routes Update 返回 500（P0）
- **API**: PUT /routes/{id}
- **預期**: 200
- **實際**: 500 Internal Server Error
- **原因**: fix 已 commit (c0a36e4f) 但容器未重建
- **修補方向**: 重建 cont-admin-api 容器使 UpdateRoute COALESCE fix 生效
- **驗證**: QA Phase 7.4
- **嚴重程度**: P0（功能阻斷 — Route 無法更新）
- **待做**: docker compose build --no-cache cont-admin-api && 重啟容器

### 🔴 BUG-PROXY-NIL-UPSTREAM: Proxy 轉發 503（新建路由 upstream targets 為 nil）（P0）
- **API**: GET /{route_path}/health via Gateway
- **預期**: 200（proxy 到 192.168.1.202:3010）
- **實際**: 503 → 404（路由創建失敗，container 重啟後仍無效）
- **原因**: PX-2 fix 已 commit (b59d5a98, Lua guard) 但 PX-1 Go fix 未 commit 且容器未重建
- **修補方向**: 補 commit PX-1 Go fix → docker build --no-cache cont-proxy && 重啟容器
- **驗證**: QA Phase 9.3 — 🔴 FAIL（2026-06-17）
- **嚴重程度**: P0（功能阻斷 — 新建路由無法轉發流量）
- **待做**: 確認 PX-1 Go fix 是否已 applied，未有的話補上再重建 proxy

## ✅ 通過項目

| Phase | 功能 | 狀態 |
|-------|------|------|
| 1 | Auth 登入 | ✅ 通過 |
| 2 | Users CRUD | ✅ 通過 |
| 3 | Groups CRUD | ✅ 通過 |
| 4 | Consumers CRUD | ✅ 通過 |
| 5 | Upstreams CRUD | ✅ 通過 |
| 6 | Services Create/List/Get/Delete | ✅ 通過（Update 500 除外）|
| 7 | Routes Create/List/Get/Delete | ✅ 通過（Update 500 除外）|
| 8 | Plugins CRUD | ✅ 全部通過 |
| 9 | Proxy 轉發鏈路 | 🔴 503（已知 nil upstream bug）|
| 10 | JWT Credential Create | ✅ 通過 |
