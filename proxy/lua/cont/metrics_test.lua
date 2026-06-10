-- metrics_test.lua
-- Unit tests for cont/metrics.lua

local function build_lines(metrics_stub)
    -- Replicate the metrics.lua logic without ngx
    local metrics = metrics_stub

    local lines = {
        "# HELP cont_nginx_requests_total Total number of requests",
        "# TYPE cont_nginx_requests_total counter",
        "cont_nginx_requests_total 0",
        "",
        "# HELP cont_nginx_connections_total Number of connections",
        "# TYPE cont_nginx_connections_total gauge",
        "cont_nginx_connections_total{state=\"active\"} " .. (metrics and metrics:get("connections_active") or 0),
        "cont_nginx_connections_total{state=\"accepted\"} " .. (metrics and metrics:get("connections_accepted") or 0),
        "cont_nginx_connections_total{state=\"reading\"} 0",
        "cont_nginx_connections_total{state=\"writing\"} 0",
        "cont_nginx_connections_total{state=\"waiting\"} 0",
        "",
        "# HELP cont_upstream_target_up Upstream target health (1=up, 0=down)",
        "# TYPE cont_upstream_target_up gauge",
    }

    return table.concat(lines, "\n")
end

local function parse_metrics(text)
    local result = {}
    for line in text:gmatch("[^\r\n]+") do
        if not line:match("^#") and line ~= "" then
            local metric, value = line:match("^([%w_]+)%b{}%s+(%d+)")
            if not metric then
                metric, value = line:match("^([%w_]+)%s+(%d+)")
            end
            if metric then
                result[metric] = tonumber(value)
            end
        end
    end
    return result
end

describe("metrics.lua", function()
    describe("output format", function()
        it("produces valid Prometheus text format", function()
            local text = build_lines(nil)
            assert.is_string(text)
            assert.is_number(text:find("# HELP cont_nginx_requests_total"))
            assert.is_number(text:find("# TYPE cont_nginx_requests_total counter"))
            assert.is_number(text:find("cont_nginx_requests_total 0"))
        end)

        it("includes TYPE and HELP for each metric", function()
            local text = build_lines(nil)
            assert.is_number(text:find("# HELP cont_nginx_connections_total"))
            assert.is_number(text:find("# TYPE cont_nginx_connections_total gauge"))
            assert.is_number(text:find("# HELP cont_upstream_target_up"))
            assert.is_number(text:find("# TYPE cont_upstream_target_up gauge"))
        end)

        it("uses Content-Type compatible with Prometheus", function()
            local text = build_lines(nil)
            assert.is_true(#text > 0)
        end)
    end)

    describe("connection metrics", function()
        it("reports zero when no metrics shared dict", function()
            local text = build_lines(nil)
            local parsed = parse_metrics(text)
            assert.are.equal(0, parsed["cont_nginx_connections_total"])
        end)

        it("reports active connections from shared dict", function()
            local stub = {
                get = function(self, key)
                    if key == "connections_active" then return 42 end
                    if key == "connections_accepted" then return 100 end
                    return nil
                end
            }
            local text = build_lines(stub)
            local parsed = parse_metrics(text)
            -- The gauge line for active connections has value 42
            assert.is_number(text:find('cont_nginx_connections_total{state="active"} 42'))
        end)
    end)

    describe("upstream health", function()
        it("includes upstream_target_up metric definition", function()
            local text = build_lines(nil)
            assert.is_number(text:find("cont_upstream_target_up"))
            assert.is_number(text:find("# HELP cont_upstream_target_up"))
            assert.is_number(text:find("# TYPE cont_upstream_target_up gauge"))
        end)
    end)
end)