#!/usr/bin/env bash
# goose-validate.sh — Run validation checks from the recipe against the working tree.
#
# Usage: ./company/scripts/goose-validate.sh [recipe-file]
#
# Runs each check defined in the recipe's retry.checks array.
# Also runs gofmt and reports per-check pass/fail.
#
# Outputs (to stdout): JSON summary of results.
# Exit code: 0 if all pass, 1 if any fail.
#
# Requires: yq, jq

set -euo pipefail

RECIPE_FILE="${1:-.github/goose-recipes/wgmesh-implementation.yaml}"

if ! command -v yq &>/dev/null; then
  echo "Error: yq is required" >&2
  exit 1
fi

if ! command -v jq &>/dev/null; then
  echo "Error: jq is required" >&2
  exit 1
fi

# ── Check for changes ──────────────────────────────────────────

if git diff --quiet && git diff --cached --quiet; then
  echo '{"has_changes":false,"all_passed":false,"checks":[]}' | jq .
  echo "No changes detected" >&2
  exit 1
fi

# ── Run checks from recipe ─────────────────────────────────────

results="[]"
all_passed=true

while IFS= read -r cmd; do
  [ -z "$cmd" ] && continue

  echo "=== Running: $cmd ===" >&2
  start=$(date +%s)

  if eval "$cmd" >&2 2>&1; then
    passed=true
  else
    passed=false
    all_passed=false
  fi

  duration=$(( $(date +%s) - start ))
  results=$(echo "$results" | jq \
    --arg cmd "$cmd" \
    --argjson passed "$passed" \
    --argjson dur "$duration" \
    '. + [{"command":$cmd,"passed":$passed,"duration_seconds":$dur}]')

done < <(yq -r '.retry.checks[] | .command' "$RECIPE_FILE" 2>/dev/null)

# ── gofmt check ────────────────────────────────────────────────

echo "=== Running: gofmt ===" >&2
gofmt -w .
unformatted=$(gofmt -l .)
if [ -z "$unformatted" ]; then
  fmt_passed=true
else
  fmt_passed=false
  all_passed=false
  echo "Unformatted files: $unformatted" >&2
fi
results=$(echo "$results" | jq \
  --argjson passed "$fmt_passed" \
  '. + [{"command":"gofmt","passed":$passed,"duration_seconds":0}]')

# ── Diff stats ──────────────────────────────────────────────────

diff_stat=$(git diff --numstat origin/main 2>/dev/null || true)
if [ -n "$diff_stat" ]; then
  files_changed=$(echo "$diff_stat" | wc -l | tr -d ' ')
  insertions=$(echo "$diff_stat" | awk '{s+=$1}END{print s+0}')
  deletions=$(echo "$diff_stat" | awk '{s+=$2}END{print s+0}')
  test_files=$(echo "$diff_stat" | grep -c '_test\.go' || true)
  test_insertions=$(echo "$diff_stat" | grep '_test\.go' | awk '{s+=$1}END{print s+0}')
else
  files_changed=0; insertions=0; deletions=0; test_files=0; test_insertions=0
fi

# ── Output JSON ─────────────────────────────────────────────────

jq -n \
  --argjson has_changes true \
  --argjson all_passed "$all_passed" \
  --argjson checks "$results" \
  --argjson files "$files_changed" \
  --argjson ins "$insertions" \
  --argjson del "$deletions" \
  --argjson tf "$test_files" \
  --argjson ti "$test_insertions" \
  '{
    has_changes: $has_changes,
    all_passed: $all_passed,
    checks: $checks,
    diff: {files_changed:$files, insertions:$ins, deletions:$del, test_files:$tf, test_insertions:$ti}
  }'

if [ "$all_passed" != "true" ]; then
  exit 1
fi
