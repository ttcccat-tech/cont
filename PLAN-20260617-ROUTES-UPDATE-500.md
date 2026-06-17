# PLAN-20260617-ROUTES-UPDATE-500

## Tasks

- [ ] TASK-RU-1: 根因追蹤 — 在 store.UpdateRoute 中加入 error log，找出導致 500 的錯誤（5 tool calls）
  - Dev Agent: 在 `store.UpdateRoute` 的 `return nil, err` 前加 `log.Printf("UpdateRoute error: %v", err)`
  - Dev Agent: rebuild + restart cont-admin-api
  - QA Agent: 觸發 `PUT /routes/{uuid}` 觀察 log 輸出
  - 預期完成定義：找到導致 500 的具體 error message

- [ ] TASK-RU-2: 修復 Routes Update 500（5~8 tool calls）
  - Dev Agent: 根據 TASK-RU-1 找到的根因修復（可能是 service_id handling、binding 問題等）
  - Dev Agent: 比對 CreateRoute vs UpdateRoute 差異並修復
  - Dev Agent: docker build --no-cache cont-admin-api
  - Dev Agent: docker stop/rm/run cont-admin-api
  - Dev Agent: git commit + push
  - QA Agent: 驗證 `PUT /routes/{uuid}` with payload → 200
  - 預期完成定義：`PUT /routes/{uuid}/description` 返回 200

- [ ] TASK-RU-3: 完整驗證 Routes Update（4 tool calls）
  - QA Agent: `PUT /routes/{uuid}` with `{"name": "newname"}` → 200
  - QA Agent: `PUT /routes/{uuid}` with `{"paths": ["/new-path"]}` → 200
  - QA Agent: `PUT /routes/{uuid}` with `{"service_id": "non-existent"}` → 400/404（非 500）
  - QA Agent: `GET /routes/{uuid}` 確認更新已寫入 → 200
  - 預期完成定義：Routes Update 所有測試通過
