#!/bin/bash
# HB-10: agent_stop continue behavior
# Logs PASS and outputs a block/continue decision. Verify agent keeps working.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
echo "PASS|HB-10|agent_stop_continue|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
echo '{"decision": "block"}'
