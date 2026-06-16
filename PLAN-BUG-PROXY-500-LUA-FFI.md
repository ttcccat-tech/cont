# PLAN-BUG-PROXY-500-LUA-FFI

## Tasks
- [ ] TASK-FFI-1: Find `#targets` usage in nginx.conf access_by_lua blocks
- [ ] TASK-FFI-2: Replace `#targets` with `next(targets)` for FFI cdata compatibility
- [ ] TASK-FFI-3: Docker build --no-cache cont-proxy
- [ ] TASK-FFI-4: Restart cont-proxy container
- [ ] TASK-FFI-5: nginx -t syntax check
- [ ] TASK-FFI-6: Smoke test — GET /{new_route}/health via Gateway → 200 (not 500)
