# SPEC-PENDING-01: Staged Development — Cosocket Refactor + Service.upstream_id

## 背景

目前 `/var/repo/cont` 存在 4 個 unstaged 變更，屬於兩項獨立的實作：

### 變更 A：Cosocket 實作（proxy 層）
- `proxy/lua/cont/jwt_validation.lua`（新檔案）：使用 cosocket 呼叫 Admin API 驗證 JWT/consumer credentials，解決 `ngx.location.capture` 在 OpenResty 1.29+ 無法跨 C-call boundary 的問題
- `proxy/lua/cont/config_sync.lua`（新檔案）：使用 cosocket 每 10s 同步 config snapshot，取代原本在 access.lua 中每次 request 都呼叫 internal API 的方式
- `proxy/lua/cont/access.lua`：移除舊的 `admin_api_call()` 和 `validate_jwt()`，改用 `jwt_validation` module
- `proxy/nginx.conf`：init_worker_by_lua_block 新增 `config_sync` 定時同步，http level 新增 `resolver` 宣告

### 變更 B：Service.upstream_id 欄位（backend schema）
- `admin-api/storage/models.go`：Service struct 新增 `UpstreamID` 欄位
- `admin-api/storage/store.go`：CreateService/GetService/UpdateService SQL 新增 `upstream_id` 欄位
- ⚠️ **缺少 migration**：沒有建立 `upstream_id` 欄位的 migration script

## 目標

**Phase 1（本次）**：確認並修復所有問題，使 codebase 達到可編譯、可啟動的狀態
- 修復 `upstream_id` migration 缺失
- 驗證 nginx -t 通過
- 驗證 docker compose build --no-cache 成功

**Phase 2（後續）**：完整的功能測試

## Scope

### In-scope
- 新增 `upstream_id` migration（v025）使 schema 與 store.go 一致
- 驗證所有 Lua modules 語法正確
- 驗證 Go build 成功
- 驗證 nginx -t 通過
- 驗證 docker compose build --no-cache 成功

### Out-of-scope
- 不修改 Plugin handler 的 access phase 邏輯
- 不實作 Service 綁定 Upstream 的具体業務邏輯（預留欄位）
- 不修改 frontend

## 驗收標準

1. `admin-api/migrator/migrations.go` 包含 `v025: services.add_upstream_id` migration
2. `go build ./admin-api/...` 成功（無 upstream_id column 錯誤）
3. `docker exec cont-proxy nginx -t` 通過
4. `docker compose build --pull --no-cache proxy` 成功
5. `docker compose build --pull --no-cache admin-api` 成功

## Tasks

### Phase 1 — Migration Fix
- [ ] TASK-MIGRATION-1: 新增 v025 migration — `ALTER TABLE services ADD COLUMN upstream_id UUID REFERENCES upstreams(id) ON DELETE SET NULL`
- [ ] TASK-MIGRATION-2: 驗證 Go build `./admin-api/...` 成功

### Phase 2 — Build Verification
- [ ] TASK-BUILD-1: 驗證 `docker compose build --pull --no-cache proxy` 成功
- [ ] TASK-BUILD-2: 驗證 `docker compose build --pull --no-cache admin-api` 成功
- [ ] TASK-BUILD-3: 驗證 `docker exec cont-proxy nginx -t` 通過
- [ ] TASK-BUILD-4: 驗證所有 Lua modules (`jwt_validation.lua`, `config_sync.lua`) 語法正確

### Phase 3 — Integration
- [ ] TASK-INT-1: 重啟所有 containers，確認 services 啟動正常
- [ ] TASK-INT-2: 驗證 `/internal/config/snapshot` API 正常回應
- [ ] TASK-INT-3: 更新 event.md，記錄本輪完成狀態
