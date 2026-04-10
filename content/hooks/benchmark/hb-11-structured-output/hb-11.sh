#!/bin/bash
# HB-11: structured output — verify agent parses JSON stdout
# Logs PASS and returns a JSON observation. Tester verifies agent did not
# treat JSON as plain text or inject it verbatim into context.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
echo "PASS|HB-11|structured_output|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
echo '{"decision": "allow", "_benchmark": "hb-11"}'
