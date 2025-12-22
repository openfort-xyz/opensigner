.PHONY: all build run clean docs run-auth-server build-auth-server stop-auth-server run-cold-storage build-cold-storage stop-cold-storage

all: build run

build:
	docker compose \
		--project-directory . \
		-f deployments/auth-server/docker-compose.yml \
		-f deployments/cold-storage/docker-compose.yml \
		-f docker-compose.yml \
		build postgres auth_service hot_storage mysql cold_storage iframe iframe-sample

clean:
	find . -name 'node_modules' -type d -prune -exec rm -rf '{}' +
	docker compose \
		--project-directory . \
		-f deployments/auth-server/docker-compose.yml \
		-f deployments/cold-storage/docker-compose.yml \
		-f docker-compose.yml \
		down --rmi 'all' -v

run:
	docker compose \
		--project-directory . \
		-f deployments/auth-server/docker-compose.yml \
		-f deployments/cold-storage/docker-compose.yml \
		-f docker-compose.yml \
		up -d postgres auth_service hot_storage mysql cold_storage iframe iframe-sample

docs:
	docker-compose up docs

# Auth Server (Auth Service + Hot Storage + PostgreSQL) Standalone Deployment
run-auth-server:
	docker compose --project-directory . -f deployments/auth-server/docker-compose.yml up -d

build-auth-server:
	docker compose --project-directory . -f deployments/auth-server/docker-compose.yml build

stop-auth-server:
	docker compose --project-directory . -f deployments/auth-server/docker-compose.yml down

# Cold Storage (Cold Storage + MySQL) Standalone Deployment
run-cold-storage:
	docker compose --project-directory . -f deployments/cold-storage/docker-compose.yml up -d

build-cold-storage:
	docker compose --project-directory . -f deployments/cold-storage/docker-compose.yml build

stop-cold-storage:
	docker compose --project-directory . -f deployments/cold-storage/docker-compose.yml down
