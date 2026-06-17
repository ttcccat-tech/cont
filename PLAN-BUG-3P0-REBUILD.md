# PLAN: 3 P0 Bugs — Container Rebuild + Verify

## Context
小黑確認：3 個 P0 bugs 的 source fix 都已在 develop branch（store.go UpdateRoute COALESCE, store.go UpdateService COALESCE, nginx.conf Lua guard），但對應容器都未重建生效。PX-1 Go fix（targetsMap init）已存在於程式碼，PX-2 Lua guard 已 commit。

## Tasks

### 🔴 BUG-SERVICES-UPDATE + BUG-ROUTES-UPDATE（共一次 build）
- [ ] TASK-REBUILD-1: Docker build --no-cache cont-admin-api（套用 store.go COALESCE fixes）
- [ ] TASK-REBUILD-2: Restart cont-admin-api container
- [ ] TASK-REBUILD-3: Verify — login to get auth token, then PUT /services/{id} → 200
- [ ] TASK-REBUILD-4: Verify — PUT /routes/{id} → 200

### 🔴 BUG-PROXY-NIL-UPSTREAM
- [ ] TASK-REBUILD-5: Verify nginx.conf Lua guard is in the built image (check running container)
- [ ] TASK-REBUILD-6: If needed, docker build --no-cache cont-proxy
- [ ] TASK-REBUILD-7: Restart cont-proxy container
- [ ] TASK-REBUILD-8: Create test upstream + service + route → GET /test_fwd/health → 200
