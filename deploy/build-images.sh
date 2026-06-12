#!/bin/bash
# Cont Gateway — Image Build Script
# Usage: ./build-images.sh [registry] [tag]
# Example: ./build-images.sh ghcr.io/ttcccat 1.0.0

set -e

REGISTRY="${1:-cont}"
TAG="${2:-latest}"
APP_DIR="$(cd "$(dirname "$0")/.." && pwd)"

echo "=== Cont Gateway Image Build ==="
echo "Registry: $REGISTRY"
echo "Tag: $TAG"
echo ""

# Build admin-api
echo "[1/3] Building cont/admin-api:${TAG} ..."
docker build \
  --build-arg BUILDKIT_INLINE_CACHE=1 \
  -t "${REGISTRY}/admin-api:${TAG}" \
  -t "${REGISTRY}/admin-api:latest" \
  "${APP_DIR}/admin-api"
echo ""

# Build proxy
echo "[2/3] Building cont/proxy:${TAG} ..."
docker build \
  --pull \
  --no-cache \
  -t "${REGISTRY}/proxy:${TAG}" \
  -t "${REGISTRY}/proxy:latest" \
  "${APP_DIR}/proxy"
echo ""

# Build frontend
echo "[3/3] Building cont/frontend:${TAG} ..."
docker build \
  --build-arg VITE_KONG_BASE=/api \
  --build-arg VITE_API_BASE=/api \
  -t "${REGISTRY}/frontend:${TAG}" \
  -t "${REGISTRY}/frontend:latest" \
  "${APP_DIR}/frontend"
echo ""

echo "=== All images built ==="
docker images | grep "${REGISTRY}"

# Push if registry is not 'cont'
if [[ "$REGISTRY" != "cont" ]]; then
  echo ""
  echo "Pushing to registry..."
  docker push -a "${REGISTRY}/admin-api"
  docker push -a "${REGISTRY}/proxy"
  docker push -a "${REGISTRY}/frontend"
  echo "Done."
fi
