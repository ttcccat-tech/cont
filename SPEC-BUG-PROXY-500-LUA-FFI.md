# SPEC-BUG-PROXY-500-LUA-FFI

## 背景
- **發現時間**: 2026-06-16 04:37 UTC（第四輪 QA cron）
- **現象**: 新建 route 透過 Gateway GET /{route}/health → 500 Internal Server Error
- **錯誤**: `attempt to get length of local 'targets' (a userdata value)`
- **小黑根因確認**: `targets` 為 FFI cdata（FFI C pointer），不能使用 Lua `#` 取長度，應改用 `next()` 檢查是否为空

## 目標
- 修復 nginx.conf（access_by_lua）中對 FFI targets 使用 `#` 取長度的問題
- 改用 `next(targets)` 判斷 targets 是否为空
- 驗證新建 route 透過 Gateway 轉發返回 200

## Scope
### In-scope
- nginx.conf access_by_lua 中遍歷 targets 的邏輯
- 使用 `next(targets)` 替代 `#targets`

### Out-of-scope
- 不修改 FFI C 層程式碼
- 不修改 upstream 解析邏輯

## 驗收標準
1. `GET /{new_route}/health` via Gateway 返回 200（不是 500）
2. 錯誤訊息 `attempt to get length of local 'targets'` 不再出現
3. Docker build --no-cache cont-proxy 成功
4. nginx -t 語法檢查通過
