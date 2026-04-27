# Kiro Provider Audit

> Audit date: 2026-03-21
> Research files: `docs/providers/kiro/{tools,content-types,hooks,skills-agents}.md`
> Codebase checked: `converter/toolmap.go`, `converter/agents.go`, `provider/kiro.go`, `installer/mcp.go`

## Inaccuracies

### 1. Tool mappings: Glob and Grep map to `read` instead of their own tools

**File:** `cli/internal/converter/toolmap.go:48-49,58-59`

Syllago maps both `Glob` and `Grep` canonical tools to Kiro's `read` tool. Research confirms Kiro has separate `glob` and `grep` built-in tools (documented at kiro.dev/docs/cli/reference/built-in-tools/).

```go
"Glob": { "kiro": "read" },   // Should be "glob"
"Grep": { "kiro": "read" },   // Should be "grep"
```

**Impact:** Agents and hooks exported to Kiro with Glob/Grep tool references will grant `read` permission instead of the correct granular tools. This is overly broad (read includes file reading) and may not actually enable glob/grep functionality if Kiro enforces per-tool permissions.

**Fix:** Change both mappings to their correct Kiro tool names.

### 2. Agent file format: Kiro now uses markdown with YAML frontmatter, not JSON

**File:** `cli/internal/converter/agents.go:386-393` (kiroAgentConfig struct), lines 449-492 (renderKiroAgent), lines 395-437 (canonicalizeKiroAgent)

Research shows Kiro agents are markdown files with YAML frontmatter (same pattern as Claude Code and Copilot CLI). The `kiroAgentConfig` struct uses JSON tags and `renderKiroAgent` outputs `.json` files. The research docs explicitly show:

```yaml
---
name: code-reviewer
description: Expert code review assistant
tools: ["read", "glob", "grep", "@context7"]
model: claude-sonnet-4
allowedTools: ["read", "glob"]
includeMcpJson: true
---
System prompt content here.
```

Syllago outputs:
```json
{ "name": "...", "description": "...", "prompt": "...", "tools": [...] }
```

**Impact:** Exported agent files will be in the wrong format. Kiro may still support the JSON format for CLI agent configs (the CLI docs show JSON examples for hook-carrying agents), but the standard format for custom agents is markdown frontmatter. This also means syllago's `kiroAgentConfig` struct is missing several fields that Kiro supports: `allowedTools`, `toolAliases`, `toolsSettings`, `mcpServers`, `resources`, `includeMcpJson`, `includePowers`, `keyboardShortcut`, `welcomeMessage`, `hooks`.

**Nuance:** The CLI agent config reference does show JSON format for agents that carry hooks. It's possible Kiro supports both formats. The research notes this ambiguity. Worth verifying whether `.kiro/agents/` accepts both `.md` and `.json` files.

### 3. FileFormat returns JSON for agents, should likely be markdown

**File:** `cli/internal/provider/kiro.go:52-59`

```go
FileFormat: func(ct catalog.ContentType) Format {
    switch ct {
    case catalog.Agents, catalog.MCP, catalog.Hooks:
        return FormatJSON
```

If agents are markdown frontmatter (per finding #2), this should return `FormatMarkdown` for agents. Hooks embedded in agent JSON files are still JSON, and MCP is JSON -- those are correct.

## Gaps

### 4. Missing Kiro tool mappings (WebSearch, Task)

**File:** `cli/internal/converter/toolmap.go:63-73`

Kiro has `web_search`, `web_fetch`, `delegate`, and `use_subagent` tools, but none are mapped:

| Canonical | Kiro Tool | Currently Mapped |
|-----------|-----------|-----------------|
| `WebSearch` | `web_search` | No kiro entry |
| `WebFetch` | `web_fetch` | No canonical exists |
| `Task` | `use_subagent` or `delegate` | No kiro entry |

**Impact:** Tool references in these categories pass through untranslated. An agent with `WebSearch` exported to Kiro keeps the literal string "WebSearch" which Kiro won't recognize.

### 5. Missing Kiro-specific agent config fields

**File:** `cli/internal/converter/agents.go:386-393`

The `kiroAgentConfig` struct only has 6 fields. Kiro agents support at least 14 config fields per research:

Missing fields with no canonical equivalent:
- `allowedTools` (auto-approved tools, distinct from `tools`)
- `toolAliases` (remap tool names)
- `toolsSettings` (per-tool path/command restrictions)
- `mcpServers` (inline MCP server definitions)
- `resources` (file://, skill://, knowledge base)
- `includeMcpJson` / `includePowers` (boolean toggles)
- `keyboardShortcut`, `welcomeMessage`
- `hooks` (inline hook definitions)

**Impact:** Round-tripping through syllago loses these Kiro-specific fields. Most critically, `allowedTools` (auto-approve) is a security-relevant field that gets silently dropped.

### 6. No Specs or Powers content types

Research identifies two Kiro-exclusive content types:
- **Specs** (`.kiro/specs/`) -- structured requirements/design/task files for spec-driven development
- **Powers** -- composite bundles of MCP + steering + hooks from partners

These have no syllago content type mapping. The research docs correctly note this. Not necessarily a gap to fix (they may be too Kiro-specific for cross-provider portability), but worth documenting as known unsupported types.

### 7. IDE-only hook events not mapped

Research documents 10 hook events total. Syllago maps 5 CLI events. The 5 IDE-only events have no mapping:
- `FileCreate`, `FileSave`, `FileDelete`
- `PreTaskExecution`, `PostTaskExecution`
- `ManualTrigger`

File events could potentially map to Claude Code's filesystem watching patterns. Low priority since these are GUI-only.

## Opportunities

### 8. Kiro's tool reference syntax is richer than syllago models

Kiro supports wildcard patterns in tool references (`@server/read_*`, `@*-mcp/status`, `?ead`), the `@builtin` shorthand, and `@powers` group. Syllago's tool translation is 1:1 string mapping. If an agent uses `@builtin` in its tools list, syllago would try to translate it as a single tool name and pass it through unchanged.

This isn't a bug today but becomes one if syllago starts ingesting Kiro-native agents that use these patterns.

### 9. Hook rendering could leverage Kiro's markdown agent format

Currently syllago renders hooks for Kiro as a `syllago-hooks.json` agent file. If Kiro supports markdown agents with inline hooks (research shows hooks as a frontmatter field), the hooks could be rendered in the markdown format instead, which is more consistent with how Kiro users expect to see agent files.

### 10. MCP path is correct

Confirmed: `cli/internal/installer/mcp.go:67` uses `.kiro/settings/mcp.json` and `cli/internal/provider/kiro.go:45` uses the same path for discovery. This matches the research finding that MCP lives at `.kiro/settings/mcp.json` (not `.kiro/mcp.json`). No issue here.

## Summary

| # | Type | Severity | Item |
|---|------|----------|------|
| 1 | Inaccuracy | **High** | Glob/Grep mapped to `read` instead of `glob`/`grep` |
| 2 | Inaccuracy | **High** | Agent format is JSON, should be markdown frontmatter |
| 3 | Inaccuracy | **Medium** | FileFormat returns JSON for agents |
| 4 | Gap | **Medium** | Missing WebSearch/Task tool mappings |
| 5 | Gap | **Low** | Missing Kiro-specific agent config fields |
| 6 | Gap | **Info** | No Specs/Powers content types (Kiro-exclusive) |
| 7 | Gap | **Info** | IDE-only hook events unmapped |
| 8 | Opportunity | **Low** | Wildcard tool reference patterns unsupported |
| 9 | Opportunity | **Low** | Hook rendering could use markdown format |
| 10 | Verified | -- | MCP path `.kiro/settings/mcp.json` is correct |
