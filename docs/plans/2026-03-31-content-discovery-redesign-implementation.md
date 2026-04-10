# Content Discovery Redesign — Implementation Plan

**Design doc:** `docs/plans/2026-03-31-content-discovery-redesign-design.md`
**New package:** `cli/internal/analyzer/`
**Module:** `github.com/OpenScribbler/syllago/cli`

Each task is 2–5 minutes of work. Tasks follow TDD rhythm: write test → run (fail) → implement → run (pass) → commit. Dependencies are stated explicitly. All file paths are absolute from repo root (`/home/hhewett/.local/src/syllago`).

---

## Phase 1 — ManifestItem schema extension

### Task 1.1 — Extend ManifestItem with optional fields

**Files modified:**
- `cli/internal/registry/registry.go`

**What to write:** Replace the existing `ManifestItem` struct with the extended version. Add the new optional fields below the existing ones:

```go
type ManifestItem struct {
    // Base fields (authored and generated manifests)
    Name      string   `yaml:"name"`
    Type      string   `yaml:"type"`
    Provider  string   `yaml:"provider,omitempty"`
    Path      string   `yaml:"path"`
    HookEvent string   `yaml:"hookEvent,omitempty"`
    HookIndex int      `yaml:"hookIndex,omitempty"`
    Scripts   []string `yaml:"scripts,omitempty"`

    // Extended fields (populated by analyzer; optional in authored manifests)
    DisplayName  string   `yaml:"displayName,omitempty"`
    Description  string   `yaml:"description,omitempty"`
    ContentHash  string   `yaml:"contentHash,omitempty"`
    References   []string `yaml:"references,omitempty"`
    ConfigSource string   `yaml:"configSource,omitempty"`
    Providers    []string `yaml:"providers,omitempty"`
}
```

**NOTE (Phase B):** `Manifest.Version` already exists in `registry.go` (line 230: `Version string yaml:"version,omitempty"`). Do NOT add a duplicate field. Verify with `grep 'Version' cli/internal/registry/registry.go` before making any change.

**Test file:** `cli/internal/registry/registry_test.go` (already exists — add cases)

Add a table-driven test `TestManifestItemRoundtrip` that marshals and unmarshals a `ManifestItem` with all new fields populated and verifies the round-trip is lossless. Test with `gopkg.in/yaml.v3`.

**Test command:** `cd cli && go test ./internal/registry/ -run TestManifestItemRoundtrip`

**Dependencies:** None.

---

## Phase 2 — Core analyzer types and interface

### Task 2.1 — Create analyzer package with core types

**Files to create:**
- `cli/internal/analyzer/types.go`

**What to write:**

```go
package analyzer

import "github.com/OpenScribbler/syllago/cli/internal/catalog"

// DetectionPattern declares a glob pattern a detector recognizes.
type DetectionPattern struct {
    Glob        string
    ContentType catalog.ContentType
    // InternalLabel overrides ContentType for detector-internal classification.
    // Use for "hook-script", "hook-wiring", "plugin-manifest", "output-style".
    // Empty means use ContentType directly.
    InternalLabel string
    Confidence    float64
}

// DependencyRef references another content item this item depends on.
type DependencyRef struct {
    Registry string
    Type     catalog.ContentType
    Name     string
}

// DetectedItem is the output of a detector's Classify call.
type DetectedItem struct {
    Name          string
    Type          catalog.ContentType
    InternalLabel string   // "hook-script", "hook-wiring", etc. Empty = use Type.
    Provider      string
    Path          string   // primary file or directory (relative to repoRoot)
    ContentHash   string   // SHA-256 of primary file content
    Confidence    float64
    Scripts       []string // referenced script files (relative to repoRoot)
    References    []string // other files needed (relative to repoRoot)
    Dependencies  []DependencyRef
    HookEvent     string
    HookIndex     int
    ConfigSource  string   // where wiring was found (e.g., ".claude/settings.json")
    DisplayName   string
    Description   string
    // A1: Providers holds alias paths for deduplicated items — paths of lower-confidence
    // duplicates that were suppressed in favor of this item (same name+type+hash).
    // Defined here (not in Phase 8) so dedup.go compiles without a separate struct edit.
    Providers     []string
}

// ContentDetector is the interface every provider detector implements.
type ContentDetector interface {
    // ProviderSlug returns the detector's provider identifier.
    ProviderSlug() string
    // Patterns returns the glob patterns this detector recognizes.
    Patterns() []DetectionPattern
    // Classify inspects a candidate path and returns detected items.
    // Returns nil if the path matched a pattern but content inspection
    // shows it is not actually content.
    // repoRoot is filepath.EvalSymlinks-resolved before being passed in.
    Classify(path string, repoRoot string) ([]*DetectedItem, error)
}

// ConfidenceCategory partitions items for UI presentation.
type ConfidenceCategory int

const (
    CategoryAuto    ConfidenceCategory = iota // > autoThreshold
    CategoryConfirm                           // >= skipThreshold and <= autoThreshold
    CategorySkip                              // < skipThreshold (never included in manifest)
)

// DefaultAutoThreshold is the default minimum confidence for auto-detection.
const DefaultAutoThreshold = 0.80

// DefaultSkipThreshold is the default minimum confidence to include at all.
const DefaultSkipThreshold = 0.50

// AnalysisResult holds the output of a full repository analysis.
type AnalysisResult struct {
    Auto    []*DetectedItem // confidence > AutoThreshold (excluding hooks/MCP)
    Confirm []*DetectedItem // confidence in [SkipThreshold, AutoThreshold], plus ALL hooks/MCP
    // Skipped items are not returned (use verbose logging)
    Warnings []string
}

// AnalysisConfig controls analyzer behavior.
type AnalysisConfig struct {
    AutoThreshold  float64  // default DefaultAutoThreshold
    SkipThreshold  float64  // default DefaultSkipThreshold
    ExcludeDirs    []string // additional per-registry exclusions
    SymlinkPolicy  string   // "ask", "follow", "skip"
}

// DefaultConfig returns the default analysis configuration.
func DefaultConfig() AnalysisConfig {
    return AnalysisConfig{
        AutoThreshold: DefaultAutoThreshold,
        SkipThreshold: DefaultSkipThreshold,
        SymlinkPolicy: "ask",
    }
}
```

**Test file:** `cli/internal/analyzer/types_test.go`

Write `TestDefaultConfig` verifying `DefaultConfig()` returns the expected threshold values and symlink policy.

**Test command:** `cd cli && go test ./internal/analyzer/ -run TestDefaultConfig`

**Dependencies:** Task 1.1 (uses `catalog.ContentType`).

---

### Task 2.2 — Filesystem walker with exclusion list

**Files to create:**
- `cli/internal/analyzer/walk.go`
- `cli/internal/analyzer/walk_test.go`

**What to write in `walk.go`:**

```go
package analyzer

import (
    "io/fs"
    "os"
    "path/filepath"
    "strings"
)

// defaultExcludeDirs are directories always skipped during analysis walks.
var defaultExcludeDirs = []string{
    "node_modules", "vendor", ".git", "dist", "build",
    "__pycache__", ".venv", ".tox", ".mypy_cache",
}

// binaryExtensions are file extensions always skipped (binary content).
var binaryExtensions = map[string]bool{
    ".exe": true, ".bin": true, ".so": true, ".dylib": true, ".dll": true,
    ".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".ico": true,
    ".pdf": true, ".zip": true, ".tar": true, ".gz": true, ".whl": true,
    ".pyc": true, ".class": true,
}

// walkLimits are per-repo limits to prevent resource exhaustion.
const (
    walkMaxFiles = 50_000
    walkMaxDepth = 30
)

// WalkResult holds all file paths collected during a walk.
type WalkResult struct {
    Paths    []string // all file paths relative to root
    Warnings []string
}

// Walk collects all non-excluded file paths under root.
// extraExcludeDirs are appended to defaultExcludeDirs.
// root must already be filepath.EvalSymlinks-resolved.
func Walk(root string, extraExcludeDirs []string) WalkResult {
    excluded := buildExcludeSet(extraExcludeDirs)
    var result WalkResult
    depth := func(path string) int {
        rel, _ := filepath.Rel(root, path)
        return strings.Count(rel, string(filepath.Separator))
    }

    _ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            result.Warnings = append(result.Warnings, "walk error at "+path+": "+err.Error())
            return nil
        }
        if d.IsDir() {
            name := d.Name()
            if excluded[name] {
                return filepath.SkipDir
            }
            if depth(path) > walkMaxDepth {
                result.Warnings = append(result.Warnings, "max depth reached at "+path)
                return filepath.SkipDir
            }
            return nil
        }
        if binaryExtensions[strings.ToLower(filepath.Ext(d.Name()))] {
            return nil
        }
        if len(result.Paths) >= walkMaxFiles {
            result.Warnings = append(result.Warnings, "file limit reached; truncating walk")
            return fs.SkipAll
        }
        rel, relErr := filepath.Rel(root, path)
        if relErr != nil {
            return nil
        }
        result.Paths = append(result.Paths, rel)
        return nil
    })
    return result
}

func buildExcludeSet(extra []string) map[string]bool {
    set := make(map[string]bool, len(defaultExcludeDirs)+len(extra))
    for _, d := range defaultExcludeDirs {
        set[d] = true
    }
    for _, d := range extra {
        set[filepath.Base(d)] = true
    }
    return set
}
```

**What to write in `walk_test.go`:** Table-driven tests covering:
- Empty directory returns zero paths
- `node_modules/` is excluded
- Extra exclude dirs are skipped
- Binary files (`.exe`, `.png`) are excluded
- Symlinks to files within root are included
- Depth limit warning fires correctly (create a 31-level deep nested dir tree)
- File limit warning fires (create a dir with `walkMaxFiles+1` stub files... use a smaller override for testing; add a `walkMaxFilesOverride` package-level var defaulting to 50000 that tests set to a small number)

Add `var walkMaxFilesOverrideForTest int` and use `max(walkMaxFiles, walkMaxFilesOverrideForTest)` → actually, simplest approach: add an unexported package var `testMaxFiles int` that, when nonzero, replaces `walkMaxFiles`. Reset in test cleanup.

**Test command:** `cd cli && go test ./internal/analyzer/ -run TestWalk`

**Dependencies:** Task 2.1.

---

### Task 2.3 — Glob pattern matcher

**Files to create:**
- `cli/internal/analyzer/match.go`
- `cli/internal/analyzer/match_test.go`

**What to write in `match.go`:**

```go
package analyzer

import "path/filepath"

// CandidateMatch records one detector's claim on a file path.
type CandidateMatch struct {
    Path     string
    Pattern  DetectionPattern
    Detector ContentDetector
}

// MatchPatterns evaluates all detectors' patterns against the path index.
// Returns one CandidateMatch per (detector, path) pair where the glob matched.
// paths are relative to repoRoot (as returned by Walk), normalized to forward
// slashes via filepath.ToSlash before matching (B4: ensures patterns work
// on all platforms since filepath.Match uses the OS separator on Windows).
func MatchPatterns(paths []string, detectors []ContentDetector) []CandidateMatch {
    var matches []CandidateMatch
    for _, det := range detectors {
        for _, pat := range det.Patterns() {
            for _, p := range paths {
                // Normalize to forward slashes before matching.
                // All detector patterns use forward slashes; Walk returns
                // filepath.Rel results which use the OS separator on Windows.
                normalized := filepath.ToSlash(p)
                ok, err := filepath.Match(pat.Glob, normalized)
                if err != nil {
                    continue
                }
                if ok {
                    matches = append(matches, CandidateMatch{
                        Path:     p,
                        Pattern:  pat,
                        Detector: det,
                    })
                }
            }
        }
    }
    return matches
}
```

> **B3 RESOLVED — `matchGlob` / `**` removed:** The original plan included a `matchGlob` helper claiming to support `**`, but no detector pattern actually uses `**` — all patterns use single `*` (e.g., `skills/*/SKILL.md`). The proposed implementation of replacing `**` with `*` was also semantically wrong (it would only match one segment, not zero-or-more, making it a misleading abstraction). The `**` machinery is removed entirely. `filepath.Match` is used directly after normalizing to forward slashes (see B4 fix above). If true `**` support is ever needed, add `github.com/bmatcuk/doublestar` as a dependency at that time.

> **B4 RESOLVED — `filepath.ToSlash` added:** Paths from `Walk` use `filepath.Rel` which returns OS-native separators (`\` on Windows). All detector glob patterns use forward slashes. Normalizing with `filepath.ToSlash` before matching ensures cross-platform correctness. This is a defensive measure — the dev environment is Linux-only but avoids future portability surprises.

**What to write in `match_test.go`:** Table-driven tests covering:
- `.cursorrules` matches the `cursor` pattern `.cursorrules`
- `skills/my-skill/SKILL.md` matches `skills/*/SKILL.md`
- `hooks/claude-code/my-hook/hook.json` matches `hooks/*/*/hook.json`
- `.claude/agents/reviewer.md` matches `.claude/agents/*.md`
- `node_modules/foo.md` does NOT match `rules/*.md` (excluded at walk time, not here — confirm this pattern does not match `node_modules/foo.md` with a pattern of `rules/*.md`)
- Multiple detectors each get their own match entries for the same path

**Test command:** `cd cli && go test ./internal/analyzer/ -run TestMatchPatterns`

**Dependencies:** Task 2.2 (imports types).

---

## Phase 3 — Syllago canonical detector

### Task 3.1 — Syllago canonical detector implementation

**Files to create:**
- `cli/internal/analyzer/detector_syllago.go`
- `cli/internal/analyzer/detector_syllago_test.go`

**What to write in `detector_syllago.go`:**

```go
package analyzer

import (
    "crypto/sha256"
    "encoding/hex"
    "os"
    "path/filepath"
    "strings"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// SyllagoDetector detects content organized in syllago's canonical layout.
type SyllagoDetector struct{}

func (d *SyllagoDetector) ProviderSlug() string { return "syllago" }

func (d *SyllagoDetector) Patterns() []DetectionPattern {
    return []DetectionPattern{
        {Glob: "skills/*/SKILL.md",         ContentType: catalog.Skills,   Confidence: 0.95},
        {Glob: "agents/*/AGENT.md",         ContentType: catalog.Agents,   Confidence: 0.95},
        {Glob: "hooks/*/*/hook.json",        ContentType: catalog.Hooks,    Confidence: 0.95},
        {Glob: "mcp/*/config.json",         ContentType: catalog.MCP,      Confidence: 0.95},
        {Glob: "rules/*/*/rule.md",         ContentType: catalog.Rules,    Confidence: 0.95},
        {Glob: "commands/*/*/command.md",   ContentType: catalog.Commands, Confidence: 0.95},
        {Glob: "loadouts/*/loadout.yaml",   ContentType: catalog.Loadouts, Confidence: 0.95},
    }
}

func (d *SyllagoDetector) Classify(path string, repoRoot string) ([]*DetectedItem, error) {
    absPath := filepath.Join(repoRoot, path)
    data, err := readFileLimited(absPath, limitMarkdown)
    if err != nil {
        return nil, nil // skip unreadable files
    }

    // Derive name from path (second segment for most types, third for provider-specific)
    name := nameFromPath(path)
    if name == "" {
        return nil, nil
    }

    ct := ctFromSyllagoPath(path)
    if ct == "" {
        return nil, nil
    }

    item := &DetectedItem{
        Name:        name,
        Type:        ct,
        Provider:    "syllago",
        Path:        filepath.Dir(path), // directory containing the primary file
        ContentHash: hashBytes(data),
        Confidence:  0.95,
    }

    // Extract display name and description from frontmatter for skills/agents.
    switch ct {
    case catalog.Skills, catalog.Agents:
        if fm := parseFrontmatterBasic(data); fm != nil {
            item.DisplayName = fm.name
            item.Description = fm.description
        }
    }

    return []*DetectedItem{item}, nil
}

// nameFromPath extracts the item name from a syllago canonical path.
// "skills/my-skill/SKILL.md" → "my-skill"
// "hooks/claude-code/lint/hook.json" → "lint"
func nameFromPath(path string) string {
    parts := strings.Split(filepath.ToSlash(path), "/")
    switch len(parts) {
    case 3: // skills/name/SKILL.md, agents/name/AGENT.md, etc.
        return parts[1]
    case 4: // hooks/provider/name/hook.json, rules/provider/name/rule.md
        return parts[2]
    }
    return ""
}

// ctFromSyllagoPath maps the first path segment to a ContentType.
func ctFromSyllagoPath(path string) catalog.ContentType {
    first := strings.Split(filepath.ToSlash(path), "/")[0]
    switch first {
    case "skills":
        return catalog.Skills
    case "agents":
        return catalog.Agents
    case "hooks":
        return catalog.Hooks
    case "mcp":
        return catalog.MCP
    case "rules":
        return catalog.Rules
    case "commands":
        return catalog.Commands
    case "loadouts":
        return catalog.Loadouts
    }
    return ""
}
```

Also add shared helpers to a new file `cli/internal/analyzer/helpers.go`:

```go
package analyzer

import (
    "bufio"
    "bytes"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "os"
    "strings"
)

// Size limits for file reads during analysis.
const (
    limitMarkdown = 1 * 1024 * 1024  // 1 MB
    limitJSON     = 256 * 1024       // 256 KB
)

// readFileLimited reads a file, returning an error if it exceeds maxBytes.
func readFileLimited(path string, maxBytes int64) ([]byte, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()

    info, err := f.Stat()
    if err != nil {
        return nil, err
    }
    if info.Size() > maxBytes {
        return nil, fmt.Errorf("file %s exceeds size limit (%d bytes)", path, maxBytes)
    }
    data := make([]byte, info.Size())
    _, err = f.Read(data)
    return data, err
}

// hashBytes returns the SHA-256 hex digest of data.
func hashBytes(data []byte) string {
    sum := sha256.Sum256(data)
    return hex.EncodeToString(sum[:])
}

// basicFrontmatter holds the name and description fields extracted from YAML frontmatter.
type basicFrontmatter struct {
    name        string
    description string
}

// parseFrontmatterBasic extracts name and description from YAML frontmatter.
// Handles the "---\n...\n---" format. Returns nil if no frontmatter found.
func parseFrontmatterBasic(data []byte) *basicFrontmatter {
    s := string(data)
    if !strings.HasPrefix(strings.TrimSpace(s), "---") {
        return nil
    }
    // Find closing ---
    rest := s[strings.Index(s, "---")+3:]
    end := strings.Index(rest, "\n---")
    if end < 0 {
        return nil
    }
    block := rest[:end]
    fm := &basicFrontmatter{}
    scanner := bufio.NewScanner(bytes.NewBufferString(block))
    for scanner.Scan() {
        line := scanner.Text()
        if k, v, ok := strings.Cut(line, ":"); ok {
            k = strings.TrimSpace(k)
            v = strings.TrimSpace(v)
            switch k {
            case "name":
                fm.name = v
            case "description":
                fm.description = v
            }
        }
    }
    return fm
}
```

**What to write in `detector_syllago_test.go`:** Table-driven tests:
- `Patterns()` returns exactly 7 patterns, all with confidence 0.95
- `Classify` on a `skills/my-skill/SKILL.md` with valid frontmatter returns a `DetectedItem` with `Name="my-skill"`, `Type=catalog.Skills`, `Confidence=0.95`
- `Classify` on a `hooks/claude-code/lint/hook.json` returns `Type=catalog.Hooks`, `Name="lint"`
- `Classify` on a path that doesn't exist returns `nil, nil`
- `Classify` on a file exceeding 1MB returns `nil, nil`
- `ContentHash` is a 64-character hex string (SHA-256)
- `ProviderSlug()` returns `"syllago"`

**Test command:** `cd cli && go test ./internal/analyzer/ -run TestSyllagoDetector`

**Dependencies:** Tasks 2.1, 2.3.

---

## Phase 4 — Simple provider detectors (batch)

### Task 4.1 — Windsurf, Cline, Roo Code, Codex, Gemini detectors

**Files to create:**
- `cli/internal/analyzer/detector_simple.go`
- `cli/internal/analyzer/detector_simple_test.go`

**What to write in `detector_simple.go`:**

Implement five simple detectors as structs with `Patterns()` declaring the globs from the design doc and `Classify()` reading the file, checking it is non-empty, and returning a `DetectedItem`. Each follows the same structure. Example for Windsurf:

```go
// WindsurfDetector detects Windsurf content.
type WindsurfDetector struct{}

func (d *WindsurfDetector) ProviderSlug() string { return "windsurf" }

func (d *WindsurfDetector) Patterns() []DetectionPattern {
    return []DetectionPattern{
        {Glob: ".windsurfrules", ContentType: catalog.Rules, Confidence: 0.95},
    }
}

func (d *WindsurfDetector) Classify(path string, repoRoot string) ([]*DetectedItem, error) {
    absPath := filepath.Join(repoRoot, path)
    data, err := readFileLimited(absPath, limitMarkdown)
    if err != nil || len(bytes.TrimSpace(data)) == 0 {
        return nil, nil
    }
    return []*DetectedItem{{
        Name:        ".windsurfrules",
        Type:        catalog.Rules,
        Provider:    "windsurf",
        Path:        path,
        ContentHash: hashBytes(data),
        Confidence:  0.95,
    }}, nil
}
```

Implement the same pattern for:

**ClineDetector** (`ProviderSlug: "cline"`):
- `.clinerules` → Rules, 0.95
- `.clinerules/*.md` → Rules, 0.90 (directory: list entries, return one item per file)

**RooCodeDetector** (`ProviderSlug: "roo-code"`):
- `.roo/rules/*.md` → Rules, 0.90
- `.roomodes` → Rules, 0.85

**CodexDetector** (`ProviderSlug: "codex"`):
- `AGENTS.md` → inspect content; if it contains agent-like headers, classify as `catalog.Agents` confidence 0.85; otherwise `catalog.Rules` confidence 0.85
- `.codex/agents/*.toml` → Agents, 0.85

**GeminiDetector** (`ProviderSlug: "gemini-cli"`):
- `GEMINI.md` → Rules, 0.85 (project-level instruction file)
- `.gemini/skills/*/SKILL.md` → Skills, 0.85

For glob patterns that match directories of files (e.g., `.clinerules/*.md`), `Classify` receives the individual file path after pattern matching, so the same single-file logic applies.

**What to write in `detector_simple_test.go`:** For each detector:
- `ProviderSlug()` returns expected slug
- `Patterns()` returns the expected count with correct confidences
- `Classify` on a temp file with content returns a non-nil item with correct type
- `Classify` on an empty file returns nil
- `Classify` on a missing file returns nil

**Test command:** `cd cli && go test ./internal/analyzer/ -run TestSimpleDetectors`

**Dependencies:** Task 3.1 (uses helpers from `helpers.go`).

---

### Task 4.2 — Copilot detector

**Files to create:**
- `cli/internal/analyzer/detector_copilot.go`
- `cli/internal/analyzer/detector_copilot_test.go`

**What to write in `detector_copilot.go`:**

```go
type CopilotDetector struct{}

func (d *CopilotDetector) ProviderSlug() string { return "vs-code-copilot" }

func (d *CopilotDetector) Patterns() []DetectionPattern {
    return []DetectionPattern{
        {Glob: ".github/copilot-instructions.md",        ContentType: catalog.Rules,  Confidence: 0.95},
        {Glob: ".github/instructions/*.instructions.md", ContentType: catalog.Rules,  Confidence: 0.90},
        {Glob: ".github/agents/*.md",                    ContentType: catalog.Agents, Confidence: 0.90},
    }
}
```

`Classify` reads the file with `readFileLimited`. For `.github/agents/*.md` files, parse frontmatter for `name` and `description`. Return a `DetectedItem` with `Provider: "vs-code-copilot"`.

**Test file:** `detector_copilot_test.go` — same pattern as Task 4.1 tests.

**Test command:** `cd cli && go test ./internal/analyzer/ -run TestCopilotDetector`

**Dependencies:** Task 4.1.

---

## Phase 5 — Top-level provider-agnostic detector

### Task 5.1 — TopLevel detector

**Files to create:**
- `cli/internal/analyzer/detector_toplevel.go`
- `cli/internal/analyzer/detector_toplevel_test.go`

**What to write in `detector_toplevel.go`:**

```go
type TopLevelDetector struct{}

func (d *TopLevelDetector) ProviderSlug() string { return "top-level" }

func (d *TopLevelDetector) Patterns() []DetectionPattern {
    return []DetectionPattern{
        {Glob: "agents/*.md",                   ContentType: catalog.Agents,   Confidence: 0.85},
        {Glob: "agents/*/*.md",                 ContentType: catalog.Agents,   Confidence: 0.80},
        {Glob: "commands/*.md",                 ContentType: catalog.Commands, Confidence: 0.85},
        {Glob: "commands/*/*.md",               ContentType: catalog.Commands, Confidence: 0.80},
        {Glob: "rules/*.md",                    ContentType: catalog.Rules,    Confidence: 0.80},
        {Glob: "rules/*.mdc",                   ContentType: catalog.Rules,    Confidence: 0.80},
        {Glob: "hooks/*.py",                    ContentType: catalog.Hooks,    InternalLabel: "hook-script", Confidence: 0.60},
        {Glob: "hooks/*.js",                    ContentType: catalog.Hooks,    InternalLabel: "hook-script", Confidence: 0.60},
        {Glob: "hooks/*.ts",                    ContentType: catalog.Hooks,    InternalLabel: "hook-script", Confidence: 0.60},
        {Glob: "hooks/*.sh",                    ContentType: catalog.Hooks,    InternalLabel: "hook-script", Confidence: 0.60},
        {Glob: "hooks/hooks.json",              ContentType: catalog.Hooks,    InternalLabel: "hook-wiring", Confidence: 0.85},
        {Glob: "hook-scripts/*/*.js",           ContentType: catalog.Hooks,    InternalLabel: "hook-script", Confidence: 0.70},
        // B2 RESOLVED — prompts/*.md maps to catalog.Rules (not a separate Prompts type).
        // Decision: collapse Prompts into Rules. Adding a separate catalog.Prompts ContentType
        // would require updates to AllContentTypes(), Label(), TUI display, scanner, installer,
        // and provider configs — significant downstream blast radius for a type that behaves
        // identically to Rules at every layer. Prompts are instructions; treating them as Rules
        // is semantically correct and keeps the type system minimal. If a dedicated Prompts type
        // is ever needed, add it as a task before Task 5.1.
        {Glob: "prompts/*.md",                  ContentType: catalog.Rules,    Confidence: 0.75},
    }
}
```

`Classify` returns `nil` for `hook-wiring` patterns (these are consumed during hook correlation in a later phase, not directly classified into items). For `hook-script`, return an item with `InternalLabel: "hook-script"` and `Confidence` from the pattern. For standard content types, read file and return item with frontmatter-derived name/description.

**What to write in `detector_toplevel_test.go`:** Tests for each pattern category. Include:
- `agents/*.md` → item with `Type=catalog.Agents`, `Confidence=0.85`
- `hooks/lint.ts` → item with `InternalLabel="hook-script"`, `Confidence=0.60`
- `hooks/hooks.json` → returns nil (wiring, not a classifiable item)
- Empty file → nil

**Test command:** `cd cli && go test ./internal/analyzer/ -run TestTopLevelDetector`

**Dependencies:** Task 4.2.

---

## Phase 6 — Cursor detector

### Task 6.1 — Cursor detector

**Files to create:**
- `cli/internal/analyzer/detector_cursor.go`
- `cli/internal/analyzer/detector_cursor_test.go`

**What to write in `detector_cursor.go`:**

```go
type CursorDetector struct{}

func (d *CursorDetector) ProviderSlug() string { return "cursor" }

func (d *CursorDetector) Patterns() []DetectionPattern {
    return []DetectionPattern{
        {Glob: ".cursorrules",                ContentType: catalog.Rules,    Confidence: 0.95},
        {Glob: ".cursor/rules/*.mdc",         ContentType: catalog.Rules,    Confidence: 0.90},
        {Glob: ".cursor/rules/*.md",          ContentType: catalog.Rules,    Confidence: 0.85},
        {Glob: ".cursor/agents/*.md",         ContentType: catalog.Agents,   Confidence: 0.90},
        {Glob: ".cursor/skills/*/SKILL.md",   ContentType: catalog.Skills,   Confidence: 0.90},
        {Glob: ".cursor/commands/*.md",       ContentType: catalog.Commands, Confidence: 0.90},
        {Glob: ".cursor/hooks.json",          ContentType: catalog.Hooks,    InternalLabel: "hook-wiring", Confidence: 0.90},
        {Glob: ".cursor/hooks/*",             ContentType: catalog.Hooks,    InternalLabel: "hook-script", Confidence: 0.70},
    }
}
```

`Classify` for `.cursor/hooks.json` returns nil (wiring is handled in hook correlation). For `.cursor/hooks/*` returns a hook-script item. For all others, reads file and returns item with frontmatter-derived metadata. `.cursor/skills/*/SKILL.md` extracts name from the directory segment (second `*`).

**Test file:** Standard pattern — one test per pattern type, plus:
- `.cursor/hooks.json` → nil
- `.cursor/hooks/lint.sh` → hook-script item

**Test command:** `cd cli && go test ./internal/analyzer/ -run TestCursorDetector`

**Dependencies:** Task 5.1.

---

## Phase 7 — Claude Code detector and Plugin detector

### Task 7.1 — Claude Code detector (hook correlation)

This is the most complex detector. It must read `.claude/settings.json`, parse hook entries, resolve script paths, and produce fully-wired `DetectedItem` structs.

**Files to create:**
- `cli/internal/analyzer/detector_cc.go`
- `cli/internal/analyzer/detector_cc_test.go`

**What to write in `detector_cc.go`:**

```go
package analyzer

import (
    "bytes"
    "encoding/json"
    "os"
    "path/filepath"
    "strings"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
    "github.com/tidwall/gjson"
)

// ClaudeCodeDetector detects Claude Code content.
type ClaudeCodeDetector struct{}

func (d *ClaudeCodeDetector) ProviderSlug() string { return "claude-code" }

func (d *ClaudeCodeDetector) Patterns() []DetectionPattern {
    return []DetectionPattern{
        {Glob: ".claude/agents/*.md",          ContentType: catalog.Agents,   Confidence: 0.90},
        {Glob: ".claude/skills/*/SKILL.md",    ContentType: catalog.Skills,   Confidence: 0.90},
        {Glob: ".claude/commands/*.md",        ContentType: catalog.Commands, Confidence: 0.90},
        {Glob: ".claude/rules/*.md",           ContentType: catalog.Rules,    Confidence: 0.90},
        {Glob: ".claude/hooks/*",              ContentType: catalog.Hooks,    InternalLabel: "hook-script", Confidence: 0.70},
        {Glob: ".claude/settings.json",        ContentType: catalog.Hooks,    InternalLabel: "hook-wiring", Confidence: 0.90},
        {Glob: ".claude/output-styles/*.md",   ContentType: catalog.Rules,    Confidence: 0.85},
        {Glob: ".mcp.json",                    ContentType: catalog.MCP,      Confidence: 0.90},
        {Glob: "CLAUDE.md",                    ContentType: catalog.Rules,    Confidence: 0.80},
    }
}

func (d *ClaudeCodeDetector) Classify(path string, repoRoot string) ([]*DetectedItem, error) {
    // Hook-wiring: parse settings.json and emit fully-wired hook items.
    if path == ".claude/settings.json" {
        return classifyCCSettings(path, repoRoot)
    }
    // Hook-script: return as hook-script item (suppressed later if consumed by wired hook).
    if strings.HasPrefix(path, ".claude/hooks/") {
        return classifyCCHookScript(path, repoRoot)
    }
    // MCP: parse .mcp.json for server entries.
    if path == ".mcp.json" {
        return classifyCCMCP(path, repoRoot)
    }
    // All other patterns: read file, extract metadata.
    return classifyCCContent(path, repoRoot)
}
```

Implement the following helper functions in the same file:

**`classifyCCSettings(path, repoRoot string) ([]*DetectedItem, error)`:**
1. Read `.claude/settings.json` with `readFileLimited(absPath, limitJSON)`
2. Extract `hooks` object with gjson
3. For each event key and each hook entry in the array:
   - Extract `command` field
   - Call `resolveHookScript(command, repoRoot)` to get the script path (may be empty for inline commands)
   - Create a `DetectedItem` with:
     - `Type: catalog.Hooks`
     - `InternalLabel: "hook"` (fully wired)
     - `HookEvent: <canonical event name from toolmap lookup>`
     - `HookIndex: <index in array>`
     - `Scripts: [<resolved script path>]` (if non-empty)
     - `ConfigSource: ".claude/settings.json"`
     - `Confidence: 0.90`
     - `Provider: "claude-code"`
     - `Path: <resolved script path if exists, else ".claude/settings.json">`
     - `Name: <script basename or event:index>`

**`resolveHookScript(command, repoRoot string) string`:**
1. Split command on whitespace
2. For each token, check if it contains `/` or `\`, or ends in `.sh`, `.py`, `.js`, `.ts`, `.rb`, `.bash`
3. If found, resolve against repoRoot: `filepath.Join(repoRoot, token)`. If the file exists, return the token (relative path). Return `""` if no script found (inline command).

**`classifyCCHookScript(path, repoRoot string) ([]*DetectedItem, error)`:**
Read the file. If it exists and is non-empty, return a hook-script item with `Confidence: 0.70`. This item will be suppressed if another item's `Scripts` list references this path.

**`classifyCCMCP(path, repoRoot string) ([]*DetectedItem, error)`:**
Read `.mcp.json`, extract `mcpServers` keys with gjson, return one `DetectedItem` per server with `Type: catalog.MCP`, `Confidence: 0.90`. Name = server key.

> **B6 RESOLVED — `mcpServerDescription` is importable:** The `analyzer` package imports `catalog` (already established by Task 2.1). The `catalog` package does NOT import `analyzer` (analyzer is a new package). No cycle exists. Call `catalog.MCPServerDescription(value)` directly. However, `mcpServerDescription` in `scanner.go` is currently unexported (lowercase). Two options: (a) export it by renaming to `MCPServerDescription` in `scanner.go`, or (b) duplicate the 10-line logic in `helpers.go` with a reference comment. Prefer option (a): rename `mcpServerDescription` → `MCPServerDescription` in `scanner.go` and update its call sites in the same file. This is the cleaner approach — the function is general utility, not scanner-internal. Add the rename as a sub-step before writing `classifyCCMCP`.

**Sub-step:** In `cli/internal/catalog/scanner.go`, rename `mcpServerDescription` to `MCPServerDescription` (exported). Update the two call sites in `scanUniversal` and `scanMCPSubdirs`. Then call `catalog.MCPServerDescription(value)` from `classifyCCMCP`.

**Phase B serialization warning:** Task 13.1 also modifies `scanner.go` (adds fields to the `manifestItem` mirror struct and updates `scanFromIndex`). Task 7.1's sub-step and Task 13.1 MUST NOT run in parallel — they modify the same file. If executing phases out of order, complete the `scanner.go` rename here first and commit before Task 13.1 begins.

Use `catalog.MCPServerDescription(value)` for the `Description` field.

**`classifyCCContent(path, repoRoot string) ([]*DetectedItem, error)`:**
Read file with `readFileLimited`. Parse frontmatter for name/description. Map path to content type using the patterns table. Return one item.

**`ccCanonicalEvent(nativeName string) string`:** Maps CC's native event names (`PreToolUse`, `PostToolUse`, `UserPromptSubmit`, `PreCompact`, `Stop`, `SubagentStop`, `PostCompact`) to canonical names.

> **B7 RESOLVED — import from `toolmap.go` directly:** The `converter` package only imports `"strings"` (standard library) — verified in `toolmap.go` line 3. The `analyzer` package imports `catalog` but `converter` does not import `catalog` or `analyzer`. No import cycle exists. Import `"github.com/OpenScribbler/syllago/cli/internal/converter"` in `detector_cc.go` and call `converter.ReverseTranslateHookEvent(nativeName, "claude-code")` directly. Do NOT duplicate the `ccEventMap` — duplication would silently diverge whenever `toolmap.go` is updated (new CC event names, renamed events). There are no tests ensuring the two maps stay in sync. Use the converter import.

**What to write in `detector_cc_test.go`:**

Use `t.TempDir()` to create a repo root. Create test fixtures. Tests:

```go
func TestClaudeCodeDetector_Patterns(t *testing.T)         // 9 patterns
func TestClaudeCodeDetector_ClassifySettings(t *testing.T) // settings.json with 2 hooks → 2 items
func TestClaudeCodeDetector_ClassifyHookScript(t *testing.T) // .claude/hooks/lint.sh → hook-script item
func TestClaudeCodeDetector_ClassifyMCP(t *testing.T)      // .mcp.json with 2 servers → 2 items
func TestClaudeCodeDetector_ClassifyAgent(t *testing.T)    // .claude/agents/reviewer.md → agent item
func TestClaudeCodeDetector_SettingsWithScript(t *testing.T) // hook command references real script → Scripts populated
func TestClaudeCodeDetector_SettingsInlineCommand(t *testing.T) // echo foo → Scripts empty
func TestClaudeCodeDetector_MalformedSettings(t *testing.T) // returns nil items, no panic
```

For `TestClaudeCodeDetector_ClassifySettings`, create a temp repo with `.claude/settings.json`:
```json
{
  "hooks": {
    "PreToolUse": [{"matcher":"Bash","command":"bun hooks/validate.ts $FILE"}],
    "PostToolUse": [{"command":"echo done"}]
  }
}
```
Also create `hooks/validate.ts` in the temp dir. Verify the first item has `Scripts: ["hooks/validate.ts"]` and second has empty Scripts.

**Test command:** `cd cli && go test ./internal/analyzer/ -run TestClaudeCodeDetector`

**Dependencies:** Task 6.1.

---

### Task 7.2 — Claude Code Plugin detector

**Files to create:**
- `cli/internal/analyzer/detector_cc_plugin.go`
- `cli/internal/analyzer/detector_cc_plugin_test.go`

**What to write in `detector_cc_plugin.go`:**

```go
type ClaudeCodePluginDetector struct{}

func (d *ClaudeCodePluginDetector) ProviderSlug() string { return "claude-code-plugin" }

func (d *ClaudeCodePluginDetector) Patterns() []DetectionPattern {
    return []DetectionPattern{
        {Glob: ".claude-plugin/plugin.json",      ContentType: catalog.Skills,   InternalLabel: "plugin-manifest", Confidence: 0.90},
        {Glob: "plugins/*/agents/*.md",           ContentType: catalog.Agents,   Confidence: 0.90},
        {Glob: "plugins/*/skills/*/SKILL.md",     ContentType: catalog.Skills,   Confidence: 0.90},
        {Glob: "plugins/*/hooks/hooks.json",      ContentType: catalog.Hooks,    InternalLabel: "hook-wiring", Confidence: 0.90},
        {Glob: "plugins/*/commands/*.md",         ContentType: catalog.Commands, Confidence: 0.90},
    }
}
```

`Classify` for `plugin-manifest` reads the JSON and uses it as a lightweight manifest to discover declared content (returns nil — plugin.json parsing is handled as a special preprocessing step by the Analyzer engine in Phase 9). For all other patterns, same read-and-classify logic as other detectors.

For `plugins/*/hooks/hooks.json` (hook-wiring), return nil (handled during hook correlation).

**Test command:** `cd cli && go test ./internal/analyzer/ -run TestClaudeCodePluginDetector`

**Dependencies:** Task 7.1.

---

## Phase 8 — Dedup engine

### Task 8.1 — Same-name deduplication and hook script suppression

**Files to create:**
- `cli/internal/analyzer/dedup.go`
- `cli/internal/analyzer/dedup_test.go`

**What to write in `dedup.go`:**

```go
package analyzer

import "github.com/OpenScribbler/syllago/cli/internal/catalog"

// itemKey is the dedup identity for a DetectedItem.
type itemKey struct {
    contentType catalog.ContentType
    name        string
}

// DeduplicateItems processes classified items:
// 1. Suppresses hook-script items whose Path appears in another item's Scripts list.
// 2. Deduplicates same (type, name) items:
//    - Same content hash → keep highest confidence, record other paths in Providers alias.
//    - Different content hash → keep both (conflicts returned separately).
// Returns deduplicated items and conflict pairs.
func DeduplicateItems(items []*DetectedItem) (deduped []*DetectedItem, conflicts [][2]*DetectedItem) {
    // Step 1: Build set of script paths consumed by wired hooks.
    consumedScripts := make(map[string]bool)
    for _, item := range items {
        for _, s := range item.Scripts {
            consumedScripts[s] = true
        }
    }

    // Step 2: Filter out consumed hook-script items.
    var filtered []*DetectedItem
    for _, item := range items {
        if item.InternalLabel == "hook-script" && consumedScripts[item.Path] {
            continue // suppressed: consumed by a wired hook
        }
        filtered = append(filtered, item)
    }

    // Step 3: Dedup by (type, name).
    seen := make(map[itemKey]*DetectedItem)
    for _, item := range filtered {
        key := itemKey{item.Type, item.Name}
        existing, ok := seen[key]
        if !ok {
            seen[key] = item
            continue
        }
        if existing.ContentHash == item.ContentHash {
            // Same content: keep higher confidence, record alias.
            // Decision #19: syllago canonical detector always wins ties —
            // prefer syllago over provider-specific over top-level.
            // providerPriority maps slugs to a rank (lower = wins).
            incomingWins := item.Confidence > existing.Confidence ||
                (item.Confidence == existing.Confidence && providerPriority(item.Provider) < providerPriority(existing.Provider))
            if incomingWins {
                item.Providers = append(item.Providers, existing.Path)
                seen[key] = item
            } else {
                existing.Providers = append(existing.Providers, item.Path)
            }
        } else {
            // Different content: record conflict.
            conflicts = append(conflicts, [2]*DetectedItem{existing, item})
        }
    }

    for _, item := range seen {
        deduped = append(deduped, item)
    }
    return deduped, conflicts
}

// providerPriority returns a sort rank for a provider slug.
// Lower rank = higher priority in tiebreaks (Decision #19, #step4).
// syllago canonical > provider-specific > top-level agnostic.
func providerPriority(slug string) int {
    switch slug {
    case "syllago":
        return 0
    case "top-level":
        return 2
    default:
        return 1 // all named providers rank between syllago and top-level
    }
}
```

Note: `Providers []string` field is already defined in `DetectedItem` (added in Task 2.1 per A1 fix). No struct edit needed here.

**Phase B note:** `DeduplicateItems` builds `deduped` by iterating over a Go `map[itemKey]*DetectedItem`, which has non-deterministic iteration order. Tests MUST NOT assert on position (e.g., `deduped[0]`). Use a helper like `findItemByName(deduped, name)` or check `len(deduped)` + field assertions after lookup.

**What to write in `dedup_test.go`:** Table-driven tests:
- Hook-script item with path in another item's Scripts list → suppressed
- Hook-script item NOT in any Scripts list → not suppressed
- Two items same (type, name, hash), lower confidence first → keep higher confidence, alias recorded
- Two items same (type, name, hash), equal confidence, syllago vs. claude-code → syllago wins (Decision #19)
- Two items same (type, name, hash), equal confidence, claude-code vs. top-level → claude-code wins
- Two items same (type, name), different hash → both returned as conflict pair
- Cross-type same-name items are NOT deduped (skill "foo" and rule "foo" are separate)
- Empty input → empty output

**Test command:** `cd cli && go test ./internal/analyzer/ -run TestDeduplicateItems`

**Dependencies:** Task 7.2 (uses all detector types to generate test inputs).

---

## Phase 9 — Reference resolver

### Task 9.1 — Markdown link and backtick path resolver

**Files to create:**
- `cli/internal/analyzer/references.go`
- `cli/internal/analyzer/references_test.go`

**What to write in `references.go`:**

```go
package analyzer

import (
    "os"
    "path/filepath"
    "regexp"
    "strings"
)

// knownContentExts are extensions treated as resolvable content references.
var knownContentExts = map[string]bool{
    ".md": true, ".json": true, ".yaml": true, ".yml": true,
    ".ts": true, ".py": true, ".sh": true, ".js": true,
}

// mdLinkRe matches markdown relative links: [text](../path)
var mdLinkRe = regexp.MustCompile(`\[([^\]]*)\]\(([^)#]+)\)`)

// backtickPathRe matches backtick-quoted strings that look like file paths.
var backtickPathRe = regexp.MustCompile("`([^`]+\.[a-zA-Z]{1,5})`")

// knownSubdirs are subdirectory names that indicate supporting content.
var knownSubdirs = []string{"references", "scripts", "helpers", "assets"}

const maxRefDepth = 3

// ResolveReferences finds files referenced by a content item.
// path is the item's primary file path (relative to repoRoot).
// repoRoot must be filepath.EvalSymlinks-resolved.
// Returns relative paths of referenced files that exist within repoRoot.
func ResolveReferences(path string, repoRoot string) []string {
    return resolveRefs(path, repoRoot, 0, make(map[string]bool))
}

func resolveRefs(path, repoRoot string, depth int, visited map[string]bool) []string {
    if depth >= maxRefDepth || visited[path] {
        return nil
    }
    visited[path] = true

    absPath := filepath.Join(repoRoot, path)
    data, err := readFileLimited(absPath, limitMarkdown)
    if err != nil {
        return nil
    }

    dir := filepath.Dir(path)
    var refs []string

    // Check known subdirs (for skills/agents with supporting content)
    itemDir := filepath.Dir(absPath) // parent directory of the primary file
    for _, sub := range knownSubdirs {
        subDir := filepath.Join(itemDir, sub)
        if info, err := os.Stat(subDir); err == nil && info.IsDir() {
            // Add all files in the subdir
            subRel, _ := filepath.Rel(repoRoot, subDir)
            filepath.WalkDir(subDir, func(p string, d os.DirEntry, e error) error {
                if e != nil || d.IsDir() {
                    return nil
                }
                rel, _ := filepath.Rel(repoRoot, p)
                refs = append(refs, rel)
                return nil
            })
            _ = subRel
        }
    }

    // Parse markdown links
    for _, match := range mdLinkRe.FindAllStringSubmatch(string(data), -1) {
        linkPath := match[2]
        if strings.HasPrefix(linkPath, "http://") || strings.HasPrefix(linkPath, "https://") {
            continue
        }
        resolved := filepath.Clean(filepath.Join(dir, linkPath))
        absResolved := filepath.Join(repoRoot, resolved)
        if !strings.HasPrefix(absResolved, repoRoot) {
            continue // boundary check
        }
        if _, err := os.Stat(absResolved); err == nil {
            refs = append(refs, resolved)
        }
    }

    // Parse backtick paths
    for _, match := range backtickPathRe.FindAllStringSubmatch(string(data), -1) {
        candidate := match[1]
        ext := filepath.Ext(candidate)
        if !knownContentExts[strings.ToLower(ext)] {
            continue
        }
        if strings.ContainsAny(candidate, " \t\"'<>|") {
            continue
        }
        resolved := filepath.Clean(filepath.Join(dir, candidate))
        absResolved := filepath.Join(repoRoot, resolved)
        if !strings.HasPrefix(absResolved, repoRoot) {
            continue
        }
        if _, err := os.Stat(absResolved); err == nil {
            refs = append(refs, resolved)
        }
    }

    return uniqueStrings(refs)
}

func uniqueStrings(ss []string) []string {
    seen := make(map[string]bool, len(ss))
    var out []string
    for _, s := range ss {
        if !seen[s] {
            seen[s] = true
            out = append(out, s)
        }
    }
    return out
}
```

**What to write in `references_test.go`:** Tests:
- Markdown link `[foo](./helpers/foo.sh)` → `helpers/foo.sh` in refs (when file exists)
- Markdown link to non-existent file → not included
- Backtick path `` `scripts/lint.ts` `` → included when file exists
- Backtick path with unknown extension → not included
- Path traversal attempt `[foo](../../etc/passwd)` → not included (boundary check)
- Known subdir `references/` → all files included
- No duplicate paths in output
- depth cap: circular reference stops at `maxRefDepth`

**Test command:** `cd cli && go test ./internal/analyzer/ -run TestResolveReferences`

**Dependencies:** Task 8.1.

---

## Phase 10 — Manifest writer

### Task 10.1 — Convert DetectedItems to ManifestItems and write registry.yaml

**Files to create:**
- `cli/internal/analyzer/manifest.go`
- `cli/internal/analyzer/manifest_test.go`

**What to write in `manifest.go`:**

```go
package analyzer

import (
    "fmt"
    "os"
    "path/filepath"

    "github.com/OpenScribbler/syllago/cli/internal/registry"
    "gopkg.in/yaml.v3"
)

// ToManifestItem converts a DetectedItem to a registry.ManifestItem.
func ToManifestItem(item *DetectedItem) registry.ManifestItem {
    return registry.ManifestItem{
        Name:         item.Name,
        Type:         string(item.Type),
        Provider:     item.Provider,
        Path:         item.Path,
        HookEvent:    item.HookEvent,
        HookIndex:    item.HookIndex,
        Scripts:      item.Scripts,
        DisplayName:  item.DisplayName,
        Description:  item.Description,
        ContentHash:  item.ContentHash,
        References:   item.References,
        ConfigSource: item.ConfigSource,
        Providers:    item.Providers,
    }
}

// WriteGeneratedManifest writes a registry.yaml to the syllago cache directory
// for the named registry. Path: ~/.syllago/registries/<name>/manifest.yaml
// This file is separate from the repo's own registry.yaml.
func WriteGeneratedManifest(name string, items []*DetectedItem) error {
    cacheDir, err := registry.CacheDir()
    if err != nil {
        return fmt.Errorf("getting cache dir: %w", err)
    }

    destDir := filepath.Join(cacheDir, name)
    if err := os.MkdirAll(destDir, 0755); err != nil {
        return fmt.Errorf("creating manifest dir: %w", err)
    }

    manifestItems := make([]registry.ManifestItem, 0, len(items))
    for _, item := range items {
        manifestItems = append(manifestItems, ToManifestItem(item))
    }

    m := registry.Manifest{
        Version: "1",
        Items:   manifestItems,
    }

    data, err := yaml.Marshal(&m)
    if err != nil {
        return fmt.Errorf("marshaling manifest: %w", err)
    }

    // **Phase B correction:** Use registry.yaml (not manifest.yaml) so the scanner's
    // loadManifestItems function finds it without any changes.
    dest := filepath.Join(destDir, "registry.yaml")
    if err := os.WriteFile(dest, data, 0644); err != nil {
        return fmt.Errorf("writing manifest: %w", err)
    }
    return nil
}
```

**Phase B correction:** The filename MUST be `registry.yaml` (already corrected in the code block above). Do NOT use `manifest.yaml`. The scanner's `loadManifestItems` reads `registry.yaml` at the registry root — using a different filename would require scanner changes that are out of scope for this task.

**What to write in `manifest_test.go`:**
- `TestToManifestItem`: all fields map correctly; hook fields preserved
- `TestWriteGeneratedManifest`: writes valid YAML to the correct path; round-trip parse succeeds; uses `registry.CacheDirOverride` for temp dir

**Test command:** `cd cli && go test ./internal/analyzer/ -run TestManifest`

**Dependencies:** Task 9.1, Task 1.1 (ManifestItem schema).

---

## Phase 11 — Re-analysis engine

### Task 11.1 — SHA-256 hash comparison for sync

**Files to create:**
- `cli/internal/analyzer/reanalysis.go`
- `cli/internal/analyzer/reanalysis_test.go`

**What to write in `reanalysis.go`:**

```go
package analyzer

import (
    "os"
    "path/filepath"

    "github.com/OpenScribbler/syllago/cli/internal/registry"
)

// ReanalysisResult holds the outcome of a sync hash comparison.
type ReanalysisResult struct {
    Unchanged []*registry.ManifestItem // path exists, hash unchanged
    Changed   []string                 // relative paths where hash changed
    Missing   []string                 // relative paths that no longer exist
    Warnings  []string
}

// DiffManifest compares the current filesystem state against an existing manifest.
// For each item in the manifest, it reads the file at item.Path (relative to repoRoot)
// and compares its SHA-256 against item.ContentHash.
// Items with empty ContentHash are treated as Unchanged (authored manifests without hashes).
func DiffManifest(manifest *registry.Manifest, repoRoot string) ReanalysisResult {
    var result ReanalysisResult
    if manifest == nil {
        return result
    }
    for i := range manifest.Items {
        item := &manifest.Items[i]
        if item.ContentHash == "" {
            result.Unchanged = append(result.Unchanged, item)
            continue
        }
        absPath := filepath.Join(repoRoot, item.Path)
        data, err := os.ReadFile(absPath)
        if err != nil {
            if os.IsNotExist(err) {
                result.Missing = append(result.Missing, item.Path)
            } else {
                result.Warnings = append(result.Warnings, "reading "+item.Path+": "+err.Error())
            }
            continue
        }
        currentHash := hashBytes(data)
        if currentHash == item.ContentHash {
            result.Unchanged = append(result.Unchanged, item)
        } else {
            result.Changed = append(result.Changed, item.Path)
        }
    }
    return result
}

// PreserveUserMetadata merges user-edited display metadata from an existing manifest item
// into a newly-analyzed DetectedItem. Only non-empty user values are preserved.
// If both path AND hash changed (treated as new item), no preservation occurs.
func PreserveUserMetadata(existing *registry.ManifestItem, detected *DetectedItem) {
    if existing.DisplayName != "" && detected.DisplayName == "" {
        detected.DisplayName = existing.DisplayName
    }
    if existing.Description != "" && detected.Description == "" {
        detected.Description = existing.Description
    }
}
```

**What to write in `reanalysis_test.go`:**
- Unchanged item → in Unchanged list
- Changed item → path in Changed list
- Missing item → path in Missing list
- Item with empty ContentHash → treated as Unchanged
- `PreserveUserMetadata`: user DisplayName preserved when detected has none
- `PreserveUserMetadata`: user DisplayName NOT preserved when detected already has one
- `PreserveUserMetadata`: both fields independently preserved

**Test command:** `cd cli && go test ./internal/analyzer/ -run TestDiffManifest`

**Dependencies:** Task 10.1.

---

## Phase 12 — Analyzer engine (orchestrator)

### Task 12.1 — Main Analyze() function

**Files to create:**
- `cli/internal/analyzer/analyzer.go`
- `cli/internal/analyzer/analyzer_test.go`

**What to write in `analyzer.go`:**

```go
package analyzer

import (
    "fmt"
    "os"
    "path/filepath"
    "sort"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// Analyzer orchestrates content discovery for a repository.
type Analyzer struct {
    config    AnalysisConfig
    detectors []ContentDetector
}

// New creates an Analyzer with all built-in detectors registered.
func New(config AnalysisConfig) *Analyzer {
    return &Analyzer{
        config: config,
        detectors: []ContentDetector{
            &SyllagoDetector{},
            &ClaudeCodeDetector{},
            &ClaudeCodePluginDetector{},
            &CursorDetector{},
            &CopilotDetector{},
            &WindsurfDetector{},
            &ClineDetector{},
            &RooCodeDetector{},
            &CodexDetector{},
            &GeminiDetector{},
            &TopLevelDetector{},
        },
    }
}

// Analyze examines repoDir and returns classified content items.
// repoDir is resolved via filepath.EvalSymlinks before analysis.
func (a *Analyzer) Analyze(repoDir string) (*AnalysisResult, error) {
    // Step 0: Resolve repoRoot.
    repoRoot, err := filepath.EvalSymlinks(repoDir)
    if err != nil {
        return nil, fmt.Errorf("resolving repo root: %w", err)
    }
    if err := validateRepoRoot(repoRoot); err != nil {
        return nil, err
    }

    result := &AnalysisResult{}

    // Step 1: Walk filesystem.
    walkResult := Walk(repoRoot, a.config.ExcludeDirs)
    result.Warnings = append(result.Warnings, walkResult.Warnings...)

    // Step 2: Pattern matching.
    candidates := MatchPatterns(walkResult.Paths, a.detectors)

    // Step 3: Classify candidates.
    var allItems []*DetectedItem
    totalBytes := int64(0)
    const maxTotalBytes = 50 * 1024 * 1024 // 50 MB per-repo limit

    for _, c := range candidates {
        if totalBytes > maxTotalBytes {
            result.Warnings = append(result.Warnings, "per-repo read limit reached; some items may not be classified")
            break
        }
        items, classifyErr := c.Detector.Classify(c.Path, repoRoot)
        if classifyErr != nil {
            result.Warnings = append(result.Warnings, fmt.Sprintf("classify %s: %s", c.Path, classifyErr))
            continue
        }
        // A4: Increment totalBytes after each Classify call so the per-repo 50 MB
        // limit actually fires. Read the file size from disk (stat is cheap; avoids
        // re-reading the file content we already read in Classify).
        if info, statErr := os.Stat(filepath.Join(repoRoot, c.Path)); statErr == nil {
            totalBytes += info.Size()
        }
        // Resolve references for skills and agents.
        for _, item := range items {
            if item == nil {
                continue
            }
            if item.Type == catalog.Skills || item.Type == catalog.Agents {
                item.References = ResolveReferences(item.Path, repoRoot)
            }
            allItems = append(allItems, item)
        }
    }

    // Step 4: Dedup + conflict resolution.
    deduped, _ := DeduplicateItems(allItems)
    // Conflicts are surfaced in the wizard UX (future work); for now, keep first.

    // Step 5: Partition by confidence.
    for _, item := range deduped {
        // Executable content always requires confirmation regardless of confidence.
        if item.Type == catalog.Hooks || item.Type == catalog.MCP {
            result.Confirm = append(result.Confirm, item)
            continue
        }
        if item.Confidence > a.config.AutoThreshold {
            result.Auto = append(result.Auto, item)
        } else if item.Confidence >= a.config.SkipThreshold {
            result.Confirm = append(result.Confirm, item)
        }
        // Below skip threshold: drop silently (verbose logging TODO).
    }

    // Sort for deterministic output.
    sort.Slice(result.Auto, func(i, j int) bool {
        return result.Auto[i].Name < result.Auto[j].Name
    })
    sort.Slice(result.Confirm, func(i, j int) bool {
        return result.Confirm[i].Name < result.Confirm[j].Name
    })

    return result, nil
}

// validateRepoRoot rejects paths that resolve to sensitive system roots.
func validateRepoRoot(resolved string) error {
    dangerous := []string{"/", "/etc", "/home", "/usr", "/var", "/sys", "/proc"}
    for _, d := range dangerous {
        if resolved == d {
            return fmt.Errorf("repo root %q resolves to a sensitive system path; refusing analysis", resolved)
        }
    }
    return nil
}
```

**What to write in `analyzer_test.go`:**

```go
func TestAnalyzer_EmptyDir(t *testing.T)          // empty temp dir → 0 items, no error
func TestAnalyzer_SyllagoCanonical(t *testing.T)  // skills/my-skill/SKILL.md → Auto list
func TestAnalyzer_HooksAlwaysConfirm(t *testing.T) // hook with 0.95 confidence → Confirm, not Auto
func TestAnalyzer_MCPAlwaysConfirm(t *testing.T)  // .mcp.json → Confirm
func TestAnalyzer_DangerousRoot(t *testing.T)     // "/" as repo root → error
func TestAnalyzer_ExcludeDirs(t *testing.T)       // content in node_modules/ → not found
func TestAnalyzer_MultiProvider(t *testing.T)     // skill in both skills/ and .claude/skills/ → one deduped item
```

For `TestAnalyzer_MultiProvider`: create a temp dir with `skills/my-skill/SKILL.md` (content hash A) and `.claude/skills/my-skill/SKILL.md` (same content → same hash A). Verify only one item returned with `len(result.Auto) == 1`.

**Test command:** `cd cli && go test ./internal/analyzer/ -run TestAnalyzer`

**Dependencies:** All of Phases 3–11.

---

## Phase 13 — Scanner simplification

### Task 13.1 — Manifest-first scanner path

The current `scanRoot` already checks for `registry.yaml` and falls back to directory walking. The design says the directory-walk fallback is deprecated (Decision #12). No code deletion yet — just ensure `scanFromIndex` handles the new ManifestItem fields introduced in Task 1.1.

**Files modified:**
- `cli/internal/catalog/scanner.go`

**B5 RESOLVED — confirmed import cycle prevents `catalog` from importing `registry`:** `scanner.go` has a comment at line 136: "registry package already imports catalog (for `IsValidRegistryName`), so catalog cannot import registry." This is verified in `registry.go` line 12: `import "github.com/OpenScribbler/syllago/cli/internal/catalog"`. The local `manifestItem` struct must remain as a duplicate. Add a `// keep in sync with registry.ManifestItem` comment to make the sync obligation explicit.

**What to change:** In `scanFromIndex`, extract `DisplayName` and `Description` from the `manifestItem` fields when they are populated, rather than always reading from disk. Update the local `manifestItem` struct mirror in `scanner.go` to include the new fields:

```go
// manifestItem mirrors registry.ManifestItem for use within the catalog
// package. It is intentionally unexported to avoid an import cycle: the
// registry package already imports catalog (for IsValidRegistryName), so
// catalog cannot import registry.
// KEEP IN SYNC with registry.ManifestItem in cli/internal/registry/registry.go.
type manifestItem struct {
    Name         string   `yaml:"name"`
    Type         string   `yaml:"type"`
    Provider     string   `yaml:"provider,omitempty"`
    Path         string   `yaml:"path"`
    HookEvent    string   `yaml:"hookEvent,omitempty"`
    HookIndex    int      `yaml:"hookIndex,omitempty"`
    Scripts      []string `yaml:"scripts,omitempty"`
    DisplayName  string   `yaml:"displayName,omitempty"`
    Description  string   `yaml:"description,omitempty"`
    ContentHash  string   `yaml:"contentHash,omitempty"`
    References   []string `yaml:"references,omitempty"`
    ConfigSource string   `yaml:"configSource,omitempty"`
    Providers    []string `yaml:"providers,omitempty"`
}
```

In `scanFromIndex`, after creating the `ContentItem`, apply `DisplayName` and `Description` from the manifest item if non-empty:

```go
if mi.DisplayName != "" {
    item.DisplayName = mi.DisplayName
}
if mi.Description != "" {
    item.Description = mi.Description
}
```

This short-circuits the disk reads for items where the manifest already carries the metadata.

**Decision #41 — Empty registry.yaml (zero items is valid):** In `scanRoot` (or `loadManifestItems`), when a `registry.yaml` exists and parses successfully but contains zero items, return an empty catalog — do not fall through to directory-walk. An empty manifest is an explicit author choice, not a stub.

**Phase B blocker:** The variable `registryYAMLExists` is not provided by the current `loadManifestItems` function — it returns `([]manifestItem, error)` and cannot distinguish "file not found" from "file exists but empty" in the caller. Fix: refactor `loadManifestItems` to return `([]manifestItem, bool, error)` where the bool indicates whether the file was found. OR add a separate `os.Stat` check before calling `loadManifestItems`. The refactor approach is cleaner. Update the signature and its single call site in `scanRoot`.

Add a guard using the existence bool:

```go
// Decision #41: empty registry.yaml is valid and means "no items here."
// Do NOT fall back to directory scanning if a manifest file exists.
manifestItems, registryYAMLExists, err := loadManifestItems(baseDir)
// ... (handle err)
if registryYAMLExists && len(manifestItems) == 0 {
    return nil // zero items, no error — explicit empty manifest
}
```

**Decision #42 — Nested registry.yaml ignored:** The scanner reads only the root-level `registry.yaml`. If subdirectories contain their own `registry.yaml` files, they are not parsed as nested registries. No code change needed since the scanner already only reads the root manifest path, but add a comment at the manifest-file lookup site: `// Only the root-level registry.yaml is read; nested manifests in subdirectories are intentionally ignored (Decision #42).`

**Test file:** `cli/internal/catalog/scanner_test.go` (existing)

Add test cases:
- `registry.yaml` with zero items → zero `ContentItem` results, no error, no fallback to directory walk
- `registry.yaml` with `displayName` and `description` fields populated → fields flow through to `ContentItem` without requiring the item's primary file to exist

**Test command:** `cd cli && go test ./internal/catalog/ -run TestScanFromIndex`

**Dependencies:** Task 1.1.

---

## Phase 14 — `syllago manifest generate` CLI command

### Task 14.1 — `manifest generate` command skeleton and wire-up

> **B1 RESOLVED — Name conflict with existing `init.go`:** `cli/cmd/syllago/init.go` already exists and registers `initCmd` (Initialize syllago for this project — provider detection, `.syllago/config.json`). The new command must NOT reuse the `init` name or overwrite that file. The command is named `syllago manifest generate` instead. It lives in a new file `generate_manifest.go` and is nested under a `manifest` subcommand group.
>
> **Also note (A5):** Add a note in `registry_cmd.go`'s `scanRoot` fallback path to call `Analyze()` as the content discovery path when no `registry.yaml` exists, replacing the call to `ScanNativeContent`. This wires the analyzer into the registry-add path, not just `syllago manifest generate`. Without this, new registries added after this plan ships will fall back to directory-walk discovery since `WriteGeneratedManifest()` is only called from `syllago manifest generate`. Add a TODO comment at minimum.

**Files to create:**
- `cli/cmd/syllago/generate_manifest.go`

**Files modified:**
- `cli/cmd/syllago/root.go` (add `manifestCmd` group and `generateManifestCmd` to root)

**Phase B note:** The `init()` function in `generate_manifest.go` already calls `rootCmd.AddCommand(manifestCmd)`. The `root.go` modification listed above may be redundant. Verify by checking whether `root.go` uses an explicit `AddCommand` list vs. relying on `init()` auto-registration. If `root.go` does NOT have an explicit `AddCommand` list, no change to `root.go` is needed.

**What to write in `generate_manifest.go`:**

```go
package main

import (
    "fmt"
    "os"
    "path/filepath"

    "github.com/OpenScribbler/syllago/cli/internal/analyzer"
    "github.com/OpenScribbler/syllago/cli/internal/registry"
    "github.com/spf13/cobra"
    "gopkg.in/yaml.v3"
)

// manifestCmd is the parent command group for manifest operations.
var manifestCmd = &cobra.Command{
    Use:   "manifest",
    Short: "Manage registry manifests",
}

var generateManifestForce bool

var generateManifestCmd = &cobra.Command{
    Use:   "generate [dir]",
    Short: "Generate a registry.yaml for this repository",
    Long: `Analyze the current repository and generate a registry.yaml manifest.
Run with --force to overwrite an existing registry.yaml. When overwriting,
a diff of added/removed/changed items is displayed before writing.`,
    Args: cobra.MaximumNArgs(1),
    RunE: runGenerateManifest,
}

func init() {
    generateManifestCmd.Flags().BoolVar(&generateManifestForce, "force", false, "overwrite existing registry.yaml")
    manifestCmd.AddCommand(generateManifestCmd)
    rootCmd.AddCommand(manifestCmd)
}

func runGenerateManifest(cmd *cobra.Command, args []string) error {
    absDir, err := os.Getwd()
    if err != nil {
        return fmt.Errorf("getting working directory: %w", err)
    }
    if len(args) > 0 {
        absDir = args[0]
    }

    destFile := filepath.Join(absDir, "registry.yaml")

    // If an existing registry.yaml is present and --force is set, show a diff first.
    if _, statErr := os.Stat(destFile); statErr == nil {
        if !generateManifestForce {
            return fmt.Errorf("registry.yaml already exists; use --force to overwrite")
        }
        // Show diff of what will change (design Decision #23).
        if existing, loadErr := registry.LoadManifestFromDir(absDir); loadErr == nil && existing != nil {
            printManifestDiff(cmd, existing, nil) // nil new items — computed after analysis below
        }
    }

    fmt.Fprintf(cmd.ErrOrStderr(), "Analyzing %s...\n", absDir)

    cfg := analyzer.DefaultConfig()
    a := analyzer.New(cfg)
    result, err := a.Analyze(absDir)
    if err != nil {
        return fmt.Errorf("analyzing repository: %w", err)
    }

    allItems := result.AllItems()
    if len(allItems) == 0 {
        return fmt.Errorf("no AI content detected; try adding content or creating a registry.yaml manually")
    }

    // If overwriting, show the diff now that we have new items.
    if _, statErr := os.Stat(destFile); statErr == nil && generateManifestForce {
        if existing, loadErr := registry.LoadManifestFromDir(absDir); loadErr == nil && existing != nil {
            printManifestDiff(cmd, existing, allItems)
        }
    }

    manifestItems := make([]registry.ManifestItem, 0, len(allItems))
    for _, item := range allItems {
        manifestItems = append(manifestItems, analyzer.ToManifestItem(item))
    }

    m := registry.Manifest{
        Version: "1",
        Items:   manifestItems,
    }

    // Use yaml.Marshal (never templates) for injection safety.
    data, err := yaml.Marshal(&m)
    if err != nil {
        return fmt.Errorf("serializing manifest: %w", err)
    }

    header := []byte("# Generated by syllago manifest generate -- edit freely, re-run with --force to regenerate\n")
    data = append(header, data...)

    if err := os.WriteFile(destFile, data, 0644); err != nil {
        return fmt.Errorf("writing registry.yaml: %w", err)
    }

    fmt.Fprintf(cmd.OutOrStdout(), "Wrote registry.yaml with %d items (%d auto-detected, %d requiring confirmation)\n",
        len(allItems), len(result.Auto), len(result.Confirm))

    for _, w := range result.Warnings {
        fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", w)
    }
    return nil
}

// printManifestDiff prints a human-readable diff of manifest items.
// existingManifest is the current file content; newItems is nil if analysis has not run yet.
func printManifestDiff(cmd *cobra.Command, existing *registry.Manifest, newItems []*analyzer.DetectedItem) {
    if newItems == nil {
        fmt.Fprintf(cmd.OutOrStdout(), "Existing registry.yaml has %d items — will be overwritten.\n", len(existing.Items))
        return
    }
    existingNames := make(map[string]bool, len(existing.Items))
    for _, mi := range existing.Items {
        existingNames[mi.Type+"/"+mi.Name] = true
    }
    newNames := make(map[string]bool, len(newItems))
    for _, di := range newItems {
        newNames[string(di.Type)+"/"+di.Name] = true
    }
    var added, removed int
    for k := range newNames {
        if !existingNames[k] {
            added++
        }
    }
    for k := range existingNames {
        if !newNames[k] {
            removed++
        }
    }
    fmt.Fprintf(cmd.OutOrStdout(), "Diff: +%d added, -%d removed (from %d existing items)\n",
        added, removed, len(existing.Items))
}
```

**Test file:** `cli/cmd/syllago/generate_manifest_test.go`

```go
func TestGenerateManifestCmd_NoContent(t *testing.T)          // empty dir → error about no content found
func TestGenerateManifestCmd_WritesFile(t *testing.T)         // dir with SKILL.md → registry.yaml written
func TestGenerateManifestCmd_ExistingNoForce(t *testing.T)    // existing registry.yaml without --force → error
func TestGenerateManifestCmd_ExistingWithForce(t *testing.T)  // existing registry.yaml with --force → overwritten
func TestGenerateManifestCmd_HeaderComment(t *testing.T)      // generated file has header comment
func TestGenerateManifestCmd_DiffOnForce(t *testing.T)        // --force with existing file → diff printed to stdout
```

Use `t.TempDir()`, create fixture content, call `generateManifestCmd.RunE(generateManifestCmd, []string{tempDir})`.

**Test command:** `cd cli && go test ./cmd/syllago/ -run TestGenerateManifestCmd`

**Dependencies:** Task 12.1.

---

### Task 14.2 — Wire analyzer into registry-add fallback path

The analyzer is only called from `syllago manifest generate` (Task 14.1). Without this task, new registries added via `syllago registry add` (or TUI add-wizard) continue falling back to directory-walk discovery. This task wires the analyzer into `registry_cmd.go` so all registry-add paths benefit from the new detection logic.

**Files modified:**
- `cli/cmd/syllago/registry_cmd.go` (or whichever file contains `scanRoot` / `ScanNativeContent` call)

**What to change:**

In the `scanRoot` function (or equivalent fallback path that runs when no `registry.yaml` is found), replace the call to `ScanNativeContent` with a call to `analyzer.Analyze()` followed by `analyzer.WriteGeneratedManifest()`. The generated manifest is then read by the scanner on the next pass.

```go
// Decision #3: When no registry.yaml exists, run the content analyzer to
// generate one. The generated manifest is written to the syllago cache
// (~/.syllago/registries/<name>/registry.yaml) and read by the scanner.
cfg := analyzer.DefaultConfig()
a := analyzer.New(cfg)
result, err := a.Analyze(repoRoot)
if err != nil {
    return fmt.Errorf("analyzing registry content: %w", err)
}
if err := analyzer.WriteGeneratedManifest(registryName, result.AllItems()); err != nil {
    return fmt.Errorf("writing generated manifest: %w", err)
}
// Proceed: scanner will now find the generated manifest.
```

Preserve any existing `ScanNativeContent` call under a `// DEPRECATED` comment rather than deleting it — the full deprecation path (Decision #12) is tracked in deferred work.

**Phase B missing context:** The existing code at lines 122–141 of `registry_cmd.go` uses `ScanNativeContent` to detect non-syllago registries and reject them with a helpful error. This gate logic must be preserved. When replacing `ScanNativeContent` with `analyzer.Analyze()`:
- If `analyzer.Analyze()` returns 0 items AND the repo has no `registry.yaml`, emit the same rejection warning and remove the clone.
- If `analyzer.Analyze()` returns 0 items AND the repo has a `registry.yaml` (even empty), treat it as a valid syllago registry (empty is valid per Decision #41).
- The `WriteGeneratedManifest()` call should only fire when items were found.
The code snippet in this task is a skeleton — implement the full gate logic, not just the analyzer call.

**Test file:** `cli/cmd/syllago/registry_cmd_test.go` (existing or create)

Add an integration test:
- `TestRegistryAddUsesAnalyzer`: create a temp git repo with `skills/my-skill/SKILL.md`, call the registry-add path, verify the generated manifest appears in the cache dir and contains the skill item.

**Test command:** `cd cli && go test ./cmd/syllago/ -run TestRegistryAddUsesAnalyzer`

**Dependencies:** Task 12.1, Task 10.1 (WriteGeneratedManifest).

---

## Phase 15 — Registry cross-validation

### Task 15.1 — Lightweight type verification for authored registry.yaml

**Files to create:**
- `cli/internal/analyzer/validate.go`
- `cli/internal/analyzer/validate_test.go`

**What to write in `validate.go`:**

```go
package analyzer

import (
    "fmt"
    "os"
    "path/filepath"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
    "github.com/OpenScribbler/syllago/cli/internal/registry"
)

// ValidationIssue describes a disagreement between registry.yaml and file content.
type ValidationIssue struct {
    ItemName      string
    DeclaredType  string
    DetectedType  catalog.ContentType
    Path          string
    Severity      string // "warning", "error"
    Message       string
}

// ValidateManifest cross-checks an authored registry.yaml against actual file content.
// Each declared item is verified:
// 1. Path resolves to a real file within repoRoot
// 2. Content type is plausible for the file (lightweight check, not full analysis)
//
// Note: This function bypasses the exclusion list intentionally — declared paths
// must be verified even if they are in excluded directories.
func ValidateManifest(m *registry.Manifest, repoRoot string) []ValidationIssue {
    if m == nil {
        return nil
    }
    var issues []ValidationIssue
    for _, item := range m.Items {
        absPath := filepath.Join(repoRoot, item.Path)

        // Boundary check
        resolved, err := filepath.EvalSymlinks(absPath)
        if err != nil || !isWithinRoot(resolved, repoRoot) {
            issues = append(issues, ValidationIssue{
                ItemName: item.Name,
                Path:     item.Path,
                Severity: "error",
                Message:  fmt.Sprintf("path %q does not resolve within repository boundary", item.Path),
            })
            continue
        }

        // Existence check
        if _, err := os.Stat(absPath); err != nil {
            issues = append(issues, ValidationIssue{
                ItemName: item.Name,
                Path:     item.Path,
                Severity: "error",
                Message:  fmt.Sprintf("path %q not found", item.Path),
            })
            continue
        }

        // Type plausibility check
        if issue := checkTypePlausibility(item, absPath); issue != nil {
            issues = append(issues, *issue)
        }
    }
    return issues
}

// checkTypePlausibility performs a lightweight check that the file content is
// consistent with the declared content type. Not a full detector run.
func checkTypePlausibility(item registry.ManifestItem, absPath string) *ValidationIssue {
    ext := filepath.Ext(absPath)
    declaredType := catalog.ContentType(item.Type)

    switch declaredType {
    case catalog.Hooks:
        // Hook files should be JSON or a script extension
        validExts := map[string]bool{".json": true, ".ts": true, ".js": true, ".py": true, ".sh": true}
        if !validExts[ext] {
            return &ValidationIssue{
                ItemName:     item.Name,
                DeclaredType: item.Type,
                Path:         item.Path,
                Severity:     "warning",
                Message:      fmt.Sprintf("declared as hook but file extension %q is unusual for hooks", ext),
            }
        }
    case catalog.MCP:
        if ext != ".json" {
            return &ValidationIssue{
                ItemName:     item.Name,
                DeclaredType: item.Type,
                Path:         item.Path,
                Severity:     "warning",
                Message:      fmt.Sprintf("declared as MCP config but file extension is %q (expected .json)", ext),
            }
        }
    case catalog.Skills, catalog.Agents, catalog.Rules, catalog.Commands:
        if ext != ".md" && ext != ".mdc" {
            return &ValidationIssue{
                ItemName:     item.Name,
                DeclaredType: item.Type,
                Path:         item.Path,
                Severity:     "warning",
                Message:      fmt.Sprintf("declared as %s but file extension is %q (expected .md)", item.Type, ext),
            }
        }
    }
    return nil
}

func isWithinRoot(resolved, root string) bool {
    return resolved == root || len(resolved) > len(root) &&
        resolved[len(root)] == filepath.Separator &&
        resolved[:len(root)] == root
}
```

**What to write in `validate_test.go`:**
- Valid manifest item → no issues
- Path outside repo boundary → error issue
- Missing path → error issue
- Hook declared but `.md` file → warning issue
- Rule declared, `.md` file → no issue
- MCP declared, `.yaml` file → warning issue
- Nil manifest → no issues, no panic

**Additional tests for `isWithinRoot` (A8):** The function has a subtle panic risk if called with a resolved path shorter than root (impossible after EvalSymlinks, but worth proving). Add explicit unit tests:
```go
func TestIsWithinRoot(t *testing.T) {
    root := "/tmp/repo"
    tests := []struct {
        resolved string
        want     bool
    }{
        {"/tmp/repo", true},                    // exactly the root
        {"/tmp/repo/subdir/file.md", true},     // nested inside root
        {"/tmp/repomalicious", false},           // same prefix but different dir (sibling)
        {"/tmp/other", false},                  // unrelated path
        {"/tmp", false},                        // parent of root
        {"/tmp/repo_evil", false},              // starts with root string but different dir
    }
    for _, tt := range tests {
        got := isWithinRoot(tt.resolved, root)
        if got != tt.want {
            t.Errorf("isWithinRoot(%q, %q) = %v, want %v", tt.resolved, root, got, tt.want)
        }
    }
}
```

**Test command:** `cd cli && go test ./internal/analyzer/ -run TestValidateManifest`

**Dependencies:** Task 12.1.

---

## Phase 16 — Confidence partitioning and AnalysisResult helpers

### Task 16.1 — AnalysisResult summary helpers and AllItems()

> **B8 RESOLVED — dependency corrected:** `AnalysisResult` is defined in Task 2.1 (`types.go`). The helpers add methods on that type and require no Phase 12 code. The dependency is **Task 2.1**, not Task 12.1. This task can be done at any point after Task 2.1 — it does not need to wait for the full analyzer engine. Move it immediately after Phase 2 in your execution order if desired, or leave it here as a finishing step. The table at the end of this plan reflects the corrected dependency.

**Files modified:**
- `cli/internal/analyzer/types.go`

**What to add:**

```go
// AllItems returns Auto and Confirm combined, Auto first.
func (r *AnalysisResult) AllItems() []*DetectedItem {
    all := make([]*DetectedItem, 0, len(r.Auto)+len(r.Confirm))
    all = append(all, r.Auto...)
    all = append(all, r.Confirm...)
    return all
}

// CountByType returns a map of ContentType to item count across Auto+Confirm.
func (r *AnalysisResult) CountByType() map[catalog.ContentType]int {
    counts := make(map[catalog.ContentType]int)
    for _, item := range r.AllItems() {
        counts[item.Type]++
    }
    return counts
}

// IsEmpty returns true if both Auto and Confirm are empty.
func (r *AnalysisResult) IsEmpty() bool {
    return len(r.Auto) == 0 && len(r.Confirm) == 0
}
```

**Test file:** `cli/internal/analyzer/types_test.go` (extend existing)

Add tests for `AllItems()`, `CountByType()`, `IsEmpty()`.

**Test command:** `cd cli && go test ./internal/analyzer/ -run TestAnalysisResult`

**Dependencies:** Task 2.1 (where `AnalysisResult` is defined). Can be executed any time after Phase 2.

---

## Phase 17 — Build verification

### Task 17.1 — Build and full test suite

**What to run:**

```bash
cd /home/hhewett/.local/src/syllago/cli && make fmt && make build && make test
```

Fix any compile errors (likely import path issues, missing type fields, or interface mismatches). Fix any test failures.

If golden file tests fail due to unrelated TUI changes, regenerate:
```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/tui/ -update-golden
```

**Checklist:**
- [ ] `make fmt` exits 0
- [ ] `make build` exits 0
- [ ] `make test` exits 0 (all packages)
- [ ] `cd cli && go test ./internal/analyzer/...` exits 0
- [ ] Coverage check: `cd cli && go test ./internal/analyzer/ -coverprofile=cov.out && go tool cover -func=cov.out | grep total` — should show >= 80%

**Dependencies:** All prior phases.

---

## Task Ordering Summary

| Phase | Tasks | Builds on |
|-------|-------|-----------|
| 1 | 1.1 | None |
| 2 | 2.1, 2.2, 2.3 | 1.1 |
| 3 | 3.1 | 2.3 |
| 4 | 4.1, 4.2 | 3.1 |
| 5 | 5.1 | 4.2 |
| 6 | 6.1 | 5.1 |
| 7 | 7.1, 7.2 | 6.1 |
| 8 | 8.1 | 7.2 |
| 9 | 9.1 | 8.1 |
| 10 | 10.1 | 9.1, 1.1 |
| 11 | 11.1 | 10.1 |
| **16** | **16.1** | **2.1 — MUST run before Phase 12** (`analyzer.go` calls `AllItems()` which is defined here) |
| 12 | 12.1 | 11.1 (all detectors), **16.1** |
| 13 | 13.1 | 1.1 (independent of phases 2-12, but MUST be serialized with Task 7.1 sub-step on scanner.go) |
| 14 | 14.1, 14.2 | 12.1, 10.1 |
| 15 | 15.1 | 12.1 |
| 17 | 17.1 | All |

**Phase B correction:** Phase 16 (Task 16.1) has been moved before Phase 12 in the execution order. `analyzer.go` (Task 12.1) calls `result.AllItems()` which is defined in Task 16.1. If 12 runs before 16, the package will not compile.

Phase 13 (scanner update) can be done in parallel with Phases 3–12 since it only depends on Task 1.1, EXCEPT that Task 7.1 has a sub-step modifying `scanner.go` (exporting `MCPServerDescription`). Task 7.1's sub-step and Task 13.1 must be serialized on `scanner.go`.

---

## Notes on Deferred Work

The following items from the design are noted here but deliberately not in scope for this plan:

- **Wizard UX integration** (checkboxList confirmation step for auto-detect results) — depends on the TUI add-wizard which has its own phase plan
- **Guided and Manual wizard paths** — deferred to add-wizard redesign
- **`syllago registry rescan` command** — calls `Analyze()` followed by `WriteGeneratedManifest()` and is a thin wrapper; implement as a follow-on task after this plan is complete
- **Symlink policy wizard prompt** — policy is respected in `walk.go` (skip), but the interactive prompt in the add-wizard is deferred
- **`syllago scan --verbose`** — diagnostic flag for surfacing skipped items; add as a follow-on task
- **Hook script integrity checking on sync** — SHA-256 install-time hash vs. `installed.json` is a separate concern from analysis; deferred
- **`syllago init --preview`** — deferred per design review
- **Conflict resolution UI** — `DeduplicateItems` returns conflict pairs; surface in wizard as follow-on
- **`installed.json` re-keying (Decision #22)** — The design specifies changing keys in `installed.json` and user metadata storage from relative path to `(registry-url, type, item-name)`. This is a data migration concern separate from the analyzer implementation and must be planned and executed as a dedicated follow-on task. It requires updating `installed.json` write/read paths across the install, uninstall, and sync flows, plus a migration step for existing installs.
- **Too-many-items auto-switch to guided mode (Error #2)** — When the analyzer finds >100 items with low average confidence, the design specifies auto-switching to guided mode with the message "Found many potential matches but results are noisy." Since Guided wizard mode is itself deferred, this error path is deferred with it. For now, the `syllago manifest generate` command surfaces the full item count and warnings; users can re-run with `--guided` once that path exists.
- **Post-add partial detection hint (Error #12)** — "Installed N items. If you expected more, run `syllago scan --verbose` to see what wasn't detected." This hint belongs in the install flow, not the analyzer. Deferred to the install workflow integration task.
- **Decision #45 (Static content requirement)** — Repos with generated/templated content that requires a build step before content files exist will produce "0 items found." The `syllago manifest generate` zero-items error message already handles this case by telling users to add content or create a `registry.yaml` manually. No additional code needed; noting here for completeness. The `// Below skip threshold: drop silently (verbose logging TODO)` placeholder in `analyzer.go` is covered by the `syllago scan --verbose` deferred task above.
