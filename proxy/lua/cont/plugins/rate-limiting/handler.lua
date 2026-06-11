-- plugins/rate-limiting/handler.lua
-- Rate limiting plugin using Redis sliding window

local redis = require("resty.redis")
local cjson = require("cjson")

local _M = { version = "0.1.0" }

-- Per-plugin counters in shared dict (fallback if Redis unavailable)
local shm = ngx.shared.cont_rate_limit

function _M.new(self)
    return setmetatable({}, { __index = _M })
end

-- Extract rate limit config from plugin config
local function get_config(plugin)
    local cfg = plugin.config or {}
    return {
        minute = cfg.minute or cfg.minute == 0 and 0 or nil,
        hour = cfg.hour or cfg.hour == 0 and 0 or nil,
        day = cfg.day or cfg.day == 0 and 0 or nil,
        second = cfg.second or cfg.second == 0 and 0 or nil,
        policy = cfg.policy or "local", -- "local" or "redis"
        redis_host = cfg.redis_host or "cont-redis",
        redis_port = cfg.redis_port or 6379,
        redis_password = cfg.redis_password or "",
        redis_database = cfg.redis_database or 0,
    }
end

-- Build Redis key for rate limit counter
local function redis_key(plugin, identifier, period)
    return string.format("ratelimit:%s:%s:%s:%s",
        plugin.id or "global",
        identifier,
        period,
        os.date("!%Y%m%d%H%M"))  -- UTC for sliding window
end

-- Check rate limit using Redis sliding window counter
local function check_redis(cfg, key, limit)
    local red = redis.new()
    red:set_timeout(500)

    local ok, err = red:connect(cfg.redis_host, cfg.redis_port)
    if not ok then
        ngx.log(ngx.ERR, "cont/rate-limiting: redis connect failed: ", err)
        return true  -- fail open if Redis is down
    end

    if cfg.redis_password and cfg.redis_password ~= "" then
        local ok, err = red:auth(cfg.redis_password)
        if not ok then
            ngx.log(ngx.WARN, "cont/rate-limiting: redis auth failed: ", err)
        end
    end

    red:select(cfg.redis_database or 0)

    -- Sliding window: increment counter and set TTL
    local count, err = red:incr(key)
    if not count then
        ngx.log(ngx.ERR, "cont/rate-limiting: redis incr failed: ", err)
        red:close()
        return true  -- fail open
    end

    if count == 1 then
        -- First request — set expiry (60 seconds for minute, 3600 for hour, etc.)
        red:expire(key, 60)
    end

    red:close()

    if limit and count > limit then
        return false, count, limit
    end
    return true, count, limit
end

-- Check rate limit using shared dict (local fallback)
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

-- Get client identifier (consumer or IP)
local function get_identifier()
    -- Use authenticated consumer if set by auth plugin
    local consumer_id = ngx.ctx.authenticated_consumer_id
    if consumer_id then
        return "consumer:" .. consumer_id
    end
    -- Fall back to IP
    return "ip:" .. (ngx.var.remote_addr or "0.0.0.0")
end

function _M.access(self, plugin)
    local cfg = get_config(plugin)
    local identifier = get_identifier()

    -- Build headers to return rate limit info
    local headers = {}

    -- Check each configured period
    local over_limit = false
    local retry_after = 0

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
            retry_after = 60 - (tonumber(os.date("!%S")) or 0)
        end
    end

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
            retry_after = math.max(retry_after, 86400 - (tonumber(os.date("!%H")) * 3600 + tonumber(os.date("!%M")) * 60 + (tonumber(os.date("!%S")) or 0)))
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