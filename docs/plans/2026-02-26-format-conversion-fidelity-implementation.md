# Format Conversion Fidelity - Implementation Plan

**Goal:** Complete format conversion so all 11 providers have accurate tool name translation, MCP merge support, hook warnings for unsupported targets, and Codex multi-agent TOML bidirectional conversion.

**Architecture:** Expand `toolmap.go`'s data tables for four new providers, add TOML agent conversion via `pelletier/go-toml/v2` in a new `codex_agents.go` file, extend `installer/mcp.go`'s `mcpConfigPath()` to accept a `projectDir` parameter for project-scoped providers, and wire hook warnings for hookless providers through the existing `renderStandardHooks` control flow.

**Tech Stack:** Go, pelletier/go-toml/v2 (already in go.mod at v2.2.4)

**Design Doc:** `docs/plans/2026-02-26-format-conversion-fidelity-design.md`

---

## Key Observations from Code Review

Before the tasks, a few facts that shaped the plan:

1. **`pelletier/go-toml/v2` is already in `cli/go.mod`** at v2.2.4. Task 9 from the design (add TOML dependency) is already done. No `go get` needed.
2. **`mcpConfigPath` has a function-pointer override pattern** (`var mcpConfigPath = mcpConfigPathImpl`) used by tests. The signature change (adding `projectDir string`) must update both the var type and the three call sites in `mcp.go`.
3. **Hookless providers need no new event tables.** `renderStandardHooks` already calls `TranslateHookEvent` and emits a warning when `!supported`. The warning message just needs to be provider-specific rather than generic. Checking the slug before the event loop is the right insertion point.
4. **Codex `SupportsType` currently returns false for `Agents`.** The provider file needs a one-line addition before `codex_agents.go` exists, so the dispatch in `agents.go` can be tested.
5. **MCP tool name format for new providers:** OpenCode uses `servername__toolname` (double underscore, same as Gemini), Zed uses `servername/toolname` (same as Copilot), Cline uses `servername__toolname`, Roo Code uses `servername__toolname`.

---

## Task 1 — Expand tool name translation table (toolmap.go)

**File:** `cli/internal/converter/toolmap.go`

**What:** Add OpenCode, Zed, Cline, and Roo Code entries to `ToolNames`. Verify Kiro entries are present (they are — `"kiro": "read"` for Read, etc.).

**Why this approach:** All four providers need forward (canonical → provider) and reverse (provider → canonical) translations. The existing map structure handles both directions through `TranslateTool` and `ReverseTranslateTool`. No structural change needed, just new map entries.

**Gotcha:** Zed's `edit_file` appears as both Write and Edit. The forward translation is fine (both emit `edit_file`). For reverse translation, the first match wins in `ReverseTranslateTool`'s range loop — there is no guaranteed iteration order, so `edit_file` may reverse to either `Write` or `Edit`. The design acknowledges this and accepts the disambiguation ambiguity. We will document this in a comment.

**Code — replace the `ToolNames` var in `cli/internal/converter/toolmap.go`:**

```go
// ToolNames maps canonical tool names (Claude Code) to provider-specific equivalents.
// Note: Zed uses "edit_file" for both Write and Edit. Reverse translation is ambiguous
// and may return either canonical name; round-trips through Zed lose the distinction.
var ToolNames = map[string]map[string]string{
	"Read": {
		"gemini-cli":  "read_file",
		"copilot-cli": "view",
		"kiro":        "read",
		"opencode":    "view",
		"zed":         "read_file",
		"cline":       "read_file",
		"roo-code":    "ReadFileTool",
	},
	"Write": {
		"gemini-cli":  "write_file",
		"copilot-cli": "apply_patch",
		"kiro":        "fs_write",
		"opencode":    "write",
		"zed":         "edit_file",
		"cline":       "write_to_file",
		"roo-code":    "WriteToFileTool",
	},
	"Edit": {
		"gemini-cli":  "replace",
		"copilot-cli": "apply_patch",
		"kiro":        "fs_write",
		"opencode":    "edit",
		"zed":         "edit_file",
		"cline":       "apply_diff",
		"roo-code":    "EditFileTool",
	},
	"Bash": {
		"gemini-cli":  "run_shell_command",
		"copilot-cli": "shell",
		"kiro":        "shell",
		"opencode":    "bash",
		"zed":         "terminal",
		"cline":       "execute_command",
		"roo-code":    "ExecuteCommandTool",
	},
	"Glob": {
		"gemini-cli":  "list_directory",
		"copilot-cli": "glob",
		"kiro":        "read",
		"opencode":    "glob",
		"zed":         "find_path",
		"cline":       "list_files",
		"roo-code":    "ListFilesTool",
	},
	"Grep": {
		"gemini-cli":  "grep_search",
		"copilot-cli": "rg",
		"kiro":        "read",
		"opencode":    "grep",
		"zed":         "grep",
		"cline":       "search_files",
		"roo-code":    "SearchFilesTool",
	},
	"WebSearch": {
		"gemini-cli": "google_search",
		"opencode":   "fetch",
		"zed":        "web_search",
	},
	"Task": {
		"copilot-cli": "task",
		"opencode":    "agent",
		"zed":         "subagent",
	},
}
```

**Success criteria:** File compiles. Existing tests still pass.

**Command:**

```
cd /home/hhewett/.local/src/syllago/cli && go build ./internal/converter/...
```

**Expected output:** No output (successful build).

---

## Task 2 — Tool name translation tests for new providers (toolmap_test.go)

**File:** `cli/internal/converter/toolmap_test.go`

**What:** Add table entries to `TestTranslateTool` and `TestReverseTranslateTool` covering all new forward and reverse mappings.

**What:** Append the following test cases to the existing `TestTranslateTool` test slice, and add a new `TestReverseTranslateTool_AllProviders` function.

**Depends on:** Task 1.

**Code — add cases to `TestTranslateTool` (append inside the existing `tests` slice):**

```go
// OpenCode
{"Read to OpenCode", "Read", "opencode", "view"},
{"Write to OpenCode", "Write", "opencode", "write"},
{"Edit to OpenCode", "Edit", "opencode", "edit"},
{"Bash to OpenCode", "Bash", "opencode", "bash"},
{"Glob to OpenCode", "Glob", "opencode", "glob"},
{"Grep to OpenCode", "Grep", "opencode", "grep"},
{"WebSearch to OpenCode", "WebSearch", "opencode", "fetch"},
{"Task to OpenCode", "Task", "opencode", "agent"},
// Zed
{"Read to Zed", "Read", "zed", "read_file"},
{"Write to Zed", "Write", "zed", "edit_file"},
{"Edit to Zed", "Edit", "zed", "edit_file"},
{"Bash to Zed", "Bash", "zed", "terminal"},
{"Glob to Zed", "Glob", "zed", "find_path"},
{"Grep to Zed", "Grep", "zed", "grep"},
{"WebSearch to Zed", "WebSearch", "zed", "web_search"},
{"Task to Zed", "Task", "zed", "subagent"},
// Cline
{"Read to Cline", "Read", "cline", "read_file"},
{"Write to Cline", "Write", "cline", "write_to_file"},
{"Edit to Cline", "Edit", "cline", "apply_diff"},
{"Bash to Cline", "Bash", "cline", "execute_command"},
{"Glob to Cline", "Glob", "cline", "list_files"},
{"Grep to Cline", "Grep", "cline", "search_files"},
{"WebSearch to Cline (no mapping)", "WebSearch", "cline", "WebSearch"},
{"Task to Cline (no mapping)", "Task", "cline", "Task"},
// Roo Code
{"Read to RooCode", "Read", "roo-code", "ReadFileTool"},
{"Write to RooCode", "Write", "roo-code", "WriteToFileTool"},
{"Edit to RooCode", "Edit", "roo-code", "EditFileTool"},
{"Bash to RooCode", "Bash", "roo-code", "ExecuteCommandTool"},
{"Glob to RooCode", "Glob", "roo-code", "ListFilesTool"},
{"Grep to RooCode", "Grep", "roo-code", "SearchFilesTool"},
{"WebSearch to RooCode (no mapping)", "WebSearch", "roo-code", "WebSearch"},
{"Task to RooCode (no mapping)", "Task", "roo-code", "Task"},
```

**Test helpers note:** The `assertEqual` helper used in these tests is defined in `cli/internal/converter/rules_test.go` (same package). Its signature is:

```go
func assertEqual(t *testing.T, expected, actual string) {
    t.Helper()
    if expected != actual {
        t.Errorf("expected %q, got %q", expected, actual)
    }
}
```

No additional definition needed — it is already available to all files in `package converter` test scope.

**Code — add new test function after `TestReverseTranslateTool`:**

```go
func TestReverseTranslateTool_AllProviders(t *testing.T) {
	tests := []struct {
		name   string
		tool   string
		source string
		want   string
	}{
		// OpenCode
		{"OpenCode view → Read", "view", "opencode", "Read"},
		{"OpenCode write → Write", "write", "opencode", "Write"},
		{"OpenCode edit → Edit", "edit", "opencode", "Edit"},
		{"OpenCode bash → Bash", "bash", "opencode", "Bash"},
		{"OpenCode glob → Glob", "glob", "opencode", "Glob"},
		{"OpenCode grep → Grep", "grep", "opencode", "Grep"},
		{"OpenCode fetch → WebSearch", "fetch", "opencode", "WebSearch"},
		{"OpenCode agent → Task", "agent", "opencode", "Task"},
		// Zed (edit_file is ambiguous; we only test the ones with unique names)
		{"Zed read_file → Read", "read_file", "zed", "Read"},
		{"Zed terminal → Bash", "terminal", "zed", "Bash"},
		{"Zed find_path → Glob", "find_path", "zed", "Glob"},
		{"Zed grep → Grep", "grep", "zed", "Grep"},
		{"Zed web_search → WebSearch", "web_search", "zed", "WebSearch"},
		{"Zed subagent → Task", "subagent", "zed", "Task"},
		// Cline
		{"Cline read_file → Read", "read_file", "cline", "Read"},
		{"Cline write_to_file → Write", "write_to_file", "cline", "Write"},
		{"Cline apply_diff → Edit", "apply_diff", "cline", "Edit"},
		{"Cline execute_command → Bash", "execute_command", "cline", "Bash"},
		{"Cline list_files → Glob", "list_files", "cline", "Glob"},
		{"Cline search_files → Grep", "search_files", "cline", "Grep"},
		// Roo Code
		{"RooCode ReadFileTool → Read", "ReadFileTool", "roo-code", "Read"},
		{"RooCode WriteToFileTool → Write", "WriteToFileTool", "roo-code", "Write"},
		{"RooCode EditFileTool → Edit", "EditFileTool", "roo-code", "Edit"},
		{"RooCode ExecuteCommandTool → Bash", "ExecuteCommandTool", "roo-code", "Bash"},
		{"RooCode ListFilesTool → Glob", "ListFilesTool", "roo-code", "Glob"},
		{"RooCode SearchFilesTool → Grep", "SearchFilesTool", "roo-code", "Grep"},
		// Unknown passes through
		{"Unknown tool passes through", "no_such_tool", "opencode", "no_such_tool"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReverseTranslateTool(tt.tool, tt.source)
			assertEqual(t, tt.want, got)
		})
	}
}
```

**Command:**

```
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/converter/ -run TestTranslateTool -v
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/converter/ -run TestReverseTranslateTool -v
```

**Expected output:** All test cases pass (PASS).

---

## Task 3 — MCP tool name format for new providers (toolmap.go)

**File:** `cli/internal/converter/toolmap.go`

**What:** Extend `TranslateMCPToolName` and `parseMCPToolName` to handle OpenCode, Zed, Cline, and Roo Code formats.

**How it works:** OpenCode, Cline, and Roo Code all use `servername__toolname` (double underscore — same as Gemini). Zed uses `servername/toolname` (same as Copilot). The `parseMCPToolName` switch gets new cases; `TranslateMCPToolName` gets new render cases for each slug.

**Gotcha:** Zed parsing reuses the `copilot-cli` logic (slash separator). OpenCode/Cline/Roo Code parsing reuses the `gemini-cli` logic (double underscore). Extract these into named groups in the switch to avoid duplicating the parsing logic.

**Code — replace `parseMCPToolName` function:**

```go
// parseMCPToolName extracts server and tool from a provider-specific MCP tool name.
func parseMCPToolName(name, sourceSlug string) (server, tool string) {
	switch sourceSlug {
	case "claude-code", "kiro":
		// mcp__server__tool
		if !strings.HasPrefix(name, "mcp__") {
			return "", ""
		}
		rest := strings.TrimPrefix(name, "mcp__")
		parts := strings.SplitN(rest, "__", 2)
		if len(parts) != 2 {
			return "", ""
		}
		return parts[0], parts[1]
	case "gemini-cli", "opencode", "cline", "roo-code":
		// server__tool
		parts := strings.SplitN(name, "__", 2)
		if len(parts) != 2 {
			return "", ""
		}
		return parts[0], parts[1]
	case "copilot-cli", "zed":
		// server/tool
		parts := strings.SplitN(name, "/", 2)
		if len(parts) != 2 {
			return "", ""
		}
		return parts[0], parts[1]
	default:
		return "", ""
	}
}
```

**Code — replace `TranslateMCPToolName` switch block:**

```go
// TranslateMCPToolName translates MCP tool name format between providers.
// Claude/Kiro: mcp__server__tool
// Gemini/OpenCode/Cline/RooCode: server__tool
// Copilot/Zed: server/tool
func TranslateMCPToolName(name, sourceSlug, targetSlug string) string {
	// Normalize to parts: server, tool
	server, tool := parseMCPToolName(name, sourceSlug)
	if server == "" {
		return name // not an MCP tool name
	}

	switch targetSlug {
	case "claude-code", "kiro":
		return "mcp__" + server + "__" + tool
	case "gemini-cli", "opencode", "cline", "roo-code":
		return server + "__" + tool
	case "copilot-cli", "zed":
		return server + "/" + tool
	default:
		return name
	}
}
```

**Command:**

```
cd /home/hhewett/.local/src/syllago/cli && go build ./internal/converter/...
```

**Expected output:** No output (successful build).

---

## Task 4 — MCP tool name tests for new providers (toolmap_test.go)

**File:** `cli/internal/converter/toolmap_test.go`

**What:** Add test cases to `TestTranslateMCPToolName` covering all new provider source and target combinations.

**Depends on:** Task 3.

**Code — append cases to the existing `tests` slice in `TestTranslateMCPToolName`:**

```go
// OpenCode as source
{"OpenCode to Claude", "github__search_repositories", "opencode", "claude-code", "mcp__github__search_repositories"},
{"OpenCode to Zed", "github__search_repositories", "opencode", "zed", "github/search_repositories"},
{"OpenCode to Cline", "github__search_repositories", "opencode", "cline", "github__search_repositories"},
// Zed as source
{"Zed to Claude", "github/search_repositories", "zed", "claude-code", "mcp__github__search_repositories"},
{"Zed to OpenCode", "github/search_repositories", "zed", "opencode", "github__search_repositories"},
// Cline as source
{"Cline to Claude", "github__search_repositories", "cline", "claude-code", "mcp__github__search_repositories"},
{"Cline to Zed", "github__search_repositories", "cline", "zed", "github/search_repositories"},
// Roo Code as source
{"RooCode to Claude", "github__search_repositories", "roo-code", "claude-code", "mcp__github__search_repositories"},
{"RooCode to Gemini", "github__search_repositories", "roo-code", "gemini-cli", "github__search_repositories"},
// Claude to new targets
{"Claude to OpenCode", "mcp__github__search_repositories", "claude-code", "opencode", "github__search_repositories"},
{"Claude to Cline", "mcp__github__search_repositories", "claude-code", "cline", "github__search_repositories"},
{"Claude to RooCode", "mcp__github__search_repositories", "claude-code", "roo-code", "github__search_repositories"},
{"Claude to Zed", "mcp__github__search_repositories", "claude-code", "zed", "github/search_repositories"},
// Non-MCP tool still passes through
{"Non-MCP unchanged for OpenCode", "regular_tool", "opencode", "claude-code", "regular_tool"},
```

**Command:**

```
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/converter/ -run TestTranslateMCPToolName -v
```

**Expected output:** All listed cases PASS.

---

## Task 5 — Hook warnings for hookless providers (hooks.go)

**File:** `cli/internal/converter/hooks.go`

**What:** When rendering hooks to OpenCode, Zed, Cline, or Roo Code, emit a single provider-level warning and return empty content rather than silently dropping hooks event-by-event.

**Why this approach:** The existing `renderStandardHooks` already handles per-event unsupported warnings via `TranslateHookEvent`. But for providers that support zero events, the per-event warnings would all fire but the structure is wrong — the user should see one clear message, not six. Adding a provider-level early return in `Render` is cleaner.

**How it works:** Before dispatching in `HooksConverter.Render`, check if the target slug is in a "no hooks" set. If so, return a `Result` with no content, no filename, and a single warning. The empty `Content` signals "nothing to write" (same convention used by other renderers for skipped items).

**Code — modify `HooksConverter.Render` in `cli/internal/converter/hooks.go`:**

Replace the existing `Render` method:

```go
// hooklessProviders is the set of provider slugs that have no documented hook system.
// Converting hooks to these providers emits a data-loss warning rather than silently dropping.
var hooklessProviders = map[string]bool{
	"opencode": true,
	"zed":      true,
	"cline":    true,
	"roo-code": true,
}

func (c *HooksConverter) Render(content []byte, target provider.Provider) (*Result, error) {
	// Hookless providers: emit one clear warning instead of silently dropping all events.
	if hooklessProviders[target.Slug] {
		return &Result{
			Warnings: []string{
				fmt.Sprintf("Target provider %q does not support hooks; hook content was not converted", target.Name),
			},
		}, nil
	}

	var cfg hooksConfig
	if err := json.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("parsing canonical hooks: %w", err)
	}

	mode := c.LLMHooksMode
	if mode == "" {
		mode = LLMHooksModeSkip
	}

	switch target.Slug {
	case "copilot-cli":
		return renderCopilotHooks(cfg, mode)
	case "kiro":
		return renderKiroHooks(cfg, mode)
	default:
		// Claude Code and Gemini CLI
		return renderStandardHooks(cfg, target.Slug, mode)
	}
}
```

**Success criteria:** Rendering hooks to OpenCode/Zed/Cline/Roo Code returns a `Result` with `Content == nil`, `Filename == ""`, and exactly one warning.

**Command:**

```
cd /home/hhewett/.local/src/syllago/cli && go build ./internal/converter/...
```

**Expected output:** No output (successful build).

---

## Task 6 — Hook warning tests for hookless providers (hooks_test.go)

**File:** `cli/internal/converter/hooks_test.go`

**What:** Add a test that verifies hookless providers return the expected warning and no content.

**Depends on:** Task 5.

**Code — append to `cli/internal/converter/hooks_test.go`:**

```go
func TestHooksToHooklessProviders(t *testing.T) {
	input := []byte(`{
		"hooks": {
			"PreToolUse": [
				{
					"matcher": "Bash",
					"hooks": [{"type": "command", "command": "echo hi"}]
				}
			]
		}
	}`)

	conv := &HooksConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	hooklessTargets := []provider.Provider{
		provider.OpenCode,
		provider.Zed,
		provider.Cline,
		provider.RooCode,
	}

	for _, target := range hooklessTargets {
		t.Run(target.Name, func(t *testing.T) {
			result, err := conv.Render(canonical.Content, target)
			if err != nil {
				t.Fatalf("Render(%s): %v", target.Name, err)
			}
			if result.Content != nil {
				t.Errorf("expected nil Content for hookless provider %s, got %q", target.Name, result.Content)
			}
			if result.Filename != "" {
				t.Errorf("expected empty Filename for hookless provider %s, got %q", target.Name, result.Filename)
			}
			if len(result.Warnings) != 1 {
				t.Errorf("expected exactly 1 warning for %s, got %d: %v", target.Name, len(result.Warnings), result.Warnings)
			}
			assertContains(t, result.Warnings[0], "does not support hooks")
		})
	}
}
```

**Command:**

```
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/converter/ -run TestHooksToHooklessProviders -v
```

**Expected output:** All four sub-tests PASS.

---

## Task 7 — MCP installer path expansion (installer/mcp.go)

**File:** `cli/internal/installer/mcp.go`

**What:** Extend `mcpConfigPath()` to accept `projectDir string` as a second parameter, and add cases for Copilot, Kiro, OpenCode, Zed, Cline, and Roo Code.

**Why this approach:** Project-scoped providers (Copilot, Kiro, OpenCode, Cline, Roo Code) need a project root to build the config path, not the home directory. The design says to extend the function signature rather than change the provider abstraction. All three call sites in `mcp.go` are internal and pass `repoRoot` as the third argument already — we thread that through.

**Gotcha:** The `mcpConfigPath` var is used in tests with a function override (`mcpConfigPath = func(...) (string, error)`). Changing the signature means updating the var type annotation, the `mcpConfigPathImpl` definition, and all override sites in `mcp_test.go`.

**Gotcha (Zed):** Zed stores MCP under `context_servers`, not `mcpServers`. The `installMCP`, `uninstallMCP`, and `checkMCPStatus` functions all hardcode the `mcpServers.` key prefix. We need a `mcpConfigKey` helper that returns the correct key for the provider.

**Code — replace `mcpConfigPath` declaration and implementation in `cli/internal/installer/mcp.go`:**

```go
// mcpConfigPath returns the config file path where MCP servers are stored for the given provider.
// projectDir is the project root for project-scoped providers; ignored for user-scoped ones.
// Declared as a var so tests can override it.
var mcpConfigPath = mcpConfigPathImpl

func mcpConfigPathImpl(prov provider.Provider, projectDir string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch prov.Slug {
	case "claude-code":
		return filepath.Join(home, ".claude.json"), nil
	case "gemini-cli":
		return filepath.Join(home, prov.ConfigDir, "settings.json"), nil
	case "copilot-cli":
		return filepath.Join(projectDir, ".copilot", "mcp.json"), nil
	case "kiro":
		return filepath.Join(projectDir, ".kiro", "settings", "mcp.json"), nil
	case "opencode":
		return filepath.Join(projectDir, "opencode.json"), nil
	case "zed":
		return filepath.Join(home, ".config", "zed", "settings.json"), nil
	case "cline":
		return filepath.Join(projectDir, ".vscode", "mcp.json"), nil
	case "roo-code":
		return filepath.Join(projectDir, ".roo", "mcp.json"), nil
	}
	return "", fmt.Errorf("MCP config path not defined for %s", prov.Name)
}

// mcpConfigKey returns the JSON key under which MCP servers are stored for the provider.
// Most providers use "mcpServers"; Zed uses "context_servers".
func mcpConfigKey(prov provider.Provider) string {
	if prov.Slug == "zed" {
		return "context_servers"
	}
	return "mcpServers"
}
```

**Code — update the three call sites to pass `repoRoot` as the second argument:**

Context: all three functions currently have `_ string` as their third parameter (the blank identifier, meaning the value is received but ignored). The caller in `installer.go` passes `repoRoot` as that argument. The current function signatures are:

```go
func installMCP(item catalog.ContentItem, prov provider.Provider, _ string) (string, error)
func uninstallMCP(item catalog.ContentItem, prov provider.Provider, _ string) (string, error)
func checkMCPStatus(item catalog.ContentItem, prov provider.Provider, _ string) Status
```

Rename `_ string` to `repoRoot string` in each function signature so the value is accessible, then pass it to `mcpConfigPath`:

In `installMCP`:
```go
cfgPath, err := mcpConfigPath(prov, repoRoot)
```

In `uninstallMCP`:
```go
cfgPath, err := mcpConfigPath(prov, repoRoot)
```

In `checkMCPStatus`:
```go
cfgPath, err := mcpConfigPath(prov, repoRoot)
```

**Code — update the hardcoded `"mcpServers."` key in all three functions to use `mcpConfigKey`:**

In `installMCP`, replace:
```go
key := "mcpServers." + item.Name
```
with:
```go
key := mcpConfigKey(prov) + "." + item.Name
```

In `uninstallMCP`, replace both occurrences:
```go
key := "mcpServers." + item.Name
```
and:
```go
if !gjson.GetBytes(fileData, key+"._syllago").Bool() {
```
The `key` variable update covers both — replace only the assignment line.

In `checkMCPStatus`, replace:
```go
key := "mcpServers." + item.Name
```
with:
```go
key := mcpConfigKey(prov) + "." + item.Name
```

Also update the return message in `installMCP`:
```go
return fmt.Sprintf("%s.%s in %s", mcpConfigKey(prov), item.Name, cfgPath), nil
```

And in `uninstallMCP`:
```go
return fmt.Sprintf("%s.%s from %s", mcpConfigKey(prov), item.Name, cfgPath), nil
```

**Code — add `readMCPConfig` helper that handles JSONC for OpenCode:**

The design requires that when reading OpenCode's `opencode.json`, comments are stripped before JSON parsing (OpenCode uses JSONC format). All three functions (`installMCP`, `uninstallMCP`, `checkMCPStatus`) currently read the config file and pass raw bytes to `gjson`. Add a helper that strips comments for JSONC providers, and call it instead of reading raw bytes directly.

Add this helper to `cli/internal/installer/mcp.go`:

```go
// readMCPConfig reads the config file and strips JSONC comments for providers that use them.
// OpenCode uses JSONC (JSON with comments); all other providers use plain JSON.
func readMCPConfig(cfgPath string, prov provider.Provider) ([]byte, error) {
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, err
	}
	if prov.Slug == "opencode" {
		return converter.ParseJSONC(data)
	}
	return data, nil
}
```

Then in `installMCP`, `uninstallMCP`, and `checkMCPStatus`, replace the existing `os.ReadFile(cfgPath)` call (where the result is passed to gjson) with `readMCPConfig(cfgPath, prov)`:

In `installMCP`:
```go
fileData, err := readMCPConfig(cfgPath, prov)
```

In `uninstallMCP`:
```go
fileData, err := readMCPConfig(cfgPath, prov)
```

In `checkMCPStatus`:
```go
fileData, err := readMCPConfig(cfgPath, prov)
```

**Note:** This requires importing `"github.com/OpenScribbler/syllago/cli/internal/converter"` in `mcp.go`. Verify the module path matches the existing imports in that file.

**Command:**

```
cd /home/hhewett/.local/src/syllago/cli && go build ./internal/installer/...
```

**Expected output:** No output (successful build).

---

## Task 8 — MCP installer tests for new providers (installer/mcp_test.go)

**File:** `cli/internal/installer/mcp_test.go`

**What:** Update the existing test's function override to match the new signature, and add tests for project-scoped path resolution and Zed's `context_servers` key.

**Depends on:** Task 7.

**Code — update the override in `TestInstallMCP_WhitelistsFields`:**

```go
// Override mcpConfigPath for test
originalFunc := mcpConfigPath
mcpConfigPath = func(p provider.Provider, projectDir string) (string, error) {
    return configFile, nil
}
defer func() { mcpConfigPath = originalFunc }()
```

**Code — add new test functions after the existing test:**

```go
func TestMCPConfigPath_ProjectScoped(t *testing.T) {
	t.Parallel()
	// Save and restore the real implementation
	origImpl := mcpConfigPath
	defer func() { mcpConfigPath = origImpl }()

	// Use the real impl directly
	mcpConfigPath = mcpConfigPathImpl

	projectDir := "/tmp/myproject"

	tests := []struct {
		prov    provider.Provider
		wantSub string // expected path substring
	}{
		{provider.Copilot, ".copilot/mcp.json"},
		{provider.Kiro, ".kiro/settings/mcp.json"},
		{provider.OpenCode, "opencode.json"},
		{provider.Cline, ".vscode/mcp.json"},
		{provider.RooCode, ".roo/mcp.json"},
	}

	for _, tt := range tests {
		t.Run(tt.prov.Name, func(t *testing.T) {
			path, err := mcpConfigPath(tt.prov, projectDir)
			if err != nil {
				t.Fatalf("mcpConfigPath(%s): %v", tt.prov.Name, err)
			}
			if !filepath.IsAbs(path) {
				t.Errorf("expected absolute path, got %q", path)
			}
			if !strings.HasSuffix(filepath.ToSlash(path), tt.wantSub) {
				t.Errorf("expected path ending in %q, got %q", tt.wantSub, path)
			}
		})
	}
}

func TestMCPConfigKey_Zed(t *testing.T) {
	if got := mcpConfigKey(provider.Zed); got != "context_servers" {
		t.Errorf("Zed key: want %q, got %q", "context_servers", got)
	}
}

func TestMCPConfigKey_Default(t *testing.T) {
	for _, prov := range []provider.Provider{provider.ClaudeCode, provider.GeminiCLI, provider.Cline, provider.RooCode} {
		if got := mcpConfigKey(prov); got != "mcpServers" {
			t.Errorf("%s key: want %q, got %q", prov.Name, "mcpServers", got)
		}
	}
}

func TestInstallMCP_ZedUsesContextServers(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	itemDir := filepath.Join(tmpDir, "zed-mcp")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}

	configData := map[string]interface{}{
		"type":    "stdio",
		"command": "npx",
		"args":    []string{"my-mcp-server"},
	}
	configJSON, err := json.Marshal(configData)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemDir, "config.json"), configJSON, 0644); err != nil {
		t.Fatal(err)
	}

	item := catalog.ContentItem{
		Name: "my-server",
		Type: catalog.MCP,
		Path: itemDir,
	}

	configFile := filepath.Join(tmpDir, "settings.json")
	if err := os.WriteFile(configFile, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	origFunc := mcpConfigPath
	mcpConfigPath = func(p provider.Provider, projectDir string) (string, error) {
		return configFile, nil
	}
	defer func() { mcpConfigPath = origFunc }()

	if _, err := installMCP(item, provider.Zed, tmpDir); err != nil {
		t.Fatalf("installMCP: %v", err)
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatal(err)
	}

	// Key must be context_servers, not mcpServers
	if !gjson.GetBytes(data, "context_servers.my-server").Exists() {
		t.Errorf("expected context_servers.my-server, got: %s", data)
	}
	if gjson.GetBytes(data, "mcpServers.my-server").Exists() {
		t.Error("mcpServers key should NOT be present for Zed")
	}
}
```

Note: this test requires the `strings` import — add it to the import block if not already present in the test file.

**Code — add new test function for OpenCode JSONC handling:**

```go
func TestInstallMCP_OpenCodeStripsJSONCComments(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	itemDir := filepath.Join(tmpDir, "opencode-mcp")
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}

	configData := map[string]interface{}{
		"type":    "stdio",
		"command": "npx",
		"args":    []string{"my-mcp-server"},
	}
	configJSON, err := json.Marshal(configData)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemDir, "config.json"), configJSON, 0644); err != nil {
		t.Fatal(err)
	}

	item := catalog.ContentItem{
		Name: "my-server",
		Type: catalog.MCP,
		Path: itemDir,
	}

	// Write an opencode.json with JSONC-style comments — installMCP must not fail parsing this.
	jsoncContent := []byte(`{
	// This is a JSONC comment
	"mcpServers": {}
}`)
	configFile := filepath.Join(tmpDir, "opencode.json")
	if err := os.WriteFile(configFile, jsoncContent, 0644); err != nil {
		t.Fatal(err)
	}

	origFunc := mcpConfigPath
	mcpConfigPath = func(p provider.Provider, projectDir string) (string, error) {
		return configFile, nil
	}
	defer func() { mcpConfigPath = origFunc }()

	if _, err := installMCP(item, provider.OpenCode, tmpDir); err != nil {
		t.Fatalf("installMCP with JSONC config: %v", err)
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatal(err)
	}

	if !gjson.GetBytes(data, "mcpServers.my-server").Exists() {
		t.Errorf("expected mcpServers.my-server to be present, got: %s", data)
	}
}
```

**Command:**

```
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/installer/ -run TestMCP -v
```

**Expected output:** All MCP tests PASS including the new ones.

---

## Task 9 — Add Agents to Codex provider support (provider/codex.go)

**File:** `cli/internal/provider/codex.go`

**What:** Add `catalog.Agents` to `SupportsType` and add an `InstallDir` case and `DiscoveryPaths` case for agents.

**Why this order:** The Codex agent converter (`codex_agents.go`) registers itself via `init()` and the `AgentsConverter.Render` dispatch uses `target.Slug`. The provider must declare support before tests can exercise the full pipeline.

**How it works:** Codex agents live in a single `config.toml` file in `~/.codex/` or `.codex/` at the project root. The install path for agents is the `.codex/` directory (the converter emits `config.toml` as the filename).

**Code — replace `Codex` var in `cli/internal/provider/codex.go`:**

```go
var Codex = Provider{
	Name:      "Codex",
	Slug:      "codex",
	ConfigDir: ".codex",
	InstallDir: func(homeDir string, ct catalog.ContentType) string {
		switch ct {
		case catalog.Rules:
			return filepath.Join(homeDir, ".codex")
		case catalog.Commands:
			return filepath.Join(homeDir, ".codex")
		case catalog.Agents:
			return filepath.Join(homeDir, ".codex")
		}
		return ""
	},
	Detect: func(homeDir string) bool {
		// Check for .codex directory
		info, err := os.Stat(filepath.Join(homeDir, ".codex"))
		if err == nil && info.IsDir() {
			return true
		}
		// Also check if codex command exists
		_, err = exec.LookPath("codex")
		return err == nil
	},
	DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
		switch ct {
		case catalog.Rules:
			return []string{filepath.Join(projectRoot, "AGENTS.md")}
		case catalog.Commands:
			return []string{filepath.Join(projectRoot, ".codex", "commands")}
		case catalog.Agents:
			return []string{filepath.Join(projectRoot, ".codex", "config.toml")}
		default:
			return nil
		}
	},
	FileFormat: func(ct catalog.ContentType) Format {
		switch ct {
		case catalog.Agents:
			return FormatTOML
		default:
			return FormatMarkdown
		}
	},
	EmitPath: func(projectRoot string) string {
		return filepath.Join(projectRoot, "AGENTS.md")
	},
	SupportsType: func(ct catalog.ContentType) bool {
		switch ct {
		case catalog.Rules, catalog.Commands, catalog.Agents:
			return true
		default:
			return false
		}
	},
}
```

**Prerequisite — add `FormatTOML` to `cli/internal/provider/provider.go`:**

The current `Format` const block in `provider.go` contains `FormatMarkdown`, `FormatMDC`, `FormatJSON`, `FormatYAML`, and `FormatJSONC` — but not `FormatTOML`. This must be added before `codex.go` will compile. In `cli/internal/provider/provider.go`, update the const block:

```go
const (
    FormatMarkdown Format = "md"
    FormatMDC      Format = "mdc"   // Cursor .mdc format
    FormatJSON     Format = "json"
    FormatYAML     Format = "yaml"
    FormatJSONC    Format = "jsonc" // JSON with comments (OpenCode)
    FormatTOML     Format = "toml"  // Codex config.toml
)
```

Do this **before** updating `codex.go`, or the build will fail.

**Command:**

```
cd /home/hhewett/.local/src/syllago/cli && go build ./internal/provider/...
```

**Expected output:** No output (successful build).

---

## Task 10 — Codex agent TOML converter (converter/codex_agents.go)

**File:** `cli/internal/converter/codex_agents.go` (new file)

**What:** Implement bidirectional Codex TOML ↔ canonical agent conversion.

**How it works:**
- **Canonicalize:** Parse TOML with `pelletier/go-toml/v2`. Extract each `[agents.<name>]` section. Map `model`, `prompt`, `tools` to `AgentMeta`. Reverse-translate tool names from Codex names to canonical. `tools` in Codex use names like `shell`, `view`, `apply_patch` — these are Copilot CLI lineage tools, so reverse-translate against `copilot-cli` slug (Codex shares the same tool name vocabulary). Produce one `AgentMeta` per agent section.
- **Render:** Collect all agents for Codex into a single `config.toml`. Build `[features] multi_agent = true` block. For each agent, emit `[agents.<slug>]` with `model`, `prompt`, `tools`. Forward-translate canonical tool names using `copilot-cli` slug (Codex tool vocabulary matches Copilot: `shell`, `view`, `apply_patch`, etc.). Emit warnings for unsupported fields.

**Gotcha:** Canonicalize produces multiple agents from one file. The `Canonicalize` interface returns a single `*Result`. We handle this by returning only the first agent in `Canonicalize` (matching how other converters work — one file in, one canonical file out). For the scanner to extract all agents from one TOML, a separate `CanonicalizeAll` function handles the multi-agent case. But for the interface contract: return the first agent, store the full TOML content in the result's `ExtraFiles` for reference.

**Revision of approach:** Re-read the design — "Canonicalization produces one canonical agent per `[agents.*]` section." The design expects the importer/scanner to call canonicalize per-agent-section, not per-file. For the `Converter` interface (one `[]byte` in, one `*Result` out), we will canonicalize the entire TOML and return the first agent as the primary result, plus additional agents in `ExtraFiles` keyed by their filename. This allows the importer to discover and save each agent file.

Actually, the cleanest approach matching the existing interface: `Canonicalize` accepts the full TOML content and returns the first agent as `Content` (canonical markdown). Additional agents are placed in `ExtraFiles["agent-<name>.md"]`. The installer or importer can iterate `ExtraFiles` to save subsequent agents.

**Helper functions note:** The code below calls two helpers that already exist in `cli/internal/converter/agents.go` — no additional definition needed:

- `buildAgentCanonical(meta AgentMeta, body string) ([]byte, error)` — constructs canonical agent markdown by marshaling `meta` to YAML frontmatter and appending `body`. Defined at line ~525 of `cli/internal/converter/agents.go`.
- `StripConversionNotes(body string) string` — removes "Conversion Note" sections added by other converters. Defined in `cli/internal/converter/embed.go`. Both are in `package converter` and accessible from `codex_agents.go` without any import.

**Code — new file `cli/internal/converter/codex_agents.go`:**

```go
package converter

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// codexConfig is the top-level structure of a Codex config.toml.
type codexConfig struct {
	Features codexFeatures        `toml:"features"`
	Agents   map[string]codexAgent `toml:"agents"`
}

type codexFeatures struct {
	MultiAgent bool `toml:"multi_agent"`
}

type codexAgent struct {
	Model  string   `toml:"model"`
	Prompt string   `toml:"prompt"`
	Tools  []string `toml:"tools"`
}

// canonicalizeCodexAgents converts a Codex config.toml (multi-agent TOML) to canonical agent
// markdown files. The first agent is returned as Content; additional agents are in ExtraFiles.
func canonicalizeCodexAgents(content []byte) (*Result, error) {
	var cfg codexConfig
	if err := toml.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("parsing Codex config.toml: %w", err)
	}

	if len(cfg.Agents) == 0 {
		return &Result{Content: []byte{}, Filename: "agent.md"}, nil
	}

	// Stable iteration order: collect and sort agent names
	names := make([]string, 0, len(cfg.Agents))
	for name := range cfg.Agents {
		names = append(names, name)
	}
	sortStrings(names)

	extraFiles := map[string][]byte{}
	var firstContent []byte
	var firstName string

	for i, name := range names {
		agent := cfg.Agents[name]

		// Translate Codex tool names to canonical.
		// Codex uses Copilot CLI tool vocabulary: shell, view, apply_patch, etc.
		var canonicalTools []string
		for _, t := range agent.Tools {
			canonicalTools = append(canonicalTools, ReverseTranslateTool(t, "copilot-cli"))
		}

		meta := AgentMeta{
			Name:  name,
			Tools: canonicalTools,
			Model: agent.Model,
		}

		canonical, err := buildAgentCanonical(meta, agent.Prompt)
		if err != nil {
			return nil, fmt.Errorf("building canonical for agent %q: %w", name, err)
		}

		filename := slugify(name) + ".md"
		if i == 0 {
			firstContent = canonical
			firstName = filename
		} else {
			extraFiles[filename] = canonical
		}
	}

	return &Result{
		Content:    firstContent,
		Filename:   firstName,
		ExtraFiles: extraFiles,
	}, nil
}

// renderCodexAgents converts a canonical agent markdown to a Codex config.toml snippet.
// Multiple calls are expected to be collected and merged into one file by the caller.
func renderCodexAgents(meta AgentMeta, body string) (*Result, error) {
	var warnings []string
	cleanBody := StripConversionNotes(body)

	// Emit warnings for fields Codex TOML doesn't support.
	if meta.MaxTurns > 0 {
		warnings = append(warnings, fmt.Sprintf("maxTurns (%d) not supported in Codex TOML agent format (dropped)", meta.MaxTurns))
	}
	if meta.PermissionMode != "" {
		warnings = append(warnings, fmt.Sprintf("permissionMode (%q) not supported in Codex TOML agent format (dropped)", meta.PermissionMode))
	}
	if len(meta.Skills) > 0 {
		warnings = append(warnings, "skills not supported in Codex TOML agent format (dropped)")
	}
	if len(meta.MCPServers) > 0 {
		warnings = append(warnings, "mcpServers not supported in Codex TOML agent format (dropped)")
	}
	if meta.Memory != "" {
		warnings = append(warnings, "memory not supported in Codex TOML agent format (dropped)")
	}
	if meta.Background {
		warnings = append(warnings, "background not supported in Codex TOML agent format (dropped)")
	}
	if meta.Isolation != "" {
		warnings = append(warnings, "isolation not supported in Codex TOML agent format (dropped)")
	}
	if len(meta.DisallowedTools) > 0 {
		warnings = append(warnings, "disallowedTools not supported in Codex TOML agent format (dropped)")
	}

	// Translate canonical tool names to Codex vocabulary (Copilot CLI lineage).
	codexTools := TranslateTools(meta.Tools, "copilot-cli")

	// Use agent name as slug for the TOML key.
	agentKey := meta.Name
	if agentKey == "" {
		agentKey = "agent"
	}
	// Normalize to valid TOML key: lowercase, replace spaces with underscores.
	agentKey = strings.ToLower(agentKey)
	agentKey = strings.ReplaceAll(agentKey, " ", "_")
	agentKey = strings.ReplaceAll(agentKey, "-", "_")

	// Build TOML output using manual string building to keep it readable.
	// We don't marshal the full codexConfig to avoid emitting empty features blocks
	// for single-agent renders — the caller assembles the full file.
	var buf bytes.Buffer
	buf.WriteString("[features]\n")
	buf.WriteString("multi_agent = true\n\n")
	buf.WriteString(fmt.Sprintf("[agents.%s]\n", agentKey))
	if meta.Model != "" {
		buf.WriteString(fmt.Sprintf("model = %q\n", meta.Model))
	}
	buf.WriteString(fmt.Sprintf("prompt = %q\n", cleanBody))
	if len(codexTools) > 0 {
		buf.WriteString("tools = [")
		for i, tool := range codexTools {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(fmt.Sprintf("%q", tool))
		}
		buf.WriteString("]\n")
	}

	return &Result{
		Content:  buf.Bytes(),
		Filename: "config.toml",
		Warnings: warnings,
	}, nil
}

// sortStrings sorts a string slice in-place (avoids importing sort for a small helper).
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
```

**Command:**

```
cd /home/hhewett/.local/src/syllago/cli && go build ./internal/converter/...
```

**Expected output:** No output (successful build).

---

## Task 11 — Codex agent TOML tests (converter/codex_agents_test.go)

**File:** `cli/internal/converter/codex_agents_test.go` (new file)

**What:** Tests covering TOML parse, tool name translation, field-drop warnings, and a full roundtrip.

**Depends on:** Task 10.

**Test helpers note:** The tests below use `assertEqual`, `assertContains`, and `assertNotContains`. All three are defined in `cli/internal/converter/rules_test.go` in `package converter`. Their signatures are:

```go
func assertEqual(t *testing.T, expected, actual string)
func assertContains(t *testing.T, haystack, needle string)
func assertNotContains(t *testing.T, haystack, needle string)
```

These are available to all `*_test.go` files in `package converter` — no additional definition needed in `codex_agents_test.go`.

**Code — new file `cli/internal/converter/codex_agents_test.go`:**

```go
package converter

import (
	"testing"
)

func TestCanonicalizeCodexAgents_Single(t *testing.T) {
	input := []byte(`
[features]
multi_agent = true

[agents.reviewer]
model = "o4-mini"
prompt = "You are a code reviewer."
tools = ["shell", "view"]
`)
	result, err := canonicalizeCodexAgents(input)
	if err != nil {
		t.Fatalf("canonicalizeCodexAgents: %v", err)
	}

	out := string(result.Content)
	// Frontmatter name should be set
	assertContains(t, out, "name: reviewer")
	// Model should be present
	assertContains(t, out, "model: o4-mini")
	// Tool names should be reverse-translated from Copilot vocabulary to canonical
	// "view" → "Read", "shell" → "Bash"
	assertContains(t, out, "Read")
	assertContains(t, out, "Bash")
	// Prompt body should be in the markdown body
	assertContains(t, out, "You are a code reviewer.")
	// Filename should be slugified agent name
	assertEqual(t, "reviewer.md", result.Filename)
}

func TestCanonicalizeCodexAgents_Multi(t *testing.T) {
	input := []byte(`
[features]
multi_agent = true

[agents.planner]
model = "o3"
prompt = "You plan tasks."
tools = ["shell"]

[agents.reviewer]
model = "o4-mini"
prompt = "You review code."
tools = ["view"]
`)
	result, err := canonicalizeCodexAgents(input)
	if err != nil {
		t.Fatalf("canonicalizeCodexAgents: %v", err)
	}

	// Primary result: first agent alphabetically (planner < reviewer)
	assertEqual(t, "planner.md", result.Filename)
	assertContains(t, string(result.Content), "You plan tasks.")

	// Second agent in ExtraFiles
	reviewerContent, ok := result.ExtraFiles["reviewer.md"]
	if !ok {
		t.Fatal("expected reviewer.md in ExtraFiles")
	}
	assertContains(t, string(reviewerContent), "You review code.")
}

func TestRenderCodexAgents_Basic(t *testing.T) {
	meta := AgentMeta{
		Name:  "reviewer",
		Model: "o4-mini",
		Tools: []string{"Read", "Bash"},
	}
	body := "You are a code reviewer."

	result, err := renderCodexAgents(meta, body)
	if err != nil {
		t.Fatalf("renderCodexAgents: %v", err)
	}

	out := string(result.Content)
	assertEqual(t, "config.toml", result.Filename)
	assertContains(t, out, "[features]")
	assertContains(t, out, "multi_agent = true")
	assertContains(t, out, "[agents.reviewer]")
	assertContains(t, out, `model = "o4-mini"`)
	assertContains(t, out, "You are a code reviewer.")
	// Read → view, Bash → shell (Copilot CLI vocabulary)
	assertContains(t, out, `"view"`)
	assertContains(t, out, `"shell"`)
}

func TestRenderCodexAgents_DroppedFieldWarnings(t *testing.T) {
	meta := AgentMeta{
		Name:           "agent",
		MaxTurns:       10,
		PermissionMode: "plan",
		Skills:         []string{"some-skill"},
		MCPServers:     []string{"myserver"},
		Memory:         "project",
		Background:     true,
		Isolation:      "worktree",
		DisallowedTools: []string{"Bash"},
	}
	body := "Do something."

	result, err := renderCodexAgents(meta, body)
	if err != nil {
		t.Fatalf("renderCodexAgents: %v", err)
	}

	if len(result.Warnings) < 8 {
		t.Errorf("expected at least 8 warnings for dropped fields, got %d: %v", len(result.Warnings), result.Warnings)
	}
}

func TestCodexAgentsRoundtrip(t *testing.T) {
	// Start from canonical agent
	meta := AgentMeta{
		Name:  "planner",
		Model: "o3",
		Tools: []string{"Read", "Bash", "Write"},
	}
	body := "You are a planning agent."

	// Render to Codex TOML
	rendered, err := renderCodexAgents(meta, body)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	// Canonicalize back from TOML
	canonical, err := canonicalizeCodexAgents(rendered.Content)
	if err != nil {
		t.Fatalf("canonicalize: %v", err)
	}

	out := string(canonical.Content)
	// Name should survive
	assertContains(t, out, "planner")
	// Model should survive
	assertContains(t, out, "o3")
	// Prompt body should survive
	assertContains(t, out, "You are a planning agent.")
	// Tools: Read→view→Read, Bash→shell→Bash, Write→apply_patch→Write
	assertContains(t, out, "Read")
	assertContains(t, out, "Bash")
	assertContains(t, out, "Write")
}
```

**Command:**

```
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/converter/ -run TestCanonicalizeCodex -v
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/converter/ -run TestRenderCodex -v
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/converter/ -run TestCodexAgents -v
```

**Expected output:** All tests PASS.

---

## Task 12 — Wire Codex dispatch into AgentsConverter (converter/agents.go)

**File:** `cli/internal/converter/agents.go`

**What:** Add Codex cases to `AgentsConverter.Canonicalize` and `AgentsConverter.Render`.

**Depends on:** Tasks 9, 10.

**Code — add to `Canonicalize` method, before the `parseAgentCanonical` call:**

```go
func (c *AgentsConverter) Canonicalize(content []byte, sourceProvider string) (*Result, error) {
	if sourceProvider == "kiro" {
		return canonicalizeKiroAgent(content)
	}
	if sourceProvider == "codex" {
		return canonicalizeCodexAgents(content)
	}

	// ... rest of existing function unchanged ...
```

**Code — add to `Render` method's switch statement:**

```go
switch target.Slug {
case "gemini-cli":
    return renderGeminiAgent(meta, body)
case "copilot-cli":
    return renderCopilotAgent(meta, body)
case "roo-code":
    return renderRooCodeAgent(meta, body)
case "opencode":
    return renderOpenCodeAgent(meta, body)
case "kiro":
    return renderKiroAgent(meta, body)
case "codex":
    return renderCodexAgents(meta, body)
default:
    // Claude Code — full frontmatter preserved
    return renderClaudeAgent(meta, body)
}
```

**Command:**

```
cd /home/hhewett/.local/src/syllago/cli && go build ./internal/converter/...
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/converter/ -v
```

**Expected output:** Build succeeds. All converter tests pass.

---

## Task 13 — Integration and roundtrip tests (converter/agents_test.go)

**File:** `cli/internal/converter/agents_test.go`

**What:** Add integration tests that exercise the full `AgentsConverter` interface for Codex (via `Canonicalize` and `Render`), and a cross-provider roundtrip test (Claude → Codex → Claude).

**Depends on:** Task 12.

**Code — append to `cli/internal/converter/agents_test.go`:**

```go
func TestAgentsConverter_CodexCanonicalize(t *testing.T) {
	input := []byte(`
[features]
multi_agent = true

[agents.tester]
model = "o4-mini"
prompt = "You write tests."
tools = ["shell", "view", "apply_patch"]
`)
	conv := &AgentsConverter{}
	result, err := conv.Canonicalize(input, "codex")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "name: tester")
	assertContains(t, out, "model: o4-mini")
	assertContains(t, out, "You write tests.")
	// shell → Bash, view → Read, apply_patch → Write
	assertContains(t, out, "Bash")
	assertContains(t, out, "Read")
	assertContains(t, out, "Write")
}

func TestAgentsConverter_CodexRender(t *testing.T) {
	// Build canonical agent
	canonical := []byte("---\nname: planner\nmodel: o3\ntools:\n  - Read\n  - Bash\n---\nYou plan.\n")

	conv := &AgentsConverter{}
	result, err := conv.Render(canonical, provider.Codex)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertEqual(t, "config.toml", result.Filename)
	assertContains(t, out, "[agents.planner]")
	assertContains(t, out, "multi_agent = true")
	assertContains(t, out, `"view"`)
	assertContains(t, out, `"shell"`)
}

func TestAgentsConverter_ClaudeToCodexToClaudeRoundtrip(t *testing.T) {
	original := []byte("---\nname: reviewer\nmodel: gpt-4o\ntools:\n  - Read\n  - Bash\n  - Write\n---\nYou review PRs.\n")

	conv := &AgentsConverter{}

	// Claude → Codex
	codexResult, err := conv.Render(original, provider.Codex)
	if err != nil {
		t.Fatalf("Render to Codex: %v", err)
	}

	// Codex → canonical
	backResult, err := conv.Canonicalize(codexResult.Content, "codex")
	if err != nil {
		t.Fatalf("Canonicalize from Codex: %v", err)
	}

	// Canonical → Claude
	claudeResult, err := conv.Render(backResult.Content, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render to Claude: %v", err)
	}

	final := string(claudeResult.Content)
	assertContains(t, final, "reviewer")
	assertContains(t, final, "gpt-4o")
	assertContains(t, final, "You review PRs.")
	// Tool names should be canonical in Claude output
	assertContains(t, final, "Read")
	assertContains(t, final, "Bash")
	assertContains(t, final, "Write")
}
```

**Command:**

```
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/converter/ -run TestAgentsConverter_Codex -v
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/converter/ -run TestAgentsConverter_ClaudeToCodexToClaudeRoundtrip -v
```

**Expected output:** All tests PASS.

---

## Task 14 — Full test suite verification

**What:** Run the complete test suite to verify no regressions from any task in this plan.

**Depends on:** All previous tasks.

**Command:**

```
cd /home/hhewett/.local/src/syllago/cli && make test
```

**Expected output:**

```
ok  	github.com/OpenScribbler/syllago/cli/internal/converter	X.XXXs
ok  	github.com/OpenScribbler/syllago/cli/internal/installer	X.XXXs
ok  	github.com/OpenScribbler/syllago/cli/internal/provider	X.XXXs
...
```

All packages green. No failures.

---

## Dependency Chain

```
Task 1 (toolmap data)
  └→ Task 2 (toolmap tests)
  └→ Task 3 (MCP tool names)
       └→ Task 4 (MCP tool name tests)

Task 5 (hook warnings)
  └→ Task 6 (hook warning tests)

Task 7 (installer path expansion)
  └→ Task 8 (installer tests)

Task 9 (Codex provider update)
  └→ Task 10 (codex_agents.go)
       └→ Task 11 (codex_agents_test.go)
       └→ Task 12 (agents.go dispatch)
            └→ Task 13 (integration tests)

Task 14 depends on all tasks
```

Tasks 1-4, 5-6, 7-8, and 9-13 are four independent workstreams that can be executed in parallel if desired.

---

## Files Modified

| File | Change |
|------|--------|
| `cli/internal/converter/toolmap.go` | Add 4 providers to `ToolNames`; expand `parseMCPToolName` and `TranslateMCPToolName` |
| `cli/internal/converter/toolmap_test.go` | Add forward/reverse tool tests and MCP name tests for new providers |
| `cli/internal/converter/hooks.go` | Add `hooklessProviders` map; early-return in `Render` with warning |
| `cli/internal/converter/hooks_test.go` | Add `TestHooksToHooklessProviders` |
| `cli/internal/converter/codex_agents.go` | New — TOML bidirectional converter |
| `cli/internal/converter/codex_agents_test.go` | New — TOML roundtrip tests |
| `cli/internal/converter/agents.go` | Add `codex` cases to `Canonicalize` and `Render` |
| `cli/internal/converter/agents_test.go` | Add integration and roundtrip tests for Codex |
| `cli/internal/installer/mcp.go` | Extend `mcpConfigPath` signature; add 6 new provider paths; add `mcpConfigKey` helper |
| `cli/internal/installer/mcp_test.go` | Update override signature; add path and key tests |
| `cli/internal/provider/codex.go` | Add `Agents` to `SupportsType`, `InstallDir`, `DiscoveryPaths`, `FileFormat` |

**Not modified (already complete):**

| File | Reason |
|------|--------|
| `cli/go.mod` | `pelletier/go-toml/v2 v2.2.4` already present |
