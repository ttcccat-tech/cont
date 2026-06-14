# SPEC-2.5-A: Cont 自有用量追蹤整合 Analytics 儀表板

## 背景
Analytics.tsx 目前只顯示 Kong Nginx metrics（nginx_requests_total 之類），沒有整合 Cont 自有的 Redis 用量數據（per-org/per-consumer/per-route hourly buckets）。需要讓使用者看到有意義的用量儀表板。

## 目標
- Backend: `GET /usage/analytics` endpoint（彙整 monthly total、plan quota、top routes、top consumers）
- Frontend: 新增 Cont 用量 panel（usage vs quota progress、hourly trend、top entities）
- kong.ts: 新增 `getAnalyticsUsage()` API call

## Scope
### In-scope
- Backend: `/usage/analytics` endpoint（monthly total, plan quota, top routes, top consumers）
- Frontend: Analytics.tsx 新增 Cont 用量 panel（usage bar, hourly trend chart, top entities table）
- kong.ts: `getAnalyticsUsage()` API function

### Out-of-scope
- 不做新的用量寫入（Redis hourly buckets 已存在）
- 不做前端 charts library 改動（使用現有 chart 元件）
- 不做 alerting（那是另一個 issue）

## 驗收標準
1. `GET /usage/analytics?org_id=X` → 200，返回 `{ monthly_total, plan_quota, usage_percentage, top_routes[], top_consumers[] }`
2. Analytics.tsx 渲染 usage vs quota progress bar
3. Analytics.tsx 渲染 hourly trend（24小時折線圖）
4. Analytics.tsx 渲染 top routes / top consumers table
5. `GET /usage/analytics` without org_id → 400（org_id required）
