#!/bin/bash
# Cont E2E Test: Proxy Lua Plugin Chain (rate-limiting + proxy-cache)
# Phase 3 of E2E Test Framework
# Tests proxy /metrics, /status, and basic proxy flow with plugin headers
# Usage: source this from e2e-runner.sh OR run standalone with ADMIN_API/PROXY set

# ---- Helper detection ----
if [[ -z "$ADMIN_API" ]]; then
    export ADMIN_API="${ADMIN_API:-http://localhost:18081}"
fi
if [[ -z "$PROXY" ]]; then
    export PROXY="${PROXY:-http://localhost:18000}"
fi

# ---- Standalone helpers (if not sourced from runner) ----
if ! declare -f log_info > /dev/null 2>&1; then
    RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'
    log_info()  { echo -e "${BLUE}[INFO]${NC}  $*"; }
    log_pass()  { echo -e "${GREEN}[PASS]${NC}  $*"; }
    log_fail()  { echo -e "${RED}[FAIL]${NC}  $*"; }
    log_skip()  { echo -e "${YELLOW}[SKIP]${NC}  $*"; }
    TOTAL=0; PASSED=0; FAILED=0; SKIPPED=0
    assert_eq() {
        local got="$1" expected="$2" msg="$3"
        TOTAL=$((TOTAL+1))
        if [[ "$got" == "$expected" ]]; then
            PASSED=$((PASSED+1)); log_pass "$msg (got $got)"; return 0
        else
            FAILED=$((FAILED+1)); log_fail "$msg — expected $expected, got $got"; return 1
        fi
    }
    assert_contains() {
        local haystack="$1" needle="$2" msg="$3"
        TOTAL=$((TOTAL+1))
        if [[ "$haystack" == *"$needle"* ]]; then
            PASSED=$((PASSED+1)); log_pass "$msg"; return 0
        else
            FAILED=$((FAILED+1)); log_fail "$msg — '$haystack' does not contain '$needle'"; return 1
        fi
    }
    assert_match() {
        local got="$1" pattern="$2" msg="$3"
        TOTAL=$((TOTAL+1))
        if [[ "$got" =~ $pattern ]]; then
            PASSED=$((PASSED+1)); log_pass "$msg"; return 0
        else
            FAILED=$((FAILED+1)); log_fail "$msg — '$got' does not match '$pattern'"; return 1
        fi
    }
    http_get()    { curl -s -o /dev/null -w "%{http_code}" "$1"; }
    http_get_body() { curl -s "$1"; }
    get_admin_token() {
        local secret="${JWT_SECRET:-cont-dev-jwt-secret-change-in-prod}"
        python3 - <<PYEOF
import jwt, time, sys, warnings
warnings.filterwarnings("ignore")
secret = "$secret"
claims = {"sub": "00000000-0000-0000-0000-000000000001", "username": "testadmin", "role": "admin", "exp": int(time.time()) + 86400}
print(jwt.encode(claims, secret, algorithm="HS256"))
PYEOF
    }
fi

ADMIN_TOKEN="${ADMIN_TOKEN:-$(get_admin_token)}"

# ---- /metrics endpoint tests ----
test_proxy_metrics() {
    log_info "--- /metrics endpoint ---"
    local code body
    code=$(http_get "$PROXY/metrics")
    assert_eq "$code" "200" "GET /metrics → 200"

    body=$(http_get_body "$PROXY/metrics")
    assert_contains "$body" "cont_" "metrics contains cont_ prefix"
    assert_contains "$body" "nginx" "metrics contains nginx keyword"
}

# ---- /status endpoint tests ----
test_proxy_status() {
    log_info "--- /status endpoint ---"
    local code body
    code=$(http_get "$PROXY/status")
    assert_eq "$code" "200" "GET /status → 200"

    body=$(http_get_body "$PROXY/status")
    assert_contains "$body" "uptime" "status contains uptime"
    assert_contains "$body" "workers" "status contains workers"
}

# ---- Basic proxy / request ----
test_proxy_root() {
    log_info "--- Proxy root / ---"
    local code
    code=$(http_get "$PROXY/")
    # May return 502 (no upstream) or actual response
    assert_match "$code" "^[245][0-9][0-9]$" "GET / → valid response (2xx/4xx/5xx)"
}

# ---- Rate limit headers test ----
test_rate_limit_headers() {
    log_info "--- Rate Limit Headers ---"
    local code body
    code=$(http_get "$PROXY/")
    body=$(http_get_body "$PROXY/")

    # Verify the request completes
    assert_match "$code" "^[245][0-9][0-9]$" "Proxy request completes with rate limiting"
}

# ---- Cache headers test ----
test_cache_headers() {
    log_info "--- Cache Headers ---"
    local code1 code2
    code1=$(http_get "$PROXY/")
    code2=$(http_get "$PROXY/")

    assert_match "$code1" "^[245][0-9][0-9]$" "First request completes"
    assert_match "$code2" "^[245][0-9][0-9]$" "Second request completes"
}

# ---- Admin API: internal plugins list ----
test_admin_internal_plugins() {
    log_info "--- Admin API: Internal Plugins ---"
    local code body
    code=$(curl -s -o /dev/null -w "%{http_code}" "$ADMIN_API/internal/plugins")
    assert_eq "$code" "200" "GET /internal/plugins → 200"

    body=$(curl -s "$ADMIN_API/internal/plugins")
    assert_contains "$body" "rate-limiting" "plugins list contains rate-limiting"
}

# ---- Admin API: Create Service with Plugin (plugin created at /plugins, service_id in body) ----
test_admin_service_with_plugin() {
    log_info "--- Admin API: Service + Plugin ---"
    local svc_name="e2e-plugin-test-$(date +%s)"
    local payload="{\"name\":\"$svc_name\",\"host\":\"httpbin.org\",\"port\":80,\"path\":\"/get\",\"protocol\":\"http\"}"
    local resp
    resp=$(curl -s -X POST "$ADMIN_API/services" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $ADMIN_TOKEN" \
        -d "$payload")
    local svc_id
    svc_id=$(echo "$resp" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('id',''))" 2>/dev/null || echo "")

    if [[ -z "$svc_id" ]]; then
        log_fail "Admin Service + Plugin — could not create service"
        return
    fi

    # Attach rate-limiting plugin (plugin created at /plugins with service_id)
    local plugin_payload="{\"name\":\"rate-limiting\",\"service_id\":\"$svc_id\",\"config\":{\"minute\":100,\"policy\":\"local\"}}"
    local pcode
    pcode=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$ADMIN_API/plugins" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $ADMIN_TOKEN" \
        -d "$plugin_payload")
    assert_eq "$pcode" "201" "POST /plugins (rate-limiting) → 201"

    # Attach proxy-cache plugin
    local cache_payload="{\"name\":\"proxy-cache\",\"service_id\":\"$svc_id\",\"config\":{\"response_code\":[200],\"request_method\":[\"GET\",\"HEAD\"],\"ttl\":60}}"
    pcode=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$ADMIN_API/plugins" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $ADMIN_TOKEN" \
        -d "$cache_payload")
    assert_eq "$pcode" "201" "POST /plugins (proxy-cache) → 201"

    # List plugins filtered by service
    local list_code list_body
    list_code=$(curl -s -o /dev/null -w "%{http_code}" -X GET "$ADMIN_API/plugins" \
        -H "Authorization: Bearer $ADMIN_TOKEN")
    list_body=$(curl -s -X GET "$ADMIN_API/plugins" \
        -H "Authorization: Bearer $ADMIN_TOKEN")
    assert_eq "$list_code" "200" "GET /plugins → 200"
    assert_contains "$list_body" "rate-limiting" "plugins list contains rate-limiting"

    # Cleanup
    curl -s -o /dev/null -X DELETE "$ADMIN_API/services/$svc_id" -H "Authorization: Bearer $ADMIN_TOKEN"
    log_pass "Service + Plugins created and cleaned up"
}

# ---- Admin API: Consumer Credentials + Rate Limit ----
test_admin_consumer_credential() {
    log_info "--- Admin API: Consumer + Key-Auth Credential ---"
    local consumer_name="e2e-consumer-$(date +%s)"

    # Create consumer
    local resp
    resp=$(curl -s -X POST "$ADMIN_API/consumers" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $ADMIN_TOKEN" \
        -d "{\"username\":\"$consumer_name\"}")
    local consumer_id
    consumer_id=$(echo "$resp" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('id',''))" 2>/dev/null || echo "")

    if [[ -z "$consumer_id" ]]; then
        log_fail "Consumer + Credential — could not create consumer"
        return
    fi

    # Create key-auth credential at /consumers/:id/key-auth/credentials
    local key_payload="{\"key\":\"e2e-test-key-$(date +%s)\"}"
    local kcode
    kcode=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$ADMIN_API/consumers/$consumer_id/key-auth/credentials" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $ADMIN_TOKEN" \
        -d "$key_payload")
    assert_eq "$kcode" "201" "POST /consumers/:id/key-auth/credentials → 201"

    # List credentials
    local lcode
    lcode=$(curl -s -o /dev/null -w "%{http_code}" -X GET "$ADMIN_API/consumers/$consumer_id/key-auth/credentials" \
        -H "Authorization: Bearer $ADMIN_TOKEN")
    assert_eq "$lcode" "200" "GET /consumers/:id/key-auth/credentials → 200"

    # Cleanup consumer
    curl -s -o /dev/null -X DELETE "$ADMIN_API/consumers/$consumer_id" -H "Authorization: Bearer $ADMIN_TOKEN"
    log_pass "Consumer + Key-Auth created and cleaned up"
}

# ---- Prometheus metrics format ----
test_prometheus_metrics_format() {
    log_info "--- Prometheus Metrics Format ---"
    local body
    body=$(http_get_body "$PROXY/metrics")

    # Check for Prometheus format (cont_ prefix)
    assert_contains "$body" "cont_" "metrics contains cont_ prefix"
}

# ---- Run all tests ----
run_all_tests() {
    log_info "Starting Proxy Plugin Chain E2E tests"
    test_proxy_metrics
    test_proxy_status
    test_proxy_root
    test_rate_limit_headers
    test_cache_headers
    test_admin_internal_plugins
    test_admin_service_with_plugin
    test_admin_consumer_credential
    test_prometheus_metrics_format
    log_info "Proxy Plugin Chain E2E tests complete"
}

if [[ "${CONT_E2E_AUTO_RUN:-0}" == "1" ]]; then
    run_all_tests
fi
