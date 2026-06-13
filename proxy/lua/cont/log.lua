-- cont.log
-- Post-request processing — plugin log() chains + structured access logging + metrics

-- ── Metrics Recording ────────────────────────────────────────────────────────
local function record_metrics()
    local metrics = ngx.shared.cont_metrics
    if not metrics then
        return
    end

    -- Basic request counter
    local ok = metrics:incr("nginx_requests_total", 1, 0)
    if not ok then
        ngx.log(ngx.WARN, "cont/log: failed to incr nginx_requests_total")
    end

    -- Status code buckets
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

    -- Request latency (in ms)
    local latency = tonumber(ngx.var.request_time or 0)
    if latency and latency > 0 then
        local ms = math.floor(latency * 1000)
        metrics:incr("nginx_request_latency_ms_sum", ms, 0)
        metrics:incr("nginx_request_latency_ms_count", 1, 0)
    end

    -- Upstream latency
    local upstream_latency = tonumber(ngx.var.upstream_response_time or 0)
    if upstream_latency and upstream_latency > 0 then
        local ms = math.floor(upstream_latency * 1000)
        metrics:incr("nginx_upstream_latency_ms_sum", ms, 0)
        metrics:incr("nginx_upstream_latency_ms_count", 1, 0)
    end

    -- Bytes sent
    local bytes_sent = tonumber(ngx.var.bytes_sent or 0)
    if bytes_sent and bytes_sent > 0 then
        metrics:incr("nginx_bytes_sent_sum", bytes_sent, 0)
    end

    -- Route-level metrics
    local route_id = ngx.ctx.route_id
    if route_id then
        metrics:incr("route_request_total{route_id=\"" .. route_id .. "\"}", 1, 0)
        if status >= 400 then
            metrics:incr("route_request_total{route_id=\"" .. route_id .. "\",code=\"" .. status .. "\"}", 1, 0)
        end
    end

    -- Consumer metrics (if authenticated)
    local consumer_id = ngx.ctx.authenticated_consumer_id
    if consumer_id then
        metrics:incr("consumer_request_total{consumer_id=\"" .. consumer_id .. "\"}", 1, 0)
    end
end

-- ── Structured Access Log ────────────────────────────────────────────────────
local function write_access_log()
    local log_format = os.getenv("CONT_LOG_FORMAT") or "json"
    local log_level = os.getenv("CONT_LOG_LEVEL") or "info"

    -- Determine log level based on status
    local status = ngx.status
    local level = "info"
    if status >= 500 then
        level = "error"
    elseif status >= 400 then
        level = "warn"
    end

    -- Skip if log level is higher than configured
    local level_priority = { debug = 0, info = 1, warn = 2, error = 3 }
    if (level_priority[level] or 0) < (level_priority[log_level] or 1) then
        return
    end

    local route = ngx.ctx.matched_route
    local service = ngx.ctx.service
    local upstream_target = ngx.ctx.upstream_target

    local entry = {
        timestamp = ngx.now() * 1000,  -- ms epoch
        request = {
            method = ngx.req.get_method(),
            uri = ngx.var.uri,
            query = ngx.var.query_string or "",
            size = tonumber(ngx.var.request_length) or 0,
        },
        response = {
            status = status,
            size = tonumber(ngx.var.bytes_sent) or 0,
            latency_ms = math.floor((tonumber(ngx.var.request_time) or 0) * 1000),
            upstream_latency_ms = math.floor((tonumber(ngx.var.upstream_response_time) or 0) * 1000),
        },
        client = {
            ip = ngx.var.remote_addr or "0.0.0.0",
            port = tonumber(ngx.var.remote_port) or 0,
        },
        upstream = upstream_target or "",
        route_id = ngx.ctx.route_id or "",
        service_id = ngx.ctx.service_id or "",
        consumer_id = ngx.ctx.authenticated_consumer_id or "",
        user_agent = ngx.var.http_user_agent or "",
        referer = ngx.var.http_referer or "",
    }

    local cjson = require("cjson")
    local log_line
    if log_format == "text" then
        log_line = string.format('[%s] %s %s %d %dms consumer=%s route=%s',
            os.date("!%Y-%m-%dT%H:%M:%SZ"),
            entry.request.method,
            entry.request.uri,
            status,
            entry.response.latency_ms,
            entry.consumer_id ~= "" and entry.consumer_id or "-",
            entry.route_id ~= "" and entry.route_id or "-"
        )
    else
        log_line = cjson.encode(entry)
    end

    if level == "error" then
        ngx.log(ngx.ERR, "cont.access: ", log_line)
    elseif level == "warn" then
        ngx.log(ngx.WARN, "cont.access: ", log_line)
    else
        ngx.log(ngx.INFO, "cont.access: ", log_line)
    end
end

-- ── Run Plugin log() Chains ─────────────────────────────────────────────────
local function run_plugin_logs()
    local cont = _G.cont
    if not cont or not cont.plugins then
        return
    end

    local function run_plugin_log(plugin)
        local plugin_name = plugin.name
        local ok, mod = pcall(require, "plugins." .. plugin_name .. ".handler")
        if not ok or not mod then return end
        local handler = mod.new()
        if handler.log then
            pcall(handler.log, handler)
        end
    end

    for _, plugin in ipairs(cont.plugins) do
        run_plugin_log(plugin)
    end

    -- Circuit breaker: run with upstream context from ngx.ctx
    local cb_upstream = ngx.ctx.cb_upstream
    if cb_upstream then
        local ok, cb = pcall(require, "cont.plugins.circuit-breaker.handler")
        if ok and cb and cb.log then
            pcall(cb.log, cb, {})
        end
    end
end

-- ── Main Log Phase ───────────────────────────────────────────────────────────
record_metrics()
write_access_log()
run_plugin_logs()
