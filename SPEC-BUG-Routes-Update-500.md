# SPEC-BUG-Routes-Update-500

## 背景
- **發現時間**: 2026-06-16 15:13 UTC
- **嚴重程度**: P0（功能阻斷 — Route 無法更新）
- **API**: PUT /routes/{id}
- **預期**: 200 Update 成功
- **實際**: 500 {"code":"INTERNAL_ERROR","message":"internal server error"}

## 根因（初步分析）
Routes Update 呼叫 Store.UpdateRoute，store.go 中間層問題

## Scope
### In-scope
- 修復 PUT /routes/{id} 返回 500 的問題
- 比對 CreateRoute 與 UpdateRoute 實作差異

### Out-of-scope
- 不改動 Route Create 邏輯（已正常）
- 不改動其他 CRUD 端點

## 驗收標準
- [ ] PUT /routes/{id} 攜帶 service_id → 返回 200
- [ ] PUT /routes/{id} 無 service_id → 返回 200（原值保留）
- [ ] GET /routes/{id} → service_id 正確反映
