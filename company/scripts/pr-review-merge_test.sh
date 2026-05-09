#!/usr/bin/env bash
# Tests for pr-review-merge.sh review-state policy (issue #595).
#
# Strategy: source the main script with required env vars set + a stub
# `check_circuit_breaker` (so error counting in the helpers does not abort
# tests), then exercise the pure helper `evaluate_review_states` against
# canned input. `fetch_effective_review_states` wraps a single `gh api`
# call and is not unit-tested here — the policy logic lives in
# `evaluate_review_states`.
#
# Run: bash company/scripts/pr-review-merge_test.sh
# Exits 0 on success, 1 on first assertion failure.

set -euo pipefail

# Required env vars for the script's fail-fast guards.
export PR_NUMBER=999
export TARGET_REPO="atvirokodosprendimai/wgmesh"
export GH_TOKEN="dummy"
export RUN_ID="test-$$"
export AUDIT_LOG="/tmp/pr-review-merge_test-audit.jsonl"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/pr-review-merge.sh"

PASS=0
FAIL=0

assert_eq() {
  local got="$1" want="$2" name="$3"
  if [[ "$got" == "$want" ]]; then
    PASS=$((PASS + 1))
    echo "  PASS  $name"
  else
    FAIL=$((FAIL + 1))
    echo "  FAIL  $name"
    echo "        got:  ${got}"
    echo "        want: ${want}"
  fi
}

# Scenario 1: single APPROVED review → PASS.
out=$(printf 'copilot-pull-request-reviewer[bot]:APPROVED\n' | evaluate_review_states)
assert_eq "$out" "PASS" "single APPROVED review yields PASS"

# Scenario 2: single COMMENTED review → PENDING:NO_APPROVED.
# This is the bug class that shipped #577, #589, #596 — the prior code
# treated this as a green light. New policy must NOT pass.
out=$(printf 'copilot-pull-request-reviewer[bot]:COMMENTED\n' | evaluate_review_states)
assert_eq "$out" "PENDING:NO_APPROVED" "single COMMENTED review yields PENDING (regression guard for #577/#589/#596)"

# Scenario 3: latestReviews already deduplicates per reviewer; the helper
# only ever sees the most-recent state per login. Verify a bare APPROVED
# yields PASS regardless of any earlier COMMENTED that might have existed.
out=$(printf 'copilot-pull-request-reviewer[bot]:APPROVED\n' | evaluate_review_states)
assert_eq "$out" "PASS" "most-recent APPROVED yields PASS (latestReviews dedup contract)"

# Scenario 4: one APPROVED + one CHANGES_REQUESTED → BLOCKED.
out=$(printf 'reviewer-a:APPROVED\nreviewer-b:CHANGES_REQUESTED\n' | evaluate_review_states)
assert_eq "$out" "BLOCKED:CHANGES_REQUESTED" "any CHANGES_REQUESTED beats APPROVED"

# Scenario 5: no reviews → PENDING:NONE.
out=$(printf '' | evaluate_review_states)
assert_eq "$out" "PENDING:NONE" "no reviews yields PENDING:NONE"

# Scenario 6: only DISMISSED reviews → PENDING:NO_APPROVED.
# Dismissed reviews are intentionally not blocking, but they also do not
# count as approval.
out=$(printf 'reviewer-a:DISMISSED\n' | evaluate_review_states)
assert_eq "$out" "PENDING:NO_APPROVED" "DISMISSED-only yields PENDING:NO_APPROVED"

# Scenario 7: APPROVED + COMMENTED from different reviewers → PASS.
# COMMENTED is informational only; it neither approves nor blocks.
out=$(printf 'reviewer-a:APPROVED\nreviewer-b:COMMENTED\n' | evaluate_review_states)
assert_eq "$out" "PASS" "APPROVED + COMMENTED-from-different-reviewer yields PASS"

# Scenario 8: APPROVED + DISMISSED → PASS.
out=$(printf 'reviewer-a:APPROVED\nreviewer-b:DISMISSED\n' | evaluate_review_states)
assert_eq "$out" "PASS" "APPROVED + DISMISSED yields PASS"

# Scenario 9: APPROVED + CHANGES_REQUESTED + COMMENTED → BLOCKED wins.
out=$(printf 'reviewer-a:APPROVED\nreviewer-b:CHANGES_REQUESTED\nreviewer-c:COMMENTED\n' | evaluate_review_states)
assert_eq "$out" "BLOCKED:CHANGES_REQUESTED" "BLOCKED wins regardless of mix"

echo ""
echo "Results: ${PASS} passed, ${FAIL} failed"
[[ "$FAIL" -eq 0 ]] || exit 1
