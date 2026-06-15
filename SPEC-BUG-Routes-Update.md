# SPEC-BUG-Routes-Update

## 背景
- QA Full-System 測試發現：`PUT /routes/{id}` 返回 `{"code":"INTERNAL_ERROR","message":"internal server error"}`
- 功能阻斷：Routes Update 操作完全失效，P0 bug

## 目標
- 修復 `PUT /routes/:id` INTERNAL_ERROR bug
- 確認更新後 Route 資料正確寫入 DB

## Scope

### In-scope
- `routes.UpdateRoute` handler (routes/routes.go)
- `store.UpdateRoute` storage function
- Route JSON deserialization（含 `Service` nested object解析）
- DB transaction / commit 邏輯

### Out-of-scope
- Route creation (CreateRoute 已正常)
- Route deletion
- Service 本身的 CRUD

## 驗收標準
1. `PUT /routes/:id` with valid JSON body → 200 OK + updated route JSON
2. `PUT /routes/:id` with invalid ID → 404 not found
3. `PUT /routes/:id` with invalid JSON → 400 validation error
4. Updated route can be retrieved via `GET /routes/:id` and matches submitted data
5. Docker build --no-cache succeeds for admin-api
6. Container restarts successfully
