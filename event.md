# Cont 開發待辦

## 🔴 未完成（進行中）

（暂无）

## 🟡 預計優化

（暂无）

## ✅ 已完成

- [x] **Editor 角色的 targets/upstreams Delete 權限確認** — commit `97fcb767`
  - buildPermissions() 修復：移除 `canD && e != "targets"` guard，使 targets 的 level=3（等同 admin 刪除權限）
  - editor 在 PermissionMatrix 的 targets.Delete=true 正確反映到前端 canDelete 判斷（perm.level >= 3）
  - upstreams.Delete=false（editor 無法刪除 upstream）已驗證正確，後端 RequirePermission("upstreams", true) 會阻擋
- [x] **前端無使用者管理頁面（Users.tsx）** — commit `e6d90ccf`
  - Users.tsx 原本存在但後端無 /users CRUD routes，導致 404
  - 新增 store.go: ListUsers/GetUser/UpdateUser/DeleteUser + GetUserByUsername NULL fix
  - 新增 routes.go: ListUsers/GetUser/CreateUser/UpdateUser/DeleteUser handlers
  - main.go: /users GET/POST/PUT/PATCH/DELETE routes (RequirePermission users)
  - QA: GET→200 ✅, POST→201 ✅ (可立即登入) ✅, PATCH→200 ✅, DELETE→204 ✅
- [x] **前端 canDelete 未區分 editor 角色** — commit `e75f574b`
  - AuthContext.tsx canDelete 僅 return user.role === 'admin'，忽略 PermissionMatrix
  - 修復：使用 perm.level >= 3 判斷（level 3 = admin），與 canWrite 一致的邏輯
  - Editor now correctly sees delete buttons for services/routes/consumers/targets
- [x] **RBAC 細粒度權限整合前端** — commit `c2fa9254` + `c9b9ff08`
  - 前端根据用户 role 动态显示/隐藏操作按钮，viewer 角色禁止显示 Create/Delete/Edit 按钮
  - AuthContext.tsx: canWrite/canDelete 依权限控制；GET /auth/me 提供 per-entity permissions
  - Backend RequirePermission 已正確阻擋 editor 對 plugins/upstreams 的 PUT/PATCH/DELETE（QA: 全部 403 ✅）
  - 發現 Login/GetMe 回傳的 permissions 誤導前端（editor 所有 entities 都報 mode=rw/level=2）
  - 新增 buildPermissions() 以 CanWrite/CanRead/CanDelete 正確計算每 entity 的 level 與 mode
  - Editor now correctly sees: plugins/upstreams → level=1/mode=r; services/routes/consumers → level=3/mode=rwd
- [x] **RBAC GET 端點補全** — commit `ecf213e8`
  - 所有 GET 端點（services, routes, upstreams, targets, consumers, plugins, workspaces）新增 `RequirePermission(entity, false)` 檢查
  - 之前只有 POST/PUT/PATCH/DELETE 有 RBAC，GET 完全開放 — viewer/editor 可讀取不應讀取的 entities
  - QA 驗證：viewer POST /services → 403 ✅，DELETE → 403 ✅；viewer GET /services → 200 ✅
- [x] **Workspace CRUD 完整化** — commit `ecf213e8`
  - 新增 `store.UpdateWorkspace()`, `store.DeleteWorkspace()` 方法
  - 新增 `routes.UpdateWorkspace()`, `routes.DeleteWorkspace()` handler
  - main.go 新增 PUT/PATCH/DELETE 端點（原本只有 GET/POST/List）
  - QA 驗證：PATCH → 200 ✅，PUT → 200 ✅，DELETE → 204 ✅
- [x] **CI/CD Pipeline 建置** — GitHub Actions CI 已完整建置（go-test, lua-test, frontend, docker, compose-test），commit `6221240d` + `1baa102f`
  - 修復：busted 僅執行 `*_test.lua`（隔離 source files）、compose-test 預先建立 `cont_default` network
- [x] **proxy/lua 深度重構** — access_test.lua（9 tests）、header_filter_test.lua（6 tests）、healthcheck IPv6 bug fix，commit `bf023d62` + `6d43f4e3` + `73bc8241` + `68fbf01a`
  - access.lua 路由匹配 +負載平衡單元測試
  - header_filter.lua 重構為回傳函式（避免載入時執行）、nginx.conf 整合修復
  - healthcheck.lua IPv6:port 解析正確化（`[::1]:8080` → host=`::1`, port=`8080`）
  - healthcheck_test.lua IPv6 測試更新，header_filter_test.lua 新增
  - 全部 40 Lua測試通過
- [x] **單元測試覆蓋率提升（Go：admin-api ✅ / Lua：proxy ✅）** — 為 admin-api (Go) 和 proxy (Lua) 核心模組建立單元測試，提升程式碼品質與回歸防護
  - Go ✅ — admin-api/storage/models_test.go（14 tests）、admin-api/routes/routes_test.go（8 tests）、admin-api/routes/auth_test.go（7 tests）→ 共 29 tests，commit `c4b52414` + `d06594e3`
  - Lua ✅ — proxy/lua/cont/metrics_test.lua（6 tests）、status_test.lua（9 tests）、healthcheck_test.lua（8 tests）→ 共 23 tests，commit `05c27cef`。busted 測試框架、lua-cjson 安裝完成
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