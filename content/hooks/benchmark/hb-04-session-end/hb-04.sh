#!/bin/bash
# HB-04: session_end event binding
# Logs PASS when the event fires.
# Requires $SYLLAGO_BENCHMARK_LOG to be set before running benchmarks.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
echo "PASS|HB-04|session_end|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
