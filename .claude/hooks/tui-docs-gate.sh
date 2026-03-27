#!/usr/bin/env bash
# TUI Docs Gate Hook
# PreToolUse hook for Bash. Blocks git commit when TUI code changes
# are staged without corresponding doc updates.

set -euo pipefail

# Read JSON input from stdin
INPUT=$(cat)

# Extract command from the tool input
COMMAND=$(echo "$INPUT" | python3 -c "
import json, sys
try:
    data = json.load(sys.stdin)
    print(data.get('input', {}).get('command', ''))
except (json.JSONDecodeError, KeyError, TypeError):
    print('')
" 2>/dev/null || echo "")

# Check bypass: --no-doc-update in the command
if [[ "$COMMAND" == *"--no-doc-update"* ]]; then
    exit 0
fi

# Check 1: Is this a git commit command?
if [[ "$COMMAND" != "git commit"* ]]; then
    exit 0
fi

# Check 2: Are there TUI .go files staged?
if ! git diff --cached --name-only 2>/dev/null | grep -q "cli/internal/tui/.*\.go$"; then
    exit 0
fi

# Check 3: Is at least one doc file staged?
STAGED=$(git diff --cached --name-only 2>/dev/null)
if echo "$STAGED" | grep -q ".claude/skills/tui-builder/SKILL.md"; then
    exit 0
fi
if echo "$STAGED" | grep -q ".claude/rules/tui-"; then
    exit 0
fi
if echo "$STAGED" | grep -q "cli/internal/tui/CLAUDE.md"; then
    exit 0
fi

# TUI code staged but no doc updates — block
cat <<'EOF'
BLOCKED: TUI code changes detected but no doc updates staged.

If this commit introduces or changes a pattern, update:
  - .claude/skills/tui-builder/SKILL.md (gotchas, contracts)
  - .claude/rules/tui-*.md (enforceable invariants)
  - cli/internal/tui/CLAUDE.md (architecture)

If no doc update is needed (e.g., pure bug fix with no new pattern),
add --no-doc-update to the commit message to bypass this gate.
EOF
exit 2
