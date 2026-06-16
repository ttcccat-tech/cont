# PLAN-BUG-JWT-CREDENTIAL-REGRESSION-2

## Tasks
- [ ] TASK-JWT-1: Check routes.go consumersRoutes `/jwt` POST handler registration
- [ ] TASK-JWT-2: Check cont-admin-api container health
- [ ] TASK-JWT-3: If needed, Docker build --no-cache cont-admin-api + restart
- [ ] TASK-JWT-4: Smoke test — POST /consumers/{test — GET /{new_route}/health via Gateway → 200
