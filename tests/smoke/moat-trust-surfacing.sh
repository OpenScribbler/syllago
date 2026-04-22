#!/usr/bin/env bash
# moat-trust-surfacing.sh — E2E smoke tests for the MOAT signing + verification
# pipeline (syllago-92i4c Phase 4).
#
# Drives all state via real CLI commands:
#   syllago moat sign → syllago registry add → syllago add
#
# Zero mkdir/cat> into paths owned by the CLI (no ~/.syllago surgery).
#
# Note on registry config isolation: the syllago binary bakes in the repo
# root at build time (ldflags -X main.repoRoot), so `registry add` always
# writes config to the real project's content root — regardless of HOME
# overrides. This is expected dev-binary behaviour. The smoke test removes
# its registries before and after each run to stay idempotent.
#
# Usage:
#   ./tests/smoke/moat-trust-surfacing.sh
#
# Prerequisites:
#   - syllago binary built (make build from cli/)
#   - git on PATH

set -euo pipefail
source "$(dirname "$0")/helpers.sh"

echo -e "${BOLD}═══ MOAT Trust Surfacing Smoke Tests ═══${RESET}"
echo ""

# ── Setup ─────────────────────────────────────────────────────────────────────

setup_test_home

# Create an isolated dir for the local git registry (outside HOME so it's not
# picked up by syllago's project-root detection).
REGISTRY_WORK=$(mktemp -d "${TMPDIR:-/tmp}/syllago-moat-reg-XXXXXX")
DEVROOT="${REGISTRY_WORK}/devroot"
mkdir -p "${DEVROOT}"

# Configure git identity for commits inside the test registry.
export GIT_AUTHOR_NAME="smoke"
export GIT_AUTHOR_EMAIL="smoke@syllago.local"
export GIT_COMMITTER_NAME="smoke"
export GIT_COMMITTER_EMAIL="smoke@syllago.local"

# ── Registry cleanup (idempotent) ─────────────────────────────────────────────
# The dev binary writes registry config to the real project content root, not
# the sandboxed HOME. Remove smoke test registries at the start so previous
# crashed runs don't cause REGISTRY_005 "already exists" failures.

_cleanup_registries() {
  syllago registry remove smoke-moat >/dev/null 2>&1 || true
  syllago registry remove smoke-moat-wrong >/dev/null 2>&1 || true
}

_cleanup_registries

# ── Build the local git registry ──────────────────────────────────────────────

_setup_registry() {
  # Content: one rule in syllago format. Rules are provider-specific so the
  # path is rules/<provider>/<name>/ — the scanner treats the first level as
  # provider namespace and the second as item name.
  mkdir -p "${REGISTRY_WORK}/rules/claude-code/smoke-moat-rule"
  cat > "${REGISTRY_WORK}/rules/claude-code/smoke-moat-rule/rule.md" << 'EOF'
---
description: MOAT smoke-test rule — safe to delete
alwaysApply: false
---

# Smoke MOAT Rule

Placeholder rule used by the MOAT E2E smoke fixture. Safe to delete.
EOF

  cat > "${REGISTRY_WORK}/rules/claude-code/smoke-moat-rule/.syllago.yaml" << 'EOF'
id: smoke-moat-rule
name: smoke-moat-rule
description: MOAT smoke-test rule
EOF

  # registry.yaml — syllago content discovery manifest.
  cat > "${REGISTRY_WORK}/registry.yaml" << 'EOF'
name: smoke-moat
description: Dev-mode registry for MOAT E2E smoke tests
version: "1.0.0"
EOF

  # manifest.json — MOAT attestation manifest (signed by syllago moat sign).
  # Byte-exact: sign and verify operate on the same bytes.
  local updated_at
  updated_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  cat > "${REGISTRY_WORK}/manifest.json" << EOF
{
  "schema_version": 1,
  "manifest_uri": "file://smoke-moat",
  "name": "smoke-moat",
  "operator": "dev",
  "updated_at": "${updated_at}",
  "registry_signing_profile": {
    "issuer": "https://dev.syllago.local/oidc",
    "subject": "https://dev.syllago.local/workflow/smoke@refs/heads/main"
  },
  "content": [],
  "revocations": []
}
EOF

  # Init git repo and commit initial content.
  git -C "${REGISTRY_WORK}" init -q
  git -C "${REGISTRY_WORK}" add .
  git -C "${REGISTRY_WORK}" commit -q -m "initial registry content"
}

_setup_registry

# ── Test 1: moat sign (dev mode) ──────────────────────────────────────────────

test_moat_sign_dev() {
  local out
  out=$(syllago moat sign \
    --manifest "${REGISTRY_WORK}/manifest.json" \
    --dev-trusted-root "${DEVROOT}" \
    --out "${REGISTRY_WORK}/manifest.json.sigstore" 2>&1) || {
    echo "  stdout/stderr: $out"
    _assert_fail
    return
  }

  assert_contains "$out" "bundle written to" \
    "moat sign should confirm bundle path"
  assert_contains "$out" "dev trusted root written to" \
    "moat sign should confirm trusted root path"

  assert_file_exists "${REGISTRY_WORK}/manifest.json.sigstore" \
    "manifest.json.sigstore must be written by moat sign"
  assert_file_exists "${DEVROOT}/trusted_root.json" \
    "trusted_root.json must be written by moat sign --dev-trusted-root"
}

run_test "moat sign (dev mode)" test_moat_sign_dev

# ── Commit the bundle so registry add picks it up via git clone ───────────────

git -C "${REGISTRY_WORK}" add manifest.json.sigstore
git -C "${REGISTRY_WORK}" commit -q -m "add MOAT bundle"

# ── Test 2: registry add with dev signing identity ────────────────────────────

test_moat_registry_add() {
  local out
  out=$(syllago registry add "file://${REGISTRY_WORK}" \
    --name smoke-moat \
    --signing-issuer "https://dev.syllago.local/oidc" \
    --signing-identity "https://dev.syllago.local/workflow/smoke@refs/heads/main" \
    2>&1) || {
    echo "  stdout/stderr: $out"
    _assert_fail
    return
  }

  # The config is written to the real project's content root (baked-in repoRoot).
  # The clone goes to $HOME/.syllago/registries/ where HOME=$SMOKE_TEST_HOME.
  assert_file_exists "${HOME}/.syllago/registries/smoke-moat" \
    "registry clone must exist after registry add"
  assert_file_exists "${HOME}/.syllago/registries/smoke-moat/manifest.json.sigstore" \
    "bundle must be present in cloned registry"
  assert_file_exists "${HOME}/.syllago/registries/smoke-moat/manifest.json" \
    "manifest.json must be present in cloned registry"
}

run_test "registry add (dev signing identity)" test_moat_registry_add

# ── Test 3: syllago add --all verifies manifest and adds content ──────────────

test_moat_add_verified() {
  local out
  out=$(syllago add --all \
    --from smoke-moat \
    --trusted-root "${DEVROOT}/trusted_root.json" \
    --no-input 2>&1) || {
    echo "  stdout/stderr: $out"
    _assert_fail
    return
  }

  # Verification was attempted against the dev trusted root.
  # The trust label ("signed") appears on stdout per emitTrustLabel.
  assert_contains "$out" "signed" \
    "syllago add should emit 'signed' trust label after successful MOAT verification"

  # Content must land in the library ($HOME/.syllago/content/ with HOME=$SMOKE_TEST_HOME).
  assert_file_exists "${HOME}/.syllago/content/rules/smoke-moat-rule" \
    "rule must be added to library after verified install"
}

run_test "syllago add --all (MOAT verified, dev trusted root)" test_moat_add_verified

# ── Test 4: syllago add rejects a mismatched signing identity ─────────────────

test_moat_add_wrong_profile_rejected() {
  # Add the registry under a different name with the wrong signing identity.
  # registry add itself doesn't verify — verification happens at add time.
  syllago registry add "file://${REGISTRY_WORK}" \
    --name smoke-moat-wrong \
    --signing-issuer "https://dev.syllago.local/oidc" \
    --signing-identity "https://dev.syllago.local/workflow/WRONG@refs/heads/main" \
    >/dev/null 2>&1 || {
    echo "  registry add (wrong profile) unexpectedly failed during clone"
    _assert_fail
    return
  }

  local out
  local exit_code=0
  out=$(syllago add --all \
    --from smoke-moat-wrong \
    --trusted-root "${DEVROOT}/trusted_root.json" \
    --no-input 2>&1) || exit_code=$?

  if [[ $exit_code -eq 0 ]]; then
    echo -e "  ${RED}FAIL${RESET}: syllago add should have rejected the mismatched identity"
    _assert_fail
    return
  fi

  assert_contains "$out" "MOAT" \
    "error output should include a MOAT error code"
}

run_test "syllago add (wrong identity rejected)" test_moat_add_wrong_profile_rejected

# ── Cleanup ────────────────────────────────────────────────────────────────────

# Remove smoke registries from the real project config. This is required
# because the dev binary uses a baked-in repoRoot and writes registry config
# to the real project's content root, not the sandboxed HOME.
_cleanup_registries

# Registry work dir cleanup (SMOKE_TEST_HOME is cleaned by trap in helpers.sh).
rm -rf "${REGISTRY_WORK}"

# ── Summary ───────────────────────────────────────────────────────────────────

summary
