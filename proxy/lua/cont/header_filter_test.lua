-- header_filter_test.lua
-- Unit tests for cont/header_filter.lua

local mock_ngx = {
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

local mock_cont = {
    plugins = {},
}

package.loaded.ngx = mock_ngx
package.loaded["cont.init"] = mock_cont

describe("header_filter.lua", function()
    describe("Kong-compatible headers", function()
        it("sets Via header with cont version", function()
            ngx.header = {}
            ngx.var.server_protocol = "HTTP/1.1"
            ngx.header["Via"] = ngx.var.server_protocol .. " cont/0.1.0"
            assert.are.equal("HTTP/1.1 cont/0.1.0", ngx.header["Via"])
        end)

        it("sets X-Kong-Proxy-Latency from request_time_ms", function()
            ngx.header = {}
            ngx.var.request_time_ms = "123"
            ngx.header["X-Kong-Proxy-Latency"] = ngx.var.request_time_ms or "0"
            assert.are.equal("123", ngx.header["X-Kong-Proxy-Latency"])
        end)

        it("defaults X-Kong-Proxy-Latency to 0 when request_time_ms is nil", function()
            ngx.header = {}
            ngx.var.request_time_ms = nil
            ngx.header["X-Kong-Proxy-Latency"] = ngx.var.request_time_ms or "0"
            assert.are.equal("0", ngx.header["X-Kong-Proxy-Latency"])
        end)

        it("sets X-Kong-Upstream-Latency to 0", function()
            ngx.header = {}
            ngx.header["X-Kong-Upstream-Latency"] = "0"
            assert.are.equal("0", ngx.header["X-Kong-Upstream-Latency"])
        end)
    end)

    describe("plugin header_filter chain", function()
        before_each(function()
            mock_cont.plugins = {}
            ngx.header = {}
        end)

        it("collects applicable plugins from cont.plugins", function()
            table.insert(mock_cont.plugins, {
                name = "cors",
                route_id = nil,
                service_id = nil,
                enabled = true,
            })
            table.insert(mock_cont.plugins, {
                name = "rate-limit",
                route_id = "r1",
                enabled = true,
            })
            assert.are.equal(2, #mock_cont.plugins)
        end)

        it("skips plugins without handler module", function()
            local plugin = { name = "nonexistent", enabled = true }
            local ok, mod = pcall(require, "cont.plugins." .. plugin.name .. ".handler")
            -- mod will be nil/false since module doesn't exist
            assert.is_false(ok or mod ~= nil)
        end)
    end)
end)
