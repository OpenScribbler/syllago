#!/bin/bash
# HB-16: hook config location — validates syllago installed to the right path
# This hook fires if and only if the agent discovered it from the correct path.
# A PASS entry in the log means discovery worked.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
echo "PASS|HB-16|discovery|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
