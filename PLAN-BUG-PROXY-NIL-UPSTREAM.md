# PLAN: BUG-PROXY-NIL-UPSTREAM

## Context

小黑分析確認：
- event.md Phase 9 Proxy 鏈路 503 的根因：nginx.conf `proxy_pass $cont_upstream` 少了 `http://` 前綴
- Fix 已存在於 git commit `c79a7320`（2026-06-17 09:57:56）
- 但 container image 構建於 09:57:30，早於 fix commit — **container 跑的還是舊程式碼**
- 真正需要的動作：**rebuild + restart**，而非再修 code

## Tasks

- [ ] TASK-PX-FIX-1: Rebuild cont-proxy container with latest code (`docker compose build --no-cache cont-proxy`)
  - 驗收：`docker images cont-cont-proxy` 的 IMAGE ID 變化（代表新 image）
  - 關聯：event.md Phase 9

- [ ] TASK-PX-FIX-2: Restart cont-proxy container
  - 驗證：`docker stop cont-proxy && docker rm cont-proxy && docker compose up -d cont-proxy`
  - 確認 container 狀態 `Up`（healthy）

- [ ] TASK-PX-FIX-3: Verify proxy forwarding returns 200
  - 條件：需要一組完整的 upstream + service + route
  - 如果 targets 為空，需先建立測試資料（192.168.1.202:3010 或其它可達 upstream）
  - 驗證：`curl -s -o /dev/null -w "%{http_code}" http://localhost:18000/{route_path}/health` → 200
  - 關聯：event.md Phase 9 驗收標準
