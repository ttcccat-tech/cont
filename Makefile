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
.PHONY: k8s-apply k8s-delete k8s-status k8s-logs k8s-port-forward

# Apply all k8s manifests (requires kubectl + running cluster)
k8s-apply:
	@echo "Applying k8s manifests..." && \
	kubectl apply -f k8s/namespace.yaml && \
	kubectl apply -f k8s/config.yaml && \
	kubectl apply -f k8s/postgres.yaml && \
	kubectl apply -f k8s/postgres-svc.yaml && \
	kubectl apply -f k8s/redis.yaml && \
	kubectl apply -f k8s/redis-svc.yaml && \
	kubectl apply -f k8s/admin-api.yaml && \
	kubectl apply -f k8s/frontend.yaml && \
	kubectl apply -f k8s/proxy.yaml && \
	kubectl rollout status deployment/cont-admin-api -n cont --timeout=120s || true && \
	kubectl get pods -n cont

# Delete all k8s resources
k8s-delete:
	@kubectl delete -f k8s/ --ignore-not-found && \
	echo "All cont k8s resources deleted."

# Show pod status
k8s-status:
	@kubectl get pods,svc -n cont -o wide

# Tail logs from all pods
k8s-logs:
	@kubectl logs -n cont -l app=cont-admin-api --tail=50 -f &
	@kubectl logs -n cont -l app=cont-proxy --tail=50 -f &
	@wait

# Port-forward for local dev access to k8s-deployed cont
k8s-port-forward:
	@echo "Forwarding ports (Ctrl-C to stop):" && \
	kubectl port-forward -n cont svc/cont-admin-api 8001:8001 & \
	kubectl port-forward -n cont svc/cont-proxy 8000:8000 & \
	kubectl port-forward -n cont svc/cont-frontend 3003:80 & \
	wait
