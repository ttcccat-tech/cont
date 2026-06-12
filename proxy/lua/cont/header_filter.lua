-- cont.header_filter
-- Response header modification — runs plugin header_filter() chains

local function run_plugin_header_filter(plugin)
    local plugin_name = plugin.name
    local ok, mod = pcall(require, "plugins." .. plugin_name .. ".handler")
    if not ok or not mod then return end
    local handler = mod.new()
    if handler.header_filter then
        pcall(handler.header_filter, handler)
    end
end

local function header_filter()
    local cont = _G.cont or {}

    local route = ngx.ctx.matched_route
    local service = ngx.ctx.service
    local plugins = cont.plugins or {}

    for _, plugin in ipairs(plugins) do
        run_plugin_header_filter(plugin)
    end

    -- Kong-compatible headers
    ngx.header["Via"] = ngx.var.server_protocol .. " cont/0.1.0"
    ngx.header["X-Kong-Proxy-Latency"] = tostring(math.floor((tonumber(ngx.var.request_time) or 0) * 1000))
    ngx.header["X-Kong-Upstream-Latency"] = tostring(math.floor((tonumber(ngx.var.upstream_response_time) or 0) * 1000))

    -- CORS headers — always enabled
    local cors_origin = os.getenv("CONT_CORS_ORIGIN") or "*"
    ngx.header["Access-Control-Allow-Origin"] = cors_origin
    ngx.header["Access-Control-Allow-Methods"] = "GET, POST, PUT, PATCH, DELETE, OPTIONS"
    ngx.header["Access-Control-Allow-Headers"] = "Content-Type, Authorization, Kong-Admin-Token, X-API-Key, X-Requested-With"
    ngx.header["Access-Control-Allow-Credentials"] = "true"
    ngx.header["Access-Control-Max-Age"] = "86400"

    -- Consumer info headers (set by access.lua)
    if ngx.ctx.authenticated_consumer_id then
        ngx.header["X-Consumer-ID"] = ngx.ctx.authenticated_consumer_id
        ngx.header["X-Credential-Identifier"] = ngx.ctx.credential_identifier or ""
    end
end

return header_filter
