# SPEC-REGRESSION-UE-1: IncrUsage Redis Write Silent Failure

## 背景

2026-06-16 小黑親自調查，確認 `POST /internal/usage/incr` 返回 `{"count":1,"success":true}` 但 Redis DBSIZE 恆為 0。

## 小黑深度調查結果（2026-06-16 04:00）

### 確認事實
- Source code (`admin-api/storage/usage.go`) 含正確 IncrUsage code（commit `194ee7b4`）
- Binary md5 in running container: `a70d5874d7d02e22d14b7633e2635f85` ✅
- Binary md5 in current Docker image (f601b5185b13): `a70d5874...` ✅（source sync 確認）
- Running container healthy: 7 minutes uptime
- Redis PING = +PONG ✅
- Redis DBSIZE = 0 across ALL 16 DBs ❌
- `POST /internal/usage/incr` returns `{"count":1,"success":true}` ❌（success but no write）

### 已排除
- ❌ Pipeline.Exec 語法問題（source code 正確）
- ❌ Redis 網路問題（PING = +PONG）
- ❌ Redis auth（無密碼）
- ❌ 時區（UTC hour format）
- ❌ key 衝突（INCR/HSET 已分離）
- ❌ binary/source desync（md5 一致）
- ❌ Docker image 過期（image 7 min ago, binary md5 matches source）

### 待查根因
懷疑：Pipeline.Exec 失敗但錯誤被 gin handler 吞掉（c.JSON 在 err != nil 分支 return，但 err 可能是 nil 而 pipe result 實際失敗）

## 目標

1. 確認 IncrUsage pipeline 實際是否執行 Redis 寫入
2. 修復 `POST /internal/usage/incr` 正確寫入 Redis
3. 驗證 `GET /internal/plan-quota/:consumer_id` current_usage 非零
4. Free plan 超限 → 429 + header

## Scope

### In-scope
- IncrUsage pipeline 根因修復
- storage/usage.go IncrUsage function 修復
- Docker build --no-cache cont-admin-api
- Container restart + 驗證 Redis 有 keys

### Out-of-scope
- 不實作新功能架構
- 不實作 Webhook retry

## Tasks

- [ ] TASK-UE-DIAG-1: Add debug logging to IncrUsage pipeline (print Exec result + error)
- [ ] TASK-UE-DIAG-2: Docker build --no-cache cont-admin-api with debug logging
- [ ] TASK-UE-DIAG-3: Restart container, trigger /internal/usage/incr, capture logs
- [ ] TASK-UE-DIAG-4: Analyze logs to identify root cause (nil err vs actual failure)
- [ ] TASK-UE-FIX-1: Apply root cause fix in storage/usage.go
- [ ] TASK-UE-FIX-2: Docker build --no-cache cont-admin-api
- [ ] TASK-UE-FIX-3: Restart container
- [ ] TASK-UE-FIX-4: Verify Redis DBSIZE > 0 after /internal/usage/incr call
- [ ] TASK-UE-FIX-5: Verify GET /internal/plan-quota returns non-zero current_usage

## 驗收標準

1. `POST /internal/usage/incr` 後 Redis 出現 `cont:usage:*` key，DBSIZE > 0
2. `GET /internal/plan-quota/:consumer_id` 返回 current_usage 非零
3. Free plan 超限 → 429 + X-RateLimit-Limit-Reached: true
4. 用量 80% → X-Usage-Warning header
5. Docker build --no-cache cont-admin-api 成功
6. Container restart 後功能正常
