-- cont.log
-- Post-request processing — plugin log() chains + metrics

local cont = require("cont.init")

-- Increment metrics counters (Prometheus-style via shared dict)
local metrics = ngx.shared.cont_metrics
if metrics then
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
end

-- Run plugin log() chains
local plugins = cont.plugins or {}
local function run_plugin_log(plugin)
    local plugin_name = plugin.name
    local ok, mod = pcall(require, "cont.plugins." .. plugin_name .. ".handler")
    if not ok or not mod then return end
    local handler = mod.new()
    if handler.log then
        pcall(handler.log, handler)
    end
end

for _, plugin in ipairs(plugins) do
    run_plugin_log(plugin)
end
