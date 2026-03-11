#!/usr/bin/env bash
# TUI Convention Enforcement Hook
# PreToolUse hook for Edit/Write operations on cli/internal/tui/*.go files.
# Checks for inline colors, hardcoded key strings, and emoji in UI output.

set -euo pipefail

# Read JSON input from stdin
INPUT=$(cat)

# Extract file_path from the tool input
FILE_PATH=$(echo "$INPUT" | python3 -c "
import json, sys
try:
    data = json.load(sys.stdin)
    # Handle both Edit (file_path at top level) and Write (file_path at top level)
    fp = data.get('input', {}).get('file_path', '')
    print(fp)
except (json.JSONDecodeError, KeyError, TypeError):
    print('')
" 2>/dev/null || echo "")

# Only check cli/internal/tui/*.go files (not test files, not styles.go, not keys.go)
if [[ ! "$FILE_PATH" =~ cli/internal/tui/[^/]+\.go$ ]]; then
    exit 0
fi
# Skip styles.go (that's WHERE colors should be defined)
if [[ "$FILE_PATH" =~ styles\.go$ ]]; then
    exit 0
fi
# Skip keys.go (that's WHERE bindings should be defined)
if [[ "$FILE_PATH" =~ keys\.go$ ]]; then
    exit 0
fi
# Skip test files
if [[ "$FILE_PATH" =~ _test\.go$ ]]; then
    exit 0
fi

# Extract the content being written/edited
CONTENT=$(echo "$INPUT" | python3 -c "
import json, sys
try:
    data = json.load(sys.stdin)
    inp = data.get('input', {})
    # Edit tool uses new_string, Write tool uses content
    text = inp.get('new_string', '') or inp.get('content', '')
    print(text)
except (json.JSONDecodeError, KeyError, TypeError):
    print('')
" 2>/dev/null || echo "")

# If no content to check, pass
if [[ -z "$CONTENT" ]]; then
    exit 0
fi

VIOLATIONS=""

# Check 1: Inline color definitions (lipgloss colors outside styles.go)
# Look for Foreground/Background with hex color literals like "#abc123"
if echo "$CONTENT" | grep -qP '(?:Foreground|Background|BorderForeground)\(.*"#[0-9a-fA-F]'; then
    VIOLATIONS+="INLINE COLOR: Found hardcoded hex color in lipgloss style call.\n"
    VIOLATIONS+="  -> Define colors in styles.go using lipgloss.AdaptiveColor, then reference the named variable.\n\n"
fi

# Check 2: Inline key string comparisons
# Look for msg.String() == "x" pattern (but not in comments)
if echo "$CONTENT" | grep -qP 'msg\.String\(\)\s*==\s*"[a-zA-Z]"'; then
    VIOLATIONS+="HARDCODED KEY: Found msg.String() == \"x\" key comparison.\n"
    VIOLATIONS+="  -> Use key.Matches(msg, keys.Foo) from keys.go instead.\n"
    VIOLATIONS+="  -> Exception: msg.Type == tea.KeyEnter/Esc/Left/Right are acceptable.\n\n"
fi

# Check 3: Emoji in UI output (common emoji ranges, excluding test strings)
# Check for common emoji patterns in string literals
if echo "$CONTENT" | grep -qP '[\x{1F300}-\x{1F9FF}\x{2600}-\x{26FF}\x{2700}-\x{27BF}]'; then
    VIOLATIONS+="EMOJI IN UI: Found emoji character in TUI code.\n"
    VIOLATIONS+="  -> Use colored symbols instead: checkmark, X, >, dash, arrow, warning triangle.\n"
    VIOLATIONS+="  -> See styles.go for the symbol conventions.\n\n"
fi

if [[ -n "$VIOLATIONS" ]]; then
    echo "TUI Convention Violation(s) in $FILE_PATH:"
    echo ""
    echo -e "$VIOLATIONS"
    echo "See cli/internal/tui/CLAUDE.md 'Before You Edit' checklist for conventions."
    exit 1
fi

exit 0
