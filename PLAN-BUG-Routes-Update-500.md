# PLAN-BUG-Routes-Update-500

## Bug
- **ID**: BUG-Routes-Update-500
- **API**: PUT /routes/{id}
- **現象**: PUT without service field → 500 INTERNAL_ERROR
- **小黑驗證**: 2026-06-17 06:02 UTC — confirm 500

## Root Cause Analysis（小黑親自分析）
- UpdateRoute handler (routes.go:621) 接收 `*storage.Route`
- JSON `{"description":"QA check"}` → `r.Service = nil`
- store.UpdateRoute() 檢查 `r.Service != nil && r.Service.ID != ""` → false → service_id 不寫入 SQL
- WHERE clause: `WHERE id=$1 AND COALESCE(NULLIF(org_id::text, ''), '000...') = COALESCE(NULLIF($13, ''), '000...')`
- 但 05:38 QA 仍看到 500 表示：當前 container binary 並非最新 develop (Jun17 05:37 commits)
- develop 上已有 fix commits (2d93db08, c0a36e4f)，但 container 是 Jun16 21:08 build
- **真正根因需在最新 develop 上重建驗證**

## Tasks

### 步驟 1: 確認 Routes Update 500 的根因
- [ ] TASK-RU-1: 確認 develop 上 UpdateRoute store.go + routes.go 程式碼正確
  - 預期: `r.Service != nil && r.Service.ID != ""` guard 存在
  - 預期: WHERE clause 使用正確 orgID placeholder
  - 驗證: `grep -n "Service != nil" admin-api/storage/store.go`

### 步驟 2: Rebuild container with latest develop
- [ ] TASK-RU-2: Docker build --no-cache cont-admin-api
  - 完成定義: `docker compose build --no-cache cont-admin-api` exit_code=0
- [ ] TASK-RU-3: Restart cont-admin-api container
  - 完成定義: `docker compose up -d cont-admin-api` + `docker ps` shows healthy

### 步驟 3: 驗證 Routes Update 修復
- [ ] TASK-RU-4: Smoke test — PUT /routes/{id} without service → 200
  - 完成定義: `curl -X PUT /routes/{id} -d '{"description":"test"}'` → 200, no INTERNAL_ERROR
- [ ] TASK-RU-5: Smoke test — PUT /routes/{id} with service_id → 200
  - 完成定義: `curl -X PUT /routes/{id} -d '{"service_id":"..."}'` → 200
- [ ] TASK-RU-6: Regression — GET /routes/{id} → service_id preserved
  - 完成定義: After partial update, GET returns original service_id

## 驗收標準（對應 SPEC-BUG-Routes-Update-500.md）
1. PUT /routes/{id} 攜帶 service_id → 返回 200
2. PUT /routes/{id} 無 service_id → 返回 200（原值保留）
3. GET /routes/{id} → service_id 正確反映
