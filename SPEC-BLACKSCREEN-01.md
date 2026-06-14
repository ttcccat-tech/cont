# SPEC: Frontend Black Screen Bug Fix

## 背景
5個前端頁面黑屏或載入失敗，嚴重影響使用者體驗。

## 目標
修復以下頁面使其正常渲染：
1. `/users` — `d.map is not a function` JS錯誤
2. `/alerts/rules` — 完全空白
3. `/billing` — 完全空白（無 Route）
4. `/config-snapshots` — 完全空白（無 Route，只有 `/config-versioning`）
5. `/api-docs` — 載入失敗

## 範圍

### In-scope
- 修復 Users.tsx 的 JS 錯誤（`d.map is not a function`）
- 修復 AlertRules.tsx 空白問題
- 新增 Billing.tsx route（或建立 Billing.tsx）
- 新增 `/config-snapshots` route（或確認 ConfigVersioning 是否應為此路徑）
- 修復 ApiDocs.tsx 載入失敗
- 確認 Sidebar 導航正確對應

### Out-of-scope
- 不修改後端邏輯
- 不修改其他已正常運作的頁面

## 驗收標準

1. `curl -s http://localhost:18080/` → 200
2. 打開 `/users` → 正常渲染用戶列表，無 JS 錯誤
3. 打開 `/alerts/rules` → 正常渲染 Alert Rules 列表
4. 打開 `/billing` → 正常渲染 Billing 頁面
5. 打開 `/config-snapshots` → 正常渲染 Config Snapshots 頁面
6. 打開 `/api-docs` → 正常渲染 API 文件
7. Sidebar 點擊「工作區」→ 正確導航到 `/workspaces`

## Tasks

- [ ] TASK-1: Fix Users.tsx `d.map is not a function` — 檢查 kong.ts API 回傳資料型別，確認是否有 Array.isArray normalize
- [ ] TASK-2: Fix AlertRules.tsx blank page — 檢查 component 是否正常 export，fetch 是否正常
- [ ] TASK-3: Fix Billing.tsx route — 檢查 BillingPortal component 是否存在、route 是否有設定
- [ ] TASK-4: Fix /config-snapshots route — 確認 ConfigVersioning 是否應為 /config-snapshots，或需要新增 component
- [ ] TASK-5: Fix ApiDocs.tsx load failure — 檢查 /docs 和 /docs.json endpoint，確認 Swagger UI 是否正常
- [ ] TASK-6: Fix sidebar "工作區" navigation — 檢查 App.tsx sidebar 連結是否對應正確路徑
