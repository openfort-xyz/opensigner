#!/bin/bash

# Script to generate iframe_nginx.conf from template
# Edit the variables below to configure your deployment

########################################################
# Configuration
AUTH_SERVER_URL="http://authservice:3000"
AUTH_SERVER_HOST="authservice"
AUTH_SERVER_HOT_PORT="8080"
COLD_STORAGE_URL="http://coldstorage:8080"
########################################################

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEMPLATE_FILE="${SCRIPT_DIR}/iframe_nginx.conf.template"
OUTPUT_FILE="${SCRIPT_DIR}/iframe_nginx.conf"
# Check if template file exists
if [[ ! -f "$TEMPLATE_FILE" ]]; then
    echo "Error: Template file not found: $TEMPLATE_FILE" >&2
    exit 1
fi

# Check if envsubst is available
if ! command -v envsubst &> /dev/null; then
    echo "Error: envsubst command not found. Please install gettext package." >&2
    echo "On macOS: brew install gettext" >&2
    echo "On Ubuntu/Debian: apt-get install gettext-base" >&2
    exit 1
fi

# Export variables for envsubst
export AUTH_SERVER_URL
export AUTH_SERVER_HOST
export AUTH_SERVER_HOT_PORT
export COLD_STORAGE_URL

# Generate config file
envsubst '$AUTH_SERVER_URL $AUTH_SERVER_HOST $AUTH_SERVER_HOT_PORT $COLD_STORAGE_URL' < "$TEMPLATE_FILE" > "$OUTPUT_FILE"

echo "Generated $OUTPUT_FILE"
echo "  AUTH_SERVER_URL: $AUTH_SERVER_URL"
echo "  AUTH_SERVER_HOST: $AUTH_SERVER_HOST"
echo "  AUTH_SERVER_HOT_PORT: $AUTH_SERVER_HOT_PORT"
echo "  COLD_STORAGE_URL: $COLD_STORAGE_URL"

