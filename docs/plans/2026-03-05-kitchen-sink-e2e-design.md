# Kitchen Sink E2E Test Repo - Design Document

**Goal:** Create a standalone test fixture repo that exercises all 11 providers with native-format content, enabling automated regression testing and consistent manual testing for the full syllago content lifecycle.

**Decision Date:** 2026-03-05

---

## Problem Statement

The `syllago add` command was redesigned with discovery mode, positional args, and item-level discovery. Manual E2E testing across 11 providers is ad-hoc — there's no consistent fixture set covering all providers, content types, and format quirks. Bugs in format conversion (e.g., Windsurf trigger fields, Cline paths syntax, OpenCode array commands) are only caught when someone manually tests that specific provider.

## Proposed Solution

Create `OpenScribbler/syllago-kitchen-sink` — a "polyglot AI project" with all 11 providers configured simultaneously using native file formats. Shell-based test harness exercises the compiled `syllago` binary exactly as users do. Lives at `~/.local/src/syllago-kitchen-sink` during development.

## Architecture

### Two colocated concerns

1. **Fixture files** (repo root): Native-format content for all 11 providers. Acts as a realistic multi-provider project that syllago discovers content from.
2. **Test harness** (`tests/`): Shell scripts that run `syllago` commands against the fixtures, assert expected output. No Go toolchain needed.

### Isolation strategy

Tests override `$HOME` to a temp directory (`mktemp -d`) so `~/.syllago/` library writes don't pollute the real system. Cleaned up on exit via `trap`.

### Three canonical items across providers

Each appears in native format for every provider that supports that content type:

| Item | Content Type | Exercises |
|------|-------------|-----------|
| **security** | rule | Glob scoping, frontmatter variants (.mdc, paths:, trigger:, globs:) |
| **code-reviewer** | agent | TOML (Codex), JSON (Kiro), YAML (Roo), Markdown (most) |
| **greeting** | skill | Directory-based items (SKILL.md inside subdirs) |

Plus MCP configs (JSON/JSONC variations) and hooks (Claude/Gemini/Copilot formats).

## Key Decisions

| Decision | Choice | Reasoning |
|----------|--------|-----------|
| Repo location | `OpenScribbler/syllago-kitchen-sink` (GitHub), `~/.local/src/` (local) | Separate repo = independent CI schedule, no coupling to syllago release |
| Test language | Shell (bash) | Tests CLI behavior at the user layer; no Go toolchain needed to run |
| Isolation | `$HOME` override + temp dir | Prevents pollution of real `~/.syllago/` library |
| Fixture format | Native per-provider (not canonical) | Tests the full discovery → canonicalize pipeline |
| Binary path | `$SYLLAGO_BIN` env var or `--syllago-path` flag, defaults to `syllago` on PATH | Flexible for CI (freshly built binary) and manual use |

## Provider Coverage Matrix

| Provider | Rules | Agents | Skills | Commands | Hooks | MCP |
|----------|:-----:|:------:|:------:|:--------:|:-----:|:---:|
| Claude Code | Y | Y | Y | Y | Y | Y |
| Gemini CLI | Y | Y | Y | Y | Y | Y |
| Cursor | Y | | | | | |
| Windsurf | Y | | | | | |
| Codex | Y | Y | | Y | | |
| Copilot CLI | Y | Y | | Y | Y | Y |
| Zed | Y | | | | | |
| Cline | Y | | | | | |
| Roo Code | Y | Y | | | | Y |
| OpenCode | Y | Y | Y | Y | | Y |
| Kiro | Y | Y | Y | | | Y |

## Edge Cases Exercised

| Edge Case | Fixture File | What It Tests |
|---|---|---|
| Cursor `.mdc` frontmatter | `.cursor/rules/security.mdc` | YAML frontmatter with `globs:` as YAML list |
| Windsurf `trigger:` variants | `.windsurfrules` | `trigger: always_on`, globs as comma-separated string |
| Cline `paths:` → canonical `globs:` | `.clinerules/security.md` | `paths:` YAML list mapping |
| Roo Code mode-specific dirs | `.roo/rules-code/` | 7 discovery paths (rules/, rules-code/, etc.) |
| `.roorules` + `.roo/rules/` coexist | Both present | Flat file AND directory discovery |
| Codex TOML agent structure | `.codex/agents/code-reviewer.toml` | `[features]` header, `[agents.slug]` sections |
| OpenCode `mcp` key (not `mcpServers`) | `opencode.json` | `command` as array, `environment` key |
| Kiro agent JSON + separate prompt file | `.kiro/agents/code-reviewer.json` | `prompt: "file://..."` reference pattern |
| Kiro steering = rules AND skills | `.kiro/steering/` | Same path discovers both content types |
| Copilot flat hooks (`bash` key) | `.copilot/hooks.json` | camelCase events, no matchers, timeoutSec |
| Gemini hooks+MCP shared settings.json | `.gemini/settings.json` | Single file with both `hooks` and `mcpServers` |
| Shared `AGENTS.md` | `AGENTS.md` | Discovered by both Codex and OpenCode |

## Repo Structure

```
syllago-kitchen-sink/
├── README.md
├── tests/
│   ├── run.sh              # entry point (CI + manual)
│   ├── lib.sh              # shared assertion helpers
│   ├── test_discovery.sh   # discovery mode for all providers
│   ├── test_add.sh         # add from each provider (format conversion)
│   ├── test_install.sh     # add → install round-trips
│   └── test_convert.sh     # convert between formats
│
├── # Claude Code
├── CLAUDE.md
├── .claude.json
├── .claude/settings.json
├── .claude/rules/security.md, code-review.md
├── .claude/skills/greeting/SKILL.md
├── .claude/agents/code-reviewer/agent.md
├── .claude/commands/summarize.md
│
├── # Gemini CLI
├── GEMINI.md
├── .gemini/settings.json
├── .gemini/skills/greeting/SKILL.md
├── .gemini/agents/code-reviewer/agent.md
├── .gemini/commands/summarize.md
│
├── # Cursor
├── .cursor/rules/security.mdc, code-review.mdc
│
├── # Windsurf
├── .windsurfrules
│
├── # Codex + OpenCode (shared)
├── AGENTS.md
├── .codex/agents/code-reviewer.toml
│
├── # Copilot CLI
├── .github/copilot-instructions.md
├── .copilot/agents/code-reviewer.md
├── .copilot/commands/summarize.md
├── .copilot/mcp.json
├── .copilot/hooks.json
│
├── # Zed
├── .rules
│
├── # Cline
├── .clinerules/security.md, code-review.md
│
├── # Roo Code
├── .roorules
├── .roo/rules/security.md
├── .roo/rules-code/code-review.md
├── .roo/mcp.json
│
├── # OpenCode
├── opencode.json
├── .opencode/skill/greeting/SKILL.md
├── .opencode/agents/code-reviewer.md
├── .opencode/commands/summarize.md
│
├── # Kiro
├── .kiro/steering/security.md, greeting.md
├── .kiro/agents/code-reviewer.json
├── .kiro/prompts/code-reviewer.md
└── .kiro/settings/mcp.json
```

## Test Harness Design

### `lib.sh` — Shared utilities

- `setup_test_home`: Creates temp dir, exports `HOME`, sets trap for cleanup
- `assert_exit_zero CMD...`: Run command, fail if non-zero exit
- `assert_output_contains CMD PATTERN`: Run command, grep output for pattern
- `assert_file_exists PATH`: Fail if file doesn't exist
- `assert_json_key FILE KEY`: Use `jq` to verify JSON key exists
- `print_summary`: Print pass/fail counts, exit non-zero if any failures

### `test_discovery.sh` — Discovery mode

For each provider, run `syllago add --from <provider>` (no positional arg = discovery mode) and verify expected items appear in output.

### `test_add.sh` — Add with format conversion

For each provider, run `syllago add --from <provider> --force` and verify files written to library with correct canonical format.

### `test_install.sh` — Add → install round-trips

Add from claude-code, then install to 6+ target providers. Verify target-format files created at expected paths.

### `test_convert.sh` — Cross-format conversion

Convert items between formats, verify output content matches expected structure.

## Success Criteria

1. `./tests/run.sh` exits 0 with "ALL SUITES PASSED" on a clean system
2. Each provider's native content is discovered correctly by `syllago add --from <provider>`
3. Format conversions produce valid target-format files (e.g., .mdc frontmatter intact)
4. Tests are isolated — no writes to real `~/.syllago/`
5. Manual exploration works: `cd syllago-kitchen-sink && syllago add --from cursor` shows Cursor content

## Open Questions

None — all format details verified against source code.

---

## Next Steps

Ready for implementation planning with `Plan` skill.
