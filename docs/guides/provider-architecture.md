# Provider Architecture Reference

Reference document for adding new providers to syllago. Derived from codebase analysis of the existing 6-provider implementation.

---

## Provider Struct

**Location:** `cli/internal/provider/provider.go`

```go
type Provider struct {
    Name      string                                           // Display name: "Claude Code"
    Slug      string                                           // Stable ID: "claude-code" (matches dir names)
    Detected  bool                                             // Detection status (populated at runtime)
    ConfigDir string                                           // Home config dir: "~/.claude"

    InstallDir     func(homeDir string, ct catalog.ContentType) string
    Detect         func(homeDir string) bool
    DiscoveryPaths func(projectRoot string, ct catalog.ContentType) []string
    FileFormat     func(ct catalog.ContentType) provider.Format
    EmitPath       func(projectRoot string) string
    SupportsType   func(ct catalog.ContentType) bool
}
```

**Constants:**
- `JSONMergeSentinel = "__json_merge__"` — indicates content needs JSON merge (MCP, Hooks) rather than filesystem placement
- Format types: `FormatMarkdown`, `FormatMDC` (Cursor), `FormatJSON`, `FormatYAML`

---

## Existing Provider Support Matrix

| Type | Claude Code | Gemini CLI | Cursor | Windsurf | Codex | Copilot CLI |
|------|:-:|:-:|:-:|:-:|:-:|:-:|
| Rules | Y | Y | Y | Y | Y | Y |
| Skills | Y | Y | - | - | - | - |
| Agents | Y | Y | - | - | - | Y |
| Commands | Y | Y | - | - | Y | Y |
| MCP | Y | Y | - | - | - | Y |
| Hooks | Y | Y | - | - | - | Y |

---

## Adding a New Provider — Checklist

### 1. Create provider file

**File:** `cli/internal/provider/<slug>.go`

Define a module-level `var` with all 7 callback functions:
- `InstallDir` — return target path per content type, `""` for unsupported, `JSONMergeSentinel` for JSON merge types
- `Detect` — check config dir or command availability
- `DiscoveryPaths` — where to scan in a project for this provider's content
- `FileFormat` — format per content type
- `EmitPath` — main rules/config export path
- `SupportsType` — boolean per content type

### 2. Register in AllProviders

Add to `AllProviders` slice in `provider.go`.

### 3. Update converters (if needed)

**Location:** `cli/internal/converter/`

For each supported content type:
- If provider uses standard markdown → existing converters may work as-is
- If provider has unique format → add `case "slug":` in `Canonicalize()` and/or `Render()`
- If provider has unique MCP format → add merge logic in `mcp.go`

### 4. Update tool/event translation maps

**Location:** `cli/internal/converter/toolmap.go`

Add entries for:
- `ToolNames` — canonical (Claude Code) tool names → provider equivalents
- `HookEvents` — canonical hook event names → provider equivalents
- `MCPToolFormat` — MCP tool name format for the provider

### 5. Add tests

Provider-specific converter tests for any unique format handling.

---

## Converter Interface

```go
type Converter interface {
    Canonicalize(content []byte, sourceProvider string) (*Result, error)
    Render(content []byte, target provider.Provider) (*Result, error)
    ContentType() catalog.ContentType
}
```

**Result:**
```go
type Result struct {
    Content    []byte            // Transformed bytes
    Filename   string            // Output filename
    Warnings   []string          // Data loss warnings
    ExtraFiles map[string][]byte // Additional generated files
}
```

---

## Content Types

**Location:** `cli/internal/catalog/types.go`

| Type | Universal? | Description |
|------|-----------|-------------|
| Skills | Yes | Reusable skill definitions |
| Agents | Yes | Agent configurations |
| Prompts | Yes | Prompt templates |
| MCP | Yes | MCP server configurations |
| Apps | Yes | Applications |
| Rules | No | Provider-specific rules/instructions |
| Hooks | No | Provider-specific hooks |
| Commands | No | Provider-specific commands |

---

## Key Files

| File | Purpose |
|------|---------|
| `cli/internal/provider/provider.go` | Provider interface & AllProviders registry |
| `cli/internal/provider/{slug}.go` | Individual provider definitions |
| `cli/internal/converter/converter.go` | Converter registry & interface |
| `cli/internal/converter/{type}.go` | Per-content-type converters |
| `cli/internal/converter/toolmap.go` | Tool/event translation maps |
| `cli/internal/installer/installer.go` | Installation & status checking |
| `cli/internal/catalog/types.go` | ContentType definitions |
| `cli/internal/catalog/scanner.go` | Discovery scanning logic |
