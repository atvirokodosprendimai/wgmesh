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

# Inline review COMMENT author login is "Copilot" (display name).
# Top-level REVIEW author login is "copilot-pull-request-reviewer".
# These are two different surfaces. Use the right login for each query.
COPILOT_REVIEW_LOGIN="copilot-pull-request-reviewer"
COPILOT_COMMENT_LOGIN="Copilot"

# Fail with the documented exit code 2 on any pre-poll setup failure.
fatal() {
  echo "ERROR: $1" >&2
  exit 2
}

# Verify gh + jq + curl available so later failures don't surface under
# set -e with an unrelated exit code (Copilot review on PR #565 round-2
# finding C: enforce the documented exit-code contract).
command -v gh >/dev/null 2>&1 || fatal "gh CLI not found"
command -v jq >/dev/null 2>&1 || fatal "jq not found"

echo "Polling Copilot review on $REPO#$PR"
echo "  timeout:        ${TIMEOUT}s"
echo "  poll interval:  ${POLL_INTERVAL}s"

# Initial review request — idempotent if already on the reviewer list.
gh pr edit "$PR" --repo "$REPO" --add-reviewer "$COPILOT_REVIEW_LOGIN" >/dev/null 2>&1 || true

# head_freshness_baseline returns the most-recent of (head commit committedDate,
# head commit authoredDate, last PR update). committedDate alone can be older
# than the actual push when force-push/rebase/cherry-pick brings in commits
# with stale git metadata (Copilot review on PR #565 round-2 finding A).
head_freshness_baseline() {
  local pr_meta committed_at authored_at updated_at
  pr_meta=$(gh pr view "$PR" --repo "$REPO" --json commits,updatedAt 2>/dev/null) || return 1
  committed_at=$(printf '%s' "$pr_meta" | jq -r '.commits[-1].committedDate // empty')
  authored_at=$(printf '%s' "$pr_meta" | jq -r '.commits[-1].authoredDate // empty')
  updated_at=$(printf '%s' "$pr_meta" | jq -r '.updatedAt // empty')
  printf '%s\n' "$committed_at" "$authored_at" "$updated_at" \
    | grep -v '^$' | sort -r | head -n 1
}

while true; do
  elapsed=$(( $(date +%s) - START ))
  # -ge so the timeout fires AT the boundary, not one sleep past it
  # (Copilot review on PR #565 round-2 finding B).
  if [ "$elapsed" -ge "$TIMEOUT" ]; then
    echo "::error::wait-for-clean-copilot timed out after ${elapsed}s (limit ${TIMEOUT}s) on $REPO#$PR" >&2
    exit 1
  fi

  # Re-read the freshness baseline every iteration. If the author pushes
  # a new commit (or rebases) during the wait, the baseline must advance —
  # otherwise an old Copilot review would falsely satisfy the time check.
  baseline=$(head_freshness_baseline) || baseline=""
  if [ -z "$baseline" ] || [ "$baseline" = "null" ]; then
    echo "[$(date -u '+%H:%M:%S')] cannot read PR freshness baseline — retrying"
    sleep "$POLL_INTERVAL"
    continue
  fi

  latest_review=$(gh pr view "$PR" --repo "$REPO" --json reviews --jq '
    [.reviews[] | select(.author.login == "'"$COPILOT_REVIEW_LOGIN"'")] | last // null' 2>/dev/null) \
    || latest_review="null"

  if [ "$latest_review" = "null" ]; then
    echo "[$(date -u '+%H:%M:%S')] no Copilot review yet — waiting ${POLL_INTERVAL}s"
    sleep "$POLL_INTERVAL"
    gh pr edit "$PR" --repo "$REPO" --add-reviewer "$COPILOT_REVIEW_LOGIN" >/dev/null 2>&1 || true
    continue
  fi

  review_at=$(printf '%s' "$latest_review" | jq -r '.submittedAt // empty')
  if [ -z "$review_at" ]; then
    sleep "$POLL_INTERVAL"
    continue
  fi

  if ! [[ "$review_at" > "$baseline" ]]; then
    echo "[$(date -u '+%H:%M:%S')] last review ($review_at) predates baseline ($baseline) — waiting"
    sleep "$POLL_INTERVAL"
    gh pr edit "$PR" --repo "$REPO" --add-reviewer "$COPILOT_REVIEW_LOGIN" >/dev/null 2>&1 || true
    continue
  fi

  # Count Copilot's own inline comments since the baseline. Inline comment
  # author login is "Copilot" (the bot identity), distinct from the review
  # surface's "copilot-pull-request-reviewer". `--paginate` follows the Link
  # header so we don't miss comments past the default 30-per-page on busy
  # PRs (round-1 finding 3). Filter avoids loop-on-human-comment (round-1
  # finding 2).
  fresh_count=$(gh api --paginate "repos/$REPO/pulls/$PR/comments" \
    --jq "[.[] | select(.user.login == \"$COPILOT_COMMENT_LOGIN\" and .created_at > \"$baseline\")] | length" 2>/dev/null \
    | awk '{sum += $1} END {print sum + 0}')

  if [ "$fresh_count" -eq 0 ]; then
    echo "[$(date -u '+%H:%M:%S')] ✓ converged: review @ $review_at, 0 fresh Copilot inline comments since $baseline"
    exit 0
  fi

  echo "[$(date -u '+%H:%M:%S')] review @ $review_at, $fresh_count fresh Copilot inline comment(s) — iterating"
  sleep "$POLL_INTERVAL"
done
