#!/bin/bash
# HB-20: stdin — logs the full stdin JSON sent by the agent
# Tester inspects the log to verify format matches spec (hookEventName, toolName, etc.)

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
STDIN_DATA=$(cat)
echo "PASS|HB-20|stdin|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
echo "HB-20|STDIN=${STDIN_DATA}" >> "$LOG"
