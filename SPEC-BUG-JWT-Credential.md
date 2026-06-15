# SPEC: BUG-JWT-Credential — JWT Credential API 返回 404

## 背景
- **問題**：`POST /consumers/{id}/jwt/credentials` 返回 404
- **根因**：運行中的 container binary 沒有 JWT credential routes（來自 commit cbb40cea），需要重新 build
- **嚴重程度**：P1 — 功能缺失

## Scope
### In-scope
- 重新 build cont-admin-api Docker image
- 重啟 cont-admin-api container
- 驗證 JWT credential API 正常運作

### Out-of-scope
- 不修改 source code（routes 已在 main.go lines 215-218）
- 不修改 migration（v026 已執行）

## 驗收標準
1. `POST /consumers/{id}/jwt/credentials` 返回 201 + JWT credential 物件
2. `GET /consumers/{id}/jwt/credentials` 返回 200 + credential 列表
3. `PATCH /consumers/{id}/jwt/credentials/:credId` 返回 200
4. `DELETE /consumers/{id}/jwt/credentials/:credId` 返回 204
5. Docker build --no-cache 成功，container healthy

## Tasks
- [ ] TASK-JWT-FIX-1: Docker build --no-cache cont-admin-api
- [ ] TASK-JWT-FIX-2: Restart cont-admin-api container
- [ ] TASK-JWT-FIX-3: 驗證 JWT credential CRUD APIs
