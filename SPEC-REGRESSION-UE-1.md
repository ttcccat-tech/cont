# SPEC-REGRESSION-UE-1: IncrUsage Redis Write Silent Failure

## 背景

小黑 2026-06-16 02:30 UTC 發現：`POST /internal/usage/incr` 返回 `{"count":1,"success":true}` 但 Redis DBSIZE 恆為 0。

**排除的根因**（小黑已排除）：
- ❌ 不是 binary 問題：binary 含 IncrUsage code
- ❌ 不是網路問題：Redis PING = +PONG
- ❌ 不是 Redis auth 問題：無密碼
- ❌ 不是時區問題：hour format 是 UTC
- ❌ 不是 key 衝突：194ee7b4 已分離 INCR(string) 和 HSET(hash) key
- ❌ 不是 Docker image 問題

**小黑 2026-06-16 03:00 UTC 根因確認**：
- ✅ Redis FLUSHALL 後，寫入完全正常
- ✅ 所有 key type 正確（string INCR, hash HSET）
- ✅ count 正確遞增（1, 2, 3...）
- ✅ 舊的 orphan `cont-admin-api-test` container 導致 Redis 狀態損壞
- **結論：非 code 問題，是 Redis 狀態汙染**

## 目標

確認 IncrUsage Redis 寫入功能正常穩定，確認 Free plan 超限阻擋恢復。

## Scope

### In-scope
- 驗證 `POST /internal/usage/incr` 寫入 Redis 成功
- 驗證 `GET /usage/org/:org_id` 回傳正確用量
- 驗證 Free plan 超限阻擋（rate-limiting-advanced）正常
- 驗證 80% warning header 正常

### Out-of-scope
- 不修改 code（code 已確認正常）
- 不修改 Redis 配置

## 驗收標準

1. `POST /internal/usage/incr` 成功寫入 Redis，回傳 `{success: true, count: N}`
2. Redis 出現 `cont:usage:{org_id}:{YYYYMMDDHH}` key，TTL > 0
3. `GET /usage/org/:org_id` 返回正確 JSON（含 total > 0）
4. Docker compose 全部 containers healthy
5. rate-limiting-advanced plugin 正確阻擋超限請求

## Tasks

- [ ] TASK-UE1-FIX: Redis FLUSHALL 清除汙染狀態
- [ ] TASK-UE1-V1: 驗證 IncrUsage 寫入 Redis 成功（count 遞增）
- [ ] TASK-UE1-V2: 驗證 /usage/org/:id 返回正確用量
- [ ] TASK-UE1-V3: 驗證 rate-limiting 超限阻擋
- [ ] TASK-UE1-V4: commit event.md 更新 regression 狀態
