#!/bin/sh
set -e

echo "Running migrations..."
pnpm dlx @better-auth/cli@1.3.4 migrate -- -y --config src/server.ts

echo "Starting app..."
exec pnpm dev -- --host "$HOST"
