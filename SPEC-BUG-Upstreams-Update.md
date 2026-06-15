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

- [ ] TASK-BUG-UU-1: Fix UpdateUpstream name preservation in store.go
  - **完成定義**: `name=$2` → `name=COALESCE(NULLIF($2,''), name)` — 空字串不覆蓋現有值
  - **對應驗收標準**: 標準 2、3
- [ ] TASK-BUG-UU-2: Docker build --no-cache admin-api
  - **完成定義**: `docker compose build --no-cache cont-admin-api` 成功，container restart 後 healthy
  - **對應驗收標準**: 標準 4
- [ ] TASK-BUG-UU-3: Restart cont-admin-api container
  - **完成定義**: `docker restart cont-admin-api` 後 container healthy
  - **對應驗收標準**: 標準 4
- [ ] TASK-BUG-UU-4: Smoke test — Create → Update{partial} → Verify name preserved
  - **完成定義**:
    1. `POST /upstreams` (name="test-upstream-uu") → 201，name="test-upstream-uu" ✅
    2. `PATCH /upstreams/{id}` (description="updated-desc") → 200，name 仍為 "test-upstream-uu" ✅
    3. `GET /upstreams/{id}` → name="test-upstream-uu"（未變）✅
  - **對應驗收標準**: 標準 1、2、3
