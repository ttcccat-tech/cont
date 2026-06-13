# Cont 2.0 系統邊界（System Boundary）

**版本目標**：用量感知型 API Gateway（超限阻擋 + 可靠 Webhook）

**發布條件**：三大功能 QA 完全通過、無已知 Bug，方可 merge 到 main 分支

---

## ✅ 2.0 範圍內（In Scope）

### 1. 用量追蹤（Usage Tracking）
- 每次 Proxy 請求寫入 Redis：org_id / consumer_id / route_id / service_id / timestamp / latency / status_code
- Redis Sorted Set（按小時分桶）：`cont:usage:{org_id}:{YYYYMMDDHH}` → increment counter
- 新增 `GET /usage/org/:org_id` API：每日/每小時用量查詢（Prometheus 風格格式可選）
- 新增 `GET /usage/consumer/:consumer_id` API：該消費者的用量
- admin API：`GET /usage/summary` 全域或依 org 查用量排行

### 2. 超限阻擋（Over-limit Enforcement）
- `plans` table 的 `request_limit`（每小時）真實生效
- Rate Limit plugin 讀取 Redis 用量 counter，超限 → 429 + `X-RateLimit-Limit-Reached: true`
- 不同 plan 不同 quota：Free=1000/hr、Pro=50000/hr、Enterprise=unlimited
- 用量到達 80% 時在 response header 夾帶 `X-Usage-Warning: 80%`
- `organizations` table 新增 `request_limit` 和 `billing_cycle` 欄位

### 3. Webhook 可靠化（Reliable Webhooks）
- `webhook_deliveries` table：記錄每次 webhook 發送（url, payload, status, attempts, last_error, created_at）
- `webhook_subscriptions` table：支援多個事件類型（api_key.approved, api_key.rejected, alert.triggered, subscription.expired）
- Go 非同步 worker：goroutine pool（最多 10 concurrent）發送 webhook
- 重試機制：失敗後 3 次重試（exponential backoff：1s → 5s → 30s）
- Admin API：`GET /webhooks` 列表、`GET /webhooks/:id/deliveries` 送達歷史、`POST /webhooks/:id/retry` 重送
- Frontend：Webhook 設定 UI（AlertRules.tsx 已有 webhook URL 欄位，可擴充）

---

## ❌ 2.0 範圍外（Out of Scope）

以下功能留待 3.0，不再 2.0 開發週期內實作：

- SAML/OIDC 企業 SSO（SAML provider）
- 用戶自助後台（Customer Portal）
- Plan quota 彈性設定（不同 plan 不同 rate_limit，非的全域固定值）
- Subscription 暫停/取消 flow（Stripe Portal 串接）
- API Key 自動展期（key rotation）
- 團隊/子帳號管理（sub-account）
- 多貨幣計費
- OAuth2 / OIDC 自助設定 UI

---

## ✅ 已具備（Existing Capabilities）

- JWT Auth + RBAC + 多租戶隔離
- API Key 申請/審批/核發 完整流程
- OAuth2/OIDC SSO（Google OAuth）
- 消費者維度認證（key-auth/basic-auth/hmac-auth）
- Prometheus + Grafana 監控
- 審計日誌（Audit Log）
- Rate-limiting plugin（含 Redis sliding window）

---

## 發布檢查清單（Merge Criteria）

- [ ] 用量寫入 Redis 每小時 counter ✅
- [ ] `GET /usage/org/:id` 返回正確 JSON ✅
- [ ] `GET /usage/consumer/:id` 返回正確 JSON ✅
- [ ] Free plan 超限 → 429 + header ✅
- [ ] 用量 80% → X-Usage-Warning header ✅
- [ ] Webhook delivery 寫入 `webhook_deliveries` table ✅
- [ ] Webhook 失敗自動重試（3次，指數回退）✅
- [ ] `GET /webhooks/:id/deliveries` 可查到送達歷史 ✅
- [ ] 所有 endpoints curl QA 通過 ✅
- [ ] No regression：現有功能（Auth/RBAC/AuthGroups/API Keys/Consumers）依然正常 ✅
