# SPEC-BUG-PROXY-UPSTREAM-WRONG

## 背景
- **發現時間**: 2026-06-16 04:37 UTC（第四輪 QA cron）
- **現象**: `GET /test-api/health` via Gateway 轉發到錯誤 upstream（final.com:80 而非 192.168.1.202:3010）
- **小黑根因確認**（2026-06-16 14:00 UTC）:
  1. `config_sync.lua:79` 存入 `cont.services = data.services` — snapshot 返回 `services` 為**陣列**
  2. `nginx.conf:510` 以 `cont.services[service_id]` 做**字典查找**（如 `cont.services["b29c2082-..."]`）
  3. 陣列以數值索引（`[1]`, `[2]`），UUID 字串索引全部返回 `nil`
  4. 因此 `service = nil`，`service.upstream_id` 為 `nil`
  5. 走到 `elseif service.host then` 分支 → `final.com:80`
  6. 正確 `service.host = 192.168.1.202` 且 `service.port = 3010`，`upstream_id = 0a694256...` 指向 `192.168.1.202:3010`
  7. 修復方向：`config_sync.lua` 將 `services` 從陣列轉為字典（`{id -> service}`），或 `nginx.conf` 改為線性搜尋

## 目標
- `GET /test-api/health` via Gateway 返回 200
- 確認轉發 upstream 為 `192.168.1.202:3010`（不是 `final.com:80`）
- Docker build --no-cache cont-proxy 成功

## Scope
### In-scope
- `config_sync.lua`: 將 `data.services` 陣列轉為 `{[service.id] = service}` 字典
- `nginx.conf`: 確認 `cont.services` 為字典，可直接用 `service_id` 查找
- 驗證 `/test-api/health` 走向正確 upstream

### Out-of-scope
- 不修改 service 建立/更新邏輯
- 不修改 snapshot API 格式（只修改記憶體中的結構）

## 驗收標準
1. `GET /test-api/health` via Gateway 返回 200
2. 確認轉發 upstream 為 `192.168.1.202:3010`（不是 `final.com:80`）
3. `docker compose build --no-cache cont-proxy` 成功
4. Container 重啟後 healthy
5. smoke test 確認 upstream 正確

## Tasks
- [ ] TASK-UPSTREAM-FIX-1: config_sync.lua — 將 services 從陣列轉為字典 `{[s.id] = s}`
- [ ] TASK-UPSTREAM-FIX-2: Docker build --no-cache cont-proxy
- [ ] TASK-UPSTREAM-FIX-3: Restart cont-proxy container
- [ ] TASK-UPSTREAM-FIX-4: Smoke test — `GET /test-api/health` → 200, upstream = `192.168.1.202:3010`
- [ ] TASK-UPSTREAM-FIX-5: Update event.md + commit
