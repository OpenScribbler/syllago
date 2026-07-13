#!/usr/bin/env bash
# Slice 5 checkpoint: Coverage Drift findings from mirrored Capability
# Documents + the non-required CI job's structural presence.
# Non-zero exit = red, zero = green (ship-run-test bash mode).
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$REPO_ROOT/cli"

go test ./internal/provider/...
SYLLAGO_COVERAGE_FEED=1 go test ./internal/provider/ -run TestCoverageFeedDrift

CI_YML="$REPO_ROOT/.github/workflows/ci.yml"
grep -q 'coverage-drift' "$CI_YML" || { echo "FAIL: ci.yml has no coverage-drift job"; exit 1; }
grep -q 'SYLLAGO_COVERAGE_FEED' "$CI_YML" || { echo "FAIL: ci.yml does not set SYLLAGO_COVERAGE_FEED"; exit 1; }
# Every uses: line must be pinned to a full 40-char SHA.
if grep -E '^\s*uses:' "$CI_YML" | grep -vE '@[0-9a-f]{40}'; then
  echo "FAIL: ci.yml has uses: lines not pinned to a full SHA"
  exit 1
fi
echo "slice5 structural checks OK"
