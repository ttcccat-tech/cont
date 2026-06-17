# SPEC-BUG-Services-Update-500

## 背景
- **發現時間**: 2026-06-16 15:13 UTC
- **嚴重程度**: 🟡 P1（API 對 invalid UUID return 500，應 return 400）
- **API**: PUT /services/{id}
- **預期**: 200 或 400（UUID 格式錯誤）
- **實際**: 500（UUID 格式錯誤時）

## 根因（小黑確診）
- API handler 直接把 `c.Param("id")` 傳給 PostgreSQL，沒有 validate UUID format
- PostgreSQL `id = '1'::uuid` → ERROR: invalid input syntax for type uuid → 500 INTERNAL_ERROR
- 當使用正確 UUID format（如 `72f6c454-2342-420a-b72c-5952e659222e`）→ ✅ 200
- QA Run #2 使用 `id=1`（非 UUID），因此看到 500

## Scope
### In-scope
- 修復：API handler 收到 non-UUID id 時 return 400（Bad Request），不回 500

### Out-of-scope
- 不改 Store 層（Store 預期收到有效 UUID）
- 不改 DB schema

## 驗收標準
- [ ] PUT /services/{id}，id 為 non-UUID（如 `1`）→ return 400 + error message
- [ ] PUT /services/{id}，id 為 valid UUID → return 200
- [ ] PUT /services/{id}，id 為 valid UUID 但記錄不存在 → return 404
