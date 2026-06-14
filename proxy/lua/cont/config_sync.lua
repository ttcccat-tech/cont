-- cont.config_sync
-- Fetches config snapshot from Admin API using cosocket (safe at init_by_lua context)
-- Called by init_by_lua_block and periodically by ngx.timer.every

local ADMIN_API_HOST = os.getenv("CONT_ADMIN_API_HOST") or "cont-admin-api"
local ADMIN_API_PORT = os.getenv("CONT_ADMIN_API_PORT") or "8001"
local SNAPSHOT_PATH = "/internal/config/snapshot"

local _M = {}

function _M.fetch_config()
    local sock = ngx.socket.tcp()
    sock:settimeout(5000)
    local ok, err = sock:connect(ADMIN_API_HOST, tonumber(ADMIN_API_PORT))
    if not ok then
        ngx.log(ngx.ERR, "config_sync: connect failed: ", err)
        return nil
    end

    local req = "GET " .. SNAPSHOT_PATH .. " HTTP/1.0\r\nHost: " .. ADMIN_API_HOST .. "\r\nConnection: close\r\n\r\n"
    local bytes, err = sock:send(req)
    if not bytes then
        ngx.log(ngx.ERR, "config_sync: send failed: ", err)
        sock:close()
        return nil
    end

    local reader = sock:receiveuntil("\r\n\r\n")
    local headers, err = reader()
    if not headers then
        ngx.log(ngx.ERR, "config_sync: receive headers failed: ", err)
        sock:close()
        return nil
    end

    local content_length = nil
    for key, val in headers:gmatch("([%w%-]+):%s*([^\r\n]+)") do
        if key:lower() == "content-length" then
            content_length = tonumber(val)
        end
    end

    local body = ""
    if content_length then
        local remaining, err = sock:receive(content_length)
        if remaining then
            body = remaining
        end
    else
        body, err = sock:receive("*a")
        if not body then
            ngx.log(ngx.WARN, "config_sync: receive body failed: ", err)
        end
    end

    sock:close()

    local parsed, data = pcall(cjson.decode, body)
    if not parsed or not data then
        ngx.log(ngx.ERR, "config_sync: JSON decode failed: ", data)
        return nil
    end

    return data
end

function _M.sync_into_cont(cont)
    local data = _M.fetch_config()
    if not data then
        ngx.log(ngx.WARN, "config_sync: fetch_config returned nil")
        return false
    end
    if not data.routes then
        ngx.log(ngx.WARN, "config_sync: data.routes is nil, body may be empty")
        return false
    end
    cont.routes = data.routes or {}
    cont.services = data.services or {}
    cont.upstreams = data.upstreams or {}
    cont.plugins = data.plugins or {}
    cont.targets = data.targets or {}
    cont.config_loaded = true
    ngx.log(ngx.WARN, "config_sync: synced ", #cont.routes, " routes, ", #cont.services, " services, config_loaded=true")
    return true
end

return _M
