.PHONY: all build run clean docs run-auth-server build-auth-server stop-auth-server

all: build run

build:
	docker compose \
		--project-directory . \
		-f deployments/auth-server/docker-compose.yml \
		-f docker-compose.yml \
		build postgres auth_service hot_storage iframe iframe-sample cold_storage mysql

clean:
	find . -name 'node_modules' -type d -prune -exec rm -rf '{}' +
	docker compose \
		--project-directory . \
		-f deployments/auth-server/docker-compose.yml \
		-f docker-compose.yml \
		down --rmi 'all' -v

run:
	docker compose \
		--project-directory . \
		-f deployments/auth-server/docker-compose.yml \
		-f docker-compose.yml \
		up -d postgres auth_service hot_storage mysql iframe iframe-sample cold_storage

docs:
	docker-compose up docs

# Auth Server (Auth Service + Hot Storage + PostgreSQL) Standalone Deployment
run-auth-server:
	docker compose --project-directory . -f deployments/auth-server/docker-compose.yml up -d

build-auth-server:
	docker compose --project-directory . -f deployments/auth-server/docker-compose.yml build

stop-auth-server:
	docker compose --project-directory . -f deployments/auth-server/docker-compose.yml down
