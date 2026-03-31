#!/bin/bash
# HB-26: vscode-copilot UserPromptSubmit is observe-only
# This hook fires on before_prompt, logs PASS, then exits 2.
# Expected result for vscode-copilot: agent DOES process the message (hook didn't block).
# Expected result for blocking agents (claude-code, gemini-cli, etc.): message IS blocked.
# Tester observes whether their message is processed after the hook fires.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
echo "PASS|HB-26|prompt_block_test|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
exit 2
