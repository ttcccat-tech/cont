# Cont QA Event Log — 2026-06-17

## QA Summary
- **Date**: 2026-06-17
- **Admin API**: http://localhost:18081
- **Gateway**: http://localhost:18000
- **Tester**: Hermes Agent (cron)
- **QA Run #2**: 2026-06-17 12:05 PM

## 🔴 Bug 記錄（2026-06-17 QA Run #2）

### 🔴 BUG-Services-Update: Services Update 返回 500（P0）
- **API**: PUT /services/{id}
- **預期**: 200 或 400（UUID 格式錯誤）
- **實際**: 500
- **小黑確診**: API handler 未 validate UUID format，PostgreSQL 直接 fail → 500
- **根因**: c.Param("id") 直接傳入 SQL，non-UUID（如 "1"）導致 `ERROR: invalid input syntax for type uuid`
- **小黑驗證**（舊）: 用有效 UUID 測試 → ✅ 200（但 event.md 錯誤記載為「修好」）
- **小黑驗證**（新，2026-06-17 14:17）:
  - PUT /services/1 → **400** ✅（invalid UUID）
  - PUT /services/{valid-uuid} → **200** ✅
- **小黑判定**: 🟡 → ✅ FIXED（UUID validation 已實作）
- **修復 commit**: 8461cea2, 6f3251e8
- **修補方向**: UUID validation handler — API 現在對 invalid UUID return 400 而非 500

### 🔴 BUG-Routes-Update: Routes Update 返回 500（P0）
- **API**: PUT /routes/{id}
- **預期**: 200 或 400（UUID 格式錯誤）
- **實際**: 500
- **小黑確診**: 同 Services，handler 未 validate UUID format
- **小黑驗證**（2026-06-17 14:17）:
  - PUT /routes/1 → **400** ✅（invalid UUID）
  - PUT /routes/{valid-uuid} → **200** ✅
- **小黑判定**: 🟡 → ✅ FIXED（UUID validation 已實作）
- **修復 commit**: 8461cea2, 6f3251e8

### ✅ BUG-Proxy-Routing: Proxy Routing 已修復
- **根因**: CreateTarget 未驗證 target 不可為空
- **修復**: develop 分支已有 validation（commit ba35db16 + 512c804a），本輪重建 containers
- **小黑驗證**:
  - `POST /upstreams/{id}/targets` with `target:""` → **400** ✅
  - `PUT /upstreams/{id}/targets/{tid}` with `target:""` → **400** ✅
  - DB 無 empty target 殘留記錄 ✅
- **驗證時間**: 2026-06-17 13:38
- **小黑判定**: 🔴 → ✅ FIXED

## ✅ 通過項目（2026-06-17 QA Run #2）

| Phase | 功能 | 狀態 |
|-------|------|------|
| 1 | Auth 登入 | ✅ 通過 |
| 2 | Users CRUD | ✅ 全部通過 |
| 3 | Groups CRUD | ✅ 全部通過 |
| 4 | Consumers CRUD | ✅ 全部通過 |
| 5 | Upstreams CRUD | ✅ 全部通過 |
| 6 | Services CRUD | ⚠️ Create/List/Get/Delete OK，Update 500 ❌ |
| 7 | Routes CRUD | ⚠️ Create/List/Get/Delete OK，Update 500 ❌ |
| 8 | Plugins CRUD | ✅ 全部通過 |
| 9 | Proxy Routing | ❌ 503 on new route |
| 10 | JWT Auth | ✅ credential API 正常 |

## 🔴 P0 Bug 彙整（需修復）
1. Services Update 500
2. Routes Update 500
3. Proxy Routing 503（upstream targets nil）