# PLAN: SPEC-UPDATE-ORGID-COHALESCE

## Tasks

- [ ] TASK-UOC-1: Fix UpdateService WHERE clause COALESCE NULLIF (store.go line 129)
- [ ] TASK-UOC-2: Docker build --no-cache cont-admin-api
- [ ] TASK-UOC-3: Restart cont-admin-api container
- [ ] TASK-UOC-4: Smoke test — PUT /services/{id} → 200
- [ ] TASK-UOC-5: Smoke test — PUT /routes/{id} → 200 (regression check)
