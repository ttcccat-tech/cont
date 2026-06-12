# Cont Gateway — Deploy Makefile
# Usage: make [target]
#
# Prerequisites:
#   - Docker Engine 24.0+
#   - Docker Compose v2.20+
#   - Registry credentials (if pushing images)

# ── Configuration ────────────────────────────────────────────────────────────
REGISTRY ?= cont
IMAGE_TAG ?= $(shell git rev-parse --short HEAD)
APP_DIR := $(shell pwd)

.PHONY: help build build-no-cache start stop restart logs ps \
        clean networks create-networks

# ── Targets ─────────────────────────────────────────────────────────────────

help:
	@echo "Cont Gateway — Deploy Targets"
	@echo ""
	@echo "  make build          Build all images (local cache)"
	@echo "  make build-no-cache Build all images (no cache, fresh)"
	@echo "  make push           Push all images to registry"
	@echo "  make start          Create network + start all services"
	@echo "  make stop           Stop all services"
	@echo "  make restart        Restart all services"
	@echo "  make logs           Tail all service logs"
	@echo "  make ps             Show running containers"
	@echo "  make clean          Stop + remove all containers and volumes"
	@echo "  make networks       Create Docker networks"

build:
	@echo "Building Cont Gateway images (tag=$(IMAGE_TAG))..."
	docker compose build

build-no-cache:
	@echo "Building Cont Gateway images (no cache)..."
	docker compose build --pull --no-cache

push: build
	@echo "Pushing images to $(REGISTRY)..."
	docker push $(REGISTRY)/admin-api:$(IMAGE_TAG)
	docker push $(REGISTRY)/admin-api:latest
	docker push $(REGISTRY)/proxy:$(IMAGE_TAG)
	docker push $(REGISTRY)/proxy:latest
	docker push $(REGISTRY)/frontend:$(IMAGE_TAG)
	docker push $(REGISTRY)/frontend:latest

start: networks
	@echo "Starting Cont Gateway..."
	docker compose up -d
	@echo ""
	@echo "Services running:"
	@docker compose ps

stop:
	docker compose stop

restart: stop start

logs:
	docker compose logs -f

ps:
	docker compose ps

clean:
	docker compose down -v
	@echo "All containers and volumes removed."

networks:
	@docker network inspect cont_net >/dev/null 2>&1 || \
		docker network create cont_net
	@echo "Network cont_net ready."

create-networks:
	$(MAKE) networks