#!/usr/bin/env bash
# Tests for pr-review-merge.sh
# Run: bash company/scripts/test-pr-review-merge.sh
# shellcheck disable=SC2016  # Mock gh scripts use single quotes intentionally
set -euo pipefail

PASS=0
FAIL=0
TMPDIR=$(mktemp -d)
ORIG_PATH="$PATH"

cleanup() {
  PATH="$ORIG_PATH"
  rm -rf "$TMPDIR"
}
trap cleanup EXIT

# ── Pre-flight checks (TEST-3) ───────────────────────────────────

for cmd in jq bash; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "FAIL: Required tool '$cmd' not found"
    exit 1
  fi
done

# ── Helpers ───────────────────────────────────────────────────────

assert_eq() {
  local desc="$1" expected="$2" actual="$3"
  if [[ "$expected" == "$actual" ]]; then
    echo "  PASS: $desc"
    PASS=$((PASS + 1))
  else
    echo "  FAIL: $desc (expected '$expected', got '$actual')"
    FAIL=$((FAIL + 1))
  fi
}

assert_contains() {
  local desc="$1" needle="$2" haystack="$3"
  if echo "$haystack" | grep -qF "$needle"; then
    echo "  PASS: $desc"
    PASS=$((PASS + 1))
  else
    echo "  FAIL: $desc (expected to contain '$needle')"
    FAIL=$((FAIL + 1))
  fi
}

assert_not_contains() {
  local desc="$1" needle="$2" haystack="$3"
  if echo "$haystack" | grep -qF "$needle"; then
    echo "  FAIL: $desc (should NOT contain '$needle')"
    FAIL=$((FAIL + 1))
  else
    echo "  PASS: $desc"
    PASS=$((PASS + 1))
  fi
}

# ── Test environment setup ────────────────────────────────────────

REAL_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_AUDIT_LOG="$TMPDIR/audit-log.jsonl"

# Create a no-op sanitise.sh for tests
mkdir -p "$TMPDIR/scripts"
cat > "$TMPDIR/scripts/sanitise.sh" <<'SANITISE'
#!/usr/bin/env bash
cat
SANITISE
chmod +x "$TMPDIR/scripts/sanitise.sh"

# Source the functions we need by creating a testable version
# that skips the require-env-vars block and main() call
setup_test_env() {
  export PR_NUMBER="42"
  export TARGET_REPO="owner/repo"
  export GH_TOKEN="test-token"
  export GITHUB_RUN_ID="test-run-123"

  # Reset state — these variables are consumed by eval'd functions from pr-review-merge.sh
  # shellcheck disable=SC2034
  ERRORS=0
  # shellcheck disable=SC2034
  AUDIT_LOG="$TEST_AUDIT_LOG"
  # shellcheck disable=SC2034
  SCRIPT_DIR="$TMPDIR/scripts"
  # shellcheck disable=SC2034
  RUN_ID="test-run-123"
  # shellcheck disable=SC2034
  PR_MAX_LINES="500"
  # shellcheck disable=SC2034
  MAX_RETRY_COUNT="3"
  # shellcheck disable=SC2034
  APPROVED_AUTHORS="copilot-swe-agent[bot],goose[bot]"
  # shellcheck disable=SC2034
  POLL_INTERVAL="0"
  # shellcheck disable=SC2034
  POLL_MAX_ATTEMPTS="1"
  # shellcheck disable=SC2034
  REVIEW_WINDOWS="1"
  # shellcheck disable=SC2034
  PROTECTED_PATHS=".github/,company/scripts/"
  # shellcheck disable=SC2034
  SECURITY_KEYWORDS="secret,token,key,password,api_key,private_key"
  # shellcheck disable=SC2034
  START_TIME=$(date +%s)

  # Clear audit log
  : > "$TEST_AUDIT_LOG"
}

# Create mock gh script in TMPDIR/bin
create_mock_gh() {
  local mock_script="$1"
  mkdir -p "$TMPDIR/bin"
  cat > "$TMPDIR/bin/gh" <<MOCK
#!/usr/bin/env bash
${mock_script}
MOCK
  chmod +x "$TMPDIR/bin/gh"
  export PATH="$TMPDIR/bin:$ORIG_PATH"
}

# Source script functions (extract everything between set -euo and main "$@")
# We source by running in a subshell with the env vars set
source_functions() {
  setup_test_env
  # Source the function definitions from the script
  eval "$(sed -n '/^log_audit()/,/^main "\$@"$/p' "$REAL_SCRIPT_DIR/pr-review-merge.sh" | head -n -1 | sed '/^main()/,$ { /^main()/d; /^}$/d; d; }')"
}

# Extract individual functions from the script for sourcing
eval_functions() {
  # Extract all function definitions (log_audit through check_manual_push)
  eval "$(awk '/^log_audit\(\)|^escalate\(\)|^check_circuit_breaker\(\)|^check_manual_only\(\)|^poll_for_review\(\)|^check_inline_comments\(\)|^run_guardrails\(\)|^reassign_agent\(\)|^merge_pr\(\)|^check_manual_push\(\)/{found=1} found{print} found && /^}$/{found=0}' "$REAL_SCRIPT_DIR/pr-review-merge.sh")"
}

# ── Tests ─────────────────────────────────────────────────────────

echo "=== pr-review-merge.sh tests ==="

# --- Test 1: log_audit ---
echo ""
echo "1. test_log_audit"
setup_test_env
eval_functions
log_audit "test_action" "test details"
entry=$(tail -1 "$TEST_AUDIT_LOG")
assert_contains "has action field" '"action":"test_action"' "$entry"
assert_contains "has details field" '"details":"test details"' "$entry"
assert_contains "has pr_number field" '"pr_number":"42"' "$entry"
assert_contains "has run_id field" '"run_id":"test-run-123"' "$entry"
assert_contains "has timestamp field" '"timestamp":' "$entry"

# --- Test 2: escalate ---
echo ""
echo "2. test_escalate"
setup_test_env
create_mock_gh 'exit 0'  # All gh commands succeed
eval_functions
output=$(escalate 123 "test reason" 2>&1)
assert_contains "produces warning annotation" "::warning::Escalating PR #123: test reason" "$output"
# Note: ERRORS increment happens inside subshell (command substitution), so
# we verify via the warning annotation output instead
assert_contains "warning annotation present" "::warning::" "$output"
entry=$(grep '"escalated"' "$TEST_AUDIT_LOG" | tail -1)
assert_contains "audit has escalated action" '"action":"escalated"' "$entry"

# --- Test 3: circuit breaker ---
echo ""
echo "3. test_circuit_breaker"
exit_code=0
bash -c '
  export PR_NUMBER="42" TARGET_REPO="owner/repo" GH_TOKEN="test" GITHUB_RUN_ID="test"
  ERRORS=0; AUDIT_LOG="/dev/null"; RUN_ID="test"; SCRIPT_DIR="'"$TMPDIR/scripts"'"
  PR_MAX_LINES=500; MAX_RETRY_COUNT=3; POLL_INTERVAL=0; POLL_MAX_ATTEMPTS=1
  APPROVED_AUTHORS="copilot-swe-agent[bot],goose[bot]"; REVIEW_WINDOWS=1
  PROTECTED_PATHS=".github/,company/scripts/"; SECURITY_KEYWORDS="secret"
  eval "$(awk '"'"'/^log_audit\(\)|^check_circuit_breaker\(\)/{found=1} found{print} found && /^}$/{found=0}'"'"' '"$REAL_SCRIPT_DIR/pr-review-merge.sh"')"
  ERRORS=5
  check_circuit_breaker
' 2>&1 || exit_code=$?
assert_eq "circuit breaker exits 1" "1" "$exit_code"

# --- Test 4: check_manual_only — label present ---
echo ""
echo "4. test_check_manual_only_present"
setup_test_env
create_mock_gh '
if [[ "$1" == "pr" && "$2" == "view" ]]; then
  echo "manual-only"
  exit 0
fi
exit 1
'
eval_functions
check_manual_only 42
exit_code=$?
assert_eq "returns 0 when manual-only present" "0" "$exit_code"
entry=$(grep '"skipped"' "$TEST_AUDIT_LOG" | tail -1)
assert_contains "audit logs skipped" '"action":"skipped"' "$entry"

# --- Test 5: check_manual_only — label absent ---
echo ""
echo "5. test_check_manual_only_absent"
setup_test_env
create_mock_gh '
if [[ "$1" == "pr" && "$2" == "view" ]]; then
  echo "enhancement"
  echo "bug"
  exit 0
fi
exit 1
'
eval_functions
exit_code=0
check_manual_only 42 || exit_code=$?
assert_eq "returns 1 when manual-only absent" "1" "$exit_code"

# --- Test 6: guardrail — author rejected ---
echo ""
echo "6. test_guardrail_author_rejected"
setup_test_env
create_mock_gh '
if [[ "$1" == "pr" && "$2" == "view" && "$*" == *"author"* ]]; then
  echo "unknown-user"
  exit 0
fi
if [[ "$1" == "pr" && "$2" == "edit" ]]; then exit 0; fi
if [[ "$1" == "pr" && "$2" == "comment" ]]; then exit 0; fi
exit 0
'
eval_functions
exit_code=0
run_guardrails || exit_code=$?
assert_eq "guardrail rejects unknown author" "1" "$exit_code"
assert_contains "audit has escalated" '"escalated"' "$(cat "$TEST_AUDIT_LOG")"

# --- Test 7: guardrail — author approved ---
echo ""
echo "7. test_guardrail_author_approved"
setup_test_env
create_mock_gh '
if [[ "$1" == "pr" && "$2" == "view" && "$*" == *"author"* ]]; then
  echo "copilot-swe-agent[bot]"
  exit 0
fi
if [[ "$1" == "pr" && "$2" == "view" && "$*" == *"files"* ]]; then
  echo "src/app.ts"
  exit 0
fi
if [[ "$1" == "pr" && "$2" == "view" && "$*" == *"additions"* ]]; then
  echo "50"
  exit 0
fi
if [[ "$1" == "pr" && "$2" == "diff" ]]; then
  echo "+console.log(hello)"
  exit 0
fi
if [[ "$1" == "pr" && "$2" == "view" && "$*" == *"headRefOid"* ]]; then
  echo "abc123"
  exit 0
fi
if [[ "$1" == "api" && "$*" == *"check-runs"* ]]; then
  echo "0"
  exit 0
fi
exit 0
'
eval_functions
exit_code=0
run_guardrails || exit_code=$?
assert_eq "guardrail passes approved author" "0" "$exit_code"

# --- Test 8: guardrail — protected path ---
echo ""
echo "8. test_guardrail_protected_path"
setup_test_env
create_mock_gh '
if [[ "$1" == "pr" && "$2" == "view" && "$*" == *"author"* ]]; then
  echo "copilot-swe-agent[bot]"
  exit 0
fi
if [[ "$1" == "pr" && "$2" == "view" && "$*" == *"files"* ]]; then
  echo ".github/workflows/ci.yml"
  exit 0
fi
if [[ "$1" == "pr" && "$2" == "edit" ]]; then exit 0; fi
if [[ "$1" == "pr" && "$2" == "comment" ]]; then exit 0; fi
exit 0
'
eval_functions
exit_code=0
run_guardrails || exit_code=$?
assert_eq "guardrail rejects protected path" "1" "$exit_code"

# --- Test 9: guardrail — size exceeded ---
echo ""
echo "9. test_guardrail_size_exceeded"
setup_test_env
create_mock_gh '
if [[ "$1" == "pr" && "$2" == "view" && "$*" == *"author"* ]]; then
  echo "copilot-swe-agent[bot]"
  exit 0
fi
if [[ "$1" == "pr" && "$2" == "view" && "$*" == *"files"* ]]; then
  echo "src/big-file.ts"
  exit 0
fi
if [[ "$1" == "pr" && "$2" == "view" && "$*" == *"additions"* ]]; then
  echo "600"
  exit 0
fi
if [[ "$1" == "pr" && "$2" == "edit" ]]; then exit 0; fi
if [[ "$1" == "pr" && "$2" == "comment" ]]; then exit 0; fi
exit 0
'
eval_functions
exit_code=0
run_guardrails || exit_code=$?
assert_eq "guardrail rejects oversized PR" "1" "$exit_code"

# --- Test 10: guardrail — security keyword ---
echo ""
echo "10. test_guardrail_security_keyword"
setup_test_env
create_mock_gh '
if [[ "$1" == "pr" && "$2" == "view" && "$*" == *"author"* ]]; then
  echo "copilot-swe-agent[bot]"
  exit 0
fi
if [[ "$1" == "pr" && "$2" == "view" && "$*" == *"files"* ]]; then
  echo "src/config.ts"
  exit 0
fi
if [[ "$1" == "pr" && "$2" == "view" && "$*" == *"additions"* ]]; then
  echo "30"
  exit 0
fi
if [[ "$1" == "pr" && "$2" == "diff" ]]; then
  echo "+++ a/src/config.ts"
  echo "+const api_key = process.env.KEY"
  exit 0
fi
if [[ "$1" == "pr" && "$2" == "edit" ]]; then exit 0; fi
if [[ "$1" == "pr" && "$2" == "comment" ]]; then exit 0; fi
exit 0
'
eval_functions
exit_code=0
run_guardrails || exit_code=$?
assert_eq "guardrail rejects security keyword" "1" "$exit_code"

# --- Test 11: guardrail — all pass ---
echo ""
echo "11. test_guardrail_all_pass"
setup_test_env
create_mock_gh '
if [[ "$1" == "pr" && "$2" == "view" && "$*" == *"author"* ]]; then
  echo "goose[bot]"
  exit 0
fi
if [[ "$1" == "pr" && "$2" == "view" && "$*" == *"files"* ]]; then
  echo "src/fix.ts"
  exit 0
fi
if [[ "$1" == "pr" && "$2" == "view" && "$*" == *"additions"* ]]; then
  echo "42"
  exit 0
fi
if [[ "$1" == "pr" && "$2" == "diff" ]]; then
  echo "+++ a/src/fix.ts"
  echo "+console.log(fixed)"
  exit 0
fi
if [[ "$1" == "pr" && "$2" == "view" && "$*" == *"headRefOid"* ]]; then
  echo "def456"
  exit 0
fi
if [[ "$1" == "api" && "$*" == *"check-runs"* ]]; then
  echo "0"
  exit 0
fi
exit 0
'
eval_functions
exit_code=0
run_guardrails || exit_code=$?
assert_eq "all guardrails pass" "0" "$exit_code"
assert_contains "audit logs guardrails_passed" '"guardrails_passed"' "$(cat "$TEST_AUDIT_LOG")"

# --- Test 12: check_manual_push — bot commit ---
echo ""
echo "12. test_check_manual_push_bot"
setup_test_env
create_mock_gh '
if [[ "$1" == "api" && "$*" == *"commits"* ]]; then
  echo "copilot-swe-agent[bot]"
  exit 0
fi
exit 0
'
eval_functions
exit_code=0
check_manual_push 42 || exit_code=$?
assert_eq "returns 1 for bot commit" "1" "$exit_code"

# --- Test 13: check_manual_push — human commit ---
echo ""
echo "13. test_check_manual_push_human"
setup_test_env
create_mock_gh '
if [[ "$1" == "api" && "$*" == *"commits"* ]]; then
  echo "human-dev"
  exit 0
fi
exit 0
'
eval_functions
exit_code=0
check_manual_push 42 || exit_code=$?
assert_eq "returns 0 for human commit" "0" "$exit_code"
assert_contains "audit logs manual_push" '"manual_push"' "$(cat "$TEST_AUDIT_LOG")"

# ── Summary (TEST-2) ─────────────────────────────────────────────

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
