# Windsurf Provider Audit

Audit date: 2026-03-21
Research files: `docs/providers/windsurf/{content-types,hooks,skills-agents,tools}.md`

---

## Inaccuracies

### 1. Hooks marked as unsupported (WRONG)

**File:** `cli/internal/converter/hooks.go:210`
**Issue:** Windsurf is listed in `hooklessProviders`, meaning hook conversion emits a
"does not support hooks" warning and drops all content. Research confirms Windsurf has
a full 12-event hook system (`hooks.json`) since v1.12.41.

**Impact:** Users cannot convert hooks to/from Windsurf. This is the single biggest
functional gap.

**Fix:** Remove `"windsurf": true` from `hooklessProviders`. Windsurf hooks use a
`{"hooks": {"event_name": [entries]}}` structure that is close to syllago's canonical
nested format but differs in field names (`command`/`show_output`/`working_directory`
vs. `type`/`command`/`timeout`/`statusMessage`). Needs a dedicated canonicalizer and
renderer, plus entries in `HookEvents` map.

### 2. Provider only supports Rules (missing 4+ content types)

**File:** `cli/internal/provider/windsurf.go:39-46`
**Issue:** `SupportsType` returns true only for `catalog.Rules`. Research shows Windsurf
supports:
- **Rules** (.windsurf/rules/*.md) -- partially implemented
- **Skills** (.windsurf/skills/<name>/) -- not implemented
- **Hooks** (.windsurf/hooks.json) -- not implemented
- **MCP** (~/.codeium/windsurf/mcp_config.json) -- not implemented
- **Workflows** (.windsurf/workflows/*.md) -- no syllago content type exists yet

### 3. Discovery only finds legacy `.windsurfrules` (missing modern paths)

**File:** `cli/internal/provider/windsurf.go:26-29` and `cli/internal/catalog/native_scan.go:271`
**Issue:** Both `DiscoveryPaths` and native scan only look at `.windsurfrules` (legacy).
Research shows the modern path is `.windsurf/rules/*.md` (directory of markdown files
with frontmatter). The legacy single-file format is still supported but deprecated.

**Fix:** Add `.windsurf/rules/` to discovery. This is a directory scan, not a single file.

### 4. EmitPath targets legacy format only

**File:** `cli/internal/provider/windsurf.go:36-38`
**Issue:** `EmitPath` returns `.windsurfrules` (legacy single file). Modern Windsurf
rules go in `.windsurf/rules/<name>.md` as individual files with YAML frontmatter
(trigger field). Emitting to the legacy path means installed rules cannot use
`model_decision`, `manual`, or `glob` activation modes.

### 5. InstallDir only handles Rules at global scope

**File:** `cli/internal/provider/windsurf.go:14-19`
**Issue:** `InstallDir` returns a path only for `catalog.Rules` and returns the global
user directory (`~/.codeium/windsurf`). Missing:
- Skills: `~/.codeium/windsurf/skills/<name>/`
- Hooks: `~/.codeium/windsurf/hooks.json` (JSON merge, `__json_merge__`)
- MCP: `~/.codeium/windsurf/mcp_config.json` (JSON merge, `__json_merge__`)

---

## Gaps

### 1. No Windsurf entries in toolmap

**File:** `cli/internal/converter/toolmap.go`
**Issue:** Neither `ToolNames` nor `HookEvents` have any `"windsurf"` entries. This
means:
- Tool name translation in rules (e.g., `Read` -> `view_line_range`) does not work
- Hook event translation (e.g., `PreToolUse` -> `pre_write_code`) cannot work
- MCP tool name format translation is missing from `TranslateMCPToolName`

**Windsurf tool mappings from research:**

| Canonical | Windsurf |
|-----------|----------|
| Read | view_line_range |
| Write | write_to_file |
| Edit | edit_file |
| Bash | run_command |
| Glob | find_by_name |
| Grep | grep_search |
| WebSearch | search_web |
| WebFetch | read_url_content |

**Windsurf hook event mappings from research:**

Windsurf hooks are event-type based, not tool-matcher based like Claude Code. The
mapping is structural, not 1:1:

| Canonical | Windsurf |
|-----------|----------|
| PreToolUse (Read matcher) | pre_read_code |
| PreToolUse (Write/Edit matcher) | pre_write_code |
| PreToolUse (Bash matcher) | pre_run_command |
| PostToolUse (Read matcher) | post_read_code |
| PostToolUse (Write/Edit matcher) | post_write_code |
| PostToolUse (Bash matcher) | post_run_command |
| UserPromptSubmit | pre_user_prompt |
| Stop | post_cascade_response |

This is a **structural mismatch**: syllago's canonical format uses event+matcher
(PreToolUse + "Bash"), while Windsurf uses distinct event names per tool category.
Converting between them requires splitting/merging events, not just name translation.

### 2. No Windsurf installer

**Observation:** No files in `cli/internal/installer/` reference Windsurf. The installer
package has no Windsurf-specific logic for install/uninstall operations.

### 3. Workflows content type does not exist in syllago

**Observation:** Windsurf has a Workflows content type (`.windsurf/workflows/*.md`,
slash-command activated). Syllago has no `catalog.Workflows` content type. This is a
content type that exists in Windsurf but has no analog in most other providers (closest
would be Claude Code's slash commands or custom commands).

### 4. Skills converter has no Windsurf-specific handling

**Observation:** Research shows Windsurf skills use the Agent Skills standard (SKILL.md
with `name`, `description`, `license`, `compatibility`, `metadata`, `allowed-tools`
frontmatter). Syllago likely has a generic skills converter, but no Windsurf-specific
paths in the provider definition.

### 5. MCP config format differences not handled

**Observation:** Windsurf MCP uses `mcp_config.json` with `mcpServers` key, supports
`serverUrl` (HTTP) and `url` (SSE) transports, and `${env:VAR}` interpolation syntax.
These may differ from other providers' MCP formats.

### 6. System-level paths not represented

**Observation:** Research documents system-level (enterprise) paths for rules, skills,
hooks on all three OSes (e.g., `/etc/windsurf/rules/*.md` on Linux). These are not
represented in the provider definition. Lower priority since syllago targets user/
workspace scope.

---

## Opportunities

### 1. Windsurf hooks are achievable with moderate effort

Windsurf's hook format (`hooks.json` with `command`/`show_output`/`working_directory`)
is simpler than Claude Code's. The main challenge is the structural mismatch (Windsurf
uses per-tool-category events vs. syllago's event+matcher pattern). A dedicated
`canonicalizeWindsurfHooks` / `renderWindsurfHooks` pair could handle the split/merge.

### 2. Skills support is nearly free

Windsurf follows the Agent Skills standard (agentskills.io). If syllago's skill handling
already supports SKILL.md + directory structure, adding Windsurf skill paths to the
provider definition may be all that's needed.

### 3. Rule conversion already works well

The `canonicalizeWindsurfRule` and `renderWindsurfRule` functions correctly handle the
`trigger` field and all four activation modes (always_on, manual, model_decision, glob).
The conversion logic is accurate per the research. The gap is only in discovery paths
and install targets, not in format conversion.

### 4. AGENTS.md is cross-provider

Windsurf reads AGENTS.md natively. Any content installed as AGENTS.md for other
providers automatically works in Windsurf without conversion.

### 5. Toolmap entries are low-hanging fruit

Adding Windsurf to `ToolNames` in toolmap.go is straightforward -- the research
documents all 23 tools with clear canonical equivalents.

---

## Priority Summary

| Priority | Item | Effort |
|----------|------|--------|
| P0 | Remove from `hooklessProviders` + add hook converter | Medium |
| P0 | Add `.windsurf/rules/` to discovery paths | Low |
| P1 | Add Windsurf to `ToolNames` in toolmap | Low |
| P1 | Update `SupportsType` for Skills, Hooks, MCP | Low |
| P1 | Update `InstallDir` / `EmitPath` for modern paths | Low |
| P2 | Add Windsurf hook event mappings (structural) | Medium |
| P2 | Add Skills paths to provider definition | Low |
| P3 | Add MCP config support | Medium |
| P3 | Consider Workflows content type | High (new type) |
