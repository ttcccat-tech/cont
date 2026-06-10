# Cont 開發待辦

## 🔴 未完成（進行中）

（暂无）

## 🟡 預計優化

- [x] **Cont Auth 正式實作（JWT / bcrypt）** — commit `cfd7f15d`，users table、AuthRequired middleware、JWT 登入廢除 mock
- [ ] 生產部署評估（需等所有 Bug 修完 + Auth 完成）

---

## ✅ 已完成

- [x] **Routes Create 不支援 service.name 格式** — commit `0a6e2060`，models.go ServiceRef + GetServiceName()，store.go GetServiceByName()，routes.go CreateRoute 解析 service.name → service.id（QA: Create ✅ Read ✅ PATCH ✅ Delete ✅）
- [x] **cont-admin-api container 未加入網路** — commit `295379b7`，docker-compose.yml 新增 networks: cont_default，確保網路正確連接
- [x] **cont-admin-api 未持久化** — commit `295379b7`，新增 restart: unless-stopped
- [x] **Services/Plugins/Routes Update PATCH 404** — commit `0cd501a0`，routes.go 缺少 PATCH 路由，已新增全部 6 個實體的 PATCH
- [x] **Services Create/Edit modal 不關閉** — commit `04125706`，移除 Modal onOk 改用 Form submit
- [x] **Plugins Create/Edit modal 不關閉** — commit `886f75d5`，同上模式
- [x] **Routes Create/Edit modal 不關閉** — commit `5790860f`，同上模式
- [x] **Services Create/Edit/Delete API QA** — Create ✅ Read ✅ Delete ✅（Update PATCH ✅）
- [x] **Plugins Create/Delete API QA** — Create ✅ Delete ✅（Update PATCH ✅）
- [x] **Routes Create/Delete API QA** — Create ✅ Read ✅ Delete ✅（Update PATCH ✅）

---

## 完整開發流程（開發守護遵循）

1. **開發執行** → 逐項處理，每修完一個問題即時 commit
2. **QA 測試** → curl / browser 驗證 Create/Read/Update/Delete 基本流程
3. **寫入 event.md** → 發現的 Bug 加入 🔴 未完成，完成的移至 ✅
4. **commit** → 每個變更单独 commit
5. **push** → 確認所有改動入庫

> 一輪：開發 → QA → Bug 寫入 event.md → commit → push = 完整回合