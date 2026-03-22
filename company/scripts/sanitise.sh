#!/usr/bin/env bash
# Sanitise text before committing to public repo or passing to LLM output.
# Reads stdin, writes sanitised text to stdout.
# Exits non-zero if secrets are detected (fail-safe: don't commit).
set -euo pipefail

input=$(cat)
found=0

# Patterns that should NEVER appear in public output
patterns=(
  # API keys and tokens
  'sk-[a-zA-Z0-9]{20,}'           # Anthropic/OpenAI style
  'ghp_[a-zA-Z0-9]{36}'           # GitHub PAT
  'ghs_[a-zA-Z0-9]{36}'           # GitHub App token
  'github_pat_[a-zA-Z0-9_]{82}'   # Fine-grained PAT
  'AKIA[0-9A-Z]{16}'              # AWS access key
  'sk_live_[a-zA-Z0-9]{24,}'      # Stripe live key
  'sk_test_[a-zA-Z0-9]{24,}'      # Stripe test key
  'whsec_[a-zA-Z0-9]{32,}'        # Stripe webhook secret

  # Private keys
  '-----BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY-----'
  '-----BEGIN PGP PRIVATE KEY BLOCK-----'

  # Common secret env var values (heuristic: long base64 after = sign)
  'SECRET[_=][a-zA-Z0-9+/]{32,}'
  'TOKEN[_=][a-zA-Z0-9+/]{32,}'
  'PASSWORD[_=].{8,}'
)

for pattern in "${patterns[@]}"; do
  if echo "$input" | grep -qEi "$pattern"; then
    echo "SANITISE ERROR: Found potential secret matching pattern: $pattern" >&2
    found=1
  fi
done

# Email pattern (basic — catches most customer PII leaks)
unknown_emails=$(echo "$input" | grep -oEi '[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}' | grep -viE '(noreply@|bot@|ghost\.lt|github\.com)' || true)
if [ -n "$unknown_emails" ]; then
  echo "SANITISE WARNING: Found potential PII email(s): $unknown_emails" >&2
  # Warning only — don't fail for emails, they might be in public commit history
fi

if [ "$found" -eq 1 ]; then
  echo "SANITISE FAILED: Refusing to output — secrets detected. Review input." >&2
  exit 1
fi

echo "$input"
