#!/usr/bin/env bash
# helpers.sh — Shared test framework for syllago provider smoke tests.
#
# Source this file from test scripts:
#   source "$(dirname "$0")/helpers.sh"
#
# Provides:
#   - assert_contains, assert_not_contains, assert_file_exists, assert_symlink
#   - setup_test_home (isolated HOME with syllago binary)
#   - cleanup_test_home
#   - run_test (named test runner with pass/fail tracking)
#   - summary (prints results, exits with correct code)

set -euo pipefail

# ── Colors ────────────────────────────────────────────────────────────────────

if [[ -t 1 ]]; then
  GREEN='\033[0;32m'
  RED='\033[0;31m'
  YELLOW='\033[0;33m'
  BOLD='\033[1m'
  RESET='\033[0m'
else
  GREEN='' RED='' YELLOW='' BOLD='' RESET=''
fi

# ── Counters ──────────────────────────────────────────────────────────────────

TESTS_PASSED=0
TESTS_FAILED=0
TESTS_SKIPPED=0
CURRENT_TEST=""
_ASSERT_FAILURES=0

# ── Assertions ────────────────────────────────────────────────────────────────
# All assertions use grep -qF (fixed-string, no eval) for safety.

# Assertions increment _ASSERT_FAILURES on failure. run_test checks this counter
# to determine pass/fail — this avoids relying on set -e (which is inhibited
# inside functions called via "func || status=$?").

_assert_fail() {
  _ASSERT_FAILURES=$((_ASSERT_FAILURES + 1))
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  local msg="${3:-expected output to contain: $needle}"
  if echo "$haystack" | grep -qF "$needle"; then
    return 0
  fi
  echo -e "  ${RED}FAIL${RESET}: $msg"
  echo "  needle:   $needle"
  echo "  haystack: ${haystack:0:200}"
  _assert_fail
}

assert_not_contains() {
  local haystack="$1"
  local needle="$2"
  local msg="${3:-expected output NOT to contain: $needle}"
  if ! echo "$haystack" | grep -qF "$needle"; then
    return 0
  fi
  echo -e "  ${RED}FAIL${RESET}: $msg"
  _assert_fail
}

assert_file_exists() {
  local path="$1"
  local msg="${2:-expected file to exist: $path}"
  if [[ -e "$path" ]]; then
    return 0
  fi
  echo -e "  ${RED}FAIL${RESET}: $msg"
  _assert_fail
}

assert_file_not_exists() {
  local path="$1"
  local msg="${2:-expected file NOT to exist: $path}"
  if [[ ! -e "$path" ]]; then
    return 0
  fi
  echo -e "  ${RED}FAIL${RESET}: $msg"
  _assert_fail
}

assert_symlink() {
  local path="$1"
  local msg="${2:-expected symlink at: $path}"
  if [[ -L "$path" ]]; then
    return 0
  fi
  echo -e "  ${RED}FAIL${RESET}: $msg"
  if [[ -e "$path" ]]; then
    echo "  (exists but is not a symlink)"
  else
    echo "  (does not exist)"
  fi
  _assert_fail
}

assert_json_field() {
  local file="$1"
  local field="$2"  # jq expression like '.mcpServers["example"]'
  local msg="${3:-expected JSON field $field in $file}"
  if [[ ! -f "$file" ]]; then
    echo -e "  ${RED}FAIL${RESET}: $msg (file not found)"
    _assert_fail
    return 0
  fi
  if jq -e "$field" "$file" >/dev/null 2>&1; then
    return 0
  fi
  echo -e "  ${RED}FAIL${RESET}: $msg"
  _assert_fail
}

assert_exit_zero() {
  local msg="${1:-expected command to succeed}"
  shift
  if "$@" >/dev/null 2>&1; then
    return 0
  fi
  echo -e "  ${RED}FAIL${RESET}: $msg"
  echo "  command: $*"
  _assert_fail
}

# ── Test Environment ──────────────────────────────────────────────────────────

SMOKE_TEST_HOME=""
ORIGINAL_HOME=""
REPO_ROOT=""

setup_test_home() {
  ORIGINAL_HOME="$HOME"
  REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

  SMOKE_TEST_HOME=$(mktemp -d "${TMPDIR:-/tmp}/syllago-smoke-XXXXXX")
  export HOME="$SMOKE_TEST_HOME"

  # Ensure syllago binary is available. Build if needed.
  if ! command -v syllago &>/dev/null; then
    echo "Building syllago..."
    (cd "$REPO_ROOT/cli" && make build) || {
      echo -e "${RED}Failed to build syllago${RESET}"
      exit 1
    }
  fi

  # Ensure syllago binary is on PATH
  if [[ -f "$ORIGINAL_HOME/.local/bin/syllago" ]]; then
    export PATH="$ORIGINAL_HOME/.local/bin:$PATH"
  fi

  # Create a .git marker so syllago's findProjectRoot() treats the temp dir
  # as the project root. Snapshots are stored at projectRoot/.syllago/snapshots/,
  # so without this, snapshots would pollute the actual repo.
  mkdir -p "$SMOKE_TEST_HOME/.git"

  # Copy auth credentials from real HOME so CLI tools can authenticate.
  # Claude Code stores SSO credentials in ~/.claude/.credentials.json
  # Gemini CLI stores OAuth credentials in ~/.gemini/oauth_creds.json
  if [[ -f "$ORIGINAL_HOME/.claude/.credentials.json" ]]; then
    mkdir -p "$SMOKE_TEST_HOME/.claude"
    cp "$ORIGINAL_HOME/.claude/.credentials.json" "$SMOKE_TEST_HOME/.claude/.credentials.json"
  fi
  if [[ -f "$ORIGINAL_HOME/.gemini/oauth_creds.json" ]]; then
    mkdir -p "$SMOKE_TEST_HOME/.gemini"
    cp "$ORIGINAL_HOME/.gemini/oauth_creds.json" "$SMOKE_TEST_HOME/.gemini/oauth_creds.json"
    # Gemini CLI also requires auth config in settings.json (security.auth.selectedType).
    # Seed the settings.json with the auth section so introspection commands work.
    if [[ -f "$ORIGINAL_HOME/.gemini/settings.json" ]]; then
      # Extract just the security section to seed auth without copying user's full config
      local auth_json
      auth_json=$(jq '{security: .security}' "$ORIGINAL_HOME/.gemini/settings.json" 2>/dev/null) || true
      if [[ -n "$auth_json" && "$auth_json" != "null" ]]; then
        echo "$auth_json" > "$SMOKE_TEST_HOME/.gemini/settings.json"
      fi
    fi
  fi

  # Preserve Go module cache in the real HOME to avoid re-downloading
  # dependencies and creating read-only files in the temp dir.
  export GOPATH="${ORIGINAL_HOME}/go"
  export GOMODCACHE="${ORIGINAL_HOME}/go/pkg/mod"

  cd "$SMOKE_TEST_HOME"

  echo -e "${BOLD}Test HOME:${RESET} $SMOKE_TEST_HOME"
  echo -e "${BOLD}Repo root:${RESET} $REPO_ROOT"
  echo ""
}

cleanup_test_home() {
  # Ensure any active loadout is removed before cleanup
  if syllago loadout status --json 2>/dev/null | grep -q '"active"'; then
    syllago loadout remove --auto 2>/dev/null || true
  fi

  if [[ -n "$SMOKE_TEST_HOME" && -d "$SMOKE_TEST_HOME" ]]; then
    # Go's module cache uses read-only directories. Fix permissions before removal.
    chmod -R u+w "$SMOKE_TEST_HOME" 2>/dev/null || true
    rm -rf "$SMOKE_TEST_HOME"
  fi

  if [[ -n "$ORIGINAL_HOME" ]]; then
    export HOME="$ORIGINAL_HOME"
  fi
}

# Always clean up on exit
trap cleanup_test_home EXIT

# ── Test Runner ───────────────────────────────────────────────────────────────

run_test() {
  local name="$1"
  shift
  CURRENT_TEST="$name"
  _ASSERT_FAILURES=0
  echo -e "${BOLD}TEST:${RESET} $name"

  # Run the test function. Assertions track failures via _ASSERT_FAILURES
  # (not exit codes, since set -e is inhibited in this context).
  "$@" 2>&1 || true

  if [[ $_ASSERT_FAILURES -eq 0 ]]; then
    echo -e "  ${GREEN}PASS${RESET}"
    TESTS_PASSED=$((TESTS_PASSED + 1))
  else
    echo -e "  ${RED}FAIL${RESET} ($_ASSERT_FAILURES assertion(s))"
    TESTS_FAILED=$((TESTS_FAILED + 1))
  fi
  echo ""
  CURRENT_TEST=""
  return 0  # Don't propagate failure — we track it
}

skip_test() {
  local name="$1"
  local reason="$2"
  echo -e "${BOLD}TEST:${RESET} $name"
  echo -e "  ${YELLOW}SKIP${RESET}: $reason"
  echo ""
  TESTS_SKIPPED=$((TESTS_SKIPPED + 1))
}

# ── Summary ───────────────────────────────────────────────────────────────────

summary() {
  echo -e "${BOLD}─── Results ───${RESET}"
  echo -e "  ${GREEN}Passed:${RESET}  $TESTS_PASSED"
  echo -e "  ${RED}Failed:${RESET}  $TESTS_FAILED"
  echo -e "  ${YELLOW}Skipped:${RESET} $TESTS_SKIPPED"
  echo ""

  if [[ $TESTS_FAILED -gt 0 ]]; then
    echo -e "${RED}${BOLD}SMOKE TESTS FAILED${RESET}"
    exit 1
  else
    echo -e "${GREEN}${BOLD}ALL SMOKE TESTS PASSED${RESET}"
    exit 0
  fi
}

# ── Provider Auth Detection ───────────────────────────────────────────────────
# These check whether a CLI tool is authenticated (SSO session active).
# Used to gate LLM-based assertions that require a live session.

claude_authenticated() {
  # claude -p with a trivial prompt; if SSO session is active this succeeds
  claude -p "say ok" --max-budget-usd 0.01 >/dev/null 2>&1
}

gemini_authenticated() {
  # gemini -p with a trivial prompt; if SSO session is active this succeeds
  gemini -p "say ok" >/dev/null 2>&1
}
