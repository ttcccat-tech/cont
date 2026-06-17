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

### 🔴 BUG-PROXY-NIL-UPSTREAM: Proxy 轉發 503（upstream_target 為空）（P0）— ✅ 已修補（2026-06-17）
- **小黑分析**（2026-06-17）:
  - 根因：`nginx.conf` 的 `proxy_pass http://$cont_upstream` 中的 `$cont_upstream` 無 scheme 前綴
  - 真正問題：container image 早於 fix commit，container 跑的還是舊程式碼
- **修補**:
  - Commit `c79a7320`: `proxy_pass $cont_upstream`（移除硬編碼 `http://` 前綴，由 Lua 動態添加 `http://`）
  - TASK-PX-FIX-1: rebuild cont-proxy image → ✅ 3fc7ca025b8b（新 image）
  - TASK-PX-FIX-2: restart container → ✅
  - TASK-PX-FIX-3: 驗證 `curl -H "Host: qa.example.com" http://localhost:18000/qa-test-route/headers` → **200 OK** ✅
- **小黑判定**: ✅ FIXED — proxy forwarding 正常，`proxy_pass $cont_upstream` + Lua `ngx.var.cont_upstream = "http://" .. target` 模式已驗證可行

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
| 9 | Proxy 轉發鏈路 | ✅ 200（via qa-test-route + qa.example.com Host header）（2026-06-17） |
| 10 | JWT Credential Create | ✅ 通過 |
