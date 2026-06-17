#!/bin/bash
#
# generate-config.sh - Generate config.js from template using environment variables
#
# Usage:
#   export INTERCOM_ENABLED=true
#   export INTERCOM_APP_ID="abc123xyz"
#   export DRIFT_ENABLED=false
#   ./deploy/web/generate-config.sh
#
# This script reads public/config.js.template and replaces placeholder variables
# with values from environment variables, then writes the result to public/config.js.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TEMPLATE_FILE="$PROJECT_ROOT/public/config.js.template"
OUTPUT_FILE="$PROJECT_ROOT/public/config.js"

# Default values
INTERCOM_ENABLED="${INTERCOM_ENABLED:-false}"
INTERCOM_APP_ID="${INTERCOM_APP_ID:-}"
DRIFT_ENABLED="${DRIFT_ENABLED:-false}"
DRIFT_APP_ID="${DRIFT_APP_ID:-}"

# Ensure template exists
if [ ! -f "$TEMPLATE_FILE" ]; then
    echo "Error: Template file not found: $TEMPLATE_FILE" >&2
    exit 1
fi

# Generate config.js from template
sed \
    -e "s/{{INTERCOM_ENABLED}}/$INTERCOM_ENABLED/g" \
    -e "s|{{INTERCOM_APP_ID}}|$INTERCOM_APP_ID|g" \
    -e "s/{{DRIFT_ENABLED}}/$DRIFT_ENABLED/g" \
    -e "s|{{DRIFT_APP_ID}}|$DRIFT_APP_ID|g" \
    "$TEMPLATE_FILE" > "$OUTPUT_FILE"

echo "Generated $OUTPUT_FILE"
echo "Configuration:"
echo "  Intercom: enabled=$INTERCOM_ENABLED, appId=${INTERCOM_APP_ID:+(set)}"
echo "  Drift: enabled=$DRIFT_ENABLED, appId=${DRIFT_APP_ID:+(set)}"
