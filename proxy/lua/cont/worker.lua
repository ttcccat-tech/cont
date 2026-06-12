-- cont.worker
-- Per-worker initialization and background timers
-- NOTE: Uses _G.cont directly since init_by_lua already ran

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

-- Config reload timer (every 10 seconds)
local function start_config_sync()
    local ok, err = ngx.timer.every(10, function()
        local ok2, err2 = pcall(function()
            sync_plugins()
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

start_config_sync()
start_healthcheck()
