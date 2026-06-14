# SPEC-INLINE-ROUTE: Nginx.conf Route Matching Inlining + upstream_id Fix

## 背景
`develop` 分支有 4 個未提交的實作變更，需要通過小黑 QA flow 才能 merge：
1. `nginx.conf` — 將 route matching + upstream selection 邏輯從 Lua module require 改為 inline `access_by_lua_block`，解決 OpenResty 1.29+ `pcall(require, "access")` yields across C-call boundary 問題
2. `admin-api/storage/store.go` — service queries 新增 `upstream_id` 欄位 fetch + 修復 `LIMIT 0` 問題（改為 `COALESCE(NULLIF($2, 0), 1000)`）
3. `proxy/lua/cont/access.lua` — jwt_validation 從 lazy load 改為直接 require，簡化程式碼
4. `proxy/lua/cont/config_sync.lua` — 日誌格式優化 + nil guard 增強

## 目標
確保 nginx.conf inline route matching 正確運作，通過 QA 驗證，merge 到 main。

## Scope
### In-scope
- nginx.conf: inline route matching + upstream selection in `access_by_lua_block`
- store.go: service upstream_id fetch + limit default fix
- access.lua: jwt_validation direct require
- config_sync.lua: logging improvements + nil guards
- Docker build --no-cache 成功
- All containers 正常啟動
- Basic proxy routing 功能正常

### Out-of-scope
- 不修改 plugin handler 邏輯
- 不修改 frontend
- 不做新的功能

## 驗收標準
1. `docker compose build --pull --no-cache proxy` 成功
2. `docker compose build --pull --no-cache cont-admin-api` 成功
3. `docker exec cont-proxy nginx -t` 通過
4. 所有 containers 正常啟動（proxy/admin-api/frontend/postgres/redis）
5. `GET /services` → 200（upstream_id 正確出現在 response）
6. `GET /` 或 proxy 路由 → 200/404（取決於是否有 matched route）
7. `docker compose ps` 顯示所有 containers healthy
