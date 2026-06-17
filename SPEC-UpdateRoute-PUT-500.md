# SPEC-UpdateRoute-PUT-500 — UpdateRoute PUT Returns 500 (REVISED)

## 背景
- **發現時間**: 2026-06-18 06:44 UTC（QA Verification）
- **QA 驗證時間**: 2026-06-18 07:30 UTC
- **小黑判定**: UpdateService ✅ 已修（commit 6a52d71e），UpdateRoute 🔴 未完全修

## 根因（小黑確認）
1. `UpdateRoute`（store.go:704-728）對 bool/int 欄位無 COALESCE 保護
2. 當 JSON 未攜帶某欄位時，Go struct 該欄位為 zero value（false/0），直接寫入 DB
3. `UpdateService` 已用 `COALESCE/NULLIF` 修復（int/port/retries），但 bool (`enabled`) 仍用 `orBool`
4. **根本問題**：JSON unmarshal 無法區分 absent field vs explicit false/0
5. **真正修復方向**：指標類型（`*bool`/`*int`）才能區分 absent vs zero value

## 目標
- 修復 `PUT /routes/{id}` 返回 500 的問題（UpdateService 已修，UpdateRoute 未修完）
- 修補落實後，UpdateRoute 必須：
  1. 正確區分 absent field vs explicit false/0
  2. 未傳入的欄位保留原值
  3. Partial update 必須保留其他欄位

## Scope

### In-scope
- `models.go` Route struct：將 bool/int 欄位改為指標類型（`*bool`/`*int`）
- `store.go` UpdateRoute：使用 `COALESCE/NULLIF` 動態 SQL
- `routes/routes.go` UpdateRoute handler：確保正確處理 nil 指標
- 指標類型 ripple effect：所有使用 Route 欄位的地方

### Out-of-scope
- CreateRoute / GetRoute / DeleteRoute（已正常）
- PATCH 方法

## 驗收標準
1. `PUT /routes/{id}` with `{"strip_path":true}` → 200，`preserve_host`/`enabled` 保留原值
2. `PUT /routes/{id}` with `{"enabled":false}` → 200，明確設為 false，其他欄位保留
3. `PUT /routes/{id}` with `{"name":"new-route","paths":["/new"]}` → 200，兩者都更新
4. `GET /routes/{id}` after update → 顯示更新後的值，未更新欄位不變
5. `PUT /routes/{id}` with `{"regex_priority":10}` → 200，其他欄位保留

## Tasks
- [ ] TASK-UR-1: models.go Route struct — 將 StripPath/PreserveHost/Enabled 改為 *bool，指針改為 *int（RegexPriority, HTTPSRedirectStatusCode, ConnectionTimeout）
- [ ] TASK-UR-2: store.go UpdateRoute — 調整 SQL，bool 用 COALESCE(NULLIF($X,''), field)，int 用 CASE WHEN $X=0 THEN field ELSE $X END
- [ ] TASK-UR-3: store.go UpdateRoute — 調整 args 追加邏輯，nil pointer 不追加（保留 DB 值）
- [ ] TASK-UR-4: store.go PatchRoute — 調整 Scan 邏輯以適配指標欄位
- [ ] TASK-UR-5: routes/routes.go UpdateRoute handler — 確保 nil 指標正確序列化（omitempty）
- [ ] TASK-UR-6: Docker build --no-cache cont-admin-api
- [ ] TASK-UR-7: Restart cont-admin-api container
- [ ] TASK-UR-8: Smoke test — PUT partial update preserves fields
- [ ] TASK-UR-9: git push origin develop