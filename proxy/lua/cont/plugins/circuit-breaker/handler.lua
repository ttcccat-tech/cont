-- plugins/circuit-breaker/handler.lua
-- Circuit Breaker plugin: upstream health-driven circuit breaker
-- States: CLOSED (normal) -> OPEN (tripped) -> HALF_OPEN (probe) -> CLOSED or OPEN
-- Tracks per-upstream failure counts in Redis; reads CB config from shared memory
-- Runs in access phase (post target-selection, pre-proxy) via pre_proxy_callback

local cjson = require("cjson")

local _M = { version = "0.1.0" }

-- Shared dict for CB config (written by backend sync, read by Lua)
local shm_config = ngx.shared.cont_circuit_breaker_config
-- Shared dict for CB state (local counter, fast read)
local shm_state = ngx.shared.cont_circuit_breaker_state

-- ── Config extraction from shared memory ─────────────────────────────────────
local function get_upstream_config(upstream_id)
    if not shm_config then return nil end
    local raw = shm_config:get("cb:" .. upstream_id)
    if not raw or raw == "" then return nil end
    local ok, cfg = pcall(cjson.decode, raw)
    if not ok then return nil end
    return cfg
end

-- ── State constants ────────────────────────────────────────────────────────────
local STATE_CLOSED = 0
local STATE_OPEN   = 1
local STATE_HALF_OPEN = 2

local STATE_NAMES = { [STATE_CLOSED] = "CLOSED", [STATE_OPEN] = "OPEN", [STATE_HALF_OPEN] = "HALF_OPEN" }

-- ── Redis connection (ngx.socket.tcp, OpenResty Alpine compatible) ────────────
local function redis_connect(sock, host, port, timeout_ms)
    sock:set_timeout(timeout_ms or 500)
    return sock:connect(host, port)
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
        sock:receive(2)
        return data
    elseif line == "*-1" or line == "*-ERR" then
        return nil, line
    end
    return line
end

local function redis_auth(sock, password)
    if not password or password == "" then return true end
    local res = redis_command(sock, "AUTH", password)
    return res == "OK"
end

local function redis_select(sock, db)
    if db == 0 then return true end
    return redis_command(sock, "SELECT", db) == "OK"
end

-- ── Redis key helpers ──────────────────────────────────────────────────────────
local function redis_key_upstream(upstream_id)
    return "cont:cb:upstream:" .. upstream_id
end

-- ── Get CB state from Redis ───────────────────────────────────────────────────
-- Returns: state (0/1/2), failure_count, last_failure_time, half_open_successes
local function get_circuit_state(upstream_id)
    local r = _G.cont
    local redis_host = r and r.redis_host or "cont-redis"
    local redis_port = r and r.redis_port or 6379
    local redis_password = r and r.redis_password or ""
    local redis_db = 0
    local redis_timeout = 500

    local sock = ngx.socket.tcp()
    local ok, err = redis_connect(sock, redis_host, redis_port, redis_timeout)
    if not ok then
        ngx.log(ngx.ERR, "cont/circuit-breaker: redis connect failed: ", err)
        return STATE_CLOSED, 0, 0, 0  -- fail open
    end
    redis_auth(sock, redis_password)
    redis_select(sock, redis_db)

    local uk = redis_key_upstream(upstream_id)
    local state_str = redis_command(sock, "HGET", uk, "state")
    local fail_count_str = redis_command(sock, "HGET", uk, "failure_count")
    local last_fail_str = redis_command(sock, "HGET", uk, "last_failure")
    local ho_succ_str = redis_command(sock, "HGET", uk, "half_open_successes")

    sock:close()

    local state = tonumber(state_str) or STATE_CLOSED
    local fail_count = tonumber(fail_count_str) or 0
    local last_failure = tonumber(last_fail_str) or 0
    local ho_successes = tonumber(ho_succ_str) or 0

    return state, fail_count, last_failure, ho_successes
end

-- ── Set CB state in Redis ─────────────────────────────────────────────────────
local function set_circuit_state(upstream_id, state, failure_count, last_failure, ho_successes)
    local r = _G.cont
    local redis_host = r and r.redis_host or "cont-redis"
    local redis_port = r and r.redis_port or 6379
    local redis_password = r and r.redis_password or ""
    local redis_db = 0
    local redis_timeout = 500

    local sock = ngx.socket.tcp()
    local ok, err = redis_connect(sock, redis_host, redis_port, redis_timeout)
    if not ok then
        ngx.log(ngx.ERR, "cont/circuit-breaker: redis set state connect failed: ", err)
        sock:close()
        return
    end
    redis_auth(sock, redis_password)
    redis_select(sock, redis_db)

    local uk = redis_key_upstream(upstream_id)
    redis_command(sock, "HSET", uk, "state", tostring(state))
    redis_command(sock, "HSET", uk, "failure_count", tostring(failure_count))
    redis_command(sock, "HSET", uk, "last_failure", tostring(last_failure))
    redis_command(sock, "HSET", uk, "half_open_successes", tostring(ho_successes))
    -- 24h TTL on upstream CB keys
    redis_command(sock, "EXPIRE", uk, 86400)

    sock:close()
end

-- ── Increment failure count ───────────────────────────────────────────────────
local function increment_failure(upstream_id)
    local r = _G.cont
    local redis_host = r and r.redis_host or "cont-redis"
    local redis_port = r and r.redis_port or 6379
    local redis_password = r and r.redis_password or ""
    local redis_db = 0
    local redis_timeout = 500

    local sock = ngx.socket.tcp()
    local ok, err = redis_connect(sock, redis_host, redis_port, redis_timeout)
    if not ok then
        sock:close()
        return
    end
    redis_auth(sock, redis_password)
    redis_select(sock, redis_db)

    local uk = redis_key_upstream(upstream_id)
    redis_command(sock, "HINCRBY", uk, "failure_count", "1")
    redis_command(sock, "HSET", uk, "last_failure", tostring(ngx.now() * 1000))
    redis_command(sock, "EXPIRE", uk, 86400)

    sock:close()
end

-- ── Reset circuit to closed ───────────────────────────────────────────────────
local function reset_circuit(upstream_id)
    set_circuit_state(upstream_id, STATE_CLOSED, 0, 0, 0)
end

-- ── Trip the circuit (move to OPEN) ──────────────────────────────────────────
local function trip_circuit(upstream_id)
    set_circuit_state(upstream_id, STATE_OPEN, 0, 0, 0)
    ngx.log(ngx.WARN, "cont/circuit-breaker: circuit OPEN for upstream=", upstream_id)
end

-- ── Main pre_proxy callback (registered in access.lua post target-selection) ──
function _M.pre_proxy(plugin, upstream_id)
    if not upstream_id or upstream_id == "" then
        return  -- no upstream, skip
    end

    -- Get CB config
    local cfg = get_upstream_config(upstream_id)
    if not cfg or cfg.enabled == false then
        return  -- CB not enabled for this upstream
    end

    local trip_threshold = cfg.trip_threshold or 5
    local half_open_success_rate = cfg.half_open_success_rate or 50  -- percent
    local half_open_max_requests = cfg.half_open_max_requests or 3
    local recovery_timeout = cfg.recovery_timeout or 30  -- seconds

    local state, fail_count, last_failure, ho_successes = get_circuit_state(upstream_id)
    local now = ngx.now() * 1000  -- ms

    if state == STATE_CLOSED then
        -- Normal operation: let request through
        -- (failures recorded in log phase)
        ngx.ctx.cb_state = STATE_CLOSED
        ngx.ctx.cb_upstream = upstream_id
        return

    elseif state == STATE_OPEN then
        -- Check if recovery timeout has elapsed -> transition to HALF_OPEN
        if last_failure > 0 and (now - last_failure) >= (recovery_timeout * 1000) then
            -- Time expired, probe with HALF_OPEN
            set_circuit_state(upstream_id, STATE_HALF_OPEN, fail_count, last_failure, 0)
            ngx.ctx.cb_state = STATE_HALF_OPEN
            ngx.ctx.cb_upstream = upstream_id
            ngx.header["X-Circuit-Breaker"] = "HALF_OPEN"
            ngx.log(ngx.INFO, "cont/circuit-breaker: OPEN -> HALF_OPEN for upstream=", upstream_id)
            return
        end

        -- Still open, reject
        ngx.header["X-Circuit-Breaker"] = "OPEN"
        ngx.header["Retry-After"] = tostring(math.ceil((recovery_timeout * 1000 - (now - last_failure)) / 1000))
        ngx.header["Content-Type"] = "application/json"
        ngx.status = 503
        ngx.say(cjson.encode({
            message = "circuit breaker open",
            error = "Service Unavailable",
            statusCode = 503,
            details = {
                upstream_id = upstream_id,
                state = "OPEN",
                retry_after = math.ceil((recovery_timeout * 1000 - (now - last_failure)) / 1000),
            },
        }))
        return ngx.exit(503)

    elseif state == STATE_HALF_OPEN then
        -- Allow limited requests through for probing
        -- ho_successes tracks how many have succeeded so far
        if ho_successes >= half_open_max_requests then
            -- Already sent enough probes, reject more until one succeeds/fails
            ngx.header["X-Circuit-Breaker"] = "HALF_OPEN"
            ngx.header["Retry-After"] = "5"
            ngx.header["Content-Type"] = "application/json"
            ngx.status = 503
            ngx.say(cjson.encode({
                message = "circuit breaker half-open, max probe requests reached",
                error = "Service Unavailable",
                statusCode = 503,
            }))
            return ngx.exit(503)
        end

        -- Allow this probe request through
        ngx.ctx.cb_state = STATE_HALF_OPEN
        ngx.ctx.cb_upstream = upstream_id
        ngx.ctx.cb_probe = true
        ngx.header["X-Circuit-Breaker"] = "HALF_OPEN"
        return
    end
end

-- ── Log phase: record success/failure ────────────────────────────────────────
function _M.log(plugin)
    local upstream_id = ngx.ctx.cb_upstream
    if not upstream_id then return end

    local cfg = get_upstream_config(upstream_id)
    if not cfg or cfg.enabled == false then return end

    local trip_threshold = cfg.trip_threshold or 5
    local half_open_success_rate = cfg.half_open_success_rate or 50
    local half_open_max_requests = cfg.half_open_max_requests or 3

    local state = ngx.ctx.cb_state or STATE_CLOSED
    local status = ngx.var.status
    local is_error = tonumber(status) >= 500 or tonumber(status) == 0

    local _, fail_count, last_failure, ho_successes = get_circuit_state(upstream_id)

    if state == STATE_CLOSED then
        if is_error then
            increment_failure(upstream_id)
            fail_count = fail_count + 1
            ngx.log(ngx.WARN, "cont/circuit-breaker: CLOSED failure #", fail_count, "/", trip_threshold, " upstream=", upstream_id)
            if fail_count >= trip_threshold then
                trip_circuit(upstream_id)
            else
                set_circuit_state(upstream_id, STATE_CLOSED, fail_count, ngx.now() * 1000, 0)
            end
        end

    elseif state == STATE_HALF_OPEN then
        if ngx.ctx.cb_probe then
            if is_error then
                -- Probe failed, trip back to OPEN
                ngx.log(ngx.WARN, "cont/circuit-breaker: HALF_OPEN probe FAILED -> OPEN upstream=", upstream_id)
                trip_circuit(upstream_id)
            else
                -- Probe succeeded, increment success counter
                ho_successes = ho_successes + 1
                local r = _G.cont
                local redis_host = r and r.redis_host or "cont-redis"
                local redis_port = r and r.redis_port or 6379
                local redis_password = r and r.redis_password or ""
                local redis_timeout = 500

                local sock = ngx.socket.tcp()
                if redis_connect(sock, redis_host, redis_port, redis_timeout) then
                    redis_auth(sock, redis_password)
                    redis_select(sock, 0)
                    local uk = redis_key_upstream(upstream_id)
                    redis_command(sock, "HSET", uk, "half_open_successes", tostring(ho_successes))
                    redis_command(sock, "EXPIRE", uk, 86400)
                    sock:close()
                end

                ngx.log(ngx.INFO, "cont/circuit-breaker: HALF_OPEN probe OK #", ho_successes, "/", half_open_max_requests, " upstream=", upstream_id)

                -- Check if enough successes to close
                local success_rate = (ho_successes / half_open_max_requests) * 100
                if ho_successes >= half_open_max_requests and success_rate >= half_open_success_rate then
                    ngx.log(ngx.INFO, "cont/circuit-breaker: HALF_OPEN -> CLOSED (success rate ", string.format("%.0f", success_rate), "%) upstream=", upstream_id)
                    reset_circuit(upstream_id)
                end
            end
        end
    end
end

return _M