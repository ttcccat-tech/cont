# SPEC-UpdateRoute-PUT-500 — UpdateRoute PUT Returns 500

## 背景
- **發現時間**: 2026-06-18 06:44 UTC（QA Verification）
- **發現方式**: QA smoke test — `PUT /routes/{id}` 返回 500 INTERNAL_ERROR
- **嚴重程度**: P0（功能阻斷 — Route 無法更新）
- **根因假設**: 與 UpdateService 相同的 `orBool` 問題（需 Dev Agent 確認）

## 目標
- 修復 `PUT /routes/{id}` 返回 500 的問題
- 修補落實後，UpdateRoute 必須：
  1. 正確區分 absent field vs explicit false
  2. 未傳入的欄位保留原值
  3. Partial update 必須保留其他欄位

## Scope

### In-scope
- `store.go` UpdateRoute 的 args/setClauses 邏輯
- JSON unmarshal 處理 absent/null 欄位
- 與 CreateRoute 比對找出差異

### Out-of-scope
- CreateRoute / GetRoute / DeleteRoute（已正常）
- Route matching logic（另一個已經修好的 bug）

## 驗收標準
1. `PUT /routes/{id}` with `{"description":"updated"}` → 200，保留原 name/paths
2. `PUT /routes/{id}` with `{"enabled":false}` → 200，明確設為 false
3. `PUT /routes/{id}` with `{"name":"new-route","paths":["/new"]}` → 200，兩者都更新
4. `GET /routes/{id}` after update → 顯示更新後的值

## 參考修復
- `UpdateUpstream` 已成功使用 `COALESCE(NULLIF($2,''), name)` 模式（event.md line 399-413）
- Dev Agent 應比對 UpdateUpstream vs UpdateRoute 的實作差異
