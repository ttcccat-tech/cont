# Cont 開發待辦

## 🔴 未完成（進行中）

（暂无）

## 🟡 預計優化

- [ ] **CI/CD Pipeline 建置** — 為 Cont 建立 GitHub Actions / GitLab CI 自動化流程：Go lint (golangci-lint) + test、Lua busted tests、Frontend build + test、Docker image build + push、docker-compose 整合測試
  - 目標：每個 PR 自動執行全堆疊測試，生產部署前通過全部 quality gates

- [ ] **proxy/lua 深度重構（access.lua / header_filter.lua）** — proxy Lua 核心模組缺少單元測試，且多個 handler 實作粗糙（healthcheck 僅框架、TODO 遍地在）。建立完整測試覆蓋並重構關鍵模組
  - access_test.lua / header_filter_test.lua 建立
  - healthcheck.lua 實作 active probing（Redis 健康狀態寫回）
  - init.lua / worker.lua 穩定性強化

## ✅ 已完成

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