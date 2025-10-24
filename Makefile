.PHONY: all build run clean docs

all: build run

build:
	docker compose build iframe iframe-sample cold_storage hot_storage auth_service

clean:
	find . -name 'node_modules' -type d -prune -exec rm -rf '{}' +
	docker-compose down --rmi 'all' -v

run:
	docker compose up postgres mysql auth_service iframe iframe-sample hot_storage cold_storage

docs:
	docker-compose up docs
