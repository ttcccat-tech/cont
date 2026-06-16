# SPEC-BUG-PROXY-UPSTREAM-WRONG

## 背景
- **發現時間**: 2026-06-16 04:37 UTC（第四輪 QA cron）
- **現象**: `GET /test-api/health` via Gateway 轉發到錯誤 upstream（final.com:80 而非 192.168.1.202:3010）
- **小黑根因確認**（2026-06-16 15:30 UTC）:
  1. `config_sync.lua` 已將 services 從陣列轉為字典（commit `4b682b33`）✅
  2. **真正根因**：`nginx.conf:489` route matching priority 邏輯 bug
  3. `if priority > highest_priority then` — 當兩個 route priority 都是 0（預設值），**最後**遍歷到的 route 勝出
  4. `with-svc` route（`paths=["/test-api"]`）在 routes array 中較後面，所以取代了 `test-api-route`
  5. `with-svc` service → `host=final.com:80`（錯誤 upstream）
  6. `test-api-route`（`paths=["/test-api"]`）service=`test-api-svc` → `host=192.168.1.202:3010`（正確）

## 目標
- `GET /test-api/health` via Gateway 返回 200
- 確認轉發 upstream 為 `192.168.1.202:3010`（不是 `final.com:80`）
- Docker build --no-cache cont-proxy 成功

## Scope
### In-scope
- `nginx.conf`: 修復 route priority 邏輯 `priority >` → `priority >=`
- 驗證 `/test-api/health` 走向正確 upstream

### Out-of-scope
- 不修改 service 建立/更新邏輯
- 不修改 config_sync.lua（已修復）

## 驗收標準
1. `GET /test-api/health` via Gateway 返回 200
2. 確認轉發 upstream 為 `192.168.1.202:3010`（不是 `final.com:80`）
3. `docker compose build --no-cache cont-proxy` 成功
4. Container 重啟後 healthy
5. smoke test 確認 upstream 正確
