# SPEC-BUG-PROXY-NEWROUTE-503

## 背景
- **發現時間**: 2026-06-16 18:51 UTC（第五輪 QA）
- **現象**: 新建路由 proxy 轉發返回 503 `{"message":"no upstream target"}`
- **注意**: 此 bug 與 BUG-PROXY-SERVICE-NIL-UPSTREAM 不同（後者為 service.targets=nil，已修復）

## 目標
修復新建路由通過 gateway 轉發時返回 503 的問題

## Scope

### In-Scope
- config_sync.lua 的 service_lookup 解析 upstream_id 邏輯
- nginx.conf route matching + upstream target 解析邏輯
- 驗證新建路由 `/guardian-test/health` → 200

### Out-of-Scope
- 不改動已通過 QA 的現有路由轉發邏輯
- 不改動 alerter / webhook 邏輯

## 驗收標準

1. [ ] `POST /routes` 新建帶 service_id 的 route → 201
2. [ ] `GET /{new_route_path}/health` via gateway → 200
3. [ ] 確認 upstream_target 正確解析（非 nil）
4. [ ] Docker build --no-cache cont-proxy 成功
5. [ ] cont-proxy 容器 healthy
