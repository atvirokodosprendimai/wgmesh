#!/usr/bin/env bash
# goose-build-context.sh — Generate codebase type context for Goose.
#
# Usage: ./company/scripts/goose-build-context.sh [output-file]
#
# Scans pkg/*/ for exported Go symbols and writes a reference file.
# Goose reads this during implementation to avoid inventing types.
#
# Optional environment:
#   MEMORY_FILE  — path to memory context file (appended to output)

set -euo pipefail

OUTPUT_FILE="${1:-/tmp/codebase-context.md}"
REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

{
  echo "# Codebase Type Reference"
  echo ""
  echo "These are the ACTUAL exported types, functions, and constants in each package."
  echo "You MUST use these exact names — do NOT invent types that don't exist here."
  echo ""

  if [ -d "$REPO_ROOT/pkg" ]; then
    for pkg in "$REPO_ROOT"/pkg/*/; do
      pkg_name=$(basename "$pkg")
      echo "## Package: $pkg_name"
      echo '```go'
      grep -rn '^type \|^func \|^const \|^var ' "$pkg"*.go 2>/dev/null \
        | grep -v '_test.go' \
        | grep -v '//' \
        | sed 's|^.*/||' \
        || echo "// no exported symbols"
      echo '```'
      echo ""
    done
  fi

  # Memory context (optional, provided by CI mem0 retrieval)
  if [ -n "${MEMORY_FILE:-}" ] && [ -s "$MEMORY_FILE" ]; then
    echo "## Memory from Past Runs"
    echo ""
    cat "$MEMORY_FILE"
    echo ""
  fi
} > "$OUTPUT_FILE"

echo "Context written to $OUTPUT_FILE ($(wc -l < "$OUTPUT_FILE") lines)"
