-- cont.status
-- Kong-compatible /status endpoint

local function get_worker_statuses()
    -- ngx.worker.count() — but individual worker data requires shared dict
    return {}
end

local status = {
    version = "cont 0.1.0",
    uptime = ngx.time() - (cont.start_time or ngx.time()),
    memory = {
        lua_vms_size = collectgarbage("count") * 1024,
        workers_count = ngx.worker.count(),
    },
    database = {
        reachable = true,  -- TODO: ping postgres from here
    },
    server = {
        total_requests = 0,
        connections_active = 0,
        connections_accepted = 0,
    },
    workers = get_worker_statuses(),
}

local cjson = require("cjson")
ngx.status = 200
ngx.say(cjson.encode(status))
