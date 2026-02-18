#!/usr/bin/env python3
"""Save learnings from a Goose implementation run to mem0.

Usage:
    python mem0-save.py <issue_number> <goose_log> <success|failure>

Parses the Goose output log, extracts key learnings (errors encountered,
patterns that worked, types discovered), and stores them in mem0 for
future runs to benefit from.

All text is sanitized before storage to prevent leaking secrets.
"""

import os
import re
import sys

try:
    from mem0 import Memory
except ImportError:
    print("mem0 not installed, skipping memory save")
    sys.exit(0)

# Model for mem0 LLM memory extraction — configurable via env var.
MEM0_MODEL = os.environ.get("MEM0_MODEL", "anthropic/claude-sonnet-4-20250514")

# Patterns that look like secrets — stripped before storing memories.
_SECRET_PATTERNS = [
    re.compile(r"sk-[A-Za-z0-9_-]{20,}"),          # OpenAI keys
    re.compile(r"m0-[A-Za-z0-9_-]{20,}"),           # mem0 keys
    re.compile(r"ghp_[A-Za-z0-9]{36,}"),             # GitHub PATs
    re.compile(r"ghs_[A-Za-z0-9]{36,}"),             # GitHub App tokens
    re.compile(r"github_pat_[A-Za-z0-9_]{20,}"),     # Fine-grained PATs
    re.compile(r"Bearer\s+[A-Za-z0-9._-]{20,}"),     # Bearer tokens
    re.compile(r"https?://[^@\s]+:[^@\s]+@\S+"),     # URLs with credentials
    re.compile(r"ANTHROPIC_API_KEY=\S+"),             # Env var leaks
    re.compile(r"OPENAI_API_KEY=\S+"),
    re.compile(r"PUSH_TOKEN=\S+"),
    re.compile(r"GITHUB_TOKEN=\S+"),
]


def sanitize_text(text: str) -> str:
    """Remove potential secrets and sensitive data from text."""
    for pattern in _SECRET_PATTERNS:
        text = pattern.sub("[REDACTED]", text)
    return text


def get_mem0_config():
    """Build mem0 config using local embeddings + z.ai LLM."""
    api_key = os.environ.get("ANTHROPIC_API_KEY", "")
    api_base = os.environ.get("ANTHROPIC_HOST", "https://api.anthropic.com")

    config = {
        "embedder": {
            "provider": "huggingface",
            "config": {
                "model": "multi-qa-MiniLM-L6-cos-v1",
            },
        },
        "vector_store": {
            "provider": "qdrant",
            "config": {
                "path": "/tmp/mem0-qdrant",
            },
        },
    }

    if api_key:
        config["llm"] = {
            "provider": "litellm",
            "config": {
                "model": MEM0_MODEL,
                "api_key": api_key,
                "api_base": api_base,
                "temperature": 0.1,
                "max_tokens": 2000,
            },
        }

    return config


def extract_build_errors(log_content: str) -> list[str]:
    """Extract go build/test/vet error patterns from log."""
    errors = []

    # Go build errors: any .go file with line:col: message
    build_errors = re.findall(
        r"(\S+\.go:\d+:\d+: .+)", log_content
    )
    for err in build_errors[:10]:  # cap at 10
        errors.append(f"Build error: {sanitize_text(err)}")

    # Go test failures: "--- FAIL: TestFoo (0.00s)"
    test_failures = re.findall(r"--- FAIL: (\S+ \(.+?\))", log_content)
    for fail in test_failures[:10]:
        errors.append(f"Test failure: {fail}")

    # Go vet issues
    vet_issues = re.findall(r"(vet: .+)", log_content)
    for issue in vet_issues[:5]:
        errors.append(f"Vet issue: {sanitize_text(issue)}")

    return errors


def extract_success_patterns(log_content: str) -> list[str]:
    """Extract patterns from successful runs."""
    patterns = []

    # Files that were modified — match any .go file path
    modified_files = re.findall(
        r"(?:create|modify|edit|write|update)\w*\s+(\S+\.go)\b",
        log_content,
        re.IGNORECASE,
    )
    if modified_files:
        unique = list(dict.fromkeys(modified_files))[:10]
        patterns.append(
            f"Modified files: {', '.join(unique)}"
        )

    # Type/function declarations only (lines starting with keyword)
    type_refs = re.findall(
        r"^(?:type|func)\s+([A-Z]\w+)",
        log_content,
        re.MULTILINE,
    )
    if type_refs:
        unique = list(dict.fromkeys(type_refs))[:15]
        patterns.append(
            f"Types/functions used: {', '.join(unique)}"
        )

    return patterns


def build_memories(
    issue_number: str,
    log_content: str,
    outcome: str,
) -> list[dict]:
    """Build memory entries from the run."""
    memories = []

    if outcome == "failure":
        errors = extract_build_errors(log_content)
        if errors:
            error_text = sanitize_text("; ".join(errors[:5]))
            memories.append({
                "messages": [
                    {
                        "role": "user",
                        "content": (
                            f"Goose failed implementing issue #{issue_number}. "
                            f"Errors: {error_text}"
                        ),
                    },
                    {
                        "role": "assistant",
                        "content": (
                            f"Issue #{issue_number} implementation failed. "
                            f"Key errors to avoid next time: {error_text}. "
                            "Always read source files before using types."
                        ),
                    },
                ],
                "metadata": {
                    "issue": issue_number,
                    "outcome": "failure",
                    "type": "error_pattern",
                },
            })

    elif outcome == "success":
        patterns = extract_success_patterns(log_content)
        if patterns:
            pattern_text = sanitize_text("; ".join(patterns))
            memories.append({
                "messages": [
                    {
                        "role": "user",
                        "content": (
                            f"Goose successfully implemented issue #{issue_number}. "
                            f"Patterns: {pattern_text}"
                        ),
                    },
                    {
                        "role": "assistant",
                        "content": (
                            f"Issue #{issue_number} was implemented successfully. "
                            f"Effective patterns: {pattern_text}. "
                            "Reuse these approaches for similar tasks."
                        ),
                    },
                ],
                "metadata": {
                    "issue": issue_number,
                    "outcome": "success",
                    "type": "success_pattern",
                },
            })

    # Always save a run summary — count only lines containing ": error"
    # or "Error:" to avoid false positives from variable names/docs.
    log_size = len(log_content)
    error_count = len(re.findall(
        r"(?::\s*error\b|Error:)", log_content
    ))
    memories.append({
        "messages": [
            {
                "role": "user",
                "content": (
                    f"Run summary for issue #{issue_number}: "
                    f"outcome={outcome}, log_size={log_size}, "
                    f"error_mentions={error_count}"
                ),
            },
            {
                "role": "assistant",
                "content": (
                    f"Recorded run for issue #{issue_number} "
                    f"with outcome '{outcome}'."
                ),
            },
        ],
        "metadata": {
            "issue": issue_number,
            "outcome": outcome,
            "type": "run_summary",
        },
    })

    return memories


def main():
    if len(sys.argv) < 4:
        print(
            f"Usage: {sys.argv[0]} <issue_number> <goose_log_file> "
            "<success|failure>"
        )
        sys.exit(1)

    issue_number = sys.argv[1]
    log_file = sys.argv[2]
    outcome = sys.argv[3]

    if outcome not in ("success", "failure"):
        print(f"Invalid outcome: {outcome}. Must be 'success' or 'failure'")
        sys.exit(1)

    # Read log
    try:
        with open(log_file) as f:
            log_content = f.read()
    except FileNotFoundError:
        print(f"Log file not found: {log_file}")
        log_content = f"No log available for issue #{issue_number}"

    # Initialize mem0
    try:
        config = get_mem0_config()
        memory = Memory.from_config(config)
    except Exception as e:
        print(f"WARNING: Failed to initialize mem0 ({type(e).__name__}): {e}")
        sys.exit(0)  # non-fatal

    # Build and save memories
    entries = build_memories(issue_number, log_content, outcome)
    saved = 0

    for entry in entries:
        try:
            memory.add(
                entry["messages"],
                user_id="goose-ci",
                metadata=entry["metadata"],
            )
            saved += 1
        except Exception as e:
            print(f"Failed to save memory ({type(e).__name__}): {e}")

    print(f"Saved {saved}/{len(entries)} memories for issue #{issue_number}")


if __name__ == "__main__":
    main()
