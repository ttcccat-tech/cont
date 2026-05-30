-- cont.healthcheck
-- Active health checks for upstream targets

-- TODO: implement active health checks
-- For each upstream, periodically probe targets via HTTP/TCP
-- Mark healthy/unhealthy in Redis

local cont = require("cont.init")
local http = require("resty.http")

local function check_target(upstream_id, target)
    local host, port = string.match(target.target, "([^:]+):(%d+)")
    if not host then
        host = target.target
        port = 80
    end

    local httpc = http.new()
    httpc:set_timeout(3000)

    local ok, err = httpc:connect(host, port)
    if not ok then
        return false, err
    end

    httpc:close()

    return true, nil
end

for upstream_id, targets in pairs(cont.targets) do
    for _, target in ipairs(targets) do
        local healthy, err = check_target(upstream_id, target)
        if not healthy then
            ngx.log(ngx.WARN, "cont: target ", target.target, " is unhealthy: ", err)
        end
    end
end

return true
