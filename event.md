# Cont 開發待辦

## 🔴 未完成（進行中）

- [ ] **Services Create UI modal 不關閉** — Ant Design Form + Modal onOk 行為問題，onFinish callback 未被正確觸發
- [ ] **Plugins Create UI modal 不關閉** — 同上，PluginScope model 已重構但 UI submit 邏輯未動
- [ ] **Services Edit QA** — 尚未完整測試更新 service name/URL
- [ ] **Routes Delete UI** — 前端刪除按鈕流程未測試

## 🟡 預計優化

- [ ] Cont Auth 正式實作（目前為 mock）
- [ ] 生產部署評估（需等所有 Bug 修完 + Auth 完成）

---

## ✅ 已完成

（每次開發輪次完成後，隨即將已完成項目移至此處並標注 commit hash）

---

## 完整開發流程（開發守護遵循）

1. **開發執行** → 逐項處理，每修完一個問題即時 commit
2. **QA 測試** → curl / browser 驗證 Create/Read/Update/Delete 基本流程
3. **寫入 event.md** → 發現的 Bug 加入 🔴 未完成，完成的移至 ✅
4. **commit** → 每個變更单独 commit
5. **push** → 確認所有改動入庫

> 一輪：開發 → QA → Bug 寫入 event.md → commit → push = 完整回合