# SPEC-BUG-Routes-Update

## Bug 概述
- **Issue**: BUG-Routes-Update
- **等級**: P0
- **API**: `PUT /routes/{id}`
- **現象**: 返回 `{"code":"INTERNAL_ERROR","message":"internal server error"}`

## 背景
Phase 2 QA 發現 PUT /routes/{id} 返回 INTERNAL_ERROR。根因分析（commit 051a4c4f）：
當 service_id 存在時，args slice 重建邏輯會錯誤地將 service_id 置於 $13，但 enabled 被推到 $14，導致所有 placeholder 索引偏移 1。

f1a50c04 merge commit 已實作 args 重建邏輯修正。

## 目標
確認 UpdateRoute 修復完成，PUT /routes/{id} 返回 200 + 更新後的 route JSON。

## Scope
### In-scope
- 驗證 store.go UpdateRoute() 的 args 重建邏輯正確
- 確認 PUT /routes/{id} 正常運作

### Out-of-scope
- CreateRoute 修復
- 其它 CRUD 操作

## 驗收標準
1. `PUT /routes/{id}` with valid JSON body (含 service_id) → 200 + updated route JSON
2. `PUT /routes/{id}` with non-existent id → 404
3. `PUT /routes/{id}` with empty name → 400 validation error
4. `PUT /routes/{id}` with paths, methods, hosts → 正確更新
5. Docker build --no-cache admin-api 成功
6. 容器重啟後服務正常

## Tasks
- [ ] TASK-RU-DIAG: 確認 UpdateRoute args 重建邏輯（f1a50c04 修復後）無其他問題
- [ ] TASK-RU-BUILD: Docker build --no-cache admin-api（確保最新程式碼）
- [ ] TASK-RU-RESTART: 重啟 admin-api container
- [ ] TASK-RU-QA: QA 驗證 PUT /routes/{id} → 200（含 service_id 案例）
