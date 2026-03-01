#!/usr/bin/env bash
# Collect GitHub signals for the company control loop.
# Requires: GITHUB_TOKEN env var (provided by Actions, or PAT).
# Output: JSON to stdout.
set -euo pipefail

REPO="${GITHUB_REPOSITORY:-atvirokodosprendimai/wgmesh}"
API="https://api.github.com"
AUTH=""
if [ -n "${GITHUB_TOKEN:-}" ]; then
  AUTH="-H Authorization: token $GITHUB_TOKEN"
fi

gh_get() {
  curl -sf $AUTH -H "Accept: application/vnd.github+json" "$API/$1" 2>/dev/null || echo "{}"
}

# Issues by function label
fn_labels=("fn:dev" "fn:ops" "fn:gtm" "fn:billing" "fn:support" "fn:legal" "needs-human")
issues_by_label="{}"
for label in "${fn_labels[@]}"; do
  count=$(gh_get "repos/$REPO/issues?labels=$(echo "$label" | sed 's/:/%3A/g')&state=open&per_page=1" | jq 'if type == "array" then length else 0 end')
  issues_by_label=$(echo "$issues_by_label" | jq --arg l "$label" --argjson c "${count:-0}" '. + {($l): $c}')
done

# Open PRs
open_prs=$(gh_get "repos/$REPO/pulls?state=open&per_page=100" | jq 'if type == "array" then length else 0 end')

# Merge rate (last 7 days)
since=$(date -u -v-7d '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || date -u -d '7 days ago' '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || echo "")
merged_prs=0
if [ -n "$since" ]; then
  merged_prs=$(gh_get "repos/$REPO/pulls?state=closed&sort=updated&direction=desc&per_page=100" | \
    jq --arg since "$since" '[.[] | select(.merged_at != null and .merged_at > $since)] | length')
fi

# Latest release
latest_release=$(gh_get "repos/$REPO/releases/latest" | jq -r '.tag_name // "none"')

# CI status (latest workflow run on default branch)
ci_status=$(gh_get "repos/$REPO/actions/runs?branch=main&per_page=1" | jq -r '.workflow_runs[0].conclusion // "unknown"')

# Stars and forks
repo_info=$(gh_get "repos/$REPO")
stars=$(echo "$repo_info" | jq '.stargazers_count // 0')
forks=$(echo "$repo_info" | jq '.forks_count // 0')
open_issues_total=$(echo "$repo_info" | jq '.open_issues_count // 0')

# Recent contributors (last 30 days via commit activity)
recent_contributors=$(gh_get "repos/$REPO/commits?per_page=100" | \
  jq '[.[].author.login // .[].commit.author.name] | unique | length')

# Build JSON safely with jq
jq -n \
  --arg collected_at "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" \
  --argjson issues_by_label "$issues_by_label" \
  --argjson open_prs "${open_prs:-0}" \
  --argjson merged_prs "${merged_prs:-0}" \
  --arg latest_release "${latest_release:-none}" \
  --arg ci_status "${ci_status:-unknown}" \
  --argjson stars "${stars:-0}" \
  --argjson forks "${forks:-0}" \
  --argjson open_issues "${open_issues_total:-0}" \
  --argjson contributors "${recent_contributors:-0}" \
  '{
    source: "github",
    collected_at: $collected_at,
    issues_by_function_label: $issues_by_label,
    open_prs: $open_prs,
    merged_prs_7d: $merged_prs,
    latest_release: $latest_release,
    ci_status: $ci_status,
    stars: $stars,
    forks: $forks,
    open_issues_total: $open_issues,
    recent_contributors_30d: $contributors
  }'
