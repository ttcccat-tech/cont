-- cont.jwt_validation
-- Validates JWT tokens via Admin API using cosocket (safe at access_by_lua context)
-- Replaces the old admin_api_call() approach that failed across C-call boundaries

local cjson = require("cjson")

local ADMIN_API_HOST = os.getenv("CONT_ADMIN_API_HOST") or "cont-admin-api"
local ADMIN_API_PORT = os.getenv("CONT_ADMIN_API_PORT") or "8001"

local _M = {}

function _M.validate_jwt(token)
    if not token or token == "" then
        return nil
    end
    local sock = ngx.socket.tcp()
    sock:settimeout(3000)
    local ok, err = sock:connect(ADMIN_API_HOST, tonumber(ADMIN_API_PORT))
    if not ok then
        return nil
    end
    local path = "/internal/validate-jwt/" .. ngx.escape_uri(token)
    local req = "GET " .. path .. " HTTP/1.0\r\nHost: " .. ADMIN_API_HOST .. "\r\nConnection: close\r\n\r\n"
    local bytes, err = sock:send(req)
    if not bytes then
        sock:close()
        return nil
    end
    local reader = sock:receiveuntil("\r\n\r\n")
    local headers, err = reader()
    if not headers then
        sock:close()
        return nil
    end
    local content_length = nil
    for k, v in headers:gmatch("([%w%-]+):%s*([^\r\n]+)") do
        if k:lower() == "content-length" then
            content_length = tonumber(v)
        end
    end
    local body = ""
    if content_length then
        local remaining, err = sock:receive(content_length)
        if remaining then body = remaining end
    else
        body, err = sock:receive("*a")
    end
    sock:close()
    if body == "" then
        return nil
    end
    local ok2, data = pcall(cjson.decode, body)
    if not ok2 or not data then
        return nil
    end
    return data.consumer_id, data.user_id
end

function _M.validate_consumer_auth(credential_type, key)
    if not key or key == "" then
        return nil
    end
    local sock = ngx.socket.tcp()
    sock:settimeout(3000)
    local ok, err = sock:connect(ADMIN_API_HOST, tonumber(ADMIN_API_PORT))
    if not ok then
        return nil
    end
    local path = "/internal/validate-cred/" .. credential_type .. "/" .. ngx.escape_uri(key)
    local req = "GET " .. path .. " HTTP/1.0\r\nHost: " .. ADMIN_API_HOST .. "\r\nConnection: close\r\n\r\n"
    local bytes, err = sock:send(req)
    if not bytes then
        sock:close()
        return nil
    end
    local reader = sock:receiveuntil("\r\n\r\n")
    local headers, err = reader()
    if not headers then
        sock:close()
        return nil
    end
    local content_length = nil
    for k, v in headers:gmatch("([%w%-]+):%s*([^\r\n]+)") do
        if k:lower() == "content-length" then
            content_length = tonumber(v)
        end
    end
    local body = ""
    if content_length then
        local remaining, err = sock:receive(content_length)
        if remaining then body = remaining end
    else
        body, err = sock:receive("*a")
    end
    sock:close()
    if body == "" then
        return nil
    end
    local ok2, data = pcall(cjson.decode, body)
    if not ok2 or not data then
        return nil
    end
    return data.consumer_id, data.user_id
end

return _M
