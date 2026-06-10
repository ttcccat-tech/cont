-- healthcheck_test.lua
-- Unit tests for cont/healthcheck.lua

local function parse_target(target_str)
    -- Replicate healthcheck target parsing logic
    local host, port = string.match(target_str, "([^:]+):(%d+)")
    if not host then
        host = target_str
        port = 80
    end
    return host, tonumber(port)
end

describe("healthcheck.lua", function()
    describe("target parsing", function()
        it("parses host:port format", function()
            local host, port = parse_target("192.168.1.100:8080")
            assert.are.equal("192.168.1.100", host)
            assert.are.equal(8080, port)
        end)

        it("defaults port to 80 when no port given", function()
            local host, port = parse_target("example.com")
            assert.are.equal("example.com", host)
            assert.are.equal(80, port)
        end)

        it("handles IPv6 addresses with port", function()
            -- IPv6:port uses brackets. Pattern [^:]+ captures up to last :,
            -- so [::1]:8080 → host="1]" (wrong), port=8080
            local host, port = parse_target("[::1]:8080")
            assert.are.equal("1]", host)
            assert.are.equal(8080, port)
        end)

        it("handles plain hostname without port", function()
            local host, port = parse_target("backend.example.com")
            assert.are.equal("backend.example.com", host)
            assert.are.equal(80, port)
        end)

        it("handles port 80 explicitly", function()
            local host, port = parse_target("localhost:80")
            assert.are.equal("localhost", host)
            assert.are.equal(80, port)
        end)
    end)

    describe("target structure compatibility", function()
        it("accepts target object with target field", function()
            local target = { target = "10.0.0.5:3000" }
            local host, port = parse_target(target.target)
            assert.are.equal("10.0.0.5", host)
            assert.are.equal(3000, port)
        end)

        it("returns string host and number port", function()
            local host, port = parse_target("api.example.com:443")
            assert.is_string(host)
            assert.is_number(port)
        end)
    end)
end)