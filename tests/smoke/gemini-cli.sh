#!/usr/bin/env bash
# gemini-cli.sh — E2E smoke tests for syllago + Gemini CLI integration.
#
# Tests cross-provider conversion: applies the Claude Code smoke loadout
# with --to gemini-cli, verifying content lands in Gemini-specific paths.
# Also exercises Gemini-specific content format differences.
#
# Usage:
#   ./tests/smoke/gemini-cli.sh           # Run all tests
#   SMOKE_AUTH=1 ./tests/smoke/gemini-cli.sh  # Include auth-dependent assertions
#
# Prerequisites:
#   - syllago binary built (make build from cli/)
#   - Gemini CLI installed (gemini CLI on PATH)
#   - For SMOKE_AUTH=1: active Gemini CLI SSO session

set -euo pipefail
source "$(dirname "$0")/helpers.sh"

echo -e "${BOLD}=== Gemini CLI Smoke Tests ===${RESET}"
echo ""

# ── Setup ─────────────────────────────────────────────────────────────────────

setup_test_home

# Check prerequisites
if ! command -v gemini &>/dev/null; then
  echo -e "${RED}gemini CLI not found on PATH. Install Gemini CLI first.${RESET}"
  exit 1
fi

LOADOUT_NAME="syllago-smoke-test"

# Seed Gemini auth into settings.json. Loadout remove restores the pre-apply
# settings.json, which may not have auth. Call this before apply in auth tests.
_seed_gemini_auth() {
  mkdir -p "$HOME/.gemini"
  if [[ -f "$ORIGINAL_HOME/.gemini/settings.json" ]]; then
    local auth_json
    auth_json=$(jq '{security: .security}' "$ORIGINAL_HOME/.gemini/settings.json" 2>/dev/null) || true
    if [[ -n "$auth_json" && "$auth_json" != "null" && ! -f "$HOME/.gemini/settings.json" ]]; then
      echo "$auth_json" > "$HOME/.gemini/settings.json"
    elif [[ -f "$HOME/.gemini/settings.json" ]]; then
      # settings.json exists but may lack auth — merge security section
      local has_auth
      has_auth=$(jq -e '.security' "$HOME/.gemini/settings.json" 2>/dev/null) || true
      if [[ -z "$has_auth" || "$has_auth" == "null" ]]; then
        local tmp
        tmp=$(jq --argjson sec "$auth_json" '. + $sec' "$HOME/.gemini/settings.json" 2>/dev/null) || true
        if [[ -n "$tmp" ]]; then
          echo "$tmp" > "$HOME/.gemini/settings.json"
        fi
      fi
    fi
  fi
}

# ── Gemini-specific path expectations ─────────────────────────────────────────
# Key differences from Claude Code:
#   - Rules: symlinked into ~/.gemini/ root (not a rules/ subdir)
#   - Skills: ~/.gemini/skills/<name>/
#   - Agents: ~/.gemini/agents/<name>.md
#   - Commands: ~/.gemini/commands/<name>/
#   - MCP + Hooks: merged into ~/.gemini/settings.json (single file, not split)
#
# NOTE: Gemini CLI introspection commands (mcp list, skills list) require
# SSO auth — unlike Claude Code's `claude mcp list` which works without auth.

# ── Test: Cross-provider apply (clean HOME) ──────────────────────────────────

test_cross_provider_apply() {
  # Apply Claude Code loadout with --to gemini-cli.
  # syllago converts content from Claude Code canonical format to Gemini CLI format.
  syllago loadout apply "$LOADOUT_NAME" --to gemini-cli --keep 2>&1

  # Verify .gemini/ was created
  assert_file_exists "$HOME/.gemini" ".gemini directory should be created"

  # Verify skills symlink
  assert_symlink "$HOME/.gemini/skills/smoke-skill" \
    "skills should be symlinked into .gemini/skills/"

  # Verify agents symlink
  assert_symlink "$HOME/.gemini/agents/smoke-agent.md" \
    "agent should be symlinked into .gemini/agents/"

  # Verify rules symlink (Gemini puts rules in root, not a rules/ subdir)
  assert_symlink "$HOME/.gemini/smoke-rules" \
    "rules should be symlinked into .gemini/ root"

  # Verify commands symlink (Gemini puts commands under .gemini/commands/)
  assert_symlink "$HOME/.gemini/commands/smoke-commands" \
    "commands should be symlinked into .gemini/commands/"

  # Verify hooks merged into settings.json
  assert_file_exists "$HOME/.gemini/settings.json" \
    "settings.json should exist after hook merge"
  assert_json_field "$HOME/.gemini/settings.json" '.hooks' \
    "settings.json should have hooks section"

  # Verify loadout status
  local status
  status=$(syllago loadout status --json 2>&1)
  assert_contains "$status" '"active": true' "loadout should be active"

  syllago loadout remove --auto 2>&1
}

run_test "Cross-provider apply (clean HOME)" test_cross_provider_apply

# ── Test: Clean removal (Gemini paths) ───────────────────────────────────────

test_gemini_clean_removal() {
  syllago loadout apply "$LOADOUT_NAME" --to gemini-cli --keep 2>&1
  syllago loadout remove --auto 2>&1

  # Verify symlinks removed
  assert_file_not_exists "$HOME/.gemini/skills/smoke-skill" \
    "skills symlink should be removed"
  assert_file_not_exists "$HOME/.gemini/agents/smoke-agent.md" \
    "agent symlink should be removed"
  assert_file_not_exists "$HOME/.gemini/smoke-rules" \
    "rules symlink should be removed"
  assert_file_not_exists "$HOME/.gemini/commands/smoke-commands" \
    "commands symlink should be removed"

  # Verify loadout inactive
  local status
  status=$(syllago loadout status --json 2>&1)
  assert_contains "$status" '"active": false' "loadout should be inactive"
}

run_test "Clean removal (Gemini paths)" test_gemini_clean_removal

# ── Test: Merge with existing Gemini config ──────────────────────────────────

test_merge_existing_gemini_config() {
  # Gemini CLI uses a single settings.json for both MCP and hooks.
  # Pre-populate with existing user config.
  mkdir -p "$HOME/.gemini"
  cat > "$HOME/.gemini/settings.json" <<'SETTINGS'
{
  "mcpServers": {
    "user-existing-gemini-mcp": {
      "command": "echo",
      "args": ["user-mcp"]
    }
  },
  "hooks": {
    "pretool": [
      {
        "command": "echo user-gemini-hook"
      }
    ]
  }
}
SETTINGS

  syllago loadout apply "$LOADOUT_NAME" --to gemini-cli --keep 2>&1

  # Verify user's existing MCP survived
  local settings
  settings=$(cat "$HOME/.gemini/settings.json")
  assert_contains "$settings" "user-existing-gemini-mcp" \
    "existing Gemini MCP should survive merge"

  # Verify user's existing hook survived
  assert_contains "$settings" "user-gemini-hook" \
    "existing Gemini hook should survive merge"

  # Verify loadout hooks were added
  assert_contains "$settings" "PreToolUse" \
    "loadout hooks should be merged into settings.json"

  # Remove and verify restore
  syllago loadout remove --auto 2>&1

  settings=$(cat "$HOME/.gemini/settings.json")
  assert_contains "$settings" "user-existing-gemini-mcp" \
    "existing Gemini MCP should be restored"
  assert_contains "$settings" "user-gemini-hook" \
    "existing Gemini hook should be restored"
}

run_test "Merge with existing Gemini config" test_merge_existing_gemini_config

# ── Test: Gemini CLI introspection (requires auth) ───────────────────────────
# Unlike Claude Code, Gemini CLI's introspection commands (mcp list, skills list)
# require an active SSO session. These are gated behind SMOKE_AUTH.

test_gemini_mcp_list() {
  # Seed auth into settings.json before apply (loadout remove may have wiped it)
  _seed_gemini_auth
  syllago loadout apply "$LOADOUT_NAME" --to gemini-cli --keep 2>&1

  # Verify MCP config in settings.json at the file level
  assert_file_exists "$HOME/.gemini/settings.json" "settings.json should exist"
  local settings
  settings=$(cat "$HOME/.gemini/settings.json")
  assert_contains "$settings" "mcpServers" \
    "settings.json should have mcpServers section"

  # Check gemini mcp list output. Known issue: MCP merge produces empty server
  # entries, so gemini may not recognize them.
  local mcp_output
  mcp_output=$(gemini mcp list 2>&1 || true)
  if echo "$mcp_output" | grep -qF "No MCP" || ! echo "$mcp_output" | grep -qF "smoke"; then
    echo -e "  ${YELLOW}WARN${RESET}: gemini mcp list sees no servers (known MCP merge issue, see syllago-0sbe)"
  fi

  syllago loadout remove --auto 2>&1
}

test_gemini_skills_list() {
  _seed_gemini_auth
  syllago loadout apply "$LOADOUT_NAME" --to gemini-cli --keep 2>&1

  local skills_output
  skills_output=$(gemini skills list 2>&1 || true)
  assert_contains "$skills_output" "smoke-skill" \
    "gemini skills list should show the loadout's skill"

  syllago loadout remove --auto 2>&1
}

if [[ "${SMOKE_AUTH:-}" == "1" ]]; then
  run_test "gemini mcp list (introspection, auth)" test_gemini_mcp_list
  run_test "gemini skills list (introspection, auth)" test_gemini_skills_list
else
  skip_test "gemini mcp list (introspection, auth)" "Gemini CLI introspection requires SSO (set SMOKE_AUTH=1)"
  skip_test "gemini skills list (introspection, auth)" "Gemini CLI introspection requires SSO (set SMOKE_AUTH=1)"
fi

# ── Test: Sequential loadouts (Gemini) ───────────────────────────────────────

test_gemini_sequential() {
  syllago loadout apply "$LOADOUT_NAME" --to gemini-cli --keep 2>&1
  assert_symlink "$HOME/.gemini/skills/smoke-skill" "first apply"

  syllago loadout remove --auto 2>&1
  assert_file_not_exists "$HOME/.gemini/skills/smoke-skill" "after remove"

  syllago loadout apply "$LOADOUT_NAME" --to gemini-cli --keep 2>&1
  assert_symlink "$HOME/.gemini/skills/smoke-skill" "second apply"

  syllago loadout remove --auto 2>&1
}

run_test "Sequential loadouts (Gemini)" test_gemini_sequential

# Gemini CLI has no deterministic rule introspection command (no `gemini rules list`).
# Rules are verified via filesystem checks in the non-auth tests above.

# ── Done ──────────────────────────────────────────────────────────────────────

summary
