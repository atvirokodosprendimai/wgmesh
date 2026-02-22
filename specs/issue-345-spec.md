# Spec: Issue #345 - Fix Spec Auto-Approve Validation Workflow

## Classification

**Bug Fix** — The spec-auto-approve workflow has validation logic bugs causing valid spec PRs to fail.

## Problem Analysis

### Current Behavior

Valid spec PRs with:
- Title: `spec: Issue #N - description`
- File: `specs/issue-N-spec.md`
- Required sections: Classification, Problem Analysis, Proposed Approach
- No code changes

Are failing validation with false positives:
- "Spec file not found" (when file exists)
- "Missing required section: ## Classification" (when section exists)
- "PR contains non-spec file changes" (when only spec file changed)

### Root Cause

The workflow has inverted/broken logic:

1. **Spec file detection**: Sets `CHECK_SPEC_EXISTS=false` initially, then finds the file but still reports "not found" due to race condition in shell logic.

2. **Section detection**: Uses `if grep -q; then` incorrectly - when grep finds match, the if block executes (marking found as missing).

3. **Code file detection**: Uses `git diff origin/main...HEAD` which compares wrong baseline.

## Proposed Approach

### Layer 1: Simplify Validation Script

Replace complex shell with simple verifiable script:

```bash
#!/bin/bash
set -e

TITLE="$1"
ISSUE_NUM=$(echo "$TITLE" | grep -oP 'Issue #\K\d+' || true)
[ -n "$ISSUE_NUM" ] || { echo "Error: No issue number"; exit 1; }

SPEC_FILE="specs/issue-${ISSUE_NUM}-spec.md"
[ -f "$SPEC_FILE" ] || { echo "Error: Spec not found"; exit 1; }

# Check only spec files changed
git diff --name-only HEAD~1 | while read f; do
    [[ "$f" =~ ^specs/ ]] || [[ "$f" =~ ^\.github/workflows/ ]] || { echo "Error: Non-spec: $f"; exit 1; }
done

# Check required sections
for section in "## Classification" "## Problem Analysis" "## Proposed Approach"; do
    grep -q "$section" "$SPEC_FILE" || { echo "Error: Missing $section"; exit 1; }
done

echo "Validation passed"
```

### Layer 2: Add Smoke Test

Create a test that verifies the workflow itself works:

1. Create a dummy test spec file
2. Run validation on it
3. Alert if validation fails for known-good input

### Layer 3: Remove Self-Approval

- Disable auto-approval until workflow is stable
- Require manual review for spec PRs
- Re-enable auto-approve after smoke test passes

## Test Strategy

1. **Unit test** the validation script with known-good and known-bad inputs
2. **Smoke test**: Run on schedule to verify workflow works
3. **Monitor**: Alert on validation failures

## Affected Files

- `.github/workflows/spec-auto-approve.yml` — Fix validation logic

## Estimated Complexity

**Low** — Simple bash script replacement.
