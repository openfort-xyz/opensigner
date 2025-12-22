# Auth Server Standalone Deployment

This directory contains a standalone deployment configuration that includes:
- PostgreSQL database (shared by both services)
- Auth Service
- Hot Storage

All three services are deployed together in one docker-compose file.

## Quick Start

1. **Copy the environment file:**
   ```bash
   cp .env.example .env
   ```

2. **Update `.env` with your configuration** (especially `AUTH_JWT_SECRET` for production)

3. **Deploy:**
   ```bash
   make run-auth-server
   ```

   Or manually:
   ```bash
   docker compose --project-directory ../.. -f deployments/auth-server/docker-compose.yml up -d
   ```

## Services

- **PostgreSQL**: Port `5432` (default)
  - Database: `authservice` (for auth service)
  - Database: `hotstorage` (for hot storage)
- **Auth Service**: `http://localhost:7052`
- **Hot Storage**: `http://localhost:7054`

## Configuration

See `.env.example` for all available environment variables. Copy it to `.env` and customize as needed.

## API Endpoints

**Auth Service:**
- Health: `http://localhost:7052/health`
- Auth endpoints: `http://localhost:7052/api/auth/*`
- JWKS: `http://localhost:7052/.well-known/jwks.json`

**Hot Storage:**
- Health: `http://localhost:7054/health`
- Device endpoints: `http://localhost:7054/v1/devices/*` and `http://localhost:7054/v2/devices/*`
- Account endpoints: `http://localhost:7054/v2/accounts/*`

## Stopping

```bash
make stop-auth-server
```

Or:
```bash
docker compose --project-directory ../.. -f deployments/auth-server/docker-compose.yml down
```

