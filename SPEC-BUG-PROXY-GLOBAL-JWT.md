# SPEC-BUG-PROXY-GLOBAL-JWT

## 背景
- **Issue**: BUG-PROXY-GLOBAL-JWT (P0)
- **發現時間**: 2026-06-16 08:26 UTC
- **現象**: Proxy 全域路由（無任何 auth plugin）也回 401

## 根因分析
nginx.conf line 522-533 的 JWT 檢查邏輯：
```lua
local is_global = (not p.route_id and not p.service_id)
if is_global or p.route_id == matched_route.id or p.service_id == service_id then
    has_jwt = true
```
`is_global` 條件讓「global JWT plugin（無 route_id/service_id）」對**所有路由**強制執行 JWT。

目前 `/internal/config/snapshot` 返回 1 個 global JWT plugin（route_id=null, service_id=null），導致所有 route 都要求 JWT。

## 目標
修復 JWT enforcement 邏輯：當 plugin 為 global 時，不應對所有路由強制執行 JWT。

## Scope
### In-scope
- 修復 nginx.conf JWT enforcement 邏輯（access_by_lua_block）
- 移除 `is_global` 條件，只對明確 attached to route/service 的 plugin 執行 JWT
- Docker build --no-cache cont-proxy
- Restart cont-proxy container
- 驗證：新建無 plugin 的 route → 200

### Out-of-scope
- 不修改 Admin API 的 plugin 資料結構
- 不修改 JWT validation 本身邏輯

## 驗收標準
1. [ ] 新建 route（無任何 plugin）→ GET /{route}/health → 200
2. [ ] 已存在 route（無 JWT plugin）→ 不再回 401
3. [ ] 有 JWT plugin 的 route → 仍正常要求 JWT token
4. [ ] Docker build --no-cache cont-proxy 成功
5. [ ] cont-proxy container healthy
