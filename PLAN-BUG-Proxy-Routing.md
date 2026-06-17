# PLAN-BUG-Proxy-Routing

## Tasks

- [ ] TASK-BPR-1: Rebuild cont-admin-api container (docker compose build --pull --no-cache cont-admin-api)
- [ ] TASK-BPR-2: Rebuild cont-proxy container (docker compose build --pull --no-cache cont-proxy)
- [ ] TASK-BPR-3: Restart both containers
- [ ] TASK-BPR-4: Verify CreateTarget rejects empty target (POST /upstreams/{id}/targets with target:"")
- [ ] TASK-BPR-5: Verify UpdateTarget rejects empty target (PUT /upstreams/{id}/targets/{target_id} with target:"")
- [ ] TASK-BPR-6: Clean up existing empty target records in DB
- [ ] TASK-BPR-7: Verify proxy routing works end-to-end (GET /{route}/health → 200)
