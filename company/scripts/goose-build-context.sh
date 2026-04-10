#!/usr/bin/env bash
# Generate codebase context for Goose implementation runs.
# Lists all exported Go symbols so Goose knows what types/functions exist
# before writing code.
#
# Usage: bash company/scripts/goose-build-context.sh /tmp/codebase-context.md

set -euo pipefail

OUTPUT="${1:?Usage: goose-build-context.sh <output-file>}"

{
  echo "# wgmesh Codebase Context"
  echo ""
  echo "Generated at $(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  echo ""

  echo "## Module"
  echo ""
  head -1 go.mod
  echo ""

  echo "## Package Structure"
  echo ""
  echo '```'
  find pkg -type f -name "*.go" ! -name "*_test.go" 2>/dev/null | sort | sed 's|^|  |'
  [ -d cmd ] && find cmd -type f -name "*.go" ! -name "*_test.go" 2>/dev/null | sort | sed 's|^|  |'
  echo '```'
  echo ""

  echo "## Exported Symbols"
  echo ""
  for dir in $(find pkg -type d 2>/dev/null; [ -d cmd ] && find cmd -type d 2>/dev/null | sort); do
    gofiles=$(find "$dir" -maxdepth 1 -name "*.go" ! -name "*_test.go" 2>/dev/null)
    [ -z "$gofiles" ] && continue
    pkg=$(basename "$dir")
    echo "### $dir"
    echo '```go'
    # Extract exported type, func, const, var declarations
    grep -hE '^(type [A-Z]|func [A-Z]|func \([a-z*]+ \*?[A-Z][^ ]+\) [A-Z]|const [A-Z]|var [A-Z])' $gofiles 2>/dev/null | sort -u || true
    echo '```'
    echo ""
  done
} > "$OUTPUT"

echo "Codebase context written to $OUTPUT ($(wc -l < "$OUTPUT") lines)"
