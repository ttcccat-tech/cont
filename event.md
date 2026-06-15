# Cont Gateway 全功能重建 — 明日工作

## 目標
從零開始重建 Cont 系統，確保使用者操作流程完整正常。

---

## Phase 1：系統基礎建設

### 1.1 清理舊環境
- [ ] 停止並移除所有 Cont containers
- [ ] 清理 PostgreSQL 資料（可選）或建立新 DB
- [ ] 清理 Docker volumes（如需）

### 1.2 啟動基礎服務
- [ ] `docker compose up -d cont-postgres`
- [ ] `docker compose up -d cont-redis`
- [ ] 驗證 DB migration 完成

### 1.3 啟動 Admin API
- [ ] `docker compose up -d cont-admin-api`
- [ ] 驗證：`curl http://localhost:18081/health`
- [ ] 驗證：`POST /auth/login` → 取得 JWT token

### 1.4 啟動 Proxy
- [ ] `docker compose up -d cont-proxy`
- [ ] 驗證：`curl http://localhost:18000/` → 200

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

### 🔴 BUG-001: GetUser 500 INTERNAL_ERROR（P0）— ✅ 已修復
- **API**: `GET /users/{id}`
- **原因**: `last_login_at` 資料庫欄位是 `TIMESTAMP WITH TIME ZONE`，可能為 NULL。Scan 直接放進 `string` 欄位，Go 不允許 NULL → string 轉換
- **修補**: 改用 `sql.NullString` 接收，再賦值給 `u.LastLoginAt`
- **檔案**: `admin-api/storage/store.go` GetUser()
- **驗證**: ✅ Create → Get → Update → Delete 全部 204

### 🔴 BUG-002: GetUser SELECT 缺少 org_id（P0）— ✅ 已修復
- **API**: `GET /users/{id}`
- **原因**: User struct 有 12 個欄位，SELECT 只選 10 個（漏了 `org_id`），導致 Scan 引數數目不匹配
- **修補**: SELECT 加入 `org_id`，Scan 加入 `&u.OrgID`
- **檔案**: `admin-api/storage/store.go` GetUser()
- **驗證**: ✅

### 🔴 BUG-003: 新建 Route 轉發 404 — config sync 未同步（P0）— 🔒 待修復
- **API**: `POST /routes` + `GET /routes/{id}` → 201成功，但 proxy 轉發 404
- **重現**: 建立 upstream → service → route 後，等待 15s，curl 仍 404
- **原因**: init_worker timer 只在 container 啟動時執行一次，後續 API 新建的 route 不會自動同步到 proxy 的 in-memory config
- **workaround**: 需重啟 cont-proxy container 才能讓新路由生效
- **影響**: 使用者透過 UI 新建 route 後無法立即使用，必須重啟 proxy
- **嚴重程度**: P0（功能阻斷）

### 🟡 BUG-004: JWT 未強制執行（P1）— 🔒 待修復
- **觀察**: `curl http://localhost:18000/test-api/health`（無 token）→ 200，應該 401
- **原因**: JWT plugin 可能未 attach 到 route，或 plugin 未正確啟用
- **影響**: Auth 功能無效，任何人都能訪問受保護端點
- **嚴重程度**: P1

### 🟡 預計優化：Routes/List 回傳格式不一致
- **觀察**: Users/Groups 回傳純陣列 `[]`，Consumers/Services/Routes/Plugins 回傳 `{"data":[], "next":""}`
- **影響**: 前端 UI 解析需要判斷格式，增加複雜度
- **建議**: 統一為 `{"data":[], "next":""}` 格式

### 🟡 預計優化：Services Create 需要先有 upstream_id
- **觀察**: Service 只能透過 `upstream_id` 建立（host/port 直接傳入會 500）
- **建議**: UI 流程需引導使用者先建 Upstream 再建 Service

---

## ✅ 本輪完成（2026-06-15 QA）

- [✅] GetUser 500 INTERNAL_ERROR — `sql.NullString` 修補 `last_login_at` NULL 問題
- [✅] GetUser SELECT 缺少 `org_id` — 加入 `org_id` 至 SELECT 和 Scan
- [✅] Users CRUD 全部通過（Create → Get → Update → Delete → 204）
- [✅] Groups CRUD 全部通過
- [✅] Consumers CRUD 全部通過
- [✅] Upstreams CRUD 全部通過
- [✅] Plugins CRUD 全部通過
- [✅] Proxy `/test-api/health` → 200 ✅ 轉發正常
- [✅] Proxy `/` → 200 ✅
- [✅] Auth 登入正常（JWT token 取得成功）

---

## 成功標準

- ✅ Admin API 所有端點正常（CRUD）
- ✅ Proxy 轉發正常（`/test-api/*` → upstream）
- ✅ 使用者可在 UI 完成所有操作
- ✅ JWT / Auth 流程正常
- ✅ Load test 0% 錯誤率（目標伺服器正常時）
- ✅ `/api-docs` 正常顯示
