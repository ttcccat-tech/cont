-- cont.body_filter
-- Response body modification — stream-based body_filter()

local cont = require("cont.init")

local plugins = cont.plugins or {}

local function run_plugin_body_filter(plugin, eof)
    local plugin_name = plugin.name
    local ok, mod = pcall(require, "cont.plugins." .. plugin_name .. ".handler")
    if not ok or not mod then return end
    local handler = mod.new()
    if handler.body_filter then
        pcall(handler.body_filter, handler, eof)
    end
end

for _, plugin in ipairs(plugins) do
    run_plugin_body_filter(plugin, ngx.arg[2])
end
