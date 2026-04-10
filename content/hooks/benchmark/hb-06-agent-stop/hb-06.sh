#!/bin/bash
# HB-06: agent_stop event binding
# Logs PASS when the event fires.
# Requires $SYLLAGO_BENCHMARK_LOG to be set before running benchmarks.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
echo "PASS|HB-06|agent_stop|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
