# Native Registry Indexing - Implementation Plan

**Goal:** Allow repos with provider-native AI content to serve as syllago registries via a content index in registry.yaml.

**Architecture:** Enhance ScanNativeContent() to produce structured items, add `registry create --from-native` wizard, extend scanner to follow index mappings, update registry add validation.

**Tech Stack:** Go, Cobra CLI, BubbleTea (wizard), YAML marshaling

**Design Doc:** docs/plans/2026-03-18-native-registry-indexing-design.md

---

## Phase 1: Manifest Extension & Scanner Foundation

### Task 1: Extend Manifest struct with Items field

**Files:**
- Modify: `cli/internal/registry/registry.go` (lines 193-202)
- Create: `cli/internal/registry/registry_test.go` (new test cases)

**Depends on:** Nothing

**Success Criteria:**
- [ ] ManifestItem struct defined with Name, Type, Provider, Path, HookEvent, HookIndex, Scripts fields
- [ ] Manifest struct has Items []ManifestItem field with yaml tag
- [ ] LoadManifest correctly parses registry.yaml with items section
- [ ] LoadManifest still works for registry.yaml without items (backwards compat)
- [ ] Tests pass

---

### Step 1: Write failing test

```go
// cli/internal/registry/registry_test.go
func TestLoadManifest_WithItems(t *testing.T) {
    dir := t.TempDir()
    content := `name: test-registry
description: Test
version: 0.1.0
items:
  - name: docs-research
    type: skills
    provider: claude-code
    path: .claude/skills/docs-research
  - name: research-agent
    type: agents
    provider: claude-code
    path: .claude/agents/research-agent.md
  - name: post-edit-linter
    type: hooks
    provider: claude-code
    path: .syllago/hooks/post-edit-linter
    hookEvent: PostToolUse
    hookIndex: 0
    scripts:
      - scripts/lint.sh
`
    os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte(content), 0644)
    m, err := loadManifestFromDir(dir)
    if err != nil {
        t.Fatal(err)
    }
    if len(m.Items) != 3 {
        t.Fatalf("expected 3 items, got %d", len(m.Items))
    }
    if m.Items[0].Name != "docs-research" || m.Items[0].Type != "skills" {
        t.Errorf("first item: got name=%q type=%q", m.Items[0].Name, m.Items[0].Type)
    }
    if m.Items[2].HookEvent != "PostToolUse" || len(m.Items[2].Scripts) != 1 {
        t.Errorf("hook item: got event=%q scripts=%v", m.Items[2].HookEvent, m.Items[2].Scripts)
    }
}

func TestLoadManifest_WithoutItems(t *testing.T) {
    dir := t.TempDir()
    content := `name: classic-registry
description: No items section
version: 0.1.0
`
    os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte(content), 0644)
    m, err := loadManifestFromDir(dir)
    if err != nil {
        t.Fatal(err)
    }
    if m.Items != nil {
        t.Errorf("expected nil items, got %v", m.Items)
    }
}
```

### Step 2: Run test to verify it fails

Run: `cd cli && go test ./internal/registry/ -run TestLoadManifest_With -v`
Expected: FAIL — ManifestItem type not defined, Items field not on Manifest

### Step 3: Implement ManifestItem and extend Manifest

```go
// cli/internal/registry/registry.go — add after Manifest struct (line 202)

// ManifestItem maps a provider-native content item to its location in the repo.
// Used by registries created with --from-native to index content without
// restructuring files into syllago-native layout.
type ManifestItem struct {
    Name      string   `yaml:"name"`
    Type      string   `yaml:"type"`                  // skills, agents, rules, hooks, commands, mcp
    Provider  string   `yaml:"provider"`              // provider slug (claude-code, cursor, etc.)
    Path      string   `yaml:"path"`                  // relative to repo root
    HookEvent string   `yaml:"hookEvent,omitempty"`   // for hooks: event name (PostToolUse, etc.)
    HookIndex int      `yaml:"hookIndex,omitempty"`   // for hooks: index in event array
    Scripts   []string `yaml:"scripts,omitempty"`      // for hooks: associated script files
}

// Add to Manifest struct:
// Items []ManifestItem `yaml:"items,omitempty"`
```

Modify the Manifest struct at line 196-202 to add:
```go
type Manifest struct {
    Name              string         `yaml:"name"`
    Description       string         `yaml:"description,omitempty"`
    Maintainers       []string       `yaml:"maintainers,omitempty"`
    Version           string         `yaml:"version,omitempty"`
    MinSyllagoVersion string         `yaml:"min_syllago_version,omitempty"`
    Items             []ManifestItem `yaml:"items,omitempty"`
}
```

### Step 4: Run test to verify it passes

Run: `cd cli && go test ./internal/registry/ -run TestLoadManifest_With -v`
Expected: PASS

### Step 5: Commit

```bash
git add cli/internal/registry/registry.go cli/internal/registry/registry_test.go
git commit -m "feat: extend Manifest with Items for native content indexing"
```

---

### Task 2: Enhance ScanNativeContent to return structured items

**Files:**
- Modify: `cli/internal/catalog/native_scan.go` (lines 8-124)
- Modify: `cli/internal/catalog/native_scan_test.go`

**Depends on:** Nothing (parallel with Task 1)

**Success Criteria:**
- [ ] NativeProviderContent.ByType returns NativeItem structs (name, type, path, description) not just file paths
- [ ] Claude Code skills/agents/commands produce individual named items
- [ ] Missing MCP and hooks patterns added for project-scoped providers
- [ ] Hooks extracted from settings.json as individual items with event/index metadata
- [ ] Tests pass

---

### Step 1: Write failing tests

```go
// cli/internal/catalog/native_scan_test.go — add new tests

func TestScanNativeContent_StructuredItems(t *testing.T) {
    dir := t.TempDir()
    // Create Claude Code skills
    skillDir := filepath.Join(dir, ".claude", "skills", "docs-research")
    os.MkdirAll(skillDir, 0755)
    os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: Docs Research\ndescription: Research docs\n---\n# Skill"), 0644)

    // Create Claude Code agents
    agentDir := filepath.Join(dir, ".claude", "agents")
    os.MkdirAll(agentDir, 0755)
    os.WriteFile(filepath.Join(agentDir, "research-agent.md"), []byte("---\nname: Research Agent\n---\n# Agent"), 0644)

    result := ScanNativeContent(dir)
    if len(result.Providers) != 1 {
        t.Fatalf("expected 1 provider, got %d", len(result.Providers))
    }
    cc := result.Providers[0]

    skills := cc.Items["skills"]
    if len(skills) != 1 || skills[0].Name != "docs-research" {
        t.Errorf("skills: expected [{Name: docs-research}], got %+v", skills)
    }
    if skills[0].DisplayName != "Docs Research" {
        t.Errorf("skill display name: expected 'Docs Research', got %q", skills[0].DisplayName)
    }

    agents := cc.Items["agents"]
    if len(agents) != 1 || agents[0].Name != "research-agent" {
        t.Errorf("agents: expected [{Name: research-agent}], got %+v", agents)
    }
}

func TestScanNativeContent_ProjectScopedHooks(t *testing.T) {
    dir := t.TempDir()
    settingsDir := filepath.Join(dir, ".claude")
    os.MkdirAll(settingsDir, 0755)
    settings := `{
        "hooks": {
            "PostToolUse": [
                {"matcher": "Edit|Write", "hooks": [{"type": "command", "command": "echo lint"}]}
            ]
        }
    }`
    os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(settings), 0644)

    result := ScanNativeContent(dir)
    if len(result.Providers) == 0 {
        t.Fatal("expected providers")
    }
    var hooks []NativeItem
    for _, p := range result.Providers {
        if p.ProviderSlug == "claude-code" {
            hooks = p.Items["hooks"]
        }
    }
    if len(hooks) != 1 {
        t.Fatalf("expected 1 hook, got %d", len(hooks))
    }
    if hooks[0].HookEvent != "PostToolUse" {
        t.Errorf("hook event: expected PostToolUse, got %q", hooks[0].HookEvent)
    }
}

func TestScanNativeContent_ProjectScopedMCP(t *testing.T) {
    dir := t.TempDir()
    copilotDir := filepath.Join(dir, ".copilot")
    os.MkdirAll(copilotDir, 0755)
    mcp := `{"mcpServers": {"my-server": {"command": "node", "args": ["server.js"]}}}`
    os.WriteFile(filepath.Join(copilotDir, "mcp.json"), []byte(mcp), 0644)

    result := ScanNativeContent(dir)
    var mcpItems []NativeItem
    for _, p := range result.Providers {
        if p.ProviderSlug == "copilot-cli" {
            mcpItems = p.Items["mcp"]
        }
    }
    if len(mcpItems) != 1 || mcpItems[0].Name != "my-server" {
        t.Errorf("mcp: expected [{Name: my-server}], got %+v", mcpItems)
    }
}
```

### Step 2: Run tests to verify they fail

Run: `cd cli && go test ./internal/catalog/ -run TestScanNativeContent_Structured -v`
Expected: FAIL — NativeItem not defined, Items field not on NativeProviderContent

### Step 3: Implement structured native scanning

Add `NativeItem` struct and refactor `NativeProviderContent`:

```go
// cli/internal/catalog/native_scan.go

// NativeItem represents a single content item found in provider-native format.
type NativeItem struct {
    Name        string // item name (derived from dir/file name)
    DisplayName string // human-readable name from frontmatter
    Description string // from frontmatter or content
    Path        string // relative to repo root
    HookEvent   string // for hooks: which event (PostToolUse, etc.)
    HookIndex   int    // for hooks: index within the event array
}

// NativeProviderContent holds found content for one provider.
type NativeProviderContent struct {
    ProviderSlug string
    ProviderName string
    Items        map[string][]NativeItem // type label -> structured items
}
```

Update `providerNativePatterns()` to add project-scoped MCP and hooks:

```go
// Add to providerNativePatterns():
// Claude Code hooks (project-scoped settings.json)
{providerSlug: "claude-code", providerName: "Claude Code", path: ".claude/settings.json", typeLabel: "hooks", embedded: true},
// Copilot CLI
{providerSlug: "copilot-cli", providerName: "Copilot CLI", path: ".copilot/mcp.json", typeLabel: "mcp", embedded: true},
{providerSlug: "copilot-cli", providerName: "Copilot CLI", path: ".copilot/hooks.json", typeLabel: "hooks", embedded: true},
// Cline
{providerSlug: "cline", providerName: "Cline", path: ".vscode/mcp.json", typeLabel: "mcp", embedded: true},
// Roo Code
{providerSlug: "roo-code", providerName: "Roo Code", path: ".roo/mcp.json", typeLabel: "mcp", embedded: true},
// Kiro
{providerSlug: "kiro", providerName: "Kiro", path: ".kiro/settings/mcp.json", typeLabel: "mcp", embedded: true},
// OpenCode
{providerSlug: "opencode", providerName: "OpenCode", path: "opencode.json", typeLabel: "mcp", embedded: true},
```

Add `embedded` field to `nativePattern`:
```go
type nativePattern struct {
    providerSlug string
    providerName string
    path         string
    typeLabel    string
    embedded     bool   // true for JSON files with embedded content (hooks in settings, MCP configs)
}
```

Refactor `ScanNativeContent()` to produce `NativeItem` structs:
- For directories (skills, agents, commands): iterate entries, create NativeItem per entry with name from dir/file name
- For single files (CLAUDE.md, .cursorrules): create one NativeItem
- For embedded JSON (settings.json hooks, mcp.json): parse JSON, extract individual items
- For skills: parse SKILL.md frontmatter for display name + description
- For agents: parse AGENT.md frontmatter
- For hooks: parse hooks object, create one NativeItem per event+index combination
- For MCP: parse mcpServers object, create one NativeItem per server name

### Step 4: Run tests to verify they pass

Run: `cd cli && go test ./internal/catalog/ -run TestScanNativeContent -v`
Expected: PASS

### Step 5: Commit

```bash
git add cli/internal/catalog/native_scan.go cli/internal/catalog/native_scan_test.go
git commit -m "feat: structured native content scanning with MCP/hooks support"
```

---

## Phase 2: Scanner Index Support

### Task 3: Add scanFromIndex to scanner

**Files:**
- Modify: `cli/internal/catalog/scanner.go` (add scanFromIndex function, modify scanRoot)
- Create: `cli/internal/catalog/scanner_index_test.go`

**Depends on:** Task 1 (ManifestItem struct)

**Success Criteria:**
- [ ] scanFromIndex builds ContentItem entries from ManifestItem paths
- [ ] Frontmatter parsing works for skills/agents pointed to by index
- [ ] collectFiles() works for directory items in index
- [ ] Hook items from index are properly typed
- [ ] scanRoot checks for manifest items before falling back to directory walk
- [ ] Existing syllago-native registries still scan correctly
- [ ] Tests pass

---

### Step 1: Write failing test

```go
// cli/internal/catalog/scanner_index_test.go
package catalog

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/registry"
)

func TestScanFromIndex_Skills(t *testing.T) {
    dir := t.TempDir()

    // Create native skill structure
    skillDir := filepath.Join(dir, ".claude", "skills", "my-skill")
    os.MkdirAll(skillDir, 0755)
    os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: My Skill\ndescription: Does things\n---\n# Instructions"), 0644)
    os.WriteFile(filepath.Join(skillDir, "helper.md"), []byte("# Helper"), 0644)

    // Create registry.yaml with items
    manifest := `name: test
items:
  - name: my-skill
    type: skills
    provider: claude-code
    path: .claude/skills/my-skill
`
    os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte(manifest), 0644)

    cat := &Catalog{RepoRoot: dir}
    if err := scanRoot(cat, dir, false); err != nil {
        t.Fatal(err)
    }

    if len(cat.Items) != 1 {
        t.Fatalf("expected 1 item, got %d", len(cat.Items))
    }
    item := cat.Items[0]
    if item.Name != "my-skill" {
        t.Errorf("name: got %q", item.Name)
    }
    if item.Type != Skills {
        t.Errorf("type: got %q", item.Type)
    }
    if item.DisplayName != "My Skill" {
        t.Errorf("display name: got %q", item.DisplayName)
    }
    if item.Description != "Does things" {
        t.Errorf("description: got %q", item.Description)
    }
    if len(item.Files) != 2 {
        t.Errorf("files: expected 2, got %d: %v", len(item.Files), item.Files)
    }
}

func TestScanFromIndex_FallbackToNativeLayout(t *testing.T) {
    dir := t.TempDir()

    // Create syllago-native skill (no registry.yaml items)
    skillDir := filepath.Join(dir, "skills", "hello")
    os.MkdirAll(skillDir, 0755)
    os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: Hello\n---\n"), 0644)

    // registry.yaml without items
    os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte("name: classic\n"), 0644)

    cat := &Catalog{RepoRoot: dir}
    if err := scanRoot(cat, dir, false); err != nil {
        t.Fatal(err)
    }

    if len(cat.Items) != 1 || cat.Items[0].Name != "hello" {
        t.Fatalf("expected fallback to native layout, got %+v", cat.Items)
    }
}
```

### Step 2: Run test to verify it fails

Run: `cd cli && go test ./internal/catalog/ -run TestScanFromIndex -v`
Expected: FAIL — scanFromIndex not yet called from scanRoot

### Step 3: Implement scanFromIndex and integrate into scanRoot

```go
// cli/internal/catalog/scanner.go — add new function

// scanFromIndex builds ContentItems from registry.yaml ManifestItem entries.
// Each item's path is resolved relative to baseDir. Metadata extraction
// (frontmatter, file listing) uses the same logic as the native scanner.
func scanFromIndex(cat *Catalog, baseDir string, items []registry.ManifestItem, local bool) error {
    for _, mi := range items {
        itemPath := filepath.Join(baseDir, mi.Path)
        ct := ContentType(mi.Type)

        info, err := os.Stat(itemPath)
        if err != nil {
            cat.Warnings = append(cat.Warnings, fmt.Sprintf("index item %q: path %q not found, skipping", mi.Name, mi.Path))
            continue
        }

        item := ContentItem{
            Name:     mi.Name,
            Type:     ct,
            Path:     itemPath,
            Provider: mi.Provider,
            Library:  local,
        }

        if info.IsDir() {
            // Extract metadata from directory items
            switch ct {
            case Skills:
                data, readErr := os.ReadFile(filepath.Join(itemPath, "SKILL.md"))
                if readErr == nil {
                    fm, fmErr := ParseFrontmatter(data)
                    if fmErr == nil {
                        if fm.Name != "" {
                            item.DisplayName = fm.Name
                        }
                        item.Description = fm.Description
                    }
                }
            case Agents:
                data, readErr := os.ReadFile(filepath.Join(itemPath, "AGENT.md"))
                if readErr == nil {
                    fm, fmErr := ParseFrontmatter(data)
                    if fmErr == nil {
                        if fm.Name != "" {
                            item.DisplayName = fm.Name
                        }
                        item.Description = fm.Description
                    }
                }
            case Hooks:
                // Look for hook.json in the directory
                hookPath := filepath.Join(itemPath, "hook.json")
                data, readErr := os.ReadFile(hookPath)
                if readErr == nil {
                    item.Description = describeHookJSON(data)
                }
            case Commands:
                // Look for command.md or any .md file
                item.Description = readDescription(filepath.Join(itemPath, "command.md"))
                if item.Description == "" {
                    entries, _ := os.ReadDir(itemPath)
                    for _, e := range entries {
                        if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
                            item.Description = readDescription(filepath.Join(itemPath, e.Name()))
                            break
                        }
                    }
                }
            }
            item.Files = collectFiles(itemPath, itemPath)
        } else {
            // Single file item
            switch ct {
            case Agents:
                data, readErr := os.ReadFile(itemPath)
                if readErr == nil {
                    fm, fmErr := ParseFrontmatter(data)
                    if fmErr == nil {
                        if fm.Name != "" {
                            item.DisplayName = fm.Name
                        }
                        item.Description = fm.Description
                    }
                }
            case Rules:
                item.Description = readDescription(itemPath)
            case Commands:
                item.Description = readDescription(itemPath)
            case Hooks:
                data, readErr := os.ReadFile(itemPath)
                if readErr == nil {
                    item.Description = describeHookJSON(data)
                }
            }
            item.Files = []string{filepath.Base(itemPath)}
        }

        // Load metadata if present
        if info.IsDir() {
            meta, metaErr := metadata.Load(itemPath)
            if metaErr == nil {
                item.Meta = meta
            }
        }

        cat.Items = append(cat.Items, item)
    }
    return nil
}
```

Modify `scanRoot()` at the top to check for manifest items:

```go
// cli/internal/catalog/scanner.go — modify scanRoot (line 122)
func scanRoot(cat *Catalog, baseDir string, local bool) error {
    // Check for registry.yaml with indexed items
    m, _ := registry.LoadManifestFromDir(baseDir)
    if m != nil && len(m.Items) > 0 {
        return scanFromIndex(cat, baseDir, m.Items, local)
    }

    // Existing: walk syllago-native layout
    for _, ct := range AllContentTypes() {
        // ... existing code ...
    }
    return nil
}
```

Note: `loadManifestFromDir` is currently unexported. Need to export it or add a wrapper.

### Step 4: Export loadManifestFromDir

```go
// cli/internal/registry/registry.go — rename loadManifestFromDir to LoadManifestFromDir
// Update all internal callers (LoadManifest uses it)
```

### Step 5: Run tests

Run: `cd cli && go test ./internal/catalog/ -run TestScanFromIndex -v`
Expected: PASS

Run: `cd cli && go test ./internal/catalog/ -v`
Expected: ALL PASS (existing tests still work)

### Step 6: Commit

```bash
git add cli/internal/catalog/scanner.go cli/internal/catalog/scanner_index_test.go cli/internal/registry/registry.go
git commit -m "feat: scanFromIndex follows registry.yaml item mappings"
```

---

## Phase 3: Registry Add Validation Update

### Task 4: Accept indexed repos in registry add

**Files:**
- Modify: `cli/internal/tui/app.go` (registry add validation, around line 317-322)
- Modify: `cli/cmd/syllago/registry_cmd.go` (CLI add validation)

**Depends on:** Task 1 (Manifest with Items), Task 3 (scanFromIndex)

**Success Criteria:**
- [ ] Registry add accepts repos with registry.yaml containing items[] (even without syllago dirs)
- [ ] Registry add still rejects repos with no registry.yaml and no syllago structure
- [ ] Registry add still rejects repos with only provider-native content and no index
- [ ] TUI and CLI paths both updated

---

### Step 1: Update TUI validation

In `cli/internal/tui/app.go`, the registry add flow (around line 317-322) does:
```go
scanResult := catalog.ScanNativeContent(dir)
if !scanResult.HasSyllagoStructure && len(scanResult.Providers) > 0 {
    return registryAddNonSyllagoMsg{...}
}
```

Add a check for registry.yaml with items before rejecting:
```go
scanResult := catalog.ScanNativeContent(dir)
if !scanResult.HasSyllagoStructure && len(scanResult.Providers) > 0 {
    // Check if repo has a registry.yaml with indexed items
    m, _ := registry.LoadManifestFromDir(dir)
    if m != nil && len(m.Items) > 0 {
        // Indexed native repo — treat as valid
    } else {
        return registryAddNonSyllagoMsg{...}
    }
}
```

### Step 2: Update CLI validation

Check `registry_cmd.go` for similar validation in the CLI `add` command and apply the same check.

### Step 3: Test manually

```bash
make build
# Create a test repo with registry.yaml items
# syllago registry add /path/to/test-repo
# Verify items appear
```

### Step 4: Commit

```bash
git add cli/internal/tui/app.go cli/cmd/syllago/registry_cmd.go
git commit -m "feat: accept indexed native repos in registry add"
```

---

## Phase 4: Registry Create --from-native Command

### Task 5: Split registry create into --new and --from-native

**Files:**
- Modify: `cli/cmd/syllago/registry_cmd.go` (lines 460-549)

**Depends on:** Nothing (command wiring only)

**Success Criteria:**
- [ ] `registry create --new <name>` calls existing Scaffold() (renamed from bare `create <name>`)
- [ ] `registry create --from-native` triggers native indexing flow
- [ ] Running `registry create` with no flags shows help explaining both modes
- [ ] --description flag works with both modes

---

### Step 1: Add flags and routing

```go
// cli/cmd/syllago/registry_cmd.go — modify registryCreateCmd

var registryCreateCmd = &cobra.Command{
    Use:   "create",
    Short: "Create a new registry (scaffold or from native content)",
    Long: `Create a new registry in one of two modes:

  --new <name>          Scaffold an empty registry directory structure
  --from-native         Index provider-native content in the current repo

Examples:
  syllago registry create --new my-rules
  syllago registry create --from-native`,
    RunE: func(cmd *cobra.Command, args []string) error {
        newName, _ := cmd.Flags().GetString("new")
        fromNative, _ := cmd.Flags().GetBool("from-native")

        if newName != "" && fromNative {
            return fmt.Errorf("cannot use --new and --from-native together")
        }
        if newName != "" {
            // Existing scaffold path
            return runRegistryCreateNew(cmd, newName)
        }
        if fromNative {
            return runRegistryCreateFromNative(cmd)
        }
        return cmd.Help()
    },
}

// registryCreateCmd.Flags():
// registryCreateCmd.Flags().String("new", "", "Name for new scaffolded registry")
// registryCreateCmd.Flags().Bool("from-native", false, "Index native AI content in current repo")
// registryCreateCmd.Flags().String("description", "", "Short description of the registry")
// registryCreateCmd.Flags().Bool("no-git", false, "Skip git init and initial commit")
```

Extract existing create logic into `runRegistryCreateNew()`.

### Step 2: Test

```bash
make build
syllago registry create --help
syllago registry create --new test-empty --description "Test"
```

### Step 3: Commit

```bash
git add cli/cmd/syllago/registry_cmd.go
git commit -m "feat: split registry create into --new and --from-native modes"
```

---

### Task 6: Implement --from-native wizard

**Files:**
- Create: `cli/cmd/syllago/registry_create_native.go`

**Depends on:** Task 2 (enhanced ScanNativeContent), Task 5 (command routing)

**Success Criteria:**
- [ ] Wizard scans current directory for native content
- [ ] Displays discovered content grouped by provider
- [ ] Offers selection modes: all, by provider, individual items
- [ ] Prompts for registry name and description
- [ ] Generates registry.yaml with items section
- [ ] Shows summary of what was indexed

---

### Step 1: Implement runRegistryCreateFromNative

```go
// cli/cmd/syllago/registry_create_native.go
package main

import (
    "bufio"
    "fmt"
    "os"
    "path/filepath"
    "strconv"
    "strings"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
    "github.com/OpenScribbler/syllago/cli/internal/output"
    "github.com/OpenScribbler/syllago/cli/internal/registry"
    "github.com/spf13/cobra"
    "gopkg.in/yaml.v3"
)

func runRegistryCreateFromNative(cmd *cobra.Command) error {
    cwd, err := os.Getwd()
    if err != nil {
        return fmt.Errorf("getting working directory: %w", err)
    }

    // Check for existing registry.yaml
    if _, err := os.Stat(filepath.Join(cwd, "registry.yaml")); err == nil {
        return fmt.Errorf("registry.yaml already exists in this directory")
    }

    // Scan for native content
    result := catalog.ScanNativeContent(cwd)
    if result.HasSyllagoStructure {
        return fmt.Errorf("this directory already has syllago structure — use 'registry create --new' instead")
    }
    if len(result.Providers) == 0 {
        return fmt.Errorf("no AI coding tool content found in this directory")
    }

    // Display discovered content
    fmt.Fprintln(output.Writer, "Scanning for AI coding tool content...\n")
    totalItems := 0
    for _, prov := range result.Providers {
        fmt.Fprintf(output.Writer, "  %s\n", prov.ProviderName)
        for typeLabel, items := range prov.Items {
            fmt.Fprintf(output.Writer, "    %3d %-10s\n", len(items), typeLabel)
            totalItems += len(items)
        }
        fmt.Fprintln(output.Writer)
    }

    if totalItems == 0 {
        return fmt.Errorf("no indexable items found")
    }

    // Selection mode
    scanner := bufio.NewScanner(os.Stdin)
    fmt.Fprintln(output.Writer, "How would you like to index this content?\n")
    fmt.Fprintln(output.Writer, "  1) All content from all providers")
    fmt.Fprintln(output.Writer, "  2) Select by provider")
    fmt.Fprintln(output.Writer, "  3) Select individual items")
    fmt.Fprintf(output.Writer, "\nChoice [1]: ")

    var selectedItems []registry.ManifestItem
    choice := "1"
    if scanner.Scan() {
        text := strings.TrimSpace(scanner.Text())
        if text != "" {
            choice = text
        }
    }

    switch choice {
    case "1":
        selectedItems = allItemsFromScan(result)
    case "2":
        selectedItems, err = selectByProvider(result, scanner)
        if err != nil {
            return err
        }
    case "3":
        selectedItems, err = selectIndividualItems(result, scanner)
        if err != nil {
            return err
        }
    default:
        return fmt.Errorf("invalid choice: %s", choice)
    }

    if len(selectedItems) == 0 {
        return fmt.Errorf("no items selected")
    }

    // Registry metadata
    desc, _ := cmd.Flags().GetString("description")
    repoName := filepath.Base(cwd)

    fmt.Fprintf(output.Writer, "\nRegistry name [%s]: ", repoName)
    if scanner.Scan() {
        text := strings.TrimSpace(scanner.Text())
        if text != "" {
            repoName = text
        }
    }

    if desc == "" {
        fmt.Fprintf(output.Writer, "Description (optional): ")
        if scanner.Scan() {
            desc = strings.TrimSpace(scanner.Text())
        }
    }

    // Generate registry.yaml
    manifest := registry.Manifest{
        Name:        repoName,
        Description: desc,
        Version:     "0.1.0",
        Items:       selectedItems,
    }

    data, err := yaml.Marshal(&manifest)
    if err != nil {
        return fmt.Errorf("marshaling registry.yaml: %w", err)
    }

    if err := os.WriteFile(filepath.Join(cwd, "registry.yaml"), data, 0644); err != nil {
        return fmt.Errorf("writing registry.yaml: %w", err)
    }

    // Summary
    fmt.Fprintf(output.Writer, "\nGenerated registry.yaml\n\n")
    fmt.Fprintf(output.Writer, "  name: %s\n", repoName)
    fmt.Fprintf(output.Writer, "  %d items indexed\n", len(selectedItems))
    fmt.Fprintf(output.Writer, "\nThis repo can now be added as a registry:\n")
    fmt.Fprintf(output.Writer, "  syllago registry add <url-to-this-repo>\n")

    return nil
}

// allItemsFromScan converts all scanned native content to ManifestItems.
func allItemsFromScan(result catalog.NativeScanResult) []registry.ManifestItem {
    var items []registry.ManifestItem
    for _, prov := range result.Providers {
        for typeLabel, nativeItems := range prov.Items {
            for _, ni := range nativeItems {
                mi := registry.ManifestItem{
                    Name:     ni.Name,
                    Type:     typeLabel,
                    Provider: prov.ProviderSlug,
                    Path:     ni.Path,
                }
                if ni.HookEvent != "" {
                    mi.HookEvent = ni.HookEvent
                    mi.HookIndex = ni.HookIndex
                }
                items = append(items, mi)
            }
        }
    }
    return items
}

// selectByProvider presents provider selection and returns items from chosen providers.
func selectByProvider(result catalog.NativeScanResult, scanner *bufio.Scanner) ([]registry.ManifestItem, error) {
    fmt.Fprintln(output.Writer, "\nSelect providers (comma-separated numbers):\n")
    for i, prov := range result.Providers {
        count := 0
        for _, items := range prov.Items {
            count += len(items)
        }
        fmt.Fprintf(output.Writer, "  %d) %s (%d items)\n", i+1, prov.ProviderName, count)
    }
    fmt.Fprintf(output.Writer, "\nProviders: ")

    if !scanner.Scan() {
        return nil, fmt.Errorf("no selection made")
    }

    var selected []registry.ManifestItem
    for _, part := range strings.Split(scanner.Text(), ",") {
        idx, err := strconv.Atoi(strings.TrimSpace(part))
        if err != nil || idx < 1 || idx > len(result.Providers) {
            continue
        }
        prov := result.Providers[idx-1]
        for typeLabel, nativeItems := range prov.Items {
            for _, ni := range nativeItems {
                mi := registry.ManifestItem{
                    Name:     ni.Name,
                    Type:     typeLabel,
                    Provider: prov.ProviderSlug,
                    Path:     ni.Path,
                }
                if ni.HookEvent != "" {
                    mi.HookEvent = ni.HookEvent
                    mi.HookIndex = ni.HookIndex
                }
                selected = append(selected, mi)
            }
        }
    }
    return selected, nil
}

// selectIndividualItems presents all items for individual selection.
func selectIndividualItems(result catalog.NativeScanResult, scanner *bufio.Scanner) ([]registry.ManifestItem, error) {
    type numberedItem struct {
        prov      catalog.NativeProviderContent
        typeLabel string
        item      catalog.NativeItem
    }

    var all []numberedItem
    for _, prov := range result.Providers {
        for typeLabel, items := range prov.Items {
            for _, item := range items {
                all = append(all, numberedItem{prov, typeLabel, item})
            }
        }
    }

    fmt.Fprintln(output.Writer, "\nAvailable items:\n")
    for i, ni := range all {
        desc := ni.item.Name
        if ni.item.DisplayName != "" {
            desc = ni.item.DisplayName
        }
        fmt.Fprintf(output.Writer, "  %3d) [%s/%s] %s\n", i+1, ni.prov.ProviderSlug, ni.typeLabel, desc)
    }

    fmt.Fprintf(output.Writer, "\nSelect items (comma-separated numbers, or 'all'): ")
    if !scanner.Scan() {
        return nil, fmt.Errorf("no selection made")
    }

    text := strings.TrimSpace(scanner.Text())
    var selected []registry.ManifestItem

    if text == "all" {
        return allItemsFromScan(result), nil
    }

    for _, part := range strings.Split(text, ",") {
        idx, err := strconv.Atoi(strings.TrimSpace(part))
        if err != nil || idx < 1 || idx > len(all) {
            continue
        }
        ni := all[idx-1]
        mi := registry.ManifestItem{
            Name:     ni.item.Name,
            Type:     ni.typeLabel,
            Provider: ni.prov.ProviderSlug,
            Path:     ni.item.Path,
        }
        if ni.item.HookEvent != "" {
            mi.HookEvent = ni.item.HookEvent
            mi.HookIndex = ni.item.HookIndex
        }
        selected = append(selected, mi)
    }
    return selected, nil
}
```

### Step 2: Test

```bash
make build
cd ~/.local/src/aembit_docs_astro
syllago registry create --from-native
```

### Step 3: Commit

```bash
git add cli/cmd/syllago/registry_create_native.go cli/cmd/syllago/registry_cmd.go
git commit -m "feat: registry create --from-native interactive wizard"
```

---

## Phase 5: User-Scoped Hook Extraction

### Task 7: Scan user settings and extract hooks to .syllago/hooks/

**Files:**
- Modify: `cli/cmd/syllago/registry_create_native.go` (add user-scoped hook scanning)
- Create: `cli/internal/registry/extract_hooks.go`

**Depends on:** Task 6 (wizard flow)

**Success Criteria:**
- [ ] Wizard prompts to scan user-scoped settings files
- [ ] Individual hooks listed for selection
- [ ] Selected hooks extracted to .syllago/hooks/<name>/hook.json
- [ ] Script files copied alongside if they exist in-repo
- [ ] Security warning displayed before copying
- [ ] Scripts outside repo flagged with warning

---

### Step 1: Implement hook extraction

```go
// cli/internal/registry/extract_hooks.go
package registry

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "github.com/tidwall/gjson"
)

// UserScopedHook represents a single hook extracted from a user settings file.
type UserScopedHook struct {
    Name        string // derived: event-matcher or event-index
    Event       string
    Index       int
    Definition  json.RawMessage // the matcher group JSON
    Command     string          // first command (for display)
    ScriptPath  string          // if command references a script file
    ScriptInRepo bool           // whether the script exists in the repo
}

// ScanUserHooks reads a provider settings file and extracts individual hooks.
func ScanUserHooks(settingsPath string, repoRoot string) ([]UserScopedHook, error) {
    data, err := os.ReadFile(settingsPath)
    if err != nil {
        return nil, fmt.Errorf("reading %s: %w", settingsPath, err)
    }

    hooksObj := gjson.GetBytes(data, "hooks")
    if !hooksObj.Exists() || !hooksObj.IsObject() {
        return nil, nil // no hooks
    }

    var hooks []UserScopedHook
    hooksObj.ForEach(func(event, entries gjson.Result) bool {
        if !entries.IsArray() {
            return true
        }
        entries.ForEach(func(_, entry gjson.Result) bool {
            idx := len(hooks)
            matcher := entry.Get("matcher").String()

            name := strings.ToLower(event.String())
            if matcher != "" {
                name += "-" + strings.ToLower(strings.ReplaceAll(matcher, "|", "-"))
            } else {
                name += fmt.Sprintf("-%d", idx)
            }

            cmd := entry.Get("hooks.0.command").String()

            h := UserScopedHook{
                Name:       name,
                Event:      event.String(),
                Index:      idx,
                Definition: []byte(entry.Raw),
                Command:    cmd,
            }

            // Check if command references a script file
            if cmd != "" && !strings.Contains(cmd, " ") {
                // Single token — might be a script path
                if filepath.IsAbs(cmd) {
                    h.ScriptPath = cmd
                    rel, err := filepath.Rel(repoRoot, cmd)
                    h.ScriptInRepo = err == nil && !strings.HasPrefix(rel, "..")
                } else if strings.HasPrefix(cmd, "./") || strings.HasPrefix(cmd, "../") {
                    h.ScriptPath = filepath.Join(repoRoot, cmd)
                    h.ScriptInRepo = true
                }
            }

            hooks = append(hooks, h)
            return true
        })
        return true
    })

    return hooks, nil
}

// ExtractHooksToDir copies selected user-scoped hooks into targetDir.
// Creates targetDir/<hook-name>/hook.json for each hook.
// Copies script files if they're in the repo.
func ExtractHooksToDir(hooks []UserScopedHook, targetDir string, repoRoot string) error {
    for _, h := range hooks {
        hookDir := filepath.Join(targetDir, h.Name)
        if err := os.MkdirAll(hookDir, 0755); err != nil {
            return fmt.Errorf("creating hook dir %s: %w", hookDir, err)
        }

        // Build flat-format hook.json
        hookJSON := map[string]interface{}{
            "event": h.Event,
        }
        // Parse the definition to extract matcher and hooks array
        var def map[string]interface{}
        if err := json.Unmarshal(h.Definition, &def); err == nil {
            if m, ok := def["matcher"]; ok {
                hookJSON["matcher"] = m
            }
            if hks, ok := def["hooks"]; ok {
                hookJSON["hooks"] = hks
            }
        }

        data, err := json.MarshalIndent(hookJSON, "", "  ")
        if err != nil {
            return fmt.Errorf("marshaling hook %s: %w", h.Name, err)
        }
        if err := os.WriteFile(filepath.Join(hookDir, "hook.json"), data, 0644); err != nil {
            return fmt.Errorf("writing hook %s: %w", h.Name, err)
        }

        // Copy script if in-repo
        if h.ScriptPath != "" && h.ScriptInRepo {
            scriptData, err := os.ReadFile(h.ScriptPath)
            if err == nil {
                scriptDest := filepath.Join(hookDir, filepath.Base(h.ScriptPath))
                if err := os.WriteFile(scriptDest, scriptData, 0755); err != nil {
                    return fmt.Errorf("copying script for %s: %w", h.Name, err)
                }
            }
        }
    }
    return nil
}
```

### Step 2: Add user-scoped hook prompts to wizard

Add after the selection step in `runRegistryCreateFromNative()`:
- Prompt to scan user settings
- List hooks for selection
- Show security warning
- Call ExtractHooksToDir
- Add extracted hooks to selectedItems with path pointing to .syllago/hooks/<name>

### Step 3: Test

```bash
make build
cd ~/.local/src/aembit_docs_astro
syllago registry create --from-native
# Choose to scan user settings
# Verify .syllago/hooks/ created with extracted hooks
```

### Step 4: Commit

```bash
git add cli/internal/registry/extract_hooks.go cli/cmd/syllago/registry_create_native.go
git commit -m "feat: user-scoped hook extraction with security warnings"
```

---

## Phase 6: End-to-End Integration

### Task 8: Integration test with aembit_docs_astro

**Files:**
- Create: `cli/internal/registry/integration_native_test.go`

**Depends on:** Tasks 1-7

**Success Criteria:**
- [ ] Full round-trip: create --from-native → registry add → items list → install
- [ ] Existing syllago-native registries unaffected
- [ ] Mixed registries (some indexed, some native) work together

---

### Step 1: Write integration test

```go
// cli/internal/registry/integration_native_test.go
func TestNativeRegistryRoundTrip(t *testing.T) {
    // 1. Create temp dir with Claude Code native structure
    dir := t.TempDir()
    // Create .claude/skills/test-skill/SKILL.md
    // Create .claude/agents/test-agent.md
    // Create .claude/commands/test-cmd.md

    // 2. Generate registry.yaml with items
    result := catalog.ScanNativeContent(dir)
    items := allItemsFromScan(result) // need to export or move helper
    manifest := registry.Manifest{Name: "test", Items: items}
    // Write registry.yaml

    // 3. Scan as registry
    sources := []catalog.RegistrySource{{Name: "test-native", Path: dir}}
    cat, err := catalog.ScanRegistriesOnly(sources)
    if err != nil {
        t.Fatal(err)
    }

    // 4. Verify items found
    if len(cat.Items) != 3 {
        t.Fatalf("expected 3 items, got %d", len(cat.Items))
    }
    for _, item := range cat.Items {
        if item.Registry != "test-native" {
            t.Errorf("item %q: registry=%q, want test-native", item.Name, item.Registry)
        }
    }
}
```

### Step 2: Run

```bash
cd cli && go test ./internal/registry/ -run TestNativeRegistryRoundTrip -v
```

### Step 3: Manual end-to-end test

```bash
make build
cd ~/.local/src/aembit_docs_astro
syllago registry create --from-native
# Select all, name it "aembit-docs"

cd ~/.local/src/syllago
syllago registry add ~/.local/src/aembit_docs_astro --name aembit-docs
syllago registry items aembit-docs
# Verify all items appear

syllago  # Launch TUI, browse registry, verify items visible
```

### Step 4: Commit

```bash
git add cli/internal/registry/integration_native_test.go
git commit -m "test: native registry round-trip integration test"
```

---

## Phase 7: Build and Final Verification

### Task 9: Build, run full test suite, manual verification

**Depends on:** All previous tasks

**Success Criteria:**
- [ ] `make build` succeeds
- [ ] `make test` passes (all existing + new tests)
- [ ] Manual test with aembit_docs_astro succeeds
- [ ] Existing registry create --new still works
- [ ] TUI golden files updated if needed

---

### Step 1: Build and test

```bash
cd cli && make build && make test
```

### Step 2: Update golden files if needed

```bash
cd cli && go test ./internal/tui/ -update-golden
```

### Step 3: Final commit

```bash
git add -A
git commit -m "chore: update golden files for native registry indexing"
```
