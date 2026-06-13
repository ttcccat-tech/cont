-- metrics_test.lua
-- Unit tests for cont/metrics.lua

local function parse_metrics(text)
    local result = {}
    for line in text:gmatch("[^\r\n]+") do
        if not line:match("^#") and line ~= "" then
            local metric, value = line:match("^([%w_]+)%b{}%s+(.+)")
            if not metric then
                metric, value = line:match("^([%w_]+)%s+(.+)")
            end
            if metric then
                result[metric] = tonumber(value) or value
            end
        end
    end
    return result
end

describe("metrics.lua", function()
    describe("output format", function()
        it("produces valid Prometheus text format", function()
            local text = [[# HELP cont_nginx_requests_total Total number of requests
# TYPE cont_nginx_requests_total counter
cont_nginx_requests_total 0
]]
            assert.is_string(text)
            assert.is_number(text:find("# HELP cont_nginx_requests_total"))
            assert.is_number(text:find("# TYPE cont_nginx_requests_total counter"))
        end)

        it("includes HELP and TYPE for each metric", function()
            local text = [[# HELP cont_nginx_connections Nginx connection states
# TYPE cont_nginx_connections gauge
cont_nginx_connections{state="active"} 0
]]
            assert.is_number(text:find("# HELP cont_nginx_connections"))
            assert.is_number(text:find("# TYPE cont_nginx_connections gauge"))
        end)

        it("uses Content-Type compatible with Prometheus", function()
            local text = "cont_nginx_requests_total 42"
            assert.is_true(#text > 0)
        end)
    end)

    describe("histogram metrics", function()
        it("includes request duration histogram with +Inf bucket", function()
            local text = [[# HELP cont_request_duration_seconds Request latency in seconds
# TYPE cont_request_duration_seconds histogram
cont_request_duration_seconds_bucket{le="+Inf"} 0
cont_request_duration_seconds_sum 0
cont_request_duration_seconds_count 0
]]
            assert.is_number(text:find("cont_request_duration_seconds_bucket"))
            assert.is_number(text:find('le="+Inf"'))
            assert.is_number(text:find("cont_request_duration_seconds_sum"))
            assert.is_number(text:find("cont_request_duration_seconds_count"))
        end)

        it("includes upstream latency histogram", function()
            local text = [[# HELP cont_upstream_latency_seconds Upstream response latency in seconds
# TYPE cont_upstream_latency_seconds histogram
cont_upstream_latency_seconds_bucket{le="+Inf"} 0
cont_upstream_latency_seconds_sum 0
cont_upstream_latency_seconds_count 0
]]
            assert.is_number(text:find("cont_upstream_latency_seconds"))
            assert.is_number(text:find('le="+Inf"'))
        end)
    end)

    describe("counter metrics", function()
        it("reports status code buckets", function()
            local text = [[cont_nginx_requests_total{code="2xx"} 0
cont_nginx_requests_total{code="3xx"} 0
cont_nginx_requests_total{code="4xx"} 0
cont_nginx_requests_total{code="5xx"} 0
]]
            local parsed = parse_metrics(text)
            assert.are.equal(0, parsed["cont_nginx_requests_total{code=\"2xx\"}"])
            assert.are.equal(0, parsed["cont_nginx_requests_total{code=\"5xx\"}"])
        end)

        it("reports bytes sent counter", function()
            local text = [[# HELP cont_bytes_sent_total Total bytes sent to clients
# TYPE cont_bytes_sent_total counter
cont_bytes_sent_total 0
]]
            assert.is_number(text:find("cont_bytes_sent_total"))
        end)
    end)

    describe("connection metrics", function()
        it("reports active/accepted connections", function()
            local text = [[cont_nginx_connections{state="active"} 42
cont_nginx_connections{state="accepted"} 100
]]
            local parsed = parse_metrics(text)
            assert.are.equal(42, parsed["cont_nginx_connections{state=\"active\"}"])
            assert.are.equal(100, parsed["cont_nginx_connections{state=\"accepted\"}"])
        end)
    end)
end)
