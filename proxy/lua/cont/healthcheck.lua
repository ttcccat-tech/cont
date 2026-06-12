-- cont.healthcheck
-- Active health checks for upstream targets
-- Probes targets via TCP connect, logs healthy/unhealthy status
-- NOTE: Uses ngx.socket.tcp (built into OpenResty, no resty.http needed)

local function parse_target(target_str)
    -- Handle IPv6:port format: [::1]:8080
    if string.sub(target_str, 1, 1) == "[" then
        local bracket_end = string.find(target_str, "%]")
        if bracket_end then
            local host = string.sub(target_str, 2, bracket_end - 1)
            local rest = string.sub(target_str, bracket_end + 1)
            if string.sub(rest, 1, 1) == ":" then
                local port = tonumber(string.sub(rest, 2))
                return host, port or 80
            end
            return host, 80
        end
    end
    -- Handle IPv4:port or hostname:port
    local host, port = string.match(target_str, "([^:]+):(%d+)")
    if not host then
        host = target_str
        port = 80
    end
    return host, tonumber(port)
end

local function check_target(upstream_id, target)
    local host, port = parse_target(target.target)

    local tcpsock = ngx.socket.tcp()
    tcpsock:set_timeout(3000)

    local ok, err = tcpsock:connect(host, port)
    tcpsock:close()

    if not ok then
        return false, err
    end

    return true, nil
end

local function run_healthchecks()
    local cont = _G.cont or {}
    for upstream_id, targets in pairs(cont.targets or {}) do
        for _, target in ipairs(targets) do
            local healthy, err = check_target(upstream_id, target)
            if not healthy then
                ngx.log(ngx.WARN, "cont: target ", target.target, " is unhealthy: ", err)
            end
        end
    end
end

run_healthchecks()
return true
