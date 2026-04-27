# Cline Provider Audit

Audit of syllago's Cline implementation against provider research docs.
Research files: `docs/providers/cline/{tools,content-types,hooks,skills-agents}.md`

## Inaccuracies

### 1. `Edit` tool mapped to `apply_diff` -- should be `replace_in_file`

**File:** `cli/internal/converter/toolmap.go:33`
**Severity:** High -- tool name translation is wrong for current Cline versions.

The toolmap maps canonical `Edit` to Cline's `apply_diff`. Research confirms Cline renamed this tool to `replace_in_file`. The `apply_diff` name is legacy (used by some forks like Roo Code, but not current Cline).

From `docs/providers/cline/tools.md` line 64-65:
> Earlier Cline versions used a tool called `apply_diff`. The current tool is `replace_in_file`.

**Fix:** Change `"cline": "apply_diff"` to `"cline": "replace_in_file"` in the `Edit` entry. Update the corresponding test expectations in `toolmap_test.go` lines 42 and 149.

### 2. Cline listed as hookless provider -- Cline has hooks since v3.36

**File:** `cli/internal/converter/hooks.go:207`
**Severity:** High -- blocks all hook conversion to/from Cline.

Cline is in the `hooklessProviders` map, causing hook exports to emit a warning instead of content. Research shows Cline supports 8 hook events as of v3.36 (TaskStart, TaskResume, TaskCancel, TaskComplete, PreToolUse, PostToolUse, UserPromptSubmit, PreCompact).

**Fix:** Remove `"cline": true` from `hooklessProviders`. Add Cline entries to `HookEvents` in `toolmap.go` for the events that map to canonical names.

### 3. Provider definition missing Hooks support

**File:** `cli/internal/provider/cline.go:48-55`
**Severity:** High -- `SupportsType` returns false for `catalog.Hooks`.

The Cline provider only declares support for `Rules` and `MCP`. It should also support `Hooks`. The `InstallDir` function also lacks a `catalog.Hooks` case -- Cline hooks are file-based executables in `.clinerules/hooks/`, not JSON merge.

**Fix:** Add `catalog.Hooks` to `SupportsType` and `InstallDir`. Hooks should return a path like `filepath.Join(projectRoot, ".clinerules", "hooks")` (project-scoped, file-based).

## Gaps

### 4. No hook event mappings for Cline in HookEvents table

**File:** `cli/internal/converter/toolmap.go:76-87`
**Severity:** Medium -- even after removing from hookless list, no event name translations exist.

Cline's hook events use the same names as some canonical events (PreToolUse, PostToolUse, UserPromptSubmit, PreCompact) but also has unique ones (TaskStart, TaskResume, TaskCancel, TaskComplete). The `HookEvents` map has no `"cline"` entries at all.

Proposed mappings to add:

| Canonical | Cline |
|-----------|-------|
| PreToolUse | PreToolUse |
| PostToolUse | PostToolUse |
| UserPromptSubmit | UserPromptSubmit |
| PreCompact | PreCompact |
| SessionStart | TaskStart |
| Stop | TaskComplete |

Unmapped Cline events (no canonical equivalent): `TaskResume`, `TaskCancel`.

### 5. Three Cline tools not in toolmap: `browser_action`, `use_mcp_tool`, `access_mcp_resource`

**File:** `cli/internal/converter/toolmap.go`
**Severity:** Low -- these are Cline-specific tools without direct canonical equivalents.

- `browser_action` -- Puppeteer browser automation (no canonical equivalent yet)
- `use_mcp_tool` / `access_mcp_resource` -- MCP dispatch tools (handled separately by MCP converter, not tool-name translation)

The MCP tools are likely fine unmapped since MCP tool invocation is handled by the MCP converter, not the toolmap. `browser_action` could be mapped if a canonical `Browser` tool is added later.

### 6. `list_code_definition_names` not in toolmap

**File:** `cli/internal/converter/toolmap.go`
**Severity:** Low -- Cline-specific tool with no canonical equivalent.

Cline's tree-sitter-based code definition listing tool has no analog in Claude Code or other providers. No action needed unless a canonical `ListDefinitions` tool is added.

### 7. Global rules path not in provider definition

**File:** `cli/internal/provider/cline.go`
**Severity:** Low -- `DiscoveryPaths` only covers workspace `.clinerules/`, not global `~/Documents/Cline/Rules/`.

Research shows Cline has global rules at `~/Documents/Cline/Rules/` (all platforms). This path isn't included in discovery. This may be intentional (syllago may only handle project-scoped content), but worth noting.

### 8. Cline hooks use file-based discovery (named executables), not JSON config

**File:** N/A -- no Cline hook converter exists yet.
**Severity:** Medium -- implementing hook support requires a new discovery/install pattern.

Cline hooks are named executables (e.g., `PreToolUse`, `PostToolUse.ps1`) in `.clinerules/hooks/`, not JSON config. This is different from Claude Code's JSON-based hook config. The converter would need to handle:
- Translating canonical JSON hook definitions to executable scripts
- Platform-specific naming (extensionless on Unix, `.ps1` on Windows)
- Enable/disable via file permissions (`chmod +x`)

### 9. MCP settings file path not platform-aware in provider

**File:** `cli/internal/provider/cline.go`
**Severity:** Low -- MCP uses `JSONMergeSentinel` but the actual target path for Cline's MCP settings varies by platform:
- macOS: `~/Library/Application Support/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json`
- Linux: `~/.config/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json`
- Windows: `%APPDATA%/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json`

The installer would need platform detection to find the correct path for JSON merge.

## Opportunities

### 10. Conditional rules (YAML frontmatter) are partially supported

The rules converter at `cli/internal/converter/rules.go:232-262` already handles Cline's `clineFrontmatter` with `paths` field for conditional activation. This is good -- it means path-based rule activation survives round-trips.

### 11. Cline's cross-provider rule detection could inform import

Research shows Cline auto-detects `.cursorrules`, `.windsurfrules`, and `AGENTS.md`. Syllago could leverage this knowledge to suggest Cline as a target when users have content from these providers.

### 12. `new_task` tool enables context handoff -- potential loadout use case

Cline's `new_task` tool with structured context blocks could be used for loadout application workflows where context needs to carry across sessions.
