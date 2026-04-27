# Kitchen Sink Repo Audit

**Date:** 2026-03-21
**Auditor:** Maive (automated audit)
**Repos compared:**
- Kitchen sink: `/home/hhewett/.local/src/syllago-kitchen-sink/`
- Provider research: `/home/hhewett/.local/src/syllago/docs/providers/`
- Provider code: `/home/hhewett/.local/src/syllago/cli/internal/provider/`
- Converter code: `/home/hhewett/.local/src/syllago/cli/internal/converter/`

---

## Executive Summary

The kitchen sink repo has **several stale fixtures and golden files** that no longer match the current provider definitions and converter output. The most critical issues are:

1. **Kiro agents**: Kitchen sink has JSON format; code now renders markdown with YAML frontmatter
2. **Copilot MCP path**: Kitchen sink uses `.copilot/mcp.json`; provider code expects `.copilot/mcp-config.json`
3. **OpenCode skills path**: Kitchen sink uses `.opencode/skill/` (singular); provider code discovers from `.opencode/skills/` (plural)
4. **Copilot hooks path**: Kitchen sink uses `.copilot/hooks.json`; provider code discovers from `.github/hooks/`
5. **Cursor rules frontmatter**: Kitchen sink uses `alwaysApply: false`; Cursor research confirms `alwaysApply` defaults to false, so this is technically correct but inconsistent with the `alwaysApply: true` on the code-review rule
6. **Missing content types**: Several providers have supported content types with no kitchen sink fixtures

---

## Per-Provider Audit

### Claude Code

| Content Type | File in Kitchen Sink | Status | Notes |
|-------------|---------------------|--------|-------|
| Rules | `.claude/rules/security.md` | CORRECT | Frontmatter with `description`, `alwaysApply`, `globs` matches research |
| Skills | `.claude/skills/greeting/SKILL.md` | CORRECT | Frontmatter with `name`, `description` matches research |
| Agents | `.claude/agents/code-reviewer.md` | CORRECT | Frontmatter with `name`, `description`, `tools`, `model` matches research |
| Commands | `.claude/commands/summarize.md` | CORRECT | Frontmatter with `description` matches research |
| Hooks | `.claude/settings.json` (hooks key) | CORRECT | `PreToolUse` event with `matcher`, `hooks` array, `type: command`, `timeout` all match research schema |
| MCP | `.claude/settings.json` (mcpServers key) | CORRECT | `mcpServers` with `type`, `command`, `args`, `env` matches research. Note: research says MCP can also be in `.mcp.json` or `~/.claude.json`; fixture uses `settings.json` which is valid |
| Root rule (CLAUDE.md) | `CLAUDE.md` | CORRECT | Plain markdown, no frontmatter, matches research |

**Missing:** No `.mcp.json` fixture (separate from settings.json). Not critical since settings.json covers the MCP format.

**Golden files:** All correct format-wise. The `security.md` golden has `---\n{}\n---` (empty frontmatter) which represents the canonical-to-provider conversion output.

---

### Gemini CLI

| Content Type | File in Kitchen Sink | Status | Notes |
|-------------|---------------------|--------|-------|
| Rules (GEMINI.md) | `GEMINI.md` | CORRECT | Plain markdown, no frontmatter, matches research |
| Skills | `.gemini/skills/greeting/SKILL.md` | CORRECT | Same SKILL.md format, frontmatter with `name`, `description` |
| Agents | `.gemini/agents/code-reviewer.md` | CORRECT | Frontmatter with `name`, `description`, `tools` using Gemini tool names (`read_file`, `grep_search`, `list_directory`, `run_shell_command`) matches research and toolmap.go |
| Commands | `.gemini/commands/summarize.toml` | CORRECT | TOML with `description` and `prompt` fields matches research |
| MCP | `.gemini/settings.json` | CORRECT | `mcpServers` key in settings.json matches research |
| Hooks | (none) | MISSING | Gemini CLI supports hooks in settings.json under the `hooks` key. No fixture exists. Provider code says hooks discovery from `.gemini/settings.json`. |

**Golden files:** All correct. Agent golden has Gemini-specific tool names. Command golden is TOML format.

---

### Cursor

| Content Type | File in Kitchen Sink | Status | Notes |
|-------------|---------------------|--------|-------|
| Rules (security.mdc) | `.cursor/rules/security.mdc` | CORRECT | Frontmatter with `description`, `alwaysApply: false`, `globs` matches research. Note: Cursor uses three fields only: `description`, `globs`, `alwaysApply`. The kitchen sink file does NOT have `globs` which is fine since it's optional. |
| Rules (code-review.mdc) | `.cursor/rules/code-review.mdc` | CORRECT | Frontmatter with `description`, `alwaysApply: true` matches research |
| Skills | (none) | MISSING | Provider code supports skills at `.cursor/skills/`. Research confirms Cursor supports skills. No fixture. |
| Commands | (none) | MISSING | Provider code supports commands at `.cursor/commands/`. Research confirms Cursor supports commands. No fixture. |
| Agents | (none) | MISSING | Provider code supports agents at `.cursor/agents/`. Research says Cursor does NOT support user-defined agent files -- **provider code disagrees with research**. The converter does have `renderCursorAgent()`. |
| Hooks | (none) | MISSING | Provider code supports hooks via `.cursor/settings.json`. Research confirms Cursor supports hooks in `.cursor/hooks.json`. Path mismatch: provider code says `settings.json`, research says `hooks.json`. |
| MCP | (none) | MISSING | Provider code supports MCP at `.cursor/mcp.json`. Research confirms this path. No fixture. |
| Legacy (.cursorrules) | (none) | MISSING | Research confirms `.cursorrules` as legacy format. Not critical to have in fixtures. |

**Golden files:** Cursor rule golden has `---\n{}\n---` (empty frontmatter) which is `.mdc` format. Correct.

---

### Windsurf

| Content Type | File in Kitchen Sink | Status | Notes |
|-------------|---------------------|--------|-------|
| Rules (.windsurfrules) | `.windsurfrules` | CORRECT | Frontmatter with `trigger: always_on`, `description` matches research. Research says frontmatter fields are `trigger` (required) and `globs` (for glob trigger). |
| Skills | (none) | MISSING | Provider code supports skills. Research confirms skills at `.windsurf/skills/`. |
| Rules (.windsurf/rules/) | (none) | MISSING | Research says modern rules go in `.windsurf/rules/*.md`. Provider code discovers from both `.windsurfrules` and `.windsurf/rules/`. |
| Hooks | (none) | MISSING | Provider code supports hooks. Research confirms hooks at `.windsurf/hooks.json`. |

**Golden files:** Rule golden has `---\n{}\n---` (empty frontmatter). This is the canonical conversion output. Correct.

**Content issue:** The `.windsurfrules` file has a `description` field in frontmatter. Research says Windsurf rules frontmatter only has `trigger` and `globs` fields -- `description` is NOT a documented field. This may be harmless (ignored by Windsurf) but is technically wrong per spec.

---

### Codex

| Content Type | File in Kitchen Sink | Status | Notes |
|-------------|---------------------|--------|-------|
| Agents | `.codex/agents/code-reviewer.toml` | CORRECT | TOML format with `[agent]` table containing `name`, `description`, `model`, `[agent.instructions]` with `content`. Research confirms standalone TOML files. However, research shows fields as flat top-level (`name`, `description`, `developer_instructions`), not nested under `[agent]`. The kitchen sink format uses `[agent]` and `[agent.instructions].content` which may be a syllago convention rather than native Codex format. |
| Rules (AGENTS.md) | `AGENTS.md` | CORRECT | Plain markdown, shared with other providers. Codex reads `AGENTS.md` natively. |
| Skills | (none) | MISSING | Research confirms skills at `.agents/skills/`. Provider code supports skills. |
| MCP | (none) | MISSING | Codex MCP is in `config.toml` (TOML format), not JSON. Provider code says JSON merge. This is a known gap -- Codex MCP is TOML-based which differs from all other providers. |
| Hooks | (none) | MISSING | Research mentions hooks in `config.toml`. Provider code lists hooks discovery at `.codex/hooks.json`. |

**Golden files:** Agent golden uses `[agent]` nesting. See note above about potential format mismatch with native Codex.

**Content issue:** The golden file has `tools = ['view', 'rg', 'glob', 'shell']`. The toolmap.go does NOT have Codex-specific Read/Grep/Glob mappings (they fall through to canonical names). Looking at the toolmap: Codex has `read_file` for Read (line 19), `grep_files` for Grep (line 77), `list_dir` for Glob (line 65), `shell` for Bash (line 53). The golden file uses `view`, `rg`, `glob` which are Copilot tool names, not Codex names. **This is wrong.** The correct Codex tools should be `read_file`, `grep_files`, `list_dir`, `shell`.

Wait -- re-reading the golden: the golden is for `codex/agents/code-reviewer.toml` and contains `tools = ['view', 'rg', 'glob', 'shell']`. But looking at toolmap.go, Codex doesn't have a mapping for `Read` (no `codex` key in the `Read` map -- actually wait, line 19 has `"codex": "read_file"`). Let me recheck... The toolmap shows: Read->codex = "read_file", Bash->codex = "shell", Glob->codex = "list_dir", Grep->codex = "grep_files". So the golden file tools `['view', 'rg', 'glob', 'shell']` are wrong -- `view` is copilot-cli's Read name, `rg` is copilot-cli's Grep name, `glob` is copilot-cli's Glob name. Only `shell` is correct for Codex.

---

### Copilot CLI

| Content Type | File in Kitchen Sink | Status | Notes |
|-------------|---------------------|--------|-------|
| Rules | `.github/copilot-instructions.md` | CORRECT | Plain markdown, no frontmatter, matches research |
| Agents | `.copilot/agents/code-reviewer.md` | WRONG | File is at `.copilot/agents/code-reviewer.md`. Research says agents go at `.github/agents/<name>.agent.md`. Provider code discovers from `.copilot/agents`, `.github/agents`, and `.claude/agents`. The file exists at a valid discovery path BUT has wrong filename (should be `code-reviewer.agent.md` per Copilot convention). Also, the frontmatter format matches Claude Code's format, not Copilot's (which uses `tools` as an array of strings like `["read", "search"]`, not YAML list). |
| Commands | `.copilot/commands/summarize.md` | CORRECT | Markdown with frontmatter, matches format. Research doesn't mention `.copilot/commands/` but provider code discovers from there. |
| Hooks | `.copilot/hooks.json` | WRONG PATH | Research says hooks go at `.github/hooks/<name>.json`. Provider code discovers from `.github/hooks`. The kitchen sink has `.copilot/hooks.json` which is NOT a valid discovery path. |
| MCP | `.copilot/mcp.json` | WRONG PATH | Research says MCP config is at `~/.copilot/mcp-config.json` or `.copilot/mcp-config.json`. Provider code discovers from `.copilot/mcp-config.json`. The kitchen sink uses `.copilot/mcp.json` (wrong filename). |
| Skills | (none) | MISSING | Research confirms skills at `.github/skills/`. Provider code discovers from `.github/skills`. |

**Hooks format issue:** The kitchen sink hooks.json uses `preToolUse` (camelCase) which matches the Copilot research schema. However, the structure uses `bash` and `timeoutSec` fields which also match research. But the file is at the wrong path.

**Golden files:** Agent golden has Claude Code tool names in the tools array (`view`, `rg`, `glob`, `shell`) which are Copilot-correct per toolmap.go. Command golden matches. Rule golden has empty frontmatter, correct.

---

### Zed

| Content Type | File in Kitchen Sink | Status | Notes |
|-------------|---------------------|--------|-------|
| Rules (.rules) | `.rules` | CORRECT | Plain text with markdown header, matches research. Zed checks for `.rules` first. |
| MCP | (none) | MISSING | Research says MCP is in `settings.json` under `context_servers` (NOT `mcpServers`). Provider code supports MCP. No fixture. |

**Golden files:** Rule golden has `---\n{}\n---` (empty frontmatter). Correct for canonical conversion output.

---

### Cline

| Content Type | File in Kitchen Sink | Status | Notes |
|-------------|---------------------|--------|-------|
| Rules (directory) | `.clinerules/code-review.md`, `.clinerules/security.md` | CORRECT | Plain markdown files in `.clinerules/` directory, matches research |
| MCP | (none) | MISSING | Research says MCP is in `cline_mcp_settings.json` (VS Code extension storage, not project). Provider code lists MCP as supported. No fixture -- but MCP is not project-scoped for Cline, so this may be intentional. |
| Hooks | (none) | MISSING | Provider code says hooks discovery from `.clinerules/hooks/`. Research does not mention hooks as a Cline feature. Provider code may be ahead of research or incorrect. |

**Golden files:** Rule golden has empty frontmatter, correct.

---

### Roo Code

| Content Type | File in Kitchen Sink | Status | Notes |
|-------------|---------------------|--------|-------|
| Rules (general) | `.roo/rules/security.md` | CORRECT | Plain markdown, matches research |
| Rules (mode-specific) | `.roo/rules-code/code-review.md` | CORRECT | Mode-specific rules at `.roo/rules-{mode-slug}/`, matches research |
| Rules (legacy) | `.roorules` | CORRECT | Single-file fallback, matches research |
| MCP | `.roo/mcp.json` | CORRECT | `mcpServers` key in JSON, matches research |
| Skills | (none) | MISSING | Research confirms skills at `.roo/skills/`. Provider code supports skills. |
| Agents (modes) | (none) | MISSING | Research says custom modes at `.roomodes` (YAML/JSON). Provider code supports agents. |

**Golden files:** Rule golden has empty frontmatter, correct.

---

### OpenCode

| Content Type | File in Kitchen Sink | Status | Notes |
|-------------|---------------------|--------|-------|
| Rules (AGENTS.md) | `AGENTS.md` | CORRECT | Shared with Codex/Copilot. OpenCode reads AGENTS.md natively. |
| Skills | `.opencode/skill/greeting/SKILL.md` | WRONG PATH | Provider code discovers from `.opencode/skills/` (plural). Kitchen sink uses `.opencode/skill/` (singular). Research says both forms may be supported for backwards compat ("Singular directory names (e.g., `agent/`) are supported for backwards compatibility"), but the provider code only checks plural. |
| Agents | `.opencode/agents/code-reviewer.md` | CORRECT | Markdown with YAML frontmatter, matches research. Frontmatter has `name`, `description` -- correct for OpenCode. |
| Commands | `.opencode/commands/summarize.md` | CORRECT | Markdown with frontmatter, matches research |
| MCP | `opencode.json` (root) | CORRECT | JSONC format with `$schema`, `mcp` key containing server definitions with `type: "local"`, `command` as array, `environment`, `enabled`. Matches research. |

**Golden files:**
- Skills golden at `opencode/skill/greeting/SKILL.md` (singular) -- matches kitchen sink but potentially wrong path
- Agent golden has canonical (Claude Code) tool names (`Read`, `Grep`, `Glob`, `Bash`). Research says OpenCode agents support markdown format. However, the toolmap.go has OpenCode mappings: Read->"read", Grep->"grep", Glob->"glob", Bash->"bash" (all lowercase). **The golden should use lowercase OpenCode tool names, not canonical names.** Wait -- looking at the converter `renderOpenCodeAgent()`, it calls `buildAgentCanonical()` which preserves canonical tool names. So OpenCode agents keep canonical names? Let me check... yes, `renderOpenCodeAgent` does NOT call `TranslateTools`. This seems intentional -- OpenCode's format is close enough to canonical that tools pass through. But this means the golden is correct for what the converter outputs, even if OpenCode natively uses lowercase names.

---

### Kiro

| Content Type | File in Kitchen Sink | Status | Notes |
|-------------|---------------------|--------|-------|
| Steering (rules) | `.kiro/steering/security.md` | CORRECT | Plain markdown with heading, matches research. No frontmatter = always inclusion mode. |
| Steering (skills) | `.kiro/steering/greeting.md` | CORRECT | Plain markdown, matches research |
| Agents | `.kiro/agents/code-reviewer.json` | WRONG FORMAT | Kitchen sink has a JSON file. The current converter code (`renderKiroAgent()`) now renders **markdown with YAML frontmatter** (`.md` files). The provider code also declares `FileFormat` for agents as `FormatMarkdown`. Research confirms Kiro agents are "Markdown with YAML frontmatter". The JSON file is stale. |
| MCP | `.kiro/settings/mcp.json` | CORRECT | `mcpServers` key in JSON, matches research and provider code |
| Hooks | (none) | MISSING | Research says hooks are in agent JSON files or `.kiro/hooks/`. Provider code discovers from `.kiro/agents`. |

**Golden files:**
- Agent golden at `kiro/agents/code-reviewer.json` is JSON format -- **STALE**, should be markdown with YAML frontmatter
- The JSON golden has `"tools": ["read", "read", "read", "shell"]` -- the triple "read" is clearly a bug (should be different tools mapped through the Kiro toolmap). Looking at toolmap.go: Read->"read", Grep->"grep", Glob->"glob", Bash->"shell". So correct Kiro tools should be `["read", "grep", "glob", "shell"]`.
- Steering goldens are correct (plain markdown)

---

## Cross-Provider Files

| File | Status | Notes |
|------|--------|-------|
| `AGENTS.md` | CORRECT | Plain markdown, read by Codex, Copilot, OpenCode, Cursor, Roo Code, Windsurf, Zed |
| `CLAUDE.md` | CORRECT | Plain markdown, read by Claude Code and OpenCode (as fallback) |
| `GEMINI.md` | CORRECT | Plain markdown, read by Gemini CLI. Content matches CLAUDE.md which is fine for test fixtures. |

---

## Test Scripts Audit

### tests/test_discovery.sh

| Test | Status | Notes |
|------|--------|-------|
| claude-code discovers | OK | Tests security, greeting, code-reviewer, summarize |
| gemini-cli discovers | OK | Tests GEMINI, greeting, code-reviewer, summarize |
| cursor discovers | OK | Tests security, code-review rules |
| windsurf discovers | OK | Tests "Rules" |
| codex discovers | OK | Tests code-reviewer agent |
| copilot-cli discovers | OK | Tests code-reviewer, summarize |
| zed discovers | OK | Tests "Rules" |
| cline discovers | OK | Tests security rule |
| roo-code discovers | OK | Tests security rule |
| opencode discovers | POTENTIALLY BROKEN | Tests greeting skill -- but skill is at `.opencode/skill/` (singular) while provider discovers from `.opencode/skills/` (plural). This test may fail. |
| kiro discovers | POTENTIALLY BROKEN | Tests code-reviewer agent -- but agent is JSON format while code now expects markdown. Discovery may still work if it just scans the directory. |

### tests/test_add.sh

| Test | Status | Notes |
|------|--------|-------|
| claude-code add | OK | Tests all content types |
| cursor add | OK | Tests .mdc conversion |
| windsurf add | OK | Tests flat rule |
| codex add | OK | Tests agent and AGENTS rule |
| copilot-cli add | OK | Tests agent, command, rule |
| cline add | OK | Tests security rule |
| roo-code add | OK | Tests security rule |
| opencode add | POTENTIALLY BROKEN | Tests skill at path that may not be discovered |
| kiro add | POTENTIALLY BROKEN | Tests agent that is now wrong format |
| gemini-cli add | OK | Tests GEMINI rule, skill |
| zed add | OK | Tests flat rule |

### tests/test_convert.sh

| Test | Status | Notes |
|------|--------|-------|
| Rule conversions | OK | Tests all 11 providers |
| Agent to codex | POTENTIALLY WRONG | Golden file has wrong tool names (`view`, `rg`, `glob` instead of `read_file`, `grep_files`, `list_dir`) |
| Agent to kiro | BROKEN | Golden is JSON but converter now outputs markdown |
| Agent to copilot-cli | OK | Golden matches converter output |
| Agent to opencode | OK | Golden keeps canonical tool names (converter behavior) |
| Agent to claude-code | OK | Golden matches |
| Agent to gemini-cli | OK | Golden has correct Gemini tool names |
| Skills conversions | OK | All tested providers |
| Command conversions | OK | All tested providers |

### tests/test_roundtrip.sh

| Test | Status | Notes |
|------|--------|-------|
| Rule roundtrips | OK | Tests 8 providers, correct fixture paths |
| Command roundtrips | OK | Tests copilot-cli, gemini-cli, opencode |
| Skill roundtrip through kiro | POTENTIALLY BROKEN | Places at `.kiro/steering/greeting.md` and re-adds as `rules` type -- may work since steering is just markdown |
| Agent roundtrips | NOTES SAY SKIPPED | Codex and Kiro agent roundtrips are explicitly skipped with "known bug" comments. These should be revisited. |

### tests/test_install.sh

| Test | Status | Notes |
|------|--------|-------|
| Install to claude-code | OK | Tests rules, skills, commands at correct `$HOME/.claude/` paths |
| Install to cursor | OK | Tests rule at `$HOME/.cursor/rule.mdc` |
| Install to windsurf | OK | Tests rule at `$HOME/.codeium/windsurf/rule.md` |
| Install to gemini-cli | OK | Tests rule and skill |
| Install to codex | OK | Tests rule |
| Install to copilot-cli | OK | Tests rule and command |
| Install to opencode | POTENTIALLY WRONG | Tests skill path `$HOME/.config/opencode/skill/greeting/SKILL.md` -- singular `skill` may be wrong; provider code says `skills` (plural) for InstallDir |

---

## Provider Specs (docs/provider-specs/) Audit

The kitchen sink repo has its own provider spec files at `docs/provider-specs/`. These are **separate from** the detailed research in the syllago repo. A quick comparison:

| Provider | Kitchen Sink Spec | Current? | Key Issues |
|----------|------------------|----------|------------|
| claude-code | Comprehensive | MOSTLY CURRENT | Mentions `alwaysApply` and `globs` for rules, correct. Missing newer fields like `paths` (from research, Claude Code rules use `paths` not `globs` -- but wait, the provider research shows Claude Code rules use `paths` for frontmatter, while Cursor uses `alwaysApply`/`globs`). **Kitchen sink spec says Claude Code uses `alwaysApply`/`globs` -- this is WRONG per research. Claude Code rules use `paths` field.** |
| cline | Brief | CURRENT | Correctly documents `.clinerules` directory |
| codex | Brief | STALE | Says agents use `[agent]` TOML nesting; research shows flat top-level fields (`name`, `description`, `developer_instructions`) |
| copilot-cli | Moderate | STALE | MCP path listed as `.copilot/mcp.json` -- should be `.copilot/mcp-config.json`. Agent path as `.copilot/agents/` -- should note `.github/agents/<name>.agent.md` convention. |
| cursor | Moderate | CURRENT | Correctly documents `.mdc` format with three frontmatter fields |
| gemini-cli | Moderate | CURRENT | Correctly documents TOML commands, `.gemini/settings.json` for MCP |
| kiro | Moderate | STALE | Documents agents as JSON format -- should be markdown with YAML frontmatter |
| opencode | Moderate | MOSTLY CURRENT | Skills path uses singular `skill/` -- should be `skills/` per provider code |
| roo-code | Brief | CURRENT | Correctly documents mode-specific rules |
| windsurf | Brief | CURRENT | Correctly documents `.windsurfrules` and `trigger` frontmatter |
| zed | Brief | CURRENT | Correctly documents `.rules` file |

---

## Critical Issues Summary (Action Required)

### P0: Broken by Code Changes

1. **Kiro agent fixture and golden**: `.kiro/agents/code-reviewer.json` must be converted to markdown with YAML frontmatter (`.kiro/agents/code-reviewer.md`). Golden file at `tests/golden/kiro/agents/code-reviewer.json` must also be updated.

2. **Kiro agent golden has triple "read" bug**: `"tools": ["read", "read", "read", "shell"]` should be `["read", "grep", "glob", "shell"]`.

3. **Codex agent golden has wrong tool names**: `tools = ['view', 'rg', 'glob', 'shell']` should be `tools = ['read_file', 'grep_files', 'list_dir', 'shell']`.

### P1: Wrong Paths (Tests Will Fail)

4. **Copilot MCP fixture**: `.copilot/mcp.json` should be `.copilot/mcp-config.json` to match provider code discovery path.

5. **Copilot hooks fixture**: `.copilot/hooks.json` should be at `.github/hooks/lint-check.json` to match provider code discovery path (`.github/hooks/`).

6. **OpenCode skills fixture**: `.opencode/skill/` (singular) should be `.opencode/skills/` (plural) to match provider code discovery path.

### P2: Missing Fixtures (Coverage Gaps)

7. **Cursor**: Missing skills, commands, hooks, MCP, and agents fixtures.
8. **Windsurf**: Missing skills fixtures (`.windsurf/skills/`).
9. **Roo Code**: Missing skills and custom modes (agents) fixtures.
10. **Gemini CLI**: Missing hooks fixture.
11. **Copilot CLI**: Missing skills fixture (`.github/skills/`).
12. **Codex**: Missing skills fixture (`.agents/skills/`).

### P3: Content Accuracy Issues

13. **Claude Code rules frontmatter**: Kitchen sink spec says `alwaysApply`/`globs` but research says Claude Code rules use `paths` field (not `globs`, not `alwaysApply`). The fixture at `.claude/rules/security.md` uses `alwaysApply` and `globs` which are **Cursor fields, not Claude Code fields**. Claude Code rules frontmatter only has `paths`. However -- looking more carefully at the provider code, Claude Code rules files ARE used with this frontmatter in the kitchen sink. This may be syllago's canonical format rather than native Claude Code format.

14. **Windsurf `.windsurfrules` has `description` field**: Research says Windsurf frontmatter only supports `trigger` and `globs`. The `description` field is not documented and may be ignored.

15. **Copilot agent filename convention**: Should be `<name>.agent.md` per research, not just `<name>.md`.

---

## Fixture-to-Provider Correctness Matrix

Key: C = Correct, W = Wrong, M = Missing, N/A = Provider doesn't support

| Provider | Rules | Skills | Agents | Commands | Hooks | MCP |
|----------|-------|--------|--------|----------|-------|-----|
| claude-code | C | C | C | C | C | C |
| gemini-cli | C | C | C | C | M | C |
| cursor | C | M | M | M | M | M |
| windsurf | C | M | N/A | N/A | M | N/A |
| codex | C | M | W (tools) | N/A | M | M |
| copilot-cli | C | M | C | C | W (path) | W (path) |
| zed | C | N/A | N/A | N/A | N/A | M |
| cline | C | N/A | N/A | N/A | M | N/A |
| roo-code | C | M | M | N/A | N/A | C |
| opencode | C | W (path) | C | C | N/A | C |
| kiro | C | C | W (format) | N/A | M | C |
