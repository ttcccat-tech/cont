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
