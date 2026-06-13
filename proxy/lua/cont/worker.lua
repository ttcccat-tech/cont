-- cont.worker
-- Per-worker initialization and background timers
-- NOTE: Uses _G.cont directly since init_by_lua already ran

local cjson = require("cjson")

-- Shared dict for CB config (written by backend sync, read by Lua)
local shm_cb_config = ngx.shared.cont_circuit_breaker_config

-- Fetch plugins from Admin API internal endpoint
local function sync_plugins()
    local res = ngx.location.capture("/__cont_api_internal__/internal/plugins")
    if res.status ~= 200 then
        ngx.log(ngx.WARN, "cont: plugin sync failed: status=", res.status)
        return
    end
    local ok, data = pcall(cjson.decode, res.body)
    if not ok then
        ngx.log(ngx.ERR, "cont: plugin sync JSON decode error: ", data)
        return
    end
    _G.cont.plugins = data.plugins or {}
    ngx.log(ngx.DEBUG, "cont: synced ", #(_G.cont.plugins or {}), " plugins")
end

-- Fetch circuit breaker configs from Admin API and write to shared memory
local function sync_circuit_breakers()
    local res = ngx.location.capture("/__cont_api_internal__/internal/circuit-breaker-configs")
    if res.status ~= 200 then
        ngx.log(ngx.WARN, "cont: CB config sync failed: status=", res.status)
        return
    end
    local ok, data = pcall(cjson.decode, res.body)
    if not ok then
        ngx.log(ngx.ERR, "cont: CB config sync JSON decode error: ", data)
        return
    end
    local configs = data.circuit_breakers or {}
    -- Clear old entries and write new ones
    if shm_cb_config then
        shm_cb_config:flush_all()
        for _, cfg in ipairs(configs) do
            local key = "cb:" .. cfg.upstream_id
            local ok2, err = pcall(function()
                shm_cb_config:set(key, cjson.encode(cfg))
            end)
            if not ok2 then
                ngx.log(ngx.ERR, "cont: CB config set error: ", err)
            end
        end
        ngx.log(ngx.DEBUG, "cont: synced ", #configs, " circuit breaker configs")
    end
end

-- Config reload timer (every 10 seconds)
local function start_config_sync()
    local ok, err = ngx.timer.every(10, function()
        local ok2, err2 = pcall(function()
            sync_plugins()
            sync_circuit_breakers()
        end)
        if not ok2 then
            ngx.log(ngx.ERR, "cont: config sync error: ", err2)
        end
    end)
    if not ok then
        ngx.log(ngx.ERR, "cont: failed to start config sync timer: ", err)
    end
end

-- Healthcheck timer (every 5 seconds)
local function start_healthcheck()
    local ok, err = ngx.timer.every(5, function()
        local ok2, err2 = pcall(require, "healthcheck")
        if not ok2 then
            ngx.log(ngx.ERR, "cont: healthcheck error: ", err2)
        end
    end)
    if not ok then
        ngx.log(ngx.ERR, "cont: failed to start healthcheck timer: ", err)
    end
end

-- Initial sync on startup
sync_plugins()
sync_circuit_breakers()

start_config_sync()
start_healthcheck()
