# Adding a Provider to Syllago

Step-by-step guide for adding a new AI coding tool as a supported provider. Based on the process used to add Amp (March 2026).

---

## Prerequisites

Before starting, research and document the provider's format for each content type:

- **Rules:** File format, frontmatter fields, file locations (project + global)
- **Skills:** SKILL.md support, frontmatter fields, install directories
- **Commands:** File format, argument placeholders, frontmatter fields
- **Agents:** Structured format or instruction-only, frontmatter fields
- **MCP:** Top-level key, per-server fields, transport types, config file location
- **Hooks:** Event names, configuration format, hook features
- **Detection:** How to detect if the provider is installed

Create a format reference doc at `docs/provider-formats/<slug>.md` using existing docs as templates.

---

## Step 1: Provider Definition

**File:** `cli/internal/provider/<slug>.go`

Create a `Provider` struct with all required fields:

```go
var MyProvider = Provider{
    Name:      "My Provider",
    Slug:      "my-provider",           // Stable identifier, used in CLI flags and switch cases
    ConfigDir: ".my-provider",           // Config directory name (relative to home)
    InstallDir: func(homeDir string, ct catalog.ContentType) string {
        // Return directory path for each content type
        // Return JSONMergeSentinel for JSON-merge types (MCP, Hooks)
        // Return "" for unsupported types
    },
    Detect: func(homeDir string) bool {
        // Check for config directory or binary
    },
    DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
        // Return paths to scan for existing content
    },
    FileFormat: func(ct catalog.ContentType) Format {
        // FormatMarkdown, FormatJSON, FormatYAML, FormatMDC, FormatTOML, FormatJSONC
    },
    EmitPath: func(projectRoot string) string {
        // Where syllago writes scan output for this provider
    },
    SupportsType: func(ct catalog.ContentType) bool {
        // Which content types this provider handles
    },
    SymlinkSupport: map[catalog.ContentType]bool{
        // true for filesystem types, false for JSON-merge types
    },
}
```

**Register:** Add the provider to `AllProviders` in `provider.go`:

```go
var AllProviders = []Provider{
    // ... existing providers ...
    MyProvider,
}
```

---

## Step 2: Converter Paths

For each supported content type, add canonicalize and/or render functions.

### Files to Modify

| Content Type | File |
|-------------|------|
| Rules | `cli/internal/converter/rules.go` |
| Skills | `cli/internal/converter/skills.go` |
| Commands | `cli/internal/converter/commands.go` |
| Agents | `cli/internal/converter/agents.go` |
| MCP | `cli/internal/converter/mcp.go` |
| Hooks | `cli/internal/converter/hooks.go` |

### Pattern

Each converter has `Canonicalize(content, sourceProvider)` and `Render(content, targetProvider)` methods.

**Canonicalize** (provider format -> canonical):
```go
// In Canonicalize() switch:
case "my-provider":
    return canonicalizeMyProviderRule(content)
```

**Render** (canonical -> provider format):
```go
// In Render() switch:
case "my-provider":
    return renderMyProviderRule(meta, body)
```

### Common Patterns

**Provider supports full canonical fields:** Use `renderClaudeSkill` pattern (emit all frontmatter).

**Provider supports subset of fields:** Create a provider-specific meta struct with only supported fields. Embed unsupported fields as prose notes via `BuildConversionNotes` / `AppendNotes`. See `renderGeminiSkill`, `renderCursorSkill`, `renderAmpSkill`.

**Provider uses different field names:** Parse provider-specific struct, map to canonical fields. See `canonicalizeOpenCodeAgent` (steps -> maxTurns), `canonicalizeWindsurfMCP` (serverUrl -> url).

**Provider uses different format (TOML, JSONC, etc.):** Use appropriate unmarshaler. See `canonicalizeGeminiCommand` (TOML), `canonicalizeOpencodeMCP` (JSONC).

**Provider uses different top-level key:** Create wrapper struct. See `ampMCPSettings` (amp.mcpServers), `vscodeMCPConfig` (servers), `opencodeMCPConfig` (mcp).

---

## Step 3: Tool Name Translation

If the provider uses different tool names, add mappings to `cli/internal/converter/toolmap.go`.

---

## Step 4: Hook Capabilities (if applicable)

If the provider supports hooks, add entries to:
- `HookCapabilities` in `cli/internal/converter/compat.go`
- `HookOutputCapabilities` in the same file
- `HookProviders()` function
- `hookConfigHints` and `hookScopingNotes` in `cli/internal/converter/skills.go`

---

## Step 5: Build and Verify

```bash
cd cli
make fmt              # Format code
make build            # Build binary
make test             # Run tests
golangci-lint run ./... # Lint check (CI enforces this)
```

### What to Check

1. **Provider appears in list:** `syllago providers`
2. **Detection works:** Provider shows as detected when config dir exists
3. **Import/export works:** Content round-trips through canonical format
4. **TUI shows provider:** Provider cards appear in the TUI
5. **Inspect output:** `syllago inspect` shows the provider

### TUI Golden Files

If TUI visual output changes (provider list, cards, etc.), regenerate golden files:
```bash
cd cli && go test ./internal/tui/ -update-golden
```

---

## Step 6: Documentation

1. **Format reference:** `docs/provider-formats/<slug>.md`
2. **Conversion reference:** Update `docs/cross-provider-conversion-reference.md` if it exists
3. **README:** Update supported providers list if applicable

---

## Checklist

- [ ] Research provider formats for all content types
- [ ] Create `docs/provider-formats/<slug>.md`
- [ ] Create `cli/internal/provider/<slug>.go`
- [ ] Add to `AllProviders` in `provider.go`
- [ ] Add converter paths for each supported content type
- [ ] Add tool name translations (if different tool names)
- [ ] Add hook capabilities (if provider supports hooks)
- [ ] `make fmt && make build && make test && golangci-lint run ./...`
- [ ] Verify CLI: `syllago providers`, `syllago inspect`
- [ ] Update TUI golden files if visual changes
- [ ] Update documentation
