# PLAN: BUG-ROUTES-UPDATE-500 + BUG-PROXY-NEWROUTE-503

## Root Cause Analysis (小黑)

### Bug 1: PUT /routes/{id} → 500
**Root Cause**: `UpdateRoute` WHERE clause (store.go line ~698) uses `org_id::text = $X` for non-empty orgID, BUT for empty orgID it compares against `''` (empty string). `CreateRoute` defaults empty orgID to zero UUID `'00000000-0000-0000-0000-000000000000'`. This mismatch means UPDATE never matches any row → 0 rows updated → returns 500.

**Fix Pattern**: `GetRoute` uses `COALESCE(org_id::text, '000...') = COALESCE($X, '000...')` — apply same pattern to `UpdateRoute`.

### Bug 2: Proxy NewRoute 503
**Root Cause**: Unknown — 需要 Dev Agent 進一步調查 config_sync.lua + nginx.conf

---

## Tasks — BUG-ROUTES-UPDATE-500

- [ ] TASK-RU-1: Fix UpdateRoute WHERE clause — add COALESCE for empty orgID handling (store.go)
- [ ] TASK-RU-2: Verify UpdateRoute arg indices still correct after WHERE fix
- [ ] TASK-RU-3: Docker build --no-cache cont-admin-api
- [ ] TASK-RU-4: Restart cont-admin-api container
- [ ] TASK-RU-5: Smoke test — `PUT /routes/{id}` with `{"description":"updated"}` → 200
- [ ] TASK-RU-6: Smoke test — `PUT /routes/{id}` with `{"service_id":"uuid"}` → 200
- [ ] TASK-RU-7: Smoke test — `PUT /routes/{id}` with `{"paths":["/new-path"]}` → 200
- [ ] TASK-RU-8: Verify GET /routes/{id} confirms update persisted → 200

## Tasks — BUG-PROXY-NEWROUTE-503

- [ ] TASK-PN-1: Investigate config_sync.lua service_lookup for upstream_id resolution
- [ ] TASK-PN-2: Investigate nginx.conf route matching + upstream target resolution
- [ ] TASK-PN-3: Fix upstream_id resolution in config_sync.lua or nginx.conf
- [ ] TASK-PN-4: Docker build --no-cache cont-proxy
- [ ] TASK-PN-5: Restart cont-proxy container
- [ ] TASK-PN-6: Smoke test — `GET /{new_route_path}/health` via gateway → 200
