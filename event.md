# Cont Gateway 全功能重建 — 明日工作

## 目標
從零開始重建 Cont 系統，確保使用者操作流程完整正常。

---

## Phase 1：系統基礎建設

### 1.1 清理舊環境
- 停止並移除所有 Cont containers
- 清理 PostgreSQL 資料（可選）或建立新 DB
- 清理 Docker volumes（如需）

### 1.2 啟動基礎服務
- `docker compose up -d cont-postgres`
- `docker compose up -d cont-redis`
- 驗證 DB migration 完成

### 1.3 啟動 Admin API
- `docker compose up -d cont-admin-api`
- 驗證：`curl http://localhost:18081/health`
- 驗證：`POST /auth/login` → 取得 JWT token

### 1.4 啟動 Proxy
- `docker compose up -d cont-proxy`
- 驗證：`curl http://localhost:18000/` → 200

---

## Phase 2：建立 Upstream + Service + Route（UI 操作）

### 2.1 建立 Upstream
- **名稱**：`test-api`
- **Target**：`192.168.1.202:3010`
- **驗證**：GET /upstreams 回傳新建的 upstream

### 2.2 建立 Service
- **名稱**：`test-api-service`
- **類型**：HTTP
- **Upstream**：選擇 `test-api`
- **驗證**：GET /services 回傳新建的 service，含 upstream_id

### 2.3 建立 Route
- **名稱**：`test-api-route`
- **Path**：`/test-api`
- **Service**：選擇 `test-api-service`
- **驗證**：GET /routes 回傳新建的 route

### 2.4 驗證 Proxy 轉發
- `curl http://localhost:18000/test-api/health` → `{"status":"ok",...}`
- 確認轉發到 `192.168.1.202:3010/health`

---

## Phase 3：使用者 CRUD 操作驗證

### 3.1 使用者管理
- [ ] 建立使用者（POST /users）
- [ ] 取得使用者列表（GET /users）
- [ ] 更新使用者（PATCH /users/{id}）
- [ ] 刪除使用者（DELETE /users/{id}）
- [ ] 登入驗證（POST /auth/login）

### 3.2 群組管理
- [ ] 建立群組（POST /groups）
- [ ] 取得群組列表（GET /groups）
- [ ] 更新群組（PATCH /groups/{id}）
- [ ] 刪除群組（DELETE /groups/{id}）

### 3.3 Consumer 管理
- [ ] 建立 Consumer（POST /consumers）
- [ ] 取得 Consumer 列表（GET /consumers）
- [ ] 刪除 Consumer（DELETE /consumers/{id}）

### 3.4 Service / Route / Upstream 管理（API 驗證）
- [ ] CRUD Service
- [ ] CRUD Route
- [ ] CRUD Upstream

---

## Phase 4：JWT 認證流程驗證

### 4.1 Plugin 啟用
- [ ] JWT plugin 啟用
- [ ] Key Auth plugin 啟用
- [ ] Rate Limiting plugin 啟用

### 4.2 受保護端點
- [ ] 無 JWT token → 401
- [ ] 有效 JWT token → 正常存取
- [ ] 過期 JWT token → 401

---

## Phase 5：Proxy 轉發壓測

### 5.1 Smoke Test
- `curl http://localhost:18000/test-api/health` → 200

### 5.2 Load Test
- 500 VUs / 60s
- 目標：`http://localhost:18000/test-api`
- 預期錯誤率 < 1%（目標伺服器正常的情況下）

---

## Phase 6：API 文件頁面驗證

- [ ] `/api-docs` 頁面正常載入
- [ ] Swagger UI 顯示所有端點
- [ ] Try-it-out 功能正常

---

## 🔴 Bug 記錄

（發現問題後填入）

## ✅ 本輪完成

- [✅] SPEC-INLINE-ROUTE — inline route matching + upstream_id fix，7 tasks 全部完成，merge to main

---

## 🟡 預計優化

（過程中發現的改進點）

---

## 成功標準

- ✅ Admin API 所有端點正常（CRUD）
- ✅ Proxy 轉發正常（`/test-api/*` → upstream）
- ✅ 使用者可在 UI 完成所有操作
- ✅ JWT / Auth 流程正常
- ✅ Load test 0% 錯誤率（目標伺服器正常時）
- ✅ `/api-docs` 正常顯示
