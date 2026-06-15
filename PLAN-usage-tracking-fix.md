# PLAN: usage-tracking-fix

## Tasks

- [ ] TASK-UTF-1: Fix access.lua TASK-UC-5 — add org_id to /internal/usage/incr request (routes/usage.go IncrUsage already handles JSON body; access.lua needs to send org_id in JSON body or query)
- [ ] TASK-UTF-2: Docker build --no-cache cont-proxy
- [ ] TASK-UTF-3: Restart cont-proxy container
- [ ] TASK-UTF-4: Manual test — POST /internal/usage/incr with org_id, verify Redis key exists
- [ ] TASK-UTF-5: QA verify GET /usage/org/:id returns 200 + valid JSON
- [ ] TASK-UTF-6: Push to develop branch
