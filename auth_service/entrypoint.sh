#!/bin/sh
set -e

echo "Running migrations..."
npm exec @better-auth/cli@1.3.4 migrate -- -y --config src/server.ts

echo "Starting app..."
exec yarn dev --host $HOST
