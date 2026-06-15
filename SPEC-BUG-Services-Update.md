# SPEC-BUG-Services-Update

## Bug 概述
- **Issue**: BUG-Services-Update
- **等級**: P0
- **API**: `PUT /services/{id}`
- **現象**: 返回 `{"code":"INTERNAL_ERROR","message":"internal server error"}`

## 背景
Phase 2 QA 發現 PUT /services/{id} 返回 INTERNAL_ERROR。初步分析指向 store.go UpdateService() 可能的 nil pointer 或 SQL error。

## 目標
修復 UpdateService，使得 PUT /services/{id} 返回 200 + 更新後的 service JSON。

## Scope
### In-scope
- store.go UpdateService() 函數的 SQL binding 問題修復
- 驗證 UpdateService 正常運作

### Out-of-scope
- CreateService 修復（已於 TASK-SU-2 處理）
- 其它 CRUD 操作

## 驗收標準
1. `PUT /services/{id}` with valid JSON body → 200 + updated service JSON
2. `PUT /services/{id}` with non-existent id → 404
3. `PUT /services/{id}` with empty name → 400 validation error
4. `PUT /services/{id}` with upstream_id field → upstream_id 正確更新
5. Docker build --no-cache admin-api 成功
6. 容器重啟後服務正常

## Tasks
- [ ] TASK-SU-DIAG: 確認 UpdateService INTERNAL_ERROR 根因（檢查 upstream_id UUID cast 是否為問題）
- [ ] TASK-SU-FIX: 修復 UpdateService（若 upstream_id UUID cast 是問題則加 NULLIF）
- [ ] TASK-SU-BUILD: Docker build --no-cache admin-api
- [ ] TASK-SU-RESTART: 重啟 admin-api container
- [ ] TASK-SU-QA: QA 驗證 PUT /services/{id} → 200
