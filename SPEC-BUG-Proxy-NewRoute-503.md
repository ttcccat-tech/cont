# SPEC-BUG-Proxy-NewRoute-503

## 背景
- **發現時間**: 2026-06-16 15:13 UTC
- **嚴重程度**: P0（功能阻斷 — 新建路由無法轉發）
- **API**: GET /{new_route_path}/health via Gateway
- **預期**: 200（轉發到 upstream 192.168.1.202:3010）
- **實際**: 503 {"message":"no upstream target"}

## 根因（初步分析）
nginx.conf access_by_lua DEBUG log 顯示 `upstream_target=nil` — config_sync.lua 同步新建 Service 時未攜帶 upstream_id

## Scope
### In-scope
- 修復新建 Service + Route 後 proxy 轉發 503 的問題
- 確認 upstream_id → targets 解析鏈完整

### Out-of-scope
- 不改動現有已正常運作的路由
- 不修改 upstream CRUD 邏輯

## 驗收標準
- [ ] POST /services（透過 upstream_id）→ 201
- [ ] POST /routes（關聯 service）→ 201
- [ ] GET /new-route/health via Gateway → 200（轉發至正確 upstream）
- [ ] 已存在路由 → 不 regression，維持 200
