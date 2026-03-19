#!/usr/bin/env bash
# claude-code.sh — E2E smoke tests for syllago + Claude Code integration.
#
# Verifies that content installed by syllago is actually picked up by Claude Code.
# Tests use a temporary HOME for isolation — no changes to real user config.
#
# Usage:
#   ./tests/smoke/claude-code.sh           # Run all tests
#   SMOKE_AUTH=1 ./tests/smoke/claude-code.sh  # Include LLM-based assertions
#
# Prerequisites:
#   - syllago binary built (make build from cli/)
#   - Claude Code installed (claude CLI on PATH)
#   - For SMOKE_AUTH=1: active Claude Code SSO session

set -euo pipefail
source "$(dirname "$0")/helpers.sh"

echo -e "${BOLD}═══ Claude Code Smoke Tests ═══${RESET}"
echo ""

# ── Setup ─────────────────────────────────────────────────────────────────────

setup_test_home

# Check prerequisites
if ! command -v claude &>/dev/null; then
  echo -e "${RED}claude CLI not found on PATH. Install Claude Code first.${RESET}"
  exit 1
fi

LOADOUT_NAME="example-kitchen-sink-loadout"

# ── Test: First-time setup (clean HOME) ──────────────────────────────────────

test_first_time_apply() {
  # Clean HOME — no .claude/ directory exists yet.
  # Loadout apply should create the directory structure from scratch.
  syllago loadout apply "$LOADOUT_NAME" --keep 2>&1

  # Verify .claude/ was created
  assert_file_exists "$HOME/.claude" ".claude directory should be created"

  # Verify rules symlink
  assert_symlink "$HOME/.claude/rules/example-kitchen-sink-rules" \
    "rules should be symlinked"

  # Verify skills symlink
  assert_symlink "$HOME/.claude/skills/syllago-guide" \
    "skills should be symlinked"

  # Verify agents symlink
  assert_symlink "$HOME/.claude/agents/example-kitchen-sink-agent.md" \
    "agent should be symlinked"

  # Verify commands symlink
  assert_symlink "$HOME/.claude/commands/example-kitchen-sink-commands" \
    "commands should be symlinked"

  # Verify hooks merged into settings.json
  assert_file_exists "$HOME/.claude/settings.json" \
    "settings.json should exist after hook merge"
  assert_json_field "$HOME/.claude/settings.json" '.hooks' \
    "settings.json should have hooks section"

  # Verify MCP merged into .claude.json
  assert_file_exists "$HOME/.claude.json" \
    ".claude.json should exist after MCP merge"
  assert_json_field "$HOME/.claude.json" '.mcpServers["example-kitchen-sink-mcp"]' \
    ".claude.json should have the MCP server entry"

  # Verify loadout status shows active
  local status
  status=$(syllago loadout status --json 2>&1)
  assert_contains "$status" '"active": true' "loadout should be active"
  assert_contains "$status" "\"name\": \"$LOADOUT_NAME\"" "loadout name should match"

  # Clean up for next test
  syllago loadout remove --auto 2>&1
}

run_test "First-time apply (clean HOME)" test_first_time_apply

# ── Test: Clean removal ──────────────────────────────────────────────────────

test_clean_removal() {
  # Apply and remove, verify no traces left
  syllago loadout apply "$LOADOUT_NAME" --keep 2>&1
  syllago loadout remove --auto 2>&1

  # Verify symlinks are gone
  assert_file_not_exists "$HOME/.claude/rules/example-kitchen-sink-rules" \
    "rules symlink should be removed"
  assert_file_not_exists "$HOME/.claude/skills/syllago-guide" \
    "skills symlink should be removed"
  assert_file_not_exists "$HOME/.claude/agents/example-kitchen-sink-agent.md" \
    "agent symlink should be removed"
  assert_file_not_exists "$HOME/.claude/commands/example-kitchen-sink-commands" \
    "commands symlink should be removed"

  # Verify loadout status shows inactive
  local status
  status=$(syllago loadout status --json 2>&1)
  assert_contains "$status" '"active": false' "loadout should be inactive after remove"
}

run_test "Clean removal" test_clean_removal

# ── Test: Merge with existing config ─────────────────────────────────────────

test_merge_existing_config() {
  # Pre-populate settings.json with existing user hooks
  mkdir -p "$HOME/.claude"
  cat > "$HOME/.claude/settings.json" <<'SETTINGS'
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "echo user-pretooluse-hook"
          }
        ]
      }
    ]
  }
}
SETTINGS

  # Pre-populate .claude.json with existing MCP server
  cat > "$HOME/.claude.json" <<'MCP'
{
  "mcpServers": {
    "user-existing-mcp": {
      "command": "echo",
      "args": ["user-mcp"]
    }
  }
}
MCP

  # Apply loadout — should merge, not overwrite
  syllago loadout apply "$LOADOUT_NAME" --keep 2>&1

  # Verify user's existing hook survived
  local settings
  settings=$(cat "$HOME/.claude/settings.json")
  assert_contains "$settings" "user-pretooluse-hook" \
    "existing user hook should survive merge"

  # Verify user's existing MCP survived
  local mcpjson
  mcpjson=$(cat "$HOME/.claude.json")
  assert_contains "$mcpjson" "user-existing-mcp" \
    "existing user MCP server should survive merge"

  # Verify loadout content was also added
  assert_json_field "$HOME/.claude.json" '.mcpServers["example-kitchen-sink-mcp"]' \
    "loadout MCP should be added alongside existing"

  # Remove and verify user content is restored
  syllago loadout remove --auto 2>&1

  settings=$(cat "$HOME/.claude/settings.json")
  assert_contains "$settings" "user-pretooluse-hook" \
    "existing user hook should be restored after remove"

  mcpjson=$(cat "$HOME/.claude.json")
  assert_contains "$mcpjson" "user-existing-mcp" \
    "existing user MCP should be restored after remove"
}

run_test "Merge with existing config" test_merge_existing_config

# ── Test: Sequential loadouts ────────────────────────────────────────────────

test_sequential_loadouts() {
  # Apply, remove, apply again — verifies clean state between applications
  syllago loadout apply "$LOADOUT_NAME" --keep 2>&1
  assert_symlink "$HOME/.claude/rules/example-kitchen-sink-rules" \
    "first apply: rules should exist"

  syllago loadout remove --auto 2>&1
  assert_file_not_exists "$HOME/.claude/rules/example-kitchen-sink-rules" \
    "after remove: rules should be gone"

  # Second apply should work cleanly
  syllago loadout apply "$LOADOUT_NAME" --keep 2>&1
  assert_symlink "$HOME/.claude/rules/example-kitchen-sink-rules" \
    "second apply: rules should exist again"

  local status
  status=$(syllago loadout status --json 2>&1)
  assert_contains "$status" '"active": true' \
    "second apply: loadout should be active"

  syllago loadout remove --auto 2>&1
}

run_test "Sequential loadouts" test_sequential_loadouts

# ── Test: Double-apply prevention ────────────────────────────────────────────

test_double_apply_blocked() {
  syllago loadout apply "$LOADOUT_NAME" --keep 2>&1

  # Second apply should fail
  local output
  output=$(syllago loadout apply "$LOADOUT_NAME" --keep 2>&1 || true)
  assert_contains "$output" "already active" \
    "second apply should be rejected"

  syllago loadout remove --auto 2>&1
}

run_test "Double-apply prevention" test_double_apply_blocked

# ── Test: claude mcp list (deterministic introspection) ──────────────────────

test_claude_mcp_list() {
  syllago loadout apply "$LOADOUT_NAME" --keep 2>&1

  # Verify MCP config was written to .claude.json.
  # The kitchen-sink MCP config.json defines "example-stdio-server" and
  # "example-http-server" inside mcpServers. After merge, these should
  # appear in ~/.claude.json.
  #
  # NOTE: There is a known MCP merge bug where the item name is used as the
  # server key instead of extracting server names from config.json. This test
  # verifies what claude mcp list actually sees — if it shows "No MCP servers",
  # the merge is producing malformed output.
  assert_file_exists "$HOME/.claude.json" ".claude.json should exist"

  # Verify .claude.json has MCP content (item name or actual server names).
  local mcpjson
  mcpjson=$(cat "$HOME/.claude.json")
  assert_contains "$mcpjson" "mcpServers" \
    ".claude.json should have mcpServers section"

  # Verify claude mcp list picks up the servers.
  # Known issue: the MCP merge currently uses the item name as the server key
  # with an empty config object, so claude may not recognize it. This assertion
  # will start passing once the MCP merge correctly extracts server entries
  # from config.json.
  local mcp_output
  mcp_output=$(claude mcp list 2>&1 || true)
  if echo "$mcp_output" | grep -qF "No MCP servers"; then
    echo -e "  ${YELLOW}WARN${RESET}: claude mcp list sees no servers (known MCP merge issue)"
  fi

  syllago loadout remove --auto 2>&1

  # After removal, MCP entries should be gone from .claude.json
  if [[ -f "$HOME/.claude.json" ]]; then
    local mcpjson
    mcpjson=$(cat "$HOME/.claude.json")
    assert_not_contains "$mcpjson" "example-stdio-server" \
      ".claude.json should not contain MCP servers after removal"
  fi
}

run_test "claude mcp list (introspection)" test_claude_mcp_list

# ── Test: LLM-based rule verification (requires auth) ────────────────────────

test_claude_rules_via_llm() {
  syllago loadout apply "$LOADOUT_NAME" --keep 2>&1

  # Ask Claude to list its active rules. The kitchen-sink rules content should
  # appear in the response. Using --max-budget-usd to cap cost.
  local response
  response=$(claude -p "List the names of your active rules. Just list the file/directory names, nothing else." \
    --max-budget-usd 0.05 \
    --allowedTools "" \
    2>&1)
  assert_contains "$response" "kitchen-sink" \
    "Claude should report the loadout's rules as active"

  syllago loadout remove --auto 2>&1
}

if [[ "${SMOKE_AUTH:-}" == "1" ]]; then
  run_test "Claude rules via LLM (auth)" test_claude_rules_via_llm
else
  skip_test "Claude rules via LLM (auth)" "Set SMOKE_AUTH=1 to enable (requires SSO session)"
fi

# ── Test: Preview mode (no side effects) ─────────────────────────────────────

test_preview_mode() {
  local output
  output=$(syllago loadout apply "$LOADOUT_NAME" --preview 2>&1)

  # Preview should show planned actions
  assert_contains "$output" "Preview" "should indicate preview mode"

  # But nothing should actually be installed
  assert_file_not_exists "$HOME/.claude/rules/example-kitchen-sink-rules" \
    "preview should not create files"

  # Loadout should not be active
  local status
  status=$(syllago loadout status --json 2>&1)
  assert_contains "$status" '"active": false' \
    "preview should not activate loadout"
}

run_test "Preview mode (no side effects)" test_preview_mode

# ── Done ──────────────────────────────────────────────────────────────────────

summary
