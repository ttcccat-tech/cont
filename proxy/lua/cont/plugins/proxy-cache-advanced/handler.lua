-- plugins/proxy-cache-advanced/handler.lua
-- Proxy cache plugin: access phase cache lookup, body_filter storage, header_filter finalization
-- Supports Redis (primary) + shared dict (local fallback) with TTL and content-type filtering

local cjson = require("cjson")

local _M = { version = "0.1.0" }

-- Shared dict for local cache fallback
local shm = ngx.shared.cont_proxy_cache

-- ── Config extraction ────────────────────────────────────────────────────────
local function get_config(plugin)
    local cfg = plugin.config or {}
    return {
        response_code = cfg.response_code or {200},
        request_method = cfg.request_method or {"GET", "HEAD"},
        content_type = cfg.content_type or {"text/*", "application/*"},
        ttl = cfg.ttl or 300,
        strategy = cfg.strategy or "local",
        redis_host = cfg.redis_host or "cont-redis",
        redis_port = cfg.redis_port or 6379,
        redis_password = cfg.redis_password or "",
        redis_database = cfg.redis_database or 1,
        cache_on_error = cfg.cache_on_error ~= false,
        vary_headers = cfg.vary_headers or {},
    }
end

-- ── Cache key builder ──────────────────────────────────────────────────────────
local function build_cache_key(plugin, request, vary_headers)
    local key = string.format("cache:%s:%s:%s",
        plugin.id or "global",
        request.method,
        request.uri)
    if request.args and request.args ~= "" then
        key = key .. "?" .. request.args
    end
    if vary_headers and #vary_headers > 0 then
        local parts = {}
        for _, hdr in ipairs(vary_headers) do
            local val = request.headers[hdr]
            if val then
                table.insert(parts, hdr .. "=" .. val)
            end
        end
        if #parts > 0 then
            key = key .. "|" .. table.concat(parts, ",")
        end
    end
    return key
end

-- ── Content type matching ─────────────────────────────────────────────────────
local function content_type_matches(ctype, filters)
    if not ctype or not filters or #filters == 0 then
        return true
    end
    for _, filter in ipairs(filters) do
        if filter == "*/*" then return true end
        if string.find(filter, "%*", 1, true) then
            local pattern = "^" .. string.gsub(filter, "*", "[^/]+") .. "$"
            if string.match(ctype, pattern) then
                return true
            end
        elseif ctype == filter then
            return true
        end
    end
    return false
end

-- ── Method cacheable check ─────────────────────────────────────────────────────
local function method_cacheable(method, allowed)
    for _, m in ipairs(allowed) do
        if m == method then return true end
    end
    return false
end

-- ── Redis operations ───────────────────────────────────────────────────────────
local function redis_do(cfg, op, key, value, ttl)
    local redis = require("resty.redis")
    local red = redis.new()
    red:set_timeout(500)
    local ok, err = red:connect(cfg.redis_host, cfg.redis_port)
    if not ok then
        ngx.log(ngx.ERR, "cont/proxy-cache: redis connect failed: ", err)
        return nil
    end
    if cfg.redis_password and cfg.redis_password ~= "" then
        red:auth(cfg.redis_password)
    end
    red:select(cfg.redis_database or 1)
    local result
    if op == "get" then
        result = red:get(key)
    elseif op == "setex" then
        result = red:setex(key, ttl, value)
    end
    red:close()
    return result
end

-- ── Local cache operations ─────────────────────────────────────────────────────
local function local_do(op, key, value, ttl)
    local shm = ngx.shared.cont_proxy_cache
    if not shm then return nil end
    if op == "get" then
        return shm:get(key)
    elseif op == "set" then
        local ok, err = shm:set(key, value, ttl)
        return ok and "OK" or nil
    end
end

-- ── Store cached response ──────────────────────────────────────────────────────
local function store_response(cfg, key, data)
    local encoded = cjson.encode(data)
    local ttl = cfg.ttl or 300
    if cfg.strategy == "redis" then
        redis_do(cfg, "setex", key, encoded, ttl)
    else
        local_do("set", key, encoded, ttl)
    end
end

-- ── Retrieve cached response ───────────────────────────────────────────────────
local function fetch_response(cfg, key)
    local data
    if cfg.strategy == "redis" then
        data = redis_do(cfg, "get", key)
    else
        data = local_do("get", key)
    end
    if data and data ~= ngx.null then
        local ok, decoded = pcall(cjson.decode, data)
        if ok then return decoded end
    end
    return nil
end

-- ── Check if status is cacheable ──────────────────────────────────────────────
local function status_cacheable(status, allowed_codes, cache_on_error)
    for _, s in ipairs(allowed_codes) do
        if s == status then return true end
    end
    return cache_on_error and status >= 400
end

-- ── Access phase: cache lookup ────────────────────────────────────────────────
function _M.access(self, plugin)
    local cfg = get_config(plugin)
    local method = ngx.req.get_method()
    local uri = ngx.var.uri
    local args = ngx.var.query_string or ""
    local headers = ngx.req.get_headers()

    -- Only cache safe methods
    if not method_cacheable(method, cfg.request_method) then
        return
    end

    -- Check content type filter
    local ctype = headers["Content-Type"] or ""
    if not content_type_matches(ctype, cfg.content_type) then
        return
    end

    local request = { method = method, uri = uri, args = args, headers = headers }
    local key = build_cache_key(plugin, request, cfg.vary_headers)

    -- Store key in ngx.var so nginx.conf can read it for conditional proxy_pass
    ngx.var.cont_cache_key = key

    local cached = fetch_response(cfg, key)
    if cached then
        ngx.var.cont_cache_hit = "1"
        ngx.header["X-Cache-Status"] = "HIT"
        ngx.header["X-Cache-Key"] = key

        -- Set upstream to non-routable address to prevent actual proxy
        ngx.var.upstream = "127.0.0.1:65535"

        -- Reconstruct response
        if cached.status then
            ngx.status = cached.status
        end
        if cached.headers then
            for name, val in pairs(cached.headers) do
                if name ~= "Content-Length" and name ~= "Transfer-Encoding" then
                    ngx.header[name] = val
                end
            end
        end

        if method == "HEAD" then
            ngx.header["Content-Length"] = cached.body and #cached.body or 0
            ngx.exit(ngx.OK)
            return
        end

        if cached.body then
            ngx.print(cached.body)
        end
        ngx.exit(ngx.OK)
        return
    end

    -- Cache miss — mark and prepare for storage
    ngx.var.cont_cache_hit = "0"
    ngx.header["X-Cache-Status"] = "MISS"
    ngx.header["X-Cache-Key"] = key

    -- Store metadata in ngx.ctx for body_filter
    ngx.ctx.cache_key = key
    ngx.ctx.cache_cfg = cfg
    ngx.ctx.cache_body_chunks = {}
end

-- ── Header filter phase: set response headers ─────────────────────────────────
function _M.header_filter(self, plugin)
    -- Set X-Cache-Status from context (already set in access, but preserve)
    local cache_hit = ngx.var.cont_cache_hit
    if cache_hit == "1" then
        ngx.header["X-Cache-Status"] = "HIT"
    elseif cache_hit == "0" then
        ngx.header["X-Cache-Status"] = "MISS"
    end
end

-- ── Body filter phase: accumulate and store response ──────────────────────────
function _M.body_filter(self, plugin)
    local key = ngx.ctx.cache_key
    local cfg = ngx.ctx.cache_cfg
    if not key or not cfg then
        return
    end

    -- Only store on cache miss (cont_cache_hit == "0")
    if ngx.var.cont_cache_hit ~= "0" then
        return
    end

    local chunk = ngx.arg[1]
    local eof = ngx.arg[2]

    if chunk and #chunk > 0 then
        table.insert(ngx.ctx.cache_body_chunks, chunk)
    end

    if eof then
        local body = table.concat(ngx.ctx.cache_body_chunks)
        local status = ngx.status
        local ctype = ngx.header["Content-Type"] or ""

        -- Check if this status is cacheable
        if not status_cacheable(status, cfg.response_code, cfg.cache_on_error) then
            ngx.ctx.cache_key = nil
            ngx.ctx.cache_cfg = nil
            ngx.ctx.cache_body_chunks = nil
            return
        end

        -- Check content type
        if not content_type_matches(ctype, cfg.content_type) then
            ngx.ctx.cache_key = nil
            ngx.ctx.cache_cfg = nil
            ngx.ctx.cache_body_chunks = nil
            return
        end

        -- Collect response headers
        local headers = {}
        for name, _ in pairs(ngx.resp.get_headers()) do
            -- skip large headers
        end
        -- Use a simple approach: store only essential headers
        local resp_headers = {}
        local h = ngx.resp.get_headers()
        if h then
            for name, val in pairs(h) do
                if name ~= "Content-Length" and name ~= "Transfer-Encoding"
                   and name ~= "Connection" and name ~= "Keep-Alive" then
                    resp_headers[name] = val
                end
            end
        end

        store_response(cfg, key, {
            status = status,
            body = body,
            content_type = ctype,
            headers = resp_headers,
        })

        -- Clean up
        ngx.ctx.cache_key = nil
        ngx.ctx.cache_cfg = nil
        ngx.ctx.cache_body_chunks = nil
    end
end

return _M
