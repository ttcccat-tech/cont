-- cont.metrics
-- Prometheus metrics endpoint (Kong /metrics compatible)

local metrics_shm = ngx.shared.cont_metrics

local function build_metrics()
    local lines = {
        "# HELP cont_nginx_requests_total Total number of requests",
        "# TYPE cont_nginx_requests_total counter",
        "cont_nginx_requests_total " .. (metrics_shm and metrics_shm:get("total_requests") or 0),
        "",
        "# HELP cont_nginx_connections_total Number of connections",
        "# TYPE cont_nginx_connections_total gauge",
        "cont_nginx_connections_total{state=\"active\"} " .. (metrics_shm and metrics_shm:get("connections_active") or 0),
        "cont_nginx_connections_total{state=\"accepted\"} " .. (metrics_shm and metrics_shm:get("connections_accepted") or 0),
        "cont_nginx_connections_total{state=\"reading\"} 0",
        "cont_nginx_connections_total{state=\"writing\"} 0",
        "cont_nginx_connections_total{state=\"waiting\"} 0",
        "",
        "# HELP cont_upstream_target_up Upstream target health (1=up, 0=down)",
        "# TYPE cont_upstream_target_up gauge",
    }

    -- Add upstream health from Redis
    -- TODO: read from Redis

    return table.concat(lines, "\n")
end

local function metrics_handler()
    ngx.status = 200
    ngx.say(build_metrics())
    return ngx.OK
end

return metrics_handler
