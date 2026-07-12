#!/usr/bin/env bash
# Slice 4 checkpoint: change detection, ETag persistence, automation summary
# — plus the >=80% coverage gate on cli/internal/capfeed.
# Non-zero exit = red, zero = green (ship-run-test bash mode).
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$REPO_ROOT/cli"

go test ./internal/capfeed/... ./cmd/capmon-pull/...

COV_OUT="$(mktemp "${TMPDIR:-/tmp}/capfeed-cov.XXXXXX.out")"
trap 'rm -f "$COV_OUT"' EXIT
go test ./internal/capfeed/ -coverprofile="$COV_OUT" > /dev/null
TOTAL="$(go tool cover -func="$COV_OUT" | awk '/^total:/ {gsub(/%/,"",$3); print $3}')"
echo "capfeed coverage: ${TOTAL}%"
awk -v t="$TOTAL" 'BEGIN { exit (t >= 80.0) ? 0 : 1 }' || {
  echo "FAIL: capfeed coverage ${TOTAL}% is below the 80% gate"
  exit 1
}
