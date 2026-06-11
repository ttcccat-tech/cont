-- cont.log
-- Post-request processing — plugin log() chains + metrics

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

-- Run plugin log() chains
local function run_plugin_logs()
    -- Use global 'cont' from init_by_lua
    if not cont or not cont.plugins then
        return
    end

    local function run_plugin_log(plugin)
        local plugin_name = plugin.name
        local ok, mod = pcall(require, "cont.plugins." .. plugin_name .. ".handler")
        if not ok or not mod then return end
        local handler = mod.new()
        if handler.log then
            pcall(handler.log, handler)
        end
    end

    for _, plugin in ipairs(cont.plugins) do
        run_plugin_log(plugin)
    end
end

-- Main log phase
record_metrics()
run_plugin_logs()