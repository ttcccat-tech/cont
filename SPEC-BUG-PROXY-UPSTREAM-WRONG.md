# SPEC-BUG-PROXY-UPSTREAM-WRONG

## 背景
- **發現時間**: 2026-06-16 04:37 UTC（第四輪 QA cron）
- **現象**: `GET /test-api/health` via Gateway 轉發到錯誤 upstream（final.com:80 而非 192.168.1.202:3010）
- **小黑根因確認**: nginx.conf line 563 `if service.upstream_id then` — 只有 service.upstream_id 有值才走 upstream邏輯，否則 fallback 到 service.host（line 595-597）。當 service.upstream_id 為 nil/falsy 時，proxy 直接用 service.host 做 upstream_target

## 目標
- 確認 config_sync.lua 正確同步 service.upstream_id
- 確認 nginx.conf 邏輯正確（service.upstream_id 優先）
- 確認 /test-api/health 返回 200 且 upstream 正確

## Scope
### In-scope
- 檢查 config_sync.lua 是否正確同步 service.upstream_id
- 檢查 nginx.conf line 563 upstream_id 邏輯
- 確認 service.upstream_id 確實有值（從 config snapshot 確認）

### Out-of-scope
- 不修改 upstream 建立邏輯
- 不修改 service 解析邏輯

## 驗收標準
1. `GET /test-api/health` via Gateway 返回 200
2. 確認轉發 upstream 為 192.168.1.202:3010（不是 final.com:80）
3. upstream_id 解析邏輯正確（無 fallback 到 service.host）
4. Docker build --no-cache cont-proxy 成功
