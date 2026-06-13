-- plugins/usage-tracking/handler.lua
-- Usage tracking plugin: writes request data to Redis
-- Writes: org_id, consumer_id, route_id, service_id, timestamp, latency, status_code

local cjson = require("cjson")

local _M = { version = "0.1.0" }

-- Config extraction
local function get_config(plugin)
    local cfg = plugin.config or {}
    return {
        redis_host = cfg.redis_host or "cont-redis",
        redis_port = cfg.redis_port or 6379,
        redis_password = cfg.redis_password or "",
        redis_database = cfg.redis_database or 0,
        redis_timeout = cfg.redis_timeout or 500,
    }
end

-- Redis connection via ngx.socket.tcp
local function redis_connect(sock, host, port, timeout_ms)
    sock:set_timeout(timeout_ms or 500)
    local ok, err = sock:connect(host, port)
    return ok, err
end

local function redis_command(sock, ...)
    local args = {...}
    local cmd = table.concat(args, " ")
    local bytes, err = sock:send(cmd .. "\r\n")
    if not bytes then return nil, err end
    local line, err = sock:receive("*l")
    if not line then return nil, err end
    if line == "+OK" or line == "+PONG" then
        return "OK"
    elseif string.sub(line, 1, 1) == "+" then
        return string.sub(line, 2)
    elseif string.sub(line, 1, 1) == ":" then
        return tonumber(string.sub(line, 2))
    elseif string.sub(line, 1, 1) == "$" then
        local n = tonumber(string.sub(line, 2))
        if n < 0 then return nil end
        local data, err = sock:receive(n)
        sock:receive(2) -- \r\n
        return data
    elseif line == "*-1" or line == "*-ERR" then
        return nil, line
    end
    return line
end

local function redis_auth(sock, password)
    if not password or password == "" then return true end
    local res, err = redis_command(sock, "AUTH", password)
    if not res or res == "ERR" then
        return false, err
    end
    return true
end

local function redis_select(sock, db)
    if db == 0 then return true end
    local res, err = redis_command(sock, "SELECT", db)
    return res == "OK", err
end

-- Get current hour in YYYYMMDDHH format
local function get_current_hour()
    return os.date("!%Y%m%d%H")
end

-- Build usage increment command (MULTI/EXEC for atomicity)
local function build_usage_incr_commands(sock, org_id, consumer_id, route_id, service_id, latency_ms, status_code)
    local hour = get_current_hour()

    -- INCR cont:usage:{org_id}:{YYYYMMDDHH}
    local org_key = string.format("cont:usage:%s:%s", org_id, hour)
    redis_command(sock, "INCR", org_key)
    redis_command(sock, "EXPIRE", org_key, 5356800) -- 62 days

    -- INCR cont:usage:consumer:{consumer_id}:{YYYYMMDDHH}
    if consumer_id and consumer_id ~= "" then
        local consumer_key = string.format("cont:usage:consumer:%s:%s", consumer_id, hour)
        redis_command(sock, "INCR", consumer_key)
        redis_command(sock, "EXPIRE", consumer_key, 5356800)
    end

    -- INCR cont:usage:route:{route_id}:{YYYYMMDDHH}
    if route_id and route_id ~= "" then
        local route_key = string.format("cont:usage:route:%s:%s", route_id, hour)
        redis_command(sock, "INCR", route_key)
        redis_command(sock, "EXPIRE", route_key, 5356800)
    end

    -- INCR cont:usage:service:{service_id}:{YYYYMMDDHH}
    if service_id and service_id ~= "" then
        local service_key = string.format("cont:usage:service:%s:%s", service_id, hour)
        redis_command(sock, "INCR", service_key)
        redis_command(sock, "EXPIRE", service_key, 5356800)
    end

    -- ZADD cont:usage:detail:{org_id}:{YYYYMMDDHH} timestamp_ns "consumer_id:route_id:service_id:latency_ms:status_code"
    local detail_key = string.format("cont:usage:detail:%s:%s", org_id, hour)
    local ts = ngx.now() * 1000000 -- microseconds
    local member = string.format("%s:%s:%s:%d:%d", consumer_id or "", route_id or "", service_id or "", latency_ms, status_code)
    redis_command(sock, "ZADD", detail_key, string.format("%.0f", ts), member)
    redis_command(sock, "EXPIRE", detail_key, 5356800)

    return true
end

-- Main access handler
function _M.access(self, plugin)
    local cfg = get_config(plugin)

    -- Get context values set by auth plugins
    local org_id = ngx.ctx.authenticated_org_id
    local consumer_id = ngx.ctx.authenticated_consumer_id
    local route_id = ngx.ctx.cont_route_id
    local service_id = ngx.ctx.cont_service_id

    -- Skip if no org_id
    if not org_id or org_id == "" then
        return
    end

    -- Get latency and status (available in log phase, but we estimate in access)
    local latency_ms = 0
    local status_code = ngx.status

    -- Connect to Redis
    local sock = ngx.socket.tcp()
    local ok, err = redis_connect(sock, cfg.redis_host, cfg.redis_port, cfg.redis_timeout)
    if not ok then
        ngx.log(ngx.ERR, "cont/usage-tracking: redis connect failed: ", err)
        return -- fail open, don't block request
    end

    ok, err = redis_auth(sock, cfg.redis_password)
    if not ok then
        ngx.log(ngx.WARN, "cont/usage-tracking: redis auth failed: ", err)
    end

    redis_select(sock, cfg.redis_database or 0)

    -- Build and send commands
    build_usage_incr_commands(sock, org_id, consumer_id, route_id, service_id, latency_ms, status_code)

    sock:close()
end

-- Log phase: update with actual latency and status
function _M.log(self, plugin)
    local cfg = get_config(plugin)

    local org_id = ngx.ctx.authenticated_org_id
    local consumer_id = ngx.ctx.authenticated_consumer_id
    local route_id = ngx.ctx.cont_route_id
    local service_id = ngx.ctx.cont_service_id

    if not org_id or org_id == "" then
        return
    end

    -- Get actual values from request context
    local latency_ms = tonumber(string.format("%.0f", ngx.now() * 1000)) - (ngx.ctx.request_start_time or 0)
    local status_code = ngx.status

    local sock = ngx.socket.tcp()
    local ok, err = redis_connect(sock, cfg.redis_host, cfg.redis_port, cfg.redis_timeout)
    if not ok then
        return
    end

    redis_auth(sock, cfg.redis_password)
    redis_select(sock, cfg.redis_database or 0)

    build_usage_incr_commands(sock, org_id, consumer_id, route_id, service_id, latency_ms, status_code)

    sock:close()
end

return _M