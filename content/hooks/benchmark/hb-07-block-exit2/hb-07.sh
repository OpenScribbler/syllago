#!/bin/bash
# HB-07: before_tool_execute blocking via exit code 2
# This hook logs PASS then exits 2 to verify blocking behavior.
# DO NOT install permanently — it will block all tool use.
# Use only for point-in-time blocking verification.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
echo "PASS|HB-07|before_tool_execute_block_exit2|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
exit 2
