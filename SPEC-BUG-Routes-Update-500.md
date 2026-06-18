# SPEC-BUG-Routes-Update-500

## 背景
- **發現時間**: 2026-06-18 06:44 AM UTC（QA Verification）
- **API**: PUT /routes/{id}
- **預期**: 200 + 更新後的 Route JSON
- **實際**: 500 INTERNAL_ERROR

## 小黑根因確認（2026-06-18 09:00）

### UpdateRoute bool 欄位已修復
`84309f98`（2026-06-18 08:29）已修復:
- `strip_path=$N` (不再 COALESCE/NULLIF)
- `preserve_host=$N`
- `enabled=$N`

### 為何仍然 500
需要 Dev Agent 實際執行一次 PUT /routes/{id} 來確認是否還有 SQL 錯誤或其他問題。

可能的殘留問題:
1. `getOneRoute` (SELECT) 的 pointer scan 問題 — `r.StripPath` 是 `*bool`，scan 時需要 `*r.StripPath`
2. 其他尚未發現的 bool/int/nullable 欄位問題
3. Routes table 的 `service_id` 為 UUID FK，若 service_id 無效可能觸發 DB 錯誤

## Scope

### In-scope
- `storage/store.go` UpdateRoute + getOneRoute: 確認所有 pointer scan 正確
- 若有殘留 COALESCE/NULLIF bool 問題，一併修復
- Docker build --no-cache cont-admin-api
- 驗證 PUT /routes/{id} → 200（含 bool 欄位 partial update）

### Out-of-scope
- PatchRoute（已有完整 field-presence detection）
- CreateRoute
- Routes 相關的 Lua/nginx 處理

## 驗收標準
1. `PUT /routes/{id}` with `{}` body → 所有非明確欄位保持 DB 舊值 → 200
2. `PUT /routes/{id}` with `{"strip_path": false}` → strip_path 設為 false → 200
3. `PUT /routes/{id}` with `{"enabled": false}` → enabled 設為 false → 200
4. `GET /routes/{id}` 之後確認欄位值正確
5. Docker build --no-cache 成功
6. Container restart 後 healthy
