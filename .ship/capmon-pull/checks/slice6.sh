#!/usr/bin/env bash
# Slice 6 checkpoint: daily Capmon Pull cron workflow hygiene + consume-only
# docs. Non-zero exit = red, zero = green (ship-run-test bash mode).
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$REPO_ROOT"

WF=".github/workflows/capmon-pull.yml"
fail() { echo "FAIL: $1"; exit 1; }

[ -f "$WF" ] || fail "$WF does not exist"

# --- Workflow hygiene (mirrors moat-trusted-root-check.yml posture) ---
grep -qE '^\s*schedule:' "$WF" || fail "no cron schedule"
grep -qE '^\s*workflow_dispatch:' "$WF" || fail "no workflow_dispatch"
grep -qE '^\s*cancel-in-progress:\s*false' "$WF" || fail "concurrency cancel-in-progress: false missing"
# Top-level permissions: contents: read (first permissions block in the file).
awk '/^permissions:/{found=1; next} found && /contents: read/{ok=1; exit} found && /^[a-z]/{exit} END{exit !ok}' "$WF" \
  || fail "top-level permissions: contents: read missing"
# Every actions/checkout must set persist-credentials: false.
CHECKOUTS="$(grep -cE 'uses:\s*actions/checkout@' "$WF")"
PERSISTS="$(grep -cE 'persist-credentials:\s*false' "$WF")"
[ "$CHECKOUTS" -ge 1 ] || fail "no actions/checkout step"
[ "$PERSISTS" -ge "$CHECKOUTS" ] || fail "a checkout step is missing persist-credentials: false ($PERSISTS/$CHECKOUTS)"
# Every uses: pinned to a full 40-char SHA.
if grep -E '^\s*uses:' "$WF" | grep -vE '@[0-9a-f]{40}'; then
  fail "uses: lines not pinned to a full SHA"
fi
grep -q -- '--body-file' "$WF" || fail "PR body must flow through --body-file, not inline"
grep -qE 'actions/cache@' "$WF" || fail "no actions/cache step for the ETag file"
grep -qE 'Aembit/get-credentials@' "$WF" || fail "no Aembit credential step (design DQ1: A)"
grep -q 'go run ./cmd/capmon-pull' "$WF" || fail "workflow does not invoke go run ./cmd/capmon-pull"
grep -q 'automation/capmon-pull' "$WF" || fail "rolling branch automation/capmon-pull not referenced"
# Injection posture: NO ${{ }} interpolation inside any run: block at all.
# Every piece of dynamic content — feed-derived or otherwise — must reach
# run: bodies via env: or files. This is a whitelist-of-nothing, not a
# blacklist of known-bad output names.
if ! awk '
  function ind(s) { match(s, /[^ ]/); return RSTART }
  # Both step forms: "run:" and "- run:". A single-line run is checked on
  # the spot; a block scalar (run: | / run: >) opens a body whose extent is
  # set by the first body line'"'"'s indent.
  /^[[:space:]]*(-[[:space:]]+)?run:/ {
    if (index($0, "${{")) { print "run-line interpolation: " $0; bad = 1 }
    if ($0 ~ /:[[:space:]]*[|>][+-]?[[:space:]]*$/) { inrun = 1; bodyind = 0 }
    next
  }
  inrun {
    if ($0 !~ /[^[:space:]]/) { next }        # blank lines stay in the block
    i = ind($0)
    if (bodyind == 0) { bodyind = i }         # first body line fixes indent
    if (i < bodyind) { inrun = 0; next }
    if (index($0, "${{")) { print "run-block interpolation: " $0; bad = 1 }
  }
  END { exit bad }
' "$WF"; then
  fail "\${{ }} interpolation inside a run: block; route through env: or files"
fi

# --- Consume-only docs: no references to deleted capmon machinery ---
DOCS=(CONTRIBUTING.md docs/guides/adding-a-provider.md docs/provider-capabilities/README.md)
for doc in "${DOCS[@]}"; do
  [ -f "$doc" ] || fail "$doc missing"
  for banned in 'syllago capmon' 'cli/internal/capmon/' '.capmon-pause'; do
    if grep -qF "$banned" "$doc"; then
      fail "$doc still references deleted capmon machinery: $banned"
    fi
  done
  # capmon.yml (the deleted workflow) is banned; capmon-pull.yml is the new
  # workflow and legitimate.
  if grep -F 'capmon.yml' "$doc" | grep -vF 'capmon-pull.yml' | grep -q .; then
    fail "$doc still references the deleted capmon.yml workflow"
  fi
done
# The README must describe the mirror contract.
grep -qi 'capability feed' docs/provider-capabilities/README.md || fail "README does not describe the Capability Feed"
grep -q 'provenance.json' docs/provider-capabilities/README.md || fail "README does not document provenance.json"

# --- Module still compiles with all entry points ---
cd cli
go build ./... > /dev/null
go vet ./... > /dev/null

echo "slice6 structural checks OK"
