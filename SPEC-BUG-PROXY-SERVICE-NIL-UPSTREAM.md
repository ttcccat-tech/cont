# SPEC-BUG-PROXY-SERVICE-NIL-UPSTREAM

## Background
- **Bug**: BUG-PROXY-SERVICE-NIL-UPSTREAM (P0)
- **發現時間**: 2026-06-16 18:51 UTC
- **嚴重程度**: P0（Proxy 轉發功能阻斷）

## Root Cause（小黑已確認）
1. Admin API `store.CreateService` 將 `upstream_id` 正確寫入 PostgreSQL ✅
2. `GET /services/{id}` 返回正確 `upstream_id` ✅
3. **問題在**: `config_sync.lua` 從 PostgreSQL 讀取 Service 時，未正確攜帶 `upstream_id` 到 proxy config
4. `config_sync.lua` 的 `service_lookup` 階段：service_found=true 但 upstream_id=nil
5. 結果：proxy fallback 到 `host="routeexample.com"`（靜態設定），導致 502 Bad Gateway

## Objective
修復 config_sync.lua 中 service upstream_id 的同步邏輯，確保新建 Service 的 upstream_id 能正確從 PostgreSQL 同步到 proxy config。

## Scope
### In-scope
- `config_sync.lua` 的 service 同步邏輯
- 確認 upstream_id 從 DB 到 proxy config 的完整路徑
- Docker build --no-cache cont-proxy
- Smoke test：新建 `/qa_fwd_*/health` → 200

### Out-of-scope
- 不修改 store.go（DB 寫入已正確）
- 不修改 Admin API handler

## Acceptance Criteria
1. `config_sync.lua` 讀取 Service 時，`upstream_id` 欄位正確讀取
2. Service 同步到 proxy config dict 時，`upstream_id` 存在於 service 物件
3. `GET /services/{id}` 在 config snapshot 中返回正確 upstream_id（非 nil）
4. 新建 Service + Route 後，`GET /qa_fwd_*/health` → 200（非 502）
5. 現有 `/test-api/health` → 200（regression check）
