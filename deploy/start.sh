#!/bin/bash
# Cont Gateway — Quick Start Script
# One-command deployment for fresh servers

set -e

APP_DIR="$(cd "$(dirname "$0")" && pwd)"

# Check Docker
if ! command -v docker &> /dev/null; then
    echo "Error: Docker is not installed."
    exit 1
fi

if ! command -v docker compose &> /dev/null; then
    echo "Error: Docker Compose v2 is not installed (try: apt install docker-compose-v2)"
    exit 1
fi

cd "$APP_DIR"

# Create .env from .env.example if not exists
if [ ! -f .env ]; then
    echo "Creating .env from .env.example..."
    cp .env.example .env
    echo ""
    echo "⚠️  Please edit .env and set JWT_SECRET before starting!"
    echo "   Generate JWT_SECRET with: openssl rand -hex 32"
    exit 1
fi

# Check JWT_SECRET is set
if grep -q "^JWT_SECRET=$" .env || grep -q '^JWT_SECRET=""' .env || grep -q "^JWT_SECRET=.*dev.*change.*prod"; then
    echo "⚠️  JWT_SECRET is not set or is still the default placeholder!"
    echo "   Generate one with: openssl rand -hex 32"
    exit 1
fi

# Create network if not exists
docker network inspect cont_net &>/dev/null || {
    echo "Creating Docker network 'cont_net'..."
    docker network create cont_net
}

# Build images locally (no registry push needed)
echo "Building Cont Gateway images..."
IMAGE_TAG=local docker compose build --pull --no-cache

echo "Starting Cont Gateway..."
docker compose up -d

echo ""
echo "=== Cont Gateway is running ==="
echo ""
echo "Services:"
echo "  Frontend :  http://localhost:18082"
echo "  Proxy    :  http://localhost:18000"
echo "  Admin API:  http://localhost:18081"
echo ""
echo "Default admin credentials:"
echo "  Username: admin"
echo "  Password: admin123  (CHANGE THIS IMMEDIATELY!)"
echo ""
echo "View logs: docker compose logs -f"
echo "Stop:     docker compose down"