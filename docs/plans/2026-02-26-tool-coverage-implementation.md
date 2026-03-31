# Tool Coverage Expansion — Implementation Plan

*Feature: tool-coverage*
*Design doc: docs/plans/2026-02-26-tool-coverage-design.md*
*Date: 2026-02-26*

Adds 5 new providers: Zed, Cline, Roo Code, OpenCode, Kiro. Target: 11 providers total.

---

## Phase 1: Infrastructure

These tasks must be done first. All later phases depend on them.

---

### Task 1.1 — Add FormatJSONC constant to provider.go

**Dependencies:** None

**Files to modify:**
- `cli/internal/provider/provider.go`

**What to implement:**

Add `FormatJSONC` alongside the existing format constants:

```go
const (
    FormatMarkdown Format = "md"
    FormatMDC      Format = "mdc"
    FormatJSON     Format = "json"
    FormatYAML     Format = "yaml"
    FormatJSONC    Format = "jsonc"
)
```

**Why JSONC is a first-class constant:** OpenCode uses JSONC for its main config file (`opencode.json`). The installer and scanner need to recognize this format distinctly so they can apply comment-stripping before unmarshaling. Without a named constant, every place that handles OpenCode files would need to hard-code the string `"jsonc"`.

**How to verify:** `make build` compiles without error.

---

### Task 1.2 — Create JSONC utility file

**Dependencies:** Task 1.1

**Files to create:**
- `cli/internal/converter/jsonc.go`
- `cli/internal/converter/jsonc_test.go`

**What to implement in `jsonc.go`:**

```go
package converter

import (
    "bytes"
    "strings"
)

// StripJSONCComments removes // line comments and /* */ block comments from
// JSON-with-comments (JSONC) input. The resulting bytes are valid JSON that can
// be passed to json.Unmarshal.
//
// Limitations: does not handle comments inside string values (e.g., a JSON string
// containing "//foo" would have "//foo" incorrectly stripped). This is acceptable
// for syllago's use case because OpenCode config files use comments only in structural
// positions, not inside string values.
func StripJSONCComments(src []byte) []byte {
    var buf bytes.Buffer
    i := 0
    inString := false
    for i < len(src) {
        c := src[i]

        // Track string boundaries to avoid stripping inside strings
        if c == '"' && (i == 0 || src[i-1] != '\\') {
            inString = !inString
            buf.WriteByte(c)
            i++
            continue
        }

        if inString {
            buf.WriteByte(c)
            i++
            continue
        }

        // Line comment: // ...
        if c == '/' && i+1 < len(src) && src[i+1] == '/' {
            for i < len(src) && src[i] != '\n' {
                i++
            }
            continue
        }

        // Block comment: /* ... */
        if c == '/' && i+1 < len(src) && src[i+1] == '*' {
            i += 2
            for i+1 < len(src) {
                if src[i] == '*' && src[i+1] == '/' {
                    i += 2
                    break
                }
                i++
            }
            continue
        }

        buf.WriteByte(c)
        i++
    }
    return bytes.TrimSpace(buf.Bytes())
}

// ParseJSONC strips comments from src then unmarshals into v using the standard
// encoding/json rules. Returns the same errors as json.Unmarshal.
func ParseJSONC(src []byte, v any) error {
    stripped := StripJSONCComments(src)
    return json.Unmarshal(stripped, v)
}
```

Add the `encoding/json` import.

**What to implement in `jsonc_test.go`:**

```go
package converter

import (
    "encoding/json"
    "testing"
)

func TestStripJSONCLineComments(t *testing.T) {
    input := []byte(`{
  // this is a comment
  "key": "value"
}`)
    stripped := StripJSONCComments(input)
    var m map[string]string
    if err := json.Unmarshal(stripped, &m); err != nil {
        t.Fatalf("unmarshal after strip: %v", err)
    }
    assertEqual(t, "value", m["key"])
}

func TestStripJSONCBlockComments(t *testing.T) {
    input := []byte(`{
  /* block comment */ "key": "value"
}`)
    stripped := StripJSONCComments(input)
    var m map[string]string
    if err := json.Unmarshal(stripped, &m); err != nil {
        t.Fatalf("unmarshal after strip: %v", err)
    }
    assertEqual(t, "value", m["key"])
}

func TestStripJSONCMixedComments(t *testing.T) {
    input := []byte(`{
  // schema comment
  "$schema": "https://opencode.ai/config.json",
  /* mcp section */
  "mcp": {
    "server": {
      "type": "local", // local = stdio
      "command": ["npx", "-y", "pkg"]
    }
  }
}`)
    stripped := StripJSONCComments(input)
    var m map[string]any
    if err := json.Unmarshal(stripped, &m); err != nil {
        t.Fatalf("unmarshal after strip: %v", err)
    }
    if _, ok := m["mcp"]; !ok {
        t.Fatal("expected mcp key to survive stripping")
    }
}

func TestStripJSONCPreservesStrings(t *testing.T) {
    // A string value that looks like a comment should not be stripped
    input := []byte(`{"url": "https://example.com/path"}`)
    stripped := StripJSONCComments(input)
    var m map[string]string
    if err := json.Unmarshal(stripped, &m); err != nil {
        t.Fatalf("unmarshal: %v", err)
    }
    assertEqual(t, "https://example.com/path", m["url"])
}
```

**How to verify:** `make test` passes. Specifically `go test ./cli/internal/converter/... -run TestStripJSONC`.

---

### Task 1.3 — Extend mcpServerConfig with OpenCode fields

**Dependencies:** Task 1.2

**Files to modify:**
- `cli/internal/converter/mcp.go`

**What to implement:**

Add OpenCode-specific fields to `mcpServerConfig`. Add them after the Gemini alternate field name block, with a clear comment:

```go
type mcpServerConfig struct {
    // Universal fields
    Command string            `json:"command,omitempty"`
    Args    []string          `json:"args,omitempty"`
    Env     map[string]string `json:"env,omitempty"`
    Cwd     string            `json:"cwd,omitempty"`

    // HTTP transport
    URL     string            `json:"url,omitempty"`
    Headers map[string]string `json:"headers,omitempty"`
    Type    string            `json:"type,omitempty"`

    // Provider-specific (preserved in canonical for round-trips)
    Trust        string   `json:"trust,omitempty"`
    IncludeTools []string `json:"includeTools,omitempty"`
    ExcludeTools []string `json:"excludeTools,omitempty"`
    Disabled     bool     `json:"disabled,omitempty"`
    AutoApprove  []string `json:"autoApprove,omitempty"`

    // Gemini alternate field names
    HTTPUrl string `json:"httpUrl,omitempty"`

    // OpenCode-specific (preserved in canonical for round-trips)
    Environment  map[string]string `json:"environment,omitempty"` // OpenCode uses "environment" not "env"
    CommandArray []string          `json:"commandArray,omitempty"` // OpenCode command as array (normalized from command[])
    Enabled      *bool             `json:"enabled,omitempty"`      // OpenCode uses "enabled" (true by default) not "disabled"
    Timeout      int               `json:"timeout,omitempty"`      // OpenCode timeout in ms
    OAuth        json.RawMessage   `json:"oauth,omitempty"`        // OpenCode OAuth config (preserved opaque)
}
```

Also add a `opencodeMCPConfig` wrapper struct for OpenCode's format (which uses `"mcp"` key, not `"mcpServers"`):

```go
// opencodeMCPConfig wraps OpenCode's mcp section format.
// OpenCode uses "mcp" (not "mcpServers") as the top-level key.
type opencodeMCPConfig struct {
    MCP map[string]opencodeServerConfig `json:"mcp"`
}

// opencodeServerConfig is OpenCode's on-disk format for a single MCP server.
// Differs from canonical in: command is an array, env key is "environment",
// enabled flips the polarity of disabled.
type opencodeServerConfig struct {
    Type        string            `json:"type,omitempty"`        // "local" | "remote"
    Command     []string          `json:"command,omitempty"`     // array form
    Environment map[string]string `json:"environment,omitempty"` // "environment" not "env"
    Enabled     *bool             `json:"enabled,omitempty"`
    Timeout     int               `json:"timeout,omitempty"`
    URL         string            `json:"url,omitempty"`
    Headers     map[string]string `json:"headers,omitempty"`
    OAuth       json.RawMessage   `json:"oauth,omitempty"`
}
```

Add to the `Canonicalize` method a new case for `"opencode"`:

```go
case "opencode":
    return canonicalizeOpencodeMCP(content)
```

Add the `canonicalizeOpencodeMCP` function:

```go
func canonicalizeOpencodeMCP(content []byte) (*Result, error) {
    stripped := StripJSONCComments(content)
    var oc opencodeMCPConfig
    if err := json.Unmarshal(stripped, &oc); err != nil {
        return nil, fmt.Errorf("parsing OpenCode MCP config: %w", err)
    }

    out := mcpConfig{MCPServers: make(map[string]mcpServerConfig)}
    for name, s := range oc.MCP {
        cfg := mcpServerConfig{
            URL:         s.URL,
            Headers:     s.Headers,
            OAuth:       s.OAuth,
            Timeout:     s.Timeout,
            CommandArray: s.Command,
            Environment: s.Environment,
        }

        // Normalize command array: first element is command, rest are args
        if len(s.Command) > 0 {
            cfg.Command = s.Command[0]
            if len(s.Command) > 1 {
                cfg.Args = s.Command[1:]
            }
            cfg.CommandArray = nil // clear — command/args are now canonical
        }

        // Normalize environment → env
        if len(s.Environment) > 0 {
            cfg.Env = s.Environment
            cfg.Environment = nil
        }

        // Normalize enabled → disabled (flip polarity)
        // OpenCode default is enabled=true; canonical default is disabled=false
        if s.Enabled != nil && !*s.Enabled {
            cfg.Disabled = true
        }

        // Preserve type for HTTP servers
        if s.Type == "remote" {
            cfg.Type = "sse"
        }

        out.MCPServers[name] = cfg
    }

    result, err := json.MarshalIndent(out, "", "  ")
    if err != nil {
        return nil, err
    }
    return &Result{Content: result, Filename: "mcp.json"}, nil
}
```

Add `renderOpencodeMCP` to the `Render` switch:

```go
case "opencode":
    return renderOpencodeMCP(cfg)
```

Add the renderer:

```go
func renderOpencodeMCP(cfg mcpConfig) (*Result, error) {
    var warnings []string
    oc := opencodeMCPConfig{MCP: make(map[string]opencodeServerConfig)}

    for name, server := range cfg.MCPServers {
        s := opencodeServerConfig{
            URL:     server.URL,
            Headers: server.Headers,
            OAuth:   server.OAuth,
            Timeout: server.Timeout,
        }

        // Determine type
        if server.URL != "" {
            s.Type = "remote"
        } else {
            s.Type = "local"
        }

        // Reconstruct command array from command + args
        if server.Command != "" {
            s.Command = append([]string{server.Command}, server.Args...)
        }

        // Translate env → environment
        if len(server.Env) > 0 {
            s.Environment = server.Env
        }

        // Translate disabled → enabled
        if server.Disabled {
            f := false
            s.Enabled = &f
        }

        // Warn about dropped fields
        if len(server.AutoApprove) > 0 {
            warnings = append(warnings, fmt.Sprintf("server %q: autoApprove dropped (Claude-specific)", name))
        }
        if server.Trust != "" {
            warnings = append(warnings, fmt.Sprintf("server %q: trust dropped (Gemini-specific)", name))
        }

        oc.MCP[name] = s
    }

    result, err := json.MarshalIndent(oc, "", "  ")
    if err != nil {
        return nil, err
    }
    return &Result{Content: result, Filename: "opencode.json", Warnings: warnings}, nil
}
```

**How to verify:** `make test` passes. Add a targeted test in Task 5.3.

---

### Task 1.4 — Add Kiro entries to toolmap.go

**Dependencies:** None (can be done in parallel with Tasks 1.1–1.3)

**Files to modify:**
- `cli/internal/converter/toolmap.go`

**What to implement:**

Add Kiro tool name mappings to `ToolNames`. OpenCode does not have user-facing tool name references in agent definitions (it uses standard Claude tool names), so no OpenCode entries are needed in ToolNames or HookEvents. See D9 in design doc for rationale.

```go
var ToolNames = map[string]map[string]string{
    "Read":      {"gemini-cli": "read_file", "copilot-cli": "view", "kiro": "read"},
    "Write":     {"gemini-cli": "write_file", "copilot-cli": "apply_patch", "kiro": "fs_write"},
    "Edit":      {"gemini-cli": "replace", "copilot-cli": "apply_patch", "kiro": "fs_write"},
    "Bash":      {"gemini-cli": "run_shell_command", "copilot-cli": "shell", "kiro": "shell"},
    "Glob":      {"gemini-cli": "list_directory", "copilot-cli": "glob", "kiro": "read"},
    "Grep":      {"gemini-cli": "grep_search", "copilot-cli": "rg", "kiro": "read"},
    "WebSearch": {"gemini-cli": "google_search"},
    "Task":      {"copilot-cli": "task"},
}
```

Note on Kiro tool name choices:
- `Read`, `Glob`, `Grep` → `"read"` (Kiro's read tool covers filesystem reads)
- `Write`, `Edit` → `"fs_write"` (Kiro's dedicated write tool)
- `Bash` → `"shell"` (direct equivalent)

Add Kiro hook event mappings to `HookEvents`:

```go
var HookEvents = map[string]map[string]string{
    "PreToolUse":       {"gemini-cli": "BeforeTool", "copilot-cli": "preToolUse", "kiro": "preToolUse"},
    "PostToolUse":      {"gemini-cli": "AfterTool", "copilot-cli": "postToolUse", "kiro": "postToolUse"},
    "UserPromptSubmit": {"gemini-cli": "BeforeAgent", "copilot-cli": "userPromptSubmitted", "kiro": "userPromptSubmit"},
    "Stop":             {"gemini-cli": "AfterAgent", "kiro": "stop"},
    "SessionStart":     {"gemini-cli": "SessionStart", "copilot-cli": "sessionStart", "kiro": "agentSpawn"},
    "SessionEnd":       {"gemini-cli": "SessionEnd", "copilot-cli": "sessionEnd"},
    "PreCompact":       {"gemini-cli": "PreCompress"},
    "Notification":     {"gemini-cli": "Notification"},
    "SubagentStart":    {},
    "SubagentCompleted":{},
}
```

Add Kiro MCP tool name format to `TranslateMCPToolName` switch:

```go
case "kiro":
    // Kiro uses the same format as Claude Code for MCP tool names: mcp__server__tool
    return "mcp__" + server + "__" + tool
```

And in `parseMCPToolName`:

```go
case "kiro":
    // Kiro uses mcp__server__tool (same as claude-code)
    if !strings.HasPrefix(name, "mcp__") {
        return "", ""
    }
    rest := strings.TrimPrefix(name, "mcp__")
    parts := strings.SplitN(rest, "__", 2)
    if len(parts) != 2 {
        return "", ""
    }
    return parts[0], parts[1]
```

**How to verify:** `make test` passes. Existing toolmap tests still pass. New entries visible in `TranslateTool("Read", "kiro")` returning `"read"`.

---

## Phase 2: Zed Provider (Simple)

Rules (`.rules`) + MCP (`context_servers` in settings.json).

---

### Task 2.1 — Create Zed provider file

**Dependencies:** Task 1.1

**Files to create:**
- `cli/internal/provider/zed.go`

**What to implement:**

```go
package provider

import (
    "os"
    "os/exec"
    "path/filepath"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
)

var Zed = Provider{
    Name:      "Zed",
    Slug:      "zed",
    ConfigDir: ".config/zed",
    InstallDir: func(homeDir string, ct catalog.ContentType) string {
        switch ct {
        case catalog.Rules:
            // Rules go in project root as .rules; returned as empty so the caller
            // places in project root. The installer handles project-relative placement.
            return ""
        case catalog.MCP:
            // MCP merges into ~/.config/zed/settings.json
            return JSONMergeSentinel
        }
        return ""
    },
    Detect: func(homeDir string) bool {
        // Check for ~/.config/zed/ directory
        info, err := os.Stat(filepath.Join(homeDir, ".config", "zed"))
        if err == nil && info.IsDir() {
            return true
        }
        // Also check if zed command exists in PATH
        _, err = exec.LookPath("zed")
        return err == nil
    },
    DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
        switch ct {
        case catalog.Rules:
            return []string{filepath.Join(projectRoot, ".rules")}
        default:
            return nil
        }
    },
    FileFormat: func(ct catalog.ContentType) Format {
        switch ct {
        case catalog.MCP:
            return FormatJSON
        default:
            return FormatMarkdown
        }
    },
    EmitPath: func(projectRoot string) string {
        return filepath.Join(projectRoot, ".rules")
    },
    SupportsType: func(ct catalog.ContentType) bool {
        switch ct {
        case catalog.Rules, catalog.MCP:
            return true
        default:
            return false
        }
    },
}
```

**How to verify:** `make build` compiles. `make test` passes.

---

### Task 2.2 — Register Zed in AllProviders

**Dependencies:** Task 2.1

**Files to modify:**
- `cli/internal/provider/provider.go`

**What to implement:**

```go
var AllProviders = []Provider{
    ClaudeCode,
    GeminiCLI,
    Cursor,
    Windsurf,
    Codex,
    CopilotCLI,
    Zed,
}
```

**How to verify:** `make build` passes.

---

### Task 2.3 — Add Zed MCP renderer

**Dependencies:** Tasks 1.3, 2.1

**Files to modify:**
- `cli/internal/converter/mcp.go`

**What to implement:**

Zed MCP uses `context_servers` key (not `mcpServers`) and requires `"source": "custom"` on each entry. It only supports stdio transport, so HTTP servers get a warning.

Add a Zed-specific server struct:

```go
// zedContextServer is Zed's on-disk format for a single MCP server entry.
type zedContextServer struct {
    Source  string            `json:"source"`            // always "custom" for user-defined
    Command string            `json:"command,omitempty"`
    Args    []string          `json:"args,omitempty"`
    Env     map[string]string `json:"env,omitempty"`
}

// zedContextServersConfig wraps the context_servers key in settings.json.
type zedContextServersConfig struct {
    ContextServers map[string]zedContextServer `json:"context_servers"`
}
```

Add `"zed"` to the Canonicalize switch:

```go
case "zed":
    return canonicalizeZedMCP(content)
```

```go
func canonicalizeZedMCP(content []byte) (*Result, error) {
    var zc zedContextServersConfig
    if err := json.Unmarshal(content, &zc); err != nil {
        return nil, fmt.Errorf("parsing Zed context_servers: %w", err)
    }

    out := mcpConfig{MCPServers: make(map[string]mcpServerConfig)}
    for name, s := range zc.ContextServers {
        out.MCPServers[name] = mcpServerConfig{
            Command: s.Command,
            Args:    s.Args,
            Env:     s.Env,
        }
    }

    result, err := json.MarshalIndent(out, "", "  ")
    if err != nil {
        return nil, err
    }
    return &Result{Content: result, Filename: "mcp.json"}, nil
}
```

Add `"zed"` to the Render switch:

```go
case "zed":
    return renderZedMCP(cfg)
```

```go
func renderZedMCP(cfg mcpConfig) (*Result, error) {
    var warnings []string
    zc := zedContextServersConfig{ContextServers: make(map[string]zedContextServer)}

    for name, server := range cfg.MCPServers {
        s := zedContextServer{
            Source:  "custom",
            Command: server.Command,
            Args:    server.Args,
            Env:     server.Env,
        }

        // Warn: Zed only supports stdio; HTTP servers cannot be represented
        if server.URL != "" {
            warnings = append(warnings, fmt.Sprintf("server %q: HTTP transport (url: %q) is not supported by Zed; server skipped", name, server.URL))
            continue
        }

        // Warn about dropped fields
        if len(server.AutoApprove) > 0 {
            warnings = append(warnings, fmt.Sprintf("server %q: autoApprove dropped (Claude-specific)", name))
        }
        if server.Trust != "" {
            warnings = append(warnings, fmt.Sprintf("server %q: trust dropped (Gemini-specific)", name))
        }

        zc.ContextServers[name] = s
    }

    // Zed merges into settings.json; wrap in context_servers for surgical merge
    result, err := json.MarshalIndent(zc, "", "  ")
    if err != nil {
        return nil, err
    }
    return &Result{Content: result, Filename: "settings.json", Warnings: warnings}, nil
}
```

**How to verify:** `make test` passes.

---

### Task 2.3b — Add Zed rules renderer

**Dependencies:** Tasks 1.1, 2.1

**Files to modify:**
- `cli/internal/converter/rules.go`

**What to implement:**

Zed rules go to a single `.rules` file (plain markdown, no frontmatter). Add to the Render switch:

```go
case "zed":
    return renderZedRule(meta, body)
```

```go
// renderZedRule renders a rule as plain markdown for Zed's .rules file.
// Zed does not support frontmatter — the file is plain markdown.
// Scope information from alwaysApply/globs is embedded as prose if needed.
func renderZedRule(meta RuleMeta, body string) (*Result, error) {
    if meta.AlwaysApply {
        return &Result{Content: []byte(body + "\n"), Filename: ".rules"}, nil
    }

    var notes []string
    switch {
    case len(meta.Globs) > 0:
        notes = append(notes, fmt.Sprintf("**Scope:** Apply only when working with files matching: %s", strings.Join(meta.Globs, ", ")))
    case meta.Description != "":
        notes = append(notes, fmt.Sprintf("**Scope:** Apply when: %s", meta.Description))
    default:
        notes = append(notes, "**Scope:** Apply only when explicitly asked.")
    }

    notesBlock := BuildConversionNotes("syllago", notes)
    result := AppendNotes(body, notesBlock)
    return &Result{Content: []byte(result + "\n"), Filename: ".rules"}, nil
}
```

Also handle `"zed"` in Canonicalize — `.rules` is plain markdown:

```go
case "zed":
    return canonicalizeMarkdownRule(content)
```

Also add Zed rule test cases to `rules_test.go`:

```go
func TestZedRuleRender(t *testing.T) {
    input := []byte("---\nalwaysApply: true\n---\n\nAlways follow these guidelines.\n")
    conv := &RulesConverter{}
    canonical, err := conv.Canonicalize(input, "claude-code")
    if err != nil {
        t.Fatalf("Canonicalize: %v", err)
    }
    result, err := conv.Render(canonical.Content, provider.Zed)
    if err != nil {
        t.Fatalf("Render: %v", err)
    }
    assertContains(t, string(result.Content), "Always follow")
    assertEqual(t, ".rules", result.Filename)
    assertNotContains(t, string(result.Content), "---") // no frontmatter
}
```

**How to verify:** `go test ./cli/internal/converter/... -run TestZedRule` passes.

---

### Task 2.4 — Write Zed MCP tests

**Dependencies:** Task 2.3

**Files to modify:**
- `cli/internal/converter/mcp_test.go`

**What to implement:**

```go
func TestZedMCPRender(t *testing.T) {
    input := []byte(`{
        "mcpServers": {
            "filesystem": {
                "command": "npx",
                "args": ["-y", "@modelcontextprotocol/server-filesystem", "/home"],
                "env": {"DEBUG": "1"}
            }
        }
    }`)

    conv := &MCPConverter{}
    canonical, err := conv.Canonicalize(input, "claude-code")
    if err != nil {
        t.Fatalf("Canonicalize: %v", err)
    }

    result, err := conv.Render(canonical.Content, provider.Zed)
    if err != nil {
        t.Fatalf("Render: %v", err)
    }

    out := string(result.Content)
    assertContains(t, out, "context_servers")
    assertContains(t, out, `"source": "custom"`)
    assertContains(t, out, "npx")
    assertContains(t, out, "DEBUG")
    assertNotContains(t, out, "mcpServers")
    assertEqual(t, "settings.json", result.Filename)
    if len(result.Warnings) > 0 {
        t.Fatalf("expected no warnings for stdio server, got: %v", result.Warnings)
    }
}

func TestZedMCPHTTPServerDropped(t *testing.T) {
    input := []byte(`{
        "mcpServers": {
            "remote": {
                "url": "https://mcp.example.com/sse",
                "type": "sse"
            }
        }
    }`)

    conv := &MCPConverter{}
    canonical, err := conv.Canonicalize(input, "claude-code")
    if err != nil {
        t.Fatalf("Canonicalize: %v", err)
    }

    result, err := conv.Render(canonical.Content, provider.Zed)
    if err != nil {
        t.Fatalf("Render: %v", err)
    }

    if len(result.Warnings) == 0 {
        t.Fatal("expected warning about HTTP server not supported by Zed")
    }
    assertNotContains(t, string(result.Content), "mcp.example.com")
}

func TestZedMCPCanonicalize(t *testing.T) {
    input := []byte(`{
        "context_servers": {
            "my-server": {
                "source": "custom",
                "command": "node",
                "args": ["server.js"],
                "env": {"PORT": "3000"}
            }
        }
    }`)

    conv := &MCPConverter{}
    result, err := conv.Canonicalize(input, "zed")
    if err != nil {
        t.Fatalf("Canonicalize: %v", err)
    }

    out := string(result.Content)
    assertContains(t, out, "mcpServers")
    assertContains(t, out, "node")
    assertContains(t, out, "PORT")
    assertNotContains(t, out, "context_servers")
    assertNotContains(t, out, `"source"`)
}
```

**How to verify:** `go test ./cli/internal/converter/... -run TestZed` passes.

---

### Task 2.5 — Write Zed provider test file

**Dependencies:** Task 2.1

**Files to create:**
- `cli/internal/provider/zed_test.go`

**What to implement:**

```go
package provider

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestZedDetect(t *testing.T) {
    dir := t.TempDir()
    if Zed.Detect(dir) {
        t.Fatal("expected no detection in empty temp dir")
    }

    if err := os.MkdirAll(filepath.Join(dir, ".config", "zed"), 0755); err != nil {
        t.Fatal(err)
    }
    if !Zed.Detect(dir) {
        t.Fatal("expected detection when ~/.config/zed/ exists")
    }
}

func TestZedSupportsType(t *testing.T) {
    if !Zed.SupportsType(catalog.Rules) {
        t.Error("Zed must support Rules")
    }
    if !Zed.SupportsType(catalog.MCP) {
        t.Error("Zed must support MCP")
    }
    if Zed.SupportsType(catalog.Agents) {
        t.Error("Zed must not support Agents")
    }
    if Zed.SupportsType(catalog.Hooks) {
        t.Error("Zed must not support Hooks")
    }
    if Zed.SupportsType(catalog.Skills) {
        t.Error("Zed must not support Skills")
    }
    if Zed.SupportsType(catalog.Commands) {
        t.Error("Zed must not support Commands")
    }
}

func TestZedDiscoveryPaths(t *testing.T) {
    paths := Zed.DiscoveryPaths("/project", catalog.Rules)
    if len(paths) != 1 {
        t.Fatalf("expected 1 discovery path, got %d", len(paths))
    }
    if paths[0] != "/project/.rules" {
        t.Errorf("expected /project/.rules, got %q", paths[0])
    }

    paths = Zed.DiscoveryPaths("/project", catalog.Agents)
    if len(paths) != 0 {
        t.Errorf("expected no agent paths, got %v", paths)
    }
}

func TestZedInstallDir(t *testing.T) {
    rulesDir := Zed.InstallDir("/home/user", catalog.Rules)
    if rulesDir != "" {
        // Rules for Zed go in project root via EmitPath, not a home dir
        t.Errorf("expected empty string for Rules InstallDir, got %q", rulesDir)
    }
    mcpDir := Zed.InstallDir("/home/user", catalog.MCP)
    if mcpDir != JSONMergeSentinel {
        t.Errorf("expected JSONMergeSentinel for MCP, got %q", mcpDir)
    }
}

func TestZedEmitPath(t *testing.T) {
    path := Zed.EmitPath("/my/project")
    if path != "/my/project/.rules" {
        t.Errorf("expected /my/project/.rules, got %q", path)
    }
}
```

**How to verify:** `go test ./cli/internal/provider/... -run TestZed` passes.

---

## Phase 3: Cline Provider (Simple)

Rules (`.clinerules/` directory) + MCP (global JSON merge).

---

### Task 3.1 — Create Cline provider file

**Dependencies:** Task 1.1

**Files to create:**
- `cli/internal/provider/cline.go`

**What to implement:**

```go
package provider

import (
    "os"
    "path/filepath"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// clineGlobalStoragePath is the Linux path to Cline's VS Code globalStorage directory.
// This is where cline_mcp_settings.json lives.
const clineGlobalStoragePath = ".config/Code/User/globalStorage/saoudrizwan.claude-dev/settings"

var Cline = Provider{
    Name:      "Cline",
    Slug:      "cline",
    ConfigDir: clineGlobalStoragePath,
    InstallDir: func(homeDir string, ct catalog.ContentType) string {
        switch ct {
        case catalog.Rules:
            // Cline rules install into .clinerules/ in the project root.
            // Empty string here means caller places relative to project root.
            return ""
        case catalog.MCP:
            return JSONMergeSentinel
        }
        return ""
    },
    Detect: func(homeDir string) bool {
        // Detect via globalStorage directory
        info, err := os.Stat(filepath.Join(homeDir, clineGlobalStoragePath))
        return err == nil && info.IsDir()
    },
    DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
        switch ct {
        case catalog.Rules:
            return []string{
                filepath.Join(projectRoot, ".clinerules"),       // directory form
                filepath.Join(projectRoot, ".clinerules"),       // legacy single-file form (same path, scanner handles both)
            }
        default:
            return nil
        }
    },
    FileFormat: func(ct catalog.ContentType) Format {
        switch ct {
        case catalog.MCP:
            return FormatJSON
        default:
            return FormatMarkdown
        }
    },
    EmitPath: func(projectRoot string) string {
        return filepath.Join(projectRoot, ".clinerules")
    },
    SupportsType: func(ct catalog.ContentType) bool {
        switch ct {
        case catalog.Rules, catalog.MCP:
            return true
        default:
            return false
        }
    },
}
```

**Why `DiscoveryPaths` returns the same path twice:** The scanner already handles the case where a path is a directory vs. a file. Listing `.clinerules` once covers both the directory and legacy single-file cases. Remove the duplicate in the final implementation — list it once:

```go
case catalog.Rules:
    return []string{filepath.Join(projectRoot, ".clinerules")}
```

**How to verify:** `make build` compiles.

---

### Task 3.2 — Register Cline in AllProviders

**Dependencies:** Task 3.1

**Files to modify:**
- `cli/internal/provider/provider.go`

**What to implement:**

```go
var AllProviders = []Provider{
    ClaudeCode,
    GeminiCLI,
    Cursor,
    Windsurf,
    Codex,
    CopilotCLI,
    Zed,
    Cline,
}
```

---

### Task 3.3 — Add Cline MCP renderer

**Dependencies:** Task 1.3, Task 3.1

**Files to modify:**
- `cli/internal/converter/mcp.go`

**What to implement:**

Cline uses the same `mcpServers` key as canonical but has `alwaysAllow` instead of `autoApprove`. Add to the Canonicalize switch:

```go
case "cline":
    return canonicalizeClineMCP(content)
```

Add a Cline-specific on-disk struct:

```go
// clineServerConfig is Cline's on-disk format for a single MCP server.
// Differs from canonical in "alwaysAllow" (vs Claude's "autoApprove").
type clineServerConfig struct {
    Command     string            `json:"command,omitempty"`
    Args        []string          `json:"args,omitempty"`
    Env         map[string]string `json:"env,omitempty"`
    AlwaysAllow []string          `json:"alwaysAllow,omitempty"`
    Disabled    bool              `json:"disabled,omitempty"`
}

type clineMCPConfig struct {
    MCPServers map[string]clineServerConfig `json:"mcpServers"`
}
```

```go
func canonicalizeClineMCP(content []byte) (*Result, error) {
    var cc clineMCPConfig
    if err := json.Unmarshal(content, &cc); err != nil {
        return nil, fmt.Errorf("parsing Cline MCP config: %w", err)
    }

    out := mcpConfig{MCPServers: make(map[string]mcpServerConfig)}
    for name, s := range cc.MCPServers {
        out.MCPServers[name] = mcpServerConfig{
            Command:     s.Command,
            Args:        s.Args,
            Env:         s.Env,
            AutoApprove: s.AlwaysAllow, // normalize to canonical field
            Disabled:    s.Disabled,
        }
    }

    result, err := json.MarshalIndent(out, "", "  ")
    if err != nil {
        return nil, err
    }
    return &Result{Content: result, Filename: "mcp.json"}, nil
}
```

Add `"cline"` to the Render switch:

```go
case "cline":
    return renderClineMCP(cfg)
```

```go
func renderClineMCP(cfg mcpConfig) (*Result, error) {
    var warnings []string
    cc := clineMCPConfig{MCPServers: make(map[string]clineServerConfig)}

    for name, server := range cfg.MCPServers {
        s := clineServerConfig{
            Command:     server.Command,
            Args:        server.Args,
            Env:         server.Env,
            AlwaysAllow: server.AutoApprove,
            Disabled:    server.Disabled,
        }

        // Warn: Cline doesn't support HTTP transport at project level
        if server.URL != "" {
            warnings = append(warnings, fmt.Sprintf("server %q: HTTP transport (url) not supported by Cline; server skipped", name))
            continue
        }

        if server.Trust != "" {
            warnings = append(warnings, fmt.Sprintf("server %q: trust dropped (Gemini-specific)", name))
        }
        if len(server.IncludeTools) > 0 {
            warnings = append(warnings, fmt.Sprintf("server %q: includeTools dropped (Gemini-specific)", name))
        }

        cc.MCPServers[name] = s
    }

    result, err := json.MarshalIndent(cc, "", "  ")
    if err != nil {
        return nil, err
    }
    return &Result{Content: result, Filename: "cline_mcp_settings.json", Warnings: warnings}, nil
}
```

**How to verify:** `make test` passes.

---

### Task 3.3b — Add Cline rules renderer

**Dependencies:** Tasks 1.1, 3.1

**Files to modify:**
- `cli/internal/converter/rules.go`

**What to implement:**

Cline rules go to `.clinerules/` as individual markdown files (or a single `.clinerules` file). The canonical format maps to plain markdown. Add to the Render switch:

```go
case "cline":
    return renderClineRule(meta, body)
```

```go
// renderClineRule renders a rule as plain markdown for Cline's .clinerules directory.
// Cline does not support frontmatter in rule files.
func renderClineRule(meta RuleMeta, body string) (*Result, error) {
    if meta.AlwaysApply {
        return &Result{Content: []byte(body + "\n"), Filename: ".clinerules"}, nil
    }

    var notes []string
    switch {
    case len(meta.Globs) > 0:
        notes = append(notes, fmt.Sprintf("**Scope:** Apply only when working with files matching: %s", strings.Join(meta.Globs, ", ")))
    case meta.Description != "":
        notes = append(notes, fmt.Sprintf("**Scope:** Apply when: %s", meta.Description))
    default:
        notes = append(notes, "**Scope:** Apply only when explicitly asked.")
    }

    notesBlock := BuildConversionNotes("syllago", notes)
    result := AppendNotes(body, notesBlock)
    return &Result{Content: []byte(result + "\n"), Filename: ".clinerules"}, nil
}
```

Also handle `"cline"` in Canonicalize:

```go
case "cline":
    return canonicalizeMarkdownRule(content)
```

Also add Cline rule test cases to `rules_test.go`:

```go
func TestClineRuleRender(t *testing.T) {
    input := []byte("---\nalwaysApply: true\n---\n\nAlways follow these guidelines.\n")
    conv := &RulesConverter{}
    canonical, err := conv.Canonicalize(input, "claude-code")
    if err != nil {
        t.Fatalf("Canonicalize: %v", err)
    }
    result, err := conv.Render(canonical.Content, provider.Cline)
    if err != nil {
        t.Fatalf("Render: %v", err)
    }
    assertContains(t, string(result.Content), "Always follow")
    assertEqual(t, ".clinerules", result.Filename)
    assertNotContains(t, string(result.Content), "---") // no frontmatter
}
```

**How to verify:** `go test ./cli/internal/converter/... -run TestClineRule` passes.

---

### Task 3.4 — Write Cline MCP tests and provider tests

**Dependencies:** Tasks 3.1, 3.3

**Files to modify:**
- `cli/internal/converter/mcp_test.go`

**Files to create:**
- `cli/internal/provider/cline_test.go`

**What to implement in `mcp_test.go`:**

```go
func TestClineMCPRender(t *testing.T) {
    input := []byte(`{
        "mcpServers": {
            "github": {
                "command": "npx",
                "args": ["-y", "@modelcontextprotocol/server-github"],
                "env": {"GITHUB_TOKEN": "tok"},
                "autoApprove": ["search_repositories"]
            }
        }
    }`)

    conv := &MCPConverter{}
    canonical, err := conv.Canonicalize(input, "claude-code")
    if err != nil {
        t.Fatalf("Canonicalize: %v", err)
    }

    result, err := conv.Render(canonical.Content, provider.Cline)
    if err != nil {
        t.Fatalf("Render: %v", err)
    }

    out := string(result.Content)
    assertContains(t, out, "mcpServers")
    assertContains(t, out, "alwaysAllow")
    assertContains(t, out, "search_repositories")
    assertNotContains(t, out, "autoApprove")
    assertEqual(t, "cline_mcp_settings.json", result.Filename)
}

func TestClineMCPCanonicalize(t *testing.T) {
    input := []byte(`{
        "mcpServers": {
            "slack": {
                "command": "npx",
                "args": ["-y", "@modelcontextprotocol/server-slack"],
                "env": {"SLACK_TOKEN": "xoxb"},
                "alwaysAllow": ["send_message"],
                "disabled": false
            }
        }
    }`)

    conv := &MCPConverter{}
    result, err := conv.Canonicalize(input, "cline")
    if err != nil {
        t.Fatalf("Canonicalize: %v", err)
    }

    out := string(result.Content)
    assertContains(t, out, "mcpServers")
    assertContains(t, out, "autoApprove")   // normalized from alwaysAllow
    assertContains(t, out, "send_message")
    assertNotContains(t, out, "alwaysAllow")
}
```

**What to implement in `cline_test.go`:**

```go
package provider

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestClineDetect(t *testing.T) {
    dir := t.TempDir()
    if Cline.Detect(dir) {
        t.Fatal("expected no detection in empty temp dir")
    }

    storagePath := filepath.Join(dir, ".config", "Code", "User", "globalStorage", "saoudrizwan.claude-dev", "settings")
    if err := os.MkdirAll(storagePath, 0755); err != nil {
        t.Fatal(err)
    }
    if !Cline.Detect(dir) {
        t.Fatal("expected detection when VS Code globalStorage path exists")
    }
}

func TestClineSupportsType(t *testing.T) {
    if !Cline.SupportsType(catalog.Rules) {
        t.Error("Cline must support Rules")
    }
    if !Cline.SupportsType(catalog.MCP) {
        t.Error("Cline must support MCP")
    }
    if Cline.SupportsType(catalog.Agents) {
        t.Error("Cline must not support Agents")
    }
    if Cline.SupportsType(catalog.Hooks) {
        t.Error("Cline must not support Hooks")
    }
}

func TestClineDiscoveryPaths(t *testing.T) {
    paths := Cline.DiscoveryPaths("/project", catalog.Rules)
    if len(paths) != 1 {
        t.Fatalf("expected 1 discovery path, got %d", len(paths))
    }
    if paths[0] != "/project/.clinerules" {
        t.Errorf("expected /project/.clinerules, got %q", paths[0])
    }
}
```

**How to verify:** `go test ./cli/internal/provider/... -run TestCline` and `go test ./cli/internal/converter/... -run TestCline` pass.

---

## Phase 4: Roo Code Provider (Medium)

Rules (mode-aware) + MCP (project-level) + Agents (custom YAML modes).

---

### Task 4.1 — Create Roo Code provider file

**Dependencies:** Task 1.1

**Files to create:**
- `cli/internal/provider/roocode.go`

**What to implement:**

```go
package provider

import (
    "os"
    "path/filepath"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
)

var RooCode = Provider{
    Name:      "Roo Code",
    Slug:      "roo-code",
    ConfigDir: ".roo",
    InstallDir: func(homeDir string, ct catalog.ContentType) string {
        // Roo Code rules and agents are project-level (no home dir install).
        // Return empty; caller handles project-relative placement.
        // MCP uses JSON merge into .roo/mcp.json in the project.
        switch ct {
        case catalog.MCP:
            return JSONMergeSentinel
        }
        return ""
    },
    Detect: func(homeDir string) bool {
        // Roo Code is detected by .roo/ directory presence in the project root.
        // There is no meaningful home detection (VS Code globalStorage is not consistent).
        // Convention: check if the home directory has a .roo/ directory as a fallback.
        info, err := os.Stat(filepath.Join(homeDir, ".roo"))
        return err == nil && info.IsDir()
    },
    DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
        switch ct {
        case catalog.Rules:
            return []string{
                filepath.Join(projectRoot, ".roo", "rules"),          // all-mode rules
                filepath.Join(projectRoot, ".roo", "rules-code"),     // code mode
                filepath.Join(projectRoot, ".roo", "rules-architect"),// architect mode
                filepath.Join(projectRoot, ".roo", "rules-ask"),      // ask mode
                filepath.Join(projectRoot, ".roo", "rules-debug"),    // debug mode
                filepath.Join(projectRoot, ".roo", "rules-orchestrator"), // orchestrator mode
                filepath.Join(projectRoot, ".roorules"),              // legacy single file
            }
        case catalog.Agents:
            // Custom modes (agents) live in .roomodes or inline in config
            return []string{filepath.Join(projectRoot, ".roomodes")}
        case catalog.MCP:
            return []string{filepath.Join(projectRoot, ".roo", "mcp.json")}
        default:
            return nil
        }
    },
    FileFormat: func(ct catalog.ContentType) Format {
        switch ct {
        case catalog.Agents:
            return FormatYAML
        case catalog.MCP:
            return FormatJSON
        default:
            return FormatMarkdown
        }
    },
    EmitPath: func(projectRoot string) string {
        return filepath.Join(projectRoot, ".roo", "rules")
    },
    SupportsType: func(ct catalog.ContentType) bool {
        switch ct {
        case catalog.Rules, catalog.MCP, catalog.Agents:
            return true
        default:
            return false
        }
    },
}
```

**How to verify:** `make build` compiles.

---

### Task 4.2 — Register Roo Code in AllProviders

**Dependencies:** Task 4.1

**Files to modify:**
- `cli/internal/provider/provider.go`

**What to implement:**

```go
var AllProviders = []Provider{
    ClaudeCode,
    GeminiCLI,
    Cursor,
    Windsurf,
    Codex,
    CopilotCLI,
    Zed,
    Cline,
    RooCode,
}
```

---

### Task 4.3 — Add Roo Code MCP renderer

**Dependencies:** Tasks 1.3, 4.1

**Files to modify:**
- `cli/internal/converter/mcp.go`

**What to implement:**

Roo Code's MCP format is identical to the canonical `mcpServers` format — same key, same field names. The only difference is the output filename is `mcp.json` (not `mcp.json`). This means the existing canonical handling covers Roo Code almost entirely.

Add to Canonicalize switch (Roo Code uses standard format, so delegate to default):

```go
case "roo-code":
    // Roo Code uses the same mcpServers format as canonical
    return c.Canonicalize(content, "claude-code")
```

Add to Render switch:

```go
case "roo-code":
    return renderRooCodeMCP(cfg)
```

```go
func renderRooCodeMCP(cfg mcpConfig) (*Result, error) {
    // Roo Code uses the standard mcpServers format.
    // Drop Claude-specific and Gemini-specific fields, same as Copilot.
    result, err := renderCopilotMCP(cfg)
    if err != nil {
        return nil, err
    }
    // Roo Code writes to .roo/mcp.json in the project root
    result.Filename = "mcp.json"
    return result, nil
}
```

**How to verify:** `make test` passes.

---

### Task 4.3b — Add Roo Code rules renderer (mode-aware)

**Dependencies:** Tasks 1.1, 4.1

**Files to modify:**
- `cli/internal/converter/rules.go`

**What to implement:**

Per design decision D4, Roo Code rules default to `.roo/rules/` (all modes) but can target mode-specific subdirectories (`rules-code`, `rules-architect`, etc.). The converter renders plain markdown; the target directory is determined by the installer via the provider's `EmitPath`. Add to the Render switch:

```go
case "roo-code":
    return renderRooCodeRule(meta, body)
```

```go
// renderRooCodeRule renders a rule as plain markdown for Roo Code's .roo/rules/ directory.
// Roo Code does not support frontmatter in rule files.
func renderRooCodeRule(meta RuleMeta, body string) (*Result, error) {
    if meta.AlwaysApply {
        return &Result{Content: []byte(body + "\n"), Filename: meta.Description + ".md"}, nil
    }

    var notes []string
    switch {
    case len(meta.Globs) > 0:
        notes = append(notes, fmt.Sprintf("**Scope:** Apply only when working with files matching: %s", strings.Join(meta.Globs, ", ")))
    case meta.Description != "":
        notes = append(notes, fmt.Sprintf("**Scope:** Apply when: %s", meta.Description))
    default:
        notes = append(notes, "**Scope:** Apply only when explicitly asked.")
    }

    notesBlock := BuildConversionNotes("syllago", notes)
    result := AppendNotes(body, notesBlock)

    filename := "rule.md"
    if meta.Description != "" {
        filename = slugify(meta.Description) + ".md"
    }
    return &Result{Content: []byte(result + "\n"), Filename: filename}, nil
}
```

Also handle `"roo-code"` in Canonicalize:

```go
case "roo-code":
    return canonicalizeMarkdownRule(content)
```

**Note on mode-aware install (D4):** The TUI mode selection (choosing `rules-code`, `rules-architect`, etc.) is an installer-level concern, not a converter concern. The converter always outputs plain markdown with a filename. The installer is responsible for placing the file in the correct mode subdirectory based on user choice. This task does not implement TUI changes — that is deferred to the installer phase or v1.1. The `EmitPath` in `roocode.go` returns `.roo/rules/` as the default (all-mode) target.

Also add Roo Code rule test cases to `rules_test.go`:

```go
func TestRooCodeRuleRender(t *testing.T) {
    input := []byte("---\nalwaysApply: true\ndescription: TypeScript rules\n---\n\nUse TypeScript strict mode.\n")
    conv := &RulesConverter{}
    canonical, err := conv.Canonicalize(input, "claude-code")
    if err != nil {
        t.Fatalf("Canonicalize: %v", err)
    }
    result, err := conv.Render(canonical.Content, provider.RooCode)
    if err != nil {
        t.Fatalf("Render: %v", err)
    }
    assertContains(t, string(result.Content), "TypeScript strict mode")
    assertNotContains(t, string(result.Content), "---") // no frontmatter
}
```

**How to verify:** `go test ./cli/internal/converter/... -run TestRooCodeRule` passes.

---

### Task 4.4 — Add Roo Code custom mode agent renderer

**Dependencies:** Tasks 1.4, 4.1

**Files to modify:**
- `cli/internal/converter/agents.go`

**What to implement:**

Roo Code agents are YAML "custom modes". Add an output struct:

```go
// rooCodeMode is Roo Code's YAML custom mode definition.
type rooCodeMode struct {
    Slug              string   `yaml:"slug"`
    Name              string   `yaml:"name"`
    RoleDefinition    string   `yaml:"roleDefinition"`
    WhenToUse         string   `yaml:"whenToUse,omitempty"`
    CustomInstructions string  `yaml:"customInstructions,omitempty"`
    Groups            []string `yaml:"groups,omitempty"`
}
```

Add Roo Code to the Render switch:

```go
case "roo-code":
    return renderRooCodeAgent(meta, body)
```

Add the renderer:

```go
func renderRooCodeAgent(meta AgentMeta, body string) (*Result, error) {
    var warnings []string
    cleanBody := StripConversionNotes(body)

    // Determine tool groups from canonical tools list
    // Roo Code uses coarse-grained groups: read, browser, edit, command
    groups := inferRooCodeGroups(meta.Tools, meta.DisallowedTools)

    mode := rooCodeMode{
        Slug:           slugify(meta.Name),
        Name:           meta.Name,
        RoleDefinition: cleanBody,
        WhenToUse:      meta.Description,
    }
    if len(groups) > 0 {
        mode.Groups = groups
    }

    // Warn about unsupported fields
    if meta.MaxTurns > 0 {
        warnings = append(warnings, fmt.Sprintf("maxTurns (%d) not supported by roo-code (dropped)", meta.MaxTurns))
    }
    if meta.Model != "" {
        warnings = append(warnings, fmt.Sprintf("model (%q) not supported in roo-code mode definition (dropped)", meta.Model))
    }
    if meta.PermissionMode != "" {
        warnings = append(warnings, fmt.Sprintf("permissionMode (%q) not supported by roo-code (dropped)", meta.PermissionMode))
    }

    out, err := yaml.Marshal(mode)
    if err != nil {
        return nil, err
    }

    name := mode.Slug
    if name == "" {
        name = "mode"
    }
    return &Result{Content: out, Filename: name + ".yaml", Warnings: warnings}, nil
}

// inferRooCodeGroups maps canonical tool names to Roo Code tool groups.
// Roo Code groups: "read" (readonly ops), "edit" (file edits), "command" (shell), "browser".
func inferRooCodeGroups(tools, disallowed []string) []string {
    disallowedSet := make(map[string]bool, len(disallowed))
    for _, t := range disallowed {
        disallowedSet[t] = true
    }

    groupSet := map[string]bool{}
    for _, t := range tools {
        if disallowedSet[t] {
            continue
        }
        switch t {
        case "Read", "Glob", "Grep":
            groupSet["read"] = true
        case "Write", "Edit", "MultiEdit":
            groupSet["edit"] = true
        case "Bash":
            groupSet["command"] = true
        case "WebFetch", "WebSearch":
            groupSet["browser"] = true
        }
    }

    // If no explicit tools, default to read+edit (most permissive safe default)
    if len(tools) == 0 && len(disallowed) == 0 {
        return []string{"read", "edit", "command", "browser"}
    }

    var groups []string
    for _, g := range []string{"read", "browser", "edit", "command"} {
        if groupSet[g] {
            groups = append(groups, g)
        }
    }
    return groups
}

// slugify converts a display name to a kebab-case slug.
func slugify(name string) string {
    var b strings.Builder
    for _, c := range strings.ToLower(name) {
        if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
            b.WriteRune(c)
        } else if c == ' ' || c == '-' || c == '_' {
            b.WriteRune('-')
        }
    }
    return strings.Trim(b.String(), "-")
}
```

Also add a Roo Code canonicalizer. Roo Code custom modes are YAML; canonicalize them to the agent canonical format:

```go
case "roo-code":
    return canonicalizeRooCodeAgent(content)
```

```go
func canonicalizeRooCodeAgent(content []byte) (*Result, error) {
    var mode rooCodeMode
    if err := yaml.Unmarshal(content, &mode); err != nil {
        return nil, fmt.Errorf("parsing Roo Code mode YAML: %w", err)
    }

    meta := AgentMeta{
        Name:        mode.Name,
        Description: mode.WhenToUse,
    }
    body := strings.TrimSpace(mode.RoleDefinition)
    if mode.CustomInstructions != "" {
        if body != "" {
            body += "\n\n"
        }
        body += mode.CustomInstructions
    }

    canonical, err := buildAgentCanonical(meta, body)
    if err != nil {
        return nil, err
    }
    return &Result{Content: canonical, Filename: "agent.md"}, nil
}
```

**How to verify:** `make test` passes.

---

### Task 4.5 — Write Roo Code tests

**Dependencies:** Tasks 4.1, 4.3, 4.4

**Files to create:**
- `cli/internal/provider/roocode_test.go`

**Files to modify:**
- `cli/internal/converter/agents_test.go`

**What to implement in `roocode_test.go`:**

```go
package provider

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestRooCodeDetect(t *testing.T) {
    dir := t.TempDir()
    if RooCode.Detect(dir) {
        t.Fatal("expected no detection in empty temp dir")
    }

    if err := os.MkdirAll(filepath.Join(dir, ".roo"), 0755); err != nil {
        t.Fatal(err)
    }
    if !RooCode.Detect(dir) {
        t.Fatal("expected detection when ~/.roo/ exists")
    }
}

func TestRooCodeSupportsType(t *testing.T) {
    if !RooCode.SupportsType(catalog.Rules) {
        t.Error("RooCode must support Rules")
    }
    if !RooCode.SupportsType(catalog.MCP) {
        t.Error("RooCode must support MCP")
    }
    if !RooCode.SupportsType(catalog.Agents) {
        t.Error("RooCode must support Agents")
    }
    if RooCode.SupportsType(catalog.Hooks) {
        t.Error("RooCode must not support Hooks")
    }
    if RooCode.SupportsType(catalog.Skills) {
        t.Error("RooCode must not support Skills")
    }
    if RooCode.SupportsType(catalog.Commands) {
        t.Error("RooCode must not support Commands")
    }
}

func TestRooCodeDiscoveryPathsRules(t *testing.T) {
    paths := RooCode.DiscoveryPaths("/project", catalog.Rules)
    // Must include global rules dir and at least the built-in mode dirs
    found := map[string]bool{}
    for _, p := range paths {
        found[p] = true
    }
    if !found["/project/.roo/rules"] {
        t.Error("expected /project/.roo/rules in discovery paths")
    }
    if !found["/project/.roo/rules-code"] {
        t.Error("expected /project/.roo/rules-code in discovery paths")
    }
}
```

**What to implement in `agents_test.go`:**

```go
func TestClaudeAgentToRooCode(t *testing.T) {
    input := []byte("---\nname: Code Reviewer\ndescription: Reviews pull requests\ntools:\n  - Read\n  - Glob\n  - Grep\n---\n\nYou are a code reviewer focused on security and maintainability.\n")

    conv := &AgentsConverter{}
    canonical, err := conv.Canonicalize(input, "claude-code")
    if err != nil {
        t.Fatalf("Canonicalize: %v", err)
    }

    result, err := conv.Render(canonical.Content, provider.RooCode)
    if err != nil {
        t.Fatalf("Render: %v", err)
    }

    out := string(result.Content)
    assertContains(t, out, "slug: code-reviewer")
    assertContains(t, out, "name: Code Reviewer")
    assertContains(t, out, "roleDefinition:")
    assertContains(t, out, "security and maintainability")
    assertContains(t, out, "read")         // tool group
    assertNotContains(t, out, "edit")      // Read/Glob/Grep don't include edit group
    assertEqual(t, "code-reviewer.yaml", result.Filename)
}

func TestRooCodeAgentCanonicalize(t *testing.T) {
    input := []byte("slug: reviewer\nname: Code Reviewer\nroleDefinition: 'You are a strict code reviewer.'\nwhenToUse: 'Use when reviewing PRs'\ngroups:\n  - read\n  - browser\n")

    conv := &AgentsConverter{}
    result, err := conv.Canonicalize(input, "roo-code")
    if err != nil {
        t.Fatalf("Canonicalize: %v", err)
    }

    out := string(result.Content)
    assertContains(t, out, "name: Code Reviewer")
    assertContains(t, out, "strict code reviewer")
}

func TestRooCodeAgentMaxTurnsWarning(t *testing.T) {
    input := []byte("---\nname: limited\ndescription: Limited agent\nmaxTurns: 5\n---\n\nDo limited things.\n")

    conv := &AgentsConverter{}
    canonical, err := conv.Canonicalize(input, "claude-code")
    if err != nil {
        t.Fatalf("Canonicalize: %v", err)
    }

    result, err := conv.Render(canonical.Content, provider.RooCode)
    if err != nil {
        t.Fatalf("Render: %v", err)
    }

    if len(result.Warnings) == 0 {
        t.Fatal("expected warning about maxTurns being dropped")
    }
}
```

**How to verify:** `go test ./cli/internal/provider/... -run TestRooCode` and `go test ./cli/internal/converter/... -run TestRooCode` pass.

---

## Phase 5: OpenCode Provider (Medium)

Rules (AGENTS.md) + Commands + Agents (markdown) + Skills + MCP (JSONC).

---

### Task 5.1 — Create OpenCode provider file

**Dependencies:** Task 1.1

**Files to create:**
- `cli/internal/provider/opencode.go`

**What to implement:**

```go
package provider

import (
    "os"
    "os/exec"
    "path/filepath"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
)

var OpenCode = Provider{
    Name:      "OpenCode",
    Slug:      "opencode",
    ConfigDir: ".config/opencode",
    InstallDir: func(homeDir string, ct catalog.ContentType) string {
        base := filepath.Join(homeDir, ".config", "opencode")
        switch ct {
        case catalog.Rules:
            return base // AGENTS.md lives in home config dir
        case catalog.Commands:
            return filepath.Join(base, "commands")
        case catalog.Agents:
            return filepath.Join(base, "agents")
        case catalog.Skills:
            return filepath.Join(base, "skill")
        case catalog.MCP:
            return JSONMergeSentinel
        }
        return ""
    },
    Detect: func(homeDir string) bool {
        // Check for ~/.config/opencode/ directory
        info, err := os.Stat(filepath.Join(homeDir, ".config", "opencode"))
        if err == nil && info.IsDir() {
            return true
        }
        // Also check if opencode command exists
        _, err = exec.LookPath("opencode")
        return err == nil
    },
    DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
        switch ct {
        case catalog.Rules:
            return []string{filepath.Join(projectRoot, "AGENTS.md")}
        case catalog.Commands:
            return []string{filepath.Join(projectRoot, ".opencode", "commands")}
        case catalog.Agents:
            return []string{filepath.Join(projectRoot, ".opencode", "agents")}
        case catalog.Skills:
            return []string{filepath.Join(projectRoot, ".opencode", "skill")}
        case catalog.MCP:
            return []string{
                filepath.Join(projectRoot, "opencode.json"),
                filepath.Join(projectRoot, "opencode.jsonc"),
            }
        default:
            return nil
        }
    },
    FileFormat: func(ct catalog.ContentType) Format {
        switch ct {
        case catalog.MCP:
            return FormatJSONC
        default:
            return FormatMarkdown
        }
    },
    EmitPath: func(projectRoot string) string {
        return filepath.Join(projectRoot, "AGENTS.md")
    },
    SupportsType: func(ct catalog.ContentType) bool {
        switch ct {
        case catalog.Rules, catalog.Commands, catalog.Agents, catalog.Skills, catalog.MCP:
            return true
        default:
            return false
        }
    },
}
```

**How to verify:** `make build` compiles.

---

### Task 5.2 — Register OpenCode in AllProviders

**Dependencies:** Task 5.1

**Files to modify:**
- `cli/internal/provider/provider.go`

**What to implement:**

```go
var AllProviders = []Provider{
    ClaudeCode,
    GeminiCLI,
    Cursor,
    Windsurf,
    Codex,
    CopilotCLI,
    Zed,
    Cline,
    RooCode,
    OpenCode,
}
```

---

### Task 5.3 — Write OpenCode MCP tests

**Dependencies:** Task 1.3, Task 5.1

**Files to modify:**
- `cli/internal/converter/mcp_test.go`

**What to implement:**

```go
func TestOpenCodeMCPRender(t *testing.T) {
    input := []byte(`{
        "mcpServers": {
            "filesystem": {
                "command": "npx",
                "args": ["-y", "@modelcontextprotocol/server-filesystem", "/home"],
                "env": {"DEBUG": "1"}
            }
        }
    }`)

    conv := &MCPConverter{}
    canonical, err := conv.Canonicalize(input, "claude-code")
    if err != nil {
        t.Fatalf("Canonicalize: %v", err)
    }

    result, err := conv.Render(canonical.Content, provider.OpenCode)
    if err != nil {
        t.Fatalf("Render: %v", err)
    }

    out := string(result.Content)
    // OpenCode uses "mcp" key, not "mcpServers"
    assertContains(t, out, `"mcp"`)
    assertNotContains(t, out, `"mcpServers"`)
    // Command must be an array
    assertContains(t, out, `"command": [`)
    assertContains(t, out, `"npx"`)
    // Env key must be "environment"
    assertContains(t, out, `"environment"`)
    assertNotContains(t, out, `"env"`)
    // Type must be "local" for stdio
    assertContains(t, out, `"type": "local"`)
    assertEqual(t, "opencode.json", result.Filename)
}

func TestOpenCodeMCPCanonicalize(t *testing.T) {
    input := []byte(`{
        "mcp": {
            "local-server": {
                "type": "local",
                "command": ["npx", "-y", "my-mcp"],
                "environment": {"API_KEY": "secret"},
                "enabled": true,
                "timeout": 5000
            }
        }
    }`)

    conv := &MCPConverter{}
    result, err := conv.Canonicalize(input, "opencode")
    if err != nil {
        t.Fatalf("Canonicalize: %v", err)
    }

    out := string(result.Content)
    assertContains(t, out, "mcpServers")
    assertContains(t, out, `"command": "npx"`)
    assertContains(t, out, `"args": [`)
    assertContains(t, out, `"env"`)
    assertContains(t, out, "API_KEY")
    assertNotContains(t, out, "environment")
    assertNotContains(t, out, `"mcp"`)
}

func TestOpenCodeMCPCanonicalizeJSONC(t *testing.T) {
    // Test that JSONC comments are stripped before parsing
    input := []byte(`{
        // Main MCP config for OpenCode
        "mcp": {
            /* database server */
            "db": {
                "type": "local",
                "command": ["db-mcp"],
                "enabled": false  // disabled
            }
        }
    }`)

    conv := &MCPConverter{}
    result, err := conv.Canonicalize(input, "opencode")
    if err != nil {
        t.Fatalf("Canonicalize with JSONC comments: %v", err)
    }

    out := string(result.Content)
    assertContains(t, out, "mcpServers")
    assertContains(t, out, "db")
    // enabled: false should map to disabled: true
    assertContains(t, out, `"disabled": true`)
}

func TestOpenCodeMCPRemoteServer(t *testing.T) {
    input := []byte(`{
        "mcpServers": {
            "remote": {
                "url": "https://mcp.example.com",
                "type": "sse"
            }
        }
    }`)

    conv := &MCPConverter{}
    canonical, err := conv.Canonicalize(input, "claude-code")
    if err != nil {
        t.Fatalf("Canonicalize: %v", err)
    }

    result, err := conv.Render(canonical.Content, provider.OpenCode)
    if err != nil {
        t.Fatalf("Render: %v", err)
    }

    out := string(result.Content)
    assertContains(t, out, `"type": "remote"`)
    assertContains(t, out, "mcp.example.com")
}
```

**How to verify:** `go test ./cli/internal/converter/... -run TestOpenCode` passes.

---

### Task 5.4 — Write OpenCode provider test

**Dependencies:** Task 5.1

**Files to create:**
- `cli/internal/provider/opencode_test.go`

**What to implement:**

```go
package provider

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestOpenCodeDetect(t *testing.T) {
    dir := t.TempDir()
    if OpenCode.Detect(dir) {
        t.Fatal("expected no detection in empty temp dir")
    }

    if err := os.MkdirAll(filepath.Join(dir, ".config", "opencode"), 0755); err != nil {
        t.Fatal(err)
    }
    if !OpenCode.Detect(dir) {
        t.Fatal("expected detection when ~/.config/opencode/ exists")
    }
}

func TestOpenCodeSupportsType(t *testing.T) {
    for _, ct := range []catalog.ContentType{catalog.Rules, catalog.Commands, catalog.Agents, catalog.Skills, catalog.MCP} {
        if !OpenCode.SupportsType(ct) {
            t.Errorf("OpenCode must support %s", ct)
        }
    }
    if OpenCode.SupportsType(catalog.Hooks) {
        t.Error("OpenCode must not support Hooks")
    }
}

func TestOpenCodeFileFormat(t *testing.T) {
    if OpenCode.FileFormat(catalog.MCP) != FormatJSONC {
        t.Error("OpenCode MCP format must be FormatJSONC")
    }
    if OpenCode.FileFormat(catalog.Rules) != FormatMarkdown {
        t.Error("OpenCode Rules format must be FormatMarkdown")
    }
}

func TestOpenCodeDiscoveryPaths(t *testing.T) {
    paths := OpenCode.DiscoveryPaths("/project", catalog.Rules)
    if len(paths) != 1 || paths[0] != "/project/AGENTS.md" {
        t.Errorf("expected /project/AGENTS.md, got %v", paths)
    }

    paths = OpenCode.DiscoveryPaths("/project", catalog.MCP)
    if len(paths) != 2 {
        t.Fatalf("expected 2 MCP discovery paths, got %d", len(paths))
    }
}

func TestOpenCodeInstallDir(t *testing.T) {
    skillDir := OpenCode.InstallDir("/home/user", catalog.Skills)
    expected := "/home/user/.config/opencode/skill"
    if skillDir != expected {
        t.Errorf("expected %q, got %q", expected, skillDir)
    }
    if OpenCode.InstallDir("/home/user", catalog.MCP) != JSONMergeSentinel {
        t.Error("MCP must return JSONMergeSentinel")
    }
}
```

**How to verify:** `go test ./cli/internal/provider/... -run TestOpenCode` passes.

---

### Task 5.5 — Add OpenCode rules renderer

**Dependencies:** Tasks 1.1, 5.1

**Files to modify:**
- `cli/internal/converter/rules.go`

**What to implement:**

OpenCode rules go to `AGENTS.md` as plain markdown (no frontmatter). Add to the Render switch:

```go
case "opencode":
    return renderOpenCodeRule(meta, body)
```

```go
// renderOpenCodeRule renders a rule as plain markdown for OpenCode's AGENTS.md.
// OpenCode does not support frontmatter in AGENTS.md — it is plain markdown.
// Scope information from alwaysApply/globs is embedded as prose if needed.
func renderOpenCodeRule(meta RuleMeta, body string) (*Result, error) {
    if meta.AlwaysApply {
        return &Result{Content: []byte(body + "\n"), Filename: "AGENTS.md"}, nil
    }

    // Embed scope as prose
    var notes []string
    switch {
    case len(meta.Globs) > 0:
        notes = append(notes, fmt.Sprintf("**Scope:** Apply only when working with files matching: %s", strings.Join(meta.Globs, ", ")))
    case meta.Description != "":
        notes = append(notes, fmt.Sprintf("**Scope:** Apply when: %s", meta.Description))
    default:
        notes = append(notes, "**Scope:** Apply only when explicitly asked.")
    }

    notesBlock := BuildConversionNotes("syllago", notes)
    result := AppendNotes(body, notesBlock)
    return &Result{Content: []byte(result + "\n"), Filename: "AGENTS.md"}, nil
}
```

Also handle `"opencode"` in Canonicalize — AGENTS.md is plain markdown:

```go
case "opencode":
    return canonicalizeMarkdownRule(content)
```

**How to verify:** `make test` passes.

---

### Task 5.6 — Add OpenCode agent renderer

**Dependencies:** Tasks 1.1, 5.1

**Files to modify:**
- `cli/internal/converter/agents.go`

**What to implement:**

OpenCode agents live in `.opencode/agents/` as markdown files with YAML frontmatter (same structure as Claude Code agents). The conversion is nearly a passthrough — translate tool names if needed. Add to the Render switch:

```go
case "opencode":
    return renderOpenCodeAgent(meta, body)
```

```go
// renderOpenCodeAgent renders a canonical agent to OpenCode's markdown format.
// OpenCode agents are markdown files with YAML frontmatter in .opencode/agents/.
// The format is nearly identical to Claude Code's sub-agents.
func renderOpenCodeAgent(meta AgentMeta, body string) (*Result, error) {
    var warnings []string
    cleanBody := StripConversionNotes(body)

    // OpenCode does not support permissionMode
    if meta.PermissionMode != "" {
        warnings = append(warnings, fmt.Sprintf("permissionMode (%q) not supported by OpenCode (dropped)", meta.PermissionMode))
    }

    canonical, err := buildAgentCanonical(AgentMeta{
        Name:        meta.Name,
        Description: meta.Description,
        Tools:       meta.Tools,
        Model:       meta.Model,
        MaxTurns:    meta.MaxTurns,
    }, cleanBody)
    if err != nil {
        return nil, err
    }

    name := "agent"
    if meta.Name != "" {
        name = slugify(meta.Name)
    }
    return &Result{Content: canonical, Filename: name + ".md", Warnings: warnings}, nil
}
```

Also handle `"opencode"` in Canonicalize (OpenCode agents use the same markdown format as Claude Code):

```go
case "opencode":
    return c.Canonicalize(content, "claude-code")
```

Add OpenCode agent test cases to `agents_test.go`:

```go
func TestClaudeAgentToOpenCode(t *testing.T) {
    input := []byte("---\nname: Refactor Bot\ndescription: Refactoring assistant\npermissionMode: bypassPermissions\n---\n\nYou help refactor code.\n")

    conv := &AgentsConverter{}
    canonical, err := conv.Canonicalize(input, "claude-code")
    if err != nil {
        t.Fatalf("Canonicalize: %v", err)
    }

    result, err := conv.Render(canonical.Content, provider.OpenCode)
    if err != nil {
        t.Fatalf("Render: %v", err)
    }

    out := string(result.Content)
    assertContains(t, out, "Refactor Bot")
    assertContains(t, out, "refactor code")
    assertNotContains(t, out, "permissionMode")  // dropped
    assertEqual(t, "refactor-bot.md", result.Filename)

    if len(result.Warnings) == 0 {
        t.Fatal("expected warning about dropped permissionMode")
    }
}
```

**How to verify:** `go test ./cli/internal/converter/... -run TestOpenCodeAgent` passes.

---

### Task 5.7 — Add OpenCode skill renderer

**Dependencies:** Tasks 1.1, 5.1

**Files to modify:**
- `cli/internal/converter/skills.go`

**What to implement:**

OpenCode skills live in `.opencode/skill/` as markdown files. The format is plain markdown (no YAML frontmatter in OpenCode skills). Add to the Render switch:

```go
case "opencode":
    return renderOpenCodeSkill(meta, body)
```

```go
// renderOpenCodeSkill renders a canonical skill to OpenCode's plain markdown format.
// OpenCode skills in .opencode/skill/ are plain markdown without frontmatter.
func renderOpenCodeSkill(meta SkillMeta, body string) (*Result, error) {
    var warnings []string
    cleanBody := StripConversionNotes(body)

    var header strings.Builder
    if meta.Name != "" {
        header.WriteString("# ")
        header.WriteString(meta.Name)
        header.WriteString("\n\n")
    }
    if meta.Description != "" {
        header.WriteString(meta.Description)
        header.WriteString("\n\n")
    }

    content := header.String() + cleanBody

    if len(meta.AllowedTools) > 0 {
        warnings = append(warnings, "allowed-tools not supported in OpenCode skill files (dropped)")
    }
    if meta.UserInvocable != nil {
        warnings = append(warnings, "user-invocable not supported in OpenCode skill files (dropped)")
    }

    name := "skill"
    if meta.Name != "" {
        name = slugify(meta.Name)
    }

    return &Result{
        Content:  []byte(content + "\n"),
        Filename: name + ".md",
        Warnings: warnings,
    }, nil
}
```

Also handle `"opencode"` in Canonicalize (OpenCode skills are plain markdown):

```go
case "opencode":
    return canonicalizeSkillFromMarkdown(content)
```

Add OpenCode skill test cases to `skills_test.go`:

```go
func TestClaudeSkillToOpenCode(t *testing.T) {
    input := []byte("---\nname: Go Expert\ndescription: Go coding guidelines\nallowed-tools:\n  - Read\n---\n\nUse idiomatic Go patterns.\n")

    conv := &SkillsConverter{}
    canonical, err := conv.Canonicalize(input, "claude-code")
    if err != nil {
        t.Fatalf("Canonicalize: %v", err)
    }

    result, err := conv.Render(canonical.Content, provider.OpenCode)
    if err != nil {
        t.Fatalf("Render: %v", err)
    }

    out := string(result.Content)
    assertContains(t, out, "# Go Expert")
    assertContains(t, out, "idiomatic Go")
    assertNotContains(t, out, "allowed-tools")
    assertEqual(t, "go-expert.md", result.Filename)

    if len(result.Warnings) == 0 {
        t.Fatal("expected warning about dropped allowed-tools")
    }
}
```

**How to verify:** `go test ./cli/internal/converter/... -run TestOpenCodeSkill` passes.

---

### Task 5.8 — Add OpenCode command renderer

**Dependencies:** Tasks 1.1, 5.1

**Files to modify:**
- `cli/internal/converter/commands.go`

**What to implement:**

OpenCode commands live in `.opencode/commands/` as markdown files with YAML frontmatter. The format maps closely to Claude Code's slash commands. Add to the Render switch:

```go
case "opencode":
    return renderOpenCodeCommand(meta, body)
```

```go
// renderOpenCodeCommand renders a canonical command to OpenCode's markdown format.
// OpenCode commands are markdown files in .opencode/commands/ with optional frontmatter.
func renderOpenCodeCommand(meta CommandMeta, body string) (*Result, error) {
    cleanBody := StripConversionNotes(body)

    name := "command"
    if meta.Name != "" {
        name = slugify(meta.Name)
    }

    // Build minimal frontmatter if description is present
    var buf strings.Builder
    if meta.Description != "" {
        buf.WriteString("---\n")
        buf.WriteString("description: ")
        buf.WriteString(meta.Description)
        buf.WriteString("\n---\n\n")
    }
    buf.WriteString(cleanBody)
    buf.WriteString("\n")

    return &Result{Content: []byte(buf.String()), Filename: name + ".md"}, nil
}
```

Also handle `"opencode"` in Canonicalize (same format as Claude Code commands):

```go
case "opencode":
    return c.Canonicalize(content, "claude-code")
```

Add OpenCode command test cases to `commands_test.go` (create or extend):

```go
func TestClaudeCommandToOpenCode(t *testing.T) {
    input := []byte("---\ndescription: Run the test suite\n---\n\nExecute all tests with coverage.\n")

    conv := &CommandsConverter{}
    canonical, err := conv.Canonicalize(input, "claude-code")
    if err != nil {
        t.Fatalf("Canonicalize: %v", err)
    }

    result, err := conv.Render(canonical.Content, provider.OpenCode)
    if err != nil {
        t.Fatalf("Render: %v", err)
    }

    out := string(result.Content)
    assertContains(t, out, "test suite")
    assertContains(t, out, "coverage")
}
```

**How to verify:** `go test ./cli/internal/converter/... -run TestOpenCodeCommand` passes.

---

## Phase 6: Kiro Provider (Complex)

Rules (steering files) + Agents (JSON + file:// prompts) + Hooks (in agent files) + MCP + Skills (steering).

---

### Task 6.1 — Create Kiro provider file

**Dependencies:** Task 1.1

**Files to create:**
- `cli/internal/provider/kiro.go`

**What to implement:**

```go
package provider

import (
    "os"
    "path/filepath"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
)

var Kiro = Provider{
    Name:      "Kiro",
    Slug:      "kiro",
    ConfigDir: ".kiro",
    InstallDir: func(homeDir string, ct catalog.ContentType) string {
        base := filepath.Join(homeDir, ".kiro")
        switch ct {
        case catalog.Rules:
            // Steering files: per project in .kiro/steering/
            // Return empty string; EmitPath handles project-relative placement
            return ""
        case catalog.Agents:
            return filepath.Join(base, "agents")
        case catalog.Skills:
            // Skills map to steering files
            return ""
        case catalog.Hooks:
            // Hooks live inside agent JSON files (syllago-hooks.json)
            return JSONMergeSentinel
        case catalog.MCP:
            return JSONMergeSentinel
        }
        return ""
    },
    Detect: func(homeDir string) bool {
        // Check ~/.kiro/ directory
        info, err := os.Stat(filepath.Join(homeDir, ".kiro"))
        return err == nil && info.IsDir()
    },
    DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
        switch ct {
        case catalog.Rules:
            return []string{filepath.Join(projectRoot, ".kiro", "steering")}
        case catalog.Agents:
            return []string{
                filepath.Join(projectRoot, ".kiro", "agents"),
                filepath.Join(projectRoot, ".kiro", "prompts"),
            }
        case catalog.Skills:
            // Skills map to steering files as well
            return []string{filepath.Join(projectRoot, ".kiro", "steering")}
        case catalog.MCP:
            return []string{filepath.Join(projectRoot, ".kiro", "settings", "mcp.json")}
        case catalog.Hooks:
            return []string{filepath.Join(projectRoot, ".kiro", "agents")}
        default:
            return nil
        }
    },
    FileFormat: func(ct catalog.ContentType) Format {
        switch ct {
        case catalog.Agents:
            return FormatJSON
        case catalog.MCP, catalog.Hooks:
            return FormatJSON
        default:
            return FormatMarkdown
        }
    },
    EmitPath: func(projectRoot string) string {
        // Kiro rules emit to the steering directory
        return filepath.Join(projectRoot, ".kiro", "steering")
    },
    SupportsType: func(ct catalog.ContentType) bool {
        switch ct {
        case catalog.Rules, catalog.Agents, catalog.Hooks, catalog.MCP, catalog.Skills:
            return true
        default:
            return false
        }
    },
}
```

**How to verify:** `make build` compiles.

---

### Task 6.2 — Register Kiro in AllProviders

**Dependencies:** Task 6.1

**Files to modify:**
- `cli/internal/provider/provider.go`

**What to implement:**

```go
var AllProviders = []Provider{
    ClaudeCode,
    GeminiCLI,
    Cursor,
    Windsurf,
    Codex,
    CopilotCLI,
    Zed,
    Cline,
    RooCode,
    OpenCode,
    Kiro,
}
```

---

### Task 6.2b — Add Kiro rules renderer (steering files)

**Dependencies:** Tasks 1.1, 6.1

**Files to modify:**
- `cli/internal/converter/rules.go`

**What to implement:**

Per design decisions D3 and D8, Kiro uses `.kiro/steering/` for both rules and skills. Steering files are pure markdown with no frontmatter. The rules renderer simply strips frontmatter and writes a plain markdown file. Add to the Render switch:

```go
case "kiro":
    return renderKiroRule(meta, body)
```

```go
// renderKiroRule renders a rule as a plain markdown steering file for Kiro.
// Kiro steering files (.kiro/steering/) are plain markdown — no frontmatter.
func renderKiroRule(meta RuleMeta, body string) (*Result, error) {
    var notes []string

    // Embed scope as prose for non-always-apply rules
    if !meta.AlwaysApply {
        switch {
        case len(meta.Globs) > 0:
            notes = append(notes, fmt.Sprintf("**Scope:** Apply only when working with files matching: %s", strings.Join(meta.Globs, ", ")))
        case meta.Description != "":
            notes = append(notes, fmt.Sprintf("**Scope:** Apply when: %s", meta.Description))
        default:
            notes = append(notes, "**Scope:** Apply only when explicitly asked.")
        }
    }

    content := body
    if len(notes) > 0 {
        notesBlock := BuildConversionNotes("syllago", notes)
        content = AppendNotes(body, notesBlock)
    }

    filename := "rule.md"
    if meta.Description != "" {
        filename = slugify(meta.Description) + ".md"
    }
    return &Result{Content: []byte(content + "\n"), Filename: filename}, nil
}
```

Also handle `"kiro"` in Canonicalize — steering files are plain markdown:

```go
case "kiro":
    return canonicalizeMarkdownRule(content)
```

Also add Kiro rule test cases to `rules_test.go`:

```go
func TestKiroRuleRender(t *testing.T) {
    input := []byte("---\nalwaysApply: true\n---\n\nAlways follow these guidelines.\n")
    conv := &RulesConverter{}
    canonical, err := conv.Canonicalize(input, "claude-code")
    if err != nil {
        t.Fatalf("Canonicalize: %v", err)
    }
    result, err := conv.Render(canonical.Content, provider.Kiro)
    if err != nil {
        t.Fatalf("Render: %v", err)
    }
    assertContains(t, string(result.Content), "Always follow")
    assertNotContains(t, string(result.Content), "---") // no frontmatter
}
```

**How to verify:** `go test ./cli/internal/converter/... -run TestKiroRule` passes.

---

### Task 6.3 — Add Kiro MCP renderer

**Dependencies:** Tasks 1.3, 1.4, 6.1

**Files to modify:**
- `cli/internal/converter/mcp.go`

**What to implement:**

Kiro's MCP format is nearly identical to canonical (`mcpServers` key, same fields) but adds `autoApprove` and `disabledTools`. Add to Canonicalize switch:

```go
case "kiro":
    // Kiro uses the standard mcpServers format; canonicalize as Claude
    return c.Canonicalize(content, "claude-code")
```

Add to Render switch:

```go
case "kiro":
    return renderKiroMCP(cfg)
```

```go
// kiroServerConfig is Kiro's on-disk MCP server format.
// Similar to canonical but adds disabledTools.
type kiroServerConfig struct {
    Command       string            `json:"command,omitempty"`
    Args          []string          `json:"args,omitempty"`
    Env           map[string]string `json:"env,omitempty"`
    URL           string            `json:"url,omitempty"`
    Headers       map[string]string `json:"headers,omitempty"`
    Disabled      bool              `json:"disabled,omitempty"`
    AutoApprove   []string          `json:"autoApprove,omitempty"`
    DisabledTools []string          `json:"disabledTools,omitempty"`
}

type kiroMCPConfig struct {
    MCPServers map[string]kiroServerConfig `json:"mcpServers"`
}

func renderKiroMCP(cfg mcpConfig) (*Result, error) {
    var warnings []string
    kc := kiroMCPConfig{MCPServers: make(map[string]kiroServerConfig)}

    for name, server := range cfg.MCPServers {
        s := kiroServerConfig{
            Command:     server.Command,
            Args:        server.Args,
            Env:         server.Env,
            URL:         server.URL,
            Headers:     server.Headers,
            Disabled:    server.Disabled,
            AutoApprove: server.AutoApprove,
        }

        // Warn about dropped Gemini-specific fields
        if server.Trust != "" {
            warnings = append(warnings, fmt.Sprintf("server %q: trust dropped (Gemini-specific)", name))
        }
        if len(server.IncludeTools) > 0 {
            warnings = append(warnings, fmt.Sprintf("server %q: includeTools dropped (Gemini-specific)", name))
        }

        kc.MCPServers[name] = s
    }

    result, err := json.MarshalIndent(kc, "", "  ")
    if err != nil {
        return nil, err
    }
    return &Result{Content: result, Filename: "mcp.json", Warnings: warnings}, nil
}
```

**How to verify:** `make test` passes.

---

### Task 6.4 — Add Kiro agent renderer (JSON + file:// prompts)

**Dependencies:** Tasks 1.4, 6.1

**Files to modify:**
- `cli/internal/converter/agents.go`

**What to implement:**

Kiro agents are JSON configs that reference markdown prompt files via `file://`. When rendering to Kiro, produce two outputs: the JSON agent config and a markdown prompt file. Use `ExtraFiles` to carry the prompt file.

Add a Kiro agent JSON struct:

```go
// kiroAgentConfig is Kiro's on-disk agent format.
type kiroAgentConfig struct {
    Name             string            `json:"name"`
    Description      string            `json:"description"`
    Prompt           string            `json:"prompt"` // "file://./prompts/<name>.md"
    Model            string            `json:"model,omitempty"`
    Tools            []string          `json:"tools,omitempty"`
    AllowedTools     []string          `json:"allowedTools,omitempty"`
    IncludeMcpJSON   bool              `json:"includeMcpJson,omitempty"`
    KeyboardShortcut string            `json:"keyboardShortcut,omitempty"`
    Resources        []string          `json:"resources,omitempty"`
    MCPServers       map[string]any    `json:"mcpServers,omitempty"`
}
```

Add Kiro to the Canonicalize switch for JSON agents:

```go
case "kiro":
    return canonicalizeKiroAgent(content)
```

```go
func canonicalizeKiroAgent(content []byte) (*Result, error) {
    var ka kiroAgentConfig
    if err := json.Unmarshal(content, &ka); err != nil {
        return nil, fmt.Errorf("parsing Kiro agent JSON: %w", err)
    }

    // Translate Kiro tool names to canonical
    var tools []string
    for _, t := range ka.Tools {
        tools = append(tools, ReverseTranslateTool(t, "kiro"))
    }
    for _, t := range ka.AllowedTools {
        // allowedTools may include granular forms like "@git/git_status"
        // Extract the base tool name for canonical translation
        base := t
        if idx := strings.Index(t, "/"); idx != -1 {
            base = t[:idx]
        }
        canonical := ReverseTranslateTool(base, "kiro")
        if !containsStr(tools, canonical) {
            tools = append(tools, canonical)
        }
    }

    meta := AgentMeta{
        Name:        ka.Name,
        Description: ka.Description,
        Tools:       tools,
        Model:       ka.Model,
    }

    // Prompt body: if it's a file:// reference, the caller must resolve it.
    // Store the reference in body as a note.
    body := ""
    if strings.HasPrefix(ka.Prompt, "file://") {
        body = fmt.Sprintf("<!-- kiro:prompt-file=%q -->\n\n(Prompt body loaded from %s)", ka.Prompt, ka.Prompt)
    } else {
        body = ka.Prompt
    }

    canonical, err := buildAgentCanonical(meta, body)
    if err != nil {
        return nil, err
    }
    return &Result{Content: canonical, Filename: "agent.md"}, nil
}

func containsStr(s []string, v string) bool {
    for _, x := range s {
        if x == v {
            return true
        }
    }
    return false
}
```

Add Kiro to the Render switch:

```go
case "kiro":
    return renderKiroAgent(meta, body)
```

```go
func renderKiroAgent(meta AgentMeta, body string) (*Result, error) {
    var warnings []string
    cleanBody := StripConversionNotes(body)

    // Determine agent name for file naming
    agentName := meta.Name
    if agentName == "" {
        agentName = "agent"
    }
    promptFilename := slugify(agentName) + ".md"
    promptRef := "file://./prompts/" + promptFilename

    // Translate tools to Kiro names
    kiroTools := TranslateTools(meta.Tools, "kiro")

    ka := kiroAgentConfig{
        Name:        meta.Name,
        Description: meta.Description,
        Prompt:      promptRef,
        Tools:       kiroTools,
        Model:       meta.Model,
    }

    // Warn about unsupported fields
    if meta.MaxTurns > 0 {
        warnings = append(warnings, fmt.Sprintf("maxTurns (%d) not supported by Kiro (dropped)", meta.MaxTurns))
    }
    if meta.PermissionMode != "" {
        warnings = append(warnings, fmt.Sprintf("permissionMode (%q) not supported by Kiro (dropped)", meta.PermissionMode))
    }
    if len(meta.DisallowedTools) > 0 {
        warnings = append(warnings, "disallowedTools not supported by Kiro; consider using tool groups instead")
    }

    agentJSON, err := json.MarshalIndent(ka, "", "  ")
    if err != nil {
        return nil, err
    }

    // The prompt goes to an ExtraFile at the prompts/ path
    promptPath := "prompts/" + promptFilename

    return &Result{
        Content:  agentJSON,
        Filename: slugify(agentName) + ".json",
        Warnings: warnings,
        ExtraFiles: map[string][]byte{
            promptPath: []byte(cleanBody + "\n"),
        },
    }, nil
}
```

**How to verify:** `make test` passes.

---

### Task 6.5 — Add Kiro hooks renderer

**Dependencies:** Tasks 1.4, 6.1

**Files to modify:**
- `cli/internal/converter/hooks.go`

**What to implement:**

Per design decision D3, Kiro hooks are written to `.kiro/agents/syllago-hooks.json`. The hooks converter's `Render` method must produce a Kiro agent JSON file with a `"hooks"` section. Add Kiro hook structs and renderer:

```go
// kiroHookEntry is a single hook in Kiro's format.
type kiroHookEntry struct {
    Command     string `json:"command"`
    Matcher     string `json:"matcher,omitempty"`
    TimeoutMs   int    `json:"timeout_ms,omitempty"`
}

// kiroHooksAgent is the shape of the syllago-hooks.json file Kiro reads.
type kiroHooksAgent struct {
    Name        string                        `json:"name"`
    Description string                        `json:"description"`
    Prompt      string                        `json:"prompt"`
    Hooks       map[string][]kiroHookEntry    `json:"hooks"`
}
```

Add Kiro to the Render switch:

```go
case "kiro":
    return renderKiroHooks(cfg, mode)
```

```go
func renderKiroHooks(cfg hooksConfig, llmMode string) (*Result, error) {
    var warnings []string
    kiroHooks := make(map[string][]kiroHookEntry)

    for event, matchers := range cfg.Hooks {
        translated, supported := TranslateHookEvent(event, "kiro")
        if !supported {
            warnings = append(warnings, fmt.Sprintf("hook event %q is not supported by Kiro (dropped)", event))
            continue
        }

        for _, m := range matchers {
            // Translate matcher tool name to Kiro equivalent
            matcher := ""
            if m.Matcher != "" {
                matcher = TranslateTool(m.Matcher, "kiro")
            }

            for _, h := range m.Hooks {
                if h.Type == "prompt" || h.Type == "agent" {
                    if llmMode == LLMHooksModeGenerate {
                        // Generate a wrapper script (same as standard handler)
                        scriptName, scriptContent := generateLLMWrapperScript(h, "kiro", event, len(kiroHooks))
                        kiroHooks[translated] = append(kiroHooks[translated], kiroHookEntry{
                            Command:   "./" + scriptName,
                            Matcher:   matcher,
                            TimeoutMs: 30000,
                        })
                        warnings = append(warnings, fmt.Sprintf("LLM hook (type: %q) converted to wrapper script %s", h.Type, scriptName))
                        _ = scriptContent // caller must write ExtraFiles
                    } else {
                        warnings = append(warnings, fmt.Sprintf("LLM-evaluated hook (type: %q) dropped for kiro", h.Type))
                    }
                    continue
                }

                entry := kiroHookEntry{
                    Command:   h.Command,
                    Matcher:   matcher,
                    TimeoutMs: h.Timeout,
                }
                kiroHooks[translated] = append(kiroHooks[translated], entry)
            }
        }
    }

    agent := kiroHooksAgent{
        Name:        "syllago-hooks",
        Description: "Hooks installed by syllago",
        Prompt:      "",
        Hooks:       kiroHooks,
    }

    result, err := json.MarshalIndent(agent, "", "  ")
    if err != nil {
        return nil, err
    }
    return &Result{Content: result, Filename: "syllago-hooks.json", Warnings: warnings}, nil
}
```

**How to verify:** `make test` passes.

---

### Task 6.6 — Add Kiro skills renderer (steering files)

**Dependencies:** Task 6.1

**Files to modify:**
- `cli/internal/converter/skills.go`

**What to implement:**

Per design decision D8, skills map to `.kiro/steering/` markdown files. Kiro steering files are pure markdown with no frontmatter. Add to the Render switch:

```go
case "kiro":
    return renderKiroSkill(meta, body)
```

```go
func renderKiroSkill(meta SkillMeta, body string) (*Result, error) {
    var warnings []string
    cleanBody := StripConversionNotes(body)

    // Kiro steering files are pure markdown — embed metadata as prose if present
    var header strings.Builder
    if meta.Name != "" {
        header.WriteString("# ")
        header.WriteString(meta.Name)
        header.WriteString("\n\n")
    }
    if meta.Description != "" {
        header.WriteString(meta.Description)
        header.WriteString("\n\n")
    }

    content := header.String() + cleanBody

    // Warn about dropped fields
    if len(meta.AllowedTools) > 0 {
        warnings = append(warnings, "allowed-tools not supported in Kiro steering files (dropped)")
    }
    if meta.UserInvocable != nil {
        warnings = append(warnings, "user-invocable not supported by Kiro steering files (dropped)")
    }

    name := "skill"
    if meta.Name != "" {
        name = slugify(meta.Name)
    }

    return &Result{
        Content:  []byte(content + "\n"),
        Filename: name + ".md",
        Warnings: warnings,
    }, nil
}
```

Also add a Kiro canonicalizer for steering files (which are plain markdown):

```go
case "kiro":
    // Kiro steering files are plain markdown — treat as always-apply rules content
    return canonicalizeSkillFromMarkdown(content)
```

```go
func canonicalizeSkillFromMarkdown(content []byte) (*Result, error) {
    // Steering files are plain markdown; wrap in minimal canonical skill format
    body := strings.TrimSpace(string(content))
    meta := SkillMeta{}
    canonical, err := buildSkillCanonical(meta, body)
    if err != nil {
        return nil, err
    }
    return &Result{Content: canonical, Filename: "SKILL.md"}, nil
}
```

**How to verify:** `make test` passes.

---

### Task 6.7 — Write Kiro provider test and converter tests

**Dependencies:** Tasks 6.1, 6.3, 6.4, 6.5, 6.6

**Files to create:**
- `cli/internal/provider/kiro_test.go`

**Files to modify:**
- `cli/internal/converter/agents_test.go`
- `cli/internal/converter/hooks_test.go`
- `cli/internal/converter/skills_test.go`
- `cli/internal/converter/mcp_test.go`

**What to implement in `kiro_test.go`:**

```go
package provider

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestKiroDetect(t *testing.T) {
    dir := t.TempDir()
    if Kiro.Detect(dir) {
        t.Fatal("expected no detection in empty temp dir")
    }
    if err := os.MkdirAll(filepath.Join(dir, ".kiro"), 0755); err != nil {
        t.Fatal(err)
    }
    if !Kiro.Detect(dir) {
        t.Fatal("expected detection when ~/.kiro/ exists")
    }
}

func TestKiroSupportsType(t *testing.T) {
    for _, ct := range []catalog.ContentType{catalog.Rules, catalog.Agents, catalog.Hooks, catalog.MCP, catalog.Skills} {
        if !Kiro.SupportsType(ct) {
            t.Errorf("Kiro must support %s", ct)
        }
    }
    if Kiro.SupportsType(catalog.Commands) {
        t.Error("Kiro must not support Commands")
    }
}

func TestKiroDiscoveryPaths(t *testing.T) {
    paths := Kiro.DiscoveryPaths("/project", catalog.Rules)
    if len(paths) != 1 || paths[0] != "/project/.kiro/steering" {
        t.Errorf("unexpected rules paths: %v", paths)
    }
    paths = Kiro.DiscoveryPaths("/project", catalog.MCP)
    if len(paths) != 1 || paths[0] != "/project/.kiro/settings/mcp.json" {
        t.Errorf("unexpected MCP paths: %v", paths)
    }
    paths = Kiro.DiscoveryPaths("/project", catalog.Hooks)
    if len(paths) != 1 || paths[0] != "/project/.kiro/agents" {
        t.Errorf("unexpected hooks paths: %v", paths)
    }
}

func TestKiroInstallDir(t *testing.T) {
    if Kiro.InstallDir("/home/user", catalog.MCP) != JSONMergeSentinel {
        t.Error("Kiro MCP must return JSONMergeSentinel")
    }
    if Kiro.InstallDir("/home/user", catalog.Hooks) != JSONMergeSentinel {
        t.Error("Kiro Hooks must return JSONMergeSentinel")
    }
    agentDir := Kiro.InstallDir("/home/user", catalog.Agents)
    if agentDir != "/home/user/.kiro/agents" {
        t.Errorf("expected /home/user/.kiro/agents, got %q", agentDir)
    }
}
```

**What to implement in `agents_test.go` (append):**

```go
func TestClaudeAgentToKiro(t *testing.T) {
    input := []byte("---\nname: AWS Expert\ndescription: AWS Rust development specialist\ntools:\n  - Read\n  - Write\n  - Bash\nmodel: claude-sonnet-4\nmaxTurns: 20\n---\n\nYou are an expert in AWS and Rust development.\n")

    conv := &AgentsConverter{}
    canonical, err := conv.Canonicalize(input, "claude-code")
    if err != nil {
        t.Fatalf("Canonicalize: %v", err)
    }

    result, err := conv.Render(canonical.Content, provider.Kiro)
    if err != nil {
        t.Fatalf("Render: %v", err)
    }

    out := string(result.Content)
    // Output is JSON
    assertContains(t, out, `"name": "AWS Expert"`)
    assertContains(t, out, `"description"`)
    assertContains(t, out, `"prompt": "file://./prompts/aws-expert.md"`)
    assertContains(t, out, `"model": "claude-sonnet-4"`)
    // Tool names translated to Kiro
    assertContains(t, out, `"read"`)
    assertContains(t, out, `"fs_write"`)
    assertContains(t, out, `"shell"`)
    assertNotContains(t, out, "Read")
    assertNotContains(t, out, "Write")
    assertEqual(t, "aws-expert.json", result.Filename)

    // Prompt body goes to ExtraFiles
    if result.ExtraFiles == nil {
        t.Fatal("expected ExtraFiles with prompt file")
    }
    promptContent, ok := result.ExtraFiles["prompts/aws-expert.md"]
    if !ok {
        t.Fatal("expected prompts/aws-expert.md in ExtraFiles")
    }
    if !strings.Contains(string(promptContent), "AWS and Rust") {
        t.Error("expected prompt body in prompts/aws-expert.md")
    }

    // maxTurns should warn
    if len(result.Warnings) == 0 {
        t.Fatal("expected warning about dropped maxTurns")
    }
}

func TestKiroAgentCanonicalize(t *testing.T) {
    input := []byte(`{
        "name": "AWS Expert",
        "description": "AWS and Rust specialist",
        "prompt": "file://./prompts/aws-expert.md",
        "model": "claude-sonnet-4",
        "tools": ["read", "fs_write", "shell", "@git"]
    }`)

    conv := &AgentsConverter{}
    result, err := conv.Canonicalize(input, "kiro")
    if err != nil {
        t.Fatalf("Canonicalize: %v", err)
    }

    out := string(result.Content)
    assertContains(t, out, "name: AWS Expert")
    assertContains(t, out, "model: claude-sonnet-4")
    // Tool names should be reverse-translated to canonical
    assertContains(t, out, "Read")  // "read" → "Read"
    assertContains(t, out, "Bash")  // "shell" → "Bash"
}
```

**What to implement in `hooks_test.go` (append):**

```go
func TestClaudeHooksToKiro(t *testing.T) {
    input := []byte(`{
        "hooks": {
            "PreToolUse": [
                {
                    "matcher": "Bash",
                    "hooks": [
                        {"type": "command", "command": "echo checking", "timeout": 5000}
                    ]
                }
            ],
            "SessionStart": [
                {
                    "hooks": [
                        {"type": "command", "command": "echo starting"}
                    ]
                }
            ]
        }
    }`)

    conv := &HooksConverter{}
    canonical, err := conv.Canonicalize(input, "claude-code")
    if err != nil {
        t.Fatalf("Canonicalize: %v", err)
    }

    result, err := conv.Render(canonical.Content, provider.Kiro)
    if err != nil {
        t.Fatalf("Render: %v", err)
    }

    out := string(result.Content)
    // Output is the syllago-hooks.json agent file
    assertContains(t, out, `"name": "syllago-hooks"`)
    assertContains(t, out, `"preToolUse"`)    // translated from PreToolUse
    assertContains(t, out, `"agentSpawn"`)    // translated from SessionStart
    assertContains(t, out, "echo checking")
    assertContains(t, out, "echo starting")
    // Matcher translated: Bash → shell
    assertContains(t, out, `"matcher": "shell"`)
    assertNotContains(t, out, "PreToolUse")
    assertEqual(t, "syllago-hooks.json", result.Filename)
}

func TestKiroHooksUnsupportedEventDropped(t *testing.T) {
    input := []byte(`{
        "hooks": {
            "PreCompact": [
                {"hooks": [{"type": "command", "command": "echo compress"}]}
            ]
        }
    }`)

    conv := &HooksConverter{}
    canonical, err := conv.Canonicalize(input, "claude-code")
    if err != nil {
        t.Fatalf("Canonicalize: %v", err)
    }

    result, err := conv.Render(canonical.Content, provider.Kiro)
    if err != nil {
        t.Fatalf("Render: %v", err)
    }

    if len(result.Warnings) == 0 {
        t.Fatal("expected warning about unsupported PreCompact event")
    }
}
```

**What to implement in `skills_test.go` (append):**

```go
func TestClaudeSkillToKiro(t *testing.T) {
    input := []byte("---\nname: TypeScript Expert\ndescription: TypeScript coding assistant\nallowed-tools:\n  - Read\n  - Write\n---\n\nYou are an expert TypeScript developer.\n")

    conv := &SkillsConverter{}
    canonical, err := conv.Canonicalize(input, "claude-code")
    if err != nil {
        t.Fatalf("Canonicalize: %v", err)
    }

    result, err := conv.Render(canonical.Content, provider.Kiro)
    if err != nil {
        t.Fatalf("Render: %v", err)
    }

    out := string(result.Content)
    // Kiro steering files are pure markdown
    assertContains(t, out, "# TypeScript Expert")
    assertContains(t, out, "TypeScript developer")
    assertNotContains(t, out, "allowed-tools")
    assertEqual(t, "typescript-expert.md", result.Filename)

    // allowed-tools should warn
    if len(result.Warnings) == 0 {
        t.Fatal("expected warning about dropped allowed-tools")
    }
}

func TestKiroSkillCanonicalize(t *testing.T) {
    input := []byte("# Tech Stack\n\nNode.js 22 with TypeScript 5.7.\nPackage manager: pnpm.\n")

    conv := &SkillsConverter{}
    result, err := conv.Canonicalize(input, "kiro")
    if err != nil {
        t.Fatalf("Canonicalize: %v", err)
    }

    out := string(result.Content)
    assertContains(t, out, "Node.js 22")
    assertContains(t, out, "pnpm")
}
```

**What to implement in `mcp_test.go` (append):**

```go
func TestKiroMCPRender(t *testing.T) {
    input := []byte(`{
        "mcpServers": {
            "search": {
                "command": "npx",
                "args": ["-y", "@modelcontextprotocol/server-brave-search"],
                "env": {"BRAVE_API_KEY": "key"},
                "autoApprove": ["brave_search"]
            }
        }
    }`)

    conv := &MCPConverter{}
    canonical, err := conv.Canonicalize(input, "claude-code")
    if err != nil {
        t.Fatalf("Canonicalize: %v", err)
    }

    result, err := conv.Render(canonical.Content, provider.Kiro)
    if err != nil {
        t.Fatalf("Render: %v", err)
    }

    out := string(result.Content)
    assertContains(t, out, "mcpServers")
    assertContains(t, out, "autoApprove")
    assertContains(t, out, "brave_search")
    assertContains(t, out, "BRAVE_API_KEY")
    assertEqual(t, "mcp.json", result.Filename)
    if len(result.Warnings) > 0 {
        t.Fatalf("expected no warnings, got: %v", result.Warnings)
    }
}
```

**How to verify:** `go test ./cli/internal/provider/... -run TestKiro` and `go test ./cli/internal/converter/... -run TestKiro` pass.

---

## Final Task: Full Test Suite

**Dependencies:** All tasks above

**Command:**

```sh
make test
```

Expected: all tests pass, zero failures, zero compilation errors.

**Additional smoke check:**

```sh
make build
./cli/syllago --help
```

The help text and provider list should now include Zed, Cline, Roo Code, OpenCode, and Kiro.

---

## Summary: Files Created and Modified

### New files (providers)
| File | Task |
|------|------|
| `cli/internal/provider/zed.go` | 2.1 |
| `cli/internal/provider/cline.go` | 3.1 |
| `cli/internal/provider/roocode.go` | 4.1 |
| `cli/internal/provider/opencode.go` | 5.1 |
| `cli/internal/provider/kiro.go` | 6.1 |

### New files (utilities)
| File | Task |
|------|------|
| `cli/internal/converter/jsonc.go` | 1.2 |

### New test files
| File | Task |
|------|------|
| `cli/internal/converter/jsonc_test.go` | 1.2 |
| `cli/internal/provider/zed_test.go` | 2.5 |
| `cli/internal/provider/cline_test.go` | 3.4 |
| `cli/internal/provider/roocode_test.go` | 4.5 |
| `cli/internal/provider/opencode_test.go` | 5.4 |
| `cli/internal/provider/kiro_test.go` | 6.7 |

### Modified files (converters)
| File | Tasks |
|------|-------|
| `cli/internal/converter/mcp.go` | 1.3, 2.3, 3.3, 4.3, 5.3, 6.3 |
| `cli/internal/converter/toolmap.go` | 1.4 |
| `cli/internal/converter/agents.go` | 4.4, 5.6, 6.4 |
| `cli/internal/converter/hooks.go` | 6.5 |
| `cli/internal/converter/skills.go` | 5.7, 6.6 |
| `cli/internal/converter/commands.go` | 5.8 |
| `cli/internal/converter/rules.go` | 2.3b, 3.3b, 4.3b, 5.5, 6.2b |

### Modified files (registry)
| File | Tasks |
|------|-------|
| `cli/internal/provider/provider.go` | 1.1, 2.2, 3.2, 4.2, 5.2, 6.2 |

### Modified test files
| File | Tasks |
|------|-------|
| `cli/internal/converter/mcp_test.go` | 2.4, 3.4, 5.3, 6.7 |
| `cli/internal/converter/agents_test.go` | 4.5, 5.6, 6.7 |
| `cli/internal/converter/hooks_test.go` | 6.7 |
| `cli/internal/converter/skills_test.go` | 5.7, 6.7 |
| `cli/internal/converter/rules_test.go` | 2.3b, 3.3b, 4.3b, 5.5, 6.2b |
| `cli/internal/converter/commands_test.go` | 5.8 |

---

## Task Dependency Graph

```
1.1 (FormatJSONC)
├── 1.2 (JSONC utility)
│   └── 1.3 (mcpServerConfig extensions + OpenCode MCP)
├── 2.1 (Zed provider) → 2.2 (register) → 2.3 (Zed MCP) → 2.3b (Zed rules renderer) → 2.4 (MCP tests)
│                                                                                       └── 2.5 (provider tests)
├── 3.1 (Cline provider) → 3.2 (register) → 3.3 (Cline MCP) → 3.3b (Cline rules renderer) → 3.4 (tests)
├── 4.1 (Roo Code provider) → 4.2 (register) → 4.3 (Roo Code MCP) → 4.3b (Roo Code rules renderer)
│                           └── 4.4 (Roo Code agent renderer) → 4.5 (tests)
├── 5.1 (OpenCode provider) → 5.2 (register) → 5.3 (OpenCode MCP tests)
│                           → 5.4 (provider tests)
│                           → 5.5 (OpenCode rules renderer)
│                           → 5.6 (OpenCode agent renderer)
│                           → 5.7 (OpenCode skill renderer)
│                           └── 5.8 (OpenCode command renderer)
└── 6.1 (Kiro provider) → 6.2 (register) → 6.2b (Kiro rules renderer)
                        → 6.3 (Kiro MCP)
                        → 6.4 (Kiro agent renderer)
                        → 6.5 (Kiro hooks renderer)
                        → 6.6 (Kiro skills renderer)
                        → 6.7 (all Kiro tests)

1.4 (toolmap additions) — independent, can be done at any time
```

Phases 2 and 3 can proceed in parallel after Task 1.1. Phases 4, 5, and 6 each need 1.1 and their respective infrastructure tasks. Kiro (Phase 6) also needs 1.4 for tool name translation.
