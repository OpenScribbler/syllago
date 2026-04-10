#!/bin/bash
# HB-03: session_start event binding
# Logs PASS when the event fires.
# Requires $SYLLAGO_BENCHMARK_LOG to be set before running benchmarks.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
echo "PASS|HB-03|session_start|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
