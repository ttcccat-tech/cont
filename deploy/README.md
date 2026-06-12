# Cont Gateway — 生產部署指南

## 系統需求

- Docker Engine 24.0+
- Docker Compose v2.20+
- 2 CPU cores, 2GB RAM minimum
- 10GB free disk space

## 快速啟動

```bash
# 1. 複製專案
git clone https://github.com/ttcccat-tech/cont.git
cd cont

# 2. 切換至 deploy 分支（此分支只放部署成品）
git checkout deploy

# 3. 複製環境變數設定檔
cp .env.example .env
# 編輯 .env，填入 JWT_SECRET 等必要參數

# 4. 建立外部網路（首次需手動建立）
docker network create cont_net

# 5. 建置並啟動所有服務
docker compose up -d

# 6. 驗證服務狀態
docker compose ps
curl http://localhost:18081/health-check
curl http://localhost:18000/status
```

## 服務埠口

| 服務 | 內部埠 | 預設外部埠 | 說明 |
|------|--------|------------|------|
| Frontend | 80 | 18082 | Cont 管理介面 |
| Proxy | 8000 | 18000 | API Gateway 代理 |
| Admin API | 8001 | 18081 | 管理 REST API |
| PostgreSQL | 5432 | 5432 | 資料庫 |
| Redis | 6379 | 6379 | 快取/工作階段 |

## 初始設定

### 建立管理員帳號

```bash
# 透過 Admin API 建立管理員（首次啟動後執行一次）
curl -X POST http://localhost:18081/users \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "YOUR_SECURE_PASSWORD",
    "role": "admin"
  }'
```

### 登入管理介面

開啟瀏覽器訪問：`http://<YOUR_SERVER>:18082`

預設管理員帳號密碼（如果已存在）：
- Username: `admin`
- Password: `admin123` （**請立即修改**）

### 建立外部 Nginx 反向代理（可選）

如果要透過網域对外提供服务，在前端 Nginx/Cloudflare 等处添加反向代理：

```nginx
# /etc/nginx/conf.d/cont.conf

# Admin API
location /api/ {
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header Host $http_host;
    proxy_pass http://127.0.0.1:18081/;
    proxy_redirect off;
}

# Cont Proxy (Gateway)
location /cont-admin/ {
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header Host $http_host;
    proxy_pass http://127.0.0.1:18000/;
    proxy_redirect off;
}

# Frontend
location / {
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header Host $http_host;
    proxy_pass http://127.0.0.1:18082/;
    proxy_redirect off;
}
```

## Docker Compose 命令

```bash
# 啟動（背景執行）
docker compose up -d

# 查看日誌
docker compose logs -f

# 停止服務
docker compose down

# 停止並清除資料（慎用）
docker compose down -v

# 重建特定服務
docker compose up -d --build proxy

# 重啟特定服務
docker compose restart admin-api
```

## 環境變數參考

| 變數 | 預設值 | 說明 |
|------|--------|------|
| `JWT_SECRET` | （必填） | JWT 簽章密鑰，建議 32+ 字元隨機字串 |
| `CONT_URL` | | 部署 URL，用於 OAuth 等回呼 |
| `CONT_CORS_ORIGIN` | `*` | CORS 允許的 Origin，生產環境建議設為網域 |
| `CONT_LOG_FORMAT` | `json` | 日誌格式：`json` 或 `text` |
| `CONT_LOG_LEVEL` | `info` | 日誌層級：`debug`, `info`, `warn`, `error` |
| `CONT_JSON_PRETTY` | `false` | 是否美化 JSON 輸出 |
| `POSTGRES_PASSWORD` | `kongpass` | PostgreSQL 密碼 |
| `REDIS_PASSWORD` | | Redis 密碼（建議設定） |
| `SLACK_WEBHOOK_URL` | | Slack Webhook URL（API Key 審批通知） |
| `EMAIL_WEBHOOK_URL` | | Email Webhook URL（API Key 審批通知） |

## HTTPS / SSL

建議透過 Cloudflare Tunnel 或 Nginx Proxy Manager 等方式處理 HTTPS，Cont 本身不內建 SSL 終止。

## 資料備份

```bash
# 備份 PostgreSQL
docker compose exec postgres pg_dump -U kong cont > backup_$(date +%Y%m%d).sql

# 恢復
cat backup_20240101.sql | docker compose exec -T postgres psql -U kong cont
```

## 更新部署

```bash
# 拉取最新程式碼
git fetch origin deploy
git checkout deploy
git pull origin deploy

# 重新建置並重啟
docker compose up -d --build
```

## 監控

```bash
# 查看服務健康狀態
curl http://localhost:18081/health-check
curl http://localhost:18000/status

# 查看 Proxy 指標
curl http://localhost:18000/metrics
```
