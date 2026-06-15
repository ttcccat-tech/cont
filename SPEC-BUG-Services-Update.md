# SPEC-BUG-Services-Update

## 背景
- QA Full-System 測試發現：`PUT /services/{id}` 返回 `{"code":"INTERNAL_ERROR","message":"internal server error"}`
- 功能阻斷：Services Update 操作完全失效，P0 bug

## 目標
- 修復 `PUT /services/:id` INTERNAL_ERROR bug
- 確認更新後 Service 資料正確寫入 DB

## Scope

### In-scope
- `services.UpdateService` handler (services/services.go)
- `store.UpdateService` storage function
- Service JSON deserialization（含 `Upstream` nested object 解析）
- DB transaction / commit 邏輯

### Out-of-scope
- Service creation (CreateService 已正常)
- Service deletion
- Upstream 本身的 CRUD

## 驗收標準
1. `PUT /services/:id` with valid JSON body → 200 OK + updated service JSON
2. `PUT /services/:id` with invalid ID → 404 not found
3. `PUT /services/:id` with invalid JSON → 400 validation error
4. Updated service can be retrieved via `GET /services/:id` and matches submitted data
5. Docker build --no-cache succeeds for admin-api
6. Container restarts successfully
