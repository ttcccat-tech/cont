# Cont QA Event Log — 2026-06-17

## QA Summary
- **Date**: 2026-06-17
- **Admin API**: http://localhost:18081
- **Gateway**: http://localhost:18000
- **Tester**: Hermes Agent (cron)
- **QA Run #2**: 2026-06-17 12:05 PM

## 🔴 Bug 記錄（2026-06-17 QA Run #2）

### 🔴 BUG-Services-Update: Services Update 返回 500（P0）
- **API**: PUT /services/{id}
- **預期**: 200 或 400（UUID 格式錯誤）
- **實際**: 500
- **小黑確診**: API handler 未 validate UUID format，PostgreSQL 直接 fail → 500
- **根因**: c.Param("id") 直接傳入 SQL，non-UUID（如 "1"）導致 `ERROR: invalid input syntax for type uuid`
- **小黑驗證**（舊）: 用有效 UUID 測試 → ✅ 200（但 event.md 錯誤記載為「修好」）
- **小黑驗證**（新，2026-06-17 14:17）:
  - PUT /services/1 → **400** ✅（invalid UUID）
  - PUT /services/{valid-uuid} → **200** ✅
- **小黑判定**: 🟡 → ✅ FIXED（UUID validation 已實作）
- **修復 commit**: 8461cea2, 6f3251e8
- **修補方向**: UUID validation handler — API 現在對 invalid UUID return 400 而非 500

### 🔴 BUG-Routes-Update: Routes Update 返回 500（P0）
- **API**: PUT /routes/{id}
- **預期**: 200 或 400（UUID 格式錯誤）
- **實際**: 500
- **小黑確診**: 同 Services，handler 未 validate UUID format
- **小黑驗證**（2026-06-17 14:17）:
  - PUT /routes/1 → **400** ✅（invalid UUID）
  - PUT /routes/{valid-uuid} → **200** ✅
- **小黑判定**: 🟡 → ✅ FIXED（UUID validation 已實作）
- **修復 commit**: 8461cea2, 6f3251e8

### ✅ BUG-Proxy-Routing: Proxy Routing 已修復
- **根因**: CreateTarget 未驗證 target 不可為空
- **修復**: develop 分支已有 validation（commit ba35db16 + 512c804a），本輪重建 containers
- **小黑驗證**:
  - `POST /upstreams/{id}/targets` with `target:""` → **400** ✅
  - `PUT /upstreams/{id}/targets/{tid}` with `target:""` → **400** ✅
  - DB 無 empty target 殘留記錄 ✅
- **驗證時間**: 2026-06-17 13:38
- **小黑判定**: 🔴 → ✅ FIXED

## ✅ 通過項目（2026-06-17 QA Run #2）

| Phase | 功能 | 狀態 |
|-------|------|------|
| 1 | Auth 登入 | ✅ 通過 |
| 2 | Users CRUD | ✅ 全部通過 |
| 3 | Groups CRUD | ✅ 全部通過 |
| 4 | Consumers CRUD | ✅ 全部通過 |
| 5 | Upstreams CRUD | ✅ 全部通過 |
| 6 | Services CRUD | ⚠️ Create/List/Get/Delete OK，Update 500 ❌ |
| 7 | Routes CRUD | ⚠️ Create/List/Get/Delete OK，Update 500 ❌ |
| 8 | Plugins CRUD | ✅ 全部通過 |
| 9 | Proxy Routing | ❌ 503 on new route |
| 10 | JWT Auth | ✅ credential API 正常 |

## QA Run #3（2026-06-17 14:22 PM）- Hermes Cron

### 測試結果摘要
| Phase | 功能 | 狀態 |
|-------|------|------|
| 1 | Auth 登入 | ✅ 通過 |
| 2 | Users CRUD | ✅ 全部通過（List=23, Create/Get/Update/Delete=OK） |
| 3 | Groups CRUD | ✅ 全部通過（List=4, Create/Get/Update/Delete=OK） |
| 4 | Consumers CRUD | ✅ 全部通過（List=19, Create/Get/Delete=OK） |
| 5 | Upstreams CRUD | ✅ 全部通過（List=31, Create/Get/Update/Delete=OK） |
| 6 | Services CRUD | ✅ 全部通過（Update=200 ✅ 與上次 QA 不同！） |
| 7 | Routes CRUD | 🔴 Update=500 ❌ |
| 8 | Plugins CRUD | ✅ 全部通過（List=8, Create/Get/Update/Delete=OK） |
| 9 | Proxy Routing | 🔴 503 no upstream target ❌ |
| 10 | JWT Auth | ⚠️ GET /test-api/health=404（非預期 path，可能無對應 route） |

### 🔴 新增 Bug（本次 QA 發現）

#### 🔴 BUG-Routes-Update: Routes Update 返回 500（P0）
- **API**: PUT /routes/{id}
- **預期**: 200 或 204
- **實際**: 500 Internal Server Error
- **根因分析**: 使用有效 UUID（380453ea-06f0-4082-ab70-e09d8ebb0bbd）仍返回 500，非 UUID validation 問題
- **驗證**: `curl -X PUT /routes/{uuid}` → 500
- **嚴重程度**: P0（功能阻斷）

#### 🔴 BUG-Proxy-Routing-503: Proxy 轉發返回 503（上游 targets 未自動創建）（P0）
- **API**: GET /{route_path}/health via Gateway（http://localhost:18000）
- **預期**: 200 或 401
- **實際**: 503 {"message":"no upstream target","error":"Service Unavailable","statusCode":503}
- **根因**: `GET /upstreams/{id}/targets` 返回 `{"data":null,"next":""}` — upstream 創建後 targets 未自動初始化
- **小黑驗證**: upstream `192.168.1.202:3010` 存在，target 記錄為 null，Lua `next(nil)` 失敗
- **已知問題**: 見 SPEC-BUG-PROXY-SERVICE-NIL-UPSTREAM.md
- **嚴重程度**: P0（Proxy 轉發阻斷）

### 🟡 已知變更（與上次 QA 不同）
- **Services Update**: 上次 QA 標註 500，本次 QA 返回 200 ✅ 已修復（UUID validation 已生效）
- **注意**: event.md 舊記錄「Services Update 500」已過時，實際已修復

## 🔴 P0 Bug 彙整（需修復）- 更新於 2026-06-17 15:42
1. ~~Services Update 500~~ → ✅ 已修復（200）
2. ~~Routes Update 500~~ → ✅ 已修復（200，strip_path 欄位可正確更新）
3. ~~Proxy Routing 503 upstream targets nil~~ → ✅ 已修復（targets map 是 [] 而非 null）

## Development Guardian Review（2026-06-17 17:00 PM）

### 🔍 小黑確診：SPECs 落後於實際程式碼

|| SPEC | Task | 程式碼現況 |
|------|------|----------|
|| SPEC-ANALYTICS-01 | TASK-ANALYTICS-1/2 (parts[3]) | ✅ redis.go:304,362 已用 `parts[3]` |
|| SPEC-BUG-PROXY-GLOBAL-JWT | TASK-GLOBAL-JWT-1 (is_global) | ✅ nginx.conf:530 無 `is_global` |

### ✅ SPEC-BLACKSCREEN-01 — 已完成（程式碼審查 + Live 驗證）

**小黑驗證**（2026-06-17 17:00）：
| Task | 說明 | 狀態 |
|------|------|------|
| TASK-1 | Users.tsx `d.map is not a function` — 已 Array.isArray normalize | ✅ commit `e85ceb37` |
| TASK-2 | AlertRules.tsx blank — component fetch fix | ✅ commit `f55927a2` |
| TASK-3 | Billing.tsx route — `BillingPortal` component imported in App.tsx:28, route `/billing` → 200 | ✅ commit `8414f23e` |
| TASK-4 | /config-snapshots route — App.tsx:144 routes to `ConfigVersioning`, returns 200 | ✅ commit `872a0eb4` |
| TASK-5 | ApiDocs.tsx — fetches `/docs.json` as text (not JSON), SwaggerUI handles YAML natively | ✅ commit `68cd45e0` |
| TASK-6 | Sidebar "工作區" → `/workspaces` — App.tsx:54 + Sidebar.tsx:54 一致 | ✅ |

**小黑判定**: 🔴 → ✅ COMPLETED（所有 6 tasks 已實作，live 路由驗證通過）

### ✅ SPEC-2.5-A — 已完成（程式碼審查）

**小黑驗證**（2026-06-17 17:00）：

| Task | 說明 | 程式碼 |
|------|------|--------|
| TASK-2.5-A1 | `/usage/analytics` endpoint | `dcd143e9` — routes/usage.go handler + storage/usage.go analytics query |
| TASK-2.5-A2 | `getAnalyticsUsage()` in kong.ts | `8dd9d44a` — kong.ts:369 `export const getAnalyticsUsage` |
| TASK-2.5-A3 | Cont usage panel in Analytics.tsx | `monthly_total`, `quota_limit`, `top_routes`, `top_consumers` all rendered (Analytics.tsx:331-420) |

**小黑判定**: 🔴 → ✅ COMPLETED

### ✅ 驗證通過
- `docker compose build --no-cache cont-proxy` ✅
- `docker compose build --no-cache cont-admin-api` ✅
- `docker exec cont-proxy nginx -t` ✅ (worker_connections warn — 🟡)
- `v025` migration applied (2026-06-15) ✅
- All 5 containers healthy ✅
- `/internal/config/snapshot` → 200 ✅
- Frontend routes: `/billing` → 200, `/config-snapshots` → 200, `/api-docs` → 200, `/workspaces` → 200 ✅

### 📋 其它 SPEC 待處理（優先順序排序）
1. ~~SPEC-BLACKSCREEN-01~~ → ✅ COMPLETED（本輪小黑關帳）
2. ~~SPEC-2.5-A~~ → ✅ COMPLETED（本輪小黑關帳）
3. **SPEC-2.5-B** — AlertRules.tsx add usage_quota alert type（🟡 pending, code exists `8daf76f4`）
4. **SPEC-usage-alerting** — alerter.go + Frontend alert rules（🟡 pending）
5. **SPEC-plugin-access-param** — run_plugin_access() plugin param fix（🟡 pending）