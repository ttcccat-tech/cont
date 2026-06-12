-- plugins/rate-limiting-advanced/handler.lua
-- Rate limiting plugin using Redis sliding window or local shared dict fallback
-- OpenResty Alpine compatible: uses ngx.socket.tcp instead of resty.redis

local cjson = require("cjson")

local _M = { version = "0.2.0" }

-- Shared dict for local fallback
local shm = ngx.shared.cont_rate_limit

-- ── Config extraction ────────────────────────────────────────────────────────
local function get_config(plugin)
    local cfg = plugin.config or {}
    return {
        minute = cfg.minute or 0,
        hour = cfg.hour or 0,
        day = cfg.day or 0,
        second = cfg.second or 0,
        policy = cfg.policy or "local",
        redis_host = cfg.redis_host or "cont-redis",
        redis_port = cfg.redis_port or 6379,
        redis_password = cfg.redis_password or "",
        redis_database = cfg.redis_database or 0,
        redis_timeout = cfg.redis_timeout or 500,
        burst = cfg.burst or 0,
    }
end

-- ── Redis via ngx.socket.tcp (OpenResty Alpine compatible) ────────────────────
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
    -- Simple Redis protocol parsing
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

-- ── Redis sliding window check ────────────────────────────────────────────────
local function check_redis(cfg, key, limit)
    local sock = ngx.socket.tcp()
    local ok, err = redis_connect(sock, cfg.redis_host, cfg.redis_port, cfg.redis_timeout)
    if not ok then
        ngx.log(ngx.ERR, "cont/rate-limiting: redis connect failed: ", err)
        return true  -- fail open
    end

    ok, err = redis_auth(sock, cfg.redis_password)
    if not ok then
        ngx.log(ngx.WARN, "cont/rate-limiting: redis auth failed: ", err)
    end

    redis_select(sock, cfg.redis_database or 0)

    -- INCR key
    local count, err = redis_command(sock, "INCR", key)
    if err then
        sock:close()
        ngx.log(ngx.ERR, "cont/rate-limiting: redis incr failed: ", err)
        return true  -- fail open
    end

    -- Set TTL only on first request (count == 1)
    if count == 1 then
        redis_command(sock, "EXPIRE", key, 60)
    end

    sock:close()

    if limit and count > limit then
        return false, count, limit
    end
    return true, count, limit
end

-- ── Local shared dict check ───────────────────────────────────────────────────
local function check_local(cfg, key, limit)
    local shm = ngx.shared.cont_rate_limit
    if not shm then
        return true  -- fail open
    end

    local count, err = shm:incr(key, 1, 0, 60)
    if err then
        ngx.log(ngx.ERR, "cont/rate-limiting: shm incr failed: ", err)
        return true  -- fail open
    end

    if limit and count > limit then
        return false, count, limit
    end
    return true, count, limit
end

-- ── Client identifier ─────────────────────────────────────────────────────────
local function get_identifier()
    local consumer_id = ngx.ctx.authenticated_consumer_id
    if consumer_id then
        return "consumer:" .. consumer_id
    end
    return "ip:" .. (ngx.var.remote_addr or "0.0.0.0")
end

-- ── Redis key builder ─────────────────────────────────────────────────────────
local function redis_key(plugin, identifier, period)
    -- Sliding window using current UTC minute
    return string.format("ratelimit:%s:%s:%s:%s",
        plugin.id or "global",
        identifier,
        period,
        os.date("!%Y%m%d%H%M"))
end

-- ── Main access handler ───────────────────────────────────────────────────────
function _M.access(self, plugin)
    local cfg = get_config(plugin)
    local identifier = get_identifier()

    local headers = {}
    local over_limit = false
    local retry_after = 0

    -- Check second limit
    if cfg.second and cfg.second > 0 then
        local key = redis_key(plugin, identifier, "s")
        local allowed, count, limit
        if cfg.policy == "redis" then
            allowed, count, limit = check_redis(cfg, key, cfg.second)
        else
            allowed, count, limit = check_local(cfg, key, cfg.second)
        end
        headers["X-RateLimit-Limit-Second"] = tostring(limit)
        headers["X-RateLimit-Remaining-Second"] = tostring(math.max(0, limit - count))
        if not allowed then
            over_limit = true
            retry_after = 1
        end
    end

    -- Check minute limit
    if cfg.minute and cfg.minute > 0 then
        local key = redis_key(plugin, identifier, "m")
        local allowed, count, limit
        if cfg.policy == "redis" then
            allowed, count, limit = check_redis(cfg, key, cfg.minute)
        else
            allowed, count, limit = check_local(cfg, key, cfg.minute)
        end
        headers["X-RateLimit-Limit-Minute"] = tostring(limit)
        headers["X-RateLimit-Remaining-Minute"] = tostring(math.max(0, limit - count))
        if not allowed then
            over_limit = true
            retry_after = math.max(retry_after, 60 - (tonumber(os.date("!%S")) or 0))
        end
    end

    -- Check hour limit
    if cfg.hour and cfg.hour > 0 then
        local key = redis_key(plugin, identifier, "h")
        local allowed, count, limit
        if cfg.policy == "redis" then
            allowed, count, limit = check_redis(cfg, key, cfg.hour)
        else
            allowed, count, limit = check_local(cfg, key, cfg.hour)
        end
        headers["X-RateLimit-Limit-Hour"] = tostring(limit)
        headers["X-RateLimit-Remaining-Hour"] = tostring(math.max(0, limit - count))
        if not allowed then
            over_limit = true
            retry_after = math.max(retry_after, 3600 - (tonumber(os.date("!%M")) * 60 + (tonumber(os.date("!%S")) or 0)))
        end
    end

    -- Check day limit
    if cfg.day and cfg.day > 0 then
        local key = redis_key(plugin, identifier, "d")
        local allowed, count, limit
        if cfg.policy == "redis" then
            allowed, count, limit = check_redis(cfg, key, cfg.day)
        else
            allowed, count, limit = check_local(cfg, key, cfg.day)
        end
        headers["X-RateLimit-Limit-Day"] = tostring(limit)
        headers["X-RateLimit-Remaining-Day"] = tostring(math.max(0, limit - count))
        if not allowed then
            over_limit = true
            retry_after = math.max(retry_after,
                86400 - (tonumber(os.date("!%H")) * 3600
                    + tonumber(os.date("!%M")) * 60
                    + (tonumber(os.date("!%S")) or 0)))
        end
    end

    -- Set rate limit headers
    for name, value in pairs(headers) do
        ngx.header[name] = value
    end

    if over_limit then
        ngx.header["Retry-After"] = tostring(math.ceil(retry_after))
        ngx.header["Content-Type"] = "application/json"
        ngx.status = 429
        ngx.say(cjson.encode({
            message = "Rate limit exceeded",
            error = "Too Many Requests",
            statusCode = 429,
        }))
        return ngx.exit(429)
    end
end

return _M
