#!/bin/bash
# HB-14: async execution — verify hook does not block main flow
# Sleeps 3 seconds. If the agent is async, it will not delay tool execution.
# If sync, tool execution will be delayed by 3 seconds.
# Tester times the tool execution latency.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
sleep 3
echo "PASS|HB-14|async_execution|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
