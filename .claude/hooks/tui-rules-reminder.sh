#!/usr/bin/env bash
# TUI Rules Reminder Hook
# PreToolUse hook for Read operations on cli/internal/tui/ files.
# Reminds Claude to consult the TUI design rules before making changes.

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

# Only trigger for cli/internal/tui/ files (not test data, not golden files)
if [[ ! "$FILE_PATH" =~ cli/internal/tui/[^/]+\.(go|md)$ ]]; then
    exit 0
fi

# Don't trigger for golden/test data files
if [[ "$FILE_PATH" =~ testdata/ ]]; then
    exit 0
fi

# Output reminder as a success message (exit 0 = non-blocking)
echo "TUI Design Rules Active — Before modifying this file, review:"
echo "  - Card grids: .claude/rules/tui-card-grid.md"
echo "  - Modals: .claude/rules/tui-modal-patterns.md"
echo "  - Keyboard: .claude/rules/tui-keyboard-bindings.md"
echo "  - Mouse: .claude/rules/tui-mouse.md"
echo "  - Scroll: .claude/rules/tui-scroll.md"
echo "  - Styles: .claude/rules/tui-styles-gate.md"
echo "  - Responsive: .claude/rules/tui-responsive.md"
echo "  - Text: .claude/rules/tui-text-handling.md"
echo "  - Pages: .claude/rules/tui-page-pattern.md"
echo "  - Items rebuild: .claude/rules/tui-items-rebuild.md"
echo "  - Testing: .claude/rules/tui-test-patterns.md, tui-boundary-testing.md"
echo "  - Design doc: docs/design/tui-spec.md"
exit 0
