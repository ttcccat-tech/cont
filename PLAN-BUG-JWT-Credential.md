# PLAN: BUG-JWT-Credential

## Tasks

- [ ] TASK-JWT-1: Add JWT credential routes to main.go: POST/GET/DELETE /consumers/:id/jwt/credentials
- [ ] TASK-JWT-2: Implement routes.CreateCredential(store, "jwt") handler with JWT credential storage
- [ ] TASK-JWT-3: Implement routes.ListCredentials(store, "jwt") handler  
- [ ] TASK-JWT-4: Implement routes.DeleteCredential(store, "jwt") handler
- [ ] TASK-JWT-5: Docker build --no-cache admin-api && docker restart cont-admin-api
- [ ] TASK-JWT-6: Verify POST /consumers/:id/jwt/credentials → 201, GET → 200, DELETE → 204

## Acceptance Criteria Mapping
- TASK-JWT-1 → 驗收標準 1,2,3 (routes registered)
- TASK-JWT-2 → 驗收標準 1 (create)
- TASK-JWT-3 → 驗收標準 2 (list)
- TASK-JWT-4 → 驗收標準 3 (delete)
- TASK-JWT-5 → 驗收標準 5 (Docker build)
- TASK-JWT-6 → 驗收標準 1,2,3,4 (API verification)

## Note
key-auth credential routes (POST/GET/DELETE /consumers/:id/key-auth/credentials) already exist in main.go lines 202-205 but return 404. Root cause likely missing handler implementation (CreateCredential with "key-auth"). Dev Agent should verify and fix as part of TASK-JWT-2 if needed.
