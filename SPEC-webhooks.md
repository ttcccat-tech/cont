# SPEC-webhooks: Webhook Reliable Delivery

## 背景

2.0 發布條件之一：**Webhook 可靠化**。`SPEC-v2.0` 要求：
- `webhook_deliveries` table：記錄每次 webhook 發送
- `webhook_subscriptions` table：支援多事件類型
- Go 非同步 worker：goroutine pool（最多 10 concurrent）
- 重試機制：失敗後 3 次重試（指數回退：1s → 5s → 30s）
- Admin API：`GET /webhooks` 列表、`GET /webhooks/:id/deliveries`、`POST /webhooks/:id/retry`

目前 `admin-api/storage/webhook.go` 有程式碼但 table 未建立，routes 未註冊，worker 未實作。

## 目標
1. 建立 `webhook_subscriptions` + `webhook_deliveries` table 並建立 CRUD
2. 建立 Webhook delivery worker（goroutine pool + 指數回退重試）
3. 在 `alerter.go` 觸發 webhook（alert.triggered 事件）
4. Admin API routes：list subscriptions, get deliveries, retry

## Scope
### In-scope
- `admin-api/migrations/`：新增 `v026_webhook_tables` migration
- `admin-api/storage/webhook.go`：確認 CRUD 正確實作
- `admin-api/routes/webhooks.go`：新增 webhooks REST API routes
- `admin-api/routes/webhooks.go`：Webhook worker 啟動 + delivery logic
- `admin-api/alerter.go`：在 `fireAlert()` 成功後呼叫 webhook
- Docker build --no-cache admin-api + restart

### Out-of-scope
- 不實作 UI（Webhook 設定 UI 在別的 issue）
- 不實作特定 URL 端點的 health check
- 不實作 Dead Letter Queue（DLQ）

## 驗收標準
1. `webhook_subscriptions` table 存在，CRUD 可正常運作
2. `webhook_deliveries` table 存在，delivery 記錄正確寫入
3. `GET /webhooks/:id/deliveries` 返回該 webhook 的 delivery 歷史
4. Webhook delivery failure → 3 次重試（指數回退 1s → 5s → 30s）
5. Alert fire → webhook 正確觸發並寫入 `webhook_deliveries`
6. Docker build --no-cache admin-api 成功
7. Container restart 後 webhook_deliveries 記錄正確

## Tasks

### [ ] TASK-WH-1: Migration — Create webhook tables
- 完成定義：`v026_webhook_tables` migration 執行後，`webhook_subscriptions` 和 `webhook_deliveries` tables 存在於 PostgreSQL
- File: `admin-api/migrations/` (new migration file)
- Tables:
  - `webhook_subscriptions`: id, org_id, url, event_types (text[]), secret, active, created_at, updated_at
  - `webhook_deliveries`: id, org_id, webhook_id, event_type, payload, status (text), attempts (int), last_error (text), response_body (text), created_at, delivered_at

### [ ] TASK-WH-2: Storage CRUD — Confirm webhook CRUD functions
- 完成定義：`CreateWebhookSubscription`, `ListWebhookSubscriptions`, `DeleteWebhookSubscription`, `ListWebhookDeliveries` 在 `storage/webhook.go` 存在且正確
- Files: `admin-api/storage/webhook.go`

### [ ] TASK-WH-3: Routes — Register webhook REST API routes
- 完成定義：`GET /webhooks` (list), `POST /webhooks` (create), `DELETE /webhooks/:id`, `GET /webhooks/:id/deliveries` 全部正常運作
- File: `admin-api/routes/webhooks.go` (new)
- Also register in `main.go` or appropriate router

### [ ] TASK-WH-4: Worker — Webhook delivery worker with retry
- 完成定義：Webhook worker 從 channel 接收 job，發送 HTTP POST，指數回退重試 3 次（1s → 5s → 30s），結果寫入 `webhook_deliveries` table
- Files: `admin-api/routes/webhooks.go` or dedicated `webhook/worker.go`
- Pool: max 10 concurrent goroutines

### [ ] TASK-WH-5: Alerter integration — Fire webhook on alert
- 完成定義：`alerter.go` 的 `fireAlert()` 成功後，將 webhook job 送入 worker queue
- File: `admin-api/routes/alerter.go`

### [ ] TASK-WH-6: Docker build — admin-api
- 完成定義：`docker compose build --no-cache cont-admin-api` 成功

### [ ] TASK-WH-7: Smoke test — Webhook end-to-end
- 完成定義：
  1. `POST /webhooks` 建立 subscription
  2. 手動 trigger alert
  3. `GET /webhooks/:id/deliveries` 有 delivery record，status=delivered
  4. 失敗重試：delivery record 有 attempts > 1
