# SPEC: Cont 自有用量追蹤整合 Analytics 儀表板

## 背景

Analytics.tsx 目前只顯示 Kong Nginx metrics（nginx_requests_total 等），缺乏對 Cont 自有用量數據（per-org/per-consumer/per-route hourly Redis buckets）的呈現，管理者無法看到「誰用了多少配額」的實際使用情形。

## 目標

- Backend: `GET /usage/analytics` endpoint，回傳彙整後的用量分析數據
- Frontend: Analytics.tsx 新增 Cont 用量 panel（usage vs quota progress、hourly trend、top entities）
- kong.ts: 新增 `getAnalyticsUsage()` API call

## Scope

### In-scope
- `GET /usage/analytics` Backend endpoint（monthly total、plan quota、top routes、top consumers）
- Frontend Analytics.tsx 新增 Cont 用量 panel
- `kong.ts` 新增 `getAnalyticsUsage()` API

### Out-of-scope
- 不修改現有 Prometheus metrics 顯示方式
- 不修改 billing 用量計算邏輯（已有 `GetMonthlyUsage()`）
- 不實作 Alert Rules UI 變更（留給另一個 issue）

## 驗收標準

1. `GET /usage/analytics?org_id=...` 回傳 200，含 `monthly_total`、`plan_quota`、`top_routes[]`、`top_consumers[]`、`usage_percentage`
2. Analytics.tsx 成功渲染 Cont 用量 panel（usage bar + hourly trend + top entities）
3. `kong.ts` 的 `getAnalyticsUsage()` API call 正確
4. 當 org 無用量時，回傳空陣列而非錯誤

## 🔴 Bug（已修復）

`GetTopRoutesByUsage` 和 `GetTopConsumersByUsage` 的 key 解析曾錯誤使用 `parts[2]`：
- `cont:usage:route:{route_id}:{YYYYMMDDHH}` → `parts[2]` = `"route"` (literal)
- 已於 redis.go:304,362 確認使用 `parts[3]` ✅

## Tasks

- [ ] TASK-ANALYTICS-1: Fix `GetTopRoutesByUsage` key parsing — change `parts[2]` to `parts[3]` ✅ **FIXED** (redis.go:304)
- [ ] TASK-ANALYTICS-2: Fix `GetTopConsumersByUsage` key parsing — change `parts[2]` to `parts[3]` ✅ **FIXED** (redis.go:362)
- [ ] TASK-ANALYTICS-3: Verify `/usage/analytics` endpoint returns correct top_routes/top_consumers data ⏳ pending QA verification
