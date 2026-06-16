# SPEC-BUG-JWT-CREDENTIAL-REGRESSION

## 背景
- **Issue**: BUG-JWT-CREDENTIAL-REGRESSION (P1)
- **發現時間**: 2026-06-16 08:26 UTC
- **現象**: POST /consumers/{id}/jwt/credentials 返回 404（2026-06-16 04:18 已修復，今日又回歸）

## 根因分析
main.go line 216 顯示 JWT credential POST endpoint：
```go
cred.POST("/jwt/credentials", routes.RequirePermission(store, "consumers", true), routes.CreateCredential(store, "jwt"))
```
而 event.md 顯示 route 為 `/consumers/:consumerId/jwt` — 需確認 routes.go 的 CreateCredential handler 是否有對應的 route path。

## 目標
確認 JWT credential API 404 的根因並修復。

## Scope
### In-scope
- 檢查 main.go consumersRoutes 的 JWT credential route 註冊
- 確認 routes.CreateCredential 是否正確處理 /jwt/credentials path
- Docker build --no-cache cont-admin-api
- Restart cont-admin-api container
- 驗證：POST /consumers/{id}/jwt/credentials → 201

### Out-of-scope
- 不修改其他 credential types (basic-auth, key-auth)

## 驗收標準
1. [ ] POST /consumers/{id}/jwt/credentials → 201
2. [ ] GET /consumers/{id}/jwt/credentials → 200
3. [ ] PATCH /consumers/{id}/jwt/credentials/:credId → 200
4. [ ] DELETE /consumers/{id}/jwt/credentials/:credId → 204
5. [ ] Docker build --no-cache cont-admin-api 成功
6. [ ] cont-admin-api container healthy
