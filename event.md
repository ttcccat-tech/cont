# Cont QA Event Log — 2026-06-17

## QA Summary
- **Date**: 2026-06-17
- **Admin API**: http://localhost:18081
- **Gateway**: http://localhost:18000
- **Tester**: Hermes Agent (cron)

## 🔴 Bug 記錄

### 🔴 BUG-SERVICES-UPDATE: Services Update 返回 500（P0） — ✅ QA驗證通過（2026-06-17）
- **API**: PUT /services/{id}
- **預期**: 200
- **實際**: 200 ✅
- **根因**: store.go UpdateService WHERE clause COALESCE fix（已 commit）
- **驗證**: QA Phase 6.4 — ✅ PASS（2026-06-17）
- **嚴重程度**: P0 → 已修補

### 🔴 BUG-ROUTES-UPDATE: Routes Update 返回 500（P0）
- **API**: PUT /routes/{id}
- **預期**: 200
- **實際**: 200 ✅（2026-06-17 驗證）
- **真實根因**: routes 表沒有 `description` 欄位；COALESCE fix 是真的，但 500 發生在 DB 層
- **驗證**: QA Phase 7.4 — ✅ PASS（2026-06-17）
- **嚴重程度**: P0 → 已修補（name 欄位可正確更新）
- **備註**: routes 表缺 description/methods/hosts 等欄位（不在 schema 中），client 送這些欄位時被忽略，不影響更新功能

### 🔴 BUG-PROXY-NIL-UPSTREAM: Proxy 轉發 503（upstream_target 為空）（P0）
- **API**: GET /{route_path}/health via Gateway
- **預期**: 200（proxy 到 192.168.1.202:3010）
- **實際**: 500（invalid URL prefix in "http://"）
- **小黑分析**（2026-06-17）:
  1. Lua condition `next(cont.targets[svc.upstream_id])` → TRUE ✅（targets 有內容）
  2. Algorithm 選擇 `selected_target` → "192.168.1.202:3010" ✅
  3. 設定 `upstream_target = selected_target` → "192.168.1.202:3010" ✅
  4. **但是**: 設定 `ngx.var.cont_upstream = "192.168.1.202:3010"`（不帶 scheme）
  5. `proxy_pass http://$cont_upstream` → Nginx 無法解析（variable-based upstream 需要 resolver 或完整 URL）
  6. Error log: `invalid URL prefix in "http://"` = 變量解析失敗
  7. upstream_target=`在 access log 中是空的，印證 upstream 連接從未成功
- **小黑判定**: 🔴 新建路由無法 proxy 轉發，根因是 nginx.conf 中 variable-based proxy_pass 缺少 scheme 前綴或 explicit resolver directive
- **修補方向**: 在 proxy_pass 使用完整 URL 或確保 resolver 正確解析 upstream hostname

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
