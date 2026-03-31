# Add UX Redesign - Implementation Plan

**Goal:** Replace the flag-driven `syllago add` command with a positional-argument interface featuring discovery mode, hash-based conflict detection, and inform-and-skip behavior.
**Architecture:** A new `cli/internal/add` package provides `DiscoverFromProvider` and `AddItems` as shared functions (for future TUI reuse); `add_cmd.go` is rewritten to use positional args and delegate to that package; metadata gains a `SourceHash` field for file-based content. Tests are rewritten top-to-bottom in the new syntax before the implementation lands.
**Design Doc:** docs/plans/2026-03-05-add-ux-redesign-design.md

---

## Phase 1: Metadata Changes

### Task 1.1: Add `SourceHash` to `metadata.Meta` and clean up redundant fields

**Files:**
- Modify: `cli/internal/metadata/metadata.go`
- Modify: `cli/internal/metadata/metadata_test.go`

**Depends on:** nothing

**Steps:**

1. In `metadata.go`, add `SourceHash` between `HasSource` and `AddedAt`, and remove the `ImportedAt`/`ImportedBy` pair (they are the pre-rename leftover the design doc flags for cleanup). The updated `Meta` struct becomes:

```go
// Meta holds metadata for a single content item.
type Meta struct {
	ID             string       `yaml:"id"`
	Name           string       `yaml:"name"`
	Description    string       `yaml:"description,omitempty"`
	Version        string       `yaml:"version,omitempty"`
	Type           string       `yaml:"type,omitempty"`
	Author         string       `yaml:"author,omitempty"`
	Source         string       `yaml:"source,omitempty"`
	Tags           []string     `yaml:"tags,omitempty"`
	Hidden         bool         `yaml:"hidden,omitempty"`
	Dependencies   []Dependency `yaml:"dependencies,omitempty"`
	CreatedAt      *time.Time   `yaml:"created_at,omitempty"`
	PromotedAt     *time.Time   `yaml:"promoted_at,omitempty"`
	PRBranch       string       `yaml:"pr_branch,omitempty"`
	SourceProvider string       `yaml:"source_provider,omitempty"`
	SourceFormat   string       `yaml:"source_format,omitempty"`
	SourceType     string       `yaml:"source_type,omitempty"`
	SourceURL      string       `yaml:"source_url,omitempty"`
	SourceHash     string       `yaml:"source_hash,omitempty"` // SHA-256 of raw source bytes, e.g. "sha256:a1b2c3..."
	HasSource      bool         `yaml:"has_source,omitempty"`
	AddedAt        *time.Time   `yaml:"added_at,omitempty"`
	AddedBy        string       `yaml:"added_by,omitempty"`
}
```

2. Verify no other file in the codebase reads `ImportedAt` or `ImportedBy`:
```bash
cd /home/hhewett/.local/src/syllago/cli && grep -rn "ImportedAt\|ImportedBy" --include="*.go" .
```
If any callers exist outside `metadata_test.go`, update them to use `AddedAt`/`AddedBy` before proceeding.

3. In `metadata_test.go`, remove `TestMetaCreatedAt`'s assertion that `ImportedAt` should be nil (line 131-133) since the field no longer exists. Update `TestSaveAndLoad` to remove the `ImportedAt` and `ImportedBy` assignments (lines 37-38). Add a new test for `SourceHash` round-trip:

```go
func TestMetaSourceHash(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	m := &Meta{
		ID:         NewID(),
		Name:       "test-skill",
		SourceHash: "sha256:abc123",
	}
	if err := Save(dir, m); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.SourceHash != "sha256:abc123" {
		t.Errorf("SourceHash: got %q, want %q", loaded.SourceHash, "sha256:abc123")
	}
}
```

4. Build and test:
```bash
cd /home/hhewett/.local/src/syllago/cli && make test 2>&1 | grep -E "FAIL|ok|metadata"
```

**Success Criteria:**
- [ ] `metadata.Meta` has `SourceHash string` with `yaml:"source_hash,omitempty"`
- [ ] `ImportedAt` and `ImportedBy` fields are removed from the struct
- [ ] `TestMetaSourceHash` passes
- [ ] `make test` passes for the `metadata` package

---

## Phase 2: New `add` Package — Core Types and Library Index

### Task 2.1: Create `cli/internal/add/add.go` with types and library index builder

**Files:**
- Create: `cli/internal/add/add.go`
- Create: `cli/internal/add/add_test.go`

**Depends on:** Task 1.1

**Steps:**

1. Create directory:
```bash
mkdir -p /home/hhewett/.local/src/syllago/cli/internal/add
```

2. Create `cli/internal/add/add.go`:

```go
// Package add provides the core logic for "syllago add": discovery from a
// provider with library-status annotation, and writing items to the library.
// Both the CLI command and the TUI use these functions directly.
package add

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/parse"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// ItemStatus describes the relationship between a discovered provider item
// and the global library.
type ItemStatus int

const (
	// StatusNew means the item is not present in the library.
	StatusNew ItemStatus = iota
	// StatusInLibrary means the item exists and its source hash matches.
	// For hooks, it means the directory exists (no hash comparison).
	StatusInLibrary
	// StatusOutdated means the item exists but the provider's source has changed.
	// Hooks never have this status.
	StatusOutdated
)

// String returns the human-readable label used in discovery output.
func (s ItemStatus) String() string {
	switch s {
	case StatusInLibrary:
		return "in library"
	case StatusOutdated:
		return "in library, outdated"
	default:
		return "new"
	}
}

// DiscoveryItem is a discovered provider file annotated with its library status.
type DiscoveryItem struct {
	Name   string
	Type   catalog.ContentType
	Path   string     // absolute path to source file
	Status ItemStatus
}

// AddStatus tracks the outcome of a single write operation.
type AddStatus int

const (
	AddStatusAdded    AddStatus = iota // item was written (new)
	AddStatusUpdated                   // item was overwritten (--force on outdated)
	AddStatusUpToDate                  // hash matched, skipped
	AddStatusSkipped                   // source changed but --force not set
	AddStatusError                     // write failed
)

// AddResult holds the per-item result from AddItems.
type AddResult struct {
	Name   string
	Type   catalog.ContentType
	Status AddStatus
	Error  error
}

// AddOptions controls the behavior of AddItems.
type AddOptions struct {
	Force    bool
	DryRun   bool
	Provider string // provider slug, used for directory layout
}

// LibraryIndex is a pre-built map from "type/provider/name" (or "type/name"
// for universal types) to the loaded metadata for that item.
type LibraryIndex map[string]*metadata.Meta

// BuildLibraryIndex scans globalDir once and returns a map keyed by
// "type/provider/name" for provider-specific types, or "type/name" for
// universal types. Items without a .syllago.yaml get a nil value entry
// (they exist but have no metadata).
func BuildLibraryIndex(globalDir string) (LibraryIndex, error) {
	idx := make(LibraryIndex)

	for _, ct := range catalog.AllContentTypes() {
		typeDir := filepath.Join(globalDir, string(ct))
		entries, err := os.ReadDir(typeDir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", typeDir, err)
		}

		if ct.IsUniversal() {
			for _, e := range entries {
				if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
					continue
				}
				itemDir := filepath.Join(typeDir, e.Name())
				meta, err := metadata.Load(itemDir)
				if err != nil {
					return nil, fmt.Errorf("loading metadata for %s/%s: %w", ct, e.Name(), err)
				}
				key := string(ct) + "/" + e.Name()
				idx[key] = meta
			}
		} else {
			// Provider-specific: typeDir/<provider>/<name>/
			for _, provEntry := range entries {
				if !provEntry.IsDir() || strings.HasPrefix(provEntry.Name(), ".") {
					continue
				}
				provDir := filepath.Join(typeDir, provEntry.Name())
				items, err := os.ReadDir(provDir)
				if err != nil {
					return nil, fmt.Errorf("reading %s: %w", provDir, err)
				}
				for _, itemEntry := range items {
					if !itemEntry.IsDir() || strings.HasPrefix(itemEntry.Name(), ".") {
						continue
					}
					itemDir := filepath.Join(provDir, itemEntry.Name())
					meta, err := metadata.Load(itemDir)
					if err != nil {
						return nil, fmt.Errorf("loading metadata for %s/%s/%s: %w", ct, provEntry.Name(), itemEntry.Name(), err)
					}
					key := string(ct) + "/" + provEntry.Name() + "/" + itemEntry.Name()
					idx[key] = meta
				}
			}
		}
	}

	return idx, nil
}

// libraryKey returns the index key for a given type, provider slug, and item name.
func libraryKey(ct catalog.ContentType, provSlug, name string) string {
	if ct.IsUniversal() {
		return string(ct) + "/" + name
	}
	return string(ct) + "/" + provSlug + "/" + name
}

// itemName derives an item name from a file path (filename without extension).
func itemName(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

// sourceHash computes "sha256:<hex>" for the given raw content bytes.
func sourceHash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("sha256:%x", sum)
}

// DiscoverFromProvider discovers all content from prov, annotates each item
// with its library status against globalDir, and returns the list.
// For hooks, status is existence-based only.
func DiscoverFromProvider(prov provider.Provider, projectRoot string, resolver *config.PathResolver, globalDir string) ([]DiscoveryItem, error) {
	report := parse.DiscoverWithResolver(prov, projectRoot, resolver)

	idx, err := BuildLibraryIndex(globalDir)
	if err != nil {
		return nil, fmt.Errorf("building library index: %w", err)
	}

	var items []DiscoveryItem
	for _, f := range report.Files {
		name := itemName(f.Path)
		key := libraryKey(f.ContentType, prov.Slug, name)
		existing, inLib := idx[key]

		var status ItemStatus
		if !inLib {
			status = StatusNew
		} else if f.ContentType == catalog.Hooks {
			// Hooks: existence-based only.
			status = StatusInLibrary
		} else {
			raw, err := os.ReadFile(f.Path)
			if err != nil {
				// Can't read source — treat as new so the error surfaces on add.
				status = StatusNew
			} else if existing != nil && existing.SourceHash == sourceHash(raw) {
				status = StatusInLibrary
			} else {
				status = StatusOutdated
			}
		}

		items = append(items, DiscoveryItem{
			Name:   name,
			Type:   f.ContentType,
			Path:   f.Path,
			Status: status,
		})
	}

	return items, nil
}
```

3. Create `cli/internal/add/add_test.go`:

```go
package add

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

func TestBuildLibraryIndex_EmptyDir(t *testing.T) {
	t.Parallel()
	globalDir := t.TempDir()
	idx, err := BuildLibraryIndex(globalDir)
	if err != nil {
		t.Fatalf("BuildLibraryIndex on empty dir: %v", err)
	}
	if len(idx) != 0 {
		t.Errorf("expected empty index, got %d entries", len(idx))
	}
}

func TestBuildLibraryIndex_UniversalItem(t *testing.T) {
	t.Parallel()
	globalDir := t.TempDir()

	// Create a skills item with metadata.
	skillDir := filepath.Join(globalDir, "skills", "my-skill")
	os.MkdirAll(skillDir, 0755)
	now := nowPtr()
	meta := &metadata.Meta{
		ID:         metadata.NewID(),
		Name:       "my-skill",
		SourceHash: "sha256:abc123",
		AddedAt:    now,
	}
	if err := metadata.Save(skillDir, meta); err != nil {
		t.Fatalf("Save: %v", err)
	}

	idx, err := BuildLibraryIndex(globalDir)
	if err != nil {
		t.Fatalf("BuildLibraryIndex: %v", err)
	}

	key := "skills/my-skill"
	m, ok := idx[key]
	if !ok {
		t.Fatalf("expected key %q in index, keys: %v", key, indexKeys(idx))
	}
	if m.SourceHash != "sha256:abc123" {
		t.Errorf("SourceHash: got %q, want %q", m.SourceHash, "sha256:abc123")
	}
}

func TestBuildLibraryIndex_ProviderSpecificItem(t *testing.T) {
	t.Parallel()
	globalDir := t.TempDir()

	ruleDir := filepath.Join(globalDir, "rules", "claude-code", "security")
	os.MkdirAll(ruleDir, 0755)
	meta := &metadata.Meta{
		ID:         metadata.NewID(),
		Name:       "security",
		SourceHash: "sha256:def456",
	}
	if err := metadata.Save(ruleDir, meta); err != nil {
		t.Fatalf("Save: %v", err)
	}

	idx, err := BuildLibraryIndex(globalDir)
	if err != nil {
		t.Fatalf("BuildLibraryIndex: %v", err)
	}

	key := "rules/claude-code/security"
	m, ok := idx[key]
	if !ok {
		t.Fatalf("expected key %q in index", key)
	}
	if m.SourceHash != "sha256:def456" {
		t.Errorf("SourceHash: got %q", m.SourceHash)
	}
}

func TestSourceHash_Deterministic(t *testing.T) {
	t.Parallel()
	content := []byte("# Security rule\nDon't trust user input.")
	h1 := sourceHash(content)
	h2 := sourceHash(content)
	if h1 != h2 {
		t.Errorf("sourceHash is not deterministic: %q vs %q", h1, h2)
	}
	if len(h1) == 0 || h1[:7] != "sha256:" {
		t.Errorf("expected sha256: prefix, got %q", h1)
	}
}

func TestSourceHash_DifferentContent(t *testing.T) {
	t.Parallel()
	h1 := sourceHash([]byte("content A"))
	h2 := sourceHash([]byte("content B"))
	if h1 == h2 {
		t.Error("different content should produce different hashes")
	}
}

func TestLibraryKey_Universal(t *testing.T) {
	t.Parallel()
	got := libraryKey(catalog.Skills, "claude-code", "my-skill")
	if got != "skills/my-skill" {
		t.Errorf("got %q, want %q", got, "skills/my-skill")
	}
}

func TestLibraryKey_ProviderSpecific(t *testing.T) {
	t.Parallel()
	got := libraryKey(catalog.Rules, "claude-code", "security")
	if got != "rules/claude-code/security" {
		t.Errorf("got %q, want %q", got, "rules/claude-code/security")
	}
}

// helpers

func nowPtr() *metadata.Meta {
	return nil
}

func indexKeys(idx LibraryIndex) []string {
	var keys []string
	for k := range idx {
		keys = append(keys, k)
	}
	return keys
}
```

4. Fix the broken `nowPtr()` helper — it should return a `*time.Time`:

```go
import "time"

func nowPtr() *time.Time {
	now := time.Now().UTC()
	return &now
}
```

Update the test to use it correctly where needed, or just remove it from the unused `TestBuildLibraryIndex_UniversalItem` since `AddedAt` is not checked there.

5. Build and test:
```bash
cd /home/hhewett/.local/src/syllago/cli && make test 2>&1 | grep -E "add|FAIL|ok"
```

**Success Criteria:**
- [ ] `cli/internal/add` package compiles
- [ ] All 6 tests pass
- [ ] `BuildLibraryIndex` correctly indexes universal and provider-specific items
- [ ] `sourceHash` returns a deterministic `"sha256:<hex>"` string

---

## Phase 3: New `add` Package — `AddItems` Write Logic

### Task 3.1: Implement `AddItems` and hash-writing in `writeItem`

**Files:**
- Modify: `cli/internal/add/add.go`
- Modify: `cli/internal/add/add_test.go`

**Depends on:** Task 2.1

**Steps:**

1. Add the following to `cli/internal/add/add.go` after the `DiscoverFromProvider` function. This replaces the write logic currently in `writeAddedContent` in `add_cmd.go` but adds hash storage and the new skip/update semantics:

```go
// AddItems writes the given items to globalDir according to opts.
// Items with StatusInLibrary are skipped unless opts.Force is true.
// Items with StatusOutdated are skipped unless opts.Force is true.
// Returns one AddResult per item.
func AddItems(items []DiscoveryItem, opts AddOptions, globalDir string, conv Converter, ver string) []AddResult {
	results := make([]AddResult, 0, len(items))
	for _, item := range items {
		r := writeItem(item, opts, globalDir, conv, ver)
		results = append(results, r)
	}
	return results
}

// Converter is the minimal interface needed for canonicalization.
// Matches converter.ContentConverter so callers can pass converter.For(ct).
type Converter interface {
	Canonicalize(raw []byte, sourceProvider string) (CanonicalResult, error)
}

// CanonicalResult holds the output of a Canonicalize call.
type CanonicalResult struct {
	Content  []byte
	Filename string
}

func writeItem(item DiscoveryItem, opts AddOptions, globalDir string, conv Converter, ver string) AddResult {
	r := AddResult{Name: item.Name, Type: item.Type}

	// Decide whether to write based on status and --force.
	switch item.Status {
	case StatusInLibrary:
		if !opts.Force {
			r.Status = AddStatusUpToDate
			return r
		}
	case StatusOutdated:
		if !opts.Force {
			r.Status = AddStatusSkipped
			return r
		}
	}

	// Determine destination directory.
	var destDir string
	if item.Type.IsUniversal() {
		destDir = filepath.Join(globalDir, string(item.Type), item.Name)
	} else {
		destDir = filepath.Join(globalDir, string(item.Type), opts.Provider, item.Name)
	}

	if opts.DryRun {
		if item.Status == StatusNew {
			r.Status = AddStatusAdded
		} else {
			r.Status = AddStatusUpdated
		}
		return r
	}

	// Read source.
	raw, err := os.ReadFile(item.Path)
	if err != nil {
		r.Status = AddStatusError
		r.Error = fmt.Errorf("reading %s: %w", item.Path, err)
		return r
	}

	hash := sourceHash(raw)

	// Canonicalize if a converter is provided.
	content := raw
	ext := filepath.Ext(item.Path)
	outFilename := ""
	if conv != nil {
		cr, canonErr := conv.Canonicalize(raw, opts.Provider)
		if canonErr == nil && cr.Content != nil {
			content = cr.Content
			if cr.Filename != "" {
				outFilename = cr.Filename
				ext = filepath.Ext(outFilename)
			}
		}
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		r.Status = AddStatusError
		r.Error = fmt.Errorf("creating %s: %w", destDir, err)
		return r
	}

	// Write canonical content file.
	contentFile := contentFilename(item.Type, item.Name, ext)
	if err := os.WriteFile(filepath.Join(destDir, contentFile), content, 0644); err != nil {
		r.Status = AddStatusError
		r.Error = fmt.Errorf("writing content: %w", err)
		return r
	}

	// Preserve original in .source/ if format differs from canonical.
	hasSource := false
	sourceExt := filepath.Ext(item.Path)
	if sourceExt != "" && sourceExt != ".md" {
		sourceDir := filepath.Join(destDir, ".source")
		if err := os.MkdirAll(sourceDir, 0755); err == nil {
			origDest := filepath.Join(sourceDir, filepath.Base(item.Path))
			os.WriteFile(origDest, raw, 0644)
			hasSource = true
		}
	}

	// Write metadata with source hash.
	if ver == "" {
		ver = "syllago"
	}
	now := timeNow()
	sourceFormatExt := strings.TrimPrefix(filepath.Ext(item.Path), ".")
	meta := &metadata.Meta{
		ID:             metadata.NewID(),
		Name:           item.Name,
		Type:           string(item.Type),
		SourceProvider: opts.Provider,
		SourceFormat:   sourceFormatExt,
		SourceType:     "provider",
		SourceHash:     hash,
		HasSource:      hasSource,
		AddedAt:        &now,
		AddedBy:        ver,
	}
	// Non-fatal metadata write failure.
	_ = metadata.Save(destDir, meta)

	if item.Status == StatusNew {
		r.Status = AddStatusAdded
	} else {
		r.Status = AddStatusUpdated
	}
	return r
}

// contentFilename returns the canonical content filename for a type.
// Mirrors contentFileForType in add_cmd.go (which will be deleted).
func contentFilename(ct catalog.ContentType, name, ext string) string {
	if ext == "" {
		ext = ".md"
	}
	switch ct {
	case catalog.Rules:
		return "rule" + ext
	case catalog.Hooks:
		return "hook.json"
	case catalog.Commands:
		return "command" + ext
	case catalog.Skills:
		return "SKILL.md"
	case catalog.Agents:
		return "agent.md"
	case catalog.Prompts:
		return "PROMPT.md"
	case catalog.MCP:
		return "mcp.json"
	default:
		return name + ext
	}
}

// timeNow is a var so tests can override it for deterministic timestamps.
var timeNow = func() time.Time { return time.Now().UTC() }
```

2. Add the required `"time"` import to the file's import block.

3. Add integration tests in `add_test.go` for `AddItems`:

```go
func TestAddItems_NewItem(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	globalDir := t.TempDir()

	// Create source file.
	content := []byte("# Security rule")
	srcPath := filepath.Join(tmp, "security.md")
	os.WriteFile(srcPath, content, 0644)

	items := []DiscoveryItem{
		{Name: "security", Type: catalog.Rules, Path: srcPath, Status: StatusNew},
	}
	opts := AddOptions{Provider: "claude-code"}
	results := AddItems(items, opts, globalDir, nil, "syllago-test")

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != AddStatusAdded {
		t.Errorf("expected AddStatusAdded, got %v", results[0].Status)
	}

	// Verify file was written.
	destDir := filepath.Join(globalDir, "rules", "claude-code", "security")
	if _, err := os.Stat(filepath.Join(destDir, "rule.md")); err != nil {
		t.Errorf("rule.md not written: %v", err)
	}

	// Verify metadata has source_hash.
	meta, err := metadata.Load(destDir)
	if err != nil || meta == nil {
		t.Fatalf("metadata load failed: %v", err)
	}
	if meta.SourceHash == "" {
		t.Error("expected source_hash to be set")
	}
	if !strings.HasPrefix(meta.SourceHash, "sha256:") {
		t.Errorf("expected sha256: prefix, got %q", meta.SourceHash)
	}
}

func TestAddItems_UpToDate_Skipped(t *testing.T) {
	t.Parallel()
	globalDir := t.TempDir()

	content := []byte("# Rule content")
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "my-rule.md")
	os.WriteFile(srcPath, content, 0644)

	// Pre-populate library with matching hash.
	destDir := filepath.Join(globalDir, "rules", "claude-code", "my-rule")
	os.MkdirAll(destDir, 0755)
	meta := &metadata.Meta{
		ID:         metadata.NewID(),
		Name:       "my-rule",
		SourceHash: sourceHash(content),
	}
	metadata.Save(destDir, meta)

	items := []DiscoveryItem{
		{Name: "my-rule", Type: catalog.Rules, Path: srcPath, Status: StatusInLibrary},
	}
	results := AddItems(items, AddOptions{Provider: "claude-code"}, globalDir, nil, "syllago-test")

	if results[0].Status != AddStatusUpToDate {
		t.Errorf("expected AddStatusUpToDate, got %v", results[0].Status)
	}
}

func TestAddItems_Outdated_SkippedWithoutForce(t *testing.T) {
	t.Parallel()
	globalDir := t.TempDir()
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "rule.md")
	os.WriteFile(srcPath, []byte("new content"), 0644)

	items := []DiscoveryItem{
		{Name: "rule", Type: catalog.Rules, Path: srcPath, Status: StatusOutdated},
	}
	results := AddItems(items, AddOptions{Provider: "claude-code"}, globalDir, nil, "test")
	if results[0].Status != AddStatusSkipped {
		t.Errorf("expected AddStatusSkipped, got %v", results[0].Status)
	}
}

func TestAddItems_Outdated_UpdatedWithForce(t *testing.T) {
	t.Parallel()
	globalDir := t.TempDir()
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "rule.md")
	os.WriteFile(srcPath, []byte("updated content"), 0644)

	items := []DiscoveryItem{
		{Name: "rule", Type: catalog.Rules, Path: srcPath, Status: StatusOutdated},
	}
	results := AddItems(items, AddOptions{Provider: "claude-code", Force: true}, globalDir, nil, "test")
	if results[0].Status != AddStatusUpdated {
		t.Errorf("expected AddStatusUpdated, got %v", results[0].Status)
	}
}

func TestAddItems_DryRun_NoWrite(t *testing.T) {
	t.Parallel()
	globalDir := t.TempDir()
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "rule.md")
	os.WriteFile(srcPath, []byte("# Content"), 0644)

	items := []DiscoveryItem{
		{Name: "rule", Type: catalog.Rules, Path: srcPath, Status: StatusNew},
	}
	results := AddItems(items, AddOptions{Provider: "claude-code", DryRun: true}, globalDir, nil, "test")
	if results[0].Status != AddStatusAdded {
		t.Errorf("expected AddStatusAdded in dry-run, got %v", results[0].Status)
	}

	// Verify nothing was actually written.
	entries, _ := os.ReadDir(globalDir)
	if len(entries) != 0 {
		t.Errorf("dry-run should not write anything, found %d entries", len(entries))
	}
}
```

4. Add the `"strings"` import to `add_test.go`.

5. Build and test:
```bash
cd /home/hhewett/.local/src/syllago/cli && make test 2>&1 | grep -E "add|FAIL|ok"
```

**Success Criteria:**
- [ ] `AddItems` correctly returns `AddStatusAdded`, `AddStatusUpToDate`, `AddStatusSkipped`, `AddStatusUpdated`
- [ ] Dry-run writes nothing to disk
- [ ] Source hash is stored in `.syllago.yaml` on add
- [ ] All 5 new tests pass

---

## Phase 4: Rewrite `add_cmd_test.go` for New Syntax (TDD)

Write the new tests before touching `add_cmd.go`. They will fail until Phase 5 lands, which is intentional.

### Task 4.1: Rewrite `add_cmd_test.go` to use positional args

**Files:**
- Modify: `cli/cmd/syllago/add_cmd_test.go`

**Depends on:** Task 3.1

**Steps:**

1. Replace the entire content of `add_cmd_test.go` with the following. Every test is rewritten for the new positional-arg interface. The `runAddPreviewJSON` helper and all `--type`/`--name`/`--preview` flag usages are gone.

```go
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// settingsJSON is a minimal claude-code settings.json with two hook groups.
const settingsJSON = `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{"type": "command", "command": "echo pre-bash", "statusMessage": "pre-bash-check"}]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Edit",
        "hooks": [{"type": "command", "command": "echo post-edit", "statusMessage": "post-edit-check"}]
      }
    ]
  }
}`

// setupHooksProject creates a temp dir with a project-scoped .claude/settings.json.
func setupHooksProject(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	claudeDir := filepath.Join(tmp, ".claude")
	os.MkdirAll(claudeDir, 0755)
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(settingsJSON), 0644)
	return tmp
}

// setupAddProject creates a temp dir with claude-code rule files.
func setupAddProject(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	rulesDir := filepath.Join(tmp, ".claude", "rules")
	os.MkdirAll(rulesDir, 0755)
	os.WriteFile(filepath.Join(rulesDir, "security.md"), []byte("# Security"), 0644)
	os.WriteFile(filepath.Join(rulesDir, "testing.md"), []byte("# Testing"), 0644)
	os.WriteFile(filepath.Join(rulesDir, "logging.md"), []byte("# Logging"), 0644)
	return tmp
}

// runDiscoveryJSON runs "syllago add --from <provider> --json" (discovery mode,
// no positional target) and returns the parsed JSON output.
func runDiscoveryJSON(t *testing.T, tmp string) map[string]interface{} {
	t.Helper()
	stdout, _ := output.SetForTest(t)
	output.JSON = true

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("dry-run", "false")

	if err := addCmd.RunE(addCmd, []string{}); err != nil {
		t.Fatalf("add discovery failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, stdout.String())
	}
	return result
}

func TestAddRequiresFrom(t *testing.T) {
	addCmd.Flags().Set("from", "")
	err := addCmd.RunE(addCmd, []string{})
	if err == nil {
		t.Error("add without --from should fail")
	}
}

func TestAddUnknownProvider(t *testing.T) {
	addCmd.Flags().Set("from", "nonexistent")
	err := addCmd.RunE(addCmd, []string{})
	if err == nil {
		t.Error("add with unknown provider should fail")
	}
}

func TestAddAllAndPositionalIsError(t *testing.T) {
	tmp := setupAddProject(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	_, _ = output.SetForTest(t)

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("all", "true")
	t.Cleanup(func() { addCmd.Flags().Set("all", "false") })

	err := addCmd.RunE(addCmd, []string{"rules"})
	if err == nil {
		t.Error("specifying both --all and a positional target should fail")
	}
	if !strings.Contains(err.Error(), "--all") {
		t.Errorf("expected error message to mention --all, got: %v", err)
	}
}

// TestAddDiscoveryMode verifies that no-arg invocation returns JSON with a
// "provider" field and "groups" array, without writing any files.
func TestAddDiscoveryMode(t *testing.T) {
	tmp := setupAddProject(t)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	result := runDiscoveryJSON(t, tmp)

	if result["provider"] != "claude-code" {
		t.Errorf("expected provider=claude-code in JSON, got: %v", result["provider"])
	}

	// No files should be written.
	entries, _ := os.ReadDir(globalDir)
	if len(entries) != 0 {
		t.Errorf("discovery mode should not write anything, found %d entries", len(entries))
	}
}

// TestAddDiscoveryJSONGroups verifies that discovered rules appear in the
// groups array with status annotations.
func TestAddDiscoveryJSONGroups(t *testing.T) {
	tmp := setupAddProject(t)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	result := runDiscoveryJSON(t, tmp)

	groups, ok := result["groups"].([]interface{})
	if !ok || len(groups) == 0 {
		t.Fatalf("expected non-empty groups array, got: %v", result["groups"])
	}

	// Find the rules group.
	var rulesGroup map[string]interface{}
	for _, g := range groups {
		gm := g.(map[string]interface{})
		if gm["type"] == "rules" {
			rulesGroup = gm
			break
		}
	}
	if rulesGroup == nil {
		t.Fatalf("expected a rules group in JSON output")
	}
	items, _ := rulesGroup["items"].([]interface{})
	if len(items) < 3 {
		t.Errorf("expected at least 3 items in rules group, got %d", len(items))
	}
}

// TestAddByType verifies "syllago add rules --from claude-code" writes files.
func TestAddByType(t *testing.T) {
	tmp := setupAddProject(t)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	_, _ = output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("dry-run", "false")
	addCmd.Flags().Set("no-input", "true")

	if err := addCmd.RunE(addCmd, []string{"rules"}); err != nil {
		t.Fatalf("add rules failed: %v", err)
	}

	// All 3 rules should have been written.
	for _, name := range []string{"security", "testing", "logging"} {
		itemDir := filepath.Join(globalDir, "rules", "claude-code", name)
		if _, err := os.Stat(itemDir); err != nil {
			t.Errorf("expected %s item dir at %s, got: %v", name, itemDir, err)
		}
	}
}

// TestAddSpecificItem verifies "syllago add rules/security --from claude-code".
func TestAddSpecificItem(t *testing.T) {
	tmp := setupAddProject(t)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	_, _ = output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("dry-run", "false")
	addCmd.Flags().Set("no-input", "true")

	if err := addCmd.RunE(addCmd, []string{"rules/security"}); err != nil {
		t.Fatalf("add rules/security failed: %v", err)
	}

	// Only security should be written.
	itemDir := filepath.Join(globalDir, "rules", "claude-code", "security")
	if _, err := os.Stat(itemDir); err != nil {
		t.Fatalf("expected security item dir at %s, got: %v", itemDir, err)
	}

	// Other rules should not exist.
	for _, name := range []string{"testing", "logging"} {
		otherDir := filepath.Join(globalDir, "rules", "claude-code", name)
		if _, err := os.Stat(otherDir); err == nil {
			t.Errorf("expected %s to NOT be written, but it was", name)
		}
	}
}

// TestAddItemNotFound verifies that "syllago add rules/nonexistent" returns an error.
func TestAddItemNotFound(t *testing.T) {
	tmp := setupAddProject(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	_, _ = output.SetForTest(t)

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("all", "false")

	err := addCmd.RunE(addCmd, []string{"rules/nonexistent-xyz"})
	if err == nil {
		t.Error("expected error for nonexistent item, got nil")
	}
}

// TestAddUnknownType verifies that "syllago add widgets" returns an error.
func TestAddUnknownType(t *testing.T) {
	tmp := setupAddProject(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	_, _ = output.SetForTest(t)

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("all", "false")

	err := addCmd.RunE(addCmd, []string{"widgets"})
	if err == nil {
		t.Error("expected error for unknown content type")
	}
}

func TestAddDryRunDoesNotWrite(t *testing.T) {
	tmp := setupAddProject(t)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	stdout, _ := output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("dry-run", "true")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("no-input", "true")
	t.Cleanup(func() { addCmd.Flags().Set("dry-run", "false") })

	if err := addCmd.RunE(addCmd, []string{"rules"}); err != nil {
		t.Fatalf("add --dry-run failed: %v", err)
	}

	entries, err := os.ReadDir(globalDir)
	if err != nil {
		t.Fatalf("could not read globalDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected global dir to be empty during --dry-run, found %d entries", len(entries))
	}

	out := stdout.String()
	if !strings.Contains(out, "dry-run") {
		t.Errorf("expected 'dry-run' in output, got: %s", out)
	}
}

func TestAddWritesToGlobalDir(t *testing.T) {
	tmp := setupAddProject(t)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	_, _ = output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("dry-run", "false")
	addCmd.Flags().Set("no-input", "true")

	if err := addCmd.RunE(addCmd, []string{"rules/security"}); err != nil {
		t.Fatalf("add failed: %v", err)
	}

	itemDir := filepath.Join(globalDir, "rules", "claude-code", "security")
	if _, err := os.Stat(itemDir); err != nil {
		t.Fatalf("expected item directory at %s, got error: %v", itemDir, err)
	}
	if _, err := os.Stat(filepath.Join(itemDir, ".syllago.yaml")); err != nil {
		t.Errorf("expected metadata at %s: %v", itemDir, err)
	}
	if _, err := os.Stat(filepath.Join(itemDir, "rule.md")); err != nil {
		t.Errorf("expected rule.md at %s: %v", itemDir, err)
	}
}

func TestAddWritesMetadata(t *testing.T) {
	tmp := setupAddProject(t)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	_, _ = output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("dry-run", "false")
	addCmd.Flags().Set("no-input", "true")

	if err := addCmd.RunE(addCmd, []string{"rules/security"}); err != nil {
		t.Fatalf("add failed: %v", err)
	}

	destDir := filepath.Join(globalDir, "rules", "claude-code", "security")
	m, err := metadata.Load(destDir)
	if err != nil || m == nil {
		t.Fatalf("metadata load failed: %v", err)
	}
	if m.SourceProvider != "claude-code" {
		t.Errorf("expected source_provider=claude-code, got %q", m.SourceProvider)
	}
	if m.AddedAt == nil {
		t.Error("expected added_at to be set")
	}
	if m.SourceType != "provider" {
		t.Errorf("expected source_type=provider, got %q", m.SourceType)
	}
	if m.SourceHash == "" {
		t.Error("expected source_hash to be set")
	}
}

func TestAddHooksDiscovery(t *testing.T) {
	tmp := setupHooksProject(t)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	stdout, _ := output.SetForTest(t)
	output.JSON = true

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("all", "false")

	if err := addCmd.RunE(addCmd, []string{}); err != nil {
		t.Fatalf("add discovery failed: %v", err)
	}

	// No files should be written.
	entries, _ := os.ReadDir(globalDir)
	if len(entries) != 0 {
		t.Errorf("discovery mode should not write anything, found %d entries", len(entries))
	}

	// JSON output should mention hooks.
	out := stdout.String()
	if !strings.Contains(strings.ToLower(out), "hook") {
		t.Errorf("expected hooks to appear in discovery JSON, got: %s", out)
	}
}

func TestAddHooksWritesToGlobalDir(t *testing.T) {
	tmp := setupHooksProject(t)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	_, _ = output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("dry-run", "false")
	addCmd.Flags().Set("scope", "project")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("exclude", "")

	if err := addCmd.RunE(addCmd, []string{"hooks"}); err != nil {
		t.Fatalf("add hooks failed: %v", err)
	}

	hooksBase := filepath.Join(globalDir, "hooks", "claude-code")
	entries, err := os.ReadDir(hooksBase)
	if err != nil {
		t.Fatalf("expected global hooks dir to exist, got error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 hook directories, got %d", len(entries))
	}

	for _, entry := range entries {
		itemDir := filepath.Join(hooksBase, entry.Name())
		if _, err := os.Stat(filepath.Join(itemDir, "hook.json")); err != nil {
			t.Errorf("expected hook.json in %s: %v", itemDir, err)
		}
		if _, err := os.Stat(filepath.Join(itemDir, ".syllago.yaml")); err != nil {
			t.Errorf("expected .syllago.yaml in %s: %v", itemDir, err)
		}
	}
}

func TestAddHooksExclude(t *testing.T) {
	tmp := setupHooksProject(t)
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	_, _ = output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("dry-run", "false")
	addCmd.Flags().Set("scope", "project")
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("exclude", "pre-bash-check")

	if err := addCmd.RunE(addCmd, []string{"hooks"}); err != nil {
		t.Fatalf("add hooks --exclude failed: %v", err)
	}

	hooksBase := filepath.Join(globalDir, "hooks", "claude-code")
	entries, err := os.ReadDir(hooksBase)
	if err != nil {
		t.Fatalf("expected global hooks dir to exist: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 hook after --exclude, got %d", len(entries))
	}
	if entries[0].Name() == "pre-bash-check" {
		t.Errorf("excluded hook 'pre-bash-check' was still added")
	}
}

func TestAddHooksForce(t *testing.T) {
	t.Run("skip without force", func(t *testing.T) {
		tmp := setupHooksProject(t)
		globalDir := t.TempDir()

		original := catalog.GlobalContentDirOverride
		catalog.GlobalContentDirOverride = globalDir
		t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

		origRoot := findProjectRoot
		findProjectRoot = func() (string, error) { return tmp, nil }
		t.Cleanup(func() { findProjectRoot = origRoot })

		// Pre-create one hook directory.
		existingDir := filepath.Join(globalDir, "hooks", "claude-code", "pre-bash-check")
		os.MkdirAll(existingDir, 0755)
		os.WriteFile(filepath.Join(existingDir, "hook.json"), []byte(`{"event":"old"}`), 0644)

		stdout, _ := output.SetForTest(t)
		if err := runAddHooks(tmp, "claude-code", false, nil, false, "project", nil); err != nil {
			t.Fatalf("runAddHooks without force failed: %v", err)
		}
		out := stdout.String()
		if !strings.Contains(out, "SKIP") {
			t.Errorf("expected SKIP message for existing hook, got: %s", out)
		}
		data, _ := os.ReadFile(filepath.Join(existingDir, "hook.json"))
		if !strings.Contains(string(data), "old") {
			t.Errorf("expected existing hook.json to be unchanged without force")
		}
	})

	t.Run("overwrite with force", func(t *testing.T) {
		tmp := setupHooksProject(t)
		globalDir := t.TempDir()

		original := catalog.GlobalContentDirOverride
		catalog.GlobalContentDirOverride = globalDir
		t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

		origRoot := findProjectRoot
		findProjectRoot = func() (string, error) { return tmp, nil }
		t.Cleanup(func() { findProjectRoot = origRoot })

		existingDir := filepath.Join(globalDir, "hooks", "claude-code", "pre-bash-check")
		os.MkdirAll(existingDir, 0755)
		os.WriteFile(filepath.Join(existingDir, "hook.json"), []byte(`{"event":"old"}`), 0644)

		_, _ = output.SetForTest(t)
		if err := runAddHooks(tmp, "claude-code", false, nil, true, "project", nil); err != nil {
			t.Fatalf("runAddHooks with force failed: %v", err)
		}
		data, _ := os.ReadFile(filepath.Join(existingDir, "hook.json"))
		if strings.Contains(string(data), `"event":"old"`) {
			t.Errorf("expected hook.json to be overwritten with force, still has old content")
		}
		if !strings.Contains(string(data), "PreToolUse") {
			t.Errorf("expected overwritten hook.json to contain 'PreToolUse', got: %s", data)
		}
	})
}

func TestAddPreservesSourceForNonCanonicalFormat(t *testing.T) {
	tmp := t.TempDir()
	globalDir := t.TempDir()

	original := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = original })

	rulesDir := filepath.Join(tmp, ".cursor", "rules")
	os.MkdirAll(rulesDir, 0755)
	os.WriteFile(filepath.Join(rulesDir, "my-rule.mdc"), []byte("# My Rule\ncontent"), 0644)

	_, _ = output.SetForTest(t)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return tmp, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	addCmd.Flags().Set("from", "cursor")
	addCmd.Flags().Set("all", "false")
	addCmd.Flags().Set("force", "true")
	addCmd.Flags().Set("dry-run", "false")
	addCmd.Flags().Set("no-input", "true")
	t.Cleanup(func() { addCmd.Flags().Set("force", "false") })

	if err := addCmd.RunE(addCmd, []string{"rules/my-rule"}); err != nil {
		t.Fatalf("add rules/my-rule failed: %v", err)
	}

	dest := filepath.Join(globalDir, "rules", "cursor", "my-rule")
	if _, err := os.Stat(filepath.Join(dest, "rule.md")); err != nil {
		t.Errorf("expected canonical rule.md: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, ".source", "my-rule.mdc")); err != nil {
		t.Errorf("expected .source/my-rule.mdc: %v", err)
	}
}
```

2. Verify the tests compile (they will fail at runtime until Phase 5):
```bash
cd /home/hhewett/.local/src/syllago/cli && go build ./cmd/syllago/... 2>&1
```

**Success Criteria:**
- [ ] `add_cmd_test.go` compiles with no errors
- [ ] All `--type`, `--name`, `--preview` flag usages are gone
- [ ] `runAddPreviewJSON` helper is replaced by `runDiscoveryJSON`
- [ ] Test count is 15 (up from 10 — new tests for `--all` conflict, type+item targeting, discovery JSON shape, unknown type)

---

## Phase 5: Rewrite `add_cmd.go`

### Task 5.1: Rewrite the cobra command definition and `init()` in `add_cmd.go`

**Files:**
- Modify: `cli/cmd/syllago/add_cmd.go`

**Depends on:** Task 4.1

**Steps:**

1. Replace the entire file content. The new version:
   - Changes `Use` to `add [<type>[/<name>]]`
   - Removes `--type`, `--name`, `--preview` flags from `init()`
   - Adds `--all` flag
   - Makes `--from` required via `MarkFlagRequired`
   - Routes to discovery, by-type, specific-item, or `--all` based on args
   - Uses `add.DiscoverFromProvider` and `add.AddItems` from the new package
   - Keeps `runAddHooks` unchanged (it still handles the hooks path)
   - Deletes `writeAddedContent`, `itemNameFromPath`, `contentFileForType`, `printAddDiscoveryReport`, `printAddDiscoveryDiagnostics` (replaced by new output functions)

```go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add [<type>[/<name>]]",
	Short: "Add content to your library from a provider",
	Long: `Discovers content from a provider and adds it to your library (~/.syllago/content/).

Omitting the positional argument runs discovery mode: shows what the provider has
and annotates each item with its library status (new, in library, outdated).

Syllago handles format conversion automatically. Once added, content can be
installed to any supported provider with "syllago install --to <provider>".

Examples:
  syllago add --from claude-code                  Discover what's available
  syllago add rules --from claude-code            Add all rules
  syllago add rules/security --from claude-code   Add a specific rule
  syllago add hooks --from claude-code            Add all hooks
  syllago add --all --from claude-code            Add everything
  syllago add rules --from claude-code --force    Force-update changed items
  syllago add rules --from claude-code --dry-run  Show what would be written

Hooks-specific flags (--exclude, --scope) are ignored for non-hook targets.

After adding, use "syllago install" to activate content in a provider.`,
	RunE: runAdd,
}

func init() {
	addCmd.Flags().String("from", "", "Provider to add from (required)")
	addCmd.MarkFlagRequired("from")
	addCmd.Flags().Bool("all", false, "Add all discovered content (cannot combine with positional target)")
	addCmd.Flags().BoolP("force", "f", false, "Overwrite existing items (update changed + re-add identical)")
	addCmd.Flags().Bool("dry-run", false, "Show what would be written without actually writing")
	addCmd.Flags().Bool("no-input", false, "Disable interactive prompts, use defaults")
	addCmd.Flags().String("base-dir", "", "Override base directory for content discovery")
	// Hooks-specific flags — hidden so they don't clutter default --help.
	addCmd.Flags().StringArray("exclude", nil, "Skip hooks by auto-derived name (hooks only)")
	addCmd.Flags().String("scope", "global", "Settings scope: global, project, or all (hooks only)")
	addCmd.Flags().MarkHidden("exclude")
	addCmd.Flags().MarkHidden("scope")
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}

	fromSlug, _ := cmd.Flags().GetString("from")
	prov := findProviderBySlug(fromSlug)
	if prov == nil {
		slugs := providerSlugs()
		output.PrintError(1, "unknown provider: "+fromSlug, "Available: "+strings.Join(slugs, ", "))
		return fmt.Errorf("unknown provider: %s", fromSlug)
	}

	allFlag, _ := cmd.Flags().GetBool("all")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	force, _ := cmd.Flags().GetBool("force")
	baseDir, _ := cmd.Flags().GetString("base-dir")

	// --all and positional target are mutually exclusive.
	if allFlag && len(args) > 0 {
		return fmt.Errorf("cannot specify both a target and --all")
	}

	// Build resolver from merged config + CLI flag.
	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("loading global config: %w", err)
	}
	projectCfg, err := config.Load(root)
	if err != nil {
		return fmt.Errorf("loading project config: %w", err)
	}
	mergedCfg := config.Merge(globalCfg, projectCfg)
	resolver := config.NewResolver(mergedCfg, baseDir)
	if err := resolver.ExpandPaths(); err != nil {
		return fmt.Errorf("expanding paths: %w", err)
	}

	globalDir := catalog.GlobalContentDir()
	if globalDir == "" {
		return fmt.Errorf("cannot determine home directory")
	}

	// Parse positional target: "" | "type" | "type/name"
	var typeTarget string
	var nameTarget string
	if len(args) > 0 {
		parts := strings.SplitN(args[0], "/", 2)
		typeTarget = parts[0]
		if len(parts) == 2 {
			nameTarget = parts[1]
		}

		// Validate the type target.
		validTypes := make(map[string]bool)
		for _, ct := range catalog.AllContentTypes() {
			validTypes[string(ct)] = true
		}
		if !validTypes[typeTarget] {
			all := make([]string, 0, len(validTypes))
			for k := range validTypes {
				all = append(all, k)
			}
			return fmt.Errorf("unknown content type %q — available: %s", typeTarget, strings.Join(all, ", "))
		}
	}

	// Hooks require their own path — route there when the type is hooks.
	if typeTarget == string(catalog.Hooks) {
		exclude, _ := cmd.Flags().GetStringArray("exclude")
		scope, _ := cmd.Flags().GetString("scope")
		previewOnly := !allFlag && len(args) == 0
		return runAddHooks(root, fromSlug, previewOnly || dryRun, exclude, force, scope, resolver)
	}

	// Discover items from provider.
	items, err := add.DiscoverFromProvider(*prov, root, resolver, globalDir)
	if err != nil {
		return fmt.Errorf("discovering content: %w", err)
	}

	// No-arg = discovery mode (informational only).
	if len(args) == 0 && !allFlag {
		return printDiscovery(items, fromSlug)
	}

	// Apply type filter.
	if typeTarget != "" {
		ct := catalog.ContentType(typeTarget)
		// Check provider support.
		if prov.SupportsType != nil && !prov.SupportsType(ct) {
			return fmt.Errorf("%s does not support %s", prov.Name, ct.Label())
		}
		var filtered []add.DiscoveryItem
		for _, it := range items {
			if it.Type == ct {
				filtered = append(filtered, it)
			}
		}
		items = filtered
	}

	// Apply name filter (type/name target).
	if nameTarget != "" {
		var filtered []add.DiscoveryItem
		for _, it := range items {
			if it.Name == nameTarget {
				filtered = append(filtered, it)
			}
		}
		if len(filtered) == 0 {
			// Collect available names for the helpful error message.
			var available []string
			for _, it := range items {
				available = append(available, it.Name)
			}
			return fmt.Errorf("no %s named %q found in %s — available: %s",
				typeTarget, nameTarget, prov.Name, strings.Join(available, ", "))
		}
		items = filtered
	}

	if len(items) == 0 {
		fmt.Fprintf(output.Writer, "No content found for %s.\n", prov.Name)
		return nil
	}

	// Execute add.
	opts := add.AddOptions{
		Force:    force,
		DryRun:   dryRun,
		Provider: fromSlug,
	}
	results := add.AddItems(items, opts, globalDir, converterFor, version)

	printAddResults(results, dryRun)
	return nil
}

// converterFor adapts converter.For to the add.Converter interface.
// Returns nil when no converter exists for the type.
func converterFor(ct catalog.ContentType) add.Converter {
	// add.AddItems calls this per-item; we adapt the existing converter.For.
	// This function is passed as the conv parameter below.
	return nil // placeholder — see Task 5.2 for wiring
}

// printDiscovery renders the discovery-mode output (no target given).
func printDiscovery(items []add.DiscoveryItem, providerSlug string) error {
	prov := findProviderBySlug(providerSlug)
	provName := providerSlug
	if prov != nil {
		provName = prov.Name
	}

	if output.JSON {
		return printDiscoveryJSON(items, providerSlug)
	}

	if len(items) == 0 {
		fmt.Fprintf(output.Writer, "No content found for %s.\n", provName)
		return nil
	}

	// Group by type.
	groups := groupByType(items)
	fmt.Fprintf(output.Writer, "\nDiscovered content from %s:\n", provName)
	for _, ct := range catalog.AllContentTypes() {
		group, ok := groups[ct]
		if !ok || len(group) == 0 {
			continue
		}
		fmt.Fprintf(output.Writer, "  %s (%d):\n", ct.Label(), len(group))
		for _, it := range group {
			fmt.Fprintf(output.Writer, "    %-20s (%s)\n", it.Name, it.Status)
		}
	}

	// Contextual footer: list types with actionable items.
	fmt.Fprintf(output.Writer, "\nAdd by type:\n")
	for _, ct := range catalog.AllContentTypes() {
		group := groups[ct]
		hasActionable := false
		for _, it := range group {
			if it.Status != add.StatusInLibrary {
				hasActionable = true
				break
			}
		}
		if hasActionable {
			fmt.Fprintf(output.Writer, "  syllago add %s --from %s\n", ct, providerSlug)
		}
	}

	// Pick a "new" item for the example, falling back to any item.
	var exampleItem *add.DiscoveryItem
	for i := range items {
		if items[i].Status == add.StatusNew {
			exampleItem = &items[i]
			break
		}
	}
	if exampleItem == nil && len(items) > 0 {
		exampleItem = &items[0]
	}
	if exampleItem != nil {
		fmt.Fprintf(output.Writer, "\nAdd a specific item:\n  syllago add %s/%s --from %s\n",
			exampleItem.Type, exampleItem.Name, providerSlug)
	}

	fmt.Fprintf(output.Writer, "\nAdd everything:\n  syllago add --all --from %s\n", providerSlug)
	fmt.Fprintf(output.Writer, "\nSee also:\n  Convert format:    syllago convert <item> --to <provider>\n  Install content:   syllago install <item> --to <provider>\n")
	return nil
}

// printDiscoveryJSON emits the JSON discovery format.
func printDiscoveryJSON(items []add.DiscoveryItem, providerSlug string) error {
	type JSONItem struct {
		Name   string `json:"name"`
		Path   string `json:"path"`
		Status string `json:"status"`
	}
	type JSONGroup struct {
		Type  string     `json:"type"`
		Count int        `json:"count"`
		Items []JSONItem `json:"items"`
	}
	type JSONResult struct {
		Provider string      `json:"provider"`
		Groups   []JSONGroup `json:"groups"`
	}

	groups := groupByType(items)
	result := JSONResult{Provider: providerSlug}
	for _, ct := range catalog.AllContentTypes() {
		group, ok := groups[ct]
		if !ok || len(group) == 0 {
			continue
		}
		g := JSONGroup{Type: string(ct), Count: len(group)}
		for _, it := range group {
			statusStr := "new"
			switch it.Status {
			case add.StatusInLibrary:
				statusStr = "in_library"
			case add.StatusOutdated:
				statusStr = "outdated"
			}
			g.Items = append(g.Items, JSONItem{
				Name:   it.Name,
				Path:   it.Path,
				Status: statusStr,
			})
		}
		result.Groups = append(result.Groups, g)
	}

	enc := json.NewEncoder(output.Writer)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

// groupByType organizes a flat item list into a map keyed by ContentType.
func groupByType(items []add.DiscoveryItem) map[catalog.ContentType][]add.DiscoveryItem {
	groups := make(map[catalog.ContentType][]add.DiscoveryItem)
	for _, it := range items {
		groups[it.Type] = append(groups[it.Type], it)
	}
	return groups
}

// printAddResults renders the per-item results and summary line.
func printAddResults(results []add.AddResult, dryRun bool) {
	if output.Quiet {
		return
	}

	counts := map[add.AddStatus]int{}
	for _, r := range results {
		if r.Error != nil {
			fmt.Fprintf(output.ErrWriter, "  Warning: failed to add %s: %v\n", r.Name, r.Error)
			continue
		}
		counts[r.Status]++
		switch r.Status {
		case add.AddStatusAdded:
			label := "added"
			if dryRun {
				label = "[dry-run] would add"
			}
			fmt.Fprintf(output.Writer, "  %-20s %s\n", r.Name, label)
		case add.AddStatusUpdated:
			label := "updated"
			if dryRun {
				label = "[dry-run] would update"
			}
			fmt.Fprintf(output.Writer, "  %-20s %s\n", r.Name, label)
		case add.AddStatusUpToDate:
			fmt.Fprintf(output.Writer, "  %-20s up to date\n", r.Name)
		case add.AddStatusSkipped:
			fmt.Fprintf(output.Writer, "  %-20s source changed (use --force to update)\n", r.Name)
		}
	}

	// Summary line.
	fmt.Fprintln(output.Writer)
	var parts []string
	if n := counts[add.AddStatusAdded] + counts[add.AddStatusUpdated]; n > 0 {
		verb := "Added"
		if dryRun {
			verb = "Would add"
		}
		parts = append(parts, fmt.Sprintf("%s %d item(s)", verb, n))
	}
	if n := counts[add.AddStatusUpToDate]; n > 0 {
		parts = append(parts, fmt.Sprintf("%d up to date", n))
	}
	if n := counts[add.AddStatusSkipped]; n > 0 {
		parts = append(parts, fmt.Sprintf("%d has updates (use --force)", n))
	}
	if len(parts) > 0 {
		fmt.Fprintln(output.Writer, strings.Join(parts, ". ")+".")
	}
}

// runAddHooks handles "syllago add hooks --from <provider>".
// Unchanged from the previous implementation.
func runAddHooks(root, fromSlug string, previewOnly bool, exclude []string, force bool, scope string, resolver *config.PathResolver) error {
	prov := findProviderBySlug(fromSlug)
	if prov == nil {
		return fmt.Errorf("unknown provider: %s", fromSlug)
	}

	baseDir := ""
	if resolver != nil {
		baseDir = resolver.BaseDir(prov.Slug)
	}
	locations, err := installer.FindSettingsLocationsWithBase(*prov, root, baseDir)
	if err != nil {
		return fmt.Errorf("finding settings locations: %w", err)
	}

	var targets []installer.SettingsLocation
	for _, loc := range locations {
		if scope == "all" || loc.Scope.String() == scope {
			targets = append(targets, loc)
		}
	}

	if len(targets) == 0 {
		fmt.Fprintf(output.Writer, "No settings.json found for %s (scope: %s).\n", fromSlug, scope)
		return nil
	}

	excludeSet := make(map[string]bool, len(exclude))
	for _, ex := range exclude {
		excludeSet[ex] = true
	}

	for _, loc := range targets {
		if err := addHooksFromLocation(fromSlug, loc, previewOnly, excludeSet, force); err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: failed to add hooks from %s: %v\n", loc.Path, err)
		}
	}
	return nil
}

// addHooksFromLocation reads a single settings.json and writes individual hooks.
// Unchanged from the previous implementation.
func addHooksFromLocation(fromSlug string, loc installer.SettingsLocation, previewOnly bool, excludeSet map[string]bool, force bool) error {
	data, err := os.ReadFile(loc.Path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", loc.Path, err)
	}

	candidates, err := converter.SplitSettingsHooks(data, fromSlug)
	if err != nil {
		return fmt.Errorf("splitting hooks from %s: %w", loc.Path, err)
	}

	var filtered []converter.HookData
	for _, hook := range candidates {
		name := converter.DeriveHookName(hook)
		if !excludeSet[name] {
			filtered = append(filtered, hook)
		}
	}

	if previewOnly {
		fmt.Fprintf(output.Writer, "Hooks in %s (%s):\n", loc.Path, loc.Scope)
		for _, hook := range filtered {
			name := converter.DeriveHookName(hook)
			matcher := hook.Matcher
			if matcher == "" {
				matcher = "*"
			}
			fmt.Fprintf(output.Writer, "  %s   (%s/%s)\n", name, hook.Event, matcher)
		}
		fmt.Fprintf(output.Writer, "\n%d hooks would be added.\n", len(filtered))
		return nil
	}

	globalDir := catalog.GlobalContentDir()
	if globalDir == "" {
		return fmt.Errorf("cannot determine home directory")
	}

	count := 0
	for _, hook := range filtered {
		name := converter.DeriveHookName(hook)
		itemDir := filepath.Join(globalDir, string(catalog.Hooks), fromSlug, name)

		if !force {
			if _, err := os.Stat(itemDir); err == nil {
				fmt.Fprintf(output.Writer, "  SKIP %s (already exists, use --force to overwrite)\n", name)
				continue
			}
		}

		if err := os.MkdirAll(itemDir, 0755); err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: failed to create %s: %v\n", itemDir, err)
			continue
		}

		hookJSON, err := json.MarshalIndent(hook, "", "  ")
		if err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: failed to marshal hook %s: %v\n", name, err)
			continue
		}
		if err := os.WriteFile(filepath.Join(itemDir, "hook.json"), hookJSON, 0644); err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: failed to write hook.json for %s: %v\n", name, err)
			continue
		}

		now := time.Now().UTC()
		meta := &metadata.Meta{
			ID:             metadata.NewID(),
			Name:           name,
			Type:           string(catalog.Hooks),
			AddedAt:        &now,
			SourceProvider: fromSlug,
			SourceFormat:   "json",
			SourceType:     "provider",
		}
		if err := metadata.Save(itemDir, meta); err != nil {
			fmt.Fprintf(output.ErrWriter, "Warning: failed to write metadata for %s: %v\n", name, err)
			continue
		}

		matcher := hook.Matcher
		if matcher == "" {
			matcher = "*"
		}
		fmt.Fprintf(output.Writer, "  %s   (%s/%s)\n", name, hook.Event, matcher)
		count++
	}
	fmt.Fprintf(output.Writer, "\nAdded %d hooks to library.\n", count)
	return nil
}
```

Note: `"encoding/json"`, `"path/filepath"`, `"time"` need to be in the imports. Remove `"parse"` import (no longer used directly) and add `"github.com/OpenScribbler/syllago/cli/internal/add"`.

2. Build:
```bash
cd /home/hhewett/.local/src/syllago/cli && go build ./cmd/syllago/... 2>&1
```
Fix any import or symbol errors.

**Success Criteria:**
- [ ] `add_cmd.go` compiles with no errors
- [ ] `--type`, `--name`, `--preview` flags are gone from `init()`
- [ ] `--all` flag is present
- [ ] `writeAddedContent`, `itemNameFromPath`, `contentFileForType`, `printAddDiscoveryReport`, `printAddDiscoveryDiagnostics` are deleted
- [ ] `runAddHooks` and `addHooksFromLocation` are preserved verbatim

---

### Task 5.2: Wire `converter.For` into `add.AddItems`

**Files:**
- Modify: `cli/cmd/syllago/add_cmd.go`
- Modify: `cli/internal/add/add.go`

**Depends on:** Task 5.1

**Steps:**

The `add.AddItems` signature currently takes a `Converter` interface instance, but file-based content items have different types. The cleanest fix is to make `AddItems` accept a `func(catalog.ContentType) Converter` factory instead of a single converter. This matches how `converter.For(ct)` works.

1. In `cli/internal/add/add.go`, change the `AddItems` signature:

```go
// ConverterFactory returns a Converter for a given content type, or nil if none exists.
type ConverterFactory func(ct catalog.ContentType) Converter

// AddItems writes selected items to the library. Returns per-item results.
func AddItems(items []DiscoveryItem, opts AddOptions, globalDir string, factory ConverterFactory, ver string) []AddResult {
	results := make([]AddResult, 0, len(items))
	for _, item := range items {
		var conv Converter
		if factory != nil {
			conv = factory(item.Type)
		}
		r := writeItem(item, opts, globalDir, conv, ver)
		results = append(results, r)
	}
	return results
}
```

2. In `add_cmd.go`, replace the placeholder `converterFor` function and the `add.AddItems` call:

```go
// converterFactory adapts converter.For to the add.ConverterFactory type.
func converterFactory(ct catalog.ContentType) add.Converter {
	c := converter.For(ct)
	if c == nil {
		return nil
	}
	return &converterAdapter{c}
}

// converterAdapter wraps converter.ContentConverter to implement add.Converter.
type converterAdapter struct {
	inner interface {
		Canonicalize(raw []byte, sourceProvider string) (converter.CanonicalizeResult, error)
	}
}

func (a *converterAdapter) Canonicalize(raw []byte, sourceProvider string) (add.CanonicalResult, error) {
	r, err := a.inner.Canonicalize(raw, sourceProvider)
	if err != nil {
		return add.CanonicalResult{}, err
	}
	return add.CanonicalResult{Content: r.Content, Filename: r.Filename}, nil
}
```

3. Check what `converter.For` actually returns and what `CanonicalizeResult` is named in that package:
```bash
cd /home/hhewett/.local/src/syllago/cli && grep -n "func For\|CanonicalizeResult\|type.*Result" internal/converter/*.go | head -30
```
Adjust type names to match exactly.

4. Replace the `converterFor` placeholder in `runAdd` with `converterFactory`:
```go
results := add.AddItems(items, opts, globalDir, converterFactory, version)
```

5. Build and test:
```bash
cd /home/hhewett/.local/src/syllago/cli && make test 2>&1 | grep -E "FAIL|ok|add"
```

**Success Criteria:**
- [ ] `converter.For(ct)` is correctly wired through the adapter
- [ ] `make test` passes for the `add` package
- [ ] `make test` passes for `cmd/syllago` (the rewritten tests from Phase 4 pass)

---

## Phase 6: Integration Testing and Cleanup

### Task 6.1: Run the full test suite and fix breakage

**Files:**
- Modify: any file with compilation errors or test failures

**Depends on:** Task 5.2

**Steps:**

1. Run the full test suite:
```bash
cd /home/hhewett/.local/src/syllago/cli && make test 2>&1
```

2. Expected failure sources and fixes:
   - If `metadata` package tests fail because `ImportedAt`/`ImportedBy` fields were referenced in other test files, update those references to use `AddedAt`/`AddedBy`.
   - If `cmd/syllago` tests fail on `TestAddPreservesSourceForNonCanonicalFormat`, verify the `.source/` preservation path in `writeItem` uses `filepath.Ext(item.Path) != ".md"` as its trigger (matching the original logic in `writeAddedContent` at lines 273-285 of the old `add_cmd.go`).
   - If hooks tests fail because `runAddHooks` signature changed, verify it still accepts the same arguments as before.

3. Search for any remaining references to the deleted flags:
```bash
cd /home/hhewett/.local/src/syllago/cli && grep -rn '"preview"\|"--preview"\|Flags.*type.*rules\|Flags.*name.*secur' --include="*_test.go" .
```
There should be zero results.

4. Build the final binary:
```bash
cd /home/hhewett/.local/src/syllago/cli && make build
```

**Success Criteria:**
- [ ] `make test` passes with zero failures
- [ ] `make build` succeeds
- [ ] No references to `--type`, `--name`, or `--preview` flags anywhere in `add_cmd.go` or `add_cmd_test.go`
- [ ] `syllago add --help` shows `add [<type>[/<name>]]` in the usage line

### Task 6.2: Manual smoke test of the new UX

**Files:** none (binary verification only)

**Depends on:** Task 6.1

**Steps:**

1. Discovery mode:
```bash
syllago add --from claude-code
```
Expected: prints "Discovered content from Claude Code:" with items and status annotations. No files written.

2. JSON discovery:
```bash
syllago add --from claude-code --json | python3 -m json.tool
```
Expected: valid JSON with `provider` and `groups` fields.

3. Type add:
```bash
syllago add rules --from claude-code --dry-run
```
Expected: shows "would add" lines for each rule. Nothing written.

4. Conflict detection (run twice):
```bash
syllago add rules --from claude-code
syllago add rules --from claude-code
```
Expected: second run shows "up to date" for all items.

5. `--all` conflict guard:
```bash
syllago add --all rules --from claude-code
```
Expected: error "cannot specify both a target and --all".

6. Unknown type:
```bash
syllago add widgets --from claude-code
```
Expected: error "unknown content type 'widgets'".

7. Item not found:
```bash
syllago add rules/nonexistent --from claude-code
```
Expected: error mentioning available rules.

**Success Criteria:**
- [ ] All 7 smoke tests produce the expected output
- [ ] No panics or unexpected errors

---

## Summary

| Phase | Tasks | Key output |
|-------|-------|------------|
| 1 | 1.1 | `SourceHash` in `metadata.Meta`, `ImportedAt`/`ImportedBy` removed |
| 2 | 2.1 | `add` package with types, `BuildLibraryIndex`, `DiscoverFromProvider` |
| 3 | 3.1 | `AddItems`, `writeItem` with hash storage and skip/update semantics |
| 4 | 4.1 | Rewritten tests (fail until Phase 5 lands — intentional TDD) |
| 5 | 5.1, 5.2 | Rewritten `add_cmd.go` with positional args, converter wiring |
| 6 | 6.1, 6.2 | Full test suite green, manual smoke test |

Total: 8 tasks. Dependencies flow linearly (1.1 → 2.1 → 3.1 → 4.1 → 5.1 → 5.2 → 6.1 → 6.2).
