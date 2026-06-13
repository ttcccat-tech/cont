-- cont.metrics
-- Prometheus metrics endpoint (Kong /metrics compatible)
-- Exposes nginx-level metrics + custom application metrics

local metrics_shm = ngx.shared.cont_metrics

-- ── Helpers ───────────────────────────────────────────────────────────────────
local function get(key, default)
    return metrics_shm and metrics_shm:get(key) or default
end

-- Compute histogram buckets for latency
-- Buckets: 5ms, 10ms, 25ms, 50ms, 100ms, 250ms, 500ms, 1s, 2.5s, 5s, 10s
local LATENCY_BUCKETS = {5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000}

local function build_histogram_lines(name, help, sum_key, count_key)
    local lines = {
        "# HELP " .. name .. " " .. help,
        "# TYPE " .. name .. " histogram",
    }
    local sum = get(sum_key, 0)
    local count = get(count_key, 0)
    for _, le in ipairs(LATENCY_BUCKETS) do
        -- Approximate: assume linear distribution within bucket
        -- Only show cumulative counts when we have data
        local bucket_count = 0
        if count > 0 then
            local fraction = math.min(sum / (count * le) * count, count)
            bucket_count = math.floor(fraction)
        end
        table.insert(lines, name .. "_bucket{le=\"" .. le .. "\"} " .. bucket_count)
    end
    table.insert(lines, name .. "_bucket{le=\"+Inf\"} " .. (count > 0 and count or 0))
    table.insert(lines, name .. "_sum " .. sum)
    table.insert(lines, name .. "_count " .. (count > 0 and count or 0))
    return table.concat(lines, "\n")
end

-- ── Main Builder ─────────────────────────────────────────────────────────────
local function build_metrics()
    local lines = {}

    -- ── Nginx-level metrics (matching what log.lua writes) ──────────────────
    -- Total requests counter
    table.insert(lines, "# HELP cont_nginx_requests_total Total number of requests")
    table.insert(lines, "# TYPE cont_nginx_requests_total counter")
    table.insert(lines, "cont_nginx_requests_total " .. get("nginx_requests_total", 0))

    -- Status code buckets
    table.insert(lines, "")
    table.insert(lines, "# HELP cont_nginx_requests_total Total requests by status class")
    table.insert(lines, "# TYPE cont_nginx_requests_total counter")
    table.insert(lines, "cont_nginx_requests_total{code=\"2xx\"} " .. get("nginx_requests_total{code=\"2xx\"}", 0))
    table.insert(lines, "cont_nginx_requests_total{code=\"3xx\"} " .. get("nginx_requests_total{code=\"3xx\"}", 0))
    table.insert(lines, "cont_nginx_requests_total{code=\"4xx\"} " .. get("nginx_requests_total{code=\"4xx\"}", 0))
    table.insert(lines, "cont_nginx_requests_total{code=\"5xx\"} " .. get("nginx_requests_total{code=\"5xx\"}", 0))

    -- ── Request latency histogram ───────────────────────────────────────────
    table.insert(lines, "")
    local latency_lines = {}
    do
        local sum = get("nginx_request_latency_ms_sum", 0)
        local count = get("nginx_request_latency_ms_count", 0)
        latency_lines = {
            "# HELP cont_request_duration_seconds Request latency in seconds",
            "# TYPE cont_request_duration_seconds histogram",
        }
        for _, le in ipairs(LATENCY_BUCKETS) do
            local le_seconds = le / 1000
            local bucket_count = 0
            if count > 0 and sum > 0 then
                local avg_ms = sum / count
                local ratio = math.min(avg_ms / le, 1)
                bucket_count = math.floor(count * ratio)
            end
            table.insert(latency_lines, string.format("cont_request_duration_seconds_bucket{le=\"%.3f\"} %d", le_seconds, bucket_count))
        end
        table.insert(latency_lines, "cont_request_duration_seconds_bucket{le=\"+Inf\"} " .. (count > 0 and count or 0))
        table.insert(latency_lines, "cont_request_duration_seconds_sum " .. string.format("%.3f", sum / 1000))
        table.insert(latency_lines, "cont_request_duration_seconds_count " .. (count > 0 and count or 0))
    end
    for _, l in ipairs(latency_lines) do
        table.insert(lines, l)
    end

    -- ── Upstream latency histogram ──────────────────────────────────────────
    table.insert(lines, "")
    local up_lines = {}
    do
        local sum = get("nginx_upstream_latency_ms_sum", 0)
        local count = get("nginx_upstream_latency_ms_count", 0)
        up_lines = {
            "# HELP cont_upstream_latency_seconds Upstream response latency in seconds",
            "# TYPE cont_upstream_latency_seconds histogram",
        }
        for _, le in ipairs(LATENCY_BUCKETS) do
            local le_seconds = le / 1000
            local bucket_count = 0
            if count > 0 and sum > 0 then
                local avg_ms = sum / count
                local ratio = math.min(avg_ms / le, 1)
                bucket_count = math.floor(count * ratio)
            end
            table.insert(up_lines, string.format("cont_upstream_latency_seconds_bucket{le=\"%.3f\"} %d", le_seconds, bucket_count))
        end
        table.insert(up_lines, "cont_upstream_latency_seconds_bucket{le=\"+Inf\"} " .. (count > 0 and count or 0))
        table.insert(up_lines, "cont_upstream_latency_seconds_sum " .. string.format("%.3f", sum / 1000))
        table.insert(up_lines, "cont_upstream_latency_seconds_count " .. (count > 0 and count or 0))
    end
    for _, l in ipairs(up_lines) do
        table.insert(lines, l)
    end

    -- ── Bytes sent ──────────────────────────────────────────────────────────
    table.insert(lines, "")
    table.insert(lines, "# HELP cont_bytes_sent_total Total bytes sent to clients")
    table.insert(lines, "# TYPE cont_bytes_sent_total counter")
    table.insert(lines, "cont_bytes_sent_total " .. get("nginx_bytes_sent_sum", 0))

    -- ── Nginx connection gauges ──────────────────────────────────────────────
    table.insert(lines, "")
    table.insert(lines, "# HELP cont_nginx_connections Nginx connection states")
    table.insert(lines, "# TYPE cont_nginx_connections gauge")
    table.insert(lines, "cont_nginx_connections{state=\"active\"} " .. get("connections_active", 0))
    table.insert(lines, "cont_nginx_connections{state=\"accepted\"} " .. get("connections_accepted", 0))
    table.insert(lines, "cont_nginx_connections{state=\"reading\"} 0")
    table.insert(lines, "cont_nginx_connections{state=\"writing\"} 0")
    table.insert(lines, "cont_nginx_connections{state=\"waiting\"} 0")

    -- ── Route-level request counters ────────────────────────────────────────
    -- Collect all route keys
    if metrics_shm then
        local keys = metrics_shm:get_keys()
        for _, key in ipairs(keys or {}) do
            if key:find("route_request_total{") then
                table.insert(lines, "")
                table.insert(lines, "# HELP cont_route_requests_total Requests per route")
                table.insert(lines, "# TYPE cont_route_requests_total counter")
                table.insert(lines, key .. " " .. (metrics_shm:get(key) or 0))
            end
        end
    end

    -- ── Consumer-level request counters ────────────────────────────────────
    if metrics_shm then
        local keys = metrics_shm:get_keys()
        for _, key in ipairs(keys or {}) do
            if key:find("consumer_request_total{") then
                table.insert(lines, "")
                table.insert(lines, "# HELP cont_consumer_requests_total Requests per consumer")
                table.insert(lines, "# TYPE cont_consumer_requests_total counter")
                table.insert(lines, key .. " " .. (metrics_shm:get(key) or 0))
            end
        end
    end

    -- ── Upstream target health ──────────────────────────────────────────────
    table.insert(lines, "")
    table.insert(lines, "# HELP cont_upstream_target_up Upstream target health (1=up, 0=down)")
    table.insert(lines, "# TYPE cont_upstream_target_up gauge")
    -- TODO: read from Redis health data (populated by healthcheck.lua)
    -- For now, emit placeholder — alerter reads from /internal/circuit-breaker-configs
    -- which includes upstream target health from Redis

    return table.concat(lines, "\n")
end

local function metrics_handler()
    ngx.status = 200
    ngx.header["Content-Type"] = "text/plain; version=0.0.4"
    ngx.say(build_metrics())
    return ngx.OK
end

return metrics_handler
