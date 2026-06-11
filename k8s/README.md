# k8s/overlays/README.md
# Cont Kustomize Overlays

## Directory Structure

```
k8s/
├── base/
│   ├── kustomization.yaml      # Base resources (namespace, postgres, redis, app deployments)
│   ├── namespace.yaml          # cont namespace
│   ├── config.yaml              # ConfigMap + Secret (base values)
│   ├── postgres.yaml            # Postgres Deployment + PVC
│   ├── postgres-svc.yaml       # Postgres ClusterIP service
│   ├── redis.yaml               # Redis Deployment
│   ├── redis-svc.yaml          # Redis ClusterIP service
│   ├── admin-api.yaml           # Admin API Deployment + Service
│   ├── frontend.yaml            # Frontend Deployment + Service
│   └── proxy.yaml               # Cont Proxy (OpenResty) Deployment + LoadBalancer
│
├── overlays/
│   ├── dev/                     # Development overlay (minikube/kind)
│   │   └── kustomization.yaml
│   └── prod/                    # Production overlay (HA, resource limits)
│       └── kustomization.yaml
│
└── README.md                    # This file
```

## Usage

### Prerequisites

```bash
# Install kustomize
# macOS
brew install kustomize
# Linux
curl -s "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh" | bash

# Or use kubectl 1.14+ (kustomize built-in)
kubectl kustomize --help
```

### Deploy Development

```bash
# Option 1: Use deploy.sh (recommended)
REGISTRY=docker.io/myuser JWT_SECRET=$(openssl rand -hex 32) ./deploy.sh apply

# Option 2: Direct kustomize
kubectl apply -k k8s/overlays/dev

# Or build and apply
kubectl kustomize k8s/overlays/dev | kubectl apply -f -

# View what would be applied
kubectl kustomize k8s/overlays/dev
```

### Deploy Production

```bash
# Set required environment variables
export CONT_ADMIN_API_IMAGE=docker.io/myuser/cont-admin-api
export CONT_FRONTEND_IMAGE=docker.io/myuser/cont-frontend
export CONT_PROXY_IMAGE=docker.io/myuser/cont-proxy
export VERSION=latest

# Apply production overlay
kubectl apply -k k8s/overlays/prod

# Diff before apply (dry-run)
kubectl diff -k k8s/overlays/prod

# Rollout status after apply
kubectl rollout status deployment -n cont
```

### Sealed Secrets for Production

For production, use [bitnami-labs/sealed-secrets](https://github.com/bitnami-labs/sealed-secrets)
to encrypt secrets before committing:

```bash
# Create a sealed secret for JWT_SECRET
kubeseal --format=yaml < jwt-secret.yaml > sealed-jwt-secret.yaml

# The sealed secret can be safely committed to source control
```

## Environment Variables

| Variable | Description | Dev Default | Prod |
|----------|-------------|-------------|------|
| `JWT_SECRET` | JWT signing secret | auto-generated | **MUST SET** |
| `POSTGRES_PASSWORD` | Postgres password | kongpass | **MUST SET** |
| `CONT_ADMIN_API_IMAGE` | Admin API image | cont-admin-api:latest | **MUST SET** |
| `CONT_FRONTEND_IMAGE` | Frontend image | cont-frontend:latest | **MUST SET** |
| `CONT_PROXY_IMAGE` | Proxy image | cont-proxy:latest | **MUST SET** |
| `VERSION` | Image tag | latest | **SET** |

## Resource Scaling

| Component | Dev Replicas | Prod Replicas |
|-----------|-------------|---------------|
| admin-api | 1 | 3 |
| frontend | 1 | 2 |
| proxy | 1 | 3 |
| postgres | 1 | 1 |
| redis | 1 | 1 |