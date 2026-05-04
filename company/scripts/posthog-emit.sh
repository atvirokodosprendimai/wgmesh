#!/usr/bin/env bash
# Emit a single event to PostHog (server-side, write-only project key).
# Non-fatal: failure prints ::warning:: and exits 0.
#
# Usage:
#   posthog-emit.sh <event_name> [<distinct_id>] [<properties_json>]
#
# Env:
#   POSTHOG_PROJECT_KEY  required; phc_... write-only project ingest key
#   POSTHOG_HOST         optional; defaults to https://eu.i.posthog.com
set -euo pipefail

EVENT="${1:?event name required}"
DISTINCT_ID="${2:-${GITHUB_REPOSITORY:-ai-pipeline-template}}"
# Bash quirk: `${3:-{}}` corrupts a set value with an extra `}` because the
# parser eats the inner `{}` as part of the default expression. Use explicit
# branching so a passed JSON object stays intact.
if [ "${3:-}" = "" ]; then
  PROPS="{}"
else
  PROPS="$3"
fi

if [ -z "${POSTHOG_PROJECT_KEY:-}" ]; then
  echo "::warning::POSTHOG_PROJECT_KEY unset, skipping event $EVENT"
  exit 0
fi

# Inject standard properties: repo, run_id, run_url, workflow when available.
# These match the GitHub Actions environment when invoked from a workflow step
# and are harmlessly null when invoked from a local shell.
STANDARD_PROPS=$(jq -nc \
  --arg repo "${GITHUB_REPOSITORY:-}" \
  --arg run_id "${GITHUB_RUN_ID:-}" \
  --arg run_url "${GITHUB_SERVER_URL:-https://github.com}/${GITHUB_REPOSITORY:-}/actions/runs/${GITHUB_RUN_ID:-}" \
  --arg workflow "${GITHUB_WORKFLOW:-}" \
  --arg ref "${GITHUB_REF_NAME:-}" \
  '{repo: $repo, run_id: $run_id, run_url: $run_url, workflow: $workflow, ref: $ref}')

MERGED_PROPS=$(jq -nc --argjson std "$STANDARD_PROPS" --argjson custom "$PROPS" '$std + $custom')

PAYLOAD=$(jq -nc \
  --arg api_key "$POSTHOG_PROJECT_KEY" \
  --arg event "$EVENT" \
  --arg distinct_id "$DISTINCT_ID" \
  --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --argjson props "$MERGED_PROPS" \
  '{api_key: $api_key, event: $event, distinct_id: $distinct_id, timestamp: $ts, properties: $props}')

URL="${POSTHOG_HOST:-https://eu.i.posthog.com}/i/v0/e/"

curl --fail-with-body --silent --show-error --max-time 10 \
  -X POST -H 'Content-Type: application/json' \
  -d "$PAYLOAD" "$URL" \
  >/dev/null \
  || { code=$?; echo "::warning::posthog emit failed for event=$EVENT (curl exit $code, non-fatal)"; }
