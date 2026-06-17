-- cont.log
-- Non-blocking access logging pipeline:
--   1. record_metrics()  — shared dict counters (unchanged, O(1))
--   2. write_access_log() — push JSON to Shared DICT (O(1), non-blocking)
--   3. background timer  — every 5s pops all items and writes to /var/log/cont/access.json.log
--   4. run_plugin_logs()  — plugin log() chains (unchanged)

local cjson = require("cjson")
local cjson_encode = cjson.encode

-- ── Metrics Recording ────────────────────────────────────────────────────────
local function record_metrics()
    local metrics = ngx.shared.cont_metrics
    if not metrics then return end

    metrics:incr("nginx_requests_total", 1, 0)

    local status = ngx.status
    if status >= 200 and status < 300 then
        metrics:incr("nginx_requests_total{code=\"2xx\"}", 1, 0)
    elseif status >= 300 and status < 400 then
        metrics:incr("nginx_requests_total{code=\"3xx\"}", 1, 0)
    elseif status >= 400 and status < 500 then
        metrics:incr("nginx_requests_total{code=\"4xx\"}", 1, 0)
    elseif status >= 500 then
        metrics:incr("nginx_requests_total{code=\"5xx\"}", 1, 0)
    end

    local latency = tonumber(ngx.var.request_time or 0)
    if latency and latency > 0 then
        local ms = math.floor(latency * 1000)
        metrics:incr("nginx_request_latency_ms_sum", ms, 0)
        metrics:incr("nginx_request_latency_ms_count", 1, 0)
    end

    local upstream_latency = tonumber(ngx.var.upstream_response_time or 0)
    if upstream_latency and upstream_latency > 0 then
        local ms = math.floor(upstream_latency * 1000)
        metrics:incr("nginx_upstream_latency_ms_sum", ms, 0)
        metrics:incr("nginx_upstream_latency_ms_count", 1, 0)
    end

    local bytes_sent = tonumber(ngx.var.bytes_sent or 0)
    if bytes_sent and bytes_sent > 0 then
        metrics:incr("nginx_bytes_sent_sum", bytes_sent, 0)
    end

    local route_id = ngx.ctx.route_id
    if route_id then
        metrics:incr("route_request_total{route_id=\"" .. route_id .. "\"}", 1, 0)
        if status >= 400 then
            metrics:incr("route_request_total{route_id=\"" .. route_id .. "\",code=\"" .. status .. "\"}", 1, 0)
        end
    end

    local consumer_id = ngx.ctx.authenticated_consumer_id
    if consumer_id then
        metrics:incr("consumer_request_total{consumer_id=\"" .. consumer_id .. "\"}", 1, 0)
    end
end

-- ── Access Log Entry Builder ─────────────────────────────────────────────────
local function build_log_entry()
    local upstream_target = ngx.ctx.upstream_target or ""

    return {
        timestamp           = math.floor(ngx.now() * 1000),  -- ms epoch
        request = {
            method  = ngx.req.get_method(),
            uri     = ngx.var.uri,
            query   = ngx.var.query_string or "",
            size    = tonumber(ngx.var.request_length) or 0,
        },
        response = {
            status            = ngx.status,
            size              = tonumber(ngx.var.bytes_sent) or 0,
            latency_ms        = math.floor((tonumber(ngx.var.request_time) or 0) * 1000),
            upstream_latency_ms = math.floor((tonumber(ngx.var.upstream_response_time) or 0) * 1000),
        },
        client = {
            ip   = ngx.var.remote_addr or "0.0.0.0",
            port = tonumber(ngx.var.remote_port) or 0,
        },
        upstream        = upstream_target,
        route_id        = ngx.ctx.route_id or "",
        service_id      = ngx.ctx.service_id or "",
        consumer_id     = ngx.ctx.authenticated_consumer_id or "",
        user_agent      = ngx.var.http_user_agent or "",
        referer         = ngx.var.http_referer or "",
        trace_id        = ngx.var.http_x_cont_trace_id or ngx.header["X-Cont-Trace-ID"] or "",
    }
end

-- ── Non-blocking Shared DICT push ──────────────────────────────────────────
local QUEUE_KEY    = "log_queue"
local QUEUE_MAX    = 10000   -- 最多緩衝 10000 筆，防止爆記憶體

local function write_access_log()
    local logs_dict = ngx.shared.cont_access_logs
    if not logs_dict then
        ngx.log(ngx.ERR, "cont/log: cont_access_logs shared dict not found")
        return
    end

    local entry = build_log_entry()
    local ok, err = logs_dict:lpush(QUEUE_KEY, cjson_encode(entry))
    if not ok then
        ngx.log(ngx.ERR, "cont/log: lpush failed: ", err)
        return
    end

    -- 防止佇列無限成長（每次 lpush 後順便檢查）
    local len = logs_dict:llen(QUEUE_KEY)
    if len > QUEUE_MAX then
        -- 刪除最舊的（rpop = queue 尾巴），只留 QUEUE_MAX 筆
        for i = 1, (len - QUEUE_MAX) do
            logs_dict:rpop(QUEUE_KEY)
        end
    end
end

-- ── Background Flush Timer ───────────────────────────────────────────────────
-- 確保只有一個 worker 在寫檔（避免同時開檔衝突）
-- 使用粗粒度 lock：ngx.writer 檔鎖，若失敗就跳過這次寫入
local LOG_FILE = "/var/log/cont/access.json.log"
local LOCK_KEY = "log_flush_lock"
local FLUSH_INTERVAL = 5  -- 秒

local function flush_access_logs(premature)
    if premature then return end

    local logs_dict = ngx.shared.cont_access_logs
    if not logs_dict then return end

    local lock_held = logs_dict:add(LOCK_KEY, 1, 1)  -- 1s NX TTL
    if not lock_held then
        -- 另一個 worker 正在寫，跳過
        return
    end

    local ok, err = pcall(function()
        local batch = {}
        while true do
            local item = logs_dict:rpop(QUEUE_KEY)  -- FIFO: rpop from tail
            if not item then break end
            table.insert(batch, item)
            if #batch >= 1000 then
                -- 一次最多取 1000 筆，避免 single timer 太久
                break
            end
        end

        if #batch == 0 then
            logs_dict:delete(LOCK_KEY)
            return
        end

        -- 一次性寫入，減少磁碟 I/O 次數
        local file, f_err = io.open(LOG_FILE, "a")
        if not file then
            ngx.log(ngx.ERR, "cont/log: file open failed: ", f_err)
            logs_dict:delete(LOCK_KEY)
            return
        end

        file:write(table.concat(batch, "\n") .. "\n")
        file:close()
    end)

    logs_dict:delete(LOCK_KEY)

    if not ok then
        ngx.log(ngx.ERR, "cont/log: flush error: ", err)
    end
end

-- 在首個 worker 啟動時註冊計時器（每 FLUSH_INTERVAL 秒一次）
local function start_flush_timer()
    local ok, err = ngx.timer.every(FLUSH_INTERVAL, flush_access_logs)
    if not ok then
        ngx.log(ngx.ERR, "cont/log: failed to start flush timer: ", err)
    else
        ngx.log(ngx.INFO, "cont/log: access log flush timer started (interval=", FLUSH_INTERVAL, "s)")
    end
end

-- ── Plugin log() Chains ──────────────────────────────────────────────────────
local function run_plugin_logs()
    local cont = _G.cont
    if not cont or not cont.plugins then return end

    for _, plugin in ipairs(cont.plugins) do
        local ok, mod = pcall(require, "plugins." .. plugin.name .. ".handler")
        if ok and mod then
            local handler = mod.new()
            if handler.log then
                pcall(handler.log, handler)
            end
        end
    end

    local cb_upstream = ngx.ctx.cb_upstream
    if cb_upstream then
        local ok, cb = pcall(require, "cont.plugins.circuit-breaker.handler")
        if ok and cb and cb.log then
            pcall(cb.log, cb, {})
        end
    end
end

-- ── Init: register background flush timer (once per worker) ─────────────────
start_flush_timer()

-- ── Main Log Phase ───────────────────────────────────────────────────────────
record_metrics()
write_access_log()
run_plugin_logs()
