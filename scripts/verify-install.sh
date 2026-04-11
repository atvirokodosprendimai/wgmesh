#!/usr/bin/env bash
# verify-install.sh — smoke-test the install paths documented in docs/quickstart.md.
# Usage: bash scripts/verify-install.sh
# Requirements: go >= 1.23 in PATH, run from the repository root.
set -euo pipefail

PASS=0
FAIL=0

pass() { echo "  PASS: $*"; ((PASS++)); }
fail() { echo "  FAIL: $*"; ((FAIL++)); }

echo "=== wgmesh install verification ==="
echo

# ── 1. build from source ────────────────────────────────────────────────────
echo "[1/4] Build from source (go build)"
BIN="$(mktemp -d)/wgmesh"
if go build -o "$BIN" . 2>/dev/null; then
  pass "go build succeeded"
else
  fail "go build failed"; FAIL=$((FAIL+1))
fi

# ── 2. wgmesh version ───────────────────────────────────────────────────────
echo "[2/4] wgmesh version"
if "$BIN" version 2>&1 | grep -qiE 'wgmesh|version'; then
  pass "wgmesh version printed expected output"
else
  fail "wgmesh version output did not contain 'wgmesh' or 'version'"
fi

# ── 3. wgmesh init --secret (no network, no root) ───────────────────────────
echo "[3/4] wgmesh init --secret"
SECRET_OUT=$("$BIN" init --secret 2>&1 || true)
if echo "$SECRET_OUT" | grep -qE 'wgmesh://v1/'; then
  pass "wgmesh init --secret printed a wgmesh://v1/ secret"
else
  fail "wgmesh init --secret did not print a wgmesh://v1/ secret. Output: $SECRET_OUT"
fi

# ── 4. wgmesh status (with a valid secret, no root) ─────────────────────────
echo "[4/4] wgmesh status (derived params, no network)"
SECRET=$(echo "$SECRET_OUT" | grep -oE 'wgmesh://v1/[A-Za-z0-9+/=_-]+' | head -1)
if [ -z "$SECRET" ]; then
  fail "Could not extract secret from step 3 output — skipping status check"
else
  STATUS_OUT=$("$BIN" status --secret "$SECRET" 2>&1 || true)
  if echo "$STATUS_OUT" | grep -qiE 'subnet|mesh|rendezvous|network'; then
    pass "wgmesh status printed derived mesh parameters"
  else
    fail "wgmesh status output did not contain expected fields. Output: $STATUS_OUT"
  fi
fi

# ── summary ─────────────────────────────────────────────────────────────────
echo
echo "Results: $PASS passed, $FAIL failed"
if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
