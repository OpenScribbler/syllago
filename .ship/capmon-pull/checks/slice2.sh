#!/usr/bin/env bash
# Slice 2 checkpoint: fail-closed provenance verification + staleness gate.
# Non-zero exit = red, zero = green (ship-run-test bash mode).
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$REPO_ROOT/cli"
go test ./internal/capfeed/... ./cmd/capmon-pull/...
