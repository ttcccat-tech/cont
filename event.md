# Cont 開發待辦

## 🔴 未完成（進行中）

（暂无）

## 🟡 預計優化

- [ ] **單元測試覆蓋率提升（Go：admin-api ✅ / Lua：proxy ⏳）** — 為 admin-api (Go) 和 proxy (Lua) 核心模組建立單元測試，提升程式碼品質與回歸防護
  - Go ✅ — admin-api/storage/models_test.go（14 tests）、admin-api/routes/routes_test.go（8 tests）、admin-api/routes/auth_test.go（7 tests）→ 共 29 tests，commit `c4b52414` + `d06594e3`
  - Lua ⏳ — proxy/lua/cont/*.lua 單元測試框架尚未建立

- [ ] **使用者管理（RBAC 權限）** — 為 Cont 新增 Role-Based Access Control：定義 admin/editor/viewer 角色、對應權限矩陣、角色指派 API，防止一般使用者誤刪系統設定
  - ✅ — commit `7f58a251`，storage/rbac.go（PermissionMatrix：admin/editor/viewer 權限矩陣）、storage/postgres.go（role column migration）、routes/routes.go（RequirePermission middleware、GET /roles、GET /roles/:role/permissions）、main.go（所有寫操作 RBAC middleware）

---

## ✅ 已完成

- [x] **Swagger/OpenAPI 文件生成** — commit `c8fa5b71`，admin-api/docs/swagger.yaml（Swagger 2.0，覆蓋 /services, /routes, /upstreams, /consumers, /plugins, /workspaces 全端點，JWT Bearer 認證，Kong-compatible schemas），main.go 新增 /docs 和 /docs.json 端點（無需認證），Dockerfile 複製 docs/ 目錄。QA: /docs ✅ 200, /docs.json ✅ 200, Auth flow ✅

- [x] **生產部署評估** — commit `ce5582e4`，移除 hardcoded IP、Docker DNS resolver、admin-api healthcheck、proxy health-based startup、JWT_SECRET 支援、docker-compose.prod.yml（PG WAL、AOF、記憶體限制）、Makefile 生產目標（build-prod/push-prod/deploy-prod/roll/db-backup）、.gitignore 修復、移除過時 version 欄位
- [x] **Prometheus Metrics實作** — commit `83832e8f`，prometheus/client_golang 接入，定義 Kong-相容 metrics（kong_nginx_requests_total, kong_nginx_connections_total, kong_service_latency_ms, cont_db_*, cont_redis_*），QA: /metrics ✅ HTTP 200，包含 Go runtime + 自定義 metrics
- [x] **Cont Auth 正式實作（JWT / bcrypt）** — commit `cfd7f15d`，users table、AuthRequired middleware、JWT 登入廢除 mock
- [x] **Routes Create 不支援 service.name 格式** — commit `0a6e2060`，models.go ServiceRef + GetServiceName()，store.go GetServiceByName()，routes.go CreateRoute 解析 service.name → service.id（QA: Create ✅ Read ✅ PATCH ✅ Delete ✅）
- [x] **cont-admin-api container 未加入網路** — commit `295379b7`，docker-compose.yml 新增 networks: cont_default，確保網路正確連接
- [x] **cont-admin-api 未持久化** — commit `295379b7`，新增 restart: unless-stopped
- [x] **Services/Plugins/Routes Update PATCH 404** — commit `0cd501a0`，routes.go 缺少 PATCH 路由，已新增全部 6 個實體的 PATCH
- [x] **Services Create/Edit modal 不關閉** — commit `04125706`，移除 Modal onOk 改用 Form submit
- [x] **Plugins Create/Edit modal 不關閉** — commit `886f75d5`，同上模式
- [x] **Routes Create/Edit modal 不關閉** — commit `5790860f`，同上模式
- [x] **Services Create/Edit/Delete API QA** — Create ✅ Read ✅ Delete ✅（Update PATCH ✅）
- [x] **Plugins Create/Delete API QA** — Create ✅ Delete ✅（Update PATCH ✅）
- [x] **Routes Create/Delete API QA** — Create ✅ Read ✅ Delete ✅（Update PATCH ✅）

---

## 完整開發流程（開發守護遵循）

1. **開發執行** → 逐項處理，每修完一個問題即時 commit
2. **QA 測試** → curl / browser 驗證 Create/Read/Update/Delete 基本流程
3. **寫入 event.md** → 發現的 Bug 加入 🔴 未完成，完成的移至 ✅
4. **commit** → 每個變更单独 commit
5. **push** → 確認所有改動入庫

> 一輪：開發 → QA → Bug 寫入 event.md → commit → push = 完整回合
