-- cont.init
-- Application initialization phase (runs once at startup via init_by_lua)
-- NOTE: ngx.location.capture cannot be used here (no request context in init_by_lua)
-- Config loading is done lazily on first request

_G.cont = {
    services = {},
    routes   = {},
    upstreams = {},
    plugins  = {},
    targets  = {},
    workers  = {},
    start_time = ngx.time(),
    config_loaded = false,
}

return _G.cont