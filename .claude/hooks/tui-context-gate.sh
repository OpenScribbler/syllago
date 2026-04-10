#!/usr/bin/env bash
# TUI Context Gate Hook
# PreToolUse hook for Edit/Write operations on cli/internal/tui/*.go files.
# Warns if /tui-builder skill hasn't been loaded in this session.

set -euo pipefail

# Read JSON input from stdin
INPUT=$(cat)

# Extract file_path from the tool input
FILE_PATH=$(echo "$INPUT" | python3 -c "
import json, sys
try:
    data = json.load(sys.stdin)
    fp = data.get('input', {}).get('file_path', '')
    print(fp)
except (json.JSONDecodeError, KeyError, TypeError):
    print('')
" 2>/dev/null || echo "")

# Only check TUI files
if [[ "$FILE_PATH" != *"cli/internal/tui/"* ]]; then
    exit 0
fi

# Check if the skill marker exists (set by /tui-builder skill)
MARKER="/tmp/syllago-tui-builder-${PPID}"
if [[ -f "$MARKER" ]]; then
    exit 0
fi

# Skill not loaded — warn (non-blocking)
echo "TUI edit detected. Run /tui-builder first to load layout rules and golden checklist."
exit 0
