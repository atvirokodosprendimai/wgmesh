#!/usr/bin/env bash
# goose-build-task.sh — Build a Goose task file from the recipe and a spec file.
#
# Usage: ./company/scripts/goose-build-task.sh <spec-file> [output-file]
#
# Reads the recipe YAML for prompt, context_files, and checks.
# Generates codebase type context and assembles everything into a task file.
#
# Requires: yq (https://github.com/mikefarah/yq)
#
# Environment:
#   RECIPE_FILE  — path to recipe YAML (default: .github/goose-recipes/wgmesh-implementation.yaml)
#   MEMORY_FILE  — optional memory context file to include

set -euo pipefail

SPEC_FILE="${1:?Usage: $0 <spec-file> [output-file]}"
OUTPUT_FILE="${2:-/tmp/goose-task.md}"
RECIPE_FILE="${RECIPE_FILE:-.github/goose-recipes/wgmesh-implementation.yaml}"
REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

if [ ! -f "$SPEC_FILE" ]; then
  echo "Error: spec file not found: $SPEC_FILE" >&2
  exit 1
fi

if [ ! -f "$RECIPE_FILE" ]; then
  echo "Error: recipe file not found: $RECIPE_FILE" >&2
  exit 1
fi

if ! command -v yq &>/dev/null; then
  echo "Error: yq is required but not found. Install: https://github.com/mikefarah/yq" >&2
  exit 1
fi

# ── Extract recipe fields ──────────────────────────────────────

prompt=$(yq -r '.prompt' "$RECIPE_FILE")
context_files=$(yq -r '.context_files // [] | .[]' "$RECIPE_FILE" 2>/dev/null || true)
checks=$(yq -r '.retry.checks[] | .command' "$RECIPE_FILE" 2>/dev/null || true)

# ── Build codebase type context ────────────────────────────────

codebase_context=""
if [ -d "$REPO_ROOT/pkg" ]; then
  codebase_context="## Codebase Type Reference

These are the ACTUAL exported types, functions, and constants in each package.
You MUST use these exact names — do NOT invent types that don't exist here.

"
  for pkg in "$REPO_ROOT"/pkg/*/; do
    pkg_name=$(basename "$pkg")
    codebase_context+="### Package: $pkg_name ($pkg)
\`\`\`go
"
    symbols=$(grep -rn '^type \|^func \|^const \|^var ' "$pkg"*.go 2>/dev/null \
      | grep -v '_test.go' \
      | grep -v '//' \
      | sed 's|^.*/||' || true)
    codebase_context+="${symbols:-// no exported symbols}
\`\`\`

"
  done
fi

# ── Assemble task file ─────────────────────────────────────────

{
  echo "# Implementation Task"
  echo ""
  echo "$prompt"
  echo ""

  # Codebase context
  if [ -n "$codebase_context" ]; then
    echo "$codebase_context"
  fi

  # Context files from recipe (e.g. .goosehints, AGENTS.md)
  for cf in $context_files; do
    if [ -f "$REPO_ROOT/$cf" ]; then
      echo "## $(basename "$cf")"
      echo ""
      cat "$REPO_ROOT/$cf"
      echo ""
    fi
  done

  # Memory context (optional, provided by CI)
  if [ -n "${MEMORY_FILE:-}" ] && [ -s "$MEMORY_FILE" ]; then
    echo "## Memory from Past Runs"
    echo ""
    cat "$MEMORY_FILE"
    echo ""
  fi

  # Specification
  echo "## Specification"
  echo ""
  cat "$SPEC_FILE"
  echo ""

  # Validation checklist (derived from recipe checks)
  echo "## Validation Checklist"
  echo ""
  echo "Follow these steps IN ORDER after implementation:"
  echo ""
  step=1
  while IFS= read -r cmd; do
    [ -z "$cmd" ] && continue
    echo "$step. Run \`$cmd\` — fix any errors, repeat until clean"
    step=$((step + 1))
  done <<< "$checks"
  echo "$step. Run \`gofmt -w .\` to fix formatting"
  echo ""
  echo "IMPORTANT RULES:"
  echo "- NEVER reference a type, field, or function that you haven't verified exists by reading the source"
  echo "- If the spec suggests code that doesn't match the real codebase, adapt to match reality"
  echo "- If the spec classification is \"wont-do\" or \"needs-info\", do NOT implement anything"

} > "$OUTPUT_FILE"

echo "Task file written to $OUTPUT_FILE ($(wc -l < "$OUTPUT_FILE") lines)"
