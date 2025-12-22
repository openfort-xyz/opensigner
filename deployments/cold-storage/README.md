# Cold Storage Deployment

This directory contains a standalone deployment configuration for the Cold Storage service and its MySQL database.

## Services

- **mysql**: MySQL 8.4 database for Cold Storage
- **cold_storage**: Cold Storage service (Shield)

## Quick Start

### Using Docker Compose

```bash
# From the project root
docker compose --project-directory . -f deployments/cold-storage/docker-compose.yml up -d
```

### Using Make

```bash
# Build and run
make build-cold-storage
make run-cold-storage

# Stop
make stop-cold-storage
```

## Environment Variables

See `.env.example` for available environment variables. Key variables:

- `MYSQL_ROOT_PASSWORD`: MySQL root password (default: `root_password`)
- `MYSQL_DATABASE`: Database name (default: `shield`)
- `MYSQL_USER`: MySQL user (default: `mysql_user`)
- `MYSQL_PASSWORD`: MySQL user password (default: `mysql_password`)
- `COLD_STORAGE_DB_HOST`: Database host (default: `mysql`)
- `COLD_STORAGE_DB_PORT`: Database port (default: `3306`)
- `COLD_STORAGE_HOST_PORT`: Host port for Cold Storage API (default: `7053`)

## Ports

- MySQL: `3306` (configurable via `MYSQL_PORT`)
- Cold Storage API: `7053` (configurable via `COLD_STORAGE_HOST_PORT`)

## Network

Services are connected to the `opensigner_db_network` network, allowing them to communicate with other services in the full stack deployment.

