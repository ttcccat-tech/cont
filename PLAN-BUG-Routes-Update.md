# PLAN: BUG-Routes-Update

## Tasks

- [ ] TASK-RU-1: Diagnose UpdateRoute INTERNAL_ERROR — inspect store.UpdateRoute (storage/), find nil pointer or DB commit issue
- [ ] TASK-RU-2: Fix the identified root cause in routes/routes.go or storage layer
- [ ] TASK-RU-3: Docker build --no-cache admin-api && docker restart cont-admin-api
- [ ] TASK-RU-4: Verify PUT /routes/:id → 200 OK with curl

## Acceptance Criteria Mapping
- TASK-RU-1 + RU-2 → 驗收標準 1,2,3,4 (functional fix)
- TASK-RU-3 → 驗收標準 5 (Docker build)
- TASK-RU-4 → 驗收標準 1,4 (API verification)
