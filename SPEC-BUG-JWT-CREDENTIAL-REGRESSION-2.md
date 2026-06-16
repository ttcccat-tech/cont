# SPEC-BUG-JWT-CREDENTIAL-REGRESSION-2

## 背景
- **發現時間**: 2026-06-16 04:37 UTC（第四輪 QA cron）
- **現象**: `POST /consumers/{id}/jwt` 返回 404 page not found
- **歷史**: 2026-06-16 上午已修復並驗證通過（BUG-JWT-CREDENTIAL-REGRESSION），現又回歸
- **小黑根因確認**: 需排查 consumersRoutes 是否正確註冊了 `/jwt` POST handler，或 container 狀態問題

## 目標
- 確認 JWT credential API handler 正確註冊
- 確認 `POST /consumers/{id}/jwt` 返回 201（不是 404）
- 確認 JWT credential CRUD 全流程正常

## Scope
### In-scope
- 檢查 routes.go 中 consumersRoutes 的 `/jwt` POST handler 註冊
- 檢查 container 狀態是否正常
- 必要時重啟 container

### Out-of-scope
- 不修改 JWT credential 核心邏輯（已有穩定版本）

## 驗收標準
1. `POST /consumers/{id}/jwt` 返回 201（created）
2. `GET /consumers/{id}/jwt/credentials` 返回 200（list）
3. JWT credential CRUD 全流程：POST 201, GET 200, PATCH 200, DELETE 204
4. Docker build --no-cache cont-admin-api 成功
