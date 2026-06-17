# SPEC-BUG-Proxy-Routing

## 背景
小黑親測發現：新建路由 proxy 到 upstream 時返回 503。根因是 `CreateTarget` 未驗證 `target` 不可為空，導致 empty string target 被寫入 DB。

## 目標
修復 Cont 2.0 的 Target create/update 驗證，防止 empty string target 寫入 DB。

## Scope

### In-scope
- `CreateTarget` handler 加 `target` 非空驗證（`target != ""`）
- `UpdateTarget` handler 加 `target` 非空驗證
- 現有 empty target 記錄的清理
- Docker build + container restart 驗證
- 重建後 proxy routing 恢復正常

### Out-of-scope
- 不重構 Target 相關的其它 business logic
- 不修改 Target 的其它欄位驗證

## 驗收標準
1. `POST /targets` with `target: ""` → 400 Bad Request
2. `PUT /targets/{id}` with `target: ""` → 400 Bad Request
3. 現有 empty target 記錄被清理或更新為有效值
4. `GET /{route_path}/health` via Gateway → 200（不再 503）
