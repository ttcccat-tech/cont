# PLAN-BUG-Proxy-503-NewRoute

## Bug
- **ID**: BUG-Proxy-503-NewRoute
- **API**: GET /new_route/health via Gateway
- **現象**: 新建 upstream + service + route chain → 503
- **小黑驗證**: 2026-06-17 06:02 UTC — confirm 503

## Root Cause Analysis（小黑親自分析）
- 根因見 SPEC-BUG-PROXY-SERVICE-NIL-UPSTREAM.md
- 問題: targetsMap[upstream_id] = nil (not []) → Lua next(nil) fails
- 修復策略: 兩處都修 (Go + Lua)
- 已知 fix commits 存在 (4b682b33 等)，但 container 是 Jun16 21:08 build，fix commits 在 Jun17
- **需在最新 develop 上驗證並 rebuild**

## Tasks

### 步驟 1: 確認 Proxy targetsMap 修復存在於 develop
- [ ] TASK-PX-1: 確認 routes/routes.go GetProxyRuntimeConfig targetsMap 初始化
  - 預期: `targetsMap[u.ID] = []ProxyTarget{}` for empty targets
  - 驗證: `grep -n "targetsMap\[u.ID\]" admin-api/routes/routes.go`
- [ ] TASK-PX-2: 確認 nginx.conf upstream nil guard
  - 預期: `type(cont.targets[svc.upstream_id]) ~= "nil"` guard
  - 驗證: `grep -n "type(cont.targets" proxy/nginx.conf`

### 步驟 2: Rebuild containers
- [ ] TASK-PX-3: Docker build --no-cache cont-proxy
  - 完成定義: `docker compose build --no-cache cont-proxy` exit_code=0
- [ ] TASK-PX-4: Restart cont-proxy container
  - 完成定義: `docker compose up -d cont-proxy` + `docker ps` shows Up

### 步驟 3: 驗證 Proxy 轉發修復
- [ ] TASK-PX-5: Smoke test — 新建 upstream + service + route → 200
  - 完成定義: `curl http://localhost:18000/qa_fwd_*/health` → 200
- [ ] TASK-PX-6: Regression — existing /test-api/health → 200
  - 完成定義: `curl http://localhost:18000/test-api/health` → 200

## 驗收標準（對應 SPEC-BUG-Proxy-NewRoute-503.md）
1. POST /services（透過 upstream_id）→ 201
2. POST /routes（關聯 service）→ 201
3. GET /new-route/health via Gateway → 200（轉發至正確 upstream）
4. 已存在路由 → 不 regression，維持 200
