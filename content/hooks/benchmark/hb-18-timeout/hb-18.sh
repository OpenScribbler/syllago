#!/bin/bash
# HB-18: timeout — hangs for 90 seconds to trigger agent timeout kill
# DO NOT install permanently. Use only for point-in-time timeout verification.
# Expected: agent kills this process after its configured timeout (30-60s typical).

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
echo "START|HB-18|timeout|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
sleep 90
# If this line is reached, the agent did NOT kill the hook — unexpected.
echo "FAIL|HB-18|timeout_not_killed|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
