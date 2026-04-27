# Copilot CLI Provider Audit

Audit of syllago codebase accuracy against Copilot CLI provider research.
Date: 2026-03-21

## Inaccuracies

### 1. Tool names: `apply_patch` should be `edit` + `create` (MEDIUM)

**File:** `cli/internal/converter/toolmap.go:20,29`

Research shows Copilot CLI uses `edit` (string replacement in existing files) and `create` (new file creation) as separate tools. Syllago maps both `Write` and `Edit` to `apply_patch`. The research notes `apply_patch` may be an internal/wire name, but the user-facing tool names in docs and events.jsonl are `edit` and `create`.

**Current:**
```go
"Write": {"copilot-cli": "apply_patch"},
"Edit":  {"copilot-cli": "apply_patch"},
```

**Should be:**
```go
"Write": {"copilot-cli": "create"},
"Edit":  {"copilot-cli": "edit"},
```

### 2. Tool name: `rg` should be `grep` (LOW)

**File:** `cli/internal/converter/toolmap.go:56`

Research shows the tool appears in events.jsonl as `grep`, not `rg`. The underlying implementation uses ripgrep, but the tool name is `grep`.

**Current:** `"Grep": {"copilot-cli": "rg"}`
**Should be:** `"Grep": {"copilot-cli": "grep"}`

### 3. Tool name: `shell` should be `bash` (LOW)

**File:** `cli/internal/converter/toolmap.go:38`

Research shows the built-in tool is named `bash`, while `shell` is the *permission category*. The permission flag syntax is `shell(COMMAND:*)` but the tool itself is `bash`.

**Current:** `"Bash": {"copilot-cli": "shell"}`
**Should be:** `"Bash": {"copilot-cli": "bash"}`

### 4. MCP config filename: `mcp.json` vs `mcp-config.json` (MEDIUM)

**File:** `cli/internal/provider/copilot.go:54`

Research shows the MCP config file is `mcp-config.json` (both at `~/.copilot/mcp-config.json` and `.copilot/mcp-config.json`). Syllago uses `mcp.json`.

**Current:** `filepath.Join(projectRoot, ".copilot", "mcp.json")`
**Should be:** `filepath.Join(projectRoot, ".copilot", "mcp-config.json")`

### 5. Hook file location: single file vs directory of files (MEDIUM)

**Files:** `cli/internal/provider/copilot.go:56`, `cli/internal/catalog/native_scan.go:264`

Research shows Copilot CLI hooks live in `.github/hooks/*.json` (a directory of JSON files, each can contain multiple event types). Syllago expects a single `.copilot/hooks.json` file. Both the provider DiscoveryPaths and the native scan pattern are wrong.

**Current:** `.copilot/hooks.json` (single file)
**Should be:** `.github/hooks/*.json` (directory of files)

### 6. Copilot hook config missing `version` field (LOW)

**File:** `cli/internal/converter/hooks.go:147-148`

Research shows Copilot hook files have a required `"version": 1` top-level field. The `copilotHooksConfig` struct omits it, so rendered hooks will be missing the version key.

**Current:**
```go
type copilotHooksConfig struct {
    Hooks map[string][]copilotHookEntry `json:"hooks"`
}
```

**Should be:**
```go
type copilotHooksConfig struct {
    Version int                            `json:"version"`
    Hooks   map[string][]copilotHookEntry  `json:"hooks"`
}
```

And `renderCopilotHooks` should set `Version: 1`.

### 7. Copilot hook config missing `env` field (LOW)

**File:** `cli/internal/converter/hooks.go:139-144`

Research shows `copilotHookEntry` supports `env` (custom environment variables) and `cwd` fields. The struct only has `bash`, `powershell`, `timeoutSec`, and `comment`.

### 8. Copilot agent metadata missing fields (MEDIUM)

**File:** `cli/internal/converter/agents.go:50-55`

Research shows Copilot agent frontmatter supports `model`, `target`, `mcp-servers`, `disable-model-invocation`, `user-invocable`, and `metadata` fields. The `copilotAgentMeta` struct only has `name`, `description`, and `tools`. The `model` field is particularly notable since the research confirms it.

**Current:**
```go
type copilotAgentMeta struct {
    Name        string   `yaml:"name,omitempty"`
    Description string   `yaml:"description,omitempty"`
    Tools       []string `yaml:"tools,omitempty"`
}
```

Missing at minimum: `model`, `target`, `mcp-servers`.

### 9. Copilot agent output filename should be `*.agent.md` (LOW)

**File:** `cli/internal/converter/agents.go:260`

Research shows Copilot agents use `<name>.agent.md` filenames. Syllago renders to generic `agent.md`.

**Current:** `Filename: "agent.md"`
**Should be:** `Filename: slugify(meta.Name) + ".agent.md"` (or similar)

### 10. Matcher support claim is wrong (LOW)

**File:** `cli/internal/converter/hooks.go:409,413`

The code says "Copilot doesn't support matchers" and drops them with a warning. Research shows Copilot CLI *does* support the Claude Code nested `matcher`/`hooks` structure for cross-provider compatibility. Matchers should be preserved when rendering to Copilot format.

---

## Gaps

### 1. Missing hook events for copilot-cli

**File:** `cli/internal/converter/toolmap.go:76-87`

Research documents 8 hook events for Copilot CLI. The HookEvents map only has 5 copilot-cli entries. Missing:

| Canonical | Copilot CLI |
|-----------|-------------|
| (new) `SubagentStop` | `subagentStop` |
| (new) `AgentStop` | `agentStop` |
| `Stop` (or new) | `errorOccurred` |

The existing `SubagentStart` and `SubagentCompleted` canonical entries have empty maps `{}` -- they should map to copilot-cli equivalents (`subagentStop` at minimum).

### 2. Skills not supported in provider definition

**File:** `cli/internal/provider/copilot.go:72-79`

Research confirms Copilot CLI supports skills (`SKILL.md` files in `.github/skills/` and `~/.copilot/skills/`). The `SupportsType` function does not include `catalog.Skills`. The `InstallDir` and `DiscoveryPaths` functions have no skills cases.

### 3. Missing tool mappings: `web_fetch`, `web_search`, async bash tools

**File:** `cli/internal/converter/toolmap.go`

Research documents 13 Copilot CLI tools. Syllago maps 7. Missing mappings:

| Canonical | Copilot CLI |
|-----------|-------------|
| `WebFetch` | `web_fetch` |
| `WebSearch` | `web_search` |
| (no canonical) | `write_bash` |
| (no canonical) | `read_bash` |
| (no canonical) | `stop_bash` |
| (no canonical) | `skill` |

`WebSearch` has no copilot-cli entry even though there is a generic `WebSearch` key in the map.

### 4. Instructions/rules discovery paths incomplete

**File:** `cli/internal/provider/copilot.go:44`

Research shows Copilot CLI reads instructions from multiple sources:
- `.github/copilot-instructions.md` (repo-wide) -- this is in the code
- `.github/instructions/*.instructions.md` (path-specific) -- missing
- `AGENTS.md` / `CLAUDE.md` / `GEMINI.md` at repo root -- missing
- `~/.copilot/copilot-instructions.md` (personal) -- missing

Only `.github/copilot-instructions.md` is discovered.

### 5. MCP config: `tools` filter field not rendered

**File:** `cli/internal/converter/mcp.go:309-346`

Research shows Copilot MCP server configs support a `tools` field (`"*"` for all, or specific tool names). The `renderCopilotMCP` function doesn't emit this field.

### 6. Agent discovery path wrong/incomplete

**File:** `cli/internal/provider/copilot.go:48-52`

Research shows agents live in `.github/agents/` (not `.copilot/agents/`). The code does include `.github/agents/` but also lists `.copilot/agents/` which is not documented, and `.claude/agents/` as a "compatibility fallback" which is not how Copilot finds agents.

### 7. Prompt files not supported

Research documents `.prompt.md` files in `.github/prompts/` as a content type. This is primarily a VS Code feature and CLI support is unverified, but it may be worth tracking.

---

## Opportunities

### 1. Tool alias mapping for agents

Research reveals Copilot agents use *aliases* for tools in frontmatter: `execute`, `read`, `edit`, `search`, `agent`, `web`, `todo`. These are different from the actual tool names (`bash`, `view`, `edit`, `grep`/`glob`, `task`, `web_fetch`/`web_search`). The agent renderer should use aliases when populating the `tools` field, not the raw tool names.

### 2. Cross-provider instruction discovery

Copilot reads `CLAUDE.md` and `GEMINI.md` at repo root alongside `AGENTS.md`. This means syllago could detect content already shared cross-provider and avoid duplicating it.

### 3. Copilot hook `type: "command"` is required

Research shows every hook entry must have `"type": "command"`. The `copilotHookEntry` struct omits this field. While there's currently only one type, the schema requires it.

### 4. MCP type field mapping

Research shows Copilot uses `"local"` or `"stdio"` for stdio servers, `"http"` for Streamable HTTP, and `"sse"` (deprecated). The canonical format uses `"stdio"` / `"streamable-http"`. A type mapping layer may be needed.
