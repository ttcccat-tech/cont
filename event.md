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
- **預期**: 200
- **實際**: 500
- **原因**: 舊 QA Run 的記錄；小黑親測 PUT /services/{id} ✅ 200（2026-06-17 12:35）
- **小黑驗證**: develop 最新程式碼 Services Update 正常
- **修補方向**: QA Run #2 的 500 可能是舊 container image 未重建
- **驗證**: 小黑親測 ✅ PASS
- **嚴重程度**: P0 → 需重建 container 驗證

### 🔴 BUG-Routes-Update: Routes Update 返回 500（P0）
- **API**: PUT /routes/{id}
- **預期**: 200
- **實際**: 500
- **原因**: 舊 QA Run 的記錄；小黑親測 PUT /routes/{id} ✅ 200（2026-06-17 12:36）
- **小黑驗證**: develop 最新程式碼 Routes Update 正常（WHERE clause fix 已生效）
- **小黑確認**: c0a36e4f fix(store): UpdateRoute WHERE clause 已正確處理 empty orgID
- **修補方向**: QA Run #2 的 500 可能是舊 container image 未重建
- **驗證**: 小黑親測 ✅ PASS
- **嚴重程度**: P0 → 需重建 container 驗證

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