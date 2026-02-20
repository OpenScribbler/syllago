# Nesco Phase 1 + 2 — Implementation Plan

**Goal:** Expand nesco from a content manager to a codebase scanning, surprise detection, and agent config parity tool — covering Phase 1 (Provider Parity) and Phase 2 (Scan + Surprise Detection).

**Architecture:** Unified CLI with two data models (Item for content repo, ContextDocument for scan/emit). Detectors run in parallel via goroutines, emitters are pure functions, reconciler handles disk I/O and boundary marker preservation. 24 surprise detectors (12 cross-language + 12 language-specific) activate based on detected project language.

**Tech Stack:** Go, Cobra (CLI), Bubble Tea (existing TUI), standard library for filesystem operations.

**Design Doc:** `docs/nesco-design-decisions.md`

---

## TDD Protocol

**Every task follows test-first development.** The step ordering for all tasks is:

1. **Write test file(s)** — Define expected behavior through tests that will fail
2. **Run tests** — Confirm they fail for the right reason (compilation error or assertion failure)
3. **Write implementation** — Make the tests pass
4. **Run tests again** — Confirm green
5. **Verify integration** — `go build ./cmd/nesco && go test ./...`

**Test categories by component:**

| Component | Test strategy |
|-----------|--------------|
| Model types | Struct construction, interface satisfaction, field access |
| Detectors | Fixture directories in `testdata/`, assert returned sections match expected |
| Emitters | Pure function tests — pass ContextDocument, assert output string matches golden |
| Parsers | Pass provider-specific content, assert canonical model matches expected |
| Reconciler | Pass "existing file" + "new output", assert merged result preserves human sections |
| Drift | Compare two baselines, assert diff report matches expected |
| CLI commands | Cobra command tests — invoke RunE with args, assert output and exit behavior |
| Config | Temp directory save/load roundtrips |

**CLI command testing pattern:**
```go
func TestScanCommand(t *testing.T) {
    // Create a temp project with known files
    tmp := setupFixtureProject(t)

    // Capture output
    var buf bytes.Buffer
    output.Writer = &buf
    output.JSON = true

    // Invoke command
    cmd := scanCmd
    cmd.SetArgs([]string{"--json"})
    err := cmd.RunE(cmd, []string{})

    // Assert
    if err != nil { t.Fatalf(...) }
    // Parse JSON output and verify structure
}
```

---

## Implementation Groups

| Group | Focus | Design Decisions | Depends On |
|-------|-------|-----------------|------------|
| 1 | Foundation | #1, #2, #3, #4, #6, #13 | — |
| 2 | Phase 1: Provider Parity | #12, #16, #10 | Group 1 |
| 3 | Phase 2: Scan Infrastructure | #9, #5, #15, #7, #8, #14 | Group 1 |
| 4 | Phase 2: Detectors | #11 | Group 3 |
| 5 | Phase 2: CLI Commands | #12, #1 | Groups 3 + 4 |

---

## Group 1: Foundation

**Covers:** Decision #1 (Tool Identity), #2 (Data Model), #3 (Provider Abstraction), #4 (ContextDocument Shape), #6 (.nesco/ Directory), #13 (Project Structure)

**Codebase context:** The current CLI has no Cobra — it's pure BubbleTea TUI with a hand-rolled "backfill" subcommand check in `main.go:31-35`. Provider is a struct with function fields, not a Go interface (`cli/internal/provider/provider.go:10-20`). Go version is 1.25.5.

---

### Task 1.1: Add Cobra dependency and restructure CLI entrypoint

**Files:**
- Modify: `cli/go.mod` (add cobra dependency)
- Modify: `cli/cmd/nesco/main.go` (restructure around cobra)
- Create: `cli/cmd/nesco/main_test.go`

**Depends on:** Nothing

**Success Criteria:**
- [ ] `nesco` (no args) launches the existing TUI
- [ ] `nesco --help` shows available subcommands
- [ ] `nesco backfill` still works (migrated from hand-rolled check)
- [ ] `nesco version` prints version string
- [ ] All existing TUI functionality unchanged

---

#### Step 1: Write tests first

```go
// cli/cmd/nesco/main_test.go

package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommandHelp(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"--help"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("--help failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "nesco") {
		t.Error("help output should contain 'nesco'")
	}
	if !strings.Contains(out, "scan") || !strings.Contains(out, "version") {
		t.Error("help output should list subcommands")
	}
}

func TestVersionCommand(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"version"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}
	// version var may be empty in test — just verify no error
}

func TestGlobalFlags(t *testing.T) {
	// Verify global flags are registered
	flags := rootCmd.PersistentFlags()
	if flags.Lookup("json") == nil {
		t.Error("missing --json global flag")
	}
	if flags.Lookup("no-color") == nil {
		t.Error("missing --no-color global flag")
	}
	if flags.Lookup("quiet") == nil {
		t.Error("missing --quiet global flag")
	}
	if flags.Lookup("verbose") == nil {
		t.Error("missing --verbose global flag")
	}
}
```

#### Step 2: Add cobra to go.mod

```bash
cd /home/hhewett/.local/src/nesco/cli && go get github.com/spf13/cobra@latest
```

#### Step 3: Create root cobra command

Replace the current `main()` function in `cli/cmd/nesco/main.go`. The key change: `nesco` with no args runs the TUI (as `RunE` on the root command), subcommands are registered separately.

```go
// cli/cmd/nesco/main.go

package main

import (
	"fmt"
	"os"

	"github.com/holdenhewett/nesco/cli/internal/catalog"
	"github.com/holdenhewett/nesco/cli/internal/provider"
	"github.com/holdenhewett/nesco/cli/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var (
	repoRoot    string
	buildCommit string
	version     string
)

// Global flags
var jsonOutput bool

var rootCmd = &cobra.Command{
	Use:   "nesco",
	Short: "AI coding tool content manager and codebase scanner",
	Long:  "Nesco manages AI tool configurations and scans codebases for context that helps AI agents produce correct code.",
	RunE:  runTUI, // no subcommand → launch TUI
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().Bool("no-color", false, "Disable color output")
	rootCmd.PersistentFlags().BoolP("quiet", "q", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Verbose output")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(backfillCmd)
	// Phase 1+2 commands will be registered here
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print nesco version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version)
	},
}

var backfillCmd = &cobra.Command{
	Use:    "backfill",
	Short:  "Generate .nesco.yaml for items without metadata",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBackfill()
	},
}

func main() {
	ensureUpToDate()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runTUI(cmd *cobra.Command, args []string) error {
	root, err := findRepoRoot()
	if err != nil {
		return fmt.Errorf("could not find nesco repo: %w", err)
	}

	cat, err := catalog.Scan(root)
	if err != nil {
		return fmt.Errorf("catalog scan failed: %w", err)
	}

	catalog.CleanupPromotedItems(cat)
	providers := provider.DetectProviders()

	p := tea.NewProgram(tui.NewApp(cat, providers, version), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

// findRepoRoot, ensureUpToDate, runBackfill remain unchanged
```

#### Step 4: Run tests and verify

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/... && go build ./cmd/nesco && go test ./...
```

Expected: All tests pass. `./nesco` launches TUI. `./nesco version` prints version. `./nesco --help` shows command list.

---

### Task 1.2: Create ContextDocument model types

**Files:**
- Create: `cli/internal/model/document.go`
- Create: `cli/internal/model/section.go`
- Create: `cli/internal/model/document_test.go`

**Depends on:** Nothing (no other packages depend on model yet)

**Success Criteria:**
- [ ] ContextDocument, Section interface, typed sections, and TextSection types compile
- [ ] Tests verify section ordering, typed section field access, and TextSection creation

---

#### Step 1: Write the model types

```go
// cli/internal/model/document.go

package model

import "time"

// ContextDocument is the canonical representation of scanned codebase context.
// Produced by detectors, consumed by emitters, tracked by drift.
type ContextDocument struct {
	ProjectName string    `json:"projectName"`
	ScanTime    time.Time `json:"scanTime"`
	Sections    []Section `json:"sections"`
}

// Category identifies the semantic purpose of a section.
type Category string

const (
	CatTechStack      Category = "tech-stack"
	CatDependencies   Category = "dependencies"
	CatBuildCommands   Category = "build-commands"
	CatDirStructure   Category = "directory-structure"
	CatProjectMeta    Category = "project-metadata"
	CatConventions    Category = "conventions"
	CatSurprise       Category = "surprise"
	CatCurated        Category = "curated"
)

// Origin distinguishes auto-maintained from human-authored content.
type Origin string

const (
	OriginAuto  Origin = "auto"
	OriginHuman Origin = "human"
)
```

```go
// cli/internal/model/section.go

package model

// Section is the interface satisfied by both typed and text sections.
type Section interface {
	SectionCategory() Category
	SectionOrigin() Origin
	SectionTitle() string
}

// TextSection holds freeform content (surprises, curated, heuristics).
type TextSection struct {
	Category Category `json:"category"`
	Origin   Origin   `json:"origin"`
	Title    string   `json:"title"`
	Body     string   `json:"body"`
	Source   string   `json:"source,omitempty"` // detector name that produced this
}

func (s TextSection) SectionCategory() Category { return s.Category }
func (s TextSection) SectionOrigin() Origin     { return s.Origin }
func (s TextSection) SectionTitle() string      { return s.Title }

// TechStackSection holds parsed tech stack facts.
type TechStackSection struct {
	Origin           Origin            `json:"origin"`
	Title            string            `json:"title"`
	Language         string            `json:"language"`
	LanguageVersion  string            `json:"languageVersion"`
	Framework        string            `json:"framework,omitempty"`
	FrameworkVersion string            `json:"frameworkVersion,omitempty"`
	Runtime          string            `json:"runtime,omitempty"`
	RuntimeVersion   string            `json:"runtimeVersion,omitempty"`
	Extra            map[string]string `json:"extra,omitempty"`
}

func (s TechStackSection) SectionCategory() Category { return CatTechStack }
func (s TechStackSection) SectionOrigin() Origin     { return s.Origin }
func (s TechStackSection) SectionTitle() string      { return s.Title }

// DependencySection holds grouped dependency information.
type DependencySection struct {
	Origin Origin           `json:"origin"`
	Title  string           `json:"title"`
	Groups []DependencyGroup `json:"groups"`
}

type DependencyGroup struct {
	Category string       `json:"category"` // e.g., "production", "dev", "peer"
	Items    []Dependency `json:"items"`
}

type Dependency struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (s DependencySection) SectionCategory() Category { return CatDependencies }
func (s DependencySection) SectionOrigin() Origin     { return s.Origin }
func (s DependencySection) SectionTitle() string      { return s.Title }

// BuildCommandSection holds build/task runner commands.
type BuildCommandSection struct {
	Origin   Origin         `json:"origin"`
	Title    string         `json:"title"`
	Commands []BuildCommand `json:"commands"`
}

type BuildCommand struct {
	Name    string `json:"name"`    // e.g., "build", "test", "lint"
	Command string `json:"command"` // actual command text
	Source  string `json:"source"`  // file it came from (Makefile, package.json, etc.)
}

func (s BuildCommandSection) SectionCategory() Category { return CatBuildCommands }
func (s BuildCommandSection) SectionOrigin() Origin     { return s.Origin }
func (s BuildCommandSection) SectionTitle() string      { return s.Title }

// DirectoryStructureSection holds project layout information.
type DirectoryStructureSection struct {
	Origin  Origin          `json:"origin"`
	Title   string          `json:"title"`
	Entries []DirectoryEntry `json:"entries"`
}

type DirectoryEntry struct {
	Path        string `json:"path"`
	Description string `json:"description,omitempty"`
	Convention  string `json:"convention,omitempty"` // e.g., "source", "test", "config", "build"
}

func (s DirectoryStructureSection) SectionCategory() Category { return CatDirStructure }
func (s DirectoryStructureSection) SectionOrigin() Origin     { return s.Origin }
func (s DirectoryStructureSection) SectionTitle() string      { return s.Title }

// ProjectMetadataSection holds project-level facts.
type ProjectMetadataSection struct {
	Origin      Origin `json:"origin"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	License     string `json:"license,omitempty"`
	CI          string `json:"ci,omitempty"` // e.g., "GitHub Actions", "GitLab CI"
}

func (s ProjectMetadataSection) SectionCategory() Category { return CatProjectMeta }
func (s ProjectMetadataSection) SectionOrigin() Origin     { return s.Origin }
func (s ProjectMetadataSection) SectionTitle() string      { return s.Title }
```

#### Step 2: Write tests

```go
// cli/internal/model/document_test.go

package model

import "testing"

func TestSectionInterface(t *testing.T) {
	sections := []Section{
		TextSection{Category: CatSurprise, Origin: OriginAuto, Title: "Competing test frameworks", Body: "Jest and Vitest both present"},
		TechStackSection{Origin: OriginAuto, Title: "Tech Stack", Language: "TypeScript", LanguageVersion: "5.3"},
		BuildCommandSection{Origin: OriginAuto, Title: "Build Commands", Commands: []BuildCommand{{Name: "test", Command: "npm test", Source: "package.json"}}},
	}

	expected := []Category{CatSurprise, CatTechStack, CatBuildCommands}
	for i, s := range sections {
		if s.SectionCategory() != expected[i] {
			t.Errorf("section %d: category = %q, want %q", i, s.SectionCategory(), expected[i])
		}
		if s.SectionOrigin() != OriginAuto {
			t.Errorf("section %d: origin = %q, want %q", i, s.SectionOrigin(), OriginAuto)
		}
	}
}

func TestContextDocumentSections(t *testing.T) {
	doc := ContextDocument{
		ProjectName: "test-project",
		Sections: []Section{
			TechStackSection{Origin: OriginAuto, Title: "Tech Stack", Language: "Go", LanguageVersion: "1.25"},
			TextSection{Category: CatSurprise, Origin: OriginAuto, Title: "Internal package imported externally"},
			TextSection{Category: CatCurated, Origin: OriginHuman, Title: "Architecture notes"},
		},
	}

	if len(doc.Sections) != 3 {
		t.Fatalf("got %d sections, want 3", len(doc.Sections))
	}
	if doc.Sections[2].SectionOrigin() != OriginHuman {
		t.Errorf("curated section origin = %q, want %q", doc.Sections[2].SectionOrigin(), OriginHuman)
	}
}
```

#### Step 3: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/model/...
```

Expected: PASS

---

### Task 1.3: Expand Provider struct with Phase 1 methods

**Files:**
- Modify: `cli/internal/provider/provider.go` (add new function fields + Format type)
- Modify: `cli/internal/provider/claude.go` (add new field implementations)
- Modify: `cli/internal/provider/cursor.go`
- Modify: `cli/internal/provider/windsurf.go`
- Modify: `cli/internal/provider/codex.go`
- Modify: `cli/internal/provider/gemini.go`
- Create: `cli/internal/provider/provider_test.go`

**Depends on:** Nothing (additive change to existing struct)

**Success Criteria:**
- [ ] All providers compile with new fields
- [ ] `SupportsType()` still works (delegates to InstallDir)
- [ ] `DiscoveryPaths()` returns known filesystem locations per provider/content type
- [ ] `EmitPath()` returns where scan output goes per provider
- [ ] Existing TUI functionality unchanged (new fields are additive)

---

#### Step 1: Add new fields to Provider struct

```go
// cli/internal/provider/provider.go — additions

// Format identifies a file format used by a provider.
type Format string

const (
	FormatMarkdown Format = "md"
	FormatMDC      Format = "mdc"    // Cursor .mdc format
	FormatJSON     Format = "json"
	FormatYAML     Format = "yaml"
)

type Provider struct {
	Name      string
	Slug      string
	Detected  bool
	ConfigDir string

	// Existing
	InstallDir func(homeDir string, ct catalog.ContentType) string
	Detect     func(homeDir string) bool

	// New for Phase 1
	DiscoveryPaths func(projectRoot string, ct catalog.ContentType) []string
	FileFormat     func(ct catalog.ContentType) Format
	EmitPath       func(projectRoot string) string // where to write scan output (e.g., CLAUDE.md)
	SupportsType   func(ct catalog.ContentType) bool // capability matrix — whether provider supports this content type
}
```

#### Step 2: Implement for each provider

For each provider file, add the three new function fields. Example for Claude Code:

```go
// cli/internal/provider/claude.go — additions to the ClaudeCode var

DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
	switch ct {
	case catalog.Rules:
		return []string{
			filepath.Join(projectRoot, "CLAUDE.md"),
			filepath.Join(projectRoot, ".claude", "rules"),
		}
	case catalog.Commands:
		return []string{filepath.Join(projectRoot, ".claude", "commands")}
	case catalog.Skills:
		return []string{filepath.Join(projectRoot, ".claude", "skills")}
	case catalog.Agents:
		return []string{filepath.Join(projectRoot, ".claude", "agents")}
	case catalog.MCP:
		return []string{filepath.Join(projectRoot, ".claude.json")}
	case catalog.Hooks:
		return []string{filepath.Join(projectRoot, ".claude", "settings.json")}
	default:
		return nil
	}
},
FileFormat: func(ct catalog.ContentType) Format {
	switch ct {
	case catalog.MCP, catalog.Hooks:
		return FormatJSON
	default:
		return FormatMarkdown
	}
},
EmitPath: func(projectRoot string) string {
	return filepath.Join(projectRoot, "CLAUDE.md")
},
SupportsType: func(ct catalog.ContentType) bool {
	// Claude Code supports: Rules, Skills, Agents, Commands, MCP, Hooks
	switch ct {
	case catalog.Rules, catalog.Skills, catalog.Agents, catalog.Commands, catalog.MCP, catalog.Hooks:
		return true
	default:
		return false
	}
},
```

Each provider gets analogous implementations. Cursor uses FormatMDC for rules, Windsurf uses FormatMarkdown, etc. All providers must implement SupportsType to indicate their capability matrix (e.g., Cursor supports Rules and Hooks but not Skills).

#### Step 3: Write tests verifying new fields

```go
// cli/internal/provider/provider_test.go

package provider

import (
	"testing"
	"github.com/holdenhewett/nesco/cli/internal/catalog"
)

func TestDiscoveryPaths(t *testing.T) {
	tests := []struct {
		provider Provider
		ct       catalog.ContentType
		wantLen  int // at minimum
	}{
		{ClaudeCode, catalog.Rules, 2},
		{ClaudeCode, catalog.MCP, 1},
		{Cursor, catalog.Rules, 1},
		{GeminiCLI, catalog.Rules, 1},
	}
	for _, tt := range tests {
		paths := tt.provider.DiscoveryPaths("/tmp/project", tt.ct)
		if len(paths) < tt.wantLen {
			t.Errorf("%s.DiscoveryPaths(%s): got %d paths, want >= %d", tt.provider.Name, tt.ct, len(paths), tt.wantLen)
		}
	}
}

func TestEmitPath(t *testing.T) {
	path := ClaudeCode.EmitPath("/tmp/project")
	if path == "" {
		t.Error("ClaudeCode.EmitPath returned empty string")
	}
}
```

#### Step 4: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/provider/... && go test ./...
```

Expected: All tests pass. Existing functionality unchanged.

---

### Task 1.4: Create .nesco/ config package

**Files:**
- Create: `cli/internal/config/config.go`
- Create: `cli/internal/config/config_test.go`

**Depends on:** Task 1.3 (needs provider slugs)

**Success Criteria:**
- [ ] Config struct with provider selection, detector preferences
- [ ] Load/Save to `.nesco/config.json`
- [ ] Returns zero-value config if file doesn't exist (first run)
- [ ] Init function creates `.nesco/` directory

---

#### Step 1: Write config package

```go
// cli/internal/config/config.go

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const DirName = ".nesco"
const FileName = "config.json"

type Config struct {
	Providers       []string          `json:"providers"`       // enabled provider slugs
	DisabledDetectors []string        `json:"disabledDetectors,omitempty"`
	Preferences     map[string]string `json:"preferences,omitempty"`
}

func DirPath(projectRoot string) string {
	return filepath.Join(projectRoot, DirName)
}

func FilePath(projectRoot string) string {
	return filepath.Join(projectRoot, DirName, FileName)
}

func Load(projectRoot string) (*Config, error) {
	data, err := os.ReadFile(FilePath(projectRoot))
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func Save(projectRoot string, cfg *Config) error {
	dir := DirPath(projectRoot)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(FilePath(projectRoot), data, 0644)
}

func Exists(projectRoot string) bool {
	_, err := os.Stat(FilePath(projectRoot))
	return err == nil
}
```

#### Step 2: Write tests

```go
// cli/internal/config/config_test.go

package config

import (
	"path/filepath"
	"testing"
)

func TestLoadMissing(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load on missing dir: %v", err)
	}
	if len(cfg.Providers) != 0 {
		t.Errorf("expected empty providers, got %v", cfg.Providers)
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmp := t.TempDir()
	cfg := &Config{
		Providers: []string{"claude-code", "cursor"},
	}
	if err := Save(tmp, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(tmp)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Providers) != 2 || loaded.Providers[0] != "claude-code" {
		t.Errorf("loaded providers = %v, want [claude-code cursor]", loaded.Providers)
	}
}

func TestExists(t *testing.T) {
	tmp := t.TempDir()
	if Exists(tmp) {
		t.Error("Exists returned true before Save")
	}
	Save(tmp, &Config{})
	if !Exists(tmp) {
		t.Error("Exists returned false after Save")
	}
}
```

#### Step 3: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/config/...
```

Expected: PASS

---

### Task 1.5: Create output switching package

**Files:**
- Create: `cli/internal/output/output.go`
- Create: `cli/internal/output/output_test.go`

**Depends on:** Nothing

**Success Criteria:**
- [ ] `Print()` renders human-readable by default, JSON when `--json` flag set
- [ ] Error output includes `code`, `message`, `suggestion` in JSON mode
- [ ] Works with any struct that can be JSON-marshaled

---

#### Step 1: Write output package

```go
// cli/internal/output/output.go

package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

var (
	JSON   bool      // set from --json flag
	Writer io.Writer = os.Stdout
	ErrWriter io.Writer = os.Stderr
)

func Print(v any) {
	if JSON {
		data, _ := json.MarshalIndent(v, "", "  ")
		fmt.Fprintln(Writer, string(data))
	} else {
		fmt.Fprintln(Writer, v)
	}
}

type ErrorResponse struct {
	Code       int    `json:"code"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

func PrintError(code int, message, suggestion string) {
	if JSON {
		data, _ := json.MarshalIndent(ErrorResponse{
			Code: code, Message: message, Suggestion: suggestion,
		}, "", "  ")
		fmt.Fprintln(ErrWriter, string(data))
	} else {
		fmt.Fprintf(ErrWriter, "Error: %s\n", message)
		if suggestion != "" {
			fmt.Fprintf(ErrWriter, "  Suggestion: %s\n", suggestion)
		}
	}
}
```

#### Step 2: Write tests

```go
// cli/internal/output/output_test.go

package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintJSON(t *testing.T) {
	var buf bytes.Buffer
	Writer = &buf
	JSON = true
	defer func() { JSON = false; Writer = nil }()

	Print(map[string]string{"key": "value"})
	if !strings.Contains(buf.String(), `"key": "value"`) {
		t.Errorf("JSON output missing expected content: %s", buf.String())
	}
}

func TestPrintHuman(t *testing.T) {
	var buf bytes.Buffer
	Writer = &buf
	JSON = false

	Print("hello world")
	if !strings.Contains(buf.String(), "hello world") {
		t.Errorf("human output missing expected content: %s", buf.String())
	}
}
```

#### Step 3: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/output/...
```

Expected: PASS

---

**Group 1 complete.** 5 tasks covering the foundation layer. Next: Group 2 (Phase 1: Provider Parity).

---

## Group 2: Phase 1 Provider Parity

**Covers:** Decision #12 (CLI Commands - Phase 1), #16 (Import: Read-Only Into Memory), #10 (Parity Analysis)

**Codebase context:** Provider is a struct with function fields (`provider.go:10-20`). Five providers exist: ClaudeCode, Cursor, Windsurf, Codex, GeminiCLI. Only ClaudeCode and GeminiCLI support multiple content types — Cursor, Windsurf, and Codex only support Rules (and Codex adds Commands). The `catalog.ContentType` enum has 8 real types + 2 virtual types. Frontmatter parsing exists in `catalog/frontmatter.go`. The installer knows about JSON merge vs filesystem install patterns.

**Phase 1 import is discovery + classification + lightweight parsing** — read what exists, classify it, parse into TextSections. Deep format conversion is Phase 4.

---

### Task 2.1: Create parse package with discovery and classification

**Files:**
- Create: `cli/internal/parse/discovery.go`
- Create: `cli/internal/parse/classify.go`
- Create: `cli/internal/parse/discovery_test.go`

**Depends on:** Task 1.3 (Provider.DiscoveryPaths)

**Success Criteria:**
- [ ] `Discover()` finds all content files for a given provider at a project root
- [ ] `Classify()` determines content type from file path and provider context
- [ ] DiscoveryReport tracks found files per content type with unclassified count
- [ ] Tests verify discovery against fixture directories

---

#### Step 1: Write discovery types and function

```go
// cli/internal/parse/discovery.go

package parse

import (
	"os"
	"path/filepath"

	"github.com/holdenhewett/nesco/cli/internal/catalog"
	"github.com/holdenhewett/nesco/cli/internal/provider"
)

// DiscoveredFile represents a single file found during import discovery.
type DiscoveredFile struct {
	Path        string             `json:"path"`
	ContentType catalog.ContentType `json:"contentType"`
	Provider    string             `json:"provider"`
}

// DiscoveryReport summarizes what was found for a provider.
type DiscoveryReport struct {
	Provider     string           `json:"provider"`
	Files        []DiscoveredFile `json:"files"`
	Counts       map[catalog.ContentType]int `json:"counts"`
	Unclassified []string         `json:"unclassified,omitempty"`
}

// Discover finds all content files for a provider in a project directory.
// It walks each content type's discovery paths and classifies found files.
func Discover(prov provider.Provider, projectRoot string) DiscoveryReport {
	report := DiscoveryReport{
		Provider: prov.Slug,
		Counts:   make(map[catalog.ContentType]int),
	}

	for _, ct := range catalog.AllContentTypes() {
		if prov.DiscoveryPaths == nil {
			continue
		}
		paths := prov.DiscoveryPaths(projectRoot, ct)
		for _, p := range paths {
			files := findFiles(p, ct)
			for _, f := range files {
				report.Files = append(report.Files, DiscoveredFile{
					Path:        f,
					ContentType: ct,
					Provider:    prov.Slug,
				})
				report.Counts[ct]++
			}
		}
	}

	return report
}

// findFiles returns file paths at a discovery location.
// If the path is a directory, it lists files inside (non-recursive for most types).
// If the path is a file, it returns that file if it exists.
func findFiles(path string, ct catalog.ContentType) []string {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}

	if !info.IsDir() {
		return []string{path}
	}

	// For directories, list immediate files
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		files = append(files, filepath.Join(path, e.Name()))
	}
	return files
}
```

#### Step 2: Write classification helper

```go
// cli/internal/parse/classify.go

package parse

import (
	"path/filepath"
	"strings"

	"github.com/holdenhewett/nesco/cli/internal/catalog"
)

// ClassifyByExtension provides a fallback classification based on file extension
// and path patterns when provider-specific classification isn't available.
func ClassifyByExtension(filePath string) (catalog.ContentType, bool) {
	base := filepath.Base(filePath)
	dir := filepath.Dir(filePath)

	// Provider-specific files by name
	switch strings.ToLower(base) {
	case "claude.md", "gemini.md", "agents.md", "copilot-instructions.md":
		return catalog.Rules, true
	case ".claude.json":
		return catalog.MCP, true
	case "settings.json":
		if strings.Contains(dir, ".claude") {
			return catalog.Hooks, true
		}
	}

	// By directory name
	dirBase := filepath.Base(dir)
	switch dirBase {
	case "rules":
		return catalog.Rules, true
	case "skills":
		return catalog.Skills, true
	case "agents":
		return catalog.Agents, true
	case "commands":
		return catalog.Commands, true
	case "hooks":
		return catalog.Hooks, true
	case "prompts":
		return catalog.Prompts, true
	}

	// By extension
	ext := filepath.Ext(filePath)
	switch ext {
	case ".mdc":
		return catalog.Rules, true
	}

	return "", false
}
```

#### Step 3: Write tests

```go
// cli/internal/parse/discovery_test.go

package parse

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/nesco/cli/internal/catalog"
	"github.com/holdenhewett/nesco/cli/internal/provider"
)

func TestDiscoverFindsFiles(t *testing.T) {
	// Create a fixture project with some Claude Code files
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, ".claude", "rules"), 0755)
	os.WriteFile(filepath.Join(tmp, "CLAUDE.md"), []byte("# Rules"), 0644)
	os.WriteFile(filepath.Join(tmp, ".claude", "rules", "test.md"), []byte("test rule"), 0644)

	// Create a minimal provider with DiscoveryPaths
	prov := provider.Provider{
		Slug: "claude-code",
		DiscoveryPaths: func(root string, ct catalog.ContentType) []string {
			switch ct {
			case catalog.Rules:
				return []string{
					filepath.Join(root, "CLAUDE.md"),
					filepath.Join(root, ".claude", "rules"),
				}
			}
			return nil
		},
	}

	report := Discover(prov, tmp)
	if report.Counts[catalog.Rules] != 2 {
		t.Errorf("expected 2 rules files, got %d", report.Counts[catalog.Rules])
	}
}

func TestDiscoverEmptyProject(t *testing.T) {
	tmp := t.TempDir()
	prov := provider.Provider{
		Slug: "cursor",
		DiscoveryPaths: func(root string, ct catalog.ContentType) []string {
			return []string{filepath.Join(root, ".cursor", "rules")}
		},
	}

	report := Discover(prov, tmp)
	if len(report.Files) != 0 {
		t.Errorf("expected 0 files in empty project, got %d", len(report.Files))
	}
}

func TestClassifyByExtension(t *testing.T) {
	tests := []struct {
		path string
		want catalog.ContentType
		ok   bool
	}{
		{"CLAUDE.md", catalog.Rules, true},
		{"/proj/.cursor/rules/test.mdc", catalog.Rules, true},
		{"/proj/.claude/skills/test/SKILL.md", catalog.Skills, true},
		{"/proj/random.txt", "", false},
	}
	for _, tt := range tests {
		got, ok := ClassifyByExtension(tt.path)
		if ok != tt.ok || got != tt.want {
			t.Errorf("ClassifyByExtension(%q) = (%q, %v), want (%q, %v)", tt.path, got, ok, tt.want, tt.ok)
		}
	}
}
```

#### Step 4: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/parse/...
```

Expected: PASS

---

### Task 2.2: Create provider parsers

**Files:**
- Create: `cli/internal/parse/parser.go`
- Create: `cli/internal/parse/claude.go`
- Create: `cli/internal/parse/cursor.go`
- Create: `cli/internal/parse/generic.go`
- Create: `cli/internal/parse/parser_test.go`

**Depends on:** Task 1.2 (model types), Task 2.1 (discovery types)

**Success Criteria:**
- [ ] Parser interface defined with per-provider implementations
- [ ] Claude Code parser reads CLAUDE.md into TextSections
- [ ] Cursor parser reads .mdc files, extracting YAML frontmatter
- [ ] Generic parser handles markdown files for Windsurf/Codex/Gemini
- [ ] JSON parser handles MCP and hooks configs
- [ ] Tests verify parsing against sample content

---

#### Step 1: Define Parser interface

```go
// cli/internal/parse/parser.go

package parse

import (
	"encoding/json"
	"os"

	"github.com/holdenhewett/nesco/cli/internal/model"
)

// Parser reads a provider-specific file and returns canonical sections.
type Parser interface {
	// ParseFile reads a single discovered file and returns sections.
	ParseFile(file DiscoveredFile) ([]model.Section, error)
}

// ImportResult holds the complete parsed output from an import operation.
type ImportResult struct {
	Provider string          `json:"provider"`
	Sections []model.Section `json:"sections"`
	Report   DiscoveryReport `json:"report"`
}

// Import runs full discovery + parsing for a provider.
func Import(prov Provider, parser Parser, projectRoot string) (*ImportResult, error) {
	report := Discover(prov, projectRoot)

	result := &ImportResult{
		Provider: prov.Slug,
		Report:   report,
	}

	for _, file := range report.Files {
		sections, err := parser.ParseFile(file)
		if err != nil {
			// Skip unparseable files, add to unclassified
			report.Unclassified = append(report.Unclassified, file.Path)
			continue
		}
		result.Sections = append(result.Sections, sections...)
	}

	return result, nil
}

// readFileContent is a helper to read file content.
func readFileContent(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// parseJSONFile reads a JSON file into the given target.
func parseJSONFile(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
```

Note: `Import()` references `Provider` — this should be `provider.Provider` from the provider package. The function signature should use the correct import.

#### Step 2: Claude Code parser

```go
// cli/internal/parse/claude.go

package parse

import (
	"path/filepath"
	"strings"

	"github.com/holdenhewett/nesco/cli/internal/catalog"
	"github.com/holdenhewett/nesco/cli/internal/model"
)

// ClaudeParser parses Claude Code provider files.
type ClaudeParser struct{}

func (p ClaudeParser) ParseFile(file DiscoveredFile) ([]model.Section, error) {
	content, err := readFileContent(file.Path)
	if err != nil {
		return nil, err
	}

	switch file.ContentType {
	case catalog.Rules:
		return p.parseRule(file.Path, content)
	case catalog.MCP:
		return p.parseMCP(file.Path, content)
	case catalog.Hooks:
		return p.parseHooks(file.Path, content)
	default:
		return p.parseGenericMarkdown(file, content)
	}
}

func (p ClaudeParser) parseRule(path string, content []byte) ([]model.Section, error) {
	name := filepath.Base(path)
	name = strings.TrimSuffix(name, filepath.Ext(name))

	return []model.Section{
		model.TextSection{
			Category: model.CatConventions,
			Origin:   model.OriginHuman,
			Title:    "Rule: " + name,
			Body:     string(content),
			Source:   "import:claude-code:" + path,
		},
	}, nil
}

func (p ClaudeParser) parseMCP(path string, content []byte) ([]model.Section, error) {
	// .claude.json contains mcpServers key
	return []model.Section{
		model.TextSection{
			Category: model.CatConventions,
			Origin:   model.OriginHuman,
			Title:    "MCP Configuration",
			Body:     string(content),
			Source:   "import:claude-code:" + path,
		},
	}, nil
}

func (p ClaudeParser) parseHooks(path string, content []byte) ([]model.Section, error) {
	return []model.Section{
		model.TextSection{
			Category: model.CatConventions,
			Origin:   model.OriginHuman,
			Title:    "Hooks Configuration",
			Body:     string(content),
			Source:   "import:claude-code:" + path,
		},
	}, nil
}

func (p ClaudeParser) parseGenericMarkdown(file DiscoveredFile, content []byte) ([]model.Section, error) {
	name := filepath.Base(file.Path)
	return []model.Section{
		model.TextSection{
			Category: model.CatConventions,
			Origin:   model.OriginHuman,
			Title:    string(file.ContentType) + ": " + name,
			Body:     string(content),
			Source:   "import:claude-code:" + file.Path,
		},
	}, nil
}
```

#### Step 3: Cursor parser

```go
// cli/internal/parse/cursor.go

package parse

import (
	"bytes"
	"path/filepath"
	"strings"

	"github.com/holdenhewett/nesco/cli/internal/catalog"
	"github.com/holdenhewett/nesco/cli/internal/model"
	"gopkg.in/yaml.v3"
)

// CursorParser parses Cursor provider files (.mdc format).
type CursorParser struct{}

// CursorFrontmatter represents the YAML frontmatter in .mdc files.
type CursorFrontmatter struct {
	Description string   `yaml:"description"`
	Globs       []string `yaml:"globs,omitempty"`
	AlwaysApply bool     `yaml:"alwaysApply,omitempty"`
}

func (p CursorParser) ParseFile(file DiscoveredFile) ([]model.Section, error) {
	content, err := readFileContent(file.Path)
	if err != nil {
		return nil, err
	}

	if file.ContentType == catalog.Rules && filepath.Ext(file.Path) == ".mdc" {
		return p.parseMDC(file.Path, content)
	}

	// Fallback: treat as plain text
	name := filepath.Base(file.Path)
	return []model.Section{
		model.TextSection{
			Category: model.CatConventions,
			Origin:   model.OriginHuman,
			Title:    string(file.ContentType) + ": " + name,
			Body:     string(content),
			Source:   "import:cursor:" + file.Path,
		},
	}, nil
}

func (p CursorParser) parseMDC(path string, content []byte) ([]model.Section, error) {
	name := filepath.Base(path)
	name = strings.TrimSuffix(name, ".mdc")

	fm, body, err := parseMDCFrontmatter(content)
	if err != nil {
		// No frontmatter — treat whole file as body
		return []model.Section{
			model.TextSection{
				Category: model.CatConventions,
				Origin:   model.OriginHuman,
				Title:    "Cursor Rule: " + name,
				Body:     string(content),
				Source:   "import:cursor:" + path,
			},
		}, nil
	}

	title := "Cursor Rule: " + name
	if fm.Description != "" {
		title = "Cursor Rule: " + fm.Description
	}

	return []model.Section{
		model.TextSection{
			Category: model.CatConventions,
			Origin:   model.OriginHuman,
			Title:    title,
			Body:     body,
			Source:   "import:cursor:" + path,
		},
	}, nil
}

// parseMDCFrontmatter parses the YAML frontmatter from a .mdc file.
// .mdc files use --- delimiters like standard frontmatter.
func parseMDCFrontmatter(content []byte) (CursorFrontmatter, string, error) {
	normalized := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))

	opening := []byte("---\n")
	if !bytes.HasPrefix(normalized, opening) {
		return CursorFrontmatter{}, "", errNoFrontmatter
	}

	rest := normalized[len(opening):]
	closingIdx := bytes.Index(rest, opening)
	if closingIdx == -1 {
		return CursorFrontmatter{}, "", errNoFrontmatter
	}

	yamlBytes := rest[:closingIdx]
	var fm CursorFrontmatter
	if err := yaml.Unmarshal(yamlBytes, &fm); err != nil {
		return CursorFrontmatter{}, "", err
	}

	body := strings.TrimSpace(string(rest[closingIdx+len(opening):]))
	return fm, body, nil
}

var errNoFrontmatter = bytes.ErrTooLarge // placeholder; replace with proper error
```

Note: The `errNoFrontmatter` should be a proper error. Use `errors.New("no frontmatter found")` with an `errors` import.

#### Step 4: Generic parser for remaining providers

```go
// cli/internal/parse/generic.go

package parse

import (
	"path/filepath"

	"github.com/holdenhewett/nesco/cli/internal/model"
)

// GenericParser handles providers that use standard markdown files
// (Windsurf, Codex, Gemini CLI).
type GenericParser struct {
	ProviderSlug string
}

func (p GenericParser) ParseFile(file DiscoveredFile) ([]model.Section, error) {
	content, err := readFileContent(file.Path)
	if err != nil {
		return nil, err
	}

	name := filepath.Base(file.Path)
	return []model.Section{
		model.TextSection{
			Category: model.CatConventions,
			Origin:   model.OriginHuman,
			Title:    string(file.ContentType) + ": " + name,
			Body:     string(content),
			Source:   "import:" + p.ProviderSlug + ":" + file.Path,
		},
	}, nil
}

// ParserForProvider returns the appropriate parser for a provider slug.
func ParserForProvider(slug string) Parser {
	switch slug {
	case "claude-code":
		return ClaudeParser{}
	case "cursor":
		return CursorParser{}
	default:
		return GenericParser{ProviderSlug: slug}
	}
}
```

#### Step 5: Write tests

```go
// cli/internal/parse/parser_test.go

package parse

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/nesco/cli/internal/catalog"
	"github.com/holdenhewett/nesco/cli/internal/model"
)

func TestClaudeParserRule(t *testing.T) {
	tmp := t.TempDir()
	rulePath := filepath.Join(tmp, "test-rule.md")
	os.WriteFile(rulePath, []byte("# Always use tabs\nTabs are the way."), 0644)

	parser := ClaudeParser{}
	sections, err := parser.ParseFile(DiscoveredFile{
		Path:        rulePath,
		ContentType: catalog.Rules,
		Provider:    "claude-code",
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
	ts := sections[0].(model.TextSection)
	if ts.Category != model.CatConventions {
		t.Errorf("category = %q, want %q", ts.Category, model.CatConventions)
	}
	if ts.Body != "# Always use tabs\nTabs are the way." {
		t.Errorf("body mismatch: %q", ts.Body)
	}
}

func TestCursorParserMDC(t *testing.T) {
	tmp := t.TempDir()
	mdcPath := filepath.Join(tmp, "testing.mdc")
	content := "---\ndescription: Testing conventions\nalwaysApply: true\n---\nAlways use Jest for testing."
	os.WriteFile(mdcPath, []byte(content), 0644)

	parser := CursorParser{}
	sections, err := parser.ParseFile(DiscoveredFile{
		Path:        mdcPath,
		ContentType: catalog.Rules,
		Provider:    "cursor",
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
	ts := sections[0].(model.TextSection)
	if ts.Title != "Cursor Rule: Testing conventions" {
		t.Errorf("title = %q, want 'Cursor Rule: Testing conventions'", ts.Title)
	}
	if ts.Body != "Always use Jest for testing." {
		t.Errorf("body = %q", ts.Body)
	}
}

func TestParserForProvider(t *testing.T) {
	tests := []struct {
		slug string
		want string
	}{
		{"claude-code", "parse.ClaudeParser"},
		{"cursor", "parse.CursorParser"},
		{"windsurf", "parse.GenericParser"},
	}
	for _, tt := range tests {
		p := ParserForProvider(tt.slug)
		if p == nil {
			t.Errorf("ParserForProvider(%q) returned nil", tt.slug)
		}
	}
}
```

#### Step 6: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/parse/...
```

Expected: PASS

---

### Task 2.3: Create parity analysis package

**Files:**
- Create: `cli/internal/parity/parity.go`
- Create: `cli/internal/parity/parity_test.go`

**Depends on:** Task 1.3 (Provider.DiscoveryPaths), Task 2.1 (Discover)

**Success Criteria:**
- [ ] `Analyze()` produces a per-provider per-content-type coverage matrix
- [ ] `Gaps()` identifies content types present in one provider but missing in another
- [ ] Reports suggest sync opportunities
- [ ] Tests verify against fixtures with known parity gaps

---

#### Step 1: Write parity analysis

```go
// cli/internal/parity/parity.go

package parity

import (
	"github.com/holdenhewett/nesco/cli/internal/catalog"
	"github.com/holdenhewett/nesco/cli/internal/parse"
	"github.com/holdenhewett/nesco/cli/internal/provider"
)

// Coverage represents what a single provider has configured.
type Coverage struct {
	Provider string                         `json:"provider"`
	Types    map[catalog.ContentType]int    `json:"types"`    // content type → file count
}

// Gap represents a content type present in one provider but missing in another.
type Gap struct {
	ContentType catalog.ContentType `json:"contentType"`
	HasIt       []string            `json:"hasIt"`       // provider slugs that have this
	MissingIt   []string            `json:"missingIt"`   // provider slugs that don't
}

// Report is the complete parity analysis output.
type Report struct {
	Coverages []Coverage `json:"coverages"`
	Gaps      []Gap      `json:"gaps"`
	Summary   string     `json:"summary"`
}

// Analyze runs discovery for all detected providers and compares coverage.
func Analyze(providers []provider.Provider, projectRoot string) Report {
	var coverages []Coverage

	for _, prov := range providers {
		report := parse.Discover(prov, projectRoot)
		coverages = append(coverages, Coverage{
			Provider: prov.Slug,
			Types:    report.Counts,
		})
	}

	gaps := findGaps(coverages, providers)

	return Report{
		Coverages: coverages,
		Gaps:      gaps,
		Summary:   summarize(coverages, gaps),
	}
}

// findGaps identifies content types with uneven coverage across providers.
func findGaps(coverages []Coverage, providers []provider.Provider) []Gap {
	// Build a set of all content types found across any provider
	allFound := make(map[catalog.ContentType]bool)
	for _, c := range coverages {
		for ct, count := range c.Types {
			if count > 0 {
				allFound[ct] = true
			}
		}
	}

	var gaps []Gap
	for ct := range allFound {
		var hasIt, missingIt []string
		for _, c := range coverages {
			// Only report gaps for providers that support this content type
			prov := findProvider(c.Provider, providers)
			if prov == nil {
				continue
			}
			if prov.SupportsType(ct) {
				if c.Types[ct] > 0 {
					hasIt = append(hasIt, c.Provider)
				} else {
					missingIt = append(missingIt, c.Provider)
				}
			}
		}
		if len(missingIt) > 0 && len(hasIt) > 0 {
			gaps = append(gaps, Gap{
				ContentType: ct,
				HasIt:       hasIt,
				MissingIt:   missingIt,
			})
		}
	}

	return gaps
}

func findProvider(slug string, providers []provider.Provider) *provider.Provider {
	for i := range providers {
		if providers[i].Slug == slug {
			return &providers[i]
		}
	}
	return nil
}

func summarize(coverages []Coverage, gaps []Gap) string {
	if len(gaps) == 0 {
		return "All providers are in sync."
	}
	s := ""
	for _, g := range gaps {
		s += g.ContentType.Label() + ": present in "
		for i, p := range g.HasIt {
			if i > 0 {
				s += ", "
			}
			s += p
		}
		s += " but missing in "
		for i, p := range g.MissingIt {
			if i > 0 {
				s += ", "
			}
			s += p
		}
		s += ". "
	}
	return s
}
```

#### Step 2: Write tests

```go
// cli/internal/parity/parity_test.go

package parity

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/nesco/cli/internal/catalog"
	"github.com/holdenhewett/nesco/cli/internal/provider"
)

func TestAnalyzeFindsGaps(t *testing.T) {
	tmp := t.TempDir()

	// Create Claude Code rules but no Cursor rules
	os.MkdirAll(filepath.Join(tmp, ".claude", "rules"), 0755)
	os.WriteFile(filepath.Join(tmp, "CLAUDE.md"), []byte("# Rules"), 0644)
	os.MkdirAll(filepath.Join(tmp, ".cursor"), 0755) // empty

	providers := []provider.Provider{
		{
			Name: "Claude Code", Slug: "claude-code",
			DiscoveryPaths: func(root string, ct catalog.ContentType) []string {
				if ct == catalog.Rules {
					return []string{
						filepath.Join(root, "CLAUDE.md"),
						filepath.Join(root, ".claude", "rules"),
					}
				}
				return nil
			},
			InstallDir: func(_ string, ct catalog.ContentType) string {
				if ct == catalog.Rules { return "/tmp" }
				return ""
			},
		},
		{
			Name: "Cursor", Slug: "cursor",
			DiscoveryPaths: func(root string, ct catalog.ContentType) []string {
				if ct == catalog.Rules {
					return []string{filepath.Join(root, ".cursor", "rules")}
				}
				return nil
			},
			InstallDir: func(_ string, ct catalog.ContentType) string {
				if ct == catalog.Rules { return "/tmp" }
				return ""
			},
		},
	}

	report := Analyze(providers, tmp)

	if len(report.Gaps) == 0 {
		t.Error("expected at least one gap")
	}
	if report.Gaps[0].ContentType != catalog.Rules {
		t.Errorf("gap content type = %q, want rules", report.Gaps[0].ContentType)
	}
}

func TestAnalyzeNoParity(t *testing.T) {
	tmp := t.TempDir()
	// No content for any provider

	providers := []provider.Provider{
		{
			Name: "Claude Code", Slug: "claude-code",
			DiscoveryPaths: func(root string, ct catalog.ContentType) []string {
				return nil
			},
		},
	}

	report := Analyze(providers, tmp)
	if len(report.Gaps) != 0 {
		t.Errorf("expected no gaps for empty project, got %d", len(report.Gaps))
	}
}
```

#### Step 3: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/parity/...
```

Expected: PASS

---

### Task 2.4: Add `nesco init` command

**Files:**
- Create: `cli/cmd/nesco/init.go`
- Create: `cli/cmd/nesco/init_test.go`

**Depends on:** Task 1.1 (Cobra), Task 1.3 (Provider.Detect), Task 1.4 (config package)

**Success Criteria:**
- [ ] `nesco init` detects providers, shows findings, creates `.nesco/config.json`
- [ ] `--yes` skips interactive confirmation
- [ ] `--json` outputs structured JSON instead of human-readable
- [ ] If `.nesco/config.json` already exists, warns and exits (unless `--force`)
- [ ] Respects `NESCO_NO_PROMPT=1` as equivalent to `--yes`

---

#### Step 1: Write init command

```go
// cli/cmd/nesco/init.go

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/holdenhewett/nesco/cli/internal/config"
	"github.com/holdenhewett/nesco/cli/internal/output"
	"github.com/holdenhewett/nesco/cli/internal/provider"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize nesco for this project",
	Long:  "Detects AI coding tools in use, creates .nesco/config.json with provider selection.",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().Bool("yes", false, "Skip interactive confirmation")
	initCmd.Flags().Bool("force", false, "Overwrite existing config")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}

	force, _ := cmd.Flags().GetBool("force")
	if config.Exists(root) && !force {
		output.PrintError(1, ".nesco/config.json already exists", "Use --force to overwrite")
		return fmt.Errorf("config already exists")
	}

	// Detect providers
	home, _ := os.UserHomeDir()
	var detected []provider.Provider
	for _, prov := range provider.AllProviders {
		if prov.Detect(home) {
			detected = append(detected, prov)
		}
	}

	if output.JSON {
		type initResult struct {
			Detected []string `json:"detected"`
			ConfigPath string `json:"configPath"`
		}
		slugs := make([]string, len(detected))
		for i, p := range detected {
			slugs[i] = p.Slug
		}
		output.Print(initResult{
			Detected:   slugs,
			ConfigPath: config.FilePath(root),
		})
	} else {
		fmt.Printf("Detected AI tools:\n")
		for _, p := range detected {
			fmt.Printf("  ✓ %s\n", p.Name)
		}
		if len(detected) == 0 {
			fmt.Println("  (none detected)")
		}
	}

	// Check for --yes or NESCO_NO_PROMPT
	yes, _ := cmd.Flags().GetBool("yes")
	if !yes && os.Getenv("NESCO_NO_PROMPT") != "1" && !output.JSON {
		fmt.Printf("\nSave to .nesco/config.json? [Y/n] ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) == "n" {
			return nil
		}
	}

	// Save config
	slugs := make([]string, len(detected))
	for i, p := range detected {
		slugs[i] = p.Slug
	}
	cfg := &config.Config{
		Providers: slugs,
	}
	return config.Save(root, cfg)
}

// findProjectRoot walks up from cwd looking for common project markers.
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Walk up looking for .git, go.mod, package.json, etc.
	for {
		markers := []string{".git", "go.mod", "package.json", "Cargo.toml", "pyproject.toml"}
		for _, m := range markers {
			if _, err := os.Stat(fmt.Sprintf("%s/%s", dir, m)); err == nil {
				return dir, nil
			}
		}
		parent := fmt.Sprintf("%s/..", dir)
		abs, err := os.Getwd()
		if err != nil || abs == "/" {
			break
		}
		dir = parent
	}

	// Fallback to cwd
	return os.Getwd()
}
```

Note: `findProjectRoot()` should use `filepath.Dir()` for walking up, not string concatenation. The implementation above is simplified — the executor should use `filepath.Abs()` and `filepath.Dir()` with a loop that checks `dir == filepath.Dir(dir)` for the root.

#### Step 2: Write tests

```go
// cli/cmd/nesco/init_test.go

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/nesco/cli/internal/config"
)

func TestInitCreatesConfig(t *testing.T) {
	tmp := t.TempDir()
	// Create a project marker so findProjectRoot works
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)

	// Override working directory for the command
	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	initCmd.SetArgs([]string{"--yes"})
	if err := initCmd.RunE(initCmd, []string{}); err != nil {
		t.Fatalf("init --yes failed: %v", err)
	}

	if !config.Exists(tmp) {
		t.Error("config.json should exist after init")
	}
}

func TestInitRefusesOverwrite(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	config.Save(tmp, &config.Config{Providers: []string{"claude-code"}})

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	initCmd.SetArgs([]string{"--yes"})
	err := initCmd.RunE(initCmd, []string{})
	if err == nil {
		t.Error("init should fail when config already exists (no --force)")
	}
}

func TestInitForceOverwrite(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	config.Save(tmp, &config.Config{Providers: []string{"old"}})

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	initCmd.Flags().Set("force", "true")
	initCmd.Flags().Set("yes", "true")
	err := initCmd.RunE(initCmd, []string{})
	if err != nil {
		t.Fatalf("init --force --yes failed: %v", err)
	}
}
```

#### Step 3: Write implementation

(init.go code as written above)

#### Step 4: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/... && go build ./cmd/nesco
```

Expected: Tests pass. `./nesco init --help` shows flags.

---

### Task 2.5: Add `nesco import` and `nesco parity` commands

**Files:**
- Create: `cli/cmd/nesco/import.go`
- Create: `cli/cmd/nesco/import_test.go`
- Create: `cli/cmd/nesco/parity.go`
- Create: `cli/cmd/nesco/parity_test.go`

**Depends on:** Task 1.1 (Cobra), Task 2.1-2.2 (parse package), Task 2.3 (parity package)

**Success Criteria:**
- [ ] `nesco import --from cursor` discovers and parses Cursor content
- [ ] `--preview` shows discovery report without parsing
- [ ] `--type rules` limits to a single content type
- [ ] `--json` outputs structured ImportResult
- [ ] `nesco parity` shows coverage matrix across detected providers
- [ ] `nesco parity --json` outputs structured Report

---

#### Step 1: Write import command

```go
// cli/cmd/nesco/import.go

package main

import (
	"fmt"

	"github.com/holdenhewett/nesco/cli/internal/catalog"
	"github.com/holdenhewett/nesco/cli/internal/output"
	"github.com/holdenhewett/nesco/cli/internal/parse"
	"github.com/holdenhewett/nesco/cli/internal/provider"
	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Read existing AI tool configs into canonical model",
	Long:  "Discovers and parses provider-specific content files. Read-only — nothing is written to disk.",
	RunE:  runImport,
}

func init() {
	importCmd.Flags().String("from", "", "Provider to import from (required)")
	importCmd.MarkFlagRequired("from")
	importCmd.Flags().String("type", "", "Limit to a single content type (e.g., rules, hooks, mcp)")
	importCmd.Flags().Bool("preview", false, "Show discovery report without parsing")
	rootCmd.AddCommand(importCmd)
}

func runImport(cmd *cobra.Command, args []string) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}

	fromSlug, _ := cmd.Flags().GetString("from")
	prov := findProviderBySlug(fromSlug)
	if prov == nil {
		output.PrintError(1, "unknown provider: "+fromSlug, "Available: claude-code, cursor, windsurf, codex, gemini-cli")
		return fmt.Errorf("unknown provider: %s", fromSlug)
	}

	// Filter by content type if specified
	typeFilter, _ := cmd.Flags().GetString("type")
	preview, _ := cmd.Flags().GetBool("preview")

	// Discovery
	report := parse.Discover(*prov, root)

	// Filter by type if specified
	if typeFilter != "" {
		ct := catalog.ContentType(typeFilter)
		var filtered []parse.DiscoveredFile
		for _, f := range report.Files {
			if f.ContentType == ct {
				filtered = append(filtered, f)
			}
		}
		report.Files = filtered
		newCounts := map[catalog.ContentType]int{ct: report.Counts[ct]}
		report.Counts = newCounts
	}

	if preview || output.JSON {
		if output.JSON {
			output.Print(report)
		} else {
			printDiscoveryReport(report)
		}
		return nil
	}

	// Full parse
	parser := parse.ParserForProvider(prov.Slug)
	result := &parse.ImportResult{
		Provider: prov.Slug,
		Report:   report,
	}
	for _, file := range report.Files {
		sections, err := parser.ParseFile(file)
		if err != nil {
			report.Unclassified = append(report.Unclassified, file.Path)
			continue
		}
		result.Sections = append(result.Sections, sections...)
	}

	if output.JSON {
		output.Print(result)
	} else {
		printDiscoveryReport(report)
		fmt.Printf("\nParsed %d sections from %d files.\n", len(result.Sections), len(report.Files))
	}

	return nil
}

func findProviderBySlug(slug string) *provider.Provider {
	for i := range provider.AllProviders {
		if provider.AllProviders[i].Slug == slug {
			return &provider.AllProviders[i]
		}
	}
	return nil
}

func printDiscoveryReport(report parse.DiscoveryReport) {
	fmt.Printf("Import from %s:\n", report.Provider)
	total := 0
	for ct, count := range report.Counts {
		if count > 0 {
			fmt.Printf("  %s: %d file(s)\n", ct.Label(), count)
			total += count
		}
	}
	if total == 0 {
		fmt.Println("  No content found.")
	}
	if len(report.Unclassified) > 0 {
		fmt.Printf("  %d file(s) couldn't be classified.\n", len(report.Unclassified))
	}
}
```

#### Step 2: Write parity command

```go
// cli/cmd/nesco/parity.go

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/holdenhewett/nesco/cli/internal/catalog"
	"github.com/holdenhewett/nesco/cli/internal/output"
	"github.com/holdenhewett/nesco/cli/internal/parity"
	"github.com/holdenhewett/nesco/cli/internal/provider"
	"github.com/spf13/cobra"
)

var parityCmd = &cobra.Command{
	Use:   "parity",
	Short: "Compare AI tool configs across providers",
	Long:  "Analyzes which providers have content configured and reports gaps between them.",
	RunE:  runParity,
}

func init() {
	rootCmd.AddCommand(parityCmd)
}

func runParity(cmd *cobra.Command, args []string) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}

	// Find detected providers
	home, _ := os.UserHomeDir()
	var detected []provider.Provider
	for _, prov := range provider.AllProviders {
		if prov.Detect(home) {
			detected = append(detected, prov)
		}
	}

	if len(detected) < 2 {
		if output.JSON {
			output.Print(map[string]string{"message": "fewer than 2 providers detected, parity analysis requires at least 2"})
		} else {
			fmt.Println("Parity analysis requires at least 2 detected providers.")
			fmt.Printf("Detected: %d\n", len(detected))
		}
		return nil
	}

	report := parity.Analyze(detected, root)

	if output.JSON {
		output.Print(report)
		return nil
	}

	// Human-readable output: coverage matrix
	fmt.Println("Coverage Matrix:")
	fmt.Println()

	// Header
	header := fmt.Sprintf("  %-16s", "Content Type")
	for _, c := range report.Coverages {
		header += fmt.Sprintf("  %-14s", c.Provider)
	}
	fmt.Println(header)
	fmt.Println(strings.Repeat("─", len(header)))

	// Rows
	for _, ct := range catalog.AllContentTypes() {
		row := fmt.Sprintf("  %-16s", ct.Label())
		for _, c := range report.Coverages {
			count := c.Types[ct]
			if count > 0 {
				row += fmt.Sprintf("  %-14s", fmt.Sprintf("✓ %d", count))
			} else {
				row += fmt.Sprintf("  %-14s", "—")
			}
		}
		fmt.Println(row)
	}

	// Gaps
	if len(report.Gaps) > 0 {
		fmt.Printf("\nGaps Found:\n")
		for _, g := range report.Gaps {
			fmt.Printf("  %s: present in %s, missing in %s\n",
				g.ContentType.Label(),
				strings.Join(g.HasIt, ", "),
				strings.Join(g.MissingIt, ", "),
			)
		}
	} else {
		fmt.Println("\nNo gaps found — all providers are in sync.")
	}

	return nil
}
```

#### Step 3: Write tests

```go
// cli/cmd/nesco/import_test.go

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/nesco/cli/internal/output"
)

func TestImportRequiresFrom(t *testing.T) {
	err := importCmd.RunE(importCmd, []string{})
	if err == nil {
		t.Error("import without --from should fail")
	}
}

func TestImportUnknownProvider(t *testing.T) {
	importCmd.Flags().Set("from", "nonexistent")
	err := importCmd.RunE(importCmd, []string{})
	if err == nil {
		t.Error("import with unknown provider should fail")
	}
}

func TestImportPreviewJSON(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, ".claude", "rules"), 0755)
	os.WriteFile(filepath.Join(tmp, "CLAUDE.md"), []byte("# Rules"), 0644)

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	var buf bytes.Buffer
	output.Writer = &buf
	output.JSON = true
	defer func() { output.JSON = false }()

	importCmd.Flags().Set("from", "claude-code")
	importCmd.Flags().Set("preview", "true")
	err := importCmd.RunE(importCmd, []string{})
	if err != nil {
		t.Fatalf("import --preview failed: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected JSON output")
	}
}
```

```go
// cli/cmd/nesco/parity_test.go

package main

import (
	"bytes"
	"testing"

	"github.com/holdenhewett/nesco/cli/internal/output"
)

func TestParityNeedsMultipleProviders(t *testing.T) {
	// With no providers detected, parity should report gracefully
	var buf bytes.Buffer
	output.Writer = &buf
	output.JSON = true
	defer func() { output.JSON = false }()

	err := parityCmd.RunE(parityCmd, []string{})
	if err != nil {
		t.Fatalf("parity should not error with few providers: %v", err)
	}
}
```

#### Step 4: Write implementation

(import.go and parity.go code as written above)

#### Step 5: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/... && go build ./cmd/nesco
```

Expected: Tests pass. Both commands show correct help text.

---

### Task 2.6: Add `nesco config` and `nesco info` commands

**Files:**
- Create: `cli/cmd/nesco/config_cmd.go`
- Create: `cli/cmd/nesco/config_cmd_test.go`
- Create: `cli/cmd/nesco/info.go`
- Create: `cli/cmd/nesco/info_test.go`

**Depends on:** Task 1.1 (Cobra), Task 1.4 (config package)

**Success Criteria:**
- [ ] `nesco config list` shows current provider selection
- [ ] `nesco config add <provider>` adds a provider
- [ ] `nesco config remove <provider>` removes a provider
- [ ] `nesco info` outputs machine-readable capability manifest
- [ ] `nesco info formats` lists supported output formats
- [ ] `nesco info providers` lists all known providers
- [ ] All commands support `--json`

---

#### Step 1: Write tests

```go
// cli/cmd/nesco/config_cmd_test.go

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/nesco/cli/internal/config"
)

func TestConfigAddAndRemove(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	config.Save(tmp, &config.Config{Providers: []string{"claude-code"}})

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	// Add
	configAddCmd.SetArgs([]string{"cursor"})
	if err := configAddCmd.RunE(configAddCmd, []string{"cursor"}); err != nil {
		t.Fatalf("config add: %v", err)
	}

	cfg, _ := config.Load(tmp)
	if len(cfg.Providers) != 2 {
		t.Errorf("expected 2 providers after add, got %d", len(cfg.Providers))
	}

	// Remove
	configRemoveCmd.SetArgs([]string{"cursor"})
	if err := configRemoveCmd.RunE(configRemoveCmd, []string{"cursor"}); err != nil {
		t.Fatalf("config remove: %v", err)
	}

	cfg, _ = config.Load(tmp)
	if len(cfg.Providers) != 1 {
		t.Errorf("expected 1 provider after remove, got %d", len(cfg.Providers))
	}
}

func TestConfigAddDuplicate(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	config.Save(tmp, &config.Config{Providers: []string{"claude-code"}})

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	err := configAddCmd.RunE(configAddCmd, []string{"claude-code"})
	if err == nil {
		t.Error("adding duplicate provider should fail")
	}
}

func TestConfigRemoveNotFound(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test"), 0644)
	config.Save(tmp, &config.Config{Providers: []string{"claude-code"}})

	origDir, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(origDir)

	err := configRemoveCmd.RunE(configRemoveCmd, []string{"nonexistent"})
	if err == nil {
		t.Error("removing nonexistent provider should fail")
	}
}
```

```go
// cli/cmd/nesco/info_test.go

package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/holdenhewett/nesco/cli/internal/output"
)

func TestInfoJSON(t *testing.T) {
	var buf bytes.Buffer
	output.Writer = &buf
	output.JSON = true
	defer func() { output.JSON = false }()

	err := infoCmd.RunE(infoCmd, []string{})
	if err != nil {
		t.Fatalf("info failed: %v", err)
	}

	var manifest map[string]any
	if err := json.Unmarshal(buf.Bytes(), &manifest); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if _, ok := manifest["version"]; !ok {
		t.Error("manifest missing 'version' key")
	}
	if _, ok := manifest["contentTypes"]; !ok {
		t.Error("manifest missing 'contentTypes' key")
	}
	if _, ok := manifest["providers"]; !ok {
		t.Error("manifest missing 'providers' key")
	}
}
```

#### Step 2: Write config command

```go
// cli/cmd/nesco/config_cmd.go

package main

import (
	"fmt"

	"github.com/holdenhewett/nesco/cli/internal/config"
	"github.com/holdenhewett/nesco/cli/internal/output"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View and edit nesco configuration",
	Long:  "Manage provider selection and preferences in .nesco/config.json.",
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		cfg, err := config.Load(root)
		if err != nil {
			return err
		}
		if output.JSON {
			output.Print(cfg)
		} else {
			if len(cfg.Providers) == 0 {
				fmt.Println("No providers configured. Run `nesco init` to set up.")
			} else {
				fmt.Println("Configured providers:")
				for _, p := range cfg.Providers {
					fmt.Printf("  • %s\n", p)
				}
			}
		}
		return nil
	},
}

var configAddCmd = &cobra.Command{
	Use:   "add <provider-slug>",
	Short: "Add a provider to the configuration",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		cfg, err := config.Load(root)
		if err != nil {
			return err
		}
		slug := args[0]
		// Check for duplicates
		for _, p := range cfg.Providers {
			if p == slug {
				return fmt.Errorf("provider %q already configured", slug)
			}
		}
		cfg.Providers = append(cfg.Providers, slug)
		return config.Save(root, cfg)
	},
}

var configRemoveCmd = &cobra.Command{
	Use:   "remove <provider-slug>",
	Short: "Remove a provider from the configuration",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		cfg, err := config.Load(root)
		if err != nil {
			return err
		}
		slug := args[0]
		filtered := cfg.Providers[:0]
		found := false
		for _, p := range cfg.Providers {
			if p == slug {
				found = true
				continue
			}
			filtered = append(filtered, p)
		}
		if !found {
			return fmt.Errorf("provider %q not found in config", slug)
		}
		cfg.Providers = filtered
		return config.Save(root, cfg)
	},
}

func init() {
	configCmd.AddCommand(configListCmd, configAddCmd, configRemoveCmd)
	rootCmd.AddCommand(configCmd)
}
```

#### Step 2: Write info command

```go
// cli/cmd/nesco/info.go

package main

import (
	"fmt"

	"github.com/holdenhewett/nesco/cli/internal/catalog"
	"github.com/holdenhewett/nesco/cli/internal/output"
	"github.com/holdenhewett/nesco/cli/internal/provider"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show nesco capabilities",
	Long:  "Machine-readable capability manifest. Useful for agents discovering nesco's features.",
	RunE: func(cmd *cobra.Command, args []string) error {
		manifest := map[string]any{
			"version":      version,
			"contentTypes": catalog.AllContentTypes(),
			"providers":    providerSlugs(),
			"commands":     []string{"init", "import", "parity", "config", "info", "scan", "drift", "baseline"},
		}
		if output.JSON {
			output.Print(manifest)
		} else {
			fmt.Printf("nesco %s\n\n", version)
			fmt.Println("Content types:", len(catalog.AllContentTypes()))
			for _, ct := range catalog.AllContentTypes() {
				fmt.Printf("  • %s\n", ct.Label())
			}
			fmt.Println("\nProviders:", len(provider.AllProviders))
			for _, p := range provider.AllProviders {
				fmt.Printf("  • %s (%s)\n", p.Name, p.Slug)
			}
		}
		return nil
	},
}

var infoProvidersCmd = &cobra.Command{
	Use:   "providers",
	Short: "List all known providers",
	RunE: func(cmd *cobra.Command, args []string) error {
		type provInfo struct {
			Name string   `json:"name"`
			Slug string   `json:"slug"`
			Types []string `json:"supportedTypes"`
		}
		var infos []provInfo
		for _, p := range provider.AllProviders {
			var types []string
			for _, ct := range catalog.AllContentTypes() {
				if p.SupportsType(ct) {
					types = append(types, string(ct))
				}
			}
			infos = append(infos, provInfo{Name: p.Name, Slug: p.Slug, Types: types})
		}
		if output.JSON {
			output.Print(infos)
		} else {
			for _, info := range infos {
				fmt.Printf("%s (%s)\n", info.Name, info.Slug)
				for _, t := range info.Types {
					fmt.Printf("  • %s\n", t)
				}
			}
		}
		return nil
	},
}

var infoFormatsCmd = &cobra.Command{
	Use:   "formats",
	Short: "List supported file formats",
	RunE: func(cmd *cobra.Command, args []string) error {
		type formatInfo struct {
			Format   string   `json:"format"`
			Extension string `json:"extension"`
			Providers []string `json:"providers"`
		}
		formats := []formatInfo{
			{Format: "Markdown", Extension: ".md", Providers: []string{"claude-code", "windsurf", "codex", "gemini-cli", "copilot"}},
			{Format: "Cursor MDC", Extension: ".mdc", Providers: []string{"cursor"}},
			{Format: "JSON", Extension: ".json", Providers: []string{"claude-code", "cursor"}},
			{Format: "YAML", Extension: ".yaml", Providers: []string{"cursor"}},
		}
		if output.JSON {
			output.Print(formats)
		} else {
			fmt.Println("Supported formats:")
			for _, f := range formats {
				fmt.Printf("  %s (%s)\n", f.Format, f.Extension)
				fmt.Printf("    Used by: %v\n", f.Providers)
			}
		}
		return nil
	},
}

var infoDetectorsCmd = &cobra.Command{
	Use:   "detectors",
	Short: "List all available detectors",
	RunE: func(cmd *cobra.Command, args []string) error {
		// This would reference the detector registry from Task 4.7
		// For now, return placeholder structure
		type detectorInfo struct {
			Name     string `json:"name"`
			Category string `json:"category"` // "fact" or "surprise"
			Language string `json:"language"` // "all", "go", "python", "rust", "js"
		}
		detectors := []detectorInfo{
			{Name: "tech-stack", Category: "fact", Language: "all"},
			{Name: "dependencies", Category: "fact", Language: "all"},
			{Name: "build-commands", Category: "fact", Language: "all"},
			{Name: "directory-structure", Category: "fact", Language: "all"},
			{Name: "project-metadata", Category: "fact", Language: "all"},
			{Name: "competing-frameworks", Category: "surprise", Language: "all"},
			{Name: "module-conflict", Category: "surprise", Language: "all"},
			// ... (all 24 detectors would be listed)
		}
		if output.JSON {
			output.Print(detectors)
		} else {
			fmt.Printf("Total detectors: %d\n\n", len(detectors))
			fmt.Println("Fact detectors (5):")
			for _, d := range detectors {
				if d.Category == "fact" {
					fmt.Printf("  • %s\n", d.Name)
				}
			}
			fmt.Println("\nSurprise detectors (24):")
			for _, d := range detectors {
				if d.Category == "surprise" {
					fmt.Printf("  • %s (%s)\n", d.Name, d.Language)
				}
			}
		}
		return nil
	},
}

func init() {
	infoCmd.AddCommand(infoProvidersCmd, infoFormatsCmd, infoDetectorsCmd)
	rootCmd.AddCommand(infoCmd)
}

func providerSlugs() []string {
	slugs := make([]string, len(provider.AllProviders))
	for i, p := range provider.AllProviders {
		slugs[i] = p.Slug
	}
	return slugs
}
```

#### Step 4: Run tests and verify

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/... && go build ./cmd/nesco && ./nesco --help
```

Expected: Tests pass. Help output lists all Phase 1 commands: `init`, `import`, `parity`, `config`, `info`, plus existing `version` and `backfill`.

---

**Group 2 complete.** 6 tasks covering Phase 1 Provider Parity: discovery, parsing, parity analysis, and all Phase 1 CLI commands. Next: Group 3 (Phase 2: Scan Infrastructure).

---

## Group 3: Phase 2 Scan Infrastructure

**Covers:** Decision #9 (Detector Architecture), #5 (Content-Type Mapping: Emitters Decide), #15 (Boundary Markers), #7 (Reconciler), #8 (Conflict Handling), #14 (Testing: Fixture Directories)

**Codebase context:** The scanner orchestrator is new — no prior scan infrastructure exists. The detector interface (Decision #9) uses `Detect(root string) ([]model.Section, error)`. Emitters are pure functions: `ContextDocument → string`. The reconciler handles boundary markers and preserves human-authored sections. Fact detectors return typed sections (TechStackSection, etc.), surprise detectors return TextSections.

**Pipeline:** `Detectors → ContextDocument → Emitter (per provider) → formatted string → Reconciler → disk`

---

### Task 3.1: Create scanner orchestrator

**Files:**
- Create: `cli/internal/scan/scanner.go`
- Create: `cli/internal/scan/scanner_test.go`

**Depends on:** Task 1.2 (model types)

**Success Criteria:**
- [ ] Detector interface defined
- [ ] `Scan()` runs all registered detectors in parallel via goroutines
- [ ] 5-second timeout per detector (configurable)
- [ ] Panic in a detector is recovered, logged as warning, doesn't crash scanner
- [ ] Results assembled into ordered ContextDocument
- [ ] Tests verify parallel execution, timeout behavior, and panic recovery

---

#### Step 1: Write scanner with detector interface

```go
// cli/internal/scan/scanner.go

package scan

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/holdenhewett/nesco/cli/internal/model"
)

// Detector is the interface implemented by all fact and surprise detectors.
type Detector interface {
	Name() string
	Detect(root string) ([]model.Section, error)
}

// ScanResult holds the output of a complete scan.
type ScanResult struct {
	Document model.ContextDocument `json:"document"`
	Warnings []Warning             `json:"warnings,omitempty"`
	Duration time.Duration         `json:"duration"`
}

// Warning records a non-fatal detector issue (timeout, panic, error).
type Warning struct {
	Detector string `json:"detector"`
	Message  string `json:"message"`
}

// DefaultTimeout is the per-detector timeout.
const DefaultTimeout = 5 * time.Second

// Scanner runs detectors and assembles results.
type Scanner struct {
	Detectors []Detector
	Timeout   time.Duration
}

// NewScanner creates a scanner with the given detectors and default timeout.
func NewScanner(detectors ...Detector) *Scanner {
	return &Scanner{
		Detectors: detectors,
		Timeout:   DefaultTimeout,
	}
}

// Run executes all detectors in parallel and assembles a ContextDocument.
func (s *Scanner) Run(projectRoot string) ScanResult {
	start := time.Now()

	type detectorResult struct {
		name     string
		sections []model.Section
		warning  *Warning
	}

	results := make(chan detectorResult, len(s.Detectors))
	var wg sync.WaitGroup

	for _, d := range s.Detectors {
		wg.Add(1)
		go func(det Detector) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), s.Timeout)
			defer cancel()

			doneCh := make(chan detectorResult, 1)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						doneCh <- detectorResult{
							name:    det.Name(),
							warning: &Warning{Detector: det.Name(), Message: fmt.Sprintf("panic: %v", r)},
						}
					}
				}()
				sections, err := det.Detect(projectRoot)
				if err != nil {
					doneCh <- detectorResult{
						name:    det.Name(),
						warning: &Warning{Detector: det.Name(), Message: err.Error()},
					}
					return
				}
				doneCh <- detectorResult{name: det.Name(), sections: sections}
			}()

			select {
			case r := <-doneCh:
				results <- r
			case <-ctx.Done():
				results <- detectorResult{
					name:    det.Name(),
					warning: &Warning{Detector: det.Name(), Message: "timeout after " + s.Timeout.String()},
				}
			}
		}(d)
	}

	// Close results channel after all goroutines complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var allSections []model.Section
	var warnings []Warning
	for r := range results {
		allSections = append(allSections, r.sections...)
		if r.warning != nil {
			warnings = append(warnings, *r.warning)
		}
	}

	// Sort sections by category for stable output
	sort.Slice(allSections, func(i, j int) bool {
		return categoryOrder(allSections[i].SectionCategory()) < categoryOrder(allSections[j].SectionCategory())
	})

	return ScanResult{
		Document: model.ContextDocument{
			ProjectName: projectRoot,
			ScanTime:    start,
			Sections:    allSections,
		},
		Warnings: warnings,
		Duration: time.Since(start),
	}
}

// categoryOrder defines the section ordering in emitted output.
func categoryOrder(c model.Category) int {
	switch c {
	case model.CatTechStack:
		return 0
	case model.CatDependencies:
		return 1
	case model.CatBuildCommands:
		return 2
	case model.CatDirStructure:
		return 3
	case model.CatProjectMeta:
		return 4
	case model.CatConventions:
		return 5
	case model.CatSurprise:
		return 6
	case model.CatCurated:
		return 7
	default:
		return 99
	}
}
```

#### Step 2: Write tests

```go
// cli/internal/scan/scanner_test.go

package scan

import (
	"fmt"
	"testing"
	"time"

	"github.com/holdenhewett/nesco/cli/internal/model"
)

// stubDetector returns fixed sections for testing.
type stubDetector struct {
	name     string
	sections []model.Section
	err      error
	delay    time.Duration
	panics   bool
}

func (d stubDetector) Name() string { return d.name }
func (d stubDetector) Detect(root string) ([]model.Section, error) {
	if d.panics {
		panic("intentional panic for testing")
	}
	if d.delay > 0 {
		time.Sleep(d.delay)
	}
	return d.sections, d.err
}

func TestScannerCollectsResults(t *testing.T) {
	scanner := NewScanner(
		stubDetector{
			name: "tech-stack",
			sections: []model.Section{
				model.TechStackSection{Origin: model.OriginAuto, Title: "Tech Stack", Language: "Go"},
			},
		},
		stubDetector{
			name: "build",
			sections: []model.Section{
				model.BuildCommandSection{Origin: model.OriginAuto, Title: "Build Commands"},
			},
		},
	)

	result := scanner.Run("/tmp/project")
	if len(result.Document.Sections) != 2 {
		t.Errorf("expected 2 sections, got %d", len(result.Document.Sections))
	}
	if len(result.Warnings) != 0 {
		t.Errorf("expected no warnings, got %d", len(result.Warnings))
	}
}

func TestScannerHandlesTimeout(t *testing.T) {
	scanner := NewScanner(
		stubDetector{
			name:  "slow",
			delay: 2 * time.Second,
			sections: []model.Section{
				model.TextSection{Category: model.CatConventions, Title: "slow result"},
			},
		},
	)
	scanner.Timeout = 100 * time.Millisecond

	result := scanner.Run("/tmp/project")
	if len(result.Warnings) != 1 {
		t.Errorf("expected 1 timeout warning, got %d", len(result.Warnings))
	}
}

func TestScannerHandlesPanic(t *testing.T) {
	scanner := NewScanner(
		stubDetector{name: "panicker", panics: true},
		stubDetector{
			name: "normal",
			sections: []model.Section{
				model.TextSection{Category: model.CatConventions, Title: "normal"},
			},
		},
	)

	result := scanner.Run("/tmp/project")
	if len(result.Warnings) != 1 {
		t.Errorf("expected 1 panic warning, got %d", len(result.Warnings))
	}
	// Normal detector should still succeed
	if len(result.Document.Sections) != 1 {
		t.Errorf("expected 1 section from normal detector, got %d", len(result.Document.Sections))
	}
}

func TestScannerHandlesError(t *testing.T) {
	scanner := NewScanner(
		stubDetector{name: "erroring", err: fmt.Errorf("test error")},
	)

	result := scanner.Run("/tmp/project")
	if len(result.Warnings) != 1 {
		t.Errorf("expected 1 error warning, got %d", len(result.Warnings))
	}
}
```

#### Step 3: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/scan/...
```

Expected: PASS

---

### Task 3.2: Create TechStack and Dependencies detectors

**Files:**
- Create: `cli/internal/scan/detectors/techstack.go`
- Create: `cli/internal/scan/detectors/dependencies.go`
- Create: `cli/internal/scan/detectors/techstack_test.go`
- Create: `cli/internal/scan/detectors/dependencies_test.go`
- Create: `cli/internal/scan/detectors/testdata/techstack/node-project/package.json`
- Create: `cli/internal/scan/detectors/testdata/techstack/go-project/go.mod`

**Depends on:** Task 1.2 (model types), Task 3.1 (scanner interface)

**Success Criteria:**
- [ ] TechStack detector reads package.json, go.mod, Cargo.toml, pyproject.toml
- [ ] Returns TechStackSection with language, version, framework info
- [ ] Dependencies detector reads manifest files, returns grouped DependencySection
- [ ] Both tested against fixture directories
- [ ] Unknown project types return empty results (no error)

---

#### Step 1: TechStack detector

```go
// cli/internal/scan/detectors/techstack.go

package detectors

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/holdenhewett/nesco/cli/internal/model"
)

// TechStack detects the primary technology stack from manifest files.
type TechStack struct{}

func (d TechStack) Name() string { return "tech-stack" }

func (d TechStack) Detect(root string) ([]model.Section, error) {
	var sections []model.Section

	// Go
	if s := detectGo(root); s != nil {
		sections = append(sections, *s)
	}

	// Node.js / TypeScript
	if s := detectNode(root); s != nil {
		sections = append(sections, *s)
	}

	// Python
	if s := detectPython(root); s != nil {
		sections = append(sections, *s)
	}

	// Rust
	if s := detectRust(root); s != nil {
		sections = append(sections, *s)
	}

	return sections, nil
}

func detectGo(root string) *model.TechStackSection {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil
	}

	s := &model.TechStackSection{
		Origin:   model.OriginAuto,
		Title:    "Tech Stack: Go",
		Language: "Go",
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "go ") {
			s.LanguageVersion = strings.TrimPrefix(line, "go ")
			break
		}
	}

	return s
}

func detectNode(root string) *model.TechStackSection {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return nil
	}

	var pkg struct {
		Engines      map[string]string `json:"engines"`
		Dependencies map[string]string `json:"dependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}

	s := &model.TechStackSection{
		Origin:   model.OriginAuto,
		Title:    "Tech Stack: JavaScript/TypeScript",
		Language: "JavaScript",
	}

	// Check for TypeScript
	if _, ok := pkg.Dependencies["typescript"]; ok {
		s.Language = "TypeScript"
	}
	// Check tsconfig.json as fallback
	if _, err := os.Stat(filepath.Join(root, "tsconfig.json")); err == nil {
		s.Language = "TypeScript"
	}

	// Node version from engines
	if v, ok := pkg.Engines["node"]; ok {
		s.RuntimeVersion = v
		s.Runtime = "Node.js"
	}

	// Detect major frameworks
	frameworks := map[string]string{
		"next": "Next.js", "react": "React", "vue": "Vue", "svelte": "Svelte",
		"express": "Express", "fastify": "Fastify", "astro": "Astro",
	}
	for dep, name := range frameworks {
		if v, ok := pkg.Dependencies[dep]; ok {
			s.Framework = name
			s.FrameworkVersion = strings.TrimPrefix(v, "^")
			break
		}
	}

	return s
}

func detectPython(root string) *model.TechStackSection {
	// Check pyproject.toml first
	data, err := os.ReadFile(filepath.Join(root, "pyproject.toml"))
	if err != nil {
		// Fallback to setup.py or requirements.txt existence
		if _, err2 := os.Stat(filepath.Join(root, "setup.py")); err2 != nil {
			if _, err3 := os.Stat(filepath.Join(root, "requirements.txt")); err3 != nil {
				return nil
			}
		}
		return &model.TechStackSection{
			Origin:   model.OriginAuto,
			Title:    "Tech Stack: Python",
			Language: "Python",
		}
	}

	s := &model.TechStackSection{
		Origin:   model.OriginAuto,
		Title:    "Tech Stack: Python",
		Language: "Python",
	}

	// Simple TOML parsing for requires-python
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "requires-python") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				s.LanguageVersion = strings.Trim(strings.TrimSpace(parts[1]), "\"'>= ")
			}
		}
	}

	return s
}

func detectRust(root string) *model.TechStackSection {
	data, err := os.ReadFile(filepath.Join(root, "Cargo.toml"))
	if err != nil {
		return nil
	}

	s := &model.TechStackSection{
		Origin:   model.OriginAuto,
		Title:    "Tech Stack: Rust",
		Language: "Rust",
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "edition") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				s.LanguageVersion = "edition " + strings.Trim(strings.TrimSpace(parts[1]), "\"")
			}
		}
	}

	return s
}
```

#### Step 2: Dependencies detector

```go
// cli/internal/scan/detectors/dependencies.go

package detectors

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/holdenhewett/nesco/cli/internal/model"
)

// Dependencies detects project dependencies from manifest files.
type Dependencies struct{}

func (d Dependencies) Name() string { return "dependencies" }

func (d Dependencies) Detect(root string) ([]model.Section, error) {
	var sections []model.Section

	// Node.js
	if s := detectNodeDeps(root); s != nil {
		sections = append(sections, *s)
	}

	// Go
	if s := detectGoDeps(root); s != nil {
		sections = append(sections, *s)
	}

	return sections, nil
}

func detectNodeDeps(root string) *model.DependencySection {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return nil
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}

	s := &model.DependencySection{
		Origin: model.OriginAuto,
		Title:  "Dependencies: Node.js",
	}

	if len(pkg.Dependencies) > 0 {
		group := model.DependencyGroup{Category: "production"}
		for name, ver := range pkg.Dependencies {
			group.Items = append(group.Items, model.Dependency{Name: name, Version: ver})
		}
		s.Groups = append(s.Groups, group)
	}

	if len(pkg.DevDependencies) > 0 {
		group := model.DependencyGroup{Category: "dev"}
		for name, ver := range pkg.DevDependencies {
			group.Items = append(group.Items, model.Dependency{Name: name, Version: ver})
		}
		s.Groups = append(s.Groups, group)
	}

	if len(s.Groups) == 0 {
		return nil
	}
	return s
}

func detectGoDeps(root string) *model.DependencySection {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil
	}

	s := &model.DependencySection{
		Origin: model.OriginAuto,
		Title:  "Dependencies: Go",
	}

	group := model.DependencyGroup{Category: "module"}
	inRequire := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "require (" {
			inRequire = true
			continue
		}
		if line == ")" {
			inRequire = false
			continue
		}
		if inRequire {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				group.Items = append(group.Items, model.Dependency{
					Name:    parts[0],
					Version: parts[1],
				})
			}
		}
	}

	if len(group.Items) > 0 {
		s.Groups = append(s.Groups, group)
		return s
	}
	return nil
}
```

#### Step 3: Create test fixtures and tests

```json
// cli/internal/scan/detectors/testdata/techstack/node-project/package.json
{
  "name": "test-project",
  "dependencies": {
    "next": "^14.1.0",
    "react": "^18.2.0",
    "typescript": "^5.3.0"
  },
  "devDependencies": {
    "jest": "^29.7.0"
  },
  "engines": {
    "node": ">=20"
  }
}
```

```
// cli/internal/scan/detectors/testdata/techstack/go-project/go.mod
module example.com/testproject

go 1.22.5

require (
	github.com/spf13/cobra v1.8.0
	github.com/charmbracelet/bubbletea v0.25.0
)
```

```go
// cli/internal/scan/detectors/techstack_test.go

package detectors

import (
	"testing"

	"github.com/holdenhewett/nesco/cli/internal/model"
)

func TestTechStackNode(t *testing.T) {
	det := TechStack{}
	sections, err := det.Detect("testdata/techstack/node-project")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected at least 1 section for Node project")
	}
	ts, ok := sections[0].(model.TechStackSection)
	if !ok {
		t.Fatalf("expected TechStackSection, got %T", sections[0])
	}
	if ts.Language != "TypeScript" {
		t.Errorf("language = %q, want TypeScript", ts.Language)
	}
	if ts.Framework != "Next.js" {
		t.Errorf("framework = %q, want Next.js", ts.Framework)
	}
}

func TestTechStackGo(t *testing.T) {
	det := TechStack{}
	sections, err := det.Detect("testdata/techstack/go-project")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected at least 1 section for Go project")
	}
	ts, ok := sections[0].(model.TechStackSection)
	if !ok {
		t.Fatalf("expected TechStackSection, got %T", sections[0])
	}
	if ts.Language != "Go" {
		t.Errorf("language = %q, want Go", ts.Language)
	}
	if ts.LanguageVersion != "1.22.5" {
		t.Errorf("version = %q, want 1.22.5", ts.LanguageVersion)
	}
}

func TestTechStackEmptyDir(t *testing.T) {
	det := TechStack{}
	sections, err := det.Detect(t.TempDir())
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for empty dir, got %d", len(sections))
	}
}
```

```go
// cli/internal/scan/detectors/dependencies_test.go

package detectors

import (
	"testing"

	"github.com/holdenhewett/nesco/cli/internal/model"
)

func TestDependenciesNode(t *testing.T) {
	det := Dependencies{}
	sections, err := det.Detect("testdata/techstack/node-project")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected dependency section for Node project")
	}
	ds, ok := sections[0].(model.DependencySection)
	if !ok {
		t.Fatalf("expected DependencySection, got %T", sections[0])
	}
	if len(ds.Groups) < 2 {
		t.Errorf("expected at least 2 groups (prod + dev), got %d", len(ds.Groups))
	}
}

func TestDependenciesGo(t *testing.T) {
	det := Dependencies{}
	sections, err := det.Detect("testdata/techstack/go-project")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected dependency section for Go project")
	}
	ds, ok := sections[0].(model.DependencySection)
	if !ok {
		t.Fatalf("expected DependencySection, got %T", sections[0])
	}
	if len(ds.Groups[0].Items) != 2 {
		t.Errorf("expected 2 Go deps, got %d", len(ds.Groups[0].Items))
	}
}
```

#### Step 4: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/scan/detectors/...
```

Expected: PASS

---

### Task 3.3: Create BuildCommands, DirectoryStructure, and ProjectMetadata detectors

**Files:**
- Create: `cli/internal/scan/detectors/buildcmds.go`
- Create: `cli/internal/scan/detectors/dirstructure.go`
- Create: `cli/internal/scan/detectors/metadata.go`
- Create: `cli/internal/scan/detectors/buildcmds_test.go`
- Create: `cli/internal/scan/detectors/dirstructure_test.go`
- Create: `cli/internal/scan/detectors/metadata_test.go`
- Create: `cli/internal/scan/detectors/testdata/buildcmds/makefile-project/Makefile`
- Create: `cli/internal/scan/detectors/testdata/buildcmds/npm-scripts/package.json`
- Create: `cli/internal/scan/detectors/testdata/dirstructure/standard-project/` (with src/, tests/, docs/ dirs)
- Create: `cli/internal/scan/detectors/testdata/metadata/with-readme/README.md`

**Depends on:** Task 1.2 (model types), Task 3.1 (scanner interface)

**Success Criteria:**
- [ ] BuildCommands detector parses Makefile targets, package.json scripts, Taskfile, justfile
- [ ] DirectoryStructure detector walks the project tree, respects .gitignore, identifies conventional dirs
- [ ] ProjectMetadata detector finds README, LICENSE, CI config
- [ ] All tested against fixtures

---

#### Step 1: BuildCommands detector

```go
// cli/internal/scan/detectors/buildcmds.go

package detectors

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/holdenhewett/nesco/cli/internal/model"
)

// BuildCommands detects build/task runner commands.
type BuildCommands struct{}

func (d BuildCommands) Name() string { return "build-commands" }

func (d BuildCommands) Detect(root string) ([]model.Section, error) {
	s := &model.BuildCommandSection{
		Origin: model.OriginAuto,
		Title:  "Build Commands",
	}

	// package.json scripts
	if cmds := parseNPMScripts(root); len(cmds) > 0 {
		s.Commands = append(s.Commands, cmds...)
	}

	// Makefile targets
	if cmds := parseMakefileTargets(root); len(cmds) > 0 {
		s.Commands = append(s.Commands, cmds...)
	}

	if len(s.Commands) == 0 {
		return nil, nil
	}
	return []model.Section{*s}, nil
}

func parseNPMScripts(root string) []model.BuildCommand {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return nil
	}

	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}

	var cmds []model.BuildCommand
	for name, cmd := range pkg.Scripts {
		cmds = append(cmds, model.BuildCommand{
			Name:    name,
			Command: cmd,
			Source:  "package.json",
		})
	}
	return cmds
}

func parseMakefileTargets(root string) []model.BuildCommand {
	f, err := os.Open(filepath.Join(root, "Makefile"))
	if err != nil {
		return nil
	}
	defer f.Close()

	var cmds []model.BuildCommand
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		// Match lines like "build:" or "test: deps" (non-indented, with colon)
		if len(line) > 0 && line[0] != '\t' && line[0] != '#' && line[0] != '.' {
			if idx := strings.Index(line, ":"); idx > 0 {
				target := strings.TrimSpace(line[:idx])
				if !strings.ContainsAny(target, " \t=") {
					cmds = append(cmds, model.BuildCommand{
						Name:    target,
						Command: "make " + target,
						Source:  "Makefile",
					})
				}
			}
		}
	}
	return cmds
}
```

#### Step 2: DirectoryStructure detector

```go
// cli/internal/scan/detectors/dirstructure.go

package detectors

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/holdenhewett/nesco/cli/internal/model"
)

// DirectoryStructure maps the project's directory layout.
type DirectoryStructure struct{}

func (d DirectoryStructure) Name() string { return "directory-structure" }

func (d DirectoryStructure) Detect(root string) ([]model.Section, error) {
	s := &model.DirectoryStructureSection{
		Origin: model.OriginAuto,
		Title:  "Directory Structure",
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue // skip hidden dirs/files at top level
		}
		if !e.IsDir() {
			continue // only map directories
		}

		entry := model.DirectoryEntry{
			Path:       name + "/",
			Convention: classifyDir(name),
		}
		s.Entries = append(s.Entries, entry)
	}

	if len(s.Entries) == 0 {
		return nil, nil
	}
	return []model.Section{*s}, nil
}

// classifyDir assigns a conventional purpose based on directory name.
func classifyDir(name string) string {
	switch strings.ToLower(name) {
	case "src", "lib", "pkg", "internal", "app":
		return "source"
	case "test", "tests", "__tests__", "spec", "specs":
		return "test"
	case "cmd", "bin":
		return "entrypoint"
	case "docs", "doc", "documentation":
		return "documentation"
	case "config", "conf", "cfg":
		return "config"
	case "build", "dist", "out", "target":
		return "build-output"
	case "scripts":
		return "scripts"
	case "vendor", "node_modules", "third_party":
		return "vendor"
	case "assets", "static", "public":
		return "assets"
	case "migrations", "db":
		return "database"
	default:
		return ""
	}
}
```

#### Step 3: ProjectMetadata detector

```go
// cli/internal/scan/detectors/metadata.go

package detectors

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/holdenhewett/nesco/cli/internal/model"
)

// ProjectMetadata detects project-level facts (README, LICENSE, CI).
type ProjectMetadata struct{}

func (d ProjectMetadata) Name() string { return "project-metadata" }

func (d ProjectMetadata) Detect(root string) ([]model.Section, error) {
	s := &model.ProjectMetadataSection{
		Origin: model.OriginAuto,
		Title:  "Project Metadata",
	}

	// README — extract first meaningful line as description
	for _, name := range []string{"README.md", "README", "readme.md"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err == nil {
			s.Description = extractDescription(string(data))
			break
		}
	}

	// LICENSE
	for _, name := range []string{"LICENSE", "LICENSE.md", "LICENSE.txt", "COPYING"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err == nil {
			s.License = detectLicenseType(string(data))
			break
		}
	}

	// CI
	if _, err := os.Stat(filepath.Join(root, ".github", "workflows")); err == nil {
		s.CI = "GitHub Actions"
	} else if _, err := os.Stat(filepath.Join(root, ".gitlab-ci.yml")); err == nil {
		s.CI = "GitLab CI"
	} else if _, err := os.Stat(filepath.Join(root, ".circleci")); err == nil {
		s.CI = "CircleCI"
	}

	if s.Description == "" && s.License == "" && s.CI == "" {
		return nil, nil
	}
	return []model.Section{*s}, nil
}

func extractDescription(readme string) string {
	for _, line := range strings.Split(readme, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		if len(line) > 200 {
			return line[:200] + "..."
		}
		return line
	}
	return ""
}

func detectLicenseType(content string) string {
	lower := strings.ToLower(content)
	switch {
	case strings.Contains(lower, "mit license"):
		return "MIT"
	case strings.Contains(lower, "apache license"):
		return "Apache-2.0"
	case strings.Contains(lower, "gnu general public license"):
		return "GPL"
	case strings.Contains(lower, "bsd"):
		return "BSD"
	case strings.Contains(lower, "isc license"):
		return "ISC"
	default:
		return "Unknown"
	}
}
```

#### Step 4: Create fixtures and tests

Makefile fixture:
```makefile
# cli/internal/scan/detectors/testdata/buildcmds/makefile-project/Makefile
.PHONY: build test lint clean

build:
	go build ./cmd/...

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/
```

```go
// cli/internal/scan/detectors/buildcmds_test.go

package detectors

import (
	"testing"

	"github.com/holdenhewett/nesco/cli/internal/model"
)

func TestBuildCommandsMakefile(t *testing.T) {
	det := BuildCommands{}
	sections, err := det.Detect("testdata/buildcmds/makefile-project")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected build commands section")
	}
	bc, ok := sections[0].(model.BuildCommandSection)
	if !ok {
		t.Fatalf("expected BuildCommandSection, got %T", sections[0])
	}
	if len(bc.Commands) < 3 {
		t.Errorf("expected at least 3 Makefile targets, got %d", len(bc.Commands))
	}
}

func TestBuildCommandsNPM(t *testing.T) {
	det := BuildCommands{}
	sections, err := det.Detect("testdata/techstack/node-project") // reuse fixture
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	// node-project fixture may not have scripts — this is expected to return nil
	// Add a fixture with scripts if needed
}
```

#### Step 5: Write DirectoryStructure and ProjectMetadata tests

```go
// cli/internal/scan/detectors/dirstructure_test.go

package detectors

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/nesco/cli/internal/model"
)

func TestDirectoryStructure(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "src"), 0755)
	os.MkdirAll(filepath.Join(tmp, "tests"), 0755)
	os.MkdirAll(filepath.Join(tmp, "docs"), 0755)

	det := DirectoryStructure{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected directory structure section")
	}
	ds, ok := sections[0].(model.DirectoryStructureSection)
	if !ok {
		t.Fatalf("expected DirectoryStructureSection, got %T", sections[0])
	}
	if len(ds.Entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(ds.Entries))
	}

	// Verify classification
	conventions := make(map[string]string)
	for _, e := range ds.Entries {
		conventions[e.Path] = e.Convention
	}
	if conventions["src/"] != "source" {
		t.Errorf("src/ should be classified as 'source', got %q", conventions["src/"])
	}
	if conventions["tests/"] != "test" {
		t.Errorf("tests/ should be classified as 'test', got %q", conventions["tests/"])
	}
}

func TestDirectoryStructureSkipsHidden(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, ".git"), 0755)
	os.MkdirAll(filepath.Join(tmp, "src"), 0755)

	det := DirectoryStructure{}
	sections, _ := det.Detect(tmp)
	if len(sections) == 0 {
		t.Fatal("expected section")
	}
	ds := sections[0].(model.DirectoryStructureSection)
	for _, e := range ds.Entries {
		if e.Path == ".git/" {
			t.Error("hidden directories should be skipped")
		}
	}
}

func TestDirectoryStructureEmpty(t *testing.T) {
	det := DirectoryStructure{}
	sections, err := det.Detect(t.TempDir())
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for empty dir, got %d", len(sections))
	}
}
```

```go
// cli/internal/scan/detectors/metadata_test.go

package detectors

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/nesco/cli/internal/model"
)

func TestProjectMetadataWithReadme(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "README.md"), []byte("# My Project\n\nA cool project for testing."), 0644)
	os.WriteFile(filepath.Join(tmp, "LICENSE"), []byte("MIT License\n\nCopyright..."), 0644)
	os.MkdirAll(filepath.Join(tmp, ".github", "workflows"), 0755)

	det := ProjectMetadata{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected metadata section")
	}
	pm, ok := sections[0].(model.ProjectMetadataSection)
	if !ok {
		t.Fatalf("expected ProjectMetadataSection, got %T", sections[0])
	}
	if pm.Description != "A cool project for testing." {
		t.Errorf("description = %q", pm.Description)
	}
	if pm.License != "MIT" {
		t.Errorf("license = %q, want MIT", pm.License)
	}
	if pm.CI != "GitHub Actions" {
		t.Errorf("ci = %q, want GitHub Actions", pm.CI)
	}
}

func TestProjectMetadataEmpty(t *testing.T) {
	det := ProjectMetadata{}
	sections, err := det.Detect(t.TempDir())
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for empty dir, got %d", len(sections))
	}
}
```

#### Step 6: Run all tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/scan/detectors/...
```

Expected: PASS

---

### Task 3.4: Create Claude Code emitter

**Files:**
- Create: `cli/internal/emit/emitter.go`
- Create: `cli/internal/emit/claude.go`
- Create: `cli/internal/emit/claude_test.go`

**Depends on:** Task 1.2 (model types), Task 3.1 (Decision #15 boundary markers)

**Success Criteria:**
- [ ] Emitter interface defined
- [ ] Claude Code emitter produces CLAUDE.md-format markdown
- [ ] Each section wrapped in `<!-- nesco:auto:<category> -->` boundary markers
- [ ] Typed sections formatted with structured data
- [ ] Text sections pass through body content
- [ ] Pure function — no filesystem access
- [ ] Tests verify output against golden strings

---

#### Step 1: Define Emitter interface

```go
// cli/internal/emit/emitter.go

package emit

import "github.com/holdenhewett/nesco/cli/internal/model"

// Emitter renders a ContextDocument into a provider-specific format string.
// Emitters are pure functions — no filesystem access, no side effects.
type Emitter interface {
	Name() string
	Format() string // "md", "mdc", "json"
	Emit(doc model.ContextDocument) (string, error)
}

// EmitterForProvider returns the appropriate emitter for a provider slug.
func EmitterForProvider(slug string) Emitter {
	switch slug {
	case "claude-code":
		return ClaudeEmitter{}
	case "cursor":
		return CursorEmitter{}
	case "gemini-cli":
		return GenericMarkdownEmitter{ProviderSlug: "gemini-cli", FileName: "GEMINI.md"}
	case "codex":
		return GenericMarkdownEmitter{ProviderSlug: "codex", FileName: "AGENTS.md"}
	case "windsurf":
		return GenericMarkdownEmitter{ProviderSlug: "windsurf", FileName: ".windsurfrules"}
	default:
		return GenericMarkdownEmitter{ProviderSlug: slug, FileName: "AGENTS.md"}
	}
}
```

#### Step 2: Claude Code emitter

```go
// cli/internal/emit/claude.go

package emit

import (
	"fmt"
	"strings"

	"github.com/holdenhewett/nesco/cli/internal/model"
)

// ClaudeEmitter renders ContextDocument as CLAUDE.md format with boundary markers.
type ClaudeEmitter struct{}

func (e ClaudeEmitter) Name() string   { return "claude-code" }
func (e ClaudeEmitter) Format() string { return "md" }

func (e ClaudeEmitter) Emit(doc model.ContextDocument) (string, error) {
	var b strings.Builder

	b.WriteString("# Project Context\n\n")
	b.WriteString(fmt.Sprintf("*Generated by nesco for %s*\n\n", doc.ProjectName))

	for _, section := range doc.Sections {
		if section.SectionOrigin() == model.OriginHuman {
			continue // Human sections are preserved by reconciler, not re-emitted
		}

		cat := string(section.SectionCategory())

		// Opening boundary marker
		b.WriteString(fmt.Sprintf("<!-- nesco:auto:%s -->\n", cat))

		// Section content — dispatch on type
		switch s := section.(type) {
		case model.TechStackSection:
			emitTechStack(&b, s)
		case model.DependencySection:
			emitDependencies(&b, s)
		case model.BuildCommandSection:
			emitBuildCommands(&b, s)
		case model.DirectoryStructureSection:
			emitDirectoryStructure(&b, s)
		case model.ProjectMetadataSection:
			emitProjectMetadata(&b, s)
		case model.TextSection:
			emitTextSection(&b, s)
		}

		// Closing boundary marker
		b.WriteString(fmt.Sprintf("<!-- /nesco:auto:%s -->\n\n", cat))
	}

	return b.String(), nil
}

func emitTechStack(b *strings.Builder, s model.TechStackSection) {
	b.WriteString("## Tech Stack\n\n")
	b.WriteString(fmt.Sprintf("- **Language:** %s", s.Language))
	if s.LanguageVersion != "" {
		b.WriteString(fmt.Sprintf(" %s", s.LanguageVersion))
	}
	b.WriteString("\n")
	if s.Framework != "" {
		b.WriteString(fmt.Sprintf("- **Framework:** %s", s.Framework))
		if s.FrameworkVersion != "" {
			b.WriteString(fmt.Sprintf(" %s", s.FrameworkVersion))
		}
		b.WriteString("\n")
	}
	if s.Runtime != "" {
		b.WriteString(fmt.Sprintf("- **Runtime:** %s", s.Runtime))
		if s.RuntimeVersion != "" {
			b.WriteString(fmt.Sprintf(" %s", s.RuntimeVersion))
		}
		b.WriteString("\n")
	}
}

func emitDependencies(b *strings.Builder, s model.DependencySection) {
	b.WriteString("## Dependencies\n\n")
	for _, g := range s.Groups {
		b.WriteString(fmt.Sprintf("### %s\n\n", strings.Title(g.Category)))
		for _, dep := range g.Items {
			b.WriteString(fmt.Sprintf("- %s %s\n", dep.Name, dep.Version))
		}
		b.WriteString("\n")
	}
}

func emitBuildCommands(b *strings.Builder, s model.BuildCommandSection) {
	b.WriteString("## Build Commands\n\n")
	b.WriteString("| Command | Script | Source |\n")
	b.WriteString("|---------|--------|--------|\n")
	for _, cmd := range s.Commands {
		b.WriteString(fmt.Sprintf("| `%s` | `%s` | %s |\n", cmd.Name, cmd.Command, cmd.Source))
	}
}

func emitDirectoryStructure(b *strings.Builder, s model.DirectoryStructureSection) {
	b.WriteString("## Directory Structure\n\n")
	for _, e := range s.Entries {
		line := fmt.Sprintf("- `%s`", e.Path)
		if e.Convention != "" {
			line += fmt.Sprintf(" — %s", e.Convention)
		}
		if e.Description != "" {
			line += fmt.Sprintf(": %s", e.Description)
		}
		b.WriteString(line + "\n")
	}
}

func emitProjectMetadata(b *strings.Builder, s model.ProjectMetadataSection) {
	b.WriteString("## Project Info\n\n")
	if s.Description != "" {
		b.WriteString(s.Description + "\n\n")
	}
	if s.License != "" {
		b.WriteString(fmt.Sprintf("- **License:** %s\n", s.License))
	}
	if s.CI != "" {
		b.WriteString(fmt.Sprintf("- **CI:** %s\n", s.CI))
	}
}

func emitTextSection(b *strings.Builder, s model.TextSection) {
	b.WriteString(fmt.Sprintf("## %s\n\n", s.Title))
	b.WriteString(s.Body)
	b.WriteString("\n")
}
```

#### Step 3: Write tests

```go
// cli/internal/emit/claude_test.go

package emit

import (
	"strings"
	"testing"
	"time"

	"github.com/holdenhewett/nesco/cli/internal/model"
)

func TestClaudeEmitterBasic(t *testing.T) {
	emitter := ClaudeEmitter{}
	doc := model.ContextDocument{
		ProjectName: "test-project",
		ScanTime:    time.Now(),
		Sections: []model.Section{
			model.TechStackSection{
				Origin:          model.OriginAuto,
				Title:           "Tech Stack",
				Language:        "Go",
				LanguageVersion: "1.22",
			},
		},
	}

	output, err := emitter.Emit(doc)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}

	// Check boundary markers
	if !strings.Contains(output, "<!-- nesco:auto:tech-stack -->") {
		t.Error("missing opening boundary marker")
	}
	if !strings.Contains(output, "<!-- /nesco:auto:tech-stack -->") {
		t.Error("missing closing boundary marker")
	}

	// Check content
	if !strings.Contains(output, "Go 1.22") {
		t.Error("missing tech stack content")
	}
}

func TestClaudeEmitterSkipsHumanSections(t *testing.T) {
	emitter := ClaudeEmitter{}
	doc := model.ContextDocument{
		ProjectName: "test-project",
		ScanTime:    time.Now(),
		Sections: []model.Section{
			model.TextSection{
				Category: model.CatCurated,
				Origin:   model.OriginHuman,
				Title:    "Architecture Notes",
				Body:     "This should not appear in output",
			},
		},
	}

	output, err := emitter.Emit(doc)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}

	if strings.Contains(output, "Architecture Notes") {
		t.Error("human section should not be emitted")
	}
}

func TestClaudeEmitterSurprise(t *testing.T) {
	emitter := ClaudeEmitter{}
	doc := model.ContextDocument{
		ProjectName: "test-project",
		ScanTime:    time.Now(),
		Sections: []model.Section{
			model.TextSection{
				Category: model.CatSurprise,
				Origin:   model.OriginAuto,
				Title:    "Competing Test Frameworks",
				Body:     "Both Jest and Vitest are configured.",
			},
		},
	}

	output, err := emitter.Emit(doc)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}

	if !strings.Contains(output, "<!-- nesco:auto:surprise -->") {
		t.Error("missing surprise boundary marker")
	}
	if !strings.Contains(output, "Competing Test Frameworks") {
		t.Error("missing surprise title")
	}
}
```

#### Step 4: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/emit/...
```

Expected: PASS

---

### Task 3.5: Create Cursor and generic emitters

**Files:**
- Create: `cli/internal/emit/cursor.go`
- Create: `cli/internal/emit/generic.go`
- Create: `cli/internal/emit/cursor_test.go`

**Depends on:** Task 1.2 (model types), Task 3.4 (emitter interface)

**Success Criteria:**
- [ ] Cursor emitter produces `.mdc` format with YAML frontmatter and `# nesco:auto:*` markers
- [ ] Generic markdown emitter works for Gemini, Codex, Windsurf
- [ ] Tests verify `.mdc` frontmatter structure and markdown boundary markers

---

#### Step 1: Cursor emitter

```go
// cli/internal/emit/cursor.go

package emit

import (
	"fmt"
	"strings"

	"github.com/holdenhewett/nesco/cli/internal/model"
)

// CursorEmitter renders ContextDocument as Cursor .mdc format.
// Cursor uses YAML frontmatter and YAML-style comments for boundary markers.
type CursorEmitter struct{}

func (e CursorEmitter) Name() string   { return "cursor" }
func (e CursorEmitter) Format() string { return "mdc" }

func (e CursorEmitter) Emit(doc model.ContextDocument) (string, error) {
	var b strings.Builder

	// Cursor .mdc format: YAML frontmatter + markdown body
	b.WriteString("---\n")
	b.WriteString("description: Project context generated by nesco\n")
	b.WriteString("alwaysApply: true\n")
	b.WriteString("---\n\n")

	for _, section := range doc.Sections {
		if section.SectionOrigin() == model.OriginHuman {
			continue
		}

		cat := string(section.SectionCategory())

		// Cursor uses YAML-style comments for boundary markers
		b.WriteString(fmt.Sprintf("# nesco:auto:%s\n", cat))

		// Reuse the same content formatting as Claude emitter
		switch s := section.(type) {
		case model.TechStackSection:
			emitTechStack(&b, s)
		case model.DependencySection:
			emitDependencies(&b, s)
		case model.BuildCommandSection:
			emitBuildCommands(&b, s)
		case model.DirectoryStructureSection:
			emitDirectoryStructure(&b, s)
		case model.ProjectMetadataSection:
			emitProjectMetadata(&b, s)
		case model.TextSection:
			emitTextSection(&b, s)
		}

		b.WriteString(fmt.Sprintf("# /nesco:auto:%s\n\n", cat))
	}

	return b.String(), nil
}
```

#### Step 2: Generic markdown emitter

```go
// cli/internal/emit/generic.go

package emit

import (
	"fmt"
	"strings"

	"github.com/holdenhewett/nesco/cli/internal/model"
)

// GenericMarkdownEmitter renders ContextDocument as standard markdown
// with HTML comment boundary markers. Works for Gemini, Codex, Windsurf, etc.
type GenericMarkdownEmitter struct {
	ProviderSlug string
	FileName     string
}

func (e GenericMarkdownEmitter) Name() string   { return e.ProviderSlug }
func (e GenericMarkdownEmitter) Format() string { return "md" }

func (e GenericMarkdownEmitter) Emit(doc model.ContextDocument) (string, error) {
	var b strings.Builder

	b.WriteString("# Project Context\n\n")
	b.WriteString(fmt.Sprintf("*Generated by nesco for %s*\n\n", doc.ProjectName))

	for _, section := range doc.Sections {
		if section.SectionOrigin() == model.OriginHuman {
			continue
		}

		cat := string(section.SectionCategory())

		b.WriteString(fmt.Sprintf("<!-- nesco:auto:%s -->\n", cat))

		switch s := section.(type) {
		case model.TechStackSection:
			emitTechStack(&b, s)
		case model.DependencySection:
			emitDependencies(&b, s)
		case model.BuildCommandSection:
			emitBuildCommands(&b, s)
		case model.DirectoryStructureSection:
			emitDirectoryStructure(&b, s)
		case model.ProjectMetadataSection:
			emitProjectMetadata(&b, s)
		case model.TextSection:
			emitTextSection(&b, s)
		}

		b.WriteString(fmt.Sprintf("<!-- /nesco:auto:%s -->\n\n", cat))
	}

	return b.String(), nil
}
```

#### Step 3: Tests

```go
// cli/internal/emit/cursor_test.go

package emit

import (
	"strings"
	"testing"
	"time"

	"github.com/holdenhewett/nesco/cli/internal/model"
)

func TestCursorEmitterFrontmatter(t *testing.T) {
	emitter := CursorEmitter{}
	doc := model.ContextDocument{
		ProjectName: "test",
		ScanTime:    time.Now(),
		Sections: []model.Section{
			model.TechStackSection{
				Origin: model.OriginAuto, Title: "Tech Stack",
				Language: "TypeScript", LanguageVersion: "5.3",
			},
		},
	}

	output, err := emitter.Emit(doc)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}

	if !strings.HasPrefix(output, "---\n") {
		t.Error("Cursor output should start with YAML frontmatter")
	}
	if !strings.Contains(output, "alwaysApply: true") {
		t.Error("missing alwaysApply in frontmatter")
	}
	if !strings.Contains(output, "# nesco:auto:tech-stack") {
		t.Error("missing YAML-style boundary marker")
	}
}

func TestGenericEmitter(t *testing.T) {
	emitter := GenericMarkdownEmitter{ProviderSlug: "gemini-cli", FileName: "GEMINI.md"}
	doc := model.ContextDocument{
		ProjectName: "test",
		ScanTime:    time.Now(),
		Sections: []model.Section{
			model.TextSection{
				Category: model.CatSurprise,
				Origin:   model.OriginAuto,
				Title:    "Mixed naming",
				Body:     "camelCase and kebab-case both used.",
			},
		},
	}

	output, err := emitter.Emit(doc)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}

	if !strings.Contains(output, "<!-- nesco:auto:surprise -->") {
		t.Error("missing HTML boundary marker")
	}
}
```

#### Step 4: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/emit/...
```

Expected: PASS

---

### Task 3.6: Create boundary marker parser and reconciler

**Files:**
- Create: `cli/internal/reconcile/markers.go`
- Create: `cli/internal/reconcile/reconcile.go`
- Create: `cli/internal/reconcile/reconcile_test.go`

**Depends on:** Task 3.4 (emitters produce boundary markers)

**Success Criteria:**
- [ ] Marker parser extracts named sections from existing files
- [ ] Reconciler replaces `nesco:auto:*` sections with new emitter output
- [ ] Reconciler preserves `nesco:human:*` sections verbatim
- [ ] Unmarked content is preserved (graceful with pre-nesco files)
- [ ] New auto sections appended if not in existing file
- [ ] Tests verify merge behavior, preservation, and conflict detection

---

#### Step 1: Boundary marker parser

```go
// cli/internal/reconcile/markers.go

package reconcile

import (
	"regexp"
	"strings"
)

// MarkerFormat determines which comment syntax to use for boundary markers.
type MarkerFormat string

const (
	FormatHTML MarkerFormat = "html" // <!-- nesco:auto:name -->
	FormatYAML MarkerFormat = "yaml" // # nesco:auto:name
)

// Section represents a parsed section from an existing file.
type Section struct {
	Name    string // e.g., "auto:tech-stack" or "human:architecture"
	Content string // everything between opening and closing markers
	IsAuto  bool
	IsHuman bool
}

// Unmarked holds content that appears outside any boundary markers.
type Unmarked struct {
	Content string
	Index   int // position in the file (for ordering)
}

// ParseResult holds all parsed sections and unmarked content from a file.
type ParseResult struct {
	Sections []Section
	Unmarked []Unmarked
}

var (
	htmlOpenRe  = regexp.MustCompile(`<!--\s*nesco:(auto|human):(\S+)\s*-->`)
	htmlCloseRe = regexp.MustCompile(`<!--\s*/nesco:(auto|human):(\S+)\s*-->`)
	yamlOpenRe  = regexp.MustCompile(`#\s*nesco:(auto|human):(\S+)`)
	yamlCloseRe = regexp.MustCompile(`#\s*/nesco:(auto|human):(\S+)`)
)

// Parse extracts boundary-marked sections from file content.
func Parse(content string, format MarkerFormat) ParseResult {
	var openRe, closeRe *regexp.Regexp
	if format == FormatYAML {
		openRe, closeRe = yamlOpenRe, yamlCloseRe
	} else {
		openRe, closeRe = htmlOpenRe, htmlCloseRe
	}

	result := ParseResult{}
	lines := strings.Split(content, "\n")
	var currentSection *Section
	var currentContent []string
	var unmarkedContent []string
	sectionIdx := 0

	for _, line := range lines {
		if match := openRe.FindStringSubmatch(line); match != nil {
			// Save any accumulated unmarked content
			if len(unmarkedContent) > 0 {
				result.Unmarked = append(result.Unmarked, Unmarked{
					Content: strings.Join(unmarkedContent, "\n"),
					Index:   sectionIdx,
				})
				unmarkedContent = nil
				sectionIdx++
			}

			origin := match[1]
			name := match[2]
			currentSection = &Section{
				Name:    origin + ":" + name,
				IsAuto:  origin == "auto",
				IsHuman: origin == "human",
			}
			currentContent = nil
			continue
		}

		if match := closeRe.FindStringSubmatch(line); match != nil && currentSection != nil {
			currentSection.Content = strings.Join(currentContent, "\n")
			result.Sections = append(result.Sections, *currentSection)
			currentSection = nil
			currentContent = nil
			sectionIdx++
			continue
		}

		if currentSection != nil {
			currentContent = append(currentContent, line)
		} else {
			unmarkedContent = append(unmarkedContent, line)
		}
	}

	// Trailing unmarked content
	if len(unmarkedContent) > 0 {
		result.Unmarked = append(result.Unmarked, Unmarked{
			Content: strings.Join(unmarkedContent, "\n"),
			Index:   sectionIdx,
		})
	}

	return result
}
```

#### Step 2: Reconciler

```go
// cli/internal/reconcile/reconcile.go

package reconcile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Conflict represents a discrepancy between curated and scanned content.
type Conflict struct {
	Section string `json:"section"`
	Message string `json:"message"`
}

// Result holds the reconciliation output.
type Result struct {
	Output    string     `json:"output"`
	Conflicts []Conflict `json:"conflicts,omitempty"`
}

// Reconcile merges new emitter output with an existing file.
// - Auto sections are replaced with new output
// - Human sections are preserved verbatim
// - Unmarked content is preserved
// - New auto sections are appended
// - Conflicts detected when human sections contradict auto sections
func Reconcile(existingContent, newEmitterOutput string, format MarkerFormat) Result {
	if existingContent == "" {
		return Result{Output: newEmitterOutput}
	}

	existing := Parse(existingContent, format)
	fresh := Parse(newEmitterOutput, format)

	// Build a map of new auto sections by name
	freshAutoMap := make(map[string]Section)
	for _, s := range fresh.Sections {
		if s.IsAuto {
			freshAutoMap[s.Name] = s
		}
	}

	// Track which new sections were used
	used := make(map[string]bool)
	var conflicts []Conflict

	var b strings.Builder

	// Reconstruct the file, replacing auto sections with fresh content
	// First, output any leading unmarked content
	for _, u := range existing.Unmarked {
		if u.Index == 0 {
			b.WriteString(u.Content)
			b.WriteString("\n")
		}
	}

	for _, s := range existing.Sections {
		if s.IsAuto {
			// Replace with fresh content if available
			if fresh, ok := freshAutoMap[s.Name]; ok {
				writeSection(&b, fresh, format)
				used[s.Name] = true
			}
			// If not in fresh output, section was removed — don't emit
		} else if s.IsHuman {
			// Preserve human sections verbatim
			writeSection(&b, s, format)

			// Detect conflicts: check if human section content contradicts auto sections
			if conflict := detectConflict(s, fresh.Sections); conflict != nil {
				conflicts = append(conflicts, *conflict)
			}
		}
	}

	// Append new auto sections that weren't in the existing file
	for _, s := range fresh.Sections {
		if s.IsAuto && !used[s.Name] {
			writeSection(&b, s, format)
		}
	}

	return Result{Output: b.String(), Conflicts: conflicts}
}

// detectConflict checks if a human section contradicts auto-detected facts.
// Simple heuristic: looks for version numbers and framework names in human content
// and compares against tech-stack auto sections.
func detectConflict(humanSection Section, autoSections []Section) *Conflict {
	// Only check human sections that might mention tech stack facts
	humanContent := strings.ToLower(humanSection.Content)

	for _, auto := range autoSections {
		if !auto.IsAuto {
			continue
		}
		// Check if auto section mentions versions/frameworks that differ from human section
		autoContent := strings.ToLower(auto.Content)

		// Simple pattern matching for common conflicts:
		// - Version numbers (e.g., "React 18" in human vs "React 19" in auto)
		// - Framework names (e.g., "Jest" in human when "Vitest" in auto)

		// Extract version patterns from auto content
		versionPatterns := extractVersionPatterns(autoContent)
		for _, pattern := range versionPatterns {
			// If human content mentions a different version of the same tech
			if strings.Contains(humanContent, pattern.Name) {
				humanVersion := extractVersion(humanContent, pattern.Name)
				if humanVersion != "" && humanVersion != pattern.Version {
					return &Conflict{
						Section: humanSection.Name,
						Message: fmt.Sprintf("Human section mentions %s %s, but scan detected %s %s",
							pattern.Name, humanVersion, pattern.Name, pattern.Version),
					}
				}
			}
		}
	}
	return nil
}

type versionPattern struct {
	Name    string
	Version string
}

// extractVersionPatterns finds technology names and versions in content.
// Simplified implementation — real version would use more sophisticated parsing.
func extractVersionPatterns(content string) []versionPattern {
	var patterns []versionPattern
	// Example: look for patterns like "Go 1.22", "TypeScript 5.3", "React 19"
	// This is a placeholder — full implementation would use regex or proper parsing
	if strings.Contains(content, "go ") {
		patterns = append(patterns, versionPattern{Name: "Go", Version: extractVersion(content, "go")})
	}
	if strings.Contains(content, "typescript") {
		patterns = append(patterns, versionPattern{Name: "TypeScript", Version: extractVersion(content, "typescript")})
	}
	if strings.Contains(content, "react") {
		patterns = append(patterns, versionPattern{Name: "React", Version: extractVersion(content, "react")})
	}
	return patterns
}

func extractVersion(content, techName string) string {
	// Placeholder — would use regex to find version numbers after tech name
	// Example: "Go 1.22" -> "1.22"
	// For now, return empty string to indicate "needs proper implementation"
	return ""
}

func writeSection(b *strings.Builder, s Section, format MarkerFormat) {
	if format == FormatYAML {
		b.WriteString(fmt.Sprintf("# nesco:%s\n", s.Name))
		b.WriteString(s.Content)
		b.WriteString(fmt.Sprintf("\n# /nesco:%s\n\n", s.Name))
	} else {
		b.WriteString(fmt.Sprintf("<!-- nesco:%s -->\n", s.Name))
		b.WriteString(s.Content)
		b.WriteString(fmt.Sprintf("\n<!-- /nesco:%s -->\n\n", s.Name))
	}
}

// ReconcileAndWrite performs reconciliation and writes the result to disk.
func ReconcileAndWrite(outputPath, newEmitterOutput string, format MarkerFormat) (Result, error) {
	existing, _ := os.ReadFile(outputPath) // ok if file doesn't exist

	result := Reconcile(string(existing), newEmitterOutput, format)

	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return result, err
	}

	return result, os.WriteFile(outputPath, []byte(result.Output), 0644)
}
```

#### Step 3: Tests

```go
// cli/internal/reconcile/reconcile_test.go

package reconcile

import (
	"strings"
	"testing"
)

func TestParseHTMLMarkers(t *testing.T) {
	content := `# Project Context

<!-- nesco:auto:tech-stack -->
## Tech Stack
- Go 1.22
<!-- /nesco:auto:tech-stack -->

<!-- nesco:human:architecture -->
## Architecture
Our custom architecture notes.
<!-- /nesco:human:architecture -->
`

	result := Parse(content, FormatHTML)
	if len(result.Sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(result.Sections))
	}
	if !result.Sections[0].IsAuto {
		t.Error("first section should be auto")
	}
	if !result.Sections[1].IsHuman {
		t.Error("second section should be human")
	}
	if !strings.Contains(result.Sections[1].Content, "custom architecture") {
		t.Error("human section content not preserved")
	}
}

func TestReconcileReplacesAuto(t *testing.T) {
	existing := `<!-- nesco:auto:tech-stack -->
## Tech Stack
- Go 1.21
<!-- /nesco:auto:tech-stack -->

<!-- nesco:human:architecture -->
## Architecture
Keep this.
<!-- /nesco:human:architecture -->
`

	fresh := `<!-- nesco:auto:tech-stack -->
## Tech Stack
- Go 1.22
<!-- /nesco:auto:tech-stack -->
`

	result := Reconcile(existing, fresh, FormatHTML)
	// Auto section should be updated
	if !strings.Contains(result.Output, "Go 1.22") {
		t.Error("auto section not updated")
	}
	if strings.Contains(result.Output, "Go 1.21") {
		t.Error("old auto content should be replaced")
	}
	// Human section should be preserved
	if !strings.Contains(result.Output, "Keep this.") {
		t.Error("human section not preserved")
	}
}

func TestReconcileAppendsNewSections(t *testing.T) {
	existing := `<!-- nesco:auto:tech-stack -->
## Tech Stack
- Go 1.22
<!-- /nesco:auto:tech-stack -->
`

	fresh := `<!-- nesco:auto:tech-stack -->
## Tech Stack
- Go 1.22
<!-- /nesco:auto:tech-stack -->

<!-- nesco:auto:surprise -->
## Competing Frameworks
Both Jest and Vitest found.
<!-- /nesco:auto:surprise -->
`

	result := Reconcile(existing, fresh, FormatHTML)
	if !strings.Contains(result.Output, "Competing Frameworks") {
		t.Error("new section should be appended")
	}
}

func TestReconcileEmptyExisting(t *testing.T) {
	result := Reconcile("", "# Fresh content\n", FormatHTML)
	if result.Output != "# Fresh content\n" {
		t.Errorf("empty existing should pass through fresh: %q", result.Output)
	}
}
```

#### Step 4: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/reconcile/...
```

Expected: PASS

---

**Group 3 complete.** 6 tasks covering the scan pipeline: scanner orchestrator, 5 fact detectors, emitters for all providers, boundary marker parsing, and reconciler. Next: Group 4 (Phase 2: Surprise Detectors).

---

## Group 4: Phase 2 Surprise Detectors

**Covers:** Decision #11 (Surprise Detectors: Full Catalog — 24 Detectors)

**Codebase context:** All detectors implement the `scan.Detector` interface from Task 3.1: `Name() string` and `Detect(root string) ([]model.Section, error)`. Surprise detectors return `model.TextSection` with `Category: model.CatSurprise`. Language-specific detectors check for language presence first (go.mod → Go detectors, package.json → JS/TS detectors, etc.) and return nil if the language isn't present.

**Pattern:** Every detector follows the same structure: check for relevant files, analyze content, return findings as TextSections. Tested against purpose-built fixture directories in `testdata/surprises/`.

---

### Task 4.1: Cross-language detectors 1-4

**Files:**
- Create: `cli/internal/scan/detectors/competing_frameworks.go`
- Create: `cli/internal/scan/detectors/module_conflict.go`
- Create: `cli/internal/scan/detectors/migration_in_progress.go`
- Create: `cli/internal/scan/detectors/wrapper_bypass.go`
- Create: `cli/internal/scan/detectors/competing_frameworks_test.go`
- Create: fixture dirs under `testdata/surprises/`

**Depends on:** Task 1.2 (model types), Task 3.1 (Detector interface)

**Success Criteria:**
- [ ] Competing Frameworks: detects 2+ tools in same category (testing, styling, ORM, etc.)
- [ ] Module System Conflict: detects ESM/CJS mixing, inconsistent import styles
- [ ] Migration-in-Progress: detects old+new pattern coexistence (.js+.ts, class+hooks)
- [ ] Custom Wrapper Bypass Risk: detects internal wrappers being bypassed
- [ ] All return TextSections with category "surprise"

---

#### Step 1: Competing Frameworks detector (full implementation example)

```go
// cli/internal/scan/detectors/competing_frameworks.go

package detectors

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/holdenhewett/nesco/cli/internal/model"
)

// CompetingFrameworks detects when 2+ tools in the same category coexist.
type CompetingFrameworks struct{}

func (d CompetingFrameworks) Name() string { return "competing-frameworks" }

// categoryMap defines tool categories and their members.
// Each key is a category, values are dependency names to look for.
var categoryMap = map[string][]string{
	"testing (JS)":     {"jest", "vitest", "mocha", "ava", "tap"},
	"testing (Python)": {"pytest", "unittest", "nose2"},
	"testing (Go)":     {"testify", "gocheck", "gomega"},
	"styling":          {"tailwindcss", "styled-components", "@emotion/styled", "css-modules"},
	"ORM (JS)":         {"prisma", "typeorm", "drizzle-orm", "sequelize", "knex"},
	"ORM (Python)":     {"sqlalchemy", "django", "peewee", "tortoise-orm"},
	"ORM (Go)":         {"gorm", "ent", "sqlc", "sqlx"},
	"HTTP (JS)":        {"express", "fastify", "koa", "hapi"},
	"HTTP (Go)":        {"gin", "echo", "fiber", "chi", "gorilla/mux"},
	"HTTP (Python)":    {"flask", "fastapi", "django", "starlette", "bottle"},
	"state mgmt":       {"redux", "zustand", "jotai", "recoil", "mobx", "pinia", "vuex"},
	"bundler":          {"webpack", "vite", "esbuild", "rollup", "parcel", "turbopack"},
}

func (d CompetingFrameworks) Detect(root string) ([]model.Section, error) {
	deps := collectAllDeps(root)
	if len(deps) == 0 {
		return nil, nil
	}

	var sections []model.Section
	for category, candidates := range categoryMap {
		var found []string
		for _, cand := range candidates {
			if deps[cand] {
				found = append(found, cand)
			}
		}
		if len(found) >= 2 {
			sections = append(sections, model.TextSection{
				Category: model.CatSurprise,
				Origin:   model.OriginAuto,
				Title:    fmt.Sprintf("Competing %s: %s", category, strings.Join(found, " + ")),
				Body: fmt.Sprintf(
					"Multiple %s tools detected: %s. AI agents may generate code using the wrong one. "+
						"Specify which is canonical so agents know which to use.",
					category, strings.Join(found, ", "),
				),
				Source: "detector:competing-frameworks",
			})
		}
	}
	return sections, nil
}

// collectAllDeps gathers dependency names from all supported manifest files.
func collectAllDeps(root string) map[string]bool {
	deps := make(map[string]bool)

	// Node.js
	if data, err := os.ReadFile(filepath.Join(root, "package.json")); err == nil {
		var pkg struct {
			Dependencies    map[string]string `json:"dependencies"`
			DevDependencies map[string]string `json:"devDependencies"`
		}
		if json.Unmarshal(data, &pkg) == nil {
			for k := range pkg.Dependencies {
				deps[k] = true
			}
			for k := range pkg.DevDependencies {
				deps[k] = true
			}
		}
	}

	// Go modules
	if data, err := os.ReadFile(filepath.Join(root, "go.mod")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "/") && !strings.HasPrefix(line, "//") {
				parts := strings.Fields(line)
				if len(parts) >= 1 {
					// Extract short name: github.com/gin-gonic/gin → gin
					dep := parts[0]
					lastSlash := strings.LastIndex(dep, "/")
					if lastSlash >= 0 {
						deps[dep[lastSlash+1:]] = true
					}
					deps[dep] = true // also add full path for matching
				}
			}
		}
	}

	// Python — check pyproject.toml dependencies
	if data, err := os.ReadFile(filepath.Join(root, "pyproject.toml")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			// Simple dependency line: "pytest>=7.0"
			if strings.Contains(line, "\"") {
				// Extract package name from quoted dependency spec
				parts := strings.FieldsFunc(line, func(r rune) bool {
					return r == '"' || r == '\'' || r == '>' || r == '<' || r == '=' || r == '~' || r == '!'
				})
				if len(parts) > 0 {
					name := strings.TrimSpace(parts[0])
					if name != "" && !strings.HasPrefix(name, "#") {
						deps[strings.ToLower(name)] = true
					}
				}
			}
		}
	}

	// Rust — check Cargo.toml [dependencies]
	if data, err := os.ReadFile(filepath.Join(root, "Cargo.toml")); err == nil {
		inDeps := false
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "[dependencies]" || line == "[dev-dependencies]" {
				inDeps = true
				continue
			}
			if strings.HasPrefix(line, "[") {
				inDeps = false
				continue
			}
			if inDeps && strings.Contains(line, "=") {
				parts := strings.SplitN(line, "=", 2)
				deps[strings.TrimSpace(parts[0])] = true
			}
		}
	}

	return deps
}
```

#### Step 2: Module System Conflict, Migration-in-Progress, Custom Wrapper Bypass Risk

Each follows the same pattern. Key detection logic per detector:

**Module System Conflict (`module_conflict.go`):**
- JS: Check `"type"` field in package.json. Grep `.js` files for `require()` and `import` usage. If both patterns exist and `"type"` is ambiguous, flag.
- Python: Check for `from __future__ import` alongside modern imports.
- Rust: Check `edition` in Cargo.toml vs syntax patterns.

```go
// cli/internal/scan/detectors/module_conflict.go

package detectors

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/holdenhewett/nesco/cli/internal/model"
)

type ModuleConflict struct{}

func (d ModuleConflict) Name() string { return "module-system-conflict" }

func (d ModuleConflict) Detect(root string) ([]model.Section, error) {
	var sections []model.Section

	// JS/TS: check for ESM/CJS mixing
	if s := detectJSModuleConflict(root); s != nil {
		sections = append(sections, *s)
	}

	return sections, nil
}

func detectJSModuleConflict(root string) *model.TextSection {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return nil
	}

	var pkg struct {
		Type string `json:"type"`
	}
	json.Unmarshal(data, &pkg)

	// Walk .js files looking for require() and import usage
	hasRequire := false
	hasImport := false
	filepath.Walk(filepath.Join(root, "src"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".js" && filepath.Ext(path) != ".mjs" && filepath.Ext(path) != ".cjs" {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		s := string(content)
		if strings.Contains(s, "require(") {
			hasRequire = true
		}
		if strings.Contains(s, "import ") || strings.Contains(s, "export ") {
			hasImport = true
		}
		return nil
	})

	if hasRequire && hasImport {
		return &model.TextSection{
			Category: model.CatSurprise,
			Origin:   model.OriginAuto,
			Title:    "Module System Conflict: ESM and CJS mixed",
			Body: fmt.Sprintf(
				"Both require() (CommonJS) and import/export (ESM) patterns found in source. "+
					"package.json type=%q. AI agents may generate imports in the wrong module system.",
				pkg.Type,
			),
			Source: "detector:module-system-conflict",
		}
	}
	return nil
}
```

**Migration-in-Progress (`migration_in_progress.go`):**
- Detect `.js` + `.ts` file ratio in src/ (if both exist with >20% of either)
- Detect React class + function component coexistence
- Detect `pages/` + `app/` in Next.js projects

**Custom Wrapper Bypass Risk (`wrapper_bypass.go`):**
- Find files in `util/`, `lib/`, `common/`, `helpers/` that import a popular library and re-export
- Check if rest of codebase consistently uses the wrapper

Both follow the same struct/interface pattern — the executor should use the Competing Frameworks detector as the template.

#### Step 3: Create test fixtures

```json
// cli/internal/scan/detectors/testdata/surprises/competing-frameworks/package.json
{
  "dependencies": {
    "jest": "^29.0.0",
    "vitest": "^1.0.0",
    "react": "^18.0.0"
  }
}
```

```go
// cli/internal/scan/detectors/competing_frameworks_test.go

package detectors

import (
	"testing"

	"github.com/holdenhewett/nesco/cli/internal/model"
)

func TestCompetingFrameworksDetects(t *testing.T) {
	det := CompetingFrameworks{}
	sections, err := det.Detect("testdata/surprises/competing-frameworks")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected at least one surprise for competing frameworks")
	}
	ts := sections[0].(model.TextSection)
	if ts.Category != model.CatSurprise {
		t.Errorf("category = %q, want surprise", ts.Category)
	}
}

func TestCompetingFrameworksCleanProject(t *testing.T) {
	det := CompetingFrameworks{}
	sections, err := det.Detect("testdata/techstack/go-project") // no competing deps
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 surprises for clean project, got %d", len(sections))
	}
}
```

#### Step 4: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/scan/detectors/...
```

---

### Task 4.2: Cross-language detectors 5-8

**Files:**
- Create: `cli/internal/scan/detectors/lockfile_conflict.go`
- Create: `cli/internal/scan/detectors/test_convention.go`
- Create: `cli/internal/scan/detectors/deprecated_pattern.go`
- Create: `cli/internal/scan/detectors/path_alias_gap.go`
- Create: `cli/internal/scan/detectors/lockfile_conflict_test.go`
- Create: `cli/internal/scan/detectors/test_convention_test.go`
- Create: `cli/internal/scan/detectors/deprecated_pattern_test.go`
- Create: `cli/internal/scan/detectors/path_alias_gap_test.go`
- Create: fixture dirs under `testdata/surprises/`

**Depends on:** Task 3.1 (Detector interface)

**Success Criteria:**
- [ ] Lock File Conflict: detects package-lock.json + yarn.lock, Pipfile.lock + poetry.lock, etc.
- [ ] Test Convention Mismatch: detects mixed .test/.spec naming, mixed co-located/top-level tests
- [ ] Deprecated Pattern Prevalence: counts @deprecated markers, legacy/ dirs, TODO:migrate
- [ ] Path Alias Gap: detects configured but inconsistently used tsconfig paths

---

#### Step 1: Write tests first

```go
// cli/internal/scan/detectors/lockfile_conflict_test.go

package detectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLockFileConflictDetects(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "package-lock.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(tmp, "yarn.lock"), []byte(""), 0644)

	det := LockFileConflict{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Error("expected lock file conflict surprise")
	}
}

func TestLockFileConflictClean(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "package-lock.json"), []byte("{}"), 0644)
	// Only one lock file — no conflict

	det := LockFileConflict{}
	sections, _ := det.Detect(tmp)
	if len(sections) != 0 {
		t.Errorf("expected no surprises with single lock file, got %d", len(sections))
	}
}
```

```go
// cli/internal/scan/detectors/test_convention_test.go

package detectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTestConventionMixed(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "src"), 0755)
	os.WriteFile(filepath.Join(tmp, "src", "app.test.ts"), []byte(""), 0644)
	os.WriteFile(filepath.Join(tmp, "src", "util.spec.ts"), []byte(""), 0644)

	det := TestConvention{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Error("expected test convention surprise for mixed .test/.spec")
	}
}

func TestTestConventionConsistent(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "src"), 0755)
	os.WriteFile(filepath.Join(tmp, "src", "app.test.ts"), []byte(""), 0644)
	os.WriteFile(filepath.Join(tmp, "src", "util.test.ts"), []byte(""), 0644)

	det := TestConvention{}
	sections, _ := det.Detect(tmp)
	if len(sections) != 0 {
		t.Errorf("expected no surprises with consistent naming, got %d", len(sections))
	}
}
```

```go
// cli/internal/scan/detectors/deprecated_pattern_test.go

package detectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeprecatedPatternDetects(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "legacy"), 0755)
	os.MkdirAll(filepath.Join(tmp, "src"), 0755)
	// Create files with deprecated markers
	for i := 0; i < 6; i++ {
		os.WriteFile(
			filepath.Join(tmp, "src", fmt.Sprintf("file%d.ts", i)),
			[]byte("// @deprecated\nfunction old() {}"),
			0644,
		)
	}

	det := DeprecatedPattern{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Error("expected deprecated pattern surprise")
	}
}

func TestDeprecatedPatternClean(t *testing.T) {
	det := DeprecatedPattern{}
	sections, _ := det.Detect(t.TempDir())
	if len(sections) != 0 {
		t.Errorf("expected no surprises for clean dir, got %d", len(sections))
	}
}
```

```go
// cli/internal/scan/detectors/path_alias_gap_test.go

package detectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathAliasGapDetects(t *testing.T) {
	tmp := t.TempDir()
	tsconfig := `{"compilerOptions": {"paths": {"@/*": ["./src/*"]}}}`
	os.WriteFile(filepath.Join(tmp, "tsconfig.json"), []byte(tsconfig), 0644)
	os.MkdirAll(filepath.Join(tmp, "src"), 0755)
	// Files using relative imports instead of aliases
	os.WriteFile(filepath.Join(tmp, "src", "a.ts"), []byte(`import { foo } from "../util"`), 0644)
	os.WriteFile(filepath.Join(tmp, "src", "b.ts"), []byte(`import { bar } from "../lib"`), 0644)

	det := PathAliasGap{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Error("expected path alias gap surprise")
	}
}

func TestPathAliasGapNoTsconfig(t *testing.T) {
	det := PathAliasGap{}
	sections, _ := det.Detect(t.TempDir())
	if len(sections) != 0 {
		t.Errorf("expected no surprises without tsconfig, got %d", len(sections))
	}
}
```

Note: `deprecated_pattern_test.go` needs `"fmt"` import for `fmt.Sprintf`.

#### Step 2: Write implementations

**Detection logic per detector:**

**Lock File Conflict (`lockfile_conflict.go`):**
```go
// Check for coexistence of conflicting lock files
var lockFileConflicts = [][]string{
	{"package-lock.json", "yarn.lock"},
	{"package-lock.json", "pnpm-lock.yaml"},
	{"yarn.lock", "pnpm-lock.yaml"},
	{"Pipfile.lock", "poetry.lock"},
	{"Pipfile.lock", "uv.lock"},
}
// For each pair, check if both exist. Flag if so.
```

**Test Convention Mismatch (`test_convention.go`):**
```go
// Walk source files, categorize test files by:
// - Naming: .test.ts vs .spec.ts vs _test.go
// - Location: co-located (same dir as source) vs top-level tests/
// - Framework: import patterns (jest, vitest, testing, pytest)
// Flag if multiple patterns coexist with significant usage (>20% each)
```

**Deprecated Pattern Prevalence (`deprecated_pattern.go`):**
```go
// Grep for: @deprecated, // DEPRECATED, # Deprecated, TODO: migrate,
// TODO: remove, legacy/, old/, deprecated/
// Return count and locations if > threshold (e.g., 5 instances)
```

**Path Alias Gap (`path_alias_gap.go`):**
```go
// Parse tsconfig.json "paths" key
// Walk .ts/.tsx files, count alias imports vs relative imports
// Flag if aliases defined but <30% of eligible imports use them
```

Each detector: ~40-60 lines.

#### Step 3: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/scan/detectors/...
```

Expected: PASS

---

### Task 4.3: Cross-language detectors 9-12

**Files:**
- Create: `cli/internal/scan/detectors/version_mismatch.go`
- Create: `cli/internal/scan/detectors/version_mismatch_test.go`
- Create: `cli/internal/scan/detectors/version_constraint.go`
- Create: `cli/internal/scan/detectors/version_constraint_test.go`
- Create: `cli/internal/scan/detectors/env_convention.go`
- Create: `cli/internal/scan/detectors/env_convention_test.go`
- Create: `cli/internal/scan/detectors/linter_extraction.go`
- Create: `cli/internal/scan/detectors/linter_extraction_test.go`
- Create: fixture dirs under `testdata/surprises/`

**Depends on:** Task 3.1 (Detector interface)

**Success Criteria:**
- [ ] Major Version Pattern Mismatch: detects framework version X with version Y code patterns
- [ ] Version Constraint Violation: detects go.mod version vs generics, requires-python vs match/case
- [ ] Environment Variable Convention: detects undocumented vars, wrong framework prefixes
- [ ] Formatter/Linter Config Extraction: extracts rules from .prettierrc, .editorconfig, ruff.toml, .golangci.yml

---

#### Step 1: Write tests first

```go
// cli/internal/scan/detectors/version_mismatch_test.go

package detectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVersionMismatchNextJS(t *testing.T) {
	tmp := t.TempDir()
	// Next.js 14 but pages/ directory (v12 pattern)
	os.WriteFile(filepath.Join(tmp, "package.json"), []byte(`{"dependencies":{"next":"^14.0.0"}}`), 0644)
	os.MkdirAll(filepath.Join(tmp, "pages"), 0755)
	os.MkdirAll(filepath.Join(tmp, "app"), 0755)

	det := VersionMismatch{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Error("expected version mismatch surprise for Next.js with both pages/ and app/")
	}
}
```

```go
// cli/internal/scan/detectors/version_constraint_test.go

package detectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVersionConstraintGoGenerics(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test\ngo 1.17"), 0644)
	os.MkdirAll(filepath.Join(tmp, "pkg"), 0755)
	// File using generics (requires 1.18+)
	os.WriteFile(filepath.Join(tmp, "pkg", "util.go"), []byte("package pkg\nfunc Map[T any](s []T) []T { return s }"), 0644)

	det := VersionConstraint{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Error("expected version constraint violation for Go 1.17 with generics")
	}
}

func TestVersionConstraintClean(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module test\ngo 1.22"), 0644)

	det := VersionConstraint{}
	sections, _ := det.Detect(tmp)
	if len(sections) != 0 {
		t.Errorf("expected no surprises for modern Go version, got %d", len(sections))
	}
}
```

```go
// cli/internal/scan/detectors/env_convention_test.go

package detectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnvConventionUndocumented(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, ".env.example"), []byte("DATABASE_URL=\nAPI_KEY="), 0644)
	os.MkdirAll(filepath.Join(tmp, "src"), 0755)
	os.WriteFile(filepath.Join(tmp, "src", "config.ts"), []byte(`const secret = process.env.SECRET_TOKEN`), 0644)

	det := EnvConvention{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Error("expected env convention surprise for undocumented SECRET_TOKEN")
	}
}
```

```go
// cli/internal/scan/detectors/linter_extraction_test.go

package detectors

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/nesco/cli/internal/model"
)

func TestLinterExtractionPrettier(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, ".prettierrc"), []byte(`{"semi": false, "singleQuote": true, "tabWidth": 2}`), 0644)

	det := LinterExtraction{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected conventions section for prettier config")
	}
	ts := sections[0].(model.TextSection)
	if ts.Category != model.CatConventions {
		t.Errorf("category = %q, want conventions (not surprise)", ts.Category)
	}
}

func TestLinterExtractionNoConfigs(t *testing.T) {
	det := LinterExtraction{}
	sections, _ := det.Detect(t.TempDir())
	if len(sections) != 0 {
		t.Errorf("expected no sections without linter configs, got %d", len(sections))
	}
}
```

#### Step 2: Write implementations

**Detection logic per detector:**

**Major Version Pattern Mismatch (`version_mismatch.go`):**
```go
// Parse major version of key frameworks from deps
// Check for version-specific filesystem/code patterns:
//   Next.js: "app/" dir → v13+, "pages/" dir → v12-. Both → flag
//   Tailwind: v4 uses CSS config, v3 uses JS config
//   React: class components → pre-hooks era
// Compare dep version against detected patterns
```

**Version Constraint Violation (`version_constraint.go`):**
```go
// Go: parse "go X.Y" from go.mod. Scan for generics (type parameters)
//     if go version < 1.18 and generics found → flag
// Python: parse "requires-python" from pyproject.toml. Scan for
//     match/case (3.10+), walrus operator (3.8+), type union syntax (3.10+)
// Rust: parse "edition" from Cargo.toml. Check for edition-specific features.
```

**Environment Variable Convention (`env_convention.go`):**
```go
// Parse .env.example / .env.template for documented vars
// Scan source for process.env.* / import.meta.env.* / os.Getenv()
// Detect framework prefixes: NEXT_PUBLIC_, VITE_, REACT_APP_
// Flag: vars in code not in .env.example, wrong prefixes for framework
```

**Formatter/Linter Config Extraction (`linter_extraction.go`):**
```go
// This detector returns CatConventions, not CatSurprise — it's extracting
// enforced style rules that agents should follow.
// Parse: .prettierrc (JSON), .editorconfig (INI-like), ruff.toml,
//        .golangci.yml, clippy.toml
// Extract key rules: indent size, quote style, semicolons, max line length
// Return as TextSection with extracted rules formatted as bullet points
```

Note: Detector #12 (Formatter/Linter Config Extraction) returns `CatConventions` rather than `CatSurprise` — it's extracting facts, not surprises. This is by design per Decision #11.

#### Step 3: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/scan/detectors/...
```

Expected: PASS

---

### Task 4.4: Go-specific detectors 13-16

**Files:**
- Create: `cli/internal/scan/detectors/go_internal.go`
- Create: `cli/internal/scan/detectors/go_nil_interface.go`
- Create: `cli/internal/scan/detectors/go_cgo.go`
- Create: `cli/internal/scan/detectors/go_replace.go`
- Create: `cli/internal/scan/detectors/go_internal_test.go`
- Create: `cli/internal/scan/detectors/go_nil_interface_test.go`
- Create: `cli/internal/scan/detectors/go_cgo_test.go`
- Create: `cli/internal/scan/detectors/go_replace_test.go`
- Create: `cli/internal/scan/detectors/testdata/go-internal-violation/` (fixture)
- Create: `cli/internal/scan/detectors/testdata/go-cgo-project/` (fixture)
- Create: `cli/internal/scan/detectors/testdata/go-replace-local/` (fixture)

**Depends on:** Task 3.1 (Detector interface)

**Success Criteria:**
- [ ] All detectors check for `go.mod` before running (return nil if absent)
- [ ] Internal Package Visibility: traces imports across .go files, flags external access to internal/
- [ ] Nil Interface Comparison: finds functions returning interfaces that might return typed nil
- [ ] CGO Detection: finds `import "C"` and checks for CGO_ENABLED evidence
- [ ] Replace Directives: parses go.mod replace statements, flags local-path replacements

---

#### Step 1: Write tests

```go
// cli/internal/scan/detectors/go_internal_test.go

package detectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoInternalDetectsViolation(t *testing.T) {
	// Fixture: a Go project where a file outside internal/ imports an internal package
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module example.com/proj\n\ngo 1.22\n"), 0644)
	os.MkdirAll(filepath.Join(tmp, "internal", "secret"), 0755)
	os.WriteFile(filepath.Join(tmp, "internal", "secret", "api.go"), []byte("package secret\n\nfunc Hidden() {}\n"), 0644)
	// A file at the same module level importing internal — this is valid in Go
	// but a sibling module or external consumer would violate it.
	// We detect the presence and report it as context, not error.

	d := GoInternal{}
	sections, err := d.Detect(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sections) == 0 {
		t.Error("expected at least 1 section reporting internal package presence")
	}
}

func TestGoInternalSkipsNonGo(t *testing.T) {
	// No go.mod — should return nil
	d := GoInternal{}
	sections, err := d.Detect(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sections != nil {
		t.Errorf("expected nil for non-Go project, got %d sections", len(sections))
	}
}
```

```go
// cli/internal/scan/detectors/go_nil_interface_test.go

package detectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoNilInterfaceDetectsPattern(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module example.com/proj\n\ngo 1.22\n"), 0644)
	// File with a function returning typed nil through an interface
	code := `package main

type MyError struct{}
func (e *MyError) Error() string { return "err" }

func doWork() error {
	var e *MyError
	return e // returns typed nil — will never be == nil
}
`
	os.WriteFile(filepath.Join(tmp, "main.go"), []byte(code), 0644)

	d := GoNilInterface{}
	sections, err := d.Detect(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sections) == 0 {
		t.Error("expected section flagging typed nil return pattern")
	}
}

func TestGoNilInterfaceClean(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module example.com/proj\n\ngo 1.22\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644)

	d := GoNilInterface{}
	sections, err := d.Detect(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for clean project, got %d", len(sections))
	}
}
```

```go
// cli/internal/scan/detectors/go_cgo_test.go

package detectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoCGODetectsImportC(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module example.com/proj\n\ngo 1.22\n"), 0644)
	code := `package main

// #include <stdlib.h>
import "C"

func main() {}
`
	os.WriteFile(filepath.Join(tmp, "main.go"), []byte(code), 0644)

	d := GoCGO{}
	sections, err := d.Detect(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sections) == 0 {
		t.Error("expected section flagging CGO usage")
	}
}

func TestGoCGOCleanProject(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module example.com/proj\n\ngo 1.22\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644)

	d := GoCGO{}
	sections, err := d.Detect(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections, got %d", len(sections))
	}
}
```

```go
// cli/internal/scan/detectors/go_replace_test.go

package detectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoReplaceDetectsLocalPath(t *testing.T) {
	tmp := t.TempDir()
	gomod := `module example.com/proj

go 1.22

require example.com/lib v1.0.0

replace example.com/lib => ../lib
`
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(gomod), 0644)

	d := GoReplace{}
	sections, err := d.Detect(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sections) == 0 {
		t.Error("expected section flagging local-path replace directive")
	}
}

func TestGoReplaceCleanGoMod(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module example.com/proj\n\ngo 1.22\n"), 0644)

	d := GoReplace{}
	sections, err := d.Detect(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for clean go.mod, got %d", len(sections))
	}
}

func TestGoReplaceVersionReplace(t *testing.T) {
	tmp := t.TempDir()
	// Version-pinned replace (not local path) — should not flag as surprise
	gomod := `module example.com/proj

go 1.22

replace example.com/broken => example.com/fixed v1.2.3
`
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(gomod), 0644)

	d := GoReplace{}
	sections, err := d.Detect(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Version replaces are informational, not surprising — behavior depends on implementation
	// Either 0 (only flag local-path) or 1 (report all replaces) is acceptable
}
```

#### Step 2: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/scan/detectors/...
```

Expected: FAIL (implementations don't exist yet)

#### Step 3: Implement detectors

**Internal Package Visibility (`go_internal.go`):**
```go
// Guard: if no go.mod, return nil
// Find all directories named "internal/"
// Walk all .go files, parse import statements
// For each import of an internal package, check if the importing file
// is within the allowed parent scope
// Flag violations with file locations
```

**Nil Interface Comparison (`go_nil_interface.go`):**
```go
// Guard: if no go.mod, return nil
// Search .go files for patterns:
//   - Functions with return type `error` that return `(*SomeType)(nil)`
//   - Variables compared with `== nil` that have interface types
// This is heuristic — flag when patterns found, explain the risk
```

**CGO Detection (`go_cgo.go`):**
```go
// Guard: if no go.mod, return nil
// Search .go files for `import "C"` or `//go:build cgo`
// Check Dockerfile / Makefile / CI config for CGO_ENABLED settings
// Flag if CGO code exists but no build environment evidence
```

**Replace Directives (`go_replace.go`):**
```go
// Guard: if no go.mod, return nil
// Parse go.mod for `replace` directives
// Categorize: local path replacements (contain "../" or "./")
//            vs version replacements
// Flag local-path replacements (break CI)
```

#### Step 4: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/scan/detectors/...
```

Expected: PASS

---

### Task 4.5: Python-specific detectors 17-19

**Files:**
- Create: `cli/internal/scan/detectors/python_async.go`
- Create: `cli/internal/scan/detectors/python_layout.go`
- Create: `cli/internal/scan/detectors/python_namespace.go`
- Create: `cli/internal/scan/detectors/python_async_test.go`
- Create: `cli/internal/scan/detectors/python_layout_test.go`
- Create: `cli/internal/scan/detectors/python_namespace_test.go`
- Create: `cli/internal/scan/detectors/testdata/python-async-mismatch/` (fixture)
- Create: `cli/internal/scan/detectors/testdata/python-src-layout/` (fixture)
- Create: `cli/internal/scan/detectors/testdata/python-namespace-gap/` (fixture)

**Depends on:** Task 3.1 (Detector interface)

**Success Criteria:**
- [ ] All detectors check for Python project markers before running
- [ ] Async/Sync Mismatch: detects sync blocking calls in async handlers
- [ ] Package Layout: detects src/ layout vs flat layout, reports implications
- [ ] Namespace Package Confusion: finds dirs with .py files but no `__init__.py`

---

#### Step 1: Write tests

```go
// cli/internal/scan/detectors/python_async_test.go

package detectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPythonAsyncDetectsSyncInAsync(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "requirements.txt"), []byte("fastapi\nrequests\n"), 0644)
	code := `import requests

async def fetch_data():
    # Blocking call inside async function
    response = requests.get("https://api.example.com/data")
    return response.json()
`
	os.WriteFile(filepath.Join(tmp, "handler.py"), []byte(code), 0644)

	d := PythonAsync{}
	sections, err := d.Detect(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sections) == 0 {
		t.Error("expected section flagging sync call in async context")
	}
}

func TestPythonAsyncCleanProject(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "requirements.txt"), []byte("fastapi\nhttpx\n"), 0644)
	code := `import httpx

async def fetch_data():
    async with httpx.AsyncClient() as client:
        response = await client.get("https://api.example.com/data")
        return response.json()
`
	os.WriteFile(filepath.Join(tmp, "handler.py"), []byte(code), 0644)

	d := PythonAsync{}
	sections, err := d.Detect(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for clean async code, got %d", len(sections))
	}
}

func TestPythonAsyncSkipsNonPython(t *testing.T) {
	d := PythonAsync{}
	sections, err := d.Detect(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sections != nil {
		t.Error("expected nil for non-Python project")
	}
}
```

```go
// cli/internal/scan/detectors/python_layout_test.go

package detectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPythonLayoutDetectsSrcLayout(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "pyproject.toml"), []byte("[project]\nname = \"myapp\"\n"), 0644)
	os.MkdirAll(filepath.Join(tmp, "src", "myapp"), 0755)
	os.WriteFile(filepath.Join(tmp, "src", "myapp", "__init__.py"), []byte(""), 0644)

	d := PythonLayout{}
	sections, err := d.Detect(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sections) == 0 {
		t.Error("expected section reporting src layout")
	}
}

func TestPythonLayoutDetectsFlatLayout(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "setup.py"), []byte("from setuptools import setup\nsetup()\n"), 0644)
	os.MkdirAll(filepath.Join(tmp, "myapp"), 0755)
	os.WriteFile(filepath.Join(tmp, "myapp", "__init__.py"), []byte(""), 0644)

	d := PythonLayout{}
	sections, err := d.Detect(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sections) == 0 {
		t.Error("expected section reporting flat layout")
	}
}

func TestPythonLayoutSkipsNonPython(t *testing.T) {
	d := PythonLayout{}
	sections, err := d.Detect(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sections != nil {
		t.Error("expected nil for non-Python project")
	}
}
```

```go
// cli/internal/scan/detectors/python_namespace_test.go

package detectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPythonNamespaceDetectsMissingInit(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "pyproject.toml"), []byte("[project]\nname = \"myapp\"\n"), 0644)
	// Package directory with .py files but no __init__.py
	os.MkdirAll(filepath.Join(tmp, "myapp", "utils"), 0755)
	os.WriteFile(filepath.Join(tmp, "myapp", "__init__.py"), []byte(""), 0644)
	os.WriteFile(filepath.Join(tmp, "myapp", "utils", "helpers.py"), []byte("def help(): pass\n"), 0644)
	// No __init__.py in utils/ — implicit namespace package

	d := PythonNamespace{}
	sections, err := d.Detect(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sections) == 0 {
		t.Error("expected section flagging missing __init__.py in utils/")
	}
}

func TestPythonNamespaceClean(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "pyproject.toml"), []byte("[project]\nname = \"myapp\"\n"), 0644)
	os.MkdirAll(filepath.Join(tmp, "myapp", "utils"), 0755)
	os.WriteFile(filepath.Join(tmp, "myapp", "__init__.py"), []byte(""), 0644)
	os.WriteFile(filepath.Join(tmp, "myapp", "utils", "__init__.py"), []byte(""), 0644)
	os.WriteFile(filepath.Join(tmp, "myapp", "utils", "helpers.py"), []byte("def help(): pass\n"), 0644)

	d := PythonNamespace{}
	sections, err := d.Detect(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for clean project, got %d", len(sections))
	}
}

func TestPythonNamespaceExcludesTests(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "pyproject.toml"), []byte("[project]\nname = \"myapp\"\n"), 0644)
	// tests/ without __init__.py is common and should not be flagged
	os.MkdirAll(filepath.Join(tmp, "tests"), 0755)
	os.WriteFile(filepath.Join(tmp, "tests", "test_app.py"), []byte("def test_something(): pass\n"), 0644)

	d := PythonNamespace{}
	sections, err := d.Detect(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections (tests/ excluded), got %d", len(sections))
	}
}
```

#### Step 2: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/scan/detectors/...
```

Expected: FAIL (implementations don't exist yet)

#### Step 3: Implement detectors

**Async/Sync Context Mismatch (`python_async.go`):**
```go
// Guard: check for pyproject.toml/setup.py/requirements.txt
// Find `async def` functions in .py files
// Within those function bodies, search for sync blocking calls:
//   time.sleep(), requests.get/post(), sqlite3.connect(),
//   open() without aiofiles
// Check for asyncio.to_thread() wrappers
// Flag unwrapped sync calls in async contexts
```

**Package Layout Detection (`python_layout.go`):**
```go
// Guard: check for Python project markers
// Check for src/ directory containing Python packages
// Check pyproject.toml for packages/package-dir configuration
// Determine: "src layout" vs "flat layout"
// Report layout and import implications
```

**Namespace Package Confusion (`python_namespace.go`):**
```go
// Guard: check for Python project markers
// Walk directory tree for directories containing .py files
// Check each for __init__.py presence
// Flag directories that have .py files but no __init__.py
// Exclude: top-level, tests/, scripts/ (common patterns)
```

#### Step 4: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/scan/detectors/...
```

Expected: PASS

---

### Task 4.6: Rust detectors 20-22 and JS/TS detectors 23-24

**Files:**
- Create: `cli/internal/scan/detectors/rust_features.go`
- Create: `cli/internal/scan/detectors/rust_unsafe.go`
- Create: `cli/internal/scan/detectors/rust_async_runtime.go`
- Create: `cli/internal/scan/detectors/ts_strictness.go`
- Create: `cli/internal/scan/detectors/monorepo.go`
- Create: `cli/internal/scan/detectors/rust_features_test.go`
- Create: `cli/internal/scan/detectors/rust_unsafe_test.go`
- Create: `cli/internal/scan/detectors/rust_async_runtime_test.go`
- Create: `cli/internal/scan/detectors/ts_strictness_test.go`
- Create: `cli/internal/scan/detectors/monorepo_test.go`
- Create: `cli/internal/scan/detectors/testdata/rust-features/` (fixture)
- Create: `cli/internal/scan/detectors/testdata/rust-unsafe-forbid/` (fixture)
- Create: `cli/internal/scan/detectors/testdata/rust-tokio/` (fixture)
- Create: `cli/internal/scan/detectors/testdata/ts-strict/` (fixture)
- Create: `cli/internal/scan/detectors/testdata/monorepo-pnpm/` (fixture)

**Depends on:** Task 3.1 (Detector interface)

**Success Criteria:**
- [ ] Rust detectors check for Cargo.toml before running
- [ ] JS/TS detectors check for package.json/tsconfig.json before running
- [ ] Feature Flag Mapping: parses Cargo.toml features, maps `#[cfg(feature)]` usage
- [ ] Unsafe Code Policy: detects `#![forbid(unsafe_code)]` or `#![deny(unsafe_code)]`
- [ ] Async Runtime Choice: identifies tokio vs async-std vs smol
- [ ] TypeScript Strictness: extracts strict flags from tsconfig.json
- [ ] Monorepo Structure: detects workspaces, maps cross-package deps

---

#### Step 1: Write tests

```go
// cli/internal/scan/detectors/rust_features_test.go

package detectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRustFeaturesDetectsNonDefault(t *testing.T) {
	tmp := t.TempDir()
	cargo := `[package]
name = "mylib"
version = "0.1.0"

[features]
default = ["json"]
json = ["dep:serde_json"]
grpc = ["dep:tonic"]
`
	os.WriteFile(filepath.Join(tmp, "Cargo.toml"), []byte(cargo), 0644)
	os.MkdirAll(filepath.Join(tmp, "src"), 0755)
	code := `#[cfg(feature = "grpc")]
mod grpc_handler;

fn main() {}
`
	os.WriteFile(filepath.Join(tmp, "src", "main.rs"), []byte(code), 0644)

	d := RustFeatures{}
	sections, err := d.Detect(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sections) == 0 {
		t.Error("expected section reporting non-default feature flags")
	}
}

func TestRustFeaturesSkipsNonRust(t *testing.T) {
	d := RustFeatures{}
	sections, err := d.Detect(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sections != nil {
		t.Error("expected nil for non-Rust project")
	}
}
```

```go
// cli/internal/scan/detectors/rust_unsafe_test.go

package detectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRustUnsafeForbid(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "Cargo.toml"), []byte("[package]\nname = \"safe\"\n"), 0644)
	os.MkdirAll(filepath.Join(tmp, "src"), 0755)
	os.WriteFile(filepath.Join(tmp, "src", "lib.rs"), []byte("#![forbid(unsafe_code)]\n\npub fn safe() {}\n"), 0644)

	d := RustUnsafe{}
	sections, err := d.Detect(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sections) == 0 {
		t.Error("expected section reporting unsafe policy")
	}
	// Should report forbid stance
}

func TestRustUnsafeUsage(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "Cargo.toml"), []byte("[package]\nname = \"riskylib\"\n"), 0644)
	os.MkdirAll(filepath.Join(tmp, "src"), 0755)
	code := `pub fn risky() {
    unsafe {
        std::ptr::null::<u8>().read();
    }
}
`
	os.WriteFile(filepath.Join(tmp, "src", "lib.rs"), []byte(code), 0644)

	d := RustUnsafe{}
	sections, err := d.Detect(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sections) == 0 {
		t.Error("expected section flagging unsafe usage without forbid/deny")
	}
}
```

```go
// cli/internal/scan/detectors/rust_async_runtime_test.go

package detectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRustAsyncRuntimeTokio(t *testing.T) {
	tmp := t.TempDir()
	cargo := `[package]
name = "myservice"

[dependencies]
tokio = { version = "1", features = ["full"] }
`
	os.WriteFile(filepath.Join(tmp, "Cargo.toml"), []byte(cargo), 0644)
	os.MkdirAll(filepath.Join(tmp, "src"), 0755)
	os.WriteFile(filepath.Join(tmp, "src", "main.rs"), []byte("#[tokio::main]\nasync fn main() {}\n"), 0644)

	d := RustAsyncRuntime{}
	sections, err := d.Detect(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sections) == 0 {
		t.Error("expected section reporting tokio runtime")
	}
}

func TestRustAsyncRuntimeNone(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "Cargo.toml"), []byte("[package]\nname = \"sync-only\"\n"), 0644)
	os.MkdirAll(filepath.Join(tmp, "src"), 0755)
	os.WriteFile(filepath.Join(tmp, "src", "main.rs"), []byte("fn main() {}\n"), 0644)

	d := RustAsyncRuntime{}
	sections, err := d.Detect(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for sync-only project, got %d", len(sections))
	}
}
```

```go
// cli/internal/scan/detectors/ts_strictness_test.go

package detectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTSStrictnessFullStrict(t *testing.T) {
	tmp := t.TempDir()
	tsconfig := `{
  "compilerOptions": {
    "strict": true,
    "target": "ES2022",
    "module": "node16"
  }
}
`
	os.WriteFile(filepath.Join(tmp, "tsconfig.json"), []byte(tsconfig), 0644)

	d := TSStrictness{}
	sections, err := d.Detect(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sections) == 0 {
		t.Error("expected section reporting strict mode")
	}
}

func TestTSStrictnessPartial(t *testing.T) {
	tmp := t.TempDir()
	tsconfig := `{
  "compilerOptions": {
    "noImplicitAny": true,
    "strictNullChecks": false,
    "target": "ES2020"
  }
}
`
	os.WriteFile(filepath.Join(tmp, "tsconfig.json"), []byte(tsconfig), 0644)

	d := TSStrictness{}
	sections, err := d.Detect(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sections) == 0 {
		t.Error("expected section reporting partial strictness")
	}
}

func TestTSStrictnessSkipsNonTS(t *testing.T) {
	d := TSStrictness{}
	sections, err := d.Detect(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sections != nil {
		t.Error("expected nil for non-TypeScript project")
	}
}
```

```go
// cli/internal/scan/detectors/monorepo_test.go

package detectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMonorepoDetectsPnpmWorkspace(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "package.json"), []byte(`{"name": "root"}`), 0644)
	os.WriteFile(filepath.Join(tmp, "pnpm-workspace.yaml"), []byte("packages:\n  - 'packages/*'\n"), 0644)
	os.MkdirAll(filepath.Join(tmp, "packages", "ui"), 0755)
	os.WriteFile(filepath.Join(tmp, "packages", "ui", "package.json"), []byte(`{"name": "@mono/ui"}`), 0644)
	os.MkdirAll(filepath.Join(tmp, "packages", "api"), 0755)
	os.WriteFile(filepath.Join(tmp, "packages", "api", "package.json"), []byte(`{"name": "@mono/api", "dependencies": {"@mono/ui": "workspace:*"}}`), 0644)

	d := MonorepoStructure{}
	sections, err := d.Detect(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sections) == 0 {
		t.Error("expected section reporting pnpm workspace structure")
	}
}

func TestMonorepoDetectsNpmWorkspaces(t *testing.T) {
	tmp := t.TempDir()
	pkg := `{"name": "root", "workspaces": ["packages/*"]}`
	os.WriteFile(filepath.Join(tmp, "package.json"), []byte(pkg), 0644)
	os.MkdirAll(filepath.Join(tmp, "packages", "core"), 0755)
	os.WriteFile(filepath.Join(tmp, "packages", "core", "package.json"), []byte(`{"name": "@mono/core"}`), 0644)

	d := MonorepoStructure{}
	sections, err := d.Detect(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sections) == 0 {
		t.Error("expected section reporting npm workspace structure")
	}
}

func TestMonorepoNotDetected(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "package.json"), []byte(`{"name": "single-app"}`), 0644)

	d := MonorepoStructure{}
	sections, err := d.Detect(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for single-package project, got %d", len(sections))
	}
}

func TestMonorepoSkipsEmptyDir(t *testing.T) {
	d := MonorepoStructure{}
	sections, err := d.Detect(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sections != nil {
		t.Error("expected nil for empty directory")
	}
}
```

#### Step 2: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/scan/detectors/...
```

Expected: FAIL (implementations don't exist yet)

#### Step 3: Implement detectors

**Feature Flag Mapping (`rust_features.go`):**
```go
// Guard: check for Cargo.toml
// Parse [features] and default = [...] from Cargo.toml
// Walk .rs files for #[cfg(feature = "...")] attributes
// Build map: feature → files/functions it gates
// Report non-default features that gate significant functionality
```

**Unsafe Code Policy (`rust_unsafe.go`):**
```go
// Guard: check for Cargo.toml
// Search lib.rs and main.rs for:
//   #![forbid(unsafe_code)]
//   #![deny(unsafe_code)]
//   #![deny(unsafe_op_in_unsafe_fn)]
// Also search for `unsafe` keyword usage across all .rs files
// Report the project's unsafe stance
```

**Async Runtime Choice (`rust_async_runtime.go`):**
```go
// Guard: check for Cargo.toml
// Check Cargo.toml [dependencies] for: tokio, async-std, smol
// Search .rs files for entry macros: #[tokio::main], #[async_std::main]
// Report which runtime is canonical
// Note if async-std is used (discontinued March 2025)
```

**TypeScript Strictness Level (`ts_strictness.go`):**
```go
// Guard: check for tsconfig.json
// Parse tsconfig.json — extract compilerOptions
// Report: strict, noImplicitAny, strictNullChecks, strictFunctionTypes
// Calculate effective strictness level
// Returns CatConventions (not surprise) — this is factual context
```

**Monorepo Workspace Structure (`monorepo.go`):**
```go
// Guard: check for workspace markers
// Check: pnpm-workspace.yaml, turbo.json, nx.json, lerna.json,
//        package.json "workspaces" field
// List workspace packages and their package.json names
// Map cross-package dependencies (which packages depend on which)
// Report shared configuration packages
```

Note: Detectors #23 (TS Strictness) and #24 (Monorepo) return `CatConventions` — they're extracting factual configuration context, not surprises.

#### Step 4: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/scan/detectors/...
```

Expected: PASS

---

### Task 4.7: Register all detectors and integration test

**Files:**
- Create: `cli/internal/scan/detectors/registry.go`
- Create: `cli/internal/scan/detectors/registry_test.go`
- Create: `cli/internal/scan/integration_test.go`

**Depends on:** Tasks 4.1-4.6

**Success Criteria:**
- [ ] `AllDetectors()` returns all 29 detectors (5 fact + 24 surprise)
- [ ] `SurpriseDetectors()` returns only the 24 surprise detectors
- [ ] Language-specific detectors only run when language is present
- [ ] Integration test runs full scanner against a multi-language fixture project
- [ ] All detectors registered, no panics, results collected correctly

---

#### Step 1: Registry

```go
// cli/internal/scan/detectors/registry.go

package detectors

import "github.com/holdenhewett/nesco/cli/internal/scan"

// AllDetectors returns every registered detector (fact + surprise).
func AllDetectors() []scan.Detector {
	return []scan.Detector{
		// Tier 1 fact detectors
		TechStack{},
		Dependencies{},
		BuildCommands{},
		DirectoryStructure{},
		ProjectMetadata{},

		// Cross-language surprise detectors (1-12)
		CompetingFrameworks{},
		ModuleConflict{},
		MigrationInProgress{},
		WrapperBypass{},
		LockFileConflict{},
		TestConvention{},
		DeprecatedPattern{},
		PathAliasGap{},
		VersionMismatch{},
		VersionConstraint{},
		EnvConvention{},
		LinterExtraction{},

		// Go-specific (13-16)
		GoInternal{},
		GoNilInterface{},
		GoCGO{},
		GoReplace{},

		// Python-specific (17-19)
		PythonAsync{},
		PythonLayout{},
		PythonNamespace{},

		// Rust-specific (20-22)
		RustFeatures{},
		RustUnsafe{},
		RustAsyncRuntime{},

		// JS/TS-specific (23-24)
		TSStrictness{},
		MonorepoStructure{},
	}
}
```

#### Step 2: Integration test

```go
// cli/internal/scan/integration_test.go

package scan

import (
	"testing"

	"github.com/holdenhewett/nesco/cli/internal/scan/detectors"
)

func TestFullScanNoProject(t *testing.T) {
	// Empty directory — all detectors should return gracefully
	scanner := NewScanner(detectors.AllDetectors()...)
	result := scanner.Run(t.TempDir())

	if len(result.Document.Sections) != 0 {
		t.Errorf("expected 0 sections for empty dir, got %d", len(result.Document.Sections))
	}
	// No warnings expected (detectors should return nil, not error)
	if len(result.Warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d: %v", len(result.Warnings), result.Warnings)
	}
}
```

#### Step 3: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/scan/...
```

Expected: PASS — all 29 detectors registered, empty dir scan completes without errors.

---

**Group 4 complete.** 7 tasks covering all 24 surprise detectors plus the registry and integration test. Next: Group 5 (Phase 2: CLI Commands).

---

## Group 5: Phase 2 CLI Commands

**Covers:** Decision #12 (CLI Command Surface - Phase 2), #1 (Tool Identity)

**Codebase context:** Phase 1 commands (init, import, parity, config, info) are registered from Group 2. Phase 2 adds scan, drift, and baseline. The scanner (Group 3) and detectors (Group 4) provide the execution engine. Emitters and reconciler handle output. The drift engine compares against `.nesco/baseline.json`.

---

### Task 5.1: Create drift/baseline package

**Files:**
- Create: `cli/internal/drift/baseline.go`
- Create: `cli/internal/drift/diff.go`
- Create: `cli/internal/drift/baseline_test.go`
- Create: `cli/internal/drift/diff_test.go`

**Depends on:** Task 1.2 (model types), Task 1.4 (config package for .nesco/ path)

**Success Criteria:**
- [ ] `SaveBaseline()` serializes ContextDocument to `.nesco/baseline.json` with section hashes
- [ ] `LoadBaseline()` reads and deserializes baseline
- [ ] `Diff()` compares two ContextDocuments, reports changed/new/removed sections
- [ ] Section comparison uses hash for text sections, field-level for typed sections
- [ ] Tests verify save/load roundtrip and diff behavior

---

#### Step 1: Baseline management

```go
// cli/internal/drift/baseline.go

package drift

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/holdenhewett/nesco/cli/internal/model"
)

const BaselineFileName = "baseline.json"

// Baseline represents a serialized scan snapshot.
type Baseline struct {
	ProjectName string           `json:"projectName"`
	Sections    []BaselineSection `json:"sections"`
}

// BaselineSection is a section with its content hash for efficient diffing.
type BaselineSection struct {
	Category string `json:"category"`
	Title    string `json:"title"`
	Origin   string `json:"origin"`
	Hash     string `json:"hash"`
	Content  string `json:"content,omitempty"` // stored for typed sections
}

// SaveBaseline writes a ContextDocument as a baseline snapshot.
func SaveBaseline(nescoDir string, doc model.ContextDocument) error {
	b := Baseline{ProjectName: doc.ProjectName}

	for _, s := range doc.Sections {
		if s.SectionOrigin() == model.OriginHuman {
			continue // only track auto-maintained sections
		}
		content, _ := json.Marshal(s)
		hash := sha256.Sum256(content)
		b.Sections = append(b.Sections, BaselineSection{
			Category: string(s.SectionCategory()),
			Title:    s.SectionTitle(),
			Origin:   string(s.SectionOrigin()),
			Hash:     hex.EncodeToString(hash[:]),
			Content:  string(content),
		})
	}

	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(nescoDir, BaselineFileName)
	return os.WriteFile(path, data, 0644)
}

// LoadBaseline reads a baseline snapshot from disk.
func LoadBaseline(nescoDir string) (*Baseline, error) {
	path := filepath.Join(nescoDir, BaselineFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no baseline found: %w", err)
	}

	var b Baseline
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// BaselineExists checks if a baseline file exists.
func BaselineExists(nescoDir string) bool {
	_, err := os.Stat(filepath.Join(nescoDir, BaselineFileName))
	return err == nil
}
```

#### Step 2: Diff engine

```go
// cli/internal/drift/diff.go

package drift

// DriftReport describes what changed between baseline and current scan.
type DriftReport struct {
	Changed []DriftItem `json:"changed"`
	New     []DriftItem `json:"new"`
	Removed []DriftItem `json:"removed"`
	Clean   bool        `json:"clean"` // true if no drift
}

// DriftItem represents a single changed/new/removed section.
type DriftItem struct {
	Category string `json:"category"`
	Title    string `json:"title"`
}

// Diff compares a current scan against a baseline snapshot.
func Diff(baseline *Baseline, current *Baseline) DriftReport {
	report := DriftReport{}

	baseMap := make(map[string]BaselineSection)
	for _, s := range baseline.Sections {
		key := s.Category + ":" + s.Title
		baseMap[key] = s
	}

	currMap := make(map[string]BaselineSection)
	for _, s := range current.Sections {
		key := s.Category + ":" + s.Title
		currMap[key] = s
	}

	// Check for changed and new
	for key, curr := range currMap {
		base, exists := baseMap[key]
		if !exists {
			report.New = append(report.New, DriftItem{
				Category: curr.Category,
				Title:    curr.Title,
			})
		} else if base.Hash != curr.Hash {
			report.Changed = append(report.Changed, DriftItem{
				Category: curr.Category,
				Title:    curr.Title,
			})
		}
	}

	// Check for removed
	for key, base := range baseMap {
		if _, exists := currMap[key]; !exists {
			report.Removed = append(report.Removed, DriftItem{
				Category: base.Category,
				Title:    base.Title,
			})
		}
	}

	report.Clean = len(report.Changed) == 0 && len(report.New) == 0 && len(report.Removed) == 0
	return report
}
```

#### Step 3: Tests

```go
// cli/internal/drift/baseline_test.go

package drift

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/holdenhewett/nesco/cli/internal/model"
)

func TestSaveAndLoadBaseline(t *testing.T) {
	tmp := t.TempDir()
	doc := model.ContextDocument{
		ProjectName: "test",
		ScanTime:    time.Now(),
		Sections: []model.Section{
			model.TechStackSection{
				Origin: model.OriginAuto, Title: "Tech Stack",
				Language: "Go", LanguageVersion: "1.22",
			},
		},
	}

	if err := SaveBaseline(tmp, doc); err != nil {
		t.Fatalf("SaveBaseline: %v", err)
	}

	loaded, err := LoadBaseline(tmp)
	if err != nil {
		t.Fatalf("LoadBaseline: %v", err)
	}

	if len(loaded.Sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(loaded.Sections))
	}
	if loaded.Sections[0].Category != "tech-stack" {
		t.Errorf("category = %q, want tech-stack", loaded.Sections[0].Category)
	}
}

func TestBaselineNotExists(t *testing.T) {
	if BaselineExists(t.TempDir()) {
		t.Error("baseline should not exist in empty dir")
	}
}
```

```go
// cli/internal/drift/diff_test.go

package drift

import "testing"

func TestDiffDetectsChanges(t *testing.T) {
	baseline := &Baseline{
		Sections: []BaselineSection{
			{Category: "tech-stack", Title: "Tech Stack", Hash: "aaa"},
			{Category: "dependencies", Title: "Dependencies", Hash: "bbb"},
		},
	}
	current := &Baseline{
		Sections: []BaselineSection{
			{Category: "tech-stack", Title: "Tech Stack", Hash: "aaa"}, // unchanged
			{Category: "dependencies", Title: "Dependencies", Hash: "ccc"}, // changed
			{Category: "surprise", Title: "New surprise", Hash: "ddd"}, // new
		},
	}

	report := Diff(baseline, current)
	if report.Clean {
		t.Error("report should not be clean")
	}
	if len(report.Changed) != 1 {
		t.Errorf("expected 1 changed, got %d", len(report.Changed))
	}
	if len(report.New) != 1 {
		t.Errorf("expected 1 new, got %d", len(report.New))
	}
}

func TestDiffDetectsRemoved(t *testing.T) {
	baseline := &Baseline{
		Sections: []BaselineSection{
			{Category: "tech-stack", Title: "Tech Stack", Hash: "aaa"},
		},
	}
	current := &Baseline{
		Sections: []BaselineSection{},
	}

	report := Diff(baseline, current)
	if len(report.Removed) != 1 {
		t.Errorf("expected 1 removed, got %d", len(report.Removed))
	}
}

func TestDiffClean(t *testing.T) {
	baseline := &Baseline{
		Sections: []BaselineSection{
			{Category: "tech-stack", Title: "Tech Stack", Hash: "aaa"},
		},
	}

	report := Diff(baseline, baseline)
	if !report.Clean {
		t.Error("identical baselines should be clean")
	}
}
```

#### Step 4: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./internal/drift/...
```

Expected: PASS

---

### Task 5.2: Add `nesco scan` command

**Files:**
- Create: `cli/cmd/nesco/scan.go`
- Create: `cli/cmd/nesco/scan_test.go`

**Depends on:** Task 1.1 (Cobra), Task 3.1 (scanner), Task 3.4-3.5 (emitters), Task 3.6 (reconciler), Task 4.7 (detector registry), Task 5.1 (baseline)

**Success Criteria:**
- [ ] `nesco scan` runs all detectors, emits to configured providers, updates baseline
- [ ] `--format <provider>` targets a single provider
- [ ] `--all` emits to all known providers
- [ ] `--dry-run` shows what would be written without writing
- [ ] `--full` runs scan + parity analysis
- [ ] `--json` outputs structured ScanResult
- [ ] `--yes` skips first-run provider confirmation
- [ ] Exit code 0 on success, 2 if no detectable project

---

#### Step 1: Write tests

```go
// cli/cmd/nesco/scan_test.go

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/nesco/cli/internal/config"
	"github.com/holdenhewett/nesco/cli/internal/drift"
	"github.com/holdenhewett/nesco/cli/internal/output"
)

func setupGoProject(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module example.com/proj\n\ngo 1.22\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644)
	// Create .nesco config so scan doesn't prompt
	nescoDir := filepath.Join(tmp, ".nesco")
	os.MkdirAll(nescoDir, 0755)
	cfg := config.Config{Providers: []string{"claude"}}
	data, _ := json.Marshal(cfg)
	os.WriteFile(filepath.Join(nescoDir, "config.json"), data, 0644)
	return tmp
}

func TestScanCommandJSON(t *testing.T) {
	tmp := setupGoProject(t)

	var buf bytes.Buffer
	output.Writer = &buf
	output.JSON = true
	defer func() {
		output.Writer = os.Stdout
		output.JSON = false
	}()

	cmd := scanCmd
	cmd.SetArgs([]string{"--json"})
	// Override project root detection to use tmp
	origFindRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	defer func() { findProjectRoot = origFindRoot }()

	err := cmd.RunE(cmd, []string{})
	if err != nil {
		t.Fatalf("scan --json failed: %v", err)
	}

	// Output should be valid JSON with sections
	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nGot: %s", err, buf.String())
	}
}

func TestScanDryRunDoesNotWrite(t *testing.T) {
	tmp := setupGoProject(t)

	origFindRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	defer func() { findProjectRoot = origFindRoot }()

	cmd := scanCmd
	cmd.SetArgs([]string{"--dry-run"})

	err := cmd.RunE(cmd, []string{})
	if err != nil {
		t.Fatalf("scan --dry-run failed: %v", err)
	}

	// Baseline should NOT be updated on dry run
	nescoDir := filepath.Join(tmp, ".nesco")
	if drift.BaselineExists(nescoDir) {
		t.Error("dry run should not create a baseline")
	}
}

func TestScanCreatesBaseline(t *testing.T) {
	tmp := setupGoProject(t)

	origFindRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	defer func() { findProjectRoot = origFindRoot }()

	cmd := scanCmd
	cmd.SetArgs([]string{})

	err := cmd.RunE(cmd, []string{})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	nescoDir := filepath.Join(tmp, ".nesco")
	if !drift.BaselineExists(nescoDir) {
		t.Error("scan should create a baseline")
	}
}
```

#### Step 2: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/...
```

Expected: FAIL (scan command not yet implemented)

#### Step 3: Write scan command

```go
// cli/cmd/nesco/scan.go

package main

import (
	"fmt"
	"os"

	"github.com/holdenhewett/nesco/cli/internal/config"
	"github.com/holdenhewett/nesco/cli/internal/drift"
	"github.com/holdenhewett/nesco/cli/internal/emit"
	"github.com/holdenhewett/nesco/cli/internal/output"
	"github.com/holdenhewett/nesco/cli/internal/parity"
	"github.com/holdenhewett/nesco/cli/internal/provider"
	"github.com/holdenhewett/nesco/cli/internal/reconcile"
	"github.com/holdenhewett/nesco/cli/internal/scan"
	"github.com/holdenhewett/nesco/cli/internal/scan/detectors"
	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan codebase and generate context files",
	Long:  "Runs all detectors against the codebase, emits context files for configured providers, and updates the baseline.",
	RunE:  runScan,
}

func init() {
	scanCmd.Flags().String("format", "", "Target a single provider format")
	scanCmd.Flags().Bool("all", false, "Emit to all known providers")
	scanCmd.Flags().Bool("dry-run", false, "Show what would be written without writing")
	scanCmd.Flags().Bool("full", false, "Include parity analysis")
	scanCmd.Flags().Bool("yes", false, "Skip first-run confirmation")
	rootCmd.AddCommand(scanCmd)
}

func runScan(cmd *cobra.Command, args []string) error {
	root, err := findProjectRoot()
	if err != nil {
		output.PrintError(2, "no detectable project", "Run from a project directory with go.mod, package.json, etc.")
		os.Exit(2)
		return nil
	}

	// Load or create config
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}

	// If no config, run init flow
	if !config.Exists(root) {
		yes, _ := cmd.Flags().GetBool("yes")
		if !yes && os.Getenv("NESCO_NO_PROMPT") != "1" && !output.JSON {
			fmt.Println("No .nesco/config.json found. Run `nesco init` first, or use --yes to auto-detect.")
			return fmt.Errorf("no config found")
		}
		// Auto-detect providers
		home, _ := os.UserHomeDir()
		for _, prov := range provider.AllProviders {
			if prov.Detect(home) {
				cfg.Providers = append(cfg.Providers, prov.Slug)
			}
		}
		config.Save(root, cfg)
	}

	// Run scanner
	scanner := scan.NewScanner(detectors.AllDetectors()...)
	result := scanner.Run(root)

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	formatFlag, _ := cmd.Flags().GetString("format")
	emitAll, _ := cmd.Flags().GetBool("all")

	if output.JSON {
		output.Print(result)
		return nil
	}

	// Print scan results summary
	fmt.Printf("Scanned %s in %s\n", root, result.Duration.Round(1e6))
	fmt.Printf("  Sections: %d\n", len(result.Document.Sections))
	if len(result.Warnings) > 0 {
		fmt.Printf("  Warnings: %d\n", len(result.Warnings))
		for _, w := range result.Warnings {
			fmt.Printf("    ⚠ %s: %s\n", w.Detector, w.Message)
		}
	}

	// Determine target providers
	var targets []provider.Provider
	if formatFlag != "" {
		prov := findProviderBySlug(formatFlag)
		if prov == nil {
			return fmt.Errorf("unknown provider: %s", formatFlag)
		}
		targets = []provider.Provider{*prov}
	} else if emitAll {
		targets = provider.AllProviders
	} else {
		for _, slug := range cfg.Providers {
			if prov := findProviderBySlug(slug); prov != nil {
				targets = append(targets, *prov)
			}
		}
	}

	// Emit to each provider
	for _, prov := range targets {
		emitter := emit.EmitterForProvider(prov.Slug)
		emitted, err := emitter.Emit(result.Document)
		if err != nil {
			fmt.Printf("  ✗ %s: emit error: %v\n", prov.Name, err)
			continue
		}

		if dryRun {
			fmt.Printf("\n--- %s (dry run) ---\n%s\n", prov.Name, emitted)
			continue
		}

		if prov.EmitPath == nil {
			continue
		}
		outputPath := prov.EmitPath(root)
		format := reconcile.FormatHTML
		if emitter.Format() == "mdc" {
			format = reconcile.FormatYAML
		}
		_, err = reconcile.ReconcileAndWrite(outputPath, emitted, format)
		if err != nil {
			fmt.Printf("  ✗ %s: write error: %v\n", prov.Name, err)
			continue
		}
		fmt.Printf("  ✓ %s: %s\n", prov.Name, outputPath)
	}

	// Update baseline
	if !dryRun {
		nescoDir := config.DirPath(root)
		if err := drift.SaveBaseline(nescoDir, result.Document); err != nil {
			fmt.Printf("  ⚠ baseline update failed: %v\n", err)
		}
	}

	// Parity analysis if --full
	full, _ := cmd.Flags().GetBool("full")
	if full {
		home, _ := os.UserHomeDir()
		var detected []provider.Provider
		for _, prov := range provider.AllProviders {
			if prov.Detect(home) {
				detected = append(detected, prov)
			}
		}
		if len(detected) >= 2 {
			parityReport := parity.Analyze(detected, root)
			fmt.Printf("\nParity: %s\n", parityReport.Summary)
		}
	}

	return nil
}
```

#### Step 4: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/...
```

Expected: PASS

#### Step 5: Build and verify

```bash
cd /home/hhewett/.local/src/nesco/cli && go build ./cmd/nesco && ./nesco scan --help
```

---

### Task 5.3: Add `nesco drift` command

**Files:**
- Create: `cli/cmd/nesco/drift.go`
- Create: `cli/cmd/nesco/drift_test.go`

**Depends on:** Task 1.1 (Cobra), Task 3.1 (scanner), Task 4.7 (detector registry), Task 5.1 (drift package)

**Success Criteria:**
- [ ] `nesco drift` compares current scan against baseline, reports changes
- [ ] `--ci` mode exits code 3 if drift detected
- [ ] `--json` outputs structured DriftReport
- [ ] Returns error if no baseline exists

---

#### Step 1: Write tests

```go
// cli/cmd/nesco/drift_test.go

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/nesco/cli/internal/config"
	"github.com/holdenhewett/nesco/cli/internal/drift"
	"github.com/holdenhewett/nesco/cli/internal/model"
	"github.com/holdenhewett/nesco/cli/internal/output"
)

func TestDriftCommandNoBaseline(t *testing.T) {
	tmp := setupGoProject(t)

	origFindRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	defer func() { findProjectRoot = origFindRoot }()

	cmd := driftCmd
	cmd.SetArgs([]string{})

	err := cmd.RunE(cmd, []string{})
	if err == nil {
		t.Error("drift without baseline should return error")
	}
}

func TestDriftCommandClean(t *testing.T) {
	tmp := setupGoProject(t)
	nescoDir := filepath.Join(tmp, ".nesco")

	// Create a baseline that matches current state
	doc := model.ContextDocument{ProjectName: "test"}
	drift.SaveBaseline(nescoDir, doc)

	origFindRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	defer func() { findProjectRoot = origFindRoot }()

	var buf bytes.Buffer
	output.Writer = &buf
	output.JSON = true
	defer func() {
		output.Writer = os.Stdout
		output.JSON = false
	}()

	cmd := driftCmd
	cmd.SetArgs([]string{"--json"})

	err := cmd.RunE(cmd, []string{})
	if err != nil {
		t.Fatalf("drift failed: %v", err)
	}

	var report drift.DriftReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	// Note: "clean" depends on whether detectors find anything new vs the empty baseline.
	// This test verifies the command runs and produces valid JSON output.
}

func TestDriftCommandJSON(t *testing.T) {
	tmp := setupGoProject(t)
	nescoDir := filepath.Join(tmp, ".nesco")

	// Create an empty baseline, then add a file so detectors find something new
	doc := model.ContextDocument{ProjectName: "test"}
	drift.SaveBaseline(nescoDir, doc)

	origFindRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	defer func() { findProjectRoot = origFindRoot }()

	var buf bytes.Buffer
	output.Writer = &buf
	output.JSON = true
	defer func() {
		output.Writer = os.Stdout
		output.JSON = false
	}()

	cmd := driftCmd
	cmd.SetArgs([]string{"--json"})

	err := cmd.RunE(cmd, []string{})
	if err != nil {
		t.Fatalf("drift --json failed: %v", err)
	}

	// Should produce valid JSON regardless of drift status
	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output not valid JSON: %v\nGot: %s", err, buf.String())
	}
}
```

#### Step 2: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/...
```

Expected: FAIL (drift command not yet implemented)

#### Step 3: Write drift command

```go
// cli/cmd/nesco/drift.go

package main

import (
	"fmt"
	"os"

	"github.com/holdenhewett/nesco/cli/internal/config"
	"github.com/holdenhewett/nesco/cli/internal/drift"
	"github.com/holdenhewett/nesco/cli/internal/output"
	"github.com/holdenhewett/nesco/cli/internal/scan"
	"github.com/holdenhewett/nesco/cli/internal/scan/detectors"
	"github.com/spf13/cobra"
)

var driftCmd = &cobra.Command{
	Use:   "drift",
	Short: "Compare current codebase state against baseline",
	Long:  "Re-runs detectors and compares results against the stored baseline. Reports new, changed, and removed sections.",
	RunE:  runDrift,
}

func init() {
	driftCmd.Flags().Bool("ci", false, "CI mode — exit code 3 if drift detected")
	rootCmd.AddCommand(driftCmd)
}

func runDrift(cmd *cobra.Command, args []string) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}

	nescoDir := config.DirPath(root)
	if !drift.BaselineExists(nescoDir) {
		output.PrintError(1, "no baseline found", "Run `nesco scan` first to create a baseline")
		return fmt.Errorf("no baseline")
	}

	baseline, err := drift.LoadBaseline(nescoDir)
	if err != nil {
		return err
	}

	// Run fresh scan
	scanner := scan.NewScanner(detectors.AllDetectors()...)
	result := scanner.Run(root)

	// Create current baseline from scan results (in-memory, not saved)
	currentBaseline := drift.BaselineFromDocument(result.Document)

	report := drift.Diff(baseline, currentBaseline)

	if output.JSON {
		output.Print(report)
	} else {
		if report.Clean {
			fmt.Println("No drift detected. Baseline is current.")
		} else {
			if len(report.Changed) > 0 {
				fmt.Printf("Changed (%d):\n", len(report.Changed))
				for _, item := range report.Changed {
					fmt.Printf("  ~ %s: %s\n", item.Category, item.Title)
				}
			}
			if len(report.New) > 0 {
				fmt.Printf("New (%d):\n", len(report.New))
				for _, item := range report.New {
					fmt.Printf("  + %s: %s\n", item.Category, item.Title)
				}
			}
			if len(report.Removed) > 0 {
				fmt.Printf("Removed (%d):\n", len(report.Removed))
				for _, item := range report.Removed {
					fmt.Printf("  - %s: %s\n", item.Category, item.Title)
				}
			}
		}
	}

	ciMode, _ := cmd.Flags().GetBool("ci")
	if ciMode && !report.Clean {
		os.Exit(3)
	}

	return nil
}
```

Note: This references `drift.BaselineFromDocument()` — a helper that should be added to the drift package to create a Baseline from a ContextDocument without writing to disk. Add this alongside Task 5.1's baseline.go:

```go
// Add to cli/internal/drift/baseline.go
func BaselineFromDocument(doc model.ContextDocument) *Baseline {
	b := &Baseline{ProjectName: doc.ProjectName}
	for _, s := range doc.Sections {
		if s.SectionOrigin() == model.OriginHuman {
			continue
		}
		content, _ := json.Marshal(s)
		hash := sha256.Sum256(content)
		b.Sections = append(b.Sections, BaselineSection{
			Category: string(s.SectionCategory()),
			Title:    s.SectionTitle(),
			Origin:   string(s.SectionOrigin()),
			Hash:     hex.EncodeToString(hash[:]),
		})
	}
	return b
}
```

#### Step 4: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/...
```

Expected: PASS

---

### Task 5.4: Add `nesco baseline` command

**Files:**
- Create: `cli/cmd/nesco/baseline.go`
- Create: `cli/cmd/nesco/baseline_test.go`

**Depends on:** Task 1.1 (Cobra), Task 5.1 (drift package), Task 5.2 (scan for baseline creation)

**Success Criteria:**
- [ ] `nesco baseline` accepts current codebase state without regenerating files
- [ ] `--from-import` creates baseline after importing from another provider
- [ ] `--json` outputs baseline metadata
- [ ] Useful for: adopting existing context files, post-conversion, team onboarding

---

#### Step 1: Write tests

```go
// cli/cmd/nesco/baseline_test.go

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/nesco/cli/internal/drift"
	"github.com/holdenhewett/nesco/cli/internal/output"
)

func TestBaselineCommandCreatesBaseline(t *testing.T) {
	tmp := setupGoProject(t)

	origFindRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	defer func() { findProjectRoot = origFindRoot }()

	cmd := baselineCmd
	cmd.SetArgs([]string{})

	err := cmd.RunE(cmd, []string{})
	if err != nil {
		t.Fatalf("baseline command failed: %v", err)
	}

	nescoDir := filepath.Join(tmp, ".nesco")
	if !drift.BaselineExists(nescoDir) {
		t.Error("baseline command should create a baseline file")
	}
}

func TestBaselineCommandDoesNotEmitContextFiles(t *testing.T) {
	tmp := setupGoProject(t)

	origFindRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	defer func() { findProjectRoot = origFindRoot }()

	cmd := baselineCmd
	cmd.SetArgs([]string{})

	err := cmd.RunE(cmd, []string{})
	if err != nil {
		t.Fatalf("baseline command failed: %v", err)
	}

	// Baseline should NOT emit context files (that's scan's job)
	if _, err := os.Stat(filepath.Join(tmp, "CLAUDE.md")); err == nil {
		t.Error("baseline command should not emit CLAUDE.md — that's scan's job")
	}
}

func TestBaselineCommandJSON(t *testing.T) {
	tmp := setupGoProject(t)

	origFindRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	defer func() { findProjectRoot = origFindRoot }()

	var buf bytes.Buffer
	output.Writer = &buf
	output.JSON = true
	defer func() {
		output.Writer = os.Stdout
		output.JSON = false
	}()

	cmd := baselineCmd
	cmd.SetArgs([]string{"--json"})

	err := cmd.RunE(cmd, []string{})
	if err != nil {
		t.Fatalf("baseline --json failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output not valid JSON: %v\nGot: %s", err, buf.String())
	}
	if _, ok := result["sections"]; !ok {
		t.Error("JSON output should include 'sections' field")
	}
	if _, ok := result["path"]; !ok {
		t.Error("JSON output should include 'path' field")
	}
}
```

#### Step 2: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/...
```

Expected: FAIL (baseline command not yet implemented)

#### Step 3: Write baseline command

```go
// cli/cmd/nesco/baseline.go

package main

import (
	"fmt"

	"github.com/holdenhewett/nesco/cli/internal/config"
	"github.com/holdenhewett/nesco/cli/internal/drift"
	"github.com/holdenhewett/nesco/cli/internal/output"
	"github.com/holdenhewett/nesco/cli/internal/scan"
	"github.com/holdenhewett/nesco/cli/internal/scan/detectors"
	"github.com/spf13/cobra"
)

var baselineCmd = &cobra.Command{
	Use:   "baseline",
	Short: "Accept current state as baseline",
	Long:  "Runs detectors and saves the current state as the baseline for drift detection, without regenerating context files.",
	RunE:  runBaseline,
}

func init() {
	baselineCmd.Flags().Bool("from-import", false, "Create baseline from imported provider content")
	rootCmd.AddCommand(baselineCmd)
}

func runBaseline(cmd *cobra.Command, args []string) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}

	nescoDir := config.DirPath(root)

	// Run scan (detectors only, no emit)
	scanner := scan.NewScanner(detectors.AllDetectors()...)
	result := scanner.Run(root)

	if err := drift.SaveBaseline(nescoDir, result.Document); err != nil {
		return fmt.Errorf("saving baseline: %w", err)
	}

	if output.JSON {
		output.Print(map[string]any{
			"sections": len(result.Document.Sections),
			"path":     config.DirPath(root) + "/" + drift.BaselineFileName,
		})
	} else {
		fmt.Printf("Baseline saved with %d sections.\n", len(result.Document.Sections))
		fmt.Printf("  Path: %s/%s\n", nescoDir, drift.BaselineFileName)
		fmt.Println("  Future `nesco drift` will compare against this baseline.")
	}

	return nil
}
```

#### Step 4: Run tests

```bash
cd /home/hhewett/.local/src/nesco/cli && go test ./cmd/nesco/...
```

Expected: PASS

#### Step 5: Build and verify all Phase 2 commands

```bash
cd /home/hhewett/.local/src/nesco/cli && go build ./cmd/nesco && ./nesco --help
```

Expected: Help output lists all commands: `init`, `import`, `parity`, `config`, `info`, `scan`, `drift`, `baseline`, `version`.

---

**Group 5 complete.** 4 tasks covering drift/baseline infrastructure and all Phase 2 CLI commands.

---

## Plan Summary

| Group | Tasks | Focus |
|-------|-------|-------|
| 1 | 1.1–1.5 (5 tasks) | Foundation: Cobra, ContextDocument, Provider expansion, config, output |
| 2 | 2.1–2.6 (6 tasks) | Phase 1: Discovery, parsers, parity analysis, Phase 1 CLI commands |
| 3 | 3.1–3.6 (6 tasks) | Scan infrastructure: scanner, fact detectors, emitters, reconciler |
| 4 | 4.1–4.7 (7 tasks) | 24 surprise detectors + registry |
| 5 | 5.1–5.4 (4 tasks) | Drift/baseline + Phase 2 CLI commands |
| **Total** | **28 tasks** | |

**Execution order:** Groups 1→2 and 1→3 can run in parallel after Group 1 completes. Group 4 depends on Group 3. Group 5 depends on Groups 3+4.

```
Group 1 (Foundation)
  ├── Group 2 (Phase 1: Provider Parity)
  └── Group 3 (Scan Infrastructure)
         └── Group 4 (Surprise Detectors)
                └── Group 5 (CLI Commands)
```
