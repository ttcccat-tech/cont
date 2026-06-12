# Cont 開發待辦

## 🔴 未完成（進行中）

- [ ] **Cont Billing/Plan Stripe 整合** — 立即啟動
  - Phase 3 of Cont SaaS multi-tenant roadmap（Phase 1: Organization 資料模型、Phase 2: Workspace org_id 隔離已完成）
  - 目標：Plan（Free/Pro/Enterprise）、Subscription、Stripe Checkout/Portal、Webhook 處理
  - 候選方向：Stripe Billing API、Plans table、Subscription 狀態管理、Webhook endpoint

- [ ] **Cont 使用者管理（RBAC 權限）** — 待實作
  - 使用者管理精細化（Workspace 級 RBAC）已完成
  - 下一階段：Resource-level RBAC UI（針對 service/route/upstream 的精細權限）

## 🟡 預計優化

- [x] **Cont Resource-level RBAC 權限指派** — commit `80d46e5e` + `5866a203`
  - store.go: `ensureResourceEntry()` 在 CreateService/CreateRoute/CreateUpstream/CreateConsumer 時自動註冊 Resource
  - `RequirePermission` 已具備 resource-level override 邏輯（lines 1562-1587）
  - Backend: `ListUserResourcePermissions`/`SetUserResourcePermissions` 已實作（users/:id/resource-permissions）
  - Backend: `ListGroupResourcePermissions`/`SetGroupResourcePermissions` 已實作（groups/:id/resource-permissions）
  - Backend: `ListResources`/`GetResource`/`CreateResource`/`DeleteResource` CRUD 已完整
  - Frontend: Groups.tsx 新增「資源權限」tab（针对 group 的资源权限覆写）
  - Frontend: Users.tsx 新增「資源權限」按鈕 + Drawer（針對特定 service/route/upstream/consumer 的 deny/read/write 權限）
  - QA: Create service → resource 自動創建 ✅, PUT resource-permissions → 200 ✅, GET → 200 ✅ (含 resource_name) ✅, DELETE service → 204 ✅
  - QA: upstream/consumer 自動創建 Resource ✅

- [x] **使用者管理精細化（RBAC 增強）** — commit `673fc680` + `a3a6bd82` + `495c3455` + `846c8d1a`
  - kong.ts: 新增 `getUserWorkspaces()` 對應 GET /workspaces/users/:userId
  - Users.tsx: 新增「指派工作區」按鈕 + Drawer（Checkbox + 角色選擇 viewer/editor/admin）
  - store.go: `ListUserWorkspaces` 回傳 `[]WorkspaceUserAssignment`（含 role 欄位）
  - routes.go: 修正 `workspaces[0].ID` → `workspaces[0].UserID`
  - QA: PUT → 200 ✅, GET → 200 ✅ (含 role), DELETE → 204 ✅
  - Bug fix `a3a6bd82`: store.go Scan w.id→WorkspaceID (not UserID), models.go 新增 WorkspaceID 欄位，kong.ts WorkspaceUserAssignment 新增 workspace_id 欄位，Users.tsx openWsDrawer 直接使用 workspace_id
  - API 驗證：GET /workspaces/users/:userId → workspace_id ✅, user_id ✅, role ✅, DELETE → 204 ✅
  - 批次更新角色 `846c8d1a`: WorkspaceDetail 成員表格支援 checkbox 多選、批次更新角色 Modal
  - 批次新增成員 `495c3455`: 新增成員 Modal 改為 Select multiple，支援一次新增多位成員

- [x] **Proxy Lua Plugin 鏈完善化** — commit `270ea8b1` + `fb15630c`
  - access.lua: JWT validation (validate_jwt), consumer auth (key-auth/basic-auth/hmac-auth via /internal/validate-cred), OPTIONS preflight, route matching, load balancing (roundrobin/leastconn/weighted-ip-hash)
  - header_filter.lua: Kong-compatible headers (Via, X-Kong-Proxy-Latency, X-Kong-Upstream-Latency), CORS env support, consumer info headers (X-Consumer-ID, X-Credential-Identifier)
  - body_filter.lua: JSON pretty-print (CONT_JSON_PRETTY), error wrapping (CONT_WRAP_ERRORS)
  - log.lua: structured JSON/text access logging (CONT_LOG_FORMAT/CONT_LOG_LEVEL), Prometheus metrics (nginx_requests_total, nginx_request_latency_ms, route/consumer labels), plugin log() chains
  - **Bug fix `23a13478`**: OpenResty Alpine 無 resty.http，移除並改用 ngx.location.capture 子請求；修復 `require("cont.status")` → `require("status")` 路徑問題（package.path 已含 cont/ 前綴）；init_by_lua_block 直接初始化 `_G.cont` 避免 module 載入 loop

- [x] **Upstreams 健康檢查 UI** — commit `177ed99b`
  - Backend: GET /upstreams/:id/health endpoint reading from Redis health keys (cont:health:{upstream}:*)
  - storage/redis.go: GetTargetHealthStatuses() + Redis() accessor
  - routes/routes.go: GetUpstreamHealth handler (returns upstream + targets with healthy flag)
  - Frontend: HealthPortal.tsx complete rewrite — upstream cards grid, summary stats, target health table, detail modal with live refresh
  - kong.ts: KongUpstream/TargetHealth/UpstreamHealth types + listUpstreams/getUpstream/getUpstreamHealth API
  - Users.tsx: fix WorkspaceOutlined → TeamOutlined (icon doesn't exist in antd)

- [x] **Metrics Dashboard** — commit `7cbabd63`
  - monitoring/prometheus/prometheus.yml: scrape cont-admin-api:8001/metrics every 10s
  - monitoring/grafana/dashboards/cont-overview.json: 6-panel Grafana dashboard
    (Request Rate, Latency, DB/Redis connections, Memory, Goroutines, Throughput)
  - monitoring/grafana/datasources.yaml: Prometheus datasource (proxy access, 10s interval)
  - monitoring/grafana/dashboards/dashboards.yaml: file-based dashboard provider
  - monitoring/docker-compose.monitoring.yml: local Prometheus v2.53 + Grafana 11.2 stack
  - QA: Prometheus scrape ✅ (cont-admin-api: up), Grafana ✅ (HTTP 200), dashboard provisioned ✅
  - 使用方式：`docker compose -f monitoring/docker-compose.monitoring.yml up -d`

- [x] **API Key 申請 Flow 完整化** — commit `8345bcd6`
  - ApiKeyRequests.tsx：申請表單（reason、scope、expire）、狀態追蹤（pending/approved/rejected）
  - 後端：核發後自動產生 key，寄送 email / Slack 通知
  - Admin：審批介面支援批次核准/拒絕多筆
  - Bug fix: GetAPIKeyRequest SQL scan 漏掉 key_value 欄位賦值，導致核准後 API 回傳 key_value: null

- [x] **Cont 單元測試覆蓋率提升（admin-api HTTP handlers integration tests）** — commit `a85f2072` + `fa236442`
  - integration/integration_test.go: 完整 HTTP 測試套件（Services/Routes/Upstreams/Targets/Consumers/Plugins/Workspaces/Users/AuthGroups/APIKeys/ConfigSnapshots/AuditLogs CRUD）
  - integration/integration.go: TestMain、seedTestUser、adminReq/makeAdminEditorToken helpers
  - storage/rbac.go: PermissionMatrix 新增 api_keys/config_snapshots/audit_logs（全部三個角色）
  - models.go 驗證修正：Upstream.Name required→omitempty、Upstream.Algorithm oneof 值修正、Target.Target required→omitempty（支援 PATCH partial updates）
  - QA: Upstreams Create→201 ✅, Read→200 ✅, PATCH→200 ✅, Delete→204 ✅
  - QA: Targets Create→201 ✅, PATCH weight→200 ✅, Delete→204 ✅
  - QA: Consumers Create→201 ✅, Delete→204 ✅; AuthGroups Create→201 ✅, Delete→204 ✅

- [x] **Cont SaaS Phase 2：Workspace 綁定 Organization + 多租戶資料隔離** — commit `d390a3b6`
  - Phase 1 已完成（Organization 資料模型、users.org_id、OTP 註冊 flow）
  - Phase 2 step 1-4 完成：Services/Routes/Upstreams/Consumers/Plugins/Workspaces org_id 過濾
    - store.go: 所有 CRUD methods 新增 orgID 參數與 WHERE org_id 過濾
    - routes.go: 所有 handlers 透過 getOrgID(c) 傳入 orgID，admin bypass
    - QA: Workspace Create→201 ✅, Read→200 ✅, Update→200 ✅, Delete→204 ✅
  - Phase 3 待做：plan/billing/Stripe 整合

## ✅ 已完成

- [x] **行事曆認證問題（COALESCE UUID bug）** — commit `95be3fc8`
  - store.go: 所有 COALESCE(org_id, '') → COALESCE(org_id, '00000000-0000-0000-0000-000000000000')
  - UUID columns cannot COALESCE with empty string in PostgreSQL
  - GetUserByUsername Login handler debug fields 增強
  - QA: Login → 200 ✅, /auth/me → 200 ✅ (含正確 permissions)

- [x] **Workspace 使用者管理 UI（Workspace 級 RBAC）** — commit `b2544b7b` + `1d74dfb3` + `3f97b2d3`
  - Backend: `ListWorkspaceUsers` store method + `ListWorkspaceUsers` handler (GET /workspaces/:id/users)
  - Backend: `ListWorkspaceUsers` 確保回傳 `[]` 而非 `null`
  - API: GET /workspaces/:id/users → 200 ✅, PUT /workspaces/:id/users → 200 ✅, DELETE /workspaces/:id/users/:userId → 204 ✅
  - Frontend: Workspaces.tsx（列表/建立/刪除 Workspace）、WorkspaceDetail.tsx（成員管理：新增成員/變更角色/移除成員）
  - App.tsx: /workspaces 和 /workspaces/:id 路由
  - kong.ts: `WorkspaceUserAssignment` interface + `getWorkspaceUsers/setWorkspaceUser/removeWorkspaceUser` API calls

- [x] **Users 頁面黑屏 + 多頁面 `.map is not a function`** — commit `955de998` + `45ee1525`
  - WorkspaceContext: `listMyWorkspaces()` / `listWorkspaces()` 加 `Array.isArray` normalize 回 array
  - kong.ts api 層統一 normalize：`api.listServices/Routes/Plugins/Consumers` 直接回 `data` array
  - 移除 `getApiKeyRequests`、`getHealthServices`、`getStatsOverview` 三個不存在後端端的 dead API calls
  - QA: services/consumers/routes/plugins LIST → 200 ✅, CREATE → 201 ✅, DELETE → 204 ✅

- [x] **API path 後端不存在（404）+ HealthPortal 修復** — commit `45ee1525` + `74891876` + `fdc9dc97`
  - ApiKeyRequests: `/apikeys/requests` → `/api-keys`（後端實作於 `/api-keys`）
  - HealthPortal: 移除 `/health/services`、`/health/summary`、`/health/check`（後端無此端點）
  - 改用 `/services` 端點顯示服務狀態（unknown）
  - handleCheck / handleEditSave 改為 no-op（後端無實作）

- [x] **Docker build layer cache 導致 JS 未更新** — 手動 `docker builder prune` + `docker build --no-cache` 解決
  - `docker compose build` 未清除 builder cache，導致新 dist 內容未寫入 image
  - 解決：先 `npm run build` → `docker builder prune` → `docker build --no-cache --force-rm` → recreate container

- [x] **Workspace 多租戶隔離（RBAC + 資料隔離）** — commit `3c80a047` + `82ab87ca`
  - `workspace_auth_groups` table（workspace_id, auth_group_id, role）
  - `user_workspaces` table（user_id, workspace_id, role）
  - Backend: `RequireWorkspacePermission` middleware (admin bypass, workspace role → level 1/2/3)
  - Backend: `SetUserWorkspace`/`RemoveUserWorkspace`/`GetUserWorkspaces` handlers
  - `GET /workspaces/:id` 檢查 user-workspace 存取（admin bypass），未授權 → 404
  - `GET /workspaces/mine` — admin 看到所有，非admin只看已指派workspace
  - 前端: Sidebar workspace switcher 修正 `w.label → w.name`
  - Frontend: `kong.ts` WS_KEY 統一 `cont_ws → kgo_ws`（與WorkspaceContext一致）
  - QA: admin所有workspace ✅, editor只看已指派 ✅, editor取未指派→404 ✅, PUT/DELETE/GET user assignment ✅

- [x] **Consumer Credentials 管理（KeyAuth / BasicAuth / HMACAuth）** — commit `ea2053af`
  - Backend: `consumer_credentials` table（type, consumer_id, key, secret, enabled, created_at）
  - API: GET/POST/DELETE /consumers/:id/key-auth|basic-auth|hmac-auth/credentials
  - Store: ListConsumerCredentials/CreateConsumerCredential/GetConsumerCredentialByKey/DeleteConsumerCredential
  - Proxy access.lua: validate_consumer_auth() + has_consumer_auth() middleware
  - Internal: GET /internal/validate-cred/:type/:key（無認證）for proxy auth validation
  - QA: List→200 ✅, Create→201 ✅, Validate→200 ✅, Delete→204 ✅
- [x] **Cont Auth OAuth2/OIDC SSO（Google provider）** — commit `a8a3c0f8`
  - Backend: `oauth_providers` + `oauth_states` tables, `oauth_provider`/`oauth_subject` users columns
  - `routes/oauth.go`: `ListOAuthProviders()`/`InitiateOAuth()`/`HandleOAuthCallback()` — 完整 authorization code flow
  - State stored in DB (10min expiry), CSRF protection, auto-provisioning of OAuth users
  - Frontend: `Login.tsx` OAuth2Service redirect flow, `handleOAuthCallback()` URL token parsing
  - 動態 OAuth provider 按鈕（從 `/auth/oauth/providers` 讀取），自動檢測 URL token
  - QA: GET /auth/oauth/providers → 200 ✅, GET /auth/google → 404 (no provider configured) ✅
- [x] **登入安全：登入頻率限制 + 暴力破解防護** — commit `99f7b99a`
  - `login_attempts` table（username, ip_address, success, attempted_at）+ 兩個 index
  - Store: `RecordFailedLogin()`/`ClearFailedLogins()`/`IsLockedOut()`/`GetLoginAttemptsByIP()`
  - Login handler: lockout 檢查（5 次失敗 / 60 秒視窗 → 300 秒鎖定）、失敗記錄、成功清除
  - Go build ✅, Docker build ✅, QA: login✅, 5x fail→429✅, locked時正確密碼亦阻擋✅
- [x] **使用者-群組指派 UI（群組 name 支援）** — commit `5c586bf6`
  - 修復：/groups/:id/members API 原本只接受 UUID，前端傳入 group name 導致 pq error
  - store.go: 新增 GetAuthGroupByName()，可依 name 查詢 auth group
  - routes.go: resolveGroupID() 同時支援 UUID 或 name 解析
  - QA: GET /groups/test-group/members → 200 ✅, PUT → 200 ✅, admin 成功加入 qa-group ✅
- [x] **AuthGroups 群組成員管理（Backend + Frontend UI）** — commit `96c74dd9`
  - Backend: `user_auth_groups` table migration, `ListGroupMembers()`/`SetGroupMembers()` store methods, `GET/PUT /groups/:id/members` routes
  - Frontend: Groups.tsx tabbed modal（Permissions + Members tabs），`getGroupMembers()`/`setGroupMembers()` API calls
  - QA: GET members →200 ✅, PUT set members → 200 ✅, Create group → 201 ✅
  - 修復：admin密碼 hash 無法通過 bcrypt 驗證（重建 hash 後登入成功）
- [x] **Audit Log ActorUserID 修補（8 個 audit log blocks）** — commit `740a925d`
  - 修補 User/AuthGroup/APIKeyRequest 全部 8 個 audit log blocks：新增 `ActorUserID` 欄位、修正 `TargetID` 從 string id 而非 int
  - 修補：CreateUser、UpdateUser、DeleteUser、CreateAuthGroup、UpdateAuthGroup、DeleteAuthGroup、ApproveAPIKey、RejectAPIKey、DeleteAPIKeyRequest
- [x] **Cont 使用者管理精細化（AuthGroups 群組管理、API Key 審批流程、Audit Log 查詢介面）** — commit `633746a1` + `e6d90ccf`
  - AuthGroups: GET/POST/PUT/PATCH/DELETE 全部可用
  - API Keys: GET/POST/PUT/PATCH/DELETE 全部可用（CreateAPIKeyRequest 使用 int64 id、GetAPIKeyRequest NULL scan 修復）
  - Audit Log: Backend `/audit` endpoint + Frontend `AuditLog.tsx` 查詢介面（過濾/排序/分頁）
  - Backend `ListAuditLogs()` + 前端 `getAuditLogs()` API 已完整串接
- [x] **API Key 審批流程通知（Slack/Email Webhook）** — commit `c4948e4b`
  - `SendAPIKeyApprovalNotification()` 非同步 goroutine，ApproveAPIKey/RejectAPIKey 後觸發
  - `notifyWebhook()` 通用 helper，支援 Slack 與 Email webhook URL
  - 環境變數：`SLACK_WEBHOOK_URL`（Slack）、`EMAIL_WEBHOOK_URL`（Email relay，如 Mailgun/SendGrid）
  - Go build ✅ 編譯通過
- [x] **Cont 自動化部署腳本增強（Kustomize base/overlays 結構）** — commit `9f2d8c3a`
  - k8s/base/kustomization.yaml：base 資源重構為 Kustomize 格式，namespace/commonLabels/configMapGenerator/secretGenerator
  - k8s/overlays/dev/kustomization.yaml：dev overlay（replicas=1、debug log、local image pull）
  - k8s/overlays/prod/kustomization.yaml：prod overlay（HA replicas、resource limits、PodDisruptionBudget、CHANGEME secrets 提示）
  - Makefile 新增：k8s-apply（kubectl apply -k base）、k8s-dev-apply、k8s-prod-apply、k8s-dev-diff、k8s-prod-diff、k8s-delete（kubectl delete -k）
  - k8s/README.md：完整使用文件（prerequisites、usage、sealed-secrets、resource scaling table）

- [x] **Groups/Alert Rules/API Keys CRUD + Config Snapshots/Health/ConfigCheck 端點實作** — commit `633746a1`
  - Groups: GET/POST/PUT/PATCH/DELETE 全部可用，PATCH 改為 partial update
  - Alert Rules: GET/POST/PUT/PATCH/DELETE 全部可用，PATCH 改為 partial update
  - API Keys: GET/POST/PUT/PATCH/DELETE 全部可用
    - APIKeyRequest.Status binding: `required` → `omitempty`（CreateAPIKeyRequest 預設 `pending`）
    - GetAPIKeyRequest: 修復 applicant_user_id/applicant_username NULL scan
    - UpdateAPIKeyRequest: 使用 int64 id，呼叫 GetAPIKeyRequest 回傳完整更新後資料
    - DeleteAPIKeyRequest: 使用 int64 id
  - Config Snapshots: GET(list)/POST/DELETE 全部可用
  - Health Check: GET /health-check ✅
  - Config Check: GET /config-check ✅
- [x] **Plugin System 完整化** — commit `caac9e45`
  - 修復 ALL_PLUGINS bug：移除 `-correlation-id`（dash prefix）、`ip-v41`（typo）、`logging`（non-plugin）
  - 新增 24 個真實 Kong plugin types（auth、security、caching、observability、logging、transformation、sessions）
  - 新增 14 個 plugin schemas：oauth2、rate-limiting-advanced、proxy-cache-advanced、gzip、websocket-size-limit、request-transformer、response-transformer、correlation-id、session、syslog、loggly、bot-detection、ldap-auth、acl（schema 修復）
  - 前端 build ✅ 驗證通過

## ✅ 已完成

- [x] **API Input Validation & Sanitization** — commit `8af8f9bf`
  - models.go: 為所有 entities 新增 binding tags（required, max, min, oneof, url, email）
  - 新增 routes/validation.go：註冊 `fqdn` 和 `host_port` 自定義 validators
  - routes.go: 新增 `badRequest()` helper，結構化 validation error 回應（`{message, errors[]}`）
  - 15 個 ShouldBindJSON error handlers 全面更新為 badRequest()
  - `IsValidTarget()` / `IsValidHostname()` / `SanitizeString()` 等輔助函數用於程式碼層驗證
  - Store 層 SQL 已全面參數化（無 SQL injection 風險）
- [x] **Editor 角色的 targets/upstreams Delete 權限確認**

## ✅ 已完成

- [x] **Editor 角色的 targets/upstreams Delete 權限確認** — commit `97fcb767`
  - buildPermissions() 修復：移除 `canD && e != "targets"` guard，使 targets 的 level=3（等同 admin 刪除權限）
  - editor 在 PermissionMatrix 的 targets.Delete=true 正確反映到前端 canDelete 判斷（perm.level >= 3）
  - upstreams.Delete=false（editor 無法刪除 upstream）已驗證正確，後端 RequirePermission("upstreams", true) 會阻擋
- [x] **前端無使用者管理頁面（Users.tsx）** — commit `e6d90ccf`
  - Users.tsx 原本存在但後端無 /users CRUD routes，導致 404
  - 新增 store.go: ListUsers/GetUser/UpdateUser/DeleteUser + GetUserByUsername NULL fix
  - 新增 routes.go: ListUsers/GetUser/CreateUser/UpdateUser/DeleteUser handlers
  - main.go: /users GET/POST/PUT/PATCH/DELETE routes (RequirePermission users)
  - QA: GET→200 ✅, POST→201 ✅ (可立即登入) ✅, PATCH→200 ✅, DELETE→204 ✅
- [x] **前端 canDelete 未區分 editor 角色** — commit `e75f574b`
  - AuthContext.tsx canDelete 僅 return user.role === 'admin'，忽略 PermissionMatrix
  - 修復：使用 perm.level >= 3 判斷（level 3 = admin），與 canWrite 一致的邏輯
  - Editor now correctly sees delete buttons for services/routes/consumers/targets
- [x] **RBAC 細粒度權限整合前端** — commit `c2fa9254` + `c9b9ff08`
  - 前端根据用户 role 动态显示/隐藏操作按钮，viewer 角色禁止显示 Create/Delete/Edit 按钮
  - AuthContext.tsx: canWrite/canDelete 依权限控制；GET /auth/me 提供 per-entity permissions
  - Backend RequirePermission 已正確阻擋 editor 對 plugins/upstreams 的 PUT/PATCH/DELETE（QA: 全部 403 ✅）
  - 發現 Login/GetMe 回傳的 permissions 誤導前端（editor 所有 entities 都報 mode=rw/level=2）
  - 新增 buildPermissions() 以 CanWrite/CanRead/CanDelete 正確計算每 entity 的 level 與 mode
  - Editor now correctly sees: plugins/upstreams → level=1/mode=r; services/routes/consumers → level=3/mode=rwd
- [x] **RBAC GET 端點補全** — commit `ecf213e8`
  - 所有 GET 端點（services, routes, upstreams, targets, consumers, plugins, workspaces）新增 `RequirePermission(entity, false)` 檢查
  - 之前只有 POST/PUT/PATCH/DELETE 有 RBAC，GET 完全開放 — viewer/editor 可讀取不應讀取的 entities
  - QA 驗證：viewer POST /services → 403 ✅，DELETE → 403 ✅；viewer GET /services → 200 ✅
- [x] **Workspace CRUD 完整化** — commit `ecf213e8`
  - 新增 `store.UpdateWorkspace()`, `store.DeleteWorkspace()` 方法
  - 新增 `routes.UpdateWorkspace()`, `routes.DeleteWorkspace()` handler
  - main.go 新增 PUT/PATCH/DELETE 端點（原本只有 GET/POST/List）
  - QA 驗證：PATCH → 200 ✅，PUT → 200 ✅，DELETE → 204 ✅
- [x] **CI/CD Pipeline 建置** — GitHub Actions CI 已完整建置（go-test, lua-test, frontend, docker, compose-test），commit `6221240d` + `1baa102f`
  - 修復：busted 僅執行 `*_test.lua`（隔離 source files）、compose-test 預先建立 `cont_default` network
- [x] **proxy/lua 深度重構** — access_test.lua（9 tests）、header_filter_test.lua（6 tests）、healthcheck IPv6 bug fix，commit `bf023d62` + `6d43f4e3` + `73bc8241` + `68fbf01a`
  - access.lua 路由匹配 +負載平衡單元測試
  - header_filter.lua 重構為回傳函式（避免載入時執行）、nginx.conf 整合修復
  - healthcheck.lua IPv6:port 解析正確化（`[::1]:8080` → host=`::1`, port=`8080`）
  - healthcheck_test.lua IPv6 測試更新，header_filter_test.lua 新增
  - 全部 40 Lua測試通過
- [x] **單元測試覆蓋率提升（Go：admin-api ✅ / Lua：proxy ✅）** — 為 admin-api (Go) 和 proxy (Lua) 核心模組建立單元測試，提升程式碼品質與回歸防護
  - Go ✅ — admin-api/storage/models_test.go（14 tests）、admin-api/routes/routes_test.go（8 tests）、admin-api/routes/auth_test.go（7 tests）→ 共 29 tests，commit `c4b52414` + `d06594e3`
  - Lua ✅ — proxy/lua/cont/metrics_test.lua（6 tests）、status_test.lua（9 tests）、healthcheck_test.lua（8 tests）→ 共 23 tests，commit `05c27cef`。busted 測試框架、lua-cjson 安裝完成
- [x] **Swagger/OpenAPI 文件生成** — commit `c8fa5b71`，admin-api/docs/swagger.yaml（Swagger 2.0，覆蓋 /services, /routes, /upstreams, /consumers, /plugins, /workspaces 全端點，JWT Bearer 認證，Kong-compatible schemas），main.go 新增 /docs 和 /docs.json 端點（無需認證），Dockerfile 複製 docs/ 目錄。QA: /docs ✅ 200, /docs.json ✅ 200, Auth flow ✅
- [x] **生產部署評估** — commit `ce5582e4`，移除 hardcoded IP、Docker DNS resolver、admin-api healthcheck、proxy health-based startup、JWT_SECRET 支援、docker-compose.prod.yml（PG WAL、AOF、記憶體限制）、Makefile 生產目標（build-prod/push-prod/deploy-prod/roll/db-backup）、.gitignore 修復、移除過時 version 欄位
- [x] **Prometheus Metrics實作** — commit `83832e8f`，prometheus/client_golang 接入，定義 Kong-相容 metrics（kong_nginx_requests_total, kong_nginx_connections_total, kong_service_latency_ms, cont_db_*, cont_redis_*），QA: /metrics ✅ HTTP 200，包含 Go runtime + 自定義 metrics
- [x] **Cont Auth 正式實作（JWT / bcrypt）** — commit `cfd7f15d`，users table、AuthRequired middleware、JWT 登入廢除 mock
- [x] **Routes Create 不支援 service.name 格式** — commit `0a6e2060`，models.go ServiceRef + GetServiceName()，store.go GetServiceByName()，routes.go CreateRoute 解析 service.name → service.id（QA: Create ✅ Read ✅ PATCH ✅ Delete ✅）
- [x] **cont-admin-api container 未加入網路** — commit `295379b7`，docker-compose.yml 新增 networks: cont_default，確保網路正確連接
- [x] **cont-admin-api 未持久化** — commit `295379b7`，新增 restart: unless-stopped
- [x] **Services/Plugins/Routes Update PATCH 404** — commit `0cd501a0`，routes.go 缺少 PATCH 路由，已新增全部 6 個實體的 PATCH
- [x] **Services Create/Edit modal 不關閉** — commit `04125706`，移除 Modal onOk 改用 Form submit
- [x] **Plugins Create/Edit modal 不關閉** — commit `886f75d5`，同上模式
- [x] **Routes Create/Edit modal 不關閉** — commit `5790860f`，同上模式
- [x] **Services Create/Edit/Delete API QA** — Create ✅ Read ✅ Delete ✅（Update PATCH ✅）
- [x] **Plugins Create/Delete API QA** — Create ✅ Delete ✅（Update PATCH ✅）
- [x] **Routes Create/Delete API QA** — Create ✅ Read ✅ Delete ✅（Update PATCH ✅）
- [x] **Users.tsx API 修復 + role/enabled 管理** — commit `e304fbe9`
  - Users.tsx 原本使用錯誤的 `/api/users` 路徑，正確為 `/users`
  - 移除自定義 axios userClient，改用 kong.ts API 函數（getUsers/createUser/updateUser/deleteUser）
  - 新增 createUser 到 kong.ts exports
  - 新增 role/enabled 欄位至 User介面、表格、編輯 Modal
  - QA: GET→200 ✅, POST→201 ✅, PUT→200 ✅, DELETE→204 ✅
- [x] **k8s/ 目錄完整 manifests（Namespace + ConfigMap/Secret + postgres + redis + admin-api + frontend + proxy）** — commit `3f8e2a1b`
  - k8s/ 包含 9 個 YAML：namespace.yaml、config.yaml（含 ConfigMap + Secret）、postgres.yaml + postgres-svc.yaml、redis.yaml + redis-svc.yaml、admin-api.yaml、frontend.yaml、proxy.yaml
  - 全部 Deployment 含 livenessProbe/readinessProbe、资源限制、镜像拉取策略
  - postgres 使用 emptyDir 持久化卷（測試用）
  - proxy 使用 LoadBalancer Service 其餘使用 ClusterIP
- [x] **proxy/Dockerfile：OpenResty proxy 映像檔建置** — commit `3f8e2a1b`
  - proxy/Dockerfile 基於 openresty/openresty:alpine，複製 lua/ 和 nginx.conf
  - QA: `docker build -t cont-proxy:test ./proxy` ✅ 建置成功
- [x] **deploy.sh：前置檢查、影像建置+推送（REGISTRY）、JWT_SECRET 自動生成、apply/delete 生命周期、rollout status** — commit `3f8e2a1b`
  - 前置檢查：kubectl 存在性 + cluster 可達性
  - REGISTRY 環境變數控制是否建置+推送
  - JWT_SECRET 未設定時自動生成（openssl rand -hex 32）
  - `./deploy.sh apply` 依賴順序 apply 所有 manifests + rollout status
  - `./deploy.sh delete` 清理所有資源
- [x] **Makefile 新增：k8s-apply、k8s-delete、k8s-status、k8s-logs、k8s-port-forward targets** — commit `3f8e2a1b`
  - k8s-apply：依賴順序 apply + rollout status
  - k8s-delete：清理所有 k8s 資源
  - k8s-status：顯示 pods 和 svc 狀態
  - k8s-logs：tail 所有 pods logs
  - k8s-port-forward：轉發 admin-api/proxy/frontend 端口到本機
- [x] **支援 `make deploy-prod` 搭配 REGISTRY + JWT_SECRET 環境變數一鍵部署至 Kubernetes** — commit `3f8e2a1b`
  - Makefile build-prod/push-prod targets 支援 VERSION（git hash）+ REGISTRY
  - deploy.sh 自動 patch image tag 注入 k8s manifests
  - `make deploy-prod` 使用 docker-compose.prod.yml

---

## 完整開發流程（開發守護遵循）

1. **開發執行** → 逐項處理，每修完一個問題即時 commit
2. **QA 測試** → curl / browser 驗證 Create/Read/Update/Delete 基本流程
3. **寫入 event.md** → 發現的 Bug 加入 🔴 未完成，完成的移至 ✅
4. **commit** → 每個變更单独 commit
5. **push** → 確認所有改動入庫

> 一輪：開發 → QA → Bug 寫入 event.md → commit → push = 完整回合