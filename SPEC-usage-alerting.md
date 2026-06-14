# SPEC: Cont 使用量預警自動化（Usage Alerting）

## 1. 背景

當前 Cont 系統已具備：
- Backend `alerter.go` 有 `evaluateUsageQuota()`，每 30s 對所有 org 自動執行 80% 門檻檢查（hardcoded，不可配置）
- Frontend `AlertRules.tsx` 有 `usage_quota` metric type 下拉選單 + `quota_metric_type`（org/consumer）下拉，但未傳送至 backend
- `AlertRule` model 已有 `ThresholdType`（absolute/percentage）和 `PercentageThreshold` 欄位閒置未用

**問題**：用量預警只能 80% 單一門檻，無法自訂（80%/90%/100%），無法針對 consumer 級別偵測，UI 與 backend 脫節。

## 2. 目標

實作可配置的用量預警系統：
- 使用者可在 AlertRules.tsx 建立 `usage_quota` 類型規則，自訂門檻（80%/90%/100%）和對象（org/consumer）
- Backend alerter.go 評估 `usage_quota` 規則時讀取 `PercentageThreshold` 和 `quota_metric_type`（org/consumer）
- 廢除 hardcoded 80% 自動檢查，改為由使用者建立的 AlertRule 驅動

## 3. Scope

### In-Scope
- Backend: `alerter.go` 廢除 hardcoded `evaluateUsageQuota()`，改為在 `evaluateRule()` 的一般規則評估流程中處理 `usage_quota`
- Backend: 新增 `computeConsumerUsageQuotaMetric()` 支援 consumer 級別用量百分比計算
- Backend: `AlertRule` 的 `PercentageThreshold`（預設 80）和 `quota_metric_type`（org/consumer）寫入/讀取
- Frontend: AlertRules.tsx 表單送出時正確攜帶 `quota_metric_type` 和 `percentage_threshold`欄位
- Frontend: AlertRules.tsx 列表顯示 `usage_quota` 規則的 threshold 數值（%）

### Out-of-Scope
- AlertRule CRUD API 實作（已存在）
- SSE 通知和 webhook delivery（已存在）
- Redis 用量 counter 實作（已存在）
- Consumer 用量追蹤（已存在，Redis 介面已實作）

## 4. 驗收標準

1. **門檻可配置**：建立 `usage_quota` 規則時可選 80%/90%/100% 門檻，backend 正確讀取 `PercentageThreshold`
2. **Org 級別**：選擇 `quota_metric_type=org` 時，alerter 對該 org 計算用量百分比並評估
3. **Consumer 級別**：選擇 `quota_metric_type=consumer` 時，alerter 對該 consumer 計算用量百分比並評估
4. **Hardcoded 廢除**：移除 `evaluateUsageQuota()` 自動全 org 掃描，既有的 80% 行為改由使用者建立規則
5. **UI 正確送出**：AlertRules.tsx POST/PUT 時攜帶 `quota_metric_type` 和 `percentage_threshold`
6. **規則評估正確**：`usage_quota` 規則在 alerter evaluate loop 中被正確評估（≥threshold 觸發）
7. **SSE + Webhook**：規則觸發時正確廣播 SSE + 寫入 AlertHistory + 觸發 webhook

## Tasks

- [ ] TASK-UA-1: Backend alerter.go — 廢除 evaluateUsageQuota() hardcoded掃描，改為在 evaluateRule() 中處理 usage_quota（讀取 PercentageThreshold）
- [ ] TASK-UA-2: Backend alerter.go — 新增 computeConsumerUsageQuotaMetric()，支援 quota_metric_type=consumer 時計算該 consumer 用量百分比
- [ ] TASK-UA-3: Backend store.go — 確認 AlertRule CRUD 正確讀寫 PercentageThreshold 和 quota_metric_type（新增欄位 migration 如需要）
- [ ] TASK-UA-4: Frontend AlertRules.tsx — 表單 POST/PUT 時正確攜帶 quota_metric_type 和 percentage_threshold 欄位
- [ ] TASK-UA-5: Frontend AlertRules.tsx — 列表條件顯示 usage_quota 規則時正確顯示門檻百分比
- [ ] TASK-UA-6: QA 驗證 — usage_quota 規則建立→觸發→SSE+broadcast+AlertHistory 完整流程
