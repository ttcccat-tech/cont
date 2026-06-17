# SPEC-UPDATE-ORGID-COHALESCE — UpdateService/UpdateRoute COALESCE Fix

## 背景
小黑於 2026-06-18 smoke test 發現：PUT /services/{id} 和 PUT /routes/{id} 都返回 HTTP 500 INTERNAL_ERROR。

## 根因
- `UpdateRoute` 的 WHERE clause 在 2026-06-17 以 commit `db0e9735` 修復了 COALESCE 問題
- 但 `UpdateService` 的 WHERE clause（store.go line 129）從未被修復，仍是舊的爛邏輯：
  ```sql
  WHERE id=$1 AND ($14 = '' OR ($14 != '' AND org_id::text = $14))
  ```
  當 DB 中 `org_id` 為 NULL 或 `'000...'` 時，條件失敗 → 500

## 目標
修復 `UpdateService` 的 WHERE clause，使其與 `UpdateRoute` 一致使用 COALESCE(NULLIF()) 模式。

## Scope
- **In-scope**: `admin-api/storage/store.go` UpdateService WHERE clause
- **Out-of-scope**: 其他 function 不動（UpdateRoute 已經修好）

## 驗收標準
1. `UpdateService` 的 WHERE clause 使用 `COALESCE(NULLIF(org_id::text, ''), '000...') = COALESCE(NULLIF($N, ''), '000...')` 模式
2. Docker build --no-cache cont-admin-api 成功
3. Container restart 後 healthy
4. `PUT /services/{id}` → 200 (description update)
5. `PUT /routes/{id}` → 200 (description update) — regression check
