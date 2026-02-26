#!/usr/bin/env python3
"""Release guard hook for Claude Code.

PreToolUse hook that blocks git tag creation and version tag pushing
unless a .release-pending.yml file exists with status: prepared.

This prevents accidental releases — tags can only be created/pushed
after the /release skill has prepared a release through its full flow.
"""

import json
import os
import re
import subprocess
import sys


def get_repo_root():
    try:
        return subprocess.check_output(
            ["git", "rev-parse", "--show-toplevel"],
            text=True,
            stderr=subprocess.DEVNULL,
        ).strip()
    except Exception:
        return None


def check_release_file(repo_root):
    """Check that .release-pending.yml exists and has status: prepared."""
    release_file = os.path.join(repo_root, ".release-pending.yml")

    if not os.path.exists(release_file):
        print("BLOCKED: Cannot create or push version tags without a prepared release.")
        print("Run /release first to prepare a release.")
        sys.exit(1)

    with open(release_file) as f:
        content = f.read()

    if "status: prepared" not in content:
        print("BLOCKED: Release is not in 'prepared' state.")
        for line in content.splitlines():
            if line.startswith("status:"):
                print(f"Current state: {line.strip()}")
        sys.exit(1)


def main():
    stdin_data = sys.stdin.read().strip()
    if not stdin_data:
        sys.exit(0)

    try:
        payload = json.loads(stdin_data)
    except json.JSONDecodeError:
        sys.exit(0)

    if payload.get("tool_name") != "Bash":
        sys.exit(0)

    cmd = payload.get("tool_input", {}).get("command", "")

    # Detect git tag creation (exclude listing: -l, --list)
    creates_tag = bool(re.search(r"git\s+tag\s+(?!-l\b|--list\b)", cmd))

    # Detect pushing version tags (v0.x.x patterns or --tags flag)
    pushes_version_tag = bool(re.search(r"git\s+push.*\bv\d", cmd))
    pushes_all_tags = bool(re.search(r"git\s+push\s+--tags", cmd))

    if not creates_tag and not pushes_version_tag and not pushes_all_tags:
        sys.exit(0)

    repo_root = get_repo_root()
    if not repo_root:
        sys.exit(0)

    check_release_file(repo_root)

    # All checks passed — allow the command
    sys.exit(0)


if __name__ == "__main__":
    main()
