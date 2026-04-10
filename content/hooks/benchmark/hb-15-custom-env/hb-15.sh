#!/bin/bash
# HB-15: custom env — verify per-hook env var is injected
# Logs PASS and whether HB15_TEST_VAR is set.
# Configure the hook with HB15_TEST_VAR=benchmark in the agent's hook config.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
if [ -n "$HB15_TEST_VAR" ]; then
  echo "PASS|HB-15|custom_env|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
else
  echo "FAIL|HB-15|custom_env|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
fi
