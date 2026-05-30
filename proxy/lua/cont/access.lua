-- cont.access
-- Route matching + plugin access() chain

local cont = require("cont.init")

-- Match request to a route
local function match_route()
    local host = ngx.var.http_host
    local path = ngx.var.uri
    local method = ngx.req.get_method()

    -- Sort routes by regex_priority descending (higher = more specific)
    local matched_route = nil
    local highest_priority = -1

    for _, route in ipairs(cont.routes) do
        if route.enabled == false then goto continue end

        -- Protocol check
        if route.protocols then
            local proto_match = false
            for _, proto in ipairs(route.protocols) do
                if proto == "http" or proto == "https" then
                    proto_match = true
                    break
                end
            end
            if not proto_match then goto continue end
        end

        -- Method match
        if route.methods and #route.methods > 0 then
            local method_match = false
            for _, m in ipairs(route.methods) do
                if m == method then
                    method_match = true
                    break
                end
            end
            if not method_match then goto continue end
        end

        -- Host match
        if route.hosts and #route.hosts > 0 then
            local host_match = false
            for _, h in ipairs(route.hosts) do
                if h == host or h == "*." .. string.sub(host, string.find(host, "%.") + 1) then
                    host_match = true
                    break
                end
            end
            if not host_match then goto continue end
        end

        -- Path match (prefix or exact)
        if route.paths and #route.paths > 0 then
            local path_match = false
            for _, p in ipairs(route.paths) do
                -- Prefix match (Kong default)
                if string.sub(path, 1, string.len(p)) == p then
                    path_match = true
                    break
                end
            end
            if not path_match then goto continue end
        end

        -- This route matched — check regex_priority
        local priority = route.regex_priority or 0
        if priority > highest_priority then
            highest_priority = priority
            matched_route = route
        end

        ::continue::
    end

    return matched_route
end

-- Run access() for matched plugin
local function run_plugin_access(plugin)
    local plugin_name = plugin.name
    local ok, mod = pcall(require, "cont.plugins." .. plugin_name .. ".handler")
    if not ok or not mod then
        return  -- plugin handler not found, skip
    end

    local handler = mod.new()
    if handler.access then
        local ok2, err = pcall(handler.access, handler)
        if not ok2 then
            ngx.log(ngx.ERR, "cont: plugin ", plugin_name, " access() error: ", err)
        end
    end
end

-- Select upstream target (load balancer)
local function select_target(upstream_id)
    if not upstream_id then
        return nil
    end

    local targets = cont.targets[upstream_id]
    if not targets or #targets == 0 then
        ngx.log(ngx.WARN, "cont: no healthy targets for upstream ", upstream_id)
        return nil
    end

    local upstream = cont.upstreams[upstream_id]
    local algorithm = upstream and upstream.algorithm or "roundrobin"

    if algorithm == "roundrobin" then
        -- Simple weighted round-robin
        local total = 0
        for _, t in ipairs(targets) do
            total = total + t.weight
        end
        local rand = math.random(total)
        local cumulative = 0
        for _, t in ipairs(targets) do
            cumulative = cumulative + t.weight
            if rand <= cumulative then
                return t.target
            end
        end
        return targets[1].target

    elseif algorithm == "leastconnections" then
        -- TODO: track per-target connection count in shared dict
        return targets[1].target

    elseif algorithm == "weighted-ip-hash" then
        -- Consistent hash by client IP
        local ip = ngx.var.remote_addr or "0.0.0.0"
        local hash = 0
        for i = 1, string.len(ip) do
            hash = (hash * 31 + string.byte(ip, i)) % 2147483647
        end
        local target_idx = (hash % #targets) + 1
        return targets[target_idx].target
    end

    return targets[1].target
end

-- Main access phase
local route = match_route()

if not route then
    ngx.status = 404
    ngx.say('{"message":"no route matched","error":"Not Found","statusCode":404}')
    return ngx.exit(404)
end

-- Store matched route in context for later phases
ngx.ctx.matched_route = route
ngx.ctx.route_id = route.id

-- Get service for this route
local service_id = route.service_id
local service = cont.services[service_id]

if not service then
    ngx.status = 503
    ngx.say('{"message":"service not found","error":"Service Unavailable","statusCode":503}')
    return ngx.exit(503)
end

ngx.ctx.service_id = service_id
ngx.ctx.service = service

-- Run per-plugin access() (global plugins first, then route, then service)
local function get_applicable_plugins(route_id, service_id)
    local out = {}
    for _, p in ipairs(cont.plugins) do
        if p.route_id == route_id or p.service_id == service_id
           or (not p.route_id and not p.service_id) then
            table.insert(out, p)
        end
    end
    return out
end

for _, plugin in ipairs(get_applicable_plugins(route.id, service_id)) do
    run_plugin_access(plugin)
    if ngx.status >= 400 then
        return  -- plugin rejected the request
    end
end

-- Determine upstream target
local upstream_target = nil

if service.host then
    -- Direct host (no upstream)
    upstream_target = service.host .. ":" .. (service.port or 80)
    ngx.var.cont_upstream = "http://" .. upstream_target
    ngx.var.cont_upstream_host = service.host
elseif service.upstream_id then
    -- Via upstream — run load balancer
    local target = select_target(service.upstream_id)
    if not target then
        ngx.status = 503
        ngx.say('{"message":"no healthy upstream","error":"Service Unavailable","statusCode":503}')
        return ngx.exit(503)
    end
    upstream_target = target
    ngx.var.cont_upstream = "http://" .. target
    ngx.var.cont_upstream_host = string.match(target, "([^:]+)")
end

-- Strip path if route.strip_path
if route.strip_path and route.paths and #route.paths > 0 then
    local prefix = route.paths[1]
    if string.sub(ngx.var.uri, 1, string.len(prefix)) == prefix then
        local new_uri = string.sub(ngx.var.uri, string.len(prefix) + 1)
        if new_uri == "" then new_uri = "/" end
        ngx.var.uri = new_uri
    end
end

ngx.ctx.upstream_target = upstream_target
return cont
