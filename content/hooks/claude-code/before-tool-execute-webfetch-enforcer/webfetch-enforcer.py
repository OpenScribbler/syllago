#!/usr/bin/env python3
"""PreToolUse hook for WebFetch.
Blocks WebFetch and redirects to Readability MCP.
Auto-allows if Readability already failed for the requested URL."""

import json
import os
import sys

PAI_DIR = os.environ.get("PAI_DIR", os.path.expanduser("~/.config/pai"))


def get_failures_file(session_id: str) -> str:
    return os.path.join(PAI_DIR, f".readability-failures-{session_id}.json")


def load_failures(session_id: str) -> list:
    try:
        with open(get_failures_file(session_id)) as f:
            data = json.load(f)
            return data.get("urls", [])
    except (FileNotFoundError, json.JSONDecodeError):
        return []


def block(message: str):
    print(json.dumps({"allow": False, "message": message}))


def allow():
    print(json.dumps({"allow": True}))


def main():
    try:
        payload = json.loads(sys.stdin.read())
    except Exception:
        block("WebFetch enforcer: could not parse payload. Use mcp__readability__parse instead.")
        return

    tool_name = payload.get("tool_name", "")
    if tool_name != "WebFetch":
        allow()
        return

    session_id = payload.get("session_id", "unknown")
    url = payload.get("tool_input", {}).get("url", "")

    # Check if Readability already failed for this URL
    if url and url in load_failures(session_id):
        allow()
        return

    # Block — Readability hasn't been tried for this URL yet
    block(
        f"WebFetch blocked. Use mcp__readability__parse first for token-efficient fetching.\n"
        f"\n"
        f'Run: mcp__readability__parse("{url}")\n'
        f"\n"
        f"If Readability fails (empty content, error, or JS-heavy site), "
        f"WebFetch will be automatically allowed for this URL on retry."
    )


if __name__ == "__main__":
    try:
        main()
    except Exception:
        # Fail closed
        block("WebFetch enforcer error. Try mcp__readability__parse first.")
