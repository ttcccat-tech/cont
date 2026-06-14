# SPEC-plugin-access-param: Fix run_plugin_access missing plugin parameter

## 背景
`proxy/lua/cont/access.lua` 的 `run_plugin_access()` 函數在呼叫 plugin handler 的 `access()` 時，沒有傳入 `plugin` 參數：
```lua
local ok2, err = pcall(handler.access, handler)  -- 缺少 plugin 參數
```
但所有 plugin handler 的 `access(self, plugin)` 都需要 `plugin` 參數才能讀取 `plugin.config`。

## 目標
修復 `run_plugin_access()` 讓 plugin handler 的 `access()` 能正確接收 `plugin` 參數。

## Scope
### In-scope
- `proxy/lua/cont/access.lua`: `run_plugin_access()` 的 pcall 呼叫加入 `plugin` 參數

### Out-of-scope
- 不修改其他 Lua 檔案
- 不修改 plugin handler 本身

## 驗收標準
1. `run_plugin_access()` 的 pcall 呼叫傳入 `plugin` 參數：`pcall(handler.access, handler, plugin)`
2. `nginx -t` 通過
3. 所有現有 Lua tests 通過
4. Docker build `--no-cache` 成功

## Tasks
- [ ] TASK-PLUGIN-1: Fix `run_plugin_access()` pcall to pass `plugin` parameter
- [ ] TASK-PLUGIN-2: Verify `nginx -t` passes
- [ ] TASK-PLUGIN-3: Run Lua tests (busted)
- [ ] TASK-PLUGIN-4: Docker build `--no-cache` succeeds
