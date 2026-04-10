# Kitchen Sink E2E Test Repo - Design Document

**Goal:** Create a standalone E2E test repo (OpenScribbler/syllago-kitchen-sink) that validates syllago's full content lifecycle across all 11 providers using golden file comparison against provider-sourced format specs.

**Decision Date:** 2026-03-06

---

## Problem Statement

Syllago supports 11 providers with complex format conversions, but E2E testing is ad-hoc. The recent `add` command redesign (discovery mode, positional args, item-level discovery) changed the primary user-facing workflow, and there's no automated way to verify the full content lifecycle across all providers.

**Goals:**
1. Regression coverage -- catch breakage in discovery, add, install, export, and convert across all 11 providers
2. Realistic testing -- exercise the compiled binary exactly as users would, in a polyglot project with all providers configured simultaneously
3. Developer experience -- fast to run locally, no Go toolchain required, clear pass/fail output
4. CI-ready -- can run as a daily canary against the latest syllago release (CI integration is follow-up work)

**Success criteria:**
- `./tests/run.sh` exits 0 when all providers work correctly
- A breaking change to any provider's discovery, add, install, or convert path causes a test failure
- New contributor can clone the repo and run tests in under 60 seconds

## Proposed Solution

A standalone GitHub repo (`OpenScribbler/syllago-kitchen-sink`) that acts as a "polyglot AI project" with all 11 providers configured simultaneously. Shell-based test harness exercises the `syllago` binary. Golden files sourced from official provider documentation serve as ground truth for format validation.

## Architecture

### Repo Structure

```
syllago-kitchen-sink/
├── README.md
├── docs/
│   └── provider-specs/           # Research docs per provider (cited sources)
│       ├── claude-code.md
│       ├── gemini-cli.md
│       ├── cursor.md
│       ├── windsurf.md
│       ├── codex.md
│       ├── copilot-cli.md
│       ├── zed.md
│       ├── cline.md
│       ├── roo-code.md
│       ├── opencode.md
│       └── kiro.md
├── tests/
│   ├── run.sh                    # Entry point (CI + manual)
│   ├── lib.sh                    # Assertion helpers + setup/teardown
│   ├── test_discovery.sh         # Discovery mode for all providers
│   ├── test_add.sh               # Add from each provider
│   ├── test_install.sh           # Add -> install round-trips
│   ├── test_convert.sh           # Convert between formats
│   └── golden/                   # Expected output per provider (from specs)
│       ├── claude-code/
│       ├── gemini-cli/
│       ├── cursor/
│       ├── windsurf/
│       ├── codex/
│       ├── copilot-cli/
│       ├── zed/
│       ├── cline/
│       ├── roo-code/
│       ├── opencode/
│       └── kiro/
│
├── # -- Provider Fixtures (native format per provider) --
├── CLAUDE.md
├── .claude/
│   ├── settings.json
│   ├── rules/security.md
│   ├── skills/greeting/SKILL.md
│   ├── agents/code-reviewer.md
│   └── commands/summarize.md
├── GEMINI.md
├── .gemini/
│   ├── settings.json
│   ├── skills/greeting/SKILL.md
│   ├── agents/code-reviewer.md
│   └── commands/summarize.md
├── .cursor/rules/security.mdc, code-review.mdc
├── .windsurfrules
├── AGENTS.md                     # Shared: Codex + OpenCode
├── .codex/agents/code-reviewer.toml
├── .github/copilot-instructions.md
├── .copilot/
│   ├── agents/code-reviewer.md
│   ├── commands/summarize.md
│   ├── mcp.json
│   └── hooks.json
├── .rules                        # Zed
├── .clinerules/security.md, code-review.md
├── .roorules
├── .roo/
│   ├── rules/security.md
│   ├── rules-code/code-review.md
│   └── mcp.json
├── opencode.json
├── .opencode/
│   ├── skill/greeting/SKILL.md
│   ├── agents/code-reviewer.md
│   └── commands/summarize.md
└── .kiro/
    ├── steering/security.md, greeting.md
    ├── agents/code-reviewer.json
    └── settings/mcp.json
```

## Key Decisions

| Decision | Choice | Reasoning |
|----------|--------|-----------|
| Separate repo | Yes | Tests the binary as a real user would. Cleaner separation from unit tests. |
| All providers in one repo | Yes | Tests cross-provider discovery conflicts and coexistence |
| Shell test harness | Bash with lib.sh helpers | No toolchain required, portable, easy to read and debug |
| All content types | Rules, skills, agents, commands, hooks, MCP | Maximizes coverage from day one |
| Golden file validation | Normalized diff against provider-spec-sourced files | Unambiguous pass/fail. No substring heuristics. |
| Golden files from official sources | Research agents per provider | Prevents tautological testing (testing syllago against syllago's own output) |
| Seed mode | --seed flag pre-populates library | Fast iteration for install/convert tests; default runs full lifecycle |
| Three canonical items | security rule, code-reviewer agent, greeting skill | Covers glob scoping, frontmatter variants, directory-based items |
| Plus commands, hooks, MCP | summarize command, hooks.json, MCP configs | Covers JSON merge codepath and all remaining content types |
| Shared AGENTS.md | Codex + OpenCode | Exercises shared discovery path |

## Content Strategy

Three canonical items appear across providers in native format, plus commands, hooks, and MCP configs.

### The Security Rule

| Provider | Format | Edge Case |
|----------|--------|-----------|
| Claude Code | `.claude/rules/security.md` (YAML frontmatter, `globs:`) | Standard markdown rule |
| Cursor | `.cursor/rules/security.mdc` | `.mdc` frontmatter with `globs:` |
| Windsurf | `.windsurfrules` (concatenated) | `trigger:` field |
| Cline | `.clinerules/security.md` | `paths:` field -> globs |
| Roo Code | `.roo/rules/security.md` + `.roorules` | Mode-specific dirs |
| Zed | `.rules` (concatenated) | Flat file, no frontmatter |
| Copilot | `.github/copilot-instructions.md` | Single flat file |

### The Code-Reviewer Agent

| Provider | Format | Edge Case |
|----------|--------|-----------|
| Claude Code | `.claude/agents/code-reviewer.md` | Markdown + YAML frontmatter |
| Gemini | `.gemini/agents/code-reviewer.md` | Same format as Claude |
| Codex | `.codex/agents/code-reviewer.toml` | TOML structured agent |
| Copilot | `.copilot/agents/code-reviewer.md` | Markdown agent |
| Kiro | `.kiro/agents/code-reviewer.json` | JSON (carries hooks) |
| Roo Code | `.roo/rules-code/code-review.md` | Mode-specific rule |
| OpenCode | `.opencode/agents/code-reviewer.md` | Markdown agent |

### The Greeting Skill

| Provider | Format | Edge Case |
|----------|--------|-----------|
| Claude Code | `.claude/skills/greeting/SKILL.md` | Canonical skill format |
| Gemini | `.gemini/skills/greeting/SKILL.md` | Same structure |
| Kiro | `.kiro/steering/greeting.md` | Steering = rules AND skills |
| OpenCode | `.opencode/skill/greeting/SKILL.md` | `skill/` not `skills/` |

### The Summarize Command

| Provider | Format |
|----------|--------|
| Claude Code | `.claude/commands/summarize.md` |
| Gemini | `.gemini/commands/summarize.md` |
| Copilot | `.copilot/commands/summarize.md` |
| OpenCode | `.opencode/commands/summarize.md` |

### MCP Configs

| Provider | File | Edge Case |
|----------|------|-----------|
| Claude Code | `.claude/settings.json` | Standard JSON |
| Gemini | `.gemini/settings.json` | Same structure |
| Copilot | `.copilot/mcp.json` | Separate MCP file |
| Roo Code | `.roo/mcp.json` | Separate MCP file |
| Kiro | `.kiro/settings/mcp.json` | Nested path |
| OpenCode | `opencode.json` | JSONC, `mcp` key at root |

### Hooks

| Provider | File | Edge Case |
|----------|------|-----------|
| Claude Code | `.claude/settings.json` (hooks key) | Merged into settings |
| Copilot | `.copilot/hooks.json` | Flat hooks format |
| Kiro | `.kiro/agents/code-reviewer.json` | Hooks embedded in agent JSON |

## Test Harness

### lib.sh -- Shared Infrastructure

**Isolation:** Creates temp dir, sets `HOME` to it, copies fixtures into `$HOME/project/`. Cleanup on exit (trap).

**Seed mode (`--seed`):** Runs `syllago add --from claude-code --all --force` to pre-populate library. Install/export/convert tests skip the add step.

**Assertion helpers:**
- `assert_exit_zero "desc" command args...`
- `assert_exit_nonzero "desc" command args...`
- `assert_output_contains "desc" "expected" command args...`
- `assert_file_exists "desc" path`
- `assert_file_contains "desc" path "expected"`
- `assert_json_key "desc" file key expected_value`
- `assert_golden "desc" actual_file golden_file` (normalized diff)
- `pass "desc"` / `fail "desc"`
- `print_summary` (pass/fail counts, exit code)

**Normalized diff:** Strip trailing whitespace, normalize line endings, ignore trailing blank lines.

### run.sh -- Entry Point

```bash
./tests/run.sh                        # Full lifecycle, all suites
./tests/run.sh --seed                  # Pre-seed library, all suites
./tests/run.sh --suite discovery       # Just discovery tests
./tests/run.sh --seed --suite install  # Pre-seeded, just install
```

### Test Suites

| Suite | Tests | Validation |
|-------|-------|-----------|
| `test_discovery.sh` | `syllago add --from <provider>` shows expected items | Output contains expected item names |
| `test_add.sh` | `syllago add <items> --from <provider>` writes to library | Files exist at expected library paths |
| `test_install.sh` | `syllago install <items> --to <provider>` writes to target | Golden file diff against provider-spec reference |
| `test_convert.sh` | `syllago convert <item> --to <provider>` produces output | Golden file diff against provider-spec reference |

Each suite includes negative tests (unknown provider, missing content type).

## Provider Spec Research Pipeline

### Research Phase (Pre-Implementation)

Before writing any golden file, research each provider's format spec from official sources. 11 parallel research agents, one per provider.

**Each agent produces:** `docs/provider-specs/<provider>.md` containing:
- Official doc URLs (cited)
- File locations and directory structure
- Format specification per content type (exact fields, types, required/optional)
- Example files from official docs (verbatim where possible)
- Known quirks or undocumented behavior

### Golden File Construction

After spec docs are reviewed:
1. Hand-write each golden file from the spec using canonical content
2. Add source header comment (where format supports comments)
3. For formats without comments (JSON, TOML), add companion `.source` file

### Maintenance Cycle

```
Provider updates their spec
  -> Update docs/provider-specs/<provider>.md
  -> Update tests/golden/<provider>/ files
  -> Run tests -> syllago converter may need updating
  -> Fix converter -> tests pass
```

## Error Handling

- Test harness uses `set -euo pipefail` for strict error handling
- Individual test failures are captured (don't abort the suite)
- `print_summary` shows all failures with diff output
- Exit code 0 only if all tests pass

## Success Criteria

1. `./tests/run.sh` exits 0 against current syllago binary
2. Breaking a provider's converter causes specific test failures
3. Golden files are traceable to official provider documentation
4. New contributor can clone and run in under 60 seconds
5. Provider spec docs serve as living reference for format specifications

## Open Questions

None -- all questions resolved during brainstorm.

---

## Next Steps

Ready for implementation planning with `Plan` skill.
