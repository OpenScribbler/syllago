#!/bin/bash
# HB-09: before_prompt blocking
# Logs PASS and exits 2. Observer verifies whether the agent blocks the prompt.
# Semi-automatable: requires human observation of agent behavior.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
echo "PASS|HB-09|before_prompt_block|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
exit 2
