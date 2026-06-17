# SPEC-BUG-Services-Update-500

## 背景
- **發現時間**: 2026-06-16 15:13 UTC
- **嚴重程度**: P0（功能阻斷 — Service 無法更新）
- **API**: PUT /services/{id}
- **預期**: 200 Update 成功
- **實際**: 500 {"code":"INTERNAL_ERROR","message":"internal server error"}

## 根因（初步分析）
Services Update 時攜帶 upstream_id，config_sync.lua 同步時未正確處理 upstream host 解析

## Scope
### In-scope
- 修復 PUT /services/{id} 返回 500 的問題
- 確認 upstream_id 在 update 時正確處理

### Out-of-scope
- 不重寫整個 config_sync.lua
- 不改動 Service Create 邏輯

## 驗收標準
- [ ] PUT /services/{id} 攜帶 upstream_id → 返回 200
- [ ] PUT /services/{id} 無 upstream_id → 返回 200（原值保留）
- [ ] GET /services/{id} → upstream_id 正確反映
