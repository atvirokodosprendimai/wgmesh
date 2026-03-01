#!/usr/bin/env bash
# Collect infrastructure health signals for the company control loop.
# Checks known service endpoints. Expand as services come online.
# Output: JSON to stdout.
set -euo pipefail

check_health() {
  local name="$1" url="$2"
  local status="unknown" latency_ms=0
  if [ -n "$url" ] && [ "$url" != "null" ]; then
    start=$(date +%s%N 2>/dev/null || echo 0)
    http_code=$(curl -sf -o /dev/null -w '%{http_code}' --connect-timeout 5 --max-time 10 "$url" 2>/dev/null || echo "000")
    end=$(date +%s%N 2>/dev/null || echo 0)
    if [ "$http_code" -ge 200 ] && [ "$http_code" -lt 400 ] 2>/dev/null; then
      status="up"
    elif [ "$http_code" = "000" ]; then
      status="unreachable"
    else
      status="error:$http_code"
    fi
    if [ "$start" != "0" ] && [ "$end" != "0" ]; then
      latency_ms=$(( (end - start) / 1000000 ))
    fi
  fi
  jq -n --arg name "$name" --arg url "$url" --arg status "$status" --argjson latency "$latency_ms" \
    '{name: $name, url: $url, status: $status, latency_ms: $latency}'
}

chimney=$(check_health "chimney" "https://chimney.beerpub.dev")
cloudroof=$(check_health "cloudroof" "https://cloudroof.eu")

# Build JSON safely with jq
jq -n \
  --arg collected_at "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" \
  --argjson chimney "$chimney" \
  --argjson cloudroof "$cloudroof" \
  '{
    source: "infrastructure",
    collected_at: $collected_at,
    services: [$chimney, $cloudroof]
  }'
