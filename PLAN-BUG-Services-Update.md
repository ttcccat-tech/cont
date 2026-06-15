# PLAN: BUG-Services-Update

## Tasks

- [ ] TASK-SU-1: Diagnose UpdateService INTERNAL_ERROR — inspect store.UpdateService (storage/store.go), find nil pointer or DB commit issue
- [ ] TASK-SU-2: Fix the identified root cause in routes/services.go or storage layer
- [ ] TASK-SU-3: Docker build --no-cache admin-api && docker restart cont-admin-api
- [ ] TASK-SU-4: Verify PUT /services/:id → 200 OK with curl

## Acceptance Criteria Mapping
- TASK-SU-1 + SU-2 → 驗收標準 1,2,3,4 (functional fix)
- TASK-SU-3 → 驗收標準 5 (Docker build)
- TASK-SU-4 → 驗收標準 1,4 (API verification)
