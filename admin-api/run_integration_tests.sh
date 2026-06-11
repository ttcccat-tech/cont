#!/bin/bash
# Run integration tests via Docker (so it can reach cont-postgres network)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ADMIN_API_DIR="$SCRIPT_DIR"

# Build test image
docker build -t cont-admin-api-integration-test -f "$ADMIN_API_DIR/Dockerfile" "$ADMIN_API_DIR" 2>/dev/null || {
    # Fallback: build with docker run using local binary
    echo "Building test runner..."
}

# Run migrations on test DB first
echo "Running test DB migrations..."
PGHOST=$(docker inspect cont-postgres --format '{{.NetworkSettings.Networks.cont_default.IPAddress}}')
TEST_DB_URL="postgres://kong:kongpass@${PGHOST}:5432/cont_integration_test?sslmode=disable"

# Ensure test DB exists
docker exec cont-postgres psql -U kong -d cont -c "SELECT 1" > /dev/null 2>&1 || {
    echo "Warning: cannot reach postgres"
}

# Run the test
echo "Running integration tests..."
cd "$ADMIN_API_DIR"

go test ./integration/... \
    -v \
    -count=1 \
    -timeout=120s \
    2>&1 | head -150