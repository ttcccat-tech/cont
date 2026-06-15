# SPEC-BUG-JWT-Credential

## 背景
- QA Full-System 測試發現：Consumer Credential API 全部返回 `404 page not found`
- `POST /consumers/{id}/jwt` — JWT credential endpoint 完全不存在
- `POST /consumers/{id}/key-auth` — key-auth credential endpoint 存在但返回 404
- 功能阻斷：無法為 Consumer 建立任何 credential，JWT/key-auth 認證流程無法閉環，P0 bug

## 目標
- 實作 `POST /consumers/:id/jwt` — 建立 JWT credential
- 實作 `GET /consumers/:id/jwt/credentials` — 列出 JWT credentials
- 實作 `DELETE /consumers/:id/jwt/credentials/:credId` — 刪除 JWT credential
- 修復 `POST /consumers/:id/key-auth` 404 bug
- 確保 credential 認證流程可在 proxy 層閉環

## Scope

### In-scope
- JWT credential CRUD endpoints under `/consumers/:id/jwt/credentials`
- key-auth credential endpoint `/consumers/:id/key-auth` (PUT/PATCH/DELETE)
- `store.CreateCredential`, `store.ListCredentials`, `store.DeleteCredential` for JWT type
- JWT credential data model (jwt_secret, algorithm, etc.)
- Admin API route registration for JWT credentials

### Out-of-scope
- OAuth2 credential types
- Consumer creation/deletion (already working)
- Proxy JWT validation logic (already exists in jwt_validation.lua)

## 驗收標準
1. `POST /consumers/:id/jwt/credentials` with `{"key":"...","algorithm":"RS256","secret":"..."}` → 201 Created
2. `GET /consumers/:id/jwt/credentials` → 200 OK with list of credentials
3. `DELETE /consumers/:id/jwt/credentials/:credId` → 204 No Content
4. `POST /consumers/:id/key-auth/credentials` → 201 Created (fixes existing 404)
5. Docker build --no-cache succeeds for admin-api
6. Container restarts successfully
7. Credential can be validated via `/internal/validate-cred/jwt/:key`
