-- cont.body_filter
-- Response body transformation — JSON pretty-print, error wrapping, gzip handling

local cont = require("init")

local plugins = cont.plugins or {}

local function run_plugin_body_filter(plugin, eof)
    local plugin_name = plugin.name
    local ok, mod = pcall(require, "plugins." .. plugin_name .. ".handler")
    if not ok or not mod then return end
    local handler = mod.new()
    if handler.body_filter then
        pcall(handler.body_filter, handler, eof)
    end
end

-- Pretty-print JSON response body (if enabled)
local function json_pretty_print(chunk)
    if not chunk or chunk == "" then
        return chunk
    end

    local pretty = os.getenv("CONT_JSON_PRETTY") or "false"
    if pretty ~= "true" then
        return chunk
    end

    -- Only pretty-print JSON responses
    local content_type = ngx.header["Content-Type"] or ""
    if not string.find(content_type, "application/json") then
        return chunk
    end

    local cjson = require("cjson")
    local ok, decoded = pcall(cjson.decode, chunk)
    if not ok then
        return chunk  -- not valid JSON, pass through
    end

    local ok2, encoded = pcall(cjson.encode, decoded)
    if not ok2 then
        return chunk
    end

    return encoded
end

-- Wrap error responses in Kong-compatible format
local function wrap_error_response(chunk, eof)
    if not chunk or chunk == "" or eof then
        return chunk
    end

    -- Only wrap if enabled and status is error
    local wrap_errors = os.getenv("CONT_WRAP_ERRORS") or "false"
    if wrap_errors ~= "true" then
        return chunk
    end

    local status = ngx.status
    if status < 400 then
        return chunk
    end

    -- Only wrap if not already wrapped
    local ok, decoded = pcall(require("cjson").decode, chunk)
    if ok and decoded and decoded.error then
        return chunk  -- already wrapped
    end

    -- Try to parse as plain text error and wrap
    local msg = string.match(chunk, '"message"%s*:%s*"([^"]+)"')
    if not msg then
        msg = chunk
    end

    local wrapped = string.format(
        '{"message":"%s","error":"%s","statusCode":%d}',
        msg, ngx.var.http_x_error or "Error", status
    )
    return wrapped
end

-- Run plugin body filters
for _, plugin in ipairs(plugins) do
    run_plugin_body_filter(plugin, ngx.arg[2])
end

-- Apply body transformations
local chunk = ngx.arg[1]
local eof = ngx.arg[2]

if not eof and chunk and chunk ~= "" then
    chunk = json_pretty_print(chunk)
    chunk = wrap_error_response(chunk, eof)
    ngx.arg[1] = chunk
end
