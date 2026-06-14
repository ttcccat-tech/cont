# SPEC-2.5-B: Cont 使用量預警自動化（Usage Alerting）

## 背景
當 org 或 consumer 接近配額時（80%/90%/100%），需要自動觸發 webhook/SSE 通知。目前 alerter.go 只評估 AlertRules，沒有檢查用量。

## 目標
- Backend: alerter.go 評估週期中一併檢查用量，接近 80%/90%/100% 門檻時 fire alert
- Frontend: AlertRules.tsx 新增 `usage_quota` alert type（threshold_type: percentage absolute）

## Scope
### In-scope
- Backend: alerter.go 新增 usage quota check（檢查 org/consumer 用量 vs plan quota）
- Backend: 用量超標時 fire alert（寫入 AlertHistory + 觸發 webhook + SSE）
- Frontend: AlertRules.tsx 新增 `usage_quota` alert type（新增一條 rule 可選用）
- Frontend: alert type 為 `usage_quota` 時，threshold 為百分比（80/90/100）

### Out-of-scope
- 不做用量寫入（Redis hourly buckets 已存在）
- 不做新的 storage schema（AlertRule conditions 結構已支援 percentage）
- 不做 notification channel 新增（使用現有 Slack/Discord/Email webhook）

## 驗收標準
1. alerter.go 評估時檢查所有 org 的當月用量 vs quota，≥80% fire alert
2. 用量 alert 寫入 AlertHistory（type=usage_quota, org_id, current_usage, quota）
3. AlertRules.tsx 新增 alert type 選擇：`usage_quota`
4. `usage_quota` alert 的 threshold 為百分比（80/90/100%）
5. 用量 alert 觸發 webhook（`alert.triggered` event）並送至 webhook_delivery engine

## Tasks
- [ ] TASK-2.5-B1: Backend — alerter.go usage quota check (evaluate org usage vs quota, fire alert at 80/90/100%)
- [ ] TASK-2.5-B2: Frontend — AlertRules.tsx add `usage_quota` alert type with percentage threshold UI
