#!/bin/bash
# Cont E2E Test: Services CRUD
# Phase 1 of E2E Test Framework

ADMIN_API="${ADMIN_API:-http://localhost:8001}"
ADMIN_TOKEN=$(get_admin_token)

SVC_NAME="e2e-test-svc-$(date +%s)"
SVC_HOST="e2e.example.com"
SVC_PORT=8080
SVC_PATH="/api"
SVC_PROTOCOL="http"

CREATED_ID=""

test_services_create() {
    log_info "=== Services Create ==="
    local payload="{\"name\":\"$SVC_NAME\",\"host\":\"$SVC_HOST\",\"port\":$SVC_PORT,\"path\":\"$SVC_PATH\",\"protocol\":\"$SVC_PROTOCOL\"}"
    local code
    code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$ADMIN_API/services" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $ADMIN_TOKEN" \
        -d "$payload")
    assert_eq "$code" "201" "POST /services → 201 Created"
}

test_services_read() {
    log_info "=== Services Read ==="
    # First create to get ID
    local payload="{\"name\":\"${SVC_NAME}-read\",\"host\":\"$SVC_HOST\",\"port\":$SVC_PORT,\"path\":\"/api\",\"protocol\":\"$SVC_PROTOCOL\"}"
    local resp
    resp=$(curl -s -X POST "$ADMIN_API/services" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $ADMIN_TOKEN" \
        -d "$payload")
    CREATED_ID=$(echo "$resp" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('id',''))" 2>/dev/null || echo "")

    if [[ -z "$CREATED_ID" ]]; then
        log_fail "Services Read — could not create service to read"
        return
    fi

    local code
    code=$(curl -s -o /dev/null -w "%{http_code}" -X GET "$ADMIN_API/services/$CREATED_ID" \
        -H "Authorization: Bearer $ADMIN_TOKEN")
    assert_eq "$code" "200" "GET /services/:id → 200 OK"

    local body
    body=$(curl -s -X GET "$ADMIN_API/services/$CREATED_ID" \
        -H "Authorization: Bearer $ADMIN_TOKEN")
    assert_contains "$body" "$SVC_HOST" "GET /services/:id contains host"
}

test_services_list() {
    log_info "=== Services List ==="
    local code
    code=$(curl -s -o /dev/null -w "%{http_code}" -X GET "$ADMIN_API/services" \
        -H "Authorization: Bearer $ADMIN_TOKEN")
    assert_eq "$code" "200" "GET /services → 200 OK"

    local body
    body=$(curl -s -X GET "$ADMIN_API/services" \
        -H "Authorization: Bearer $ADMIN_TOKEN")
    assert_contains "$body" "[" "GET /services returns JSON array"
}

test_services_update() {
    log_info "=== Services Update ==="
    # Create first
    local payload="{\"name\":\"${SVC_NAME}-update\",\"host\":\"$SVC_HOST\",\"port\":$SVC_PORT,\"path\":\"/api\",\"protocol\":\"$SVC_PROTOCOL\"}"
    local resp
    resp=$(curl -s -X POST "$ADMIN_API/services" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $ADMIN_TOKEN" \
        -d "$payload")
    local upd_id
    upd_id=$(echo "$resp" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('id',''))" 2>/dev/null || echo "")

    if [[ -z "$upd_id" ]]; then
        log_fail "Services Update — could not create service to update"
        return
    fi

    local updated_payload="{\"name\":\"${SVC_NAME}-updated\",\"host\":\"updated.example.com\",\"port\":9090}"
    local code
    code=$(curl -s -o /dev/null -w "%{http_code}" -X PUT "$ADMIN_API/services/$upd_id" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $ADMIN_TOKEN" \
        -d "$updated_payload")
    assert_eq "$code" "200" "PUT /services/:id → 200 OK"

    # Verify update
    local body
    body=$(curl -s -X GET "$ADMIN_API/services/$upd_id" \
        -H "Authorization: Bearer $ADMIN_TOKEN")
    assert_contains "$body" "updated.example.com" "PUT /services/:id reflects update"

    # Cleanup
    curl -s -o /dev/null -X DELETE "$ADMIN_API/services/$upd_id" -H "Authorization: Bearer $ADMIN_TOKEN"
}

test_services_patch() {
    log_info "=== Services Patch ==="
    # Create first
    local payload="{\"name\":\"${SVC_NAME}-patch\",\"host\":\"$SVC_HOST\",\"port\":$SVC_PORT,\"path\":\"/api\",\"protocol\":\"$SVC_PROTOCOL\"}"
    local resp
    resp=$(curl -s -X POST "$ADMIN_API/services" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $ADMIN_TOKEN" \
        -d "$payload")
    local patch_id
    patch_id=$(echo "$resp" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('id',''))" 2>/dev/null || echo "")

    if [[ -z "$patch_id" ]]; then
        log_fail "Services Patch — could not create service to patch"
        return
    fi

    local patch_payload="{\"port\":9999}"
    local code
    code=$(curl -s -o /dev/null -w "%{http_code}" -X PATCH "$ADMIN_API/services/$patch_id" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $ADMIN_TOKEN" \
        -d "$patch_payload")
    assert_eq "$code" "200" "PATCH /services/:id → 200 OK"

    # Cleanup
    curl -s -o /dev/null -X DELETE "$ADMIN_API/services/$patch_id" -H "Authorization: Bearer $ADMIN_TOKEN"
}

test_services_delete() {
    log_info "=== Services Delete ==="
    # Create first
    local payload="{\"name\":\"${SVC_NAME}-delete\",\"host\":\"$SVC_HOST\",\"port\":$SVC_PORT,\"path\":\"/api\",\"protocol\":\"$SVC_PROTOCOL\"}"
    local resp
    resp=$(curl -s -X POST "$ADMIN_API/services" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $ADMIN_TOKEN" \
        -d "$payload")
    local del_id
    del_id=$(echo "$resp" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('id',''))" 2>/dev/null || echo "")

    if [[ -z "$del_id" ]]; then
        log_fail "Services Delete — could not create service to delete"
        return
    fi

    local code
    code=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$ADMIN_API/services/$del_id" \
        -H "Authorization: Bearer $ADMIN_TOKEN")
    assert_eq "$code" "204" "DELETE /services/:id → 204 No Content"

    # Verify deleted
    code=$(curl -s -o /dev/null -w "%{http_code}" -X GET "$ADMIN_API/services/$del_id" \
        -H "Authorization: Bearer $ADMIN_TOKEN")
    assert_eq "$code" "404" "GET /services/:id after delete → 404"
}

test_services_unauthorized() {
    log_info "=== Services Unauthorized ==="
    local code
    code=$(curl -s -o /dev/null -w "%{http_code}" -X GET "$ADMIN_API/services")
    assert_eq "$code" "401" "GET /services without token → 401"

    code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$ADMIN_API/services" \
        -H "Content-Type: application/json" \
        -d '{"name":"unauth-test","host":"x.com","port":80}')
    assert_eq "$code" "401" "POST /services without token → 401"
}

test_services_validation() {
    log_info "=== Services Validation ==="
    # Missing required fields
    local code
    code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$ADMIN_API/services" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $ADMIN_TOKEN" \
        -d '{"name":""}')
    assert_eq "$code" "400" "POST /services with empty name → 400"

    # Invalid protocol
    code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$ADMIN_API/services" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $ADMIN_TOKEN" \
        -d '{"name":"test-svc","host":"x.com","port":80,"protocol":"ftp"}')
    assert_eq "$code" "400" "POST /services with invalid protocol → 400"
}

# Run all tests
log_info "Starting Services CRUD E2E tests"
test_services_create
test_services_read
test_services_list
test_services_update
test_services_patch
test_services_delete
test_services_unauthorized
test_services_validation
log_info "Services CRUD E2E tests complete"
