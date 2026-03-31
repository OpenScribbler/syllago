#!/bin/bash
# HB-19: env vars — logs all injected project/session env vars
# Tester checks log for expected CLAUDE_PROJECT_DIR, GEMINI_PROJECT_DIR, etc.

LOG="${SYLLAGO_BENCHMARK_LOG:-/tmp/syllago-benchmark.log}"
echo "PASS|HB-19|env_vars|$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$LOG"
# Log all known agent env vars (will be empty on unsupported agents)
echo "HB-19|CLAUDE_PROJECT_DIR=${CLAUDE_PROJECT_DIR:-UNSET}" >> "$LOG"
echo "HB-19|GEMINI_PROJECT_DIR=${GEMINI_PROJECT_DIR:-UNSET}" >> "$LOG"
echo "HB-19|CURSOR_PROJECT_DIR=${CURSOR_PROJECT_DIR:-UNSET}" >> "$LOG"
echo "HB-19|AUGMENT_PROJECT_DIR=${AUGMENT_PROJECT_DIR:-UNSET}" >> "$LOG"
echo "HB-19|ROOT_WORKSPACE_PATH=${ROOT_WORKSPACE_PATH:-UNSET}" >> "$LOG"
