# PLAN: BUG-Services-Update-500 + BUG-Routes-Update-500

## Tasks

### TASK-UUID-SVC: Add UUID validation to UpdateService handler
- **檔案**: admin-api/routes/routes.go（UpdateService 函數）
- **改動**: 在 ShouldBindJSON 前，先 validate `c.Param("id")` 是 valid UUID v4
- **完成定義**: PUT /services/1 → 400, PUT /services/{valid-uuid} → 200
- **對應 SPEC**: 驗收標準 1

### TASK-UUID-RTE: Add UUID validation to UpdateRoute handler
- **檔案**: admin-api/routes/routes.go（UpdateRoute 函數）
- **改動**: 在 ShouldBindJSON 前，先 validate `c.Param("id")` 是 valid UUID v4
- **完成定義**: PUT /routes/1 → 400, PUT /routes/{valid-uuid} → 200
- **對應 SPEC**: 驗收標準 1

### TASK-UUID-BUILD: Docker build + restart
- **觸發**: 每個 task 完成後執行
- **完成定義**: `docker compose build --pull --no-cache cont-admin-api` 成功，container 重啟 healthy

### TASK-UUID-TEST: QA 驗證
- **對應**: QA Agent 執行 curl 測試
- **完成定義**: 
  - PUT /services/1 → 400 ✅
  - PUT /routes/1 → 400 ✅
  - PUT /services/{uuid} → 200 ✅
  - PUT /routes/{uuid} → 200 ✅
