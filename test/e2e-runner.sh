#!/bin/bash
# Cont E2E Test Runner
# Usage: ./test/e2e-runner.sh [test-file...]
# If no args, runs all tests in test/ directory

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ADMIN_API="${ADMIN_API:-http://localhost:8001}"
PROXY="${PROXY:-http://localhost:8000}"
RESULTS_DIR="${SCRIPT_DIR}/results"
mkdir -p "$RESULTS_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Counters
TOTAL=0
PASSED=0
FAILED=0
SKIPPED=0

log_info()  { echo -e "${BLUE}[INFO]${NC}  $*"; }
log_pass()  { echo -e "${GREEN}[PASS]${NC}  $*"; }
log_fail()  { echo -e "${RED}[FAIL]${NC}  $*"; }
log_skip()  { echo -e "${YELLOW}[SKIP]${NC}  $*"; }

# HTTP helpers
http_get()    { curl -s -o /dev/null -w "%{http_code}" "$1"; }
http_get_body() { curl -s "$1"; }
http_post()   { curl -s -o /dev/null -w "%{http_code}" -X POST "$1" -H "Content-Type: application/json" -d "$2"; }
http_put()    { curl -s -o /dev/null -w "%{http_code}" -X PUT "$1" -H "Content-Type: application/json" -d "$2"; }
http_patch()  { curl -s -o /dev/null -w "%{http_code}" -X PATCH "$1" -H "Content-Type: application/json" -d "$2"; }
http_delete() { curl -s -o /dev/null -w "%{http_code}" -X DELETE "$1"; }

# Assert helpers
assert_eq() {
    local got="$1" expected="$2" msg="$3"
    TOTAL=$((TOTAL+1))
    if [[ "$got" == "$expected" ]]; then
        PASSED=$((PASSED+1))
        log_pass "$msg (got $got)"
        return 0
    else
        FAILED=$((FAILED+1))
        log_fail "$msg — expected $expected, got $got"
        return 1
    fi
}

assert_match() {
    local got="$1" pattern="$2" msg="$3"
    TOTAL=$((TOTAL+1))
    if [[ "$got" =~ $pattern ]]; then
        PASSED=$((PASSED+1))
        log_pass "$msg"
        return 0
    else
        FAILED=$((FAILED+1))
        log_fail "$msg — '$got' does not match '$pattern'"
        return 1
    fi
}

assert_contains() {
    local haystack="$1" needle="$2" msg="$3"
    TOTAL=$((TOTAL+1))
    if [[ "$haystack" == *"$needle"* ]]; then
        PASSED=$((PASSED+1))
        log_pass "$msg"
        return 0
    else
        FAILED=$((FAILED+1))
        log_fail "$msg — '$haystack' does not contain '$needle'"
        return 1
    fi
}

skip_test() {
    local msg="$1"
    SKIPPED=$((SKIPPED+1))
    log_skip "$msg"
}

# Admin JWT token (from docker-compose env or default test secret)
get_admin_token() {
    local secret="${JWT_SECRET:-cont-dev-jwt-secret-change-in-prod}"
    # Use python3 to generate a valid JWT for testadmin
    python3 - <<PYEOF
import jwt, time, sys, warnings
warnings.filterwarnings("ignore")
secret = "$secret"
claims = {"sub": "00000000-0000-0000-0000-000000000001", "username": "testadmin", "role": "admin", "exp": int(time.time()) + 86400}
print(jwt.encode(claims, secret, algorithm="HS256"))
PYEOF
}

get_editor_token() {
    local secret="${JWT_SECRET:-cont-dev-jwt-secret-change-in-prod}"
    python3 - <<PYEOF
import jwt, time, warnings
warnings.filterwarnings("ignore")
secret = "$secret"
claims = {"sub": "00000000-0000-0000-0000-000000000002", "username": "testeditor", "role": "editor", "exp": int(time.time()) + 86400}
print(jwt.encode(claims, secret, algorithm="HS256"))
PYEOF
}

# Check if admin-api is reachable
check_admin_api() {
    local code=$(http_get "$ADMIN_API/status")
    if [[ "$code" != "200" ]]; then
        echo "ERROR: admin-api not reachable at $ADMIN_API (got $code)"
        exit 1
    fi
}

# Print summary
print_summary() {
    echo ""
    echo "========================================"
    echo -e "Results: ${GREEN}$PASSED passed${NC}, ${RED}$FAILED failed${NC}, ${YELLOW}$SKIPPED skipped${NC}, ${BLUE}$TOTAL total${NC}"
    echo "========================================"
    if [[ $FAILED -gt 0 ]]; then
        exit 1
    fi
}

# Main
main() {
    echo ""
    echo "========================================"
    echo "  Cont E2E Test Runner"
    echo "  ADMIN_API: $ADMIN_API"
    echo "  PROXY:     $PROXY"
    echo "========================================"
    echo ""

    check_admin_api

    if [[ $# -gt 0 ]]; then
        # Run specified test files
        for f in "$@"; do
            if [[ -f "$f" ]]; then
                log_info "Running $f"
                source "$f"
            else
                log_fail "Test file not found: $f"
            fi
        done
    else
        # Run all test scripts in test/ directory
        for f in "$SCRIPT_DIR"/e2e-*.sh; do
            if [[ -f "$f" && "$f" != "$SCRIPT_DIR/e2e-runner.sh" ]]; then
                log_info "Running $f"
                source "$f"
            fi
        done
    fi

    print_summary
}

main "$@"
