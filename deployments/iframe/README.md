# Iframe Deployment

This directory contains a standalone deployment configuration for the Iframe service.

## Services

- **iframe**: Iframe service for secure key operations

## Quick Start

1. **Copy the environment file:**
   ```bash
   cp .env.example .env
   ```

2. **Update `.env` with your configuration** (if needed)

3. **Deploy:**
   ```bash
   make run-iframe
   ```

   Or manually:
   ```bash
   docker compose --project-directory . -f deployments/iframe/docker-compose.yml up -d
   ```

## Configuration

See `.env.example` for all available environment variables. Copy it to `.env` and customize as needed. Key variables:

- `IFRAME_CONTAINER_PORT`: Container port (default: `5173`)
- `IFRAME_HOST_PORT`: Host port (default: `7050`)

## Ports

- Iframe: `7050` (configurable via `IFRAME_HOST_PORT`)


## Running Standalone

When running the iframe service standalone (outside of the main Docker Compose setup), you need to configure the nginx configuration to point to your service endpoints.

### Step 1: Configure Service URLs

Before deploying, you must generate the nginx configuration file. Edit `opensigner/iframe/generate-nginx-config.sh` and modify the configuration section with your service URLs.

```

**Example configurations:**

- **Local development** (services running on localhost):
  ```bash
  AUTH_SERVER_URL="http://localhost:3000"
  AUTH_SERVER_HOST="localhost"
  AUTH_SERVER_HOT_PORT="8080"
  COLD_STORAGE_URL="http://localhost:8080"
  ```

- **Remote services** (services on different hosts):
  ```bash
  AUTH_SERVER_URL="http://auth.example.com:3000"
  AUTH_SERVER_HOST="auth.example.com"
  AUTH_SERVER_HOT_PORT="8080"
  COLD_STORAGE_URL="http://cold.example.com:8080"
  ```

- **Docker Compose** (default - services via Docker network):
  ```bash
  AUTH_SERVER_URL="http://authservice:3000"
  AUTH_SERVER_HOST="authservice"
  AUTH_SERVER_HOT_PORT="8080"
  COLD_STORAGE_URL="http://coldstorage:8080"
  ```

### Step 2: Generate Nginx Configuration and deploy

By running the following command, the script generates the nginx configuration and deploys the docker:

```bash
make run-iframe
```

You can also run the nginx script individually:
```bash
cd opensigner/iframe
./generate-nginx-config.sh
```

## Dependencies

The iframe service depends on `auth_service`, `hot_storage`, and `cold_storage` being available on the network. When running standalone, ensure these services are also running and accessible at the URLs you configure in `generate-nginx-config.sh`.

## Stopping

```bash
make stop-iframe
```

Or:
```bash
docker compose --project-directory . -f deployments/iframe/docker-compose.yml down
```

