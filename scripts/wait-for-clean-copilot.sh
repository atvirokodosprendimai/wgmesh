#!/usr/bin/env bash
# wait-for-clean-copilot.sh — block until Copilot's review on a PR converges.
#
# Usage:
#   scripts/wait-for-clean-copilot.sh <pr-number> [<owner/repo>]
#
# Env overrides:
#   TIMEOUT=1800        # seconds (default 30 min)
#   POLL_INTERVAL=30    # seconds between polls
#
# Convergence criteria (all must hold):
#   1. Latest copilot-pull-request-reviewer review submittedAt > last commit's
#      committedDate. Ensures Copilot has reviewed the latest code.
#   2. Zero inline review comments created after that commit. Ensures Copilot
#      had no new findings.
#
# Side-effects:
#   - Re-requests review on each poll cycle (Copilot doesn't auto-re-review
#     on subsequent commits in many configurations).
#
# Exit codes:
#   0 — Copilot converged (clean review)
#   1 — Timeout reached without convergence
#   2 — Argument or auth error
#
# Born from wgmesh PR #560 incident (2026-05-05): 8 review rounds chased
# manually because there was no programmatic way to detect convergence.
set -euo pipefail

PR="${1:-}"
if [ -z "$PR" ]; then
  echo "Usage: $0 <pr-number> [<owner/repo>]" >&2
  exit 2
fi

REPO="${2:-${GITHUB_REPOSITORY:-}}"
if [ -z "$REPO" ]; then
  REPO=$(gh repo view --json nameWithOwner --jq '.nameWithOwner' 2>/dev/null || true)
fi
if [ -z "$REPO" ]; then
  echo "ERROR: cannot determine repo. Pass owner/repo as 2nd arg or set GITHUB_REPOSITORY." >&2
  exit 2
fi

TIMEOUT="${TIMEOUT:-1800}"
POLL_INTERVAL="${POLL_INTERVAL:-30}"
START=$(date +%s)

echo "Polling Copilot review on $REPO#$PR"
echo "  timeout:        ${TIMEOUT}s"
echo "  poll interval:  ${POLL_INTERVAL}s"

# Initial review request — idempotent if already on the reviewer list.
gh pr edit "$PR" --repo "$REPO" --add-reviewer copilot-pull-request-reviewer >/dev/null 2>&1 || true

while true; do
  elapsed=$(( $(date +%s) - START ))
  if [ "$elapsed" -gt "$TIMEOUT" ]; then
    echo "::error::wait-for-clean-copilot timed out after ${TIMEOUT}s on $REPO#$PR" >&2
    exit 1
  fi

  # Re-read the head commit timestamp every iteration. If the author pushes
  # a new commit while we wait, the convergence baseline must advance —
  # otherwise an old Copilot review would falsely satisfy the time check.
  last_commit_at=$(gh pr view "$PR" --repo "$REPO" --json commits --jq '.commits[-1].committedDate')
  if [ -z "$last_commit_at" ] || [ "$last_commit_at" = "null" ]; then
    echo "[$(date -u '+%H:%M:%S')] cannot read last commit timestamp — retrying"
    sleep "$POLL_INTERVAL"
    continue
  fi

  latest_review=$(gh pr view "$PR" --repo "$REPO" --json reviews --jq '
    [.reviews[] | select(.author.login == "copilot-pull-request-reviewer")] | last // null')

  if [ "$latest_review" = "null" ]; then
    echo "[$(date -u '+%H:%M:%S')] no Copilot review yet — waiting ${POLL_INTERVAL}s"
    sleep "$POLL_INTERVAL"
    gh pr edit "$PR" --repo "$REPO" --add-reviewer copilot-pull-request-reviewer >/dev/null 2>&1 || true
    continue
  fi

  review_at=$(printf '%s' "$latest_review" | jq -r '.submittedAt')

  if ! [[ "$review_at" > "$last_commit_at" ]]; then
    echo "[$(date -u '+%H:%M:%S')] last review ($review_at) predates last commit ($last_commit_at) — waiting"
    sleep "$POLL_INTERVAL"
    gh pr edit "$PR" --repo "$REPO" --add-reviewer copilot-pull-request-reviewer >/dev/null 2>&1 || true
    continue
  fi

  # Count Copilot's own inline comments since last commit. Filter on author
  # login so a human reviewer's comment doesn't keep this script looping.
  # `--paginate` follows the Link header so we don't miss comments past the
  # default 30-per-page on busy PRs.
  fresh_count=$(gh api --paginate "repos/$REPO/pulls/$PR/comments" \
    --jq "[.[] | select(.user.login == \"copilot-pull-request-reviewer\" and .created_at > \"$last_commit_at\")] | length" \
    | awk '{sum += $1} END {print sum + 0}')

  if [ "$fresh_count" -eq 0 ]; then
    echo "[$(date -u '+%H:%M:%S')] ✓ converged: review @ $review_at, 0 fresh Copilot inline comments since $last_commit_at"
    exit 0
  fi

  echo "[$(date -u '+%H:%M:%S')] review @ $review_at, $fresh_count fresh Copilot inline comment(s) — iterating"
  sleep "$POLL_INTERVAL"
done
