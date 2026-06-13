-- cont.access
-- Route matching + plugin access() chain
-- Implements: JWT validation, API Key/BasicAuth/HMACAuth, rate-limit, OPTIONS preflight
-- NOTE: Uses ngx.location.capture (no resty.http in OpenResty Alpine)

local cjson = require("cjson")

-- Access init from _G (avoids require loop in init_by_lua context)
local function get_cont()
    return _G.cont or {}
end

-- Generate a random trace ID (32 hex chars)
local function generate_trace_id()
    local chars = "0123456789abcdef"
    local id = {}
    for i = 1, 32 do
        id[i] = chars:sub(math.random(1, 16), 16)
    end
    return table.concat(id)
end

-- ── Internal Admin API call via ngx.location.capture ──────────────────────────
local function admin_api_call(path)
    local trace_id = ngx.var.http_x_cont_trace_id or ""
    local headers = {}
    if trace_id ~= "" then
        headers["X-Cont-Trace-ID"] = trace_id
    end
    local res = ngx.location.capture("/__cont_api_internal__" .. path, {
        headers = headers,
    })
    return res.status, res.body
end

-- ── JWT Validation (via Admin API /internal/validate-jwt) ─────────────────────
local function validate_jwt(token)
    if not token or token == "" then
        return nil
    end
    local status, body = admin_api_call("/internal/validate-jwt/" .. ngx.escape_uri(token))
    if status ~= 200 then
        return nil
    end
    local ok, data = pcall(cjson.decode, body)
    if not ok or not data then
        return nil
    end
    return data.consumer_id, data.user_id
end

-- ── Consumer Auth Validation ─────────────────────────────────────────────────
local function validate_consumer_auth(credential_type)
    local key = nil
    local secret = nil

    if credential_type == "key-auth" then
        key = ngx.var.http_x_api_key or ngx.var.arg_apikey
        if not key or key == "" then
            ngx.header["WWW-Authenticate"] = 'Key realm="API"'
            ngx.status = 401
            ngx.say('{"message":"No API key provided","error":"Unauthorized","statusCode":401}')
            return false
        end
    elseif credential_type == "basic-auth" then
        local auth_hdr = ngx.var.http_authorization or ""
        if auth_hdr:sub(1, 6) ~= "Basic " then
            ngx.header["WWW-Authenticate"] = 'Basic realm="API"'
            ngx.status = 401
            ngx.say('{"message":"No basic auth credentials provided","error":"Unauthorized","statusCode":401}')
            return false
        end
        local b64 = auth_hdr:sub(7)
        local decoded = ngx.decode_base64(b64)
        if not decoded or decoded == "" then
            ngx.status = 401
            ngx.say('{"message":"Invalid basic auth encoding","error":"Unauthorized","statusCode":401}')
            return false
        end
        local colon_pos = decoded:find(":")
        if not colon_pos then
            ngx.status = 401
            ngx.say('{"message":"Invalid basic auth format (expected username:password)","error":"Unauthorized","statusCode":401}')
            return false
        end
        key = decoded:sub(1, colon_pos - 1)
        secret = decoded:sub(colon_pos + 1)
    elseif credential_type == "hmac-auth" then
        local auth_hdr = ngx.var.http_authorization or ""
        if auth_hdr:sub(1, 9) ~= "HMACAuth " then
            ngx.header["WWW-Authenticate"] = 'HMACAuth realm="API"'
            ngx.status = 401
            ngx.say('{"message":"No HMAC auth credentials provided","error":"Unauthorized","statusCode":401}')
            return false
        end
        local payload = auth_hdr:sub(10)
        local colon1 = payload:find(":")
        local colon2 = colon1 and payload:find(":", colon1 + 1)
        if not colon1 or not colon2 then
            ngx.status = 401
            ngx.say('{"message":"Invalid HMAC auth format (expected client_id:signature:timestamp)","error":"Unauthorized","statusCode":401}')
            return false
        end
        key = payload:sub(1, colon1 - 1)
        secret = payload:sub(colon1 + 1, colon2 - 1)
    end

    -- Call Admin API to validate credential
    local status, body = admin_api_call("/internal/validate-cred/" .. credential_type .. "/" .. ngx.escape_uri(key))
    if status ~= 200 then
        ngx.status = 401
        ngx.say('{"message":"Invalid credentials","error":"Unauthorized","statusCode":401}')
        return false
    end

    local ok, res = pcall(cjson.decode, body)
    if ok and res and res.consumer_id then
        ngx.ctx.authenticated_consumer_id = res.consumer_id
        ngx.ctx.credential_identifier = key
    end
    return true
end

-- ── Route Matching ───────────────────────────────────────────────────────────
local function match_route()
    local cont = get_cont()
    local host = ngx.var.http_host
    local path = ngx.var.uri
    local method = ngx.req.get_method()

    local matched_route = nil
    local highest_priority = -1

    for _, route in ipairs(cont.routes or {}) do
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
                if h == host or (string.find(h, "%*.") == 1 and string.sub(host, #h - string.find(h, "%*.") + 2) == string.sub(h, 3)) then
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
                if string.sub(path, 1, string.len(p)) == p then
                    path_match = true
                    break
                end
            end
            if not path_match then goto continue end
        end

        local priority = route.regex_priority or 0
        if priority > highest_priority then
            highest_priority = priority
            matched_route = route
        end

        ::continue::
    end

    return matched_route
end

-- ── Plugin Access ────────────────────────────────────────────────────────────
local function run_plugin_access(plugin)
    local plugin_name = plugin.name
    local ok, mod = pcall(require, "plugins." .. plugin_name .. ".handler")
    if not ok or not mod then
        return
    end

    local handler = mod.new()
    if handler.access then
        local ok2, err = pcall(handler.access, handler)
        if not ok2 then
            ngx.log(ngx.ERR, "cont: plugin ", plugin_name, " access() error: ", err)
        end
    end
end

-- ── Load Balancer ────────────────────────────────────────────────────────────
local function select_target(upstream_id)
    local cont = get_cont()
    if not upstream_id then
        return nil
    end

    local targets = cont.targets and cont.targets[upstream_id]
    if not targets or #targets == 0 then
        ngx.log(ngx.WARN, "cont: no healthy targets for upstream ", upstream_id)
        return nil
    end

    local upstream = cont.upstreams and cont.upstreams[upstream_id]
    local algorithm = upstream and upstream.algorithm or "roundrobin"

    if algorithm == "roundrobin" then
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
        return targets[1].target

    elseif algorithm == "weighted-ip-hash" then
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

-- ── Get Applicable Plugins ───────────────────────────────────────────────────
local function get_applicable_plugins(route_id, service_id)
    local cont = get_cont()
    local out = {}
    for _, p in ipairs(cont.plugins or {}) do
        if p.route_id == route_id or p.service_id == service_id
           or (not p.route_id and not p.service_id) then
            table.insert(out, p)
        end
    end
    return out
end

-- ── OPTIONS Preflight Handler ────────────────────────────────────────────────
local function handle_options_preflight()
    local origin = ngx.var.http_origin or "*"
    ngx.header["Access-Control-Allow-Origin"] = origin
    ngx.header["Access-Control-Allow-Methods"] = "GET, POST, PUT, PATCH, DELETE, OPTIONS"
    ngx.header["Access-Control-Allow-Headers"] = "Content-Type, Authorization, Kong-Admin-Token, X-API-Key, X-Requested-With"
    ngx.header["Access-Control-Allow-Credentials"] = "true"
    ngx.header["Access-Control-Max-Age"] = "86400"
    ngx.status = 204
    return ngx.exit(204)
end

-- ── Main Access Phase ─────────────────────────────────────────────────────────
if ngx.req.get_method() == "OPTIONS" then
    handle_options_preflight()
end

local cont = get_cont()

-- Generate or propagate trace ID for distributed tracing
local trace_id = ngx.var.http_x_cont_trace_id
if not trace_id or trace_id == "" then
    trace_id = generate_trace_id()
end
ngx.var.cont_trace_id = trace_id
ngx.header["X-Cont-Trace-ID"] = trace_id

if not cont.routes or #cont.routes == 0 then
    -- Load config from Admin API if not yet loaded
    local status, body = admin_api_call("/internal/config/snapshot")
    if status == 200 then
        local ok, data = pcall(cjson.decode, body)
        if ok and data then
            _G.cont.routes = data.routes or {}
            _G.cont.services = data.services or {}
            _G.cont.upstreams = data.upstreams or {}
            _G.cont.plugins = data.plugins or {}
            _G.cont.targets = data.targets or {}
            _G.cont.config_loaded = true
            cont = _G.cont
        end
    end
end

local route = match_route()

if not route then
    ngx.status = 404
    ngx.say('{"message":"no route matched","error":"Not Found","statusCode":404}')
    return ngx.exit(404)
end

ngx.ctx.matched_route = route
ngx.ctx.route_id = route.id

local service_id = route.service_id
local service = cont.services and cont.services[service_id]

if not service then
    ngx.status = 503
    ngx.say('{"message":"service not found","error":"Service Unavailable","statusCode":503}')
    return ngx.exit(503)
end

ngx.ctx.service_id = service_id
ngx.ctx.service = service

-- JWT auth check
if route and service_id then
    local has_jwt = false
    for _, p in ipairs(cont.plugins or {}) do
        if p.name == "jwt" and (p.route_id == route.id or p.service_id == service_id) then
            has_jwt = true
            break
        end
    end
    if has_jwt then
        local auth_hdr = ngx.var.http_authorization or ""
        local token = nil
        if string.sub(auth_hdr, 1, 7) == "Bearer " then
            token = auth_hdr:sub(8)
        elseif ngx.var.http_x_auth_token then
            token = ngx.var.http_x_auth_token
        end
        if token then
            local consumer_id, user_id = validate_jwt(token)
            if consumer_id then
                ngx.ctx.authenticated_consumer_id = consumer_id
                ngx.ctx.authenticated_user_id = user_id
            else
                ngx.status = 401
                ngx.say('{"message":"Invalid or expired JWT token","error":"Unauthorized","statusCode":401}')
                return ngx.exit(401)
            end
        else
            ngx.header["WWW-Authenticate"] = 'Bearer realm="API"'
            ngx.status = 401
            ngx.say('{"message":"No JWT token provided","error":"Unauthorized","statusCode":401}')
            return ngx.exit(401)
        end
    end
end

-- Consumer auth check
local function has_consumer_auth(route, service_id)
    for _, p in ipairs(cont.plugins or {}) do
        if p.name == "key-auth" or p.name == "basic-auth" or p.name == "hmac-auth" then
            if p.route_id == route.id or p.service_id == service_id then
                return p.name
            end
        end
    end
    return nil
end

local cred_type = has_consumer_auth(route, service_id)
if cred_type then
    if not validate_consumer_auth(cred_type) then
        return
    end
end

-- Record request start time for latency tracking in plugins
ngx.ctx.request_start_time = ngx.now() * 1000

-- Run per-plugin access()
for _, plugin in ipairs(get_applicable_plugins(route.id, service_id)) do
    if plugin.name == "jwt" or plugin.name == "key-auth"
       or plugin.name == "basic-auth" or plugin.name == "hmac-auth" then
    else
        run_plugin_access(plugin)
    end
    if ngx.status >= 400 then
        return
    end
end

-- Determine upstream target
local upstream_target = nil

if service.host then
    upstream_target = service.host .. ":" .. (service.port or 80)
    ngx.var.cont_upstream = "http://" .. upstream_target
    ngx.var.cont_upstream_host = service.host
elseif service.upstream_id then
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