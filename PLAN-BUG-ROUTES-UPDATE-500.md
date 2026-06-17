# PLAN: BUG-ROUTES-UPDATE-500

## Tasks

- [ ] TASK-RU-1: Inspect routes/routes.go UpdateRoute handler — 找出 500 根因（panic/recover 位置）
- [ ] TASK-RU-2: Inspect store/routes.go UpdateRoute — 比對 CreateRoute 欄位處理差異
- [ ] TASK-RU-3: Fix identified discrepancy — 套用與 CreateRoute 一致的欄位處理
- [ ] TASK-RU-4: Docker build --no-cache cont-admin-api
- [ ] TASK-RU-5: Restart cont-admin-api container
- [ ] TASK-RU-6: QA smoke — PUT /routes/{id} with description → 200
