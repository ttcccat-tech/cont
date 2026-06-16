# SPEC-BUG-ROUTES-UPDATE — Routes Update 500 Internal Error

## Background
- **發現時間**: 2026-06-17 01:18 UTC
- **API**: PUT /routes/{id}
- **預期**: 200 OK
- **實際**: 500 Internal Server Error (INTERNAL_ERROR)

## Root Cause（小黑確認）
1. `UpdateRoute` handler (routes/routes.go:624) 用 `c.ShouldBindJSON(&r)` 綁定請求
2. 如果 request body **沒有攜帶 `service` 欄位**（如 `{"name":"new-name"}`），則 `r.Service = nil`
3. `r.GetServiceID()` 返回 `""`（空字串）
4. `store.UpdateRoute` 判斷 `svcID != ""` → true（空字串 != "" 為 true）
5. 結果：`args = append(args, "")` → `setClauses = append(setClauses, "service_id=$13")`
6. SQL UPDATE 寫入 `service_id = NULL`，覆蓋原本 DB 中的正確 service_id
7. 若是 NOT NULL 欄位 → SQL error 500；若是 NULL 允許 → service_id 被清空

**對比 CreateRoute**：`serviceID != ""` 時才 append `serviceIDArg`，空字串不寫入 SQL
**對比 UpdateRoute**：空字串也 append，導致 `service_id=NULL`

## Scope
### In-scope
- 修復 `store.UpdateRoute` 的 service_id 處理邏輯
- 修復後驗證：部分欄位更新（不傳 service）不影響 service_id

### Out-of-scope
- 不改 CreateRoute（邏輯正確）
- 不改 Route JSON format

## Acceptance Criteria
1. `PUT /routes/{id}` request without `service` field → 200 OK, service_id preserved
2. `PUT /routes/{id}` request with new `service.id` → 200 OK, service_id updated
3. `PUT /routes/{id}` request with `"service": null` → 200 OK, service_id set to NULL (explicit clear)
4. Existing routes with service_id can be updated by name alone without losing service_id

## Tasks
- [ ] TASK-RU-FIX-1: Fix store.UpdateRoute service_id logic — skip writing service_id column when Service is nil (same as CreateRoute)
- [ ] TASK-RU-FIX-2: Docker build --no-cache cont-admin-api
- [ ] TASK-RU-FIX-3: Restart cont-admin-api container
- [ ] TASK-RU-FIX-4: Smoke test — update route name only (no service) → service_id preserved, returns 200
- [ ] TASK-RU-FIX-5: Smoke test — update route with new service_id → updated, returns 200