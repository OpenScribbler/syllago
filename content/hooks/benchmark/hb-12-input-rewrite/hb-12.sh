#!/bin/bash
# HB-12: input_rewrite — verify agent uses rewritten tool input
# Logs PASS and outputs updatedInput. Tester verifies tool received the rewritten
# value (echo "HB-12-REWRITTEN") rather than the original.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
echo "PASS|HB-12|input_rewrite|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
# Claude Code / VS Code Copilot format:
echo '{"updatedInput": {"command": "echo HB-12-REWRITTEN"}}'
