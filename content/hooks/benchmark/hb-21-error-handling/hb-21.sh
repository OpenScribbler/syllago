#!/bin/bash
# HB-21: error handling — exits 1 to test fail-open behavior
# Expected: agent continues normally despite hook failure.
# If agent halts or shows error to user, fail-open is not working.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
echo "PASS|HB-21|error_handling_exit1|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
exit 1
