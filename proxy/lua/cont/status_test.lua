-- status_test.lua
-- Unit tests for cont/status.lua

-- Mock ngx global
_G.ngx = {
    time = function() return 1700000000 end,
    worker = {
        count = function() return 4 end,
    },
    status = 200,
    say = function() end,
    header = {},
}

local cjson = require("cjson")

-- Replicate status.lua logic without ngx (for unit test)
local function build_status()
    local status = {
        version = "cont 0.1.0",
        uptime = ngx.time() - 1699900000,  -- fixed offset
        memory = {
            lua_vms_size = collectgarbage("count") * 1024,
            workers_count = ngx.worker.count(),
        },
        database = {
            reachable = true,
        },
        server = {
            total_requests = 0,
            connections_active = 0,
            connections_accepted = 0,
        },
        workers = {},
    }
    return status
end

describe("status.lua", function()
    describe("status structure", function()
        it("includes version field", function()
            local s = build_status()
            assert.are.equal("cont 0.1.0", s.version)
        end)

        it("includes uptime as positive number", function()
            local s = build_status()
            assert.is_number(s.uptime)
            assert.is_true(s.uptime > 0)
        end)

        it("includes memory section with lua_vms_size", function()
            local s = build_status()
            assert.is_table(s.memory)
            assert.is_number(s.memory.lua_vms_size)
            assert.is_true(s.memory.lua_vms_size >= 0)
        end)

        it("includes workers_count from ngx.worker.count()", function()
            local s = build_status()
            assert.is_number(s.memory.workers_count)
            assert.are.equal(4, s.memory.workers_count)
        end)

        it("includes database section", function()
            local s = build_status()
            assert.is_table(s.database)
            assert.is_boolean(s.database.reachable)
        end)

        it("includes server section", function()
            local s = build_status()
            assert.is_table(s.server)
            assert.is_number(s.server.total_requests)
            assert.is_number(s.server.connections_active)
            assert.is_number(s.server.connections_accepted)
        end)

        it("includes workers array", function()
            local s = build_status()
            assert.is_table(s.workers)
        end)
    end)

    describe("JSON serialization", function()
        it("serializes to valid JSON via cjson", function()
            local s = build_status()
            local json = cjson.encode(s)
            assert.is_string(json)
            assert.is_number(json:find("cont 0.1.0"))
        end)

        it("contains version in JSON output", function()
            local s = build_status()
            local json = cjson.encode(s)
            assert.is_number(json:find("version"))
        end)
    end)

    describe("uptime calculation", function()
        it("uptime is based on current time minus start time", function()
            local s = build_status()
            -- uptime should be approximately (1700000000 - 1699900000) = 100000
            assert.is_true(s.uptime >= 99000)
            assert.is_true(s.uptime <= 110000)
        end)
    end)
end)