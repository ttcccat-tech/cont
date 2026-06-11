-- cont.worker
-- Per-worker initialization and background timers

local cont = require("init")

-- Config reload timer (every 10 seconds)
local function start_config_sync()
    local ok, err = ngx.timer.every(10, function()
        -- Reload routes, services, plugins from Admin API
        local ok2, err2 = pcall(function()
            cont.build_route_table()
            cont.load_plugins()
            cont.load_upstreams()
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
        local ok2, err2 = pcall(require, "cont.healthcheck")
        if not ok2 then
            ngx.log(ngx.ERR, "cont: healthcheck error: ", err2)
        end
    end)
    if not ok then
        ngx.log(ngx.ERR, "cont: failed to start healthcheck timer: ", err)
    end
end

start_config_sync()
start_healthcheck()
