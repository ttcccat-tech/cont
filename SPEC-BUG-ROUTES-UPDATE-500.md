# SPEC-BUG-ROUTES-UPDATE-500

## 背景
- **發現時間**: 2026-06-16 18:51 UTC（第五輪 QA）
- **小黑驗證**: 2026-06-17 04:00 UTC —小黑親自 curl 確認 `PUT /routes/{id}` 返回 500

## 目標
修復 `PUT /routes/{id}` 返回 500 INTERNAL_ERROR 的 bug

## Scope

### In-Scope
- `store.UpdateRoute` 的 SQL UPDATE 邏輯
- 比對 `CreateRoute` 找出導致 500 的差異
- 修復後驗證 Routes Update 返回 200

### Out-of-Scope
- 不改动 CreateRoute 逻辑
- 不改动 proxy 转发逻辑（已有其他 SPEC 處理）

## 驗收標準

1. [ ] `PUT /routes/{id}` 攜帶 `{"description": "updated"}` → 200
2. [ ] `PUT /routes/{id}` 攜帶 `{"service_id": "uuid"}` → 200
3. [ ] `PUT /routes/{id}` 攜帶 `{"paths": ["/new-path"]}` → 200
4. [ ] `GET /routes/{id}` 確認更新已寫入 → 200 + 更新後欄位
5. [ ] Docker build --no-cache cont-admin-api 成功
6. [ ] 容器 healthy
