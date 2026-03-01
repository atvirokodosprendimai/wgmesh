#!/usr/bin/env bash
# goose-run.sh — Run Goose with retry logic, reading settings from the recipe.
#
# Usage: ./company/scripts/goose-run.sh <task-file> [recipe-file]
#
# Reads model, max_turns, and retry settings from the recipe YAML.
# Retries on rate limits or short output. Builds fix instructions on retry.
# Outputs metrics JSON to /tmp/goose-metrics.json.
#
# Environment:
#   GOOSE_PROVIDER   — override recipe provider (default: from recipe)
#   GOOSE_MODEL      — override recipe model (default: from recipe)
#
# Requires: yq, jq, goose CLI

set -euo pipefail

TASK_FILE="${1:?Usage: $0 <task-file> [recipe-file]}"
RECIPE_FILE="${2:-.github/goose-recipes/wgmesh-implementation.yaml}"

if [ ! -f "$TASK_FILE" ]; then
  echo "Error: task file not found: $TASK_FILE" >&2
  exit 1
fi

if ! command -v yq &>/dev/null; then
  echo "Error: yq is required" >&2
  exit 1
fi

# ── Read settings from recipe ──────────────────────────────────

PROVIDER="${GOOSE_PROVIDER:-$(yq -r '.settings.goose_provider // "google"' "$RECIPE_FILE")}"
MODEL="${GOOSE_MODEL:-$(yq -r '.settings.goose_model // "gemini-2.5-flash"' "$RECIPE_FILE")}"
MAX_TURNS=$(yq -r '.settings.max_turns // 50' "$RECIPE_FILE")
MAX_RETRIES=$(yq -r '.retry.max_retries // 2' "$RECIPE_FILE")

export GOOSE_PROVIDER="$PROVIDER"
export GOOSE_MODEL="$MODEL"

echo "Goose config: provider=$PROVIDER model=$MODEL max_turns=$MAX_TURNS retries=$MAX_RETRIES" >&2

# ── Retry loop ──────────────────────────────────────────────────

MAX_ATTEMPTS=$((MAX_RETRIES + 1))
BACKOFF=30
SUCCEEDED=false
JOB_START=$(date +%s)
ATTEMPTS_JSON="[]"

for ATTEMPT in $(seq 1 "$MAX_ATTEMPTS"); do
  echo "=== Goose attempt $ATTEMPT/$MAX_ATTEMPTS ===" >&2
  ATTEMPT_START=$(date +%s)

  if [ "$ATTEMPT" -gt 1 ]; then
    # On retry: build fix instructions from validation errors
    echo "Building fix instructions from previous failure..." >&2

    ERRORS=$({
      echo "Your previous attempt had errors. Fix them:"
      echo ""
      for check_cmd in $(yq -r '.retry.checks[] | .command' "$RECIPE_FILE" 2>/dev/null); do
        echo "=== $check_cmd errors ==="
        eval "$check_cmd" 2>&1 | head -40 || true
        echo ""
      done
      echo "Fix ALL errors above. Then run all checks until clean."
    })

    cat > /tmp/goose-fix.md << FIXEOF
# Fix Implementation Errors (Attempt $ATTEMPT)

$ERRORS

Read the error messages carefully. Common mistakes:
- Using types/fields that don't exist — read the actual source file first
- Test expected strings not matching actual error format
- Missing imports

FIXEOF

    CURRENT_TASK="/tmp/goose-fix.md"
  else
    CURRENT_TASK="$TASK_FILE"
  fi

  set +e
  goose run \
    --no-session \
    --with-builtin "developer" \
    -i "$CURRENT_TASK" \
    --max-turns "$MAX_TURNS" \
    2>&1 | tee /tmp/goose-output.log
  GOOSE_EXIT=$?
  set -e

  LINES=$(wc -l < /tmp/goose-output.log)

  # ── Non-recoverable errors ──
  CLEAN_LOG=$(grep -vE 'grep|\.yml|\.yaml|\.sh|^\s*#|^\s*//|^\d+:' /tmp/goose-output.log || true)

  if echo "$CLEAN_LOG" | grep -qiE "401 unauthorized|authentication failed|invalid.{0,3}api.{0,3}key|api key is invalid"; then
    echo "Error: authentication failure (non-recoverable)" >&2
    exit 1
  fi

  if echo "$CLEAN_LOG" | grep -qi "unexpected argument"; then
    echo "Error: Goose CLI argument error (non-recoverable)" >&2
    exit 1
  fi

  # ── Success detection ──
  TOO_SHORT=false
  [ "$LINES" -lt 5 ] && TOO_SHORT=true

  HAS_SUCCESS=false
  if grep -qE "(go build|go test|gofmt|go vet).*(succeeded|passed|pass|clean|no issues)" /tmp/goose-output.log || \
     grep -qE "All (validation|acceptance|checks)" /tmp/goose-output.log || \
     grep -qE "Implementation (complete|done|finished)" /tmp/goose-output.log; then
    HAS_SUCCESS=true
  fi

  RATE_LIMITED=false
  if grep -qiE "rate.?limit.?(exceeded|hit|reached)|quota.?exceeded|resource.?exhausted|HTTP.?429|status.?429|too many requests" /tmp/goose-output.log; then
    RATE_LIMITED=true
  fi

  # Determine outcome
  OUTCOME="error"
  if [ "$TOO_SHORT" = "false" ] && { [ "$HAS_SUCCESS" = "true" ] || [ "$RATE_LIMITED" = "false" ]; }; then
    OUTCOME="success"
    SUCCEEDED=true
  elif [ "$RATE_LIMITED" = "true" ]; then
    OUTCOME="rate_limited"
  elif [ "$TOO_SHORT" = "true" ]; then
    OUTCOME="short_output"
  fi

  ATTEMPT_END=$(date +%s)
  ATTEMPTS_JSON=$(echo "$ATTEMPTS_JSON" | jq \
    --argjson n "$ATTEMPT" \
    --argjson d "$((ATTEMPT_END - ATTEMPT_START))" \
    --argjson ec "$GOOSE_EXIT" \
    --argjson ol "$LINES" \
    --arg oc "$OUTCOME" \
    '. + [{"number":$n,"duration_seconds":$d,"exit_code":$ec,"output_lines":$ol,"outcome":$oc}]') || true

  if [ "$SUCCEEDED" = "true" ]; then
    echo "Goose completed on attempt $ATTEMPT (exit=$GOOSE_EXIT, outcome=$OUTCOME)" >&2
    break
  fi

  # ── Backoff ──
  SUGGESTED_DELAY=$(grep -oP 'retry in \K[0-9]+' /tmp/goose-output.log 2>/dev/null | tail -1 || true)
  if [ -n "$SUGGESTED_DELAY" ] && [ "$SUGGESTED_DELAY" -gt "$BACKOFF" ] 2>/dev/null; then
    WAIT=$((SUGGESTED_DELAY + 5))
  else
    WAIT=$BACKOFF
  fi

  echo "Attempt $ATTEMPT failed ($OUTCOME). Waiting ${WAIT}s..." >&2
  sleep "$WAIT"
  BACKOFF=$((BACKOFF * 2))
done

# ── Output metrics ──────────────────────────────────────────────

JOB_END=$(date +%s)
jq -n \
  --argjson succeeded "$SUCCEEDED" \
  --argjson total_attempts "$ATTEMPT" \
  --argjson duration "$((JOB_END - JOB_START))" \
  --arg model "$MODEL" \
  --argjson attempts "$ATTEMPTS_JSON" \
  '{
    model: $model,
    succeeded: $succeeded,
    total_attempts: $total_attempts,
    duration_seconds: $duration,
    attempts: $attempts
  }' > /tmp/goose-metrics.json

cat /tmp/goose-metrics.json

if [ "$SUCCEEDED" != "true" ]; then
  echo "Goose failed after $MAX_ATTEMPTS attempts" >&2
  tail -50 /tmp/goose-output.log >&2
  exit 1
fi
