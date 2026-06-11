.PHONY: all build up down restart logs admin-build admin-run

all: up

# Build everything
build: admin-build
	docker compose build proxy

# Start full stack
up:
	docker compose up -d
	@echo "cont Gateway running:"
	@echo "  Proxy:   http://localhost:8000"
	@echo "  Admin:   http://localhost:8001"
	@echo "  Status:  http://localhost:8000/status"
	@echo "  Metrics: http://localhost:8000/metrics"

# Stop
down:
	docker compose down

# Restart
restart: down up

# Logs
logs:
	docker compose logs -f

# Build admin-api
admin-build:
	docker compose build admin-api

# Run admin-api locally (Go must be installed)
admin-run:
	cd admin-api && go run main.go

# Init DB
db-migrate:
	docker compose run --rm admin-api ./cont-admin-api --migrate

# Test route match
test-route:
	curl -s http://localhost:8000/mock -H "Host: example.com" -v

# Prometheus metrics
metrics:
	curl -s http://localhost:8000/metrics | head -20

# ── Production Deployment ────────────────────────────────────
# Build and tag with git hash for traceability
VERSION ?= $(shell git rev-parse --short HEAD)
IMAGE_ADMIN_API := cont-admin-api:$(VERSION)
IMAGE_FRONTEND := cont-frontend:$(VERSION)

# Build production images with version tags
build-prod:
	docker compose build --build-arg GIT_COMMIT=$(VERSION) admin-api frontend
	docker tag cont-admin-api $(IMAGE_ADMIN_API)
	docker tag cont-frontend $(IMAGE_FRONTEND)
	@echo "Built: $(IMAGE_ADMIN_API) and $(IMAGE_FRONTEND)"

# Push images to registry (configure REGISTRY in environment)
push-prod:
	docker tag cont-admin-api $(REGISTRY)/cont-admin-api:$(VERSION)
	docker tag cont-frontend $(REGISTRY)/cont-frontend:$(VERSION)
	docker push $(REGISTRY)/cont-admin-api:$(VERSION)
	docker push $(REGISTRY)/cont-frontend:$(VERSION)

# Deploy to production (single node)
deploy-prod:
	JWT_SECRET=$$(openssl rand -hex 32) \
	docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d
	@echo "Production deployed with version $(VERSION)"

# Rolling restart (zero-downtime)
roll:
	docker compose up -d --no-deps admin-api
	@echo "Rolled admin-api"

# Database backup
db-backup:
	@mkdir -p backups
	docker compose exec -T postgres pg_dump -U kong cont > backups/cont-$(shell date +%Y%m%d-%H%M%S).sql
	@echo "Backup saved to backups/"

# ── Kubernetes Deployment ─────────────────────────────────────────────────────
.PHONY: k8s-apply k8s-dev-apply k8s-prod-apply k8s-delete k8s-diff k8s-status k8s-logs k8s-port-forward

# Apply all k8s manifests via kustomize (requires kubectl + running cluster)
k8s-apply:
	@echo "Applying k8s/base via kustomize..." && \
	kubectl apply -k k8s/base && \
	kubectl rollout status deployment/cont-admin-api -n cont --timeout=120s || true && \
	kubectl get pods -n cont

# Apply dev overlay (minikube/kind friendly)
k8s-dev-apply:
	@echo "Applying k8s/overlays/dev via kustomize..." && \
	kubectl apply -k k8s/overlays/dev && \
	kubectl rollout status deployment/cont-admin-api -n cont --timeout=120s || true && \
	kubectl get pods -n cont

# Apply prod overlay (HA, resource limits, no hardcoded secrets)
# Required env: CONT_ADMIN_API_IMAGE, CONT_FRONTEND_IMAGE, CONT_PROXY_IMAGE, VERSION
k8s-prod-apply:
	@echo "Applying k8s/overlays/prod via kustomize..." && \
	kubectl apply -k k8s/overlays/prod && \
	kubectl rollout status deployment/cont-admin-api -n cont --timeout=120s && \
	kubectl rollout status deployment/cont-proxy -n cont --timeout=120s && \
	kubectl get pods -n cont

# Diff (dry-run) for any overlay
k8s-diff:
	@kubectl diff -k k8s/base

k8s-dev-diff:
	@kubectl diff -k k8s/overlays/dev

k8s-prod-diff:
	@kubectl diff -k k8s/overlays/prod

# Delete all k8s resources (use kustomize for cleanup)
k8s-delete:
	@kubectl delete -k k8s/base --ignore-not-found && \
	echo "All cont k8s resources deleted."

# Show pod/service status
k8s-status:
	@kubectl get pods,svc,configmap,secret -n cont -o wide

# Tail logs from all pods
k8s-logs:
	@kubectl logs -n cont -l app=cont-admin-api --tail=50 -f &
	@kubectl logs -n cont -l app=cont-proxy --tail=50 -f &
	@kubectl logs -n cont -l app=cont-frontend --tail=50 -f &
	@wait

# Port-forward for local dev access to k8s-deployed cont
k8s-port-forward:
	@echo "Forwarding ports (Ctrl-C to stop):" && \
	kubectl port-forward -n cont svc/cont-admin-api 8001:8001 & \
	kubectl port-forward -n cont svc/cont-proxy 8000:8000 & \
	kubectl port-forward -n cont svc/cont-frontend 3003:80 & \
	wait
