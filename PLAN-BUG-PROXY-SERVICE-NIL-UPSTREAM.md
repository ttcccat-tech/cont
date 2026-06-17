# PLAN: BUG-PROXY-SERVICE-NIL-UPSTREAM

## Tasks

- [ ] TASK-PX-1: Fix Go — routes/routes.go GetProxyRuntimeConfig: targetsMap[upstream.ID] = []ProxyTarget{} when no targets
- [ ] TASK-PX-2: Fix Lua — nginx.conf: add type(cont.targets[svc.upstream_id]) ~= "nil" guard before next()
- [ ] TASK-PX-3: Docker build --no-cache cont-proxy
- [ ] TASK-PX-4: Restart cont-proxy container
- [ ] TASK-PX-5: QA — create new service+upstream+route → GET /qa_fwd_*/health → 200
- [ ] TASK-PX-6: QA — existing /test-api/health → 200 (regression)
