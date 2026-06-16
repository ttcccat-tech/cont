# PLAN-BUG-PROXY-UPSTREAM-WRONG

## Tasks
- [ ] TASK-UPSTREAM-FIX-1: Fix nginx.conf route priority logic (`priority >` → `priority >=`)
- [ ] TASK-UPSTREAM-FIX-2: Docker build --no-cache cont-proxy
- [ ] TASK-UPSTREAM-FIX-3: Restart cont-proxy container
- [ ] TASK-UPSTREAM-FIX-4: Smoke test — `GET /test-api/health` → 200, upstream = `192.168.1.202:3010`
