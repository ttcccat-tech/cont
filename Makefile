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
