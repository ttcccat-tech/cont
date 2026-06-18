# SPEC-BUG-Services-Update-500

## 背景
- **發現時間**: 2026-06-18 06:44 AM UTC（QA Verification）
- **API**: PUT /services/{id}
- **預期**: 200 + 更新後的 Service JSON
- **實際**: 500 INTERNAL_ERROR

## 小黑根因確認（2026-06-18 09:00）

### 為何 UpdateService 500
1. `UpdateService` (store.go:119) 使用 `COALESCE(NULLIF(...))` 模式處理 string/int 欄位
2. **Bool 欄位 (`enabled`) 也用了 COALESCE/NULLIF，但 PostgreSQL bool 不接受 `''` 字串**，導致 SQL 錯誤
3. 正確做法（UpdateRoute 已在 `84309f98` 修復）: bool 欄位用直接賦值 `$N`，不透過 COALESCE/NULLIF
4. `orBool(svc.Enabled, true)` 邏輯問題: 當 JSON body 為 `{}` 時，`Enabled=false`（Go zero value），`orBool(false, true) → true`，覆蓋 DB 舊值

### SQL 錯誤推導
```sql
-- store.go:130:
enabled=$13  -- $13 = orBool(svc.Enabled, true)

-- 當 svc.Enabled = false (zero value):
-- orBool(false, true) → true → 意圖: "若未提供 enabled 欄位，保持 DB 值"
-- 但 SQL: enabled=true (沒有 COALESCE，總是寫入)
-- vs COALESCE(NULLIF($13,''), enabled) → NULLIF(true, '') = true → OK
-- vs 直接 enabled=$13 = true → 但若要保留舊值做不到
```

真正 bug: **UpdateService 的 bool enabled 欄位沒有 COALESCE 保留舊值邏輯**，導致部分場景 SQL 錯誤或邏輯錯誤。

## Scope

### In-scope
- `storage/store.go` UpdateService: 修復 `enabled` 欄位的 SQL 處理（對齊 UpdateRoute `84309f98` 修復模式）
- Docker build --no-cache cont-admin-api
- 驗證 PUT /services/{id} → 200

### Out-of-scope
- PatchService（已有完整 field-presence detection，無需修改）
- CreateService
- 其他 model 的 Update

## 驗收標準
1. `PUT /services/{id}` with `{}` body → enabled 欄位保持 DB 舊值 → 200
2. `PUT /services/{id}` with `{"enabled": false}` → enabled 設為 false → 200
3. `PUT /services/{id}` with `{"enabled": true}` → enabled 設為 true → 200
4. Docker build --no-cache 成功
5. Container restart 後 healthy
