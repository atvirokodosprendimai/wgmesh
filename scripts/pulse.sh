#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

WINDOW="${WINDOW:-}"
POLAR_TOKEN="${POLAR_TOKEN:-}"
COROOT_API_TOKEN="${COROOT_API_TOKEN:-}"
GITHUB_TOKEN="${GITHUB_TOKEN:-}"
GH_REPO="${GH_REPO:-}"

if [[ -z "$WINDOW" ]]; then
	echo "error: WINDOW is required (examples: 24h, 7d)" >&2
	exit 1
fi

TMP_FILES=()
cleanup() {
	local file
	for file in "${TMP_FILES[@]}"; do
		[[ -n "$file" && -e "$file" ]] && rm -f "$file"
	done
}
trap cleanup EXIT

mktemp_tracked() {
	local file
	file="$(mktemp)"
	TMP_FILES+=("$file")
	printf '%s\n' "$file"
}

sanitize_reason() {
	printf '%s' "$1" | tr '\n' ' ' | sed -E 's/[[:space:]]+/ /g; s/^ //; s/ $//' | cut -c 1-160
}

parse_window_seconds() {
	local raw="$1"
	local amount unit
	unit="${raw: -1}"
	amount="${raw%?}"
	if ! [[ "$amount" =~ ^[0-9]+$ ]] || [[ "$amount" -le 0 ]]; then
		echo "error: WINDOW must be a positive Nh or Nd value, got: $raw" >&2
		exit 1
	fi
	case "$unit" in
		h) printf '%s\n' $((amount * 60 * 60)) ;;
		d) printf '%s\n' $((amount * 24 * 60 * 60)) ;;
		*)
			echo "error: WINDOW must end with h or d, got: $raw" >&2
			exit 1
			;;
	esac
}

date_iso() {
	local epoch="$1"
	if date -u -d "@0" '+%Y-%m-%dT%H:%M:%SZ' >/dev/null 2>&1; then
		date -u -d "@$epoch" '+%Y-%m-%dT%H:%M:%SZ'
	else
		date -u -r "$epoch" '+%Y-%m-%dT%H:%M:%SZ'
	fi
}

date_human() {
	local epoch="$1"
	if date -u -d "@0" '+%Y-%m-%d %H:%M UTC' >/dev/null 2>&1; then
		date -u -d "@$epoch" '+%Y-%m-%d %H:%M UTC'
	else
		date -u -r "$epoch" '+%Y-%m-%d %H:%M UTC'
	fi
}

date_stamp() {
	local epoch="$1"
	if date -u -d "@0" '+%Y-%m-%d_%H-%M' >/dev/null 2>&1; then
		date -u -d "@$epoch" '+%Y-%m-%d_%H-%M'
	else
		date -u -r "$epoch" '+%Y-%m-%d_%H-%M'
	fi
}

iso_to_epoch() {
	local iso="$1"
	local normalized
	normalized="$(printf '%s' "$iso" | sed -E 's/\.[0-9]+Z$/Z/')"
	if date -u -d "$normalized" '+%s' >/dev/null 2>&1; then
		date -u -d "$normalized" '+%s'
	else
		date -j -u -f '%Y-%m-%dT%H:%M:%SZ' "$normalized" '+%s'
	fi
}

WINDOW_SECONDS="$(parse_window_seconds "$WINDOW")"
NOW_EPOCH="$(date -u '+%s')"
END_EPOCH=$((NOW_EPOCH - 15 * 60))
START_EPOCH=$((END_EPOCH - WINDOW_SECONDS))
PRIOR_END_EPOCH="$START_EPOCH"
PRIOR_START_EPOCH=$((START_EPOCH - WINDOW_SECONDS))

START="$(date_iso "$START_EPOCH")"
END="$(date_iso "$END_EPOCH")"
PRIOR_START="$(date_iso "$PRIOR_START_EPOCH")"
PRIOR_END="$(date_iso "$PRIOR_END_EPOCH")"
REPORT_EPOCH="$NOW_EPOCH"
REPORT_STAMP="$(date_stamp "$REPORT_EPOCH")"
REPORT_HUMAN="$(date_human "$REPORT_EPOCH")"
REPORT_PATH="docs/pulse-reports/${REPORT_STAMP}.md"

CONFIG_FILE=".compound-engineering/config.local.yaml"
extract_config_value() {
	local key="$1"
	[[ -f "$CONFIG_FILE" ]] || return 0
	grep -E "^[[:space:]]*$key:" "$CONFIG_FILE" | head -n 1 |
		sed -E "s/^[^:]+:[[:space:]]*//; s/[[:space:]]+#.*$//; s/^['\"]//; s/['\"]$//; s/^[[:space:]]+//; s/[[:space:]]+$//"
}

csv_to_lines() {
	printf '%s\n' "$1" | tr ',' '\n' | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//' | sed '/^$/d'
}

PENDING_METRICS_RAW="$(extract_config_value pulse_pending_metrics || true)"
EXCLUDED_METRICS_RAW="$(extract_config_value pulse_excluded_metrics || true)"

is_csv_member() {
	local needle="$1"
	local item
	while IFS= read -r item; do
		[[ "$item" == "$needle" ]] && return 0
	done < <(csv_to_lines "$EXCLUDED_METRICS_RAW")
	return 1
}

slug_metric() {
	case "$1" in
		"Paying customers") printf '%s\n' "paying_customers" ;;
		"Cost coverage ratio") printf '%s\n' "cost_coverage_ratio" ;;
		"Weekly active meshes") printf '%s\n' "weekly_active_meshes" ;;
		"Time-to-mesh (p50)") printf '%s\n' "time_to_mesh_p50" ;;
		"Unsolicited positive feedback rate") printf '%s\n' "unsolicited_positive_feedback_rate" ;;
		*) printf '%s\n' "$1" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9]+/_/g; s/^_//; s/_$//' ;;
	esac
}

strategy_metrics() {
	[[ -f STRATEGY.md ]] || return 0
	awk '
		/^## Key metrics/ { in_section = 1; next }
		in_section && /^## / { exit }
		in_section && /^- / { print }
	' STRATEGY.md | sed -E 's/^- \*\*([^*]+)\*\*.*/\1/'
}

format_delta() {
	local current="$1"
	local prior="$2"
	local diff pct
	diff=$((current - prior))
	if [[ "$prior" -eq 0 ]]; then
		if [[ "$current" -eq 0 ]]; then
			printf '%s\n' "0 vs prior"
		else
			printf '+%s vs prior\n' "$diff"
		fi
	else
		pct="$(awk -v d="$diff" -v p="$prior" 'BEGIN { printf "%+.0f", (d / p) * 100 }')"
		printf '%+d, %s%% vs prior\n' "$diff" "$pct"
	fi
}

format_money_cents() {
	awk -v cents="$1" 'BEGIN { printf "$%.2f", cents / 100 }'
}

count_iso_lines_in_window() {
	local input="$1"
	local start_epoch="$2"
	local end_epoch="$3"
	local count=0
	local iso epoch
	while IFS= read -r iso; do
		[[ -z "$iso" ]] && continue
		if epoch="$(iso_to_epoch "$iso" 2>/dev/null)"; then
			if [[ "$epoch" -ge "$start_epoch" && "$epoch" -lt "$end_epoch" ]]; then
				count=$((count + 1))
			fi
		fi
	done <<< "$input"
	printf '%s\n' "$count"
}

QUERY_FAILURES=()

POLAR_CREATED_RENDER="no data (not queried)"
POLAR_CANCELLED_RENDER="no data (not queried)"
PAYING_CUSTOMERS_RENDER="no data (not queried)"
POLAR_HEADLINE="Polar: no data (not queried)."

query_polar() {
	if [[ -z "$POLAR_TOKEN" ]]; then
		POLAR_CREATED_RENDER="no data (token missing)"
		POLAR_CANCELLED_RENDER="no data (token missing)"
		PAYING_CUSTOMERS_RENDER="no data (token missing)"
		POLAR_HEADLINE="Polar: no data (token missing)."
		return 0
	fi
	if ! command -v jq >/dev/null 2>&1; then
		QUERY_FAILURES+=("polar: query failed: jq not found")
		POLAR_CREATED_RENDER="no data (query failed: jq not found)"
		POLAR_CANCELLED_RENDER="no data (query failed: jq not found)"
		PAYING_CUSTOMERS_RENDER="no data (query failed: jq not found)"
		POLAR_HEADLINE="Polar: no data (query failed: jq not found)."
		return 0
	fi

	local all_pages body created prior_created cancelled prior_cancelled active mrr_cents next pages reason result url
	all_pages="$(mktemp_tracked)"
	url="https://api.polar.sh/v1/subscriptions?limit=100"
	pages=0
	while [[ -n "$url" && "$pages" -lt 5 ]]; do
		if ! body="$(curl -fsS -H "Authorization: Bearer $POLAR_TOKEN" "$url" 2>&1)"; then
			reason="$(sanitize_reason "$body")"
			QUERY_FAILURES+=("polar: query failed: $reason")
			POLAR_CREATED_RENDER="no data (query failed: $reason)"
			POLAR_CANCELLED_RENDER="no data (query failed: $reason)"
			PAYING_CUSTOMERS_RENDER="no data (query failed: $reason)"
			POLAR_HEADLINE="Polar: no data (query failed)."
			return 0
		fi
		printf '%s\n' "$body" >> "$all_pages"
		next="$(printf '%s' "$body" | jq -r '.pagination.next_page_url // .pagination.next // empty' 2>/dev/null || true)"
		url="$next"
		pages=$((pages + 1))
	done

	if ! result="$(jq -s -r \
		--argjson start "$START_EPOCH" \
		--argjson end "$END_EPOCH" \
		--argjson prior_start "$PRIOR_START_EPOCH" \
		--argjson prior_end "$PRIOR_END_EPOCH" '
		def ts:
			if . == null then null
			else (sub("\\.[0-9]+Z$"; "Z") | fromdateiso8601?)
			end;
		def cents:
			(.amount // .price_amount // .recurring_amount // .price.amount // 0) | tonumber? // 0;
		[.[] | (.items? // .data? // .subscriptions? // [])[]] as $subs |
		[
			($subs | map(select((.created_at | ts) as $t | $t != null and $t >= $start and $t < $end)) | length),
			($subs | map(select((.created_at | ts) as $t | $t != null and $t >= $prior_start and $t < $prior_end)) | length),
			($subs | map(select((.ended_at | ts) as $t | $t != null and $t >= $start and $t < $end)) | length),
			($subs | map(select((.ended_at | ts) as $t | $t != null and $t >= $prior_start and $t < $prior_end)) | length),
			($subs | map(select(.status == "active")) | length),
			($subs | map(select(.status == "active") | cents) | add // 0)
		] | @tsv
	' "$all_pages" 2>&1)"; then
		reason="$(sanitize_reason "$result")"
		QUERY_FAILURES+=("polar: query failed: $reason")
		POLAR_CREATED_RENDER="no data (query failed: $reason)"
		POLAR_CANCELLED_RENDER="no data (query failed: $reason)"
		PAYING_CUSTOMERS_RENDER="no data (query failed: $reason)"
		POLAR_HEADLINE="Polar: no data (query failed)."
		return 0
	fi

	IFS=$'\t' read -r created prior_created cancelled prior_cancelled active mrr_cents <<< "$result"
	POLAR_CREATED_RENDER="${created} ($(format_delta "$created" "$prior_created"))"
	POLAR_CANCELLED_RENDER="${cancelled} ($(format_delta "$cancelled" "$prior_cancelled"))"
	PAYING_CUSTOMERS_RENDER="${active} active subscriptions ($(format_money_cents "$mrr_cents") MRR)"
	POLAR_HEADLINE="Polar: ${created} subscriptions created, ${cancelled} cancelled; ${active} active subscriptions."
}

COROOT_RENDER="no data (not queried)"
COROOT_PROJECT_RENDER="no data (not queried)"

query_coroot_probe() {
	# TODO: Replace the project-list probe with Coroot latency/error queries once the endpoint shape is finalized.
	if [[ -z "$COROOT_API_TOKEN" ]]; then
		COROOT_RENDER="no data (token missing)"
		COROOT_PROJECT_RENDER="no data (token missing)"
		return 0
	fi
	if ! command -v jq >/dev/null 2>&1; then
		QUERY_FAILURES+=("coroot: query failed: jq not found")
		COROOT_RENDER="no data (query failed: jq not found)"
		COROOT_PROJECT_RENDER="no data (query failed: jq not found)"
		return 0
	fi

	local body count reason
	if ! body="$(curl -fsS -m 10 -H "Authorization: Bearer $COROOT_API_TOKEN" "https://table.beerpub.dev/api/projects" 2>&1)"; then
		reason="$(sanitize_reason "$body")"
		QUERY_FAILURES+=("coroot: query failed: $reason")
		COROOT_RENDER="no data (query failed: $reason)"
		COROOT_PROJECT_RENDER="no data (query failed: $reason)"
		return 0
	fi
	if ! count="$(printf '%s' "$body" | jq -r 'if type == "array" then length elif .projects then (.projects | length) elif .items then (.items | length) else 0 end' 2>&1)"; then
		reason="$(sanitize_reason "$count")"
		QUERY_FAILURES+=("coroot: query failed: $reason")
		COROOT_RENDER="no data (query failed: $reason)"
		COROOT_PROJECT_RENDER="no data (query failed: $reason)"
		return 0
	fi
	COROOT_PROJECT_RENDER="${count} projects visible"
	COROOT_RENDER="no data (Coroot query shape not finalized — see scripts/pulse.sh TODO)"
}

GITHUB_STAR_RENDER="no data (not queried)"
GITHUB_STAR_COUNT=0
GITHUB_EXTERNAL_ISSUES_RENDER="no data (not queried)"
GITHUB_EXTERNAL_ISSUE_COUNT=0
GITHUB_EXTERNAL_USER_COUNT=0
AWAITING_VERIFICATION_OPEN=0

gh_ready() {
	if ! command -v gh >/dev/null 2>&1; then
		printf '%s\n' "gh CLI not found"
		return 1
	fi
	if [[ -n "$GITHUB_TOKEN" ]]; then
		export GH_TOKEN="$GITHUB_TOKEN"
		return 0
	fi
	if gh auth status -h github.com >/dev/null 2>&1; then
		return 0
	fi
	printf '%s\n' "token missing"
	return 1
}

query_github_stars() {
	local reason stars current prior
	if [[ -z "$GH_REPO" ]]; then
		GITHUB_STAR_RENDER="no data (GH_REPO missing)"
		return 0
	fi
	if ! reason="$(gh_ready 2>&1)"; then
		GITHUB_STAR_RENDER="no data ($reason)"
		return 0
	fi
	if ! stars="$(gh api "repos/$GH_REPO/stargazers" \
		--header "Accept: application/vnd.github.v3.star+json" \
		--paginate \
		--jq '.[].starred_at' 2>&1)"; then
		reason="$(sanitize_reason "$stars")"
		QUERY_FAILURES+=("github_stars: query failed: $reason")
		GITHUB_STAR_RENDER="no data (query failed: $reason)"
		return 0
	fi
	current="$(count_iso_lines_in_window "$stars" "$START_EPOCH" "$END_EPOCH")"
	prior="$(count_iso_lines_in_window "$stars" "$PRIOR_START_EPOCH" "$PRIOR_END_EPOCH")"
	GITHUB_STAR_COUNT="$current"
	GITHUB_STAR_RENDER="${current} ($(format_delta "$current" "$prior"))"
}

query_github_external_issues() {
	local current created is_bot issues login reason users_file
	if [[ -z "$GH_REPO" ]]; then
		GITHUB_EXTERNAL_ISSUES_RENDER="no data (GH_REPO missing)"
		return 0
	fi
	if ! reason="$(gh_ready 2>&1)"; then
		GITHUB_EXTERNAL_ISSUES_RENDER="no data ($reason)"
		return 0
	fi
	if ! issues="$(gh issue list -R "$GH_REPO" --state all --limit 100 \
		--json number,createdAt,closedAt,author,labels \
		--jq '.[] | [.createdAt, (.author.login // ""), (.author.is_bot // false)] | @tsv' 2>&1)"; then
		reason="$(sanitize_reason "$issues")"
		QUERY_FAILURES+=("github_issues: query failed: $reason")
		GITHUB_EXTERNAL_ISSUES_RENDER="no data (query failed: $reason)"
		return 0
	fi

	current=0
	users_file="$(mktemp_tracked)"
	while IFS=$'\t' read -r created login is_bot; do
		[[ -z "$created" || -z "$login" ]] && continue
		[[ "$login" == "nycterent" || "$is_bot" == "true" ]] && continue
		local created_epoch
		if ! created_epoch="$(iso_to_epoch "$created" 2>/dev/null)"; then
			continue
		fi
		if [[ "$created_epoch" -ge "$START_EPOCH" && "$created_epoch" -lt "$END_EPOCH" ]]; then
			current=$((current + 1))
			printf '%s\n' "$login" >> "$users_file"
		fi
	done <<< "$issues"

	GITHUB_EXTERNAL_ISSUE_COUNT="$current"
	GITHUB_EXTERNAL_USER_COUNT="$(sort -u "$users_file" | sed '/^$/d' | wc -l | tr -d ' ')"
	GITHUB_EXTERNAL_ISSUES_RENDER="${GITHUB_EXTERNAL_ISSUE_COUNT} issues from ${GITHUB_EXTERNAL_USER_COUNT} distinct external users"

	if awaiting="$(gh issue list -R "$GH_REPO" --state open --label awaiting-verification --limit 100 \
		--json number --jq 'length' 2>/dev/null)"; then
		AWAITING_VERIFICATION_OPEN="$awaiting"
	fi
}

query_polar
query_coroot_probe
query_github_stars
query_github_external_issues

COST_COVERAGE_RENDER="no data (costs.json missing)"
if [[ -f costs.json ]]; then
	COST_COVERAGE_RENDER="no data (cost parsing not implemented)"
fi

WEEKLY_ACTIVE_MESHES_RENDER="no data (instrumentation pending)"
TIME_TO_MESH_RENDER="no data (instrumentation pending)"
PRIMARY_ENGAGEMENT_RENDER="no data (instrumentation pending)"
VALUE_REALIZATION_RENDER="no data (instrumentation pending)"

render_strategy_metrics() {
	local metric slug
	while IFS= read -r metric; do
		[[ -z "$metric" ]] && continue
		slug="$(slug_metric "$metric")"
		if is_csv_member "$slug" || is_csv_member "$metric"; then
			continue
		fi
		case "$slug" in
			paying_customers) printf '  - **%s:** %s\n' "$metric" "$PAYING_CUSTOMERS_RENDER" ;;
			cost_coverage_ratio) printf '  - **%s:** %s\n' "$metric" "$COST_COVERAGE_RENDER" ;;
			weekly_active_meshes) printf '  - **%s:** %s\n' "$metric" "$WEEKLY_ACTIVE_MESHES_RENDER" ;;
			time_to_mesh_p50) printf '  - **%s:** %s\n' "$metric" "$TIME_TO_MESH_RENDER" ;;
			unsolicited_positive_feedback_rate) printf '  - **%s:** %s (positive-feedback tagging pending)\n' "$metric" "$GITHUB_EXTERNAL_ISSUES_RENDER" ;;
			*) printf '  - **%s:** no data\n' "$metric" ;;
		esac
	done < <(strategy_metrics)
}

render_followups() {
	local failure item printed=0
	while IFS= read -r item; do
		[[ -z "$item" ]] && continue
		printf -- "- Instrument pending pulse metric: \`%s\`.\n" "$item"
		printed=$((printed + 1))
	done < <(csv_to_lines "$PENDING_METRICS_RAW")
	for failure in "${QUERY_FAILURES[@]}"; do
		printf -- '- Investigate pulse query failure: %s.\n' "$failure"
		printed=$((printed + 1))
	done
	if [[ "$AWAITING_VERIFICATION_OPEN" -gt 0 ]]; then
		printf -- '- Verify reporter close-loop on awaiting-verification issues.\n'
		printed=$((printed + 1))
	fi
	if [[ "$printed" -eq 0 ]]; then
		printf -- '- Review pending instrumentation for pulse metrics before the next scheduled run.\n'
	fi
}

mkdir -p docs/pulse-reports
{
	printf '# wgmesh Pulse - %s - %s\n\n' "$WINDOW" "$REPORT_HUMAN"
	printf '## Headlines\n\n'
	printf -- '- %s\n' "$POLAR_HEADLINE"
	printf -- '- GitHub: %s stars added; %s.\n' "$GITHUB_STAR_COUNT" "$GITHUB_EXTERNAL_ISSUES_RENDER"
	printf -- '- Weekly active meshes and time-to-mesh remain no data (instrumentation pending).\n\n'
	printf '## Usage\n\n'
	printf -- '- **Primary engagement:** %s\n' "$PRIMARY_ENGAGEMENT_RENDER"
	printf -- '- **Value realization:** %s\n' "$VALUE_REALIZATION_RENDER"
	printf -- '- **Completions / conversions:**\n'
	printf '  - **polar_subscription_created:** %s\n' "$POLAR_CREATED_RENDER"
	printf '  - **polar_subscription_cancelled:** %s\n' "$POLAR_CANCELLED_RENDER"
	printf '  - **github_star_added:** %s\n' "$GITHUB_STAR_RENDER"
	printf -- '- **Strategy metrics:**\n'
	render_strategy_metrics
	printf '\n## System performance\n\n'
	printf -- '- **Coroot projects:** %s\n' "$COROOT_PROJECT_RENDER"
	printf -- '- **Latency and errors:** %s\n\n' "$COROOT_RENDER"
	printf '## Followups\n\n'
	render_followups
	printf '\n---\n'
	printf '_Source windows: github [%s → %s], polar [%s → %s], coroot [%s → %s]. Trailing buffer: 15m. Generated by .github/workflows/pulse.yml._\n' \
		"$START" "$END" "$START" "$END" "$START" "$END"
} > "$REPORT_PATH"

echo "Saved pulse report to $REPORT_PATH"
echo "Prior windows: github [$PRIOR_START -> $PRIOR_END], polar [$PRIOR_START -> $PRIOR_END]."
