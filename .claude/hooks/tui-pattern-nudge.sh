#!/usr/bin/env bash
# TUI Pattern Nudge Hook
# PostToolUse hook for Bash. After TUI tests pass with code changes,
# reminds to update documentation if a new pattern was established.

set -euo pipefail

# Read JSON input from stdin
INPUT=$(cat)

# Extract command and stdout from the tool result
COMMAND=$(echo "$INPUT" | python3 -c "
import json, sys
try:
    data = json.load(sys.stdin)
    print(data.get('input', {}).get('command', ''))
except (json.JSONDecodeError, KeyError, TypeError):
    print('')
" 2>/dev/null || echo "")

# Check 1: Is this a TUI test command?
if [[ "$COMMAND" != *"go test ./internal/tui/"* ]]; then
    exit 0
fi

# Check 2: Did the tests pass? Look for FAIL in output
STDOUT=$(echo "$INPUT" | python3 -c "
import json, sys
try:
    data = json.load(sys.stdin)
    print(data.get('result', {}).get('stdout', ''))
except (json.JSONDecodeError, KeyError, TypeError):
    print('')
" 2>/dev/null || echo "")

if echo "$STDOUT" | grep -q "FAIL"; then
    exit 0
fi

# Check 3: Are there TUI code changes in git diff?
if ! git diff --name-only 2>/dev/null | grep -q "cli/internal/tui/.*\.go$"; then
    exit 0
fi

# All three conditions met — emit the reminder
cat <<'EOF'
TUI tests passed after code changes. If you established a new pattern,
fixed a bug, or discovered a gotcha:
  - Update .claude/skills/tui-builder/SKILL.md (gotchas, message contracts)
  - Update .claude/rules/tui-*.md (enforceable invariants)
  - Update cli/internal/tui/CLAUDE.md (architecture changes)
EOF
exit 0
