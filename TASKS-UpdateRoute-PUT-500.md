# Tasks — SPEC-UpdateRoute-PUT-500

## Context
- Issue: `PUT /routes/{id}` returns 500 INTERNAL_ERROR
- Root cause: likely same orBool pattern as UpdateService (Dev Agent must confirm)
- Reference: UpdateRoute was previously fixed (event.md line 363-367) — this appears to be a regression

## Tasks

- [ ] TASK-URP-1: Investigate UpdateRoute store.go — compare current UpdateRoute vs UpdateService/UpdateUpstream args/setClauses pattern (Dev Agent)
- [ ] TASK-URP-2: Fix UpdateRoute args/setClauses — apply COALESCE/NULLIF pattern or equivalent field-presence detection (Dev Agent)
- [ ] TASK-URP-3: Docker build --no-cache cont-admin-api (Dev Agent)
- [ ] TASK-URP-4: Restart cont-admin-api container (Dev Agent)
- [ ] TASK-URP-5: Smoke test — PUT /routes/{id} with partial payload → 200, fields preserved (QA Agent)
