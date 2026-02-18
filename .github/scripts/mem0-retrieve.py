#!/usr/bin/env python3
"""Retrieve relevant memories from mem0 for a Goose implementation run.

Usage:
    python mem0-retrieve.py <issue_number> <output_file>

Queries mem0 for past learnings related to the issue, common Go build
errors, and implementation patterns. Writes formatted context to output_file.
"""

import os
import sys

# mem0 is optional — if not installed or DB empty, produce empty context
try:
    from mem0 import Memory
except ImportError:
    print("mem0 not installed, skipping memory retrieval")
    if len(sys.argv) >= 3:
        with open(sys.argv[2], "w") as f:
            f.write("")
    sys.exit(0)

# Model for mem0 LLM memory extraction — configurable via env var.
# Defaults to a small/cheap model suitable for memory summarization.
MEM0_MODEL = os.environ.get("MEM0_MODEL", "anthropic/claude-sonnet-4-20250514")


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

    # Use litellm with Anthropic proxy if API key available,
    # otherwise skip LLM (mem0 will use default or fail gracefully)
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


def retrieve_memories(issue_number: str) -> list[str]:
    """Query mem0 for memories relevant to this implementation run."""
    try:
        config = get_mem0_config()
        memory = Memory.from_config(config)
    except ImportError as e:
        print(f"mem0 dependency missing: {e}")
        return []
    except FileNotFoundError as e:
        print(f"mem0 database not found (first run?): {e}")
        return []
    except Exception as e:
        print(f"WARNING: Failed to initialize mem0 ({type(e).__name__}): {e}")
        return []

    memories = []
    # Issue-specific query first, then broader patterns
    queries = [
        f"issue #{issue_number} implementation",
        "go build undefined type errors in wgmesh",
        "successful goose implementation patterns for wgmesh",
    ]

    for query in queries:
        try:
            results = memory.search(
                query,
                user_id="goose-ci",
                limit=3,
            )
            if isinstance(results, dict) and "results" in results:
                result_list = results["results"]
            elif isinstance(results, list):
                result_list = results
            else:
                continue

            for r in result_list:
                text = r.get("memory", "") if isinstance(r, dict) else str(r)
                if text and text not in memories:
                    memories.append(text)
        except Exception as e:
            print(f"mem0 search failed for '{query}' ({type(e).__name__}): {e}")
            continue

    return memories


def format_context(memories: list[str]) -> str:
    """Format memories as markdown context for Goose."""
    if not memories:
        return ""

    lines = [
        "## Lessons from Previous Runs",
        "",
        "The following knowledge was accumulated from past Goose CI runs.",
        "Use these lessons to avoid repeating mistakes:",
        "",
    ]
    for i, mem in enumerate(memories, 1):
        lines.append(f"{i}. {mem}")

    lines.append("")
    return "\n".join(lines)


def main():
    if len(sys.argv) < 3:
        print(f"Usage: {sys.argv[0]} <issue_number> <output_file>")
        sys.exit(1)

    issue_number = sys.argv[1]
    output_file = sys.argv[2]

    print(f"Retrieving memories for issue #{issue_number}...")
    memories = retrieve_memories(issue_number)
    print(f"Found {len(memories)} relevant memories")

    context = format_context(memories)
    with open(output_file, "w") as f:
        f.write(context)

    if memories:
        print("Memory context written to", output_file)
    else:
        print("No memories found, empty context file created")


if __name__ == "__main__":
    main()
