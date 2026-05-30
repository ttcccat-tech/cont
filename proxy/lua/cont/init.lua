-- cont.init
-- Application initialization phase (runs once at startup via init_by_lua)

local cont = {}

-- Load configuration from cont.yml or Admin API
function cont.load_config()
    -- TODO: Load declarative config (cont.yml)
    -- For now, fetch initial state from Admin API
    local http = require("resty.http")
    local httpc = http.new()
    httpc:set_timeout(3000)

    local res, err = httpc:request_uri("http://127.0.0.1:8001/services", {
        method = "GET",
        headers = {
            ["Kong-Admin-Token"] = "changeme",
        }
    })

    if not res or res.status ~= 200 then
        ngx.log(ngx.WARN, "cont: could not load config from Admin API: ", err or res.status)
        return
    end

    local cjson = require("cjson")
    local data = cjson.decode(res.body)

    if data and data.data then
        for _, svc in ipairs(data.data) do
            cont.services[svc.id] = svc
        end
    end
end

-- Build route table for fast matching
function cont.build_route_table()
    local http = require("resty.http")
    local httpc = http.new()
    httpc:set_timeout(3000)

    local res, err = httpc:request_uri("http://127.0.0.1:8001/routes", {
        method = "GET",
        headers = {
            ["Kong-Admin-Token"] = "changeme",
        }
    })

    if not res or res.status ~= 200 then
        ngx.log(ngx.WARN, "cont: could not load routes: ", err or res.status)
        return
    end

    local cjson = require("cjson")
    local data = cjson.decode(res.body)

    cont.routes = {}  -- reset

    if data and data.data then
        for _, route in ipairs(data.data) do
            table.insert(cont.routes, route)
        end
    end

    ngx.log(ngx.NOTICE, "cont: loaded ", #cont.routes, " routes")
end

-- Load plugins
function cont.load_plugins()
    local http = require("resty.http")
    local httpc = http.new()
    httpc:set_timeout(3000)

    local res, err = httpc:request_uri("http://127.0.0.1:8001/plugins", {
        method = "GET",
        headers = {
            ["Kong-Admin-Token"] = "changeme",
        }
    })

    if not res or res.status ~= 200 then
        ngx.log(ngx.WARN, "cont: could not load plugins: ", err or res.status)
        return
    end

    local cjson = require("cjson")
    local data = cjson.decode(res.body)

    cont.plugins = {}  -- reset

    if data and data.data then
        for _, plugin in ipairs(data.data) do
            if plugin.enabled then
                table.insert(cont.plugins, plugin)
                ngx.log(ngx.NOTICE, "cont: loaded plugin: ", plugin.name)
            end
        end
    end
end

-- Load upstreams and targets
function cont.load_upstreams()
    local http = require("resty.http")
    local httpc = http.new()
    httpc:set_timeout(3000)

    local res, err = httpc:request_uri("http://127.0.0.1:8001/upstreams", {
        method = "GET",
        headers = {
            ["Kong-Admin-Token"] = "changeme",
        }
    })

    if not res or res.status ~= 200 then
        ngx.log(ngx.WARN, "cont: could not load upstreams: ", err or res.status)
        return
    end

    local cjson = require("cjson")
    local data = cjson.decode(res.body)

    cont.upstreams = {}

    if data and data.data then
        for _, upstream in ipairs(data.data) do
            cont.upstreams[upstream.id] = upstream
            -- Load targets for this upstream
            cont.load_targets(upstream.id)
        end
    end
end

function cont.load_targets(upstream_id)
    local http = require("resty.http")
    local httpc = http.new()
    httpc:set_timeout(3000)

    local url = "http://127.0.0.1:8001/upstreams/" .. upstream_id .. "/targets"
    local res, err = httpc:request_uri(url, {
        method = "GET",
        headers = {
            ["Kong-Admin-Token"] = "changeme",
        }
    })

    if not res or res.status ~= 200 then
        return
    end

    local cjson = require("cjson")
    local data = cjson.decode(res.body)

    cont.targets[upstream_id] = {}

    if data and data.data then
        for _, target in ipairs(data.data) do
            if target.enabled then
                table.insert(cont.targets[upstream_id], {
                    target = target.target,
                    weight = target.weight or 100,
                })
            end
        end
    end
end

-- Initial load
cont.load_config()
cont.build_route_table()
cont.load_plugins()
cont.load_upstreams()

return cont
