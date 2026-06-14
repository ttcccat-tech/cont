# PLAN: break down SPEC-PENDING-01 into 9 tasks

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
