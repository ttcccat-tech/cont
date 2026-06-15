# SPEC-BUG-Upstreams-Update: Upstream Update 清除 name 欄位

## 背景

P1 bug：當 `PUT /upstreams/{id}` 只傳入部分欄位（如 `{"description":"..."}`）時，未傳入的 `name` 欄位被清空成空字串，導致負載平衡目標識別失效。

根因：`store.go` 的 `UpdateUpstream` 使用 `args/setClauses` 直接建 UPDATE SET 子句，未指定 name 時變成 `name=''`，覆蓋了原本的值。

## 目標

修復 `UpdateUpstream` 邏輯，確保部分更新時未傳入欄位保留原值。

## Scope

### In-scope
- `store.go` → `UpdateUpstream()`：使用 COALESCE 或 GET-then-UPDATE 保留未更新欄位
- 驗證：Create → name="test" ✅，再 Update {description:"x"} → name="test" 保留 ✅
- Docker build --no-cache admin-api
- Restart container

### Out-of-scope
- 不改動其他 CRUD（如 Create、Delete、Get）
- 不實作新欄位

## 驗收標準

1. `POST /upstreams` (name="test-upstream") → 201，返回 name="test-upstream" ✅
2. `PATCH /upstreams/{id}` (description="updated") → 200，name 仍為 "test-upstream" ✅
3. `GET /upstreams/{id}` → name="test-upstream"（未變）✅
4. Docker build --no-cache admin-api 成功
5. Container restart 後 healthy

## Tasks

- [ ] TASK-BUG-UU-1: Analyze UpdateUpstream args/setClauses in store.go
- [ ] TASK-BUG-UU-2: Fix UpdateUpstream using COALESCE or GET-then-UPDATE pattern
- [ ] TASK-BUG-UU-3: Docker build --no-cache admin-api
- [ ] TASK-BUG-UU-4: Restart cont-admin-api container
- [ ] TASK-BUG-UU-5: Smoke test — Create → Update{partial} → Verify name preserved
