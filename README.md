# cont — API Gateway Management Platform

Cloud-native API Gateway with Kong-compatible Admin API, built on OpenResty (NGINX + lua-nginx-module) and Go.

## 系統需求

| 項目 | 最低需求 |
|------|---------|
| CPU | 2 cores |
| 記憶體 | 4 GB |
| 磁碟 | 20 GB |
| Docker | 20.10+ |
| Docker Compose | 2.0+ |
| 網路 | 需開通以下連接埠 |

## 連接埠對照表

| 連接埠 | 服務 | 說明 |
|--------|------|------|
| 18000 | Proxy | API 代理入口（對外） |
| 18081 | Admin API | 管理 API（僅供前端使用） |
| 18082 | Frontend | Web 管理介面 |
| 5432 | PostgreSQL | 資料庫（僅本機開啟） |
| 6379 | Redis | 快取/限速（僅本機開啟） |

## 安裝流程

### 1. 克隆專案

```bash
git clone https://github.com/ttcccat-tech/cont.git
cd cont
```

### 2. 建立網路

```bash
docker network create cont_default
```

### 3. 啟動系統（單行指令）

```bash
make up
```

或直接使用 docker compose：

```bash
docker compose up -d
```

### 4. 確認服務狀態

```bash
# 檢查所有容器是否正常運行
docker compose ps

# 預期輸出：
# NAME                STATUS
# cont-postgres       Healthy
# cont-redis          Healthy
# cont-admin-api      Healthy
# cont-proxy          Running
# cont-frontend       Running
```

### 5. 開啟管理介面

```
http://localhost:18082
```

預設帳號：`admin`
預設密碼：`admin123`

> **重要：** 首次登入後請立即變更密碼。

## 常用指令

```bash
# 啟動
make up

# 停止
make down

# 重啟
make restart

# 查看即時日誌
make logs

# 查看 Prometheus 指標
make metrics

# 重建前端（含快取清除）
docker compose build --pull --no-cache cont-frontend
docker compose up -d cont-frontend
```

## 開發者指令

```bash
# 重建 Admin API
make admin-build

# 本機執行 Admin API（需已安裝 Go 1.22+）
make admin-run

# 資料庫遷移
make db-migrate

# 測試路由比對
make test-route
```

## 架構圖

```
Browser/Client
      │
      ▼
┌─────────────────────────────────────┐
│   OpenResty Proxy (port 18000)       │
│   Lua Plugin Chain                   │
│   Route → Service → Upstream → Target│
└─────────────────────────────────────┘
      │                    │
      ▼                    ▼
┌─────────────┐     ┌──────────┐
│ Admin API   │     │ Upstream │
│ Go+Gin 18081│     │ Targets  │
└─────────────┘     └──────────┘
      │                    │
      ▼                    ▼
┌─────────────┐     ┌──────────┐
│ PostgreSQL  │     │  Redis   │
│ (config)    │     │(rate limit│
└─────────────┘     │ /cache)  │
                    └──────────┘
```

## 技術堆疊

| 元件 | 技術 |
|------|------|
| 代理引擎 | OpenResty (NGINX + lua-nginx-module) |
| 管理 API | Go 1.22 + Gin |
| 資料庫 | PostgreSQL 16 |
| 快取/限速 | Redis 7 |
| 前端管理介面 | React 18 + Ant Design 5 |
| 指標收集 | Prometheus (Kong-compatible) |

## API 端點（Kong Admin API 相容）

```
GET|POST   /services
GET|PUT|DELETE /services/:id
GET|POST   /routes
GET|PUT|DELETE /routes/:id
GET|POST   /upstreams
GET|PUT|DELETE /upstreams/:id
GET|POST   /upstreams/:id/targets
GET|PUT|DELETE /upstreams/:id/targets/:target_id
GET|POST   /consumers
GET|PUT|DELETE /consumers/:id
GET|POST   /plugins
GET|PUT|DELETE /plugins/:id
GET        /status
GET        /metrics
```

## 預設帳號

| 帳號 | 密碼 | 角色 |
|------|------|------|
| admin | admin123 | admin（完整權限） |
| user | user123 | editor（有限權限） |

> **注意：** admin 密碼以 bcrypt hash 儲存於資料庫。上線後請立即變更預設密碼。

## 環境變數（選填）

| 變數 | 說明 | 預設值 |
|------|------|--------|
| `JWT_SECRET` | JWT 簽章金鑰 | `cont-dev-jwt-secret-change-in-prod` |
| `CONT_URL` | 外部訪問網址 | （空） |

## 疑難排解

### 容器無法啟動

```bash
# 查看詳細日誌
docker compose logs cont-admin-api
docker compose logs cont-proxy

# 檢查網路
docker network inspect cont_default
```

### 前端顯示 API 載入失敗

確認 `cont-admin-api` 已就緒（Healthy）後再存取前端。

### PostgreSQL 連線失敗

等待 `cont-postgres` 變成 Healthy（約 10-15 秒）再啟動其他服務。

## License

BSD 2-Clause / NGINX Compatible
