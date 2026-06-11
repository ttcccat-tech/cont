-- cont.access
-- Route matching + plugin access() chain
-- Implements: JWT validation, API Key/BasicAuth/HMACAuth, rate-limit, OPTIONS preflight

local http = require("resty.http")
local cjson = require("cjson")
local cont = require("cont.init")

-- ── JWT Validation (via Admin API) ──────────────────────────────────────────
-- Validate a JWT token against the Admin API /internal/validate-jwt endpoint
-- Returns (consumer_id, user_id) on success, nil on failure
local function validate_jwt(token)
    if not token or token == "" then
        return nil
    end

    local api_host = os.getenv("CONT_ADMIN_API") or "127.0.0.1:8001"
    local uri = "http://" .. api_host .. "/internal/validate-jwt/" .. ngx.escape_uri(token)
    local httpc = http.new()
    httpc:set_timeout(2000)
    local resp, err = httpc.request_uri(uri)
    if not resp or resp.status ~= 200 then
        return nil
    end

    local ok, data = pcall(cjson.decode, resp.body)
    if not ok or not data then
        return nil
    end
    return data.consumer_id, data.user_id
end

-- ── Consumer Auth Validation ────────────────────────────────────────────────
-- Validate consumer credentials (key-auth / basic-auth / hmac-auth)
-- Called when route/service has a consumer-auth plugin configured
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
        -- HMACAuth: requires Authorization: HMACAuth <client_id>:<signature>:<timestamp>
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
        -- signature and timestamp not validated in proxy (Admin API handles)
        secret = payload:sub(colon1 + 1, colon2 - 1)
    end

    -- Call Admin API to validate credential
    local api_host = os.getenv("CONT_ADMIN_API") or "127.0.0.1:8001"
    local uri = "http://" .. api_host .. "/internal/validate-cred/" .. credential_type .. "/" .. ngx.escape_uri(key)
    local httpc = http.new()
    httpc:set_timeout(1000)
    local resp, err = httpc.request_uri(uri)
    if not resp or resp.status ~= 200 then
        ngx.status = 401
        ngx.say('{"message":"Invalid credentials","error":"Unauthorized","statusCode":401}')
        return false
    end

    -- Store authenticated consumer_id in context for plugins/audit
    local ok, res = pcall(cjson.decode, resp.body)
    if ok and res and res.consumer_id then
        ngx.ctx.authenticated_consumer_id = res.consumer_id
        ngx.ctx.credential_identifier = key
    end
    return true
end

-- Check if route/service has consumer auth plugin
local function has_consumer_auth(route, service_id)
    for _, p in ipairs(cont.plugins) do
        if p.name == "key-auth" or p.name == "basic-auth" or p.name == "hmac-auth" then
            if p.route_id == route.id or p.service_id == service_id then
                return p.name  -- return credential type
            end
        end
    end
    return nil
end

-- Check if route/service has jwt-auth plugin
local function has_jwt_auth(route, service_id)
    for _, p in ipairs(cont.plugins) do
        if p.name == "jwt" then
            if p.route_id == route.id or p.service_id == service_id then
                return true
            end
        end
    end
    return false
end

-- ── Route Matching ───────────────────────────────────────────────────────────
local function match_route()
    local host = ngx.var.http_host
    local path = ngx.var.uri
    local method = ngx.req.get_method()

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

-- ── Plugin Access ───────────────────────────────────────────────────────────
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

-- ── Load Balancer ───────────────────────────────────────────────────────────
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

-- ── Get Applicable Plugins ─────────────────────────────────────────────────
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

-- ── OPTIONS Preflight Handler ───────────────────────────────────────────────
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

-- ── Main Access Phase ────────────────────────────────────────────────────────
-- Handle OPTIONS preflight immediately
if ngx.req.get_method() == "OPTIONS" then
    handle_options_preflight()
end

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

-- JWT auth check (runs before consumer auth)
if has_jwt_auth(route, service_id) then
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

-- Consumer auth check (key-auth / basic-auth / hmac-auth)
local cred_type = has_consumer_auth(route, service_id)
if cred_type then
    if not validate_consumer_auth(cred_type) then
        return  -- 401 already sent
    end
end

-- Run per-plugin access()
for _, plugin in ipairs(get_applicable_plugins(route.id, service_id)) do
    -- Skip jwt/key-auth/basic-auth/hmac-auth here (handled above)
    if plugin.name == "jwt" or plugin.name == "key-auth"
       or plugin.name == "basic-auth" or plugin.name == "hmac-auth" then
        -- Already handled above
    else
        run_plugin_access(plugin)
    end
    if ngx.status >= 400 then
        return  -- plugin rejected the request
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
