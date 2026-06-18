# PLAN: BUG-Routes-Update-500

## Tasks

- [ ] TASK-RU-1: Diagnose remaining UpdateRoute 500 — run curl test to identify exact SQL or code error
  - 對應驗收標準: (diagnostic only)
  - 完成定義: 確認 500 的具體錯誤訊息（SQL error / null pointer / binding error）

- [ ] TASK-RU-2: Fix any remaining issues in UpdateRoute/getOneRoute pointer handling
  - 對應驗收標準: 1, 2, 3
  - 完成定義: store.go UpdateRoute + getOneRoute 所有 pointer scan 正確

- [ ] TASK-RU-3: Docker build --no-cache cont-admin-api
  - 對應驗收標準: 5
  - 完成定義: docker build exit code = 0

- [ ] TASK-RU-4: Restart cont-admin-api container
  - 對應驗收標準: 6
  - 完成定義: docker ps cont-admin-api status = healthy

- [ ] TASK-RU-5: Smoke test — PUT /routes/{id} → 200
  - 對應驗收標準: 1, 2, 3, 4
  - 完成定義:
    - `curl -X PUT /routes/{id} -d {}` → 200, 所有未指定欄位保持 DB 舊值
    - `curl -X PUT /routes/{id} -d '{"strip_path":false}'` → 200, strip_path=false
    - `curl -X PUT /routes/{id} -d '{"enabled":false}'` → 200, enabled=false
    - `curl -X GET /routes/{id}` → 確認上述欄位值正確
