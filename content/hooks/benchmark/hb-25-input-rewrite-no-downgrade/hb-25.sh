#!/bin/bash
# HB-25: input_rewrite capability detection
# When converted for gemini-cli or cursor, this hook should rewrite input, NOT block.
# If the tool is blocked, the converter incorrectly applied blocking fallback.
# Tester verifies: (1) tool executes (not blocked), (2) input is rewritten.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
echo "PASS|HB-25|input_rewrite_no_downgrade|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
# Gemini CLI format:
echo '{"hookSpecificOutput": {"tool_input": {"command": "echo HB-25-REWRITTEN"}}}'
# Note: When testing cursor, use {"preToolUse": {"updated_input": {"command": "echo HB-25-REWRITTEN"}}}
