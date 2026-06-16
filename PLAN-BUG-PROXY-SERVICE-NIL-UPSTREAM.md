# PLAN-BUG-PROXY-SERVICE-NIL-UPSTREAM

## Tasks

- [ ] TASK-FIX-1: Fix routes/routes.go targetsMap — initialize empty array when no targets
  - In GetProxyRuntimeConfig, after the for loop building targetsMap, initialize any nil entries to []
  - Acceptance: `GET /internal/config/snapshot` returns `[]` not `null` for upstreams without targets

- [ ] TASK-FIX-2: Fix nginx.conf — add nil type check in upstream target selection
  - In nginx.conf line 566, change condition to also check `type(cont.targets[service.upstream_id]) ~= "nil"`
  - Acceptance: `next(nil)` is never called in the upstream selection logic

- [ ] TASK-FIX-3: Docker build --no-cache cont-proxy
  - Build the cont-proxy container with --no-cache
  - Acceptance: Docker build succeeds without errors

- [ ] TASK-FIX-4: Restart cont-proxy container
  - docker stop cont-proxy && docker rm cont-proxy && docker run -d ...
  - Acceptance: Container is healthy and running

- [ ] TASK-FIX-5: Smoke test — new service+upstream+route → 200
  - Create new upstream, target, service (via upstream_id), route
  - GET /qa_fwd_*/health → 200
  - Acceptance: 200 response, not 502

- [ ] TASK-FIX-6: Regression check — existing /test-api/health → 200
  - GET /test-api/health → 200
  - Acceptance: 200 response
