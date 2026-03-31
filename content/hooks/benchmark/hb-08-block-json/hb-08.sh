#!/bin/bash
# HB-08: before_tool_execute blocking via JSON decision field
# Outputs JSON decision and exits 0. Verify the agent honors the JSON block.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
echo "PASS|HB-08|before_tool_execute_block_json|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
# Output varies by agent during testing — use agent-specific format
echo '{"decision": "block", "reason": "HB-08 benchmark test"}'
