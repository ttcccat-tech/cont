# PLAN-BUG-PROXY-UPSTREAM-WRONG

## Tasks
- [ ] TASK-UPSTREAM-1: Inspect config_sync.lua upstream_id → target resolution logic
- [ ] TASK-UPSTREAM-2: Identify why upstream_id resolution fails for test-api route
- [ ] TASK-UPSTREAM-3: Fix upstream_id resolution (if code issue) or verify proper registration
- [ ] TASK-UPSTREAM-4: Docker build --no-cache cont-proxy
- [ ] TASK-UPSTREAM-5: Restart cont-proxy container
- [ ] TASK-UPSTREAM-6: Smoke test — GET /test-api/health via Gateway → 200 with correct upstream
