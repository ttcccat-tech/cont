# SPEC-BUG-PROXY-SERVICE-NIL-UPSTREAM

## Background
- **Bug**: BUG-PROXY-SERVICE-NIL-UPSTREAM (P0)
- **發現時間**: 2026-06-16 18:51 UTC
- **嚴重程度**: P0（Proxy 轉發功能阻斷）

## Root Cause（小黑確認）
1. Admin API `store.CreateService` 將 `upstream_id` 正確寫入 PostgreSQL ✅
2. `GET /services/{id}` 返回正確 `upstream_id` ✅
3. **真正根因**：`routes/routes.go` `GetProxyRuntimeConfig` 中的 `targetsMap` 構建邏輯
   - 當 `ListTargetsByUpstream` 返回 0 rows 時，`targetsMap[upstream.ID]` **從未被 set**
   - Go JSON marshal 對 map 中不存在的 key → `null` 而非 `[]`
   - 在 Lua 中，`cont.targets[upstream_id]` = `null`（lightuserdata nil）
   - `next(nil)` 在 Lua 中失敗（非 absent key），導致 condition 為 false
   - 結果：Service 有 upstream_id 但條件失敗，fallback 到 `service.host`（靜態 host）
   - 若 service.host 也是靜態 host 而非正確 upstream，導致 502

## Fix Strategy
**兩處都修**：
1. **Go (store.go)**：確保 `targetsMap[upstream.ID] = []ProxyTarget{}`（empty array not null）
2. **Lua (nginx.conf)**：增加 `type(cont.targets[svc.upstream_id]) ~= "nil"` guard

## Scope
### In-scope
- `routes/routes.go` `GetProxyRuntimeConfig` targetsMap 初始化
- `nginx.conf` upstream target selection 的 nil check
- Docker build --no-cache cont-proxy
- Smoke test：新建 service+upstream+route → 200

### Out-of-scope
- 不修改 store.CreateService（DB 寫入已正確）
- 不修改其他 Admin API 邏輯

## Acceptance Criteria
1. `GET /internal/config/snapshot` 的 `targets` map 中，所有 upstream_id 都對應陣列（即使是 `[]` 而非 `null`）
2. Lua condition `next(cont.targets[svc.upstream_id])` 對 empty array 返回 false（Lua `next({}) = nil`）
3. 新建 Service（via upstream_id）+ Route → `GET /qa_fwd_*/health` → 200（非 502）
4. 現有 `/test-api/health` → 200（regression check）
