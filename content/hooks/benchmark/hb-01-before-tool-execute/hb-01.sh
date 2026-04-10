#!/bin/bash
# HB-01: before_tool_execute event binding
# Logs PASS when the event fires.
# Requires $SYLLAGO_BENCHMARK_LOG to be set before running benchmarks.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
echo "PASS|HB-01|before_tool_execute|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
