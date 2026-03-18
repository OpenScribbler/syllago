#!/usr/bin/env bash
# PostToolUse hook: Run wizard invariant tests after any TUI file edit.
# Provides immediate feedback — NOT a hard gate (PostToolUse cannot undo edits).

set -euo pipefail

INPUT=$(cat)

FILE_PATH=$(echo "$INPUT" | python3 -c "
import json, sys
try:
    data = json.load(sys.stdin)
    fp = data.get('input', {}).get('file_path', '')
    print(fp)
except (json.JSONDecodeError, KeyError, TypeError):
    print('')
" 2>/dev/null || echo "")

# Only trigger on cli/internal/tui/*.go files
if [[ ! "$FILE_PATH" =~ cli/internal/tui/[^/]+\.go$ ]]; then
    exit 0
fi

# Run wizard invariant tests
cd "$(git rev-parse --show-toplevel)/cli" && go test ./internal/tui/ -run "TestWizardInvariant" -count=1 2>&1
