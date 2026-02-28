#!/usr/bin/env bash
# Collect contribution signals for the company control loop.
# Reads git log and existing contributor ledger.
# Output: JSON to stdout.
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

# Recent git contributors (last 7 days)
since=$(date -u -v-7d '+%Y-%m-%d' 2>/dev/null || date -u -d '7 days ago' '+%Y-%m-%d' 2>/dev/null || echo "2026-02-21")
recent_authors=$(git log --since="$since" --format='%aN' 2>/dev/null | sort -u | jq -R . | jq -s .)

# AI agent activity (count commits by bots in last 7 days)
bot_commits=$(git log --since="$since" --format='%aN' 2>/dev/null | grep -ciE 'copilot|goose|github-actions|bot' || echo 0)

# Direct dependencies count
dep_count=$(grep -cE '^\t[a-z].*// indirect$' "$REPO_ROOT/go.mod" 2>/dev/null || echo 0)
direct_dep_count=$(grep -cE '^\t[a-z]' "$REPO_ROOT/go.mod" 2>/dev/null || echo 0)
direct_dep_count=$((direct_dep_count - dep_count))

# Unreciprocated count from ledger
unreciprocated=0
if [ -f "$REPO_ROOT/company/contributors.json" ]; then
  unreciprocated=$(jq '.unreciprocated | length' "$REPO_ROOT/company/contributors.json" 2>/dev/null || echo 0)
fi

cat <<EOF
{
  "source": "contributions",
  "collected_at": "$(date -u '+%Y-%m-%dT%H:%M:%SZ')",
  "recent_git_authors_7d": $recent_authors,
  "bot_commits_7d": $bot_commits,
  "direct_dependencies": $direct_dep_count,
  "unreciprocated_count": $unreciprocated
}
EOF
