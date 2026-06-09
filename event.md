# Cont 開發待辦

## 🔴 未完成（進行中）

- [ ] **cont-admin-api container 未加入網路** — 重建後 networks={} 導致無法解析 postgres，需要每次 docker network connect。已寫入啟動腳本修復。
- [ ] **cont-admin-api 未持久化** — docker run 沒有 --restart=unless-stopped，重啟後會消失。

## 🟡 預計優化

- [ ] Cont Auth 正式實作（目前為 mock）
- [ ] 生產部署評估（需等所有 Bug 修完 + Auth 完成）

---

## ✅ 已完成

- [x] **Services/Plugins/Routes Update PATCH 404** — commit `0cd501a0`，routes.go 缺少 PATCH 路由，已新增全部 6 個實體的 PATCH
- [x] **Services Create/Edit modal 不關閉** — commit `04125706`，移除 Modal onOk 改用 Form submit
- [x] **Plugins Create/Edit modal 不關閉** — commit `886f75d5`，同上模式
- [x] **Routes Create/Edit modal 不關閉** — commit `5790860f`，同上模式
- [x] **Services Create/Edit/Delete API QA** — Create ✅ Delete ✅（Update PATCH 404 為已知 bug）
- [x] **Plugins Create/Delete API QA** — Create ✅ Delete ✅（Update PATCH 404 為已知 bug）
- [x] **Routes Create/Delete API QA** — Create ✅ Delete ✅（Update PATCH 404 為已知 bug）

---

## 完整開發流程（開發守護遵循）

1. **開發執行** → 逐項處理，每修完一個問題即時 commit
2. **QA 測試** → curl / browser 驗證 Create/Read/Update/Delete 基本流程
3. **寫入 event.md** → 發現的 Bug 加入 🔴 未完成，完成的移至 ✅
4. **commit** → 每個變更单独 commit
5. **push** → 確認所有改動入庫

> 一輪：開發 → QA → Bug 寫入 event.md → commit → push = 完整回合