# Tasks — SPEC-UpdateService-PUT-500

## Context
- Issue: `PUT /services/{id}` returns 500 INTERNAL_ERROR
- Root cause: `orBool(svc.Enabled, true)` cannot distinguish absent vs false; absent fields overwrite DB values
- Reference: `UpdateUpstream` fix uses `COALESCE(NULLIF($2,''), name)` SQL pattern + map field detection

## Tasks

- [ ] TASK-USP-1: Investigate UpdateService store.go — compare UpdateService vs UpdateUpstream args/setClauses pattern (Dev Agent)
- [ ] TASK-USP-2: Fix UpdateService args/setClauses — apply same COALESCE/NULLIF pattern used in UpdateUpstream (Dev Agent)
- [ ] TASK-USP-3: Docker build --no-cache cont-admin-api (Dev Agent)
- [ ] TASK-USP-4: Restart cont-admin-api container (Dev Agent)
- [ ] TASK-USP-5: Smoke test — PUT /services/{id} with partial payload → 200, fields preserved (QA Agent)
