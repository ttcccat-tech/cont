-- cont.header_filter
-- Response header modification — runs plugin header_filter() chains

local function run_plugin_header_filter(plugin)
    local plugin_name = plugin.name
    local ok, mod = pcall(require, "cont.plugins." .. plugin_name .. ".handler")
    if not ok or not mod then return end
    local handler = mod.new()
    if handler.header_filter then
        pcall(handler.header_filter, handler)
    end
end

local function header_filter()
    local cont = require("cont.init")

    local route = ngx.ctx.matched_route
    local service = ngx.ctx.service
    local plugins = cont.plugins or {}

    for _, plugin in ipairs(plugins) do
        run_plugin_header_filter(plugin)
    end

    -- Kong-compatible headers
    ngx.header["Via"] = ngx.var.server_protocol .. " cont/0.1.0"
    ngx.header["X-Kong-Proxy-Latency"] = ngx.var.request_time_ms or "0"
    ngx.header["X-Kong-Upstream-Latency"] = "0"
end

return header_filter
