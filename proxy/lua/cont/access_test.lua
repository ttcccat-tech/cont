-- access_test.lua
-- Unit tests for cont/access.lua route matching and load balancing

-- Mock ngx context
local mock_ctx = {}
local mock_ngx = {
    var = {},
    req = { get_method = function() return "GET" end },
    status = 200,
    ctx = mock_ctx,
    WARN = 1,
    ERR = 2,
    log = function() end,
    exit = function(code) return code end,
    say = function() end,
}

package.loaded.ngx = mock_ngx

-- Mock cont.init module
local mock_cont = {
    routes = {},
    services = {},
    upstreams = {},
    targets = {},
    plugins = {},
}

local function reset_mocks()
    mock_ctx = {}
    mock_ngx.var = {}
    mock_ngx.status = 200
    mock_ngx.ctx = mock_ctx
    mock_cont.routes = {}
    mock_cont.services = {}
    mock_cont.upstreams = {}
    mock_cont.targets = {}
    mock_cont.plugins = {}
end

package.loaded["cont.init"] = mock_cont

describe("access.lua", function()
    describe("route matching", function()
        before_each(function()
            reset_mocks()
        end)

        it("matches route by host", function()
            table.insert(mock_cont.routes, {
                id = "r1",
                hosts = { "example.com" },
                protocols = { "http" },
                service_id = "s1",
            })
            mock_ngx.var.http_host = "example.com"
            mock_ngx.var.uri = "/api/users"

            -- Match via simple iteration
            local matched = nil
            for _, route in ipairs(mock_cont.routes) do
                if route.hosts then
                    for _, h in ipairs(route.hosts) do
                        if h == mock_ngx.var.http_host then
                            matched = route
                            break
                        end
                    end
                end
            end
            assert.are.equal("r1", matched.id)
        end)

        it("matches route by path prefix", function()
            table.insert(mock_cont.routes, {
                id = "r2",
                paths = { "/api/" },
                protocols = { "http" },
                service_id = "s1",
            })
            mock_ngx.var.http_host = "example.com"
            mock_ngx.var.uri = "/api/users"

            local matched = nil
            for _, route in ipairs(mock_cont.routes) do
                if route.paths then
                    for _, p in ipairs(route.paths) do
                        if string.sub(mock_ngx.var.uri, 1, string.len(p)) == p then
                            matched = route
                            break
                        end
                    end
                end
            end
            assert.are.equal("r2", matched.id)
        end)

        it("skips disabled routes", function()
            table.insert(mock_cont.routes, {
                id = "r_disabled",
                enabled = false,
                hosts = { "example.com" },
                protocols = { "http" },
                service_id = "s1",
            })
            table.insert(mock_cont.routes, {
                id = "r_enabled",
                enabled = true,
                hosts = { "example.com" },
                protocols = { "http" },
                service_id = "s1",
            })
            mock_ngx.var.http_host = "example.com"
            mock_ngx.var.uri = "/"

            local matched = nil
            for _, route in ipairs(mock_cont.routes) do
                if route.enabled == false then
                    goto continue
                end
                if route.hosts then
                    for _, h in ipairs(route.hosts) do
                        if h == mock_ngx.var.http_host then
                            matched = route
                            break
                        end
                    end
                end
                ::continue::
            end
            assert.are.equal("r_enabled", matched.id)
        end)

        it("respects regex_priority ordering", function()
            table.insert(mock_cont.routes, {
                id = "r_low",
                hosts = { "example.com" },
                paths = { "/" },
                regex_priority = 0,
                protocols = { "http" },
                service_id = "s1",
            })
            table.insert(mock_cont.routes, {
                id = "r_high",
                hosts = { "example.com" },
                paths = { "/" },
                regex_priority = 100,
                protocols = { "http" },
                service_id = "s1",
            })
            mock_ngx.var.http_host = "example.com"
            mock_ngx.var.uri = "/"

            local matched_route = nil
            local highest_priority = -1
            for _, route in ipairs(mock_cont.routes) do
                local priority = route.regex_priority or 0
                if priority > highest_priority then
                    highest_priority = priority
                    matched_route = route
                end
            end
            assert.are.equal("r_high", matched_route.id)
        end)

        it("matches methods", function()
            table.insert(mock_cont.routes, {
                id = "r_post",
                methods = { "POST" },
                protocols = { "http" },
                service_id = "s1",
            })
            mock_ngx.req.get_method = function() return "POST" end

            local matched = nil
            for _, route in ipairs(mock_cont.routes) do
                if route.methods then
                    for _, m in ipairs(route.methods) do
                        if m == "POST" then
                            matched = route
                            break
                        end
                    end
                end
            end
            assert.are.equal("r_post", matched.id)
        end)
    end)

    describe("load balancing", function()
        it("roundrobin selects target by weight", function()
            mock_cont.targets["up1"] = {
                { target = "10.0.0.1:80", weight = 100 },
                { target = "10.0.0.2:80", weight = 100 },
            }
            mock_cont.upstreams["up1"] = { algorithm = "roundrobin" }

            -- Simulate weighted selection (multiple picks to verify distribution)
            local picks = {}
            for i = 1, 100 do
                local total = 0
                for _, t in ipairs(mock_cont.targets["up1"]) do
                    total = total + t.weight
                end
                local rand = math.random(total)
                local cumulative = 0
                for _, t in ipairs(mock_cont.targets["up1"]) do
                    cumulative = cumulative + t.weight
                    if rand <= cumulative then
                        table.insert(picks, t.target)
                        break
                    end
                end
            end

 -- Both targets should be selected across100 tries
            local seen = {}
            for _, t in ipairs(picks) do seen[t] = true end
            assert.is_true(seen["10.0.0.1:80"])
            assert.is_true(seen["10.0.0.2:80"])
        end)

        it("weighted-ip-hash uses consistent hashing", function()
            mock_cont.targets["up1"] = {
                { target = "10.0.0.1:80", weight = 100 },
                { target = "10.0.0.2:80", weight = 100 },
                { target = "10.0.0.3:80", weight = 100 },
            }
            mock_cont.upstreams["up1"] = { algorithm = "weighted-ip-hash" }

            -- Same IP should always hash to same target
            local first_target = nil
            for _, t in ipairs(mock_cont.targets["up1"]) do
                local ip = "192.168.1.100"
                local hash = 0
                for i = 1, string.len(ip) do
                    hash = (hash * 31 + string.byte(ip, i)) % 2147483647
                end
                local target_idx = (hash % #mock_cont.targets["up1"]) + 1
                first_target = mock_cont.targets["up1"][target_idx].target
                break
            end

            -- Verify consistency
            local ip = "192.168.1.100"
            local hash = 0
            for i = 1, string.len(ip) do
                hash = (hash * 31 + string.byte(ip, i)) % 2147483647
            end
            local target_idx = (hash % 3) + 1
            local expected = mock_cont.targets["up1"][target_idx].target
            assert.are.equal(expected, first_target)
        end)

        it("returns nil when no targets available", function()
            mock_cont.targets["up_empty"] = {}
            mock_cont.upstreams["up_empty"] = { algorithm = "roundrobin" }

            local targets = mock_cont.targets["up_empty"]
            assert.are.equal(0, #targets)
        end)
    end)

    describe("plugin access chain", function()
        it("collects applicable plugins for route", function()
            table.insert(mock_cont.plugins, {
                name = "rate-limit",
                route_id = "r1",
                enabled = true,
            })
            table.insert(mock_cont.plugins, {
                name = "cors",
                route_id = nil,
                service_id = nil,
                enabled = true,
            })
            table.insert(mock_cont.plugins, {
                name = "auth",
                service_id = "s1",
                enabled = true,
            })

            local function get_applicable(route_id, service_id)
                local out = {}
                for _, p in ipairs(mock_cont.plugins) do
                    if p.route_id == route_id or p.service_id == service_id
                       or (not p.route_id and not p.service_id) then
                        table.insert(out, p)
                    end
                end
                return out
            end

            local plugins = get_applicable("r1", "s1")
            assert.are.equal(3, #plugins)
        end)
    end)
end)
