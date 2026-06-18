# SPEC-UpdateService-PUT-500 — UpdateService PUT Returns 500

## 背景
- **發現時間**: 2026-06-18 06:44 UTC（QA Verification）
- **發現方式**: QA smoke test — `PUT /services/{id}` 返回 500 INTERNAL_ERROR
- **嚴重程度**: P0（功能阻斷 — Service 無法更新）

## 目標
- 修復 `PUT /services/{id}` 返回 500 的問題
- 修補落實後，UpdateService 必須：
  1. 正確區分 absent field vs explicit false
  2. 未傳入的欄位保留原值（不覆蓋）
  3. Partial update（只傳 description）必須保留 enabled/name 等未傳欄位

## Scope

### In-scope
- `store.go` UpdateService 的 args/setClauses 邏輯
- JSON unmarshal 處理 absent/null 欄位
- Go model struct 的 field 處理

### Out-of-scope
- CreateService / GetService / DeleteService（已正常）
- PATCH 方法（已知 workaround）

## 驗收標準
1. `PUT /services/{id}` with `{"description":"updated"}` → 200，保留原 enabled/name
2. `PUT /services/{id}` with `{"enabled":false}` → 200，明確設為 false
3. `PUT /services/{id}` with `{"name":"new-name","enabled":true}` → 200，兩者都更新
4. `GET /services/{id}` after update → 顯示更新後的值

## 已知根因
- `orBool(svc.Enabled, true)` — 無法區分 absent vs false
- JSON unmarshal 時，absent field = zero value，無法判斷是使用者沒傳還是傳了 false
- 參考 `UpdateUpstream` 的成功修復模式：使用 `COALESCE(NULLIF($2,''), name)` SQL + map 檢測
