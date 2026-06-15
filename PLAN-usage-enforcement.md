# PLAN: usage-enforcement

## Atomic Tasks

- [ ] **TASK-UE-1**: Fix IncrUsage Redis write (root cause investigation + fix + verify)
  - 對應驗收標準: #1 Redis key exists after POST /internal/usage/incr

- [ ] **TASK-UE-2**: Docker build --no-cache admin-api + restart
  - 對應驗收標準: #5 admin-api container healthy

- [ ] **TASK-UE-3**: Docker build --no-cache cont-proxy + restart
  - 對應驗收標準: #6 cont-proxy container healthy

- [ ] **TASK-UE-4**: Smoke test full enforcement flow
  - 對應驗收標準: #1 #2 #3 #4 (POST /internal/usage/incr → Redis, plan-quota current_usage, 429 blocking, 80% warning)

## Task Details

### TASK-UE-1: Fix IncrUsage Redis write
**Dev Agent** must:
1. Investigate why `POST /internal/usage/incr` returns success but Redis DBSIZE=0
2. Check inside running container: does `/app/cont-admin-api` have IncrUsage compiled in? (`strings /app/cont-admin-api | grep IncrUsage`)
3. Check Redis connectivity from inside container (`wget -qO- http://localhost:8001/internal/usage/incr --post-data='{"org_id":"debug"}'` → check Redis KEYS)
4. If IncrUsage function exists but Redis write fails, examine `storage/usage.go` pipeline execution
5. Fix the root cause (likely: IncrUsage not being called, or Redis pipeline not committing)
6. Commit fix: `TASK-UE-1: fix IncrUsage Redis write`

**Completion Definition**: `curl -X POST http://localhost:18081/internal/usage/incr -H "Content-Type: application/json" -d '{"org_id":"test-ue","route_id":"r1","service_id":"s1","latency_ms":1,"status_code":200}'` → `docker exec cont-redis redis-cli DBSIZE` returns >= 1

### TASK-UE-2: Docker build --no-cache admin-api
**Dev Agent** must:
1. `cd /var/repo/cont && docker compose build --no-cache cont-admin-api`
2. `docker compose up -d cont-admin-api`
3. Wait for healthy: `docker ps --format "{{.Names}} {{.Status}}" | grep cont-admin-api | grep healthy`
4. Commit: `TASK-UE-2: docker build --no-cache admin-api`

**Completion Definition**: Container `cont-admin-api` shows `(healthy)` and `docker compose build` log shows `Image cont-cont-admin-api Built`

### TASK-UE-3: Docker build --no-cache cont-proxy
**Dev Agent** must:
1. `cd /var/repo/cont && docker compose build --no-cache cont-proxy`
2. `docker compose up -d cont-proxy`
3. `docker exec cont-proxy nginx -t`
4. Commit: `TASK-UE-3: docker build --no-cache cont-proxy`

**Completion Definition**: `nginx: configuration file test is successful` and container `cont-proxy` is Up

### TASK-UE-4: Smoke test full enforcement flow
**QA Agent** must verify:
1. `POST /internal/usage/incr` → Redis DBSIZE >= 1
2. `GET /internal/plan-quota/default` → 200 + JSON with `current_usage` field
3. 80% warning test: manually set usage scenario → `X-Usage-Warning` header appears
4. 429 blocking test: mock over-limit scenario → `ngx.status = 429` + `X-RateLimit-Limit-Reached: true`
5. Commit: `TASK-UE-4: usage enforcement smoke tests passed`
