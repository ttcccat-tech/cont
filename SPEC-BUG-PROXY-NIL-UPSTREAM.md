# SPEC-BUG-PROXY-NIL-UPSTREAM

## 背景

Proxy 轉發鏈路（Phase 9）目前返回 503。根因已確認：

1. Lua `access_by_lua_file` 中演算法選擇 `selected_target = "192.168.1.202:3010"` ✅
2. 設定 `ngx.var.cont_upstream = "192.168.1.202:3010"`（**不帶 scheme**）⚠️
3. Nginx `proxy_pass http://$cont_upstream` → 無法解析，error: `invalid URL prefix in "http://"`
4. 原因：Nginx variable-based upstream 需要完整 URL（`http://host:port`）或 explicit `resolver` directive

## 目標

修復 proxy 轉發，讓 `GET /{route_path}/health` 透過 Kong Gateway 正確 proxy 到 upstream `192.168.1.202:3010`，返回 200。

## Scope

### In-scope
- 修復 `access_by_lua_file` 中 `ngx.var.cont_upstream` 的設定（加上 `http://` 前綴）
- 或在 `nginx.conf` 中為 variable-based `proxy_pass` 加上 `resolver`
- Docker build 驗證 `docker compose build --no-cache`
- 重啟 container 後以 `curl http://localhost:18000/{route_path}/health` 驗證返回 200

### Out-of-scope
- 不修改 upstream 健康檢查邏輯
- 不修改負載平衡演算法
- 不修改 Services/Routes CRUD

## 驗收標準

1. `curl -s http://localhost:18000/{route_path}/health` 返回 HTTP 200
2. 確認 upstream target (`192.168.1.202:3010`) 在 access log 中不再為空
3. `docker compose build --no-cache` 成功，無 Lua 語法錯誤
4. 其它已通過的 API（Phase 1-10）不受影響

## 根因分析摘要

| 步驟 | 現況 | 問題 |
|------|------|------|
| Lua 選擇 target | ✅ `"192.168.1.202:3010"` | — |
| 設定 `ngx.var.cont_upstream` | ⚠️ 無 scheme 前綴 | `proxy_pass http://$cont_upstream` 失敗 |
| Nginx resolver | ❌ 缺少或無效 | variable upstream 無法解析 |
