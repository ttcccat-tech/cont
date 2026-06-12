-- cont.rewrite
-- URL normalization and method handling

local cont = require("init")

-- Set upstream variables (used by proxy_pass)
ngx.var.cont_upstream = "http://127.0.0.1:80"
ngx.var.cont_upstream_host = "_"

-- Normalize trailing slash (Kong behavior)
local path = ngx.var.uri
if path == "/" then
    -- keep as-is
elseif string.sub(path, -1) == "/" and string.len(path) > 1 then
    -- Kong strips trailing slash from path before matching
    -- But we handle strip_path per-route in access phase
end

-- Handle OPTIONS preflight (CORS preflight — handled in access plugin)
-- Handle WebSocket upgrade
if ngx.var.upstream_http_upgrade == "websocket" then
    ngx.var.cont_upstream = "http://127.0.0.1:80"  -- upstream target set in access
end

return cont
