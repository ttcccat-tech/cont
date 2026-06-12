#!/bin/bash
# Cont E2E Test: Billing/Plan/Stripe Flow
# Phase 4 of E2E Test Framework

ADMIN_API="${ADMIN_API:-http://localhost:8001}"
ADMIN_TOKEN=$(get_admin_token)
EDITOR_TOKEN=$(get_editor_token)

# ── Billing Plans ────────────────────────────────────────────────────────────

test_billing_plans_list() {
    log_info "=== Billing Plans List ==="
    local code
    code=$(curl -s -o /dev/null -w "%{http_code}" "$ADMIN_API/billing/plans")
    assert_eq "$code" "200" "GET /billing/plans → 200 OK (no auth required)"

    local body
    body=$(curl -s "$ADMIN_API/billing/plans")
    assert_contains "$body" "[" "GET /billing/plans returns JSON array"

    # Should contain at least free/pro/enterprise plans
    assert_contains "$body" "free" "GET /billing/plans contains free plan"
    assert_contains "$body" "pro" "GET /billing/plans contains pro plan"
    assert_contains "$body" "enterprise" "GET /billing/plans contains enterprise plan"
}

# ── Billing Subscription ─────────────────────────────────────────────────────

test_billing_subscription_authenticated() {
    log_info "=== Billing Subscription (authenticated) ==="
    local code
    code=$(curl -s -o /dev/null -w "%{http_code}" "$ADMIN_API/billing/subscription" \
        -H "Authorization: Bearer $ADMIN_TOKEN")
    assert_eq "$code" "200" "GET /billing/subscription → 200 with auth"

    local body
    body=$(curl -s "$ADMIN_API/billing/subscription" \
        -H "Authorization: Bearer $ADMIN_TOKEN")
    assert_contains "$body" "plan_name" "GET /billing/subscription contains plan_name"
    assert_contains "$body" "status" "GET /billing/subscription contains status"
}

test_billing_subscription_unauthenticated() {
    log_info "=== Billing Subscription (unauthenticated) ==="
    local code
    code=$(curl -s -o /dev/null -w "%{http_code}" "$ADMIN_API/billing/subscription")
    # Unauthenticated request to /billing/subscription — should be 401 (auth required)
    assert_eq "$code" "401" "GET /billing/subscription without token → 401"
}

test_billing_subscription_editor_role() {
    log_info "=== Billing Subscription (editor role) ==="
    local code
    code=$(curl -s -o /dev/null -w "%{http_code}" "$ADMIN_API/billing/subscription" \
        -H "Authorization: Bearer $EDITOR_TOKEN")
    # Editor should also be able to read their subscription
    assert_eq "$code" "200" "GET /billing/subscription with editor token → 200"
}

# ── Billing Checkout ─────────────────────────────────────────────────────────

test_billing_checkout_requires_auth() {
    log_info "=== Billing Checkout (auth required) ==="
    local code
    code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$ADMIN_API/billing/checkout" \
        -H "Content-Type: application/json" \
        -d '{"plan_name":"pro","billing_cycle":"monthly"}')
    assert_eq "$code" "401" "POST /billing/checkout without token → 401"
}

test_billing_checkout_requires_stripe() {
    log_info "=== Billing Checkout (requires Stripe config) ==="
    # Even with auth, without STRIPE_SECRET_KEY → 400
    local code
    code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$ADMIN_API/billing/checkout" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $ADMIN_TOKEN" \
        -d '{"plan_name":"pro","billing_cycle":"monthly"}')
    # Without Stripe: 400 "Stripe is not configured"
    assert_eq "$code" "400" "POST /billing/checkout without Stripe → 400"
}

test_billing_checkout_validation() {
    log_info "=== Billing Checkout (validation) ==="
    # Invalid plan_name
    local code
    code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$ADMIN_API/billing/checkout" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $ADMIN_TOKEN" \
        -d '{"plan_name":"invalid_plan","billing_cycle":"monthly"}')
    assert_eq "$code" "400" "POST /billing/checkout with invalid plan_name → 400"

    # Invalid billing_cycle
    code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$ADMIN_API/billing/checkout" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $ADMIN_TOKEN" \
        -d '{"plan_name":"pro","billing_cycle":"weekly"}')
    assert_eq "$code" "400" "POST /billing/checkout with invalid billing_cycle → 400"
}

# ── Billing Portal ────────────────────────────────────────────────────────────

test_billing_portal_requires_auth() {
    log_info "=== Billing Portal (auth required) ==="
    local code
    code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$ADMIN_API/billing/portal" \
        -H "Content-Type: application/json")
    assert_eq "$code" "401" "POST /billing/portal without token → 401"
}

test_billing_portal_no_billing_account() {
    log_info "=== Billing Portal (no billing account) ==="
    # Even with auth, if no Stripe customer yet → 404
    local code
    code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$ADMIN_API/billing/portal" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $ADMIN_TOKEN")
    # Without Stripe customer: 404 "no billing account found"
    assert_eq "$code" "404" "POST /billing/portal without subscription → 404"
}

# ── Stripe Webhook ────────────────────────────────────────────────────────────

test_billing_webhook_no_stripe() {
    log_info "=== Stripe Webhook (no Stripe config) ==="
    # Without STRIPE_SECRET_KEY, webhook returns 400
    local code
    code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$ADMIN_API/webhooks/stripe" \
        -H "Content-Type: application/json" \
        -d '{"type":"checkout.session.completed","data":{}}')
    assert_eq "$code" "400" "POST /webhooks/stripe without Stripe → 400"
}

test_billing_webhook_invalid_payload() {
    log_info "=== Stripe Webhook (invalid JSON) ==="
    local code
    code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$ADMIN_API/webhooks/stripe" \
        -H "Content-Type: application/json" \
        -d 'not-valid-json')
    assert_eq "$code" "400" "POST /webhooks/stripe with invalid JSON → 400"
}

# ── List Subscriptions (admin only) ─────────────────────────────────────────

test_billing_list_subscriptions_authenticated() {
    log_info "=== List Subscriptions (admin) ==="
    local code
    code=$(curl -s -o /dev/null -w "%{http_code}" "$ADMIN_API/billing/subscriptions" \
        -H "Authorization: Bearer $ADMIN_TOKEN")
    assert_eq "$code" "200" "GET /billing/subscriptions → 200 (admin)"
}

test_billing_list_subscriptions_unauthenticated() {
    log_info "=== List Subscriptions (unauthenticated) ==="
    local code
    code=$(curl -s -o /dev/null -w "%{http_code}" "$ADMIN_API/billing/subscriptions")
    assert_eq "$code" "401" "GET /billing/subscriptions without token → 401"
}

# Run all tests
log_info "Starting Billing E2E tests"
test_billing_plans_list
test_billing_subscription_authenticated
test_billing_subscription_unauthenticated
test_billing_subscription_editor_role
test_billing_checkout_requires_auth
test_billing_checkout_requires_stripe
test_billing_checkout_validation
test_billing_portal_requires_auth
test_billing_portal_no_billing_account
test_billing_webhook_no_stripe
test_billing_webhook_invalid_payload
test_billing_list_subscriptions_authenticated
test_billing_list_subscriptions_unauthenticated
log_info "Billing E2E tests complete"
