# SPEC-BUG-PROXY-UPSTREAM-WRONG

## 背景
- **發現時間**: 2026-06-16 04:37 UTC（第四輪 QA cron）
- **現象**: `GET /test-api/health` via Gateway 轉發到錯誤 upstream（final.com:80 而非 192.168.1.202:3010）
- **小黑根因確認**: upstream_id 解析失敗，正確 upstream 未被使用，fallback 到 service.host

## 目標
- 確認 config_sync.lua 中 upstream_id → target 解析邏輯正確
- 確認路由轉發到正確的 upstream（192.168.1.202:3010）
- 確認 /test-api/health 返回 200 且 upstream 正確

## Scope
### In-scope
- config_sync.lua 中 upstream_id → target 解析邏輯
- 確認 upstream_id 解析失敗時的 fallback 行為
- 確認最終轉發 upstream 為預期目標

### Out-of-scope
- 不修改 upstream 建立邏輯
- 不修改 service 解析邏輯

## 驗收標準
1. `GET /test-api/health` via Gateway 返回 200
2. 確認轉發 upstream 為 192.168.1.202:3010（不是 final.com:80）
3. upstream_id 解析邏輯正確（無 fallback 到 service.host）
4. Docker build --no-cache cont-proxy 成功
