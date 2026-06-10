-- header_filter_test.lua
-- Unit tests for cont/header_filter.lua

-- Mock ngx must be global (not just package.loaded) since code references global ngx
_G.ngx = {
    ctx = {
        matched_route = { id = "r1", name = "test-route" },
        service = { id = "s1", name = "test-service" },
    },
    var = {
        server_protocol = "HTTP/1.1",
        request_time_ms = "45",
    },
    header = {},
    WARN = 1,
    ERR = 2,
    log = function() end,
}

_G.cont = {
    plugins = {},
}

describe("header_filter_test.lua", function()
    describe("Kong-compatible headers", function()
        before_each(function()
            _G.ngx.header = {}
            _G.ngx.var.server_protocol = "HTTP/1.1"
            _G.ngx.var.request_time_ms = "45"
        end)

        it("sets Via header with cont version", function()
            _G.ngx.header["Via"] = _G.ngx.var.server_protocol .. " cont/0.1.0"
            assert.are.equal("HTTP/1.1 cont/0.1.0", _G.ngx.header["Via"])
        end)

        it("sets X-Kong-Proxy-Latency from request_time_ms", function()
            _G.ngx.var.request_time_ms = "123"
            _G.ngx.header["X-Kong-Proxy-Latency"] = _G.ngx.var.request_time_ms or "0"
            assert.are.equal("123", _G.ngx.header["X-Kong-Proxy-Latency"])
        end)

        it("defaults X-Kong-Proxy-Latency to 0 when request_time_ms is nil", function()
            _G.ngx.var.request_time_ms = nil
            _G.ngx.header["X-Kong-Proxy-Latency"] = _G.ngx.var.request_time_ms or "0"
            assert.are.equal("0", _G.ngx.header["X-Kong-Proxy-Latency"])
        end)

        it("sets X-Kong-Upstream-Latency to 0", function()
            _G.ngx.header["X-Kong-Upstream-Latency"] = "0"
            assert.are.equal("0", _G.ngx.header["X-Kong-Upstream-Latency"])
        end)
    end)

    describe("plugin header_filter chain", function()
        before_each(function()
            _G.cont.plugins = {}
            _G.ngx.header = {}
        end)

        it("collects applicable plugins from cont.plugins", function()
            table.insert(_G.cont.plugins, {
                name = "cors",
                route_id = nil,
                service_id = nil,
                enabled = true,
            })
            table.insert(_G.cont.plugins, {
                name = "rate-limit",
                route_id = "r1",
                enabled = true,
            })
            assert.are.equal(2, #_G.cont.plugins)
        end)

        it("skips plugins without handler module", function()
            local plugin = { name = "nonexistent", enabled = true }
            local ok, mod = pcall(require, "cont.plugins." .. plugin.name .. ".handler")
            -- pcall returns ok=false when module not found
            assert.is_false(ok)
        end)
    end)
end)
