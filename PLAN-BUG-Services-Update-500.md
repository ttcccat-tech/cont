# PLAN: BUG-Services-Update-500

## Tasks

- [ ] TASK-SU-1: Fix UpdateService enabled field SQL — remove orBool, use direct bool assignment (對齊 UpdateRoute 84309f98 模式)
  - 對應驗收標準: 1, 2, 3
  - 完成定義: store.go UpdateService enabled=$13 不再有 COALESCE/NULLIF，SQL 直進 DB

- [ ] TASK-SU-2: Docker build --no-cache cont-admin-api
  - 對應驗收標準: 4
  - 完成定義: docker build exit code = 0

- [ ] TASK-SU-3: Restart cont-admin-api container
  - 對應驗收標準: 5
  - 完成定義: docker ps cont-admin-api status = healthy

- [ ] TASK-SU-4: Smoke test — PUT /services/{id} → 200
  - 對應驗收標準: 1, 2, 3
  - 完成定義:
    - `curl -X PUT /services/{id} -d {}` → 200, enabled 保持 DB 舊值
    - `curl -X PUT /services/{id} -d '{"enabled":false}'` → 200, enabled=false
    - `curl -X PUT /services/{id} -d '{"enabled":true}'` → 200, enabled=true
