#!/usr/bin/env bash
# deploy.sh — One-shot Cont deployment to current kubectl context
# Usage: REGISTRY=docker.io/myuser JWT_SECRET=$(openssl rand -hex 32) ./deploy.sh [apply|delete]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
K8S_DIR="$SCRIPT_DIR/k8s"
ACTION="${1:-apply}"

# ── Color output ───────────────────────────────────────────────────────────────
RED='\033[0;31m'; YEL='\033[0;33m'; GRN='\033[0;32m'; NC='\033[0m'
info()  { echo -e "${GRN}[INFO]${NC} $*"; }
warn()  { echo -e "${YEL}[WARN]${NC} $*"; }
err()   { echo -e "${RED}[ERR]${NC} $*" >&2; exit 1; }

# ── Pre-flight checks ─────────────────────────────────────────────────────────
info "Pre-flight checks..."

if ! command -v kubectl &>/dev/null; then
  err "kubectl not found. Cannot deploy to Kubernetes."
fi

if ! kubectl cluster-info &>/dev/null; then
  err "Cannot reach Kubernetes cluster. Check your kubeconfig."
fi

REGISTRY="${REGISTRY:-}"
if [[ -z "$REGISTRY" ]]; then
  warn "REGISTRY not set — images must already be present on cluster nodes."
  warn "Set REGISTRY (e.g. REGISTRY=docker.io/myuser) before building."
fi

JWT_SECRET="${JWT_SECRET:-}"
if [[ -z "$JWT_SECRET" ]]; then
  warn "JWT_SECRET not set — generating ephemeral secret (OK for dev)."
  JWT_SECRET="$(openssl rand -hex 32)"
fi

# ── Build images (if REGISTRY set) ────────────────────────────────────────────
if [[ -n "$REGISTRY" ]]; then
  info "Building images with REGISTRY=$REGISTRY..."
  VERSION="${VERSION:-$(git rev-parse --short HEAD)}"

  info "Building cont-admin-api:$VERSION..."
  docker build -t "$REGISTRY/cont-admin-api:$VERSION" ./admin-api
  docker build -t "$REGISTRY/cont-frontend:$VERSION" ./frontend
  docker build -t "$REGISTRY/cont-proxy:latest" ./proxy

  info "Pushing images..."
  docker push "$REGISTRY/cont-admin-api:$VERSION"
  docker push "$REGISTRY/cont-frontend:$VERSION"
  docker push "$REGISTRY/cont-proxy:latest"

  # Update k8s yamls to use tagged images
  info "Patching image references in k8s/ ..."
  sed -i.bak \
    "s|image: cont-admin-api:latest|image: $REGISTRY/cont-admin-api:$VERSION|g" \
    "$K8S_DIR/admin-api.yaml"
  sed -i \
    "s|image: cont-frontend:latest|image: $REGISTRY/cont-frontend:$VERSION|g" \
    "$K8S_DIR/frontend.yaml"
fi

# ── Patch JWT_SECRET into Secret ──────────────────────────────────────────────
info "Patching JWT_SECRET into cont-secrets..."
python3 - <<'PYEOF'
import sys, subprocess, re

secret_path = "/var/repo/cont/k8s/config.yaml"
with open(secret_path) as f:
    content = f.read()

jwt = sys.argv[1] if len(sys.argv) > 1 else "changeme"
content = re.sub(r'(JWT_SECRET: )"[^"]*"', f'\\1"{jwt}"', content)
with open(secret_path, "w") as f:
    f.write(content)
print("patched")
PYEOF
"$JWT_SECRET"

# ── Apply / Delete ────────────────────────────────────────────────────────────
if [[ "$ACTION" == "delete" ]]; then
  info "Deleting Cont from Kubernetes..."
  kubectl delete -f "$K8S_DIR" --ignore-not-found
  info "Done."
  exit 0
fi

info "Applying Kubernetes manifests..."
kubectl apply -f "$K8S_DIR/namespace.yaml"
kubectl apply -f "$K8S_DIR/config.yaml"
kubectl apply -f "$K8S_DIR/postgres.yaml"
kubectl apply -f "$K8S_DIR/postgres-svc.yaml"
kubectl apply -f "$K8S_DIR/redis.yaml"
kubectl apply -f "$K8S_DIR/redis-svc.yaml"
kubectl apply -f "$K8S_DIR/admin-api.yaml"
kubectl apply -f "$K8S_DIR/frontend.yaml"
kubectl apply -f "$K8S_DIR/proxy.yaml"

info "Waiting for pods..."
kubectl rollout status deployment/cont-admin-api -n cont --timeout=120s || true
kubectl rollout status deployment/cont-frontend -n cont --timeout=60s || true
kubectl rollout status deployment/cont-proxy -n cont --timeout=60s || true

info "Pods:"
kubectl get pods -n cont -o wide

info ""
info "Cont deployed to Kubernetes!"
info "  Admin API:  http://<node>:$(kubectl get svc cont-proxy -n cont -o jsonpath='{.spec.ports[0].nodePort}')"
info "  Frontend:   http://<node>:$(kubectl get svc cont-frontend -n cont -o jsonpath='{.spec.ports[0].nodePort}')"
info ""
info "Teardown: $0 delete"