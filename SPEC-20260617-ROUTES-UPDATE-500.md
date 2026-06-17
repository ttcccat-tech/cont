# SPEC-20260617-ROUTES-UPDATE-500

## 🔴 P0 Bug — Routes Update 返回 500

## 背景
- **發現時間**: 2026-06-16（第五輪 QA）
- **小黑驗證（舊）**: UUID validation 加入 handler → 🟡 → ✅（誤判）
- **小黑驗證（2026-06-17 14:22 QA Run #3）**: 使用有效 UUID（380453ea-06f0-4082-ab70-e09d8ebb0bbd）仍返回 500
- **根因不在 UUID validation**：UUID validation handler 已存在（commit 6f3251e8），但 500 仍然發生

## 小黑根因分析（深入追蹤）

### 可能性 1：store.UpdateRoute SQL 層 error
`store.UpdateRoute` 動態建構 SQL，handle optional service_id。
若某個非 UUID 字串（如空字串、特殊字元）導致 SQL 錯誤，store 層直接 return err → handler 收到 err → 500。

### 可能性 2：Route model binding 有問題
`ShouldBindJSON(&r)` 時，若 Route struct 的 `Service` 欄位 binding 失敗（如 service_id format 不對），可能導致 panic 或異常。

### 可能性 3：DB 層 constraint violation
`service_id` 外鍵約束存在，但 `UpdateRoute` 有時會傳入不存在的 service_id。

### 待 Dev Agent 確認
1. 在 `store.UpdateRoute` 的 `return nil, err` 前 log 或 print 錯誤內容
2. 比對 `CreateRoute` 和 `UpdateRoute` 的差異（可能是 service_id 處理方式不同）
3. 檢查 `GetRoute` → 修改 → `UpdateRoute` 的 flow

## 目標
修復 `PUT /routes/{id}` 返回 500 的 bug，確保：
- valid UUID + 正常 payload → 200
- invalid UUID → 400
- 錯誤的 service_id → 400 或 404（非 500）

## Scope

### In-Scope
- `routes/routes.go` UpdateRoute handler
- `store/store.go` UpdateRoute SQL 建構
- 比對 CreateRoute vs UpdateRoute 差異
- 修復後 Docker build --no-cache cont-admin-api

### Out-of-Scope
- 不改動其他 route handler（ListRoute, GetRoute, DeleteRoute）
- 不改動 proxy 邏輯

## 驗收標準
1. `PUT /routes/{uuid}` with `{"description": "updated"}` → 200
2. `PUT /routes/{uuid}` with `{"name": "newname"}` → 200
3. `PUT /routes/{uuid}` with `{"service_id": "non-existent-uuid"}` → 400 或 404（非 500）
4. `PUT /routes/{uuid}` with `{"paths": ["/new-path"]}` → 200
5. `GET /routes/{uuid}` 確認更新已寫入 → 200
6. Docker build --no-cache cont-admin-api 成功
7. 容器 healthy（`docker ps` show healthy）
