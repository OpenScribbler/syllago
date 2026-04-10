# TUI Confirm-Bucket Rendering — Implementation Plan

**Goal:** Surface content-signal confidence tiers in the TUI — tier badges on library items (install wizard) and an interactive triage step in the add wizard for confirm-bucket items.

**Architecture:**
- Surface 1: Three new fields on `metadata.Meta` + `TierForMeta()` in the analyzer package + tier badge in `install_view.go`
- Surface 2: New `addStepTriage` step in `add_wizard.go` — conditional, between Discovery and Review. Discovery backends run `analyzer.Analyze()` and return confirm items. Triage step renders a split-pane UI with responsive column collapse, keyboard+mouse parity, and merges selected items into Discovery on advance.

**Tech Stack:** Go 1.23, BubbleTea, Lipgloss, Bubblezone, Vitest (N/A — Go tests)

**Design Doc:** `docs/plans/2026-04-02-tui-confirm-bucket-design.md`

---

## Phase 1: Security Foundations

Security fixes that unblock all later work. ReadFileContent path containment must be in place before the triage preview calls it with untrusted paths. SafeResolve must exist before it's used in Phase 5.

### Task 1.1 — Fix ReadFileContent path containment

**Files:**
- Modify: `cli/internal/catalog/fileinfo.go`
- Modify: `cli/internal/catalog/fileinfo_test.go`

**Depends on:** nothing

**Steps:**

1. Open `cli/internal/catalog/fileinfo.go`. Replace the body of `ReadFileContent` (lines 56–72) to add containment validation before the `os.ReadFile` call:

```go
func ReadFileContent(itemPath, relPath string, maxLines int) (string, error) {
    resolved := filepath.Clean(filepath.Join(itemPath, relPath))
    base := filepath.Clean(itemPath)
    if !strings.HasPrefix(resolved, base+string(filepath.Separator)) && resolved != base {
        return "", fmt.Errorf("path traversal: %s escapes %s", relPath, itemPath)
    }
    data, err := os.ReadFile(resolved)
    if err != nil {
        return "", fmt.Errorf("reading file %s: %w", relPath, err)
    }
    content := string(data)
    lines := strings.Split(content, "\n")
    if len(lines) > maxLines {
        extra := len(lines) - maxLines
        content = strings.Join(lines[:maxLines], "\n")
        content += fmt.Sprintf("\n\n(%d more lines)", extra)
    }
    return content, nil
}
```

Note: `strings` is already imported. No import changes needed.

2. Add test cases to `fileinfo_test.go` (or create it if it doesn't exist):

```go
func TestReadFileContent_PathTraversal(t *testing.T) {
    t.Parallel()
    dir := t.TempDir()
    // Write a file inside the base
    os.WriteFile(filepath.Join(dir, "safe.md"), []byte("ok"), 0644)
    // Write a file outside the base
    outer := t.TempDir()
    os.WriteFile(filepath.Join(outer, "secret.txt"), []byte("secret"), 0644)

    tests := []struct {
        name    string
        relPath string
        wantErr bool
    }{
        {"safe relative", "safe.md", false},
        {"traversal dotdot", "../secret.txt", true},
        {"traversal embedded", "sub/../../secret.txt", true},
        {"traversal absolute", filepath.Join(outer, "secret.txt"), true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := ReadFileContent(dir, tt.relPath, 100)
            if tt.wantErr && err == nil {
                t.Error("expected error, got nil")
            }
            if !tt.wantErr && err != nil {
                t.Errorf("unexpected error: %v", err)
            }
        })
    }
}
```

**Success Criteria:**
- `cd cli && go test ./internal/catalog/... -run TestReadFileContent` → pass
- Test with `relPath = "../../../etc/passwd"` returns non-nil error

---

### Task 1.2 — Add SafeResolve utility

**Files:**
- Create: `cli/internal/catalog/saferesolve.go`
- Create: `cli/internal/catalog/saferesolve_test.go`

**Depends on:** Task 1.1

**Steps:**

1. Create `cli/internal/catalog/saferesolve.go`:

```go
package catalog

import (
    "fmt"
    "path/filepath"
    "strings"
)

// SafeResolve joins baseDir and untrustedPath, resolves symlinks, and validates
// that the result stays within baseDir. Returns an error if the path escapes.
// This guards against both "../" traversal and symlink escape attacks.
func SafeResolve(baseDir, untrustedPath string) (string, error) {
    joined := filepath.Clean(filepath.Join(baseDir, untrustedPath))
    resolved, err := filepath.EvalSymlinks(joined)
    if err != nil {
        // If the path doesn't exist yet (e.g., destination for copy), fall back
        // to the cleaned path for containment check only.
        resolved = joined
    }
    base := filepath.Clean(baseDir)
    if !strings.HasPrefix(resolved, base+string(filepath.Separator)) && resolved != base {
        return "", fmt.Errorf("path escapes base: %s not under %s", untrustedPath, baseDir)
    }
    return resolved, nil
}
```

2. Create `cli/internal/catalog/saferesolve_test.go`:

```go
package catalog

import (
    "os"
    "path/filepath"
    "testing"
)

func TestSafeResolve(t *testing.T) {
    t.Parallel()
    dir := t.TempDir()
    os.MkdirAll(filepath.Join(dir, "sub"), 0755)
    os.WriteFile(filepath.Join(dir, "sub", "file.md"), []byte("x"), 0644)

    outer := t.TempDir()
    os.WriteFile(filepath.Join(outer, "secret.txt"), []byte("s"), 0644)

    tests := []struct {
        name      string
        base      string
        untrusted string
        wantErr   bool
    }{
        {"safe nested", dir, "sub/file.md", false},
        {"safe direct", dir, "file.md", false},
        {"traversal dotdot", dir, "../secret.txt", true},
        {"traversal absolute", dir, outer, true},
        {"traversal embedded", dir, "sub/../../secret.txt", true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := SafeResolve(tt.base, tt.untrusted)
            if tt.wantErr && err == nil {
                t.Errorf("expected error for %q", tt.untrusted)
            }
            if !tt.wantErr && err != nil {
                t.Errorf("unexpected error: %v", err)
            }
        })
    }
}

func TestSafeResolve_SymlinkEscape(t *testing.T) {
    t.Parallel()
    dir := t.TempDir()
    outer := t.TempDir()
    os.WriteFile(filepath.Join(outer, "secret.txt"), []byte("s"), 0644)
    // Create a symlink inside dir that points outside
    os.Symlink(outer, filepath.Join(dir, "escape"))

    _, err := SafeResolve(dir, "escape/secret.txt")
    if err == nil {
        t.Error("expected error for symlink escape, got nil")
    }
}
```

**Success Criteria:**
- `cd cli && go test ./internal/catalog/... -run TestSafeResolve` → pass
- Symlink escape test passes

---

## Phase 2: Metadata Schema

Add three new optional fields to `Meta`. Strip them at the import boundary.

### Task 2.1 — Add confidence fields to Meta struct

**Files:**
- Modify: `cli/internal/metadata/metadata.go`
- Modify: `cli/internal/metadata/metadata_test.go` (or create if absent)

**Depends on:** nothing

**Steps:**

1. In `cli/internal/metadata/metadata.go`, add three fields after line 65 (after `SourceProject`):

```go
    // Content-signal detection fields — scanner-computed, never read from incoming YAML.
    // Set by add.writeItem() from the actual discovery source, not from package metadata.
    Confidence      float64 `yaml:"confidence,omitempty"`
    DetectionSource string  `yaml:"detection_source,omitempty"`
    DetectionMethod string  `yaml:"detection_method,omitempty"` // "automatic" or "user-directed"
```

2. Add a roundtrip test to confirm the fields marshal/unmarshal:

```go
func TestMeta_ConfidenceRoundtrip(t *testing.T) {
    t.Parallel()
    dir := t.TempDir()
    m := &Meta{
        ID:              "test-id",
        Name:            "test",
        Confidence:      0.75,
        DetectionSource: "content-signal",
        DetectionMethod: "automatic",
    }
    if err := Save(dir, m); err != nil {
        t.Fatalf("Save: %v", err)
    }
    loaded, err := Load(dir)
    if err != nil {
        t.Fatalf("Load: %v", err)
    }
    if loaded.Confidence != 0.75 {
        t.Errorf("Confidence: got %v, want 0.75", loaded.Confidence)
    }
    if loaded.DetectionSource != "content-signal" {
        t.Errorf("DetectionSource: got %q, want content-signal", loaded.DetectionSource)
    }
    if loaded.DetectionMethod != "automatic" {
        t.Errorf("DetectionMethod: got %q, want automatic", loaded.DetectionMethod)
    }
}
```

**Success Criteria:**
- `cd cli && go test ./internal/metadata/... -run TestMeta_ConfidenceRoundtrip` → pass
- Existing metadata tests still pass

---

### Task 2.2 — Strip analyzer fields at import boundary (allowlist)

**Files:**
- Modify: `cli/internal/add/add.go`
- Modify: `cli/internal/add/add_test.go`

**Depends on:** Task 2.1

**Steps:**

1. In `cli/internal/add/add.go`, find `writeItem`. The function currently reads `item.Path` via `os.ReadFile`. After the `hash := sourceHash(raw)` line, the code reads the source file but doesn't load any source `.syllago.yaml`. The allowlist strip happens when we copy supporting files from `item.SourceDir`.

   The existing `copySupportingFiles` copies files from `item.SourceDir` to `destDir`, which may include a `.syllago.yaml` from the source package. After `copySupportingFiles` completes and before `metadata.Save` is called, we need to strip the analyzer fields from any `.syllago.yaml` that was copied.

   Locate the `copySupportingFiles` call block (around line 269–276) and add a strip call afterward:

```go
    // Copy supporting files (subdirectories, non-primary files) for directory-based items.
    if item.SourceDir != "" {
        if err := copySupportingFiles(item.SourceDir, destDir, filepath.Base(item.Path)); err != nil {
            r.Status = AddStatusError
            r.Error = fmt.Errorf("copying supporting files: %w", err)
            return r
        }
        // Strip scanner-computed fields from any .syllago.yaml copied from source.
        // These fields are set below from the actual discovery source, not from
        // attacker-controlled package metadata.
        stripAnalyzerMetadata(destDir)
    }
```

2. Add `stripAnalyzerMetadata` function in `add.go` (or a new `add_security.go` file):

```go
// stripAnalyzerMetadata zeros out scanner-computed fields on any .syllago.yaml
// that was copied from a source package. Prevents a malicious package from
// pre-setting confidence/detection_source/detection_method to influence
// tier badges, pre-check behavior, or display.
// Non-fatal: load/save errors are silently ignored (metadata.Save overwrites anyway).
func stripAnalyzerMetadata(destDir string) {
    m, err := metadata.Load(destDir)
    if err != nil || m == nil {
        return
    }
    m.Confidence = 0
    m.DetectionSource = ""
    m.DetectionMethod = ""
    _ = metadata.Save(destDir, m)
}
```

3. In `writeItem`, add confidence fields to the `meta` struct construction (after the taint propagation block, before `metadata.Save`):

```go
    // Set analyzer fields from actual discovery source (never from source package YAML).
    if item.Confidence > 0 {
        meta.Confidence = item.Confidence
        meta.DetectionSource = item.DetectionSource
        meta.DetectionMethod = item.DetectionMethod
    }
```

This requires `add.DiscoveryItem` to carry these fields — add them to the struct:

```go
// In DiscoveryItem struct (add.go or types file), add:
Confidence      float64
DetectionSource string
DetectionMethod string
```

4. Update `copySupportingFiles` in `add.go` (or wherever it lives) to use `catalog.SafeResolve` for each source file path before copying (I15). This guards against symlink escapes from untrusted packages:

```go
// In copySupportingFiles, before each os.ReadFile/copy call:
_, err := catalog.SafeResolve(srcDir, relPath)
if err != nil {
    // Skip files that escape the source root — log at debug level
    continue
}
```

Also ensure `.syllago.yaml` is skipped in the copy loop to eliminate the race window between copy and `stripAnalyzerMetadata`:

```go
if filepath.Base(relPath) == metadata.FileName {
    continue // metadata.Save writes the authoritative version
}
```

Add `"github.com/OpenScribbler/syllago/cli/internal/catalog"` to the imports in `add.go` if not already present.

5. Add a test that confirms a malicious `.syllago.yaml` with `confidence: 0.99` is stripped:

```go
func TestWriteItem_StripsMaliciousAnalyzerFields(t *testing.T) {
    t.Parallel()
    dir := t.TempDir()
    // Create source dir with a malicious .syllago.yaml
    srcDir := filepath.Join(dir, "src", "rules", "evil-rule")
    os.MkdirAll(srcDir, 0755)
    os.WriteFile(filepath.Join(srcDir, "rule.md"), []byte("# Rule"), 0644)
    // Malicious metadata with attacker-set confidence
    os.WriteFile(filepath.Join(srcDir, metadata.FileName), []byte(
        "id: evil\nname: evil-rule\nconfidence: 0.99\ndetection_method: user-directed\n"), 0644)

    destDir := filepath.Join(dir, "content")
    item := DiscoveryItem{
        Name:      "evil-rule",
        Type:      catalog.Rules,
        Path:      filepath.Join(srcDir, "rule.md"),
        SourceDir: srcDir,
        Status:    StatusNew,
        // No Confidence set on the item itself — no legitimate detection
    }
    result := writeItem(item, AddOptions{}, destDir, nil, "syllago-test")
    if result.Status == AddStatusError {
        t.Fatalf("writeItem error: %v", result.Error)
    }

    destItemDir := filepath.Join(destDir, "rules", "evil-rule")
    m, err := metadata.Load(destItemDir)
    if err != nil {
        t.Fatalf("Load: %v", err)
    }
    if m.Confidence != 0 {
        t.Errorf("Confidence should be stripped, got %v", m.Confidence)
    }
    if m.DetectionMethod != "" {
        t.Errorf("DetectionMethod should be stripped, got %q", m.DetectionMethod)
    }
}
```

**Success Criteria:**
- `cd cli && go test ./internal/add/... -run TestWriteItem_StripsMaliciousAnalyzerFields` → pass
- `cd cli && go test ./internal/add/...` → pass

---

## Phase 3: Surface 1 — Tier Badge

`TierForMeta` function in the analyzer package. Tier badge rendering in the install wizard review step.

### Task 3.1 — Add TierForMeta to analyzer package

**Files:**
- Modify: `cli/internal/analyzer/confidence.go`
- Modify: `cli/internal/analyzer/confidence_test.go` (or create)

**Depends on:** nothing (no dependency on metadata schema changes)

**Steps:**

1. Append to `cli/internal/analyzer/confidence.go`:

```go
// TierForMeta applies tier logic from persisted metadata fields.
// Uses DetectionMethod instead of floating-point equality to identify
// user-directed items — avoids fragile float comparison at the 0.60 boundary.
func TierForMeta(confidence float64, method string) ConfidenceTier {
    if method == "user-directed" {
        return TierUser
    }
    switch {
    case confidence < 0.60:
        return TierLow
    case confidence < 0.70:
        return TierMedium
    default:
        return TierHigh
    }
}
```

2. Add tests:

```go
func TestTierForMeta(t *testing.T) {
    t.Parallel()
    tests := []struct {
        name       string
        confidence float64
        method     string
        want       ConfidenceTier
    }{
        {"user-directed regardless of confidence", 0.0, "user-directed", TierUser},
        {"user-directed high confidence", 0.95, "user-directed", TierUser},
        {"low below 0.60", 0.55, "", TierLow},
        {"medium exactly 0.60", 0.60, "", TierMedium},
        {"medium at 0.65", 0.65, "", TierMedium},
        {"high at 0.70", 0.70, "", TierHigh},
        {"high at 0.85", 0.85, "", TierHigh},
        {"zero confidence no method", 0.0, "", TierLow},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := TierForMeta(tt.confidence, tt.method)
            if got != tt.want {
                t.Errorf("TierForMeta(%v, %q) = %q, want %q", tt.confidence, tt.method, got, tt.want)
            }
        })
    }
}
```

**Success Criteria:**
- `cd cli && go test ./internal/analyzer/... -run TestTierForMeta` → pass
- `grep -n "func TierForMeta" cli/internal/analyzer/confidence.go` → returns a match with the correct signature `TierForMeta(confidence float64, method string) ConfidenceTier`

---

### Task 3.2 — Tier badge in install wizard review step

**Files:**
- Modify: `cli/internal/tui/install_view.go`
- Modify: `cli/internal/tui/golden_test.go` (add two new golden tests)

**Depends on:** Task 3.1, Task 2.1

**Steps:**

1. In `cli/internal/tui/install_view.go`, add a helper to render the tier dot+label string (add near the top of the file, after imports):

```go
// tierBadge returns a colored "● Label" string for a confidence tier.
// Returns empty string for items that are not content-signal detected.
func tierBadge(tier analyzer.ConfidenceTier) string {
    dot := "●"
    switch tier {
    case analyzer.TierLow:
        return lipgloss.NewStyle().Foreground(warningColor).Render(dot + " Low confidence (content-signal)")
    case analyzer.TierMedium:
        return lipgloss.NewStyle().Foreground(primaryColor).Render(dot + " Medium confidence (content-signal)")
    case analyzer.TierHigh:
        return lipgloss.NewStyle().Foreground(successColor).Render(dot + " High confidence (content-signal)")
    case analyzer.TierUser:
        return lipgloss.NewStyle().Foreground(accentColor).Render(dot + " User-asserted (content-signal)")
    }
    return ""
}
```

Add import `"github.com/OpenScribbler/syllago/cli/internal/analyzer"` to the import block.

2. In `viewReview()`, after the `"Method:"` line is appended (around line 261), add detection line rendering:

```go
    // Detection line — only shown for content-signal items
    if m.item.Meta != nil && m.item.Meta.DetectionSource != "" {
        tier := analyzer.TierForMeta(m.item.Meta.Confidence, m.item.Meta.DetectionMethod)
        badge := tierBadge(tier)
        if badge != "" {
            summaryLines = append(summaryLines, pad+mutedStyle.Render("Detection: ")+badge)
        }
    }
```

3. Add two golden tests in `golden_test.go` (find existing golden test patterns and follow them):

```go
func TestGolden_InstallReview_TierBadge(t *testing.T) {
    // Install wizard review step showing Medium confidence tier badge
    // Set up item with DetectionSource and Confidence on Meta
    // Navigate to review step
    // requireGolden(t, "install-review-tier-badge-80x30", snapshotApp(t, app))
}

func TestGolden_InstallReview_NoTierBadge(t *testing.T) {
    // Install wizard review step with no DetectionSource — badge not shown
    // requireGolden(t, "install-review-no-tier-badge-80x30", snapshotApp(t, app))
}
```

(Full golden test implementations follow the pattern in `golden_test.go`. After implementing, run `-update-golden` to generate baselines.)

**Success Criteria:**
- `cd cli && go test ./internal/tui/... -run TestGolden_Install` → pass
- `cd cli && go build ./...` → no errors

---

## Phase 4: Triage Step Scaffolding

Step enum change, new state fields, `clearTriageState`, `buildShellLabels`, `validateStep` extension.

### Task 4.1 — Add addStepTriage to step enum and update all constants

**Files:**
- Modify: `cli/internal/tui/add_wizard.go`

**Depends on:** nothing

**Steps:**

1. In `add_wizard.go` (lines 27–33), add `addStepTriage` between `addStepDiscovery` and `addStepReview`:

```go
const (
    addStepSource    addStep = iota
    addStepType
    addStepDiscovery
    addStepTriage     // NEW — conditional, between Discovery and Review
    addStepReview
    addStepExecute
)
```

The existing `addStepReview` shifts from 3→4 and `addStepExecute` shifts from 4→5. All code references these by name so no raw-integer fixes are needed.

2. Add the triage focus zone type and new state types below the existing zone declarations (after `addReviewZone`):

```go
// --- Triage step types ---

type triageFocusZone int

const (
    triageZoneItems   triageFocusZone = iota
    triageZonePreview
    triageZoneButtons
)

// addConfirmItem holds a confirm-bucket item awaiting user triage.
type addConfirmItem struct {
    detected    *analyzer.DetectedItem
    tier        analyzer.ConfidenceTier
    displayName string
    itemType    catalog.ContentType
    path        string      // primary file path (relative to source root)
    sourceDir   string      // absolute directory containing the item
}
```

Add `"github.com/OpenScribbler/syllago/cli/internal/analyzer"` to imports in `add_wizard.go`.

3. Add new fields to `addDiscoveryItem` struct:

```go
type addDiscoveryItem struct {
    // ... existing fields ...
    confidence      float64
    detectionSource string
    tier            analyzer.ConfidenceTier
}
```

4. Add new fields to `addWizardModel` struct (after the `// Git source` block):

```go
    // Triage step
    hasTriageStep   bool
    confirmItems    []addConfirmItem
    confirmSelected map[int]bool
    confirmCursor   int
    confirmOffset   int
    confirmPreview  previewModel
    confirmFocus    triageFocusZone
```

Also add `confirmItems []addConfirmItem` to `addDiscoveryDoneMsg`:

```go
type addDiscoveryDoneMsg struct {
    seq              int
    items            []addDiscoveryItem
    confirmItems     []addConfirmItem  // NEW
    err              error
    tmpDir           string
    sourceRegistry   string
    sourceVisibility string
}
```

**Success Criteria:**
- `cd cli && go build ./...` → no errors (with any undefined references for the new methods noted)
- `grep -n "addStepTriage" cli/internal/tui/add_wizard.go` → returns at least 3 matches (enum constant definition, and the new struct/type usages)

---

### Task 4.2 — buildShellLabels, clearTriageState, updateMaxStep compatibility

**Files:**
- Modify: `cli/internal/tui/add_wizard.go`

**Depends on:** Task 4.1

**Steps:**

1. Add `buildShellLabels()` method:

```go
// buildShellLabels returns the correct step label slice for the current permutation.
// Must be called from openAddWizard, handleDiscoveryDone, and clearTriageState.
func (m *addWizardModel) buildShellLabels() []string {
    if m.preFilterType != "" && m.hasTriageStep {
        return []string{"Source", "Discovery", "Triage", "Review", "Execute"}
    }
    if m.preFilterType != "" {
        return []string{"Source", "Discovery", "Review", "Execute"}
    }
    if m.hasTriageStep {
        return []string{"Source", "Type", "Discovery", "Triage", "Review", "Execute"}
    }
    return []string{"Source", "Type", "Discovery", "Review", "Execute"}
}
```

2. Update `openAddWizard` to call `buildShellLabels()` instead of inline label construction. Replace the existing `stepLabels` block:

```go
    // hasTriageStep is false at open time; buildShellLabels handles the initial state
    m := &addWizardModel{
        // ...existing fields...
    }
    m.shell = newWizardShell("Add", m.buildShellLabels())
```

(Since `buildShellLabels` reads `m.preFilterType` and `m.hasTriageStep`, `m` must be partially constructed first. Assign preFilterType before calling buildShellLabels, or pass it as a parameter. The simplest fix: set `preFilterType` during struct literal construction, then call `SetSteps` after construction.)

3. Add `clearTriageState()` method:

```go
// clearTriageState resets all triage step state. Call from goBackFromDiscovery()
// and advanceFromSource() to prevent stale triage data surviving source changes.
func (m *addWizardModel) clearTriageState() {
    m.hasTriageStep = false
    m.confirmItems = nil
    m.confirmSelected = nil
    m.confirmCursor = 0
    m.confirmOffset = 0
    m.confirmPreview = previewModel{}
    m.confirmFocus = triageZoneItems
    // Reset maxStep to Discovery — steps beyond Discovery are invalidated when
    // triage state changes. Without this, stale maxStep allows clicking into
    // out-of-bounds breadcrumb steps.
    m.maxStep = addStepDiscovery
    m.shell.SetSteps(m.buildShellLabels())
    m.updateMaxStep()
}
```

4. Call `clearTriageState()` from `goBackFromDiscovery()` and `advanceFromSource()`:
- In `goBackFromDiscovery()`, add `m.clearTriageState()` before the step assignment.
- In `advanceFromSource()`, add `m.clearTriageState()` after `m.risks = nil`.

**Success Criteria:**
- `cd cli && go build ./...` → no errors
- `grep -n "func.*buildShellLabels\|func.*clearTriageState" cli/internal/tui/add_wizard.go` → returns both function definitions

---

### Task 4.3 — shellIndexForStep and stepForShellIndex (4-permutation)

**Files:**
- Modify: `cli/internal/tui/add_wizard.go`

**Depends on:** Task 4.1, 4.2

**Steps:**

1. Replace the existing `shellIndexForStep` with the 4-permutation version:

```go
// shellIndexForStep maps an addStep to the wizard shell breadcrumb index,
// accounting for all 4 permutations of Type and Triage step inclusion.
func (m *addWizardModel) shellIndexForStep(s addStep) int {
    // Permutation table:
    // +Type +Triage: Source(0) Type(1) Discovery(2) Triage(3) Review(4) Execute(5)
    // +Type -Triage: Source(0) Type(1) Discovery(2) Review(3) Execute(4)
    // -Type +Triage: Source(0) Discovery(1) Triage(2) Review(3) Execute(4)
    // -Type -Triage: Source(0) Discovery(1) Review(2) Execute(3)
    hasType := m.preFilterType == ""
    has := m.hasTriageStep

    switch s {
    case addStepSource:
        return 0
    case addStepType:
        if !hasType {
            panic("shellIndexForStep: addStepType in -Type permutation")
        }
        return 1
    case addStepDiscovery:
        if hasType {
            return 2
        }
        return 1
    case addStepTriage:
        if !has {
            panic("shellIndexForStep: addStepTriage in -Triage permutation")
        }
        if hasType {
            return 3
        }
        return 2
    case addStepReview:
        if hasType && has {
            return 4
        }
        if hasType || has {
            return 3
        }
        return 2
    case addStepExecute:
        if hasType && has {
            return 5
        }
        if hasType || has {
            return 4
        }
        return 3
    }
    return int(s)
}
```

2. Add the inverse `stepForShellIndex()` method:

```go
// stepForShellIndex is the inverse of shellIndexForStep.
// Used by breadcrumb click handler to map shell index → step enum.
func (m *addWizardModel) stepForShellIndex(idx int) addStep {
    hasType := m.preFilterType == ""
    hasTriage := m.hasTriageStep

    switch {
    case hasType && hasTriage:
        // Source(0) Type(1) Discovery(2) Triage(3) Review(4) Execute(5)
        return addStep(idx) // direct mapping
    case hasType && !hasTriage:
        // Source(0) Type(1) Discovery(2) Review(3) Execute(4)
        switch idx {
        case 0:
            return addStepSource
        case 1:
            return addStepType
        case 2:
            return addStepDiscovery
        case 3:
            return addStepReview
        case 4:
            return addStepExecute
        }
    case !hasType && hasTriage:
        // Source(0) Discovery(1) Triage(2) Review(3) Execute(4)
        switch idx {
        case 0:
            return addStepSource
        case 1:
            return addStepDiscovery
        case 2:
            return addStepTriage
        case 3:
            return addStepReview
        case 4:
            return addStepExecute
        }
    default:
        // -Type -Triage: Source(0) Discovery(1) Review(2) Execute(3)
        switch idx {
        case 0:
            return addStepSource
        case 1:
            return addStepDiscovery
        case 2:
            return addStepReview
        case 3:
            return addStepExecute
        }
    }
    return addStepSource
}
```

**Success Criteria:**
- `cd cli && go build ./...` → no errors
- `grep -n "func.*shellIndexForStep\|func.*stepForShellIndex" cli/internal/tui/add_wizard.go` → returns both function definitions

---

### Task 4.4 — validateStep and stepHints extensions

**Files:**
- Modify: `cli/internal/tui/add_wizard.go`

**Depends on:** Task 4.1

**Steps:**

1. Extend `validateStep()` with the triage case:

```go
    case addStepTriage:
        if !m.hasTriageStep {
            panic("wizard invariant: addStepTriage entered without hasTriageStep")
        }
        if len(m.confirmItems) == 0 {
            panic("wizard invariant: addStepTriage entered with empty confirmItems")
        }
```

2. Extend `stepHints()` with the triage case (insert before `addStepReview`):

```go
    case addStepTriage:
        return append([]string{
            "↑/↓ navigate", "space toggle", "a all", "n none",
            "tab switch panes", "enter next", "esc back",
        }, base...)
```

3. Extend `View()` switch in `add_wizard_view.go` with `case addStepTriage: content = m.viewTriage()` (stub for now — full implementation in Phase 6).

**Success Criteria:**
- `cd cli && go build ./...` → no errors
- `grep -n "addStepTriage" cli/internal/tui/add_wizard.go` → returns a match in both `validateStep()` and `stepHints()` case blocks

---

## Phase 5: Discovery Backend Wiring

Wire `analyzer.Analyze()` into the three relevant backends (registry, local, git) and return confirm items. Provider backend uses pattern-only (no analyzer).

### Task 5.1 — discoverFromLocalPath backend

**Files:**
- Modify: `cli/internal/tui/add_wizard.go`

**Depends on:** Task 4.1, Task 1.2

**Steps:**

1. Find `discoverFromLocalPath` in `add_wizard.go`. Change its return signature:

```go
func discoverFromLocalPath(dir string, types []catalog.ContentType, contentRoot string,
) ([]addDiscoveryItem, []addConfirmItem, error)
```

2. After the existing pattern-based discovery returns its items, add the analyzer pass. Build a dedup set from pattern-detected items, run analyzer, merge Auto items (dedup), return Confirm items:

```go
    // Build dedup set from pattern-detected paths
    patternPaths := make(map[string]bool, len(items))
    for _, it := range items {
        patternPaths[filepath.ToSlash(filepath.Clean(it.path))] = true
    }

    // Run analyzer (content-signal fallback)
    typeSet := make(map[catalog.ContentType]bool)
    for _, t := range types {
        typeSet[t] = true
    }

    az := analyzer.New(analyzer.DefaultConfig())
    result, azErr := az.Analyze(dir)
    var toastMsg string
    if azErr != nil {
        toastMsg = "Content analysis unavailable — showing pattern-detected items only."
        // Fall through: return pattern items with empty confirm slice + toast
        return items, nil, nil // push toast separately via cmd (handled in handleDiscoveryDone)
    }

    // Merge Auto items (dedup, type-filtered)
    for _, detected := range result.Auto {
        if !typeSet[detected.Type] {
            continue
        }
        canon := filepath.ToSlash(filepath.Clean(detected.Path))
        if patternPaths[canon] {
            continue // pattern-detected wins
        }
        patternPaths[canon] = true
        di := addDiscoveryItem{
            name:            detected.DisplayName,
            itemType:        detected.Type,
            path:            filepath.Join(dir, detected.Path),
            sourceDir:       filepath.Join(dir, filepath.Dir(detected.Path)),
            status:          add.StatusNew,
            detectionSource: "content-signal",
            tier:            analyzer.TierForItem(detected),
        }
        items = append(items, di)
    }

    // Build Confirm items (type-filtered)
    var confirmItems []addConfirmItem
    for _, detected := range result.Confirm {
        if !typeSet[detected.Type] {
            continue
        }
        name := detected.DisplayName
        if name == "" {
            name = detected.Name
        }
        ci := addConfirmItem{
            detected:    detected,
            tier:        analyzer.TierForItem(detected),
            displayName: name,
            itemType:    detected.Type,
            path:        detected.Path,
            sourceDir:   filepath.Join(dir, filepath.Dir(detected.Path)),
        }
        confirmItems = append(confirmItems, ci)
    }

    // If azErr != nil (early-returned above), toastMsg is set but not yet pushed.
    // The toast is pushed in Task 5.5 via addDiscoveryDoneMsg.analyzerToast field — see Task 5.5.
    _ = toastMsg
    return items, confirmItems, nil
```

Note: the early-return path (`return items, nil, nil`) when `azErr != nil` does not carry the toast message back to the caller. Task 5.5 wires a `analyzerToast string` field on `addDiscoveryDoneMsg` so that `handleDiscoveryDone` can push the toast via `pushToast`. Each backend that catches an analyzer error must populate this field instead of silently dropping the message. Apply the same pattern in Tasks 5.2 and 5.3.

3. Update all callers of `discoverFromLocalPath` to accept the new return values and propagate `confirmItems` into `addDiscoveryDoneMsg`.

**Success Criteria:**
- `cd cli && go build ./...` → no errors
- `grep -n "func discoverFromLocalPath" cli/internal/tui/add_wizard.go` → return signature includes `[]addConfirmItem` as second return value

---

### Task 5.2 — discoverFromGitURL backend

**Files:**
- Modify: `cli/internal/tui/add_wizard.go`

**Depends on:** Task 5.1 (same pattern)

**Steps:**

Change `discoverFromGitURL` return signature to include `[]addConfirmItem`. Apply the same analyzer pass pattern as Task 5.1, using the git clone temp dir as the `dir` argument to `az.Analyze()`.

Dedup must also call `filepath.EvalSymlinks` on paths before comparison, since git clones don't have symlinks (but the pattern is the same for consistency).

```go
func discoverFromGitURL(url string, types []catalog.ContentType, contentRoot string,
) ([]addDiscoveryItem, []addConfirmItem, string, error)
```

**Success Criteria:**
- `cd cli && go build ./...` → no errors
- `grep -n "func discoverFromGitURL" cli/internal/tui/add_wizard.go` → return signature includes `[]addConfirmItem` as second return value

---

### Task 5.3 — discoverFromRegistry backend

**Files:**
- Modify: `cli/internal/tui/add_wizard.go`

**Depends on:** Task 5.1

**Steps:**

Change `discoverFromRegistry` return signature to include `[]addConfirmItem`. Apply the same analyzer pass pattern. Registry clone dir is the `dir` argument.

```go
func discoverFromRegistry(reg catalog.RegistrySource, types []catalog.ContentType, contentRoot string,
) ([]addDiscoveryItem, []addConfirmItem, error)
```

**Success Criteria:**
- `cd cli && go build ./...` → no errors
- `grep -n "func discoverFromRegistry" cli/internal/tui/add_wizard.go` → return signature includes `[]addConfirmItem` as second return value

---

### Task 5.4 — Provider backend: pattern-only (no analyzer)

**Files:**
- Modify: `cli/internal/tui/add_wizard.go`

**Depends on:** Task 4.1

**Steps:**

Change `discoverFromProvider` return signature to include `[]addConfirmItem` (always nil for provider source):

```go
func discoverFromProvider(prov provider.Provider, projectRoot string, cfg *config.Config,
    contentRoot string, selectedTypes []catalog.ContentType,
) ([]addDiscoveryItem, []addConfirmItem, error)
```

Return `nil, nil, err` on error and `items, nil, nil` on success. No analyzer call — provider directories use pattern detection only (I11: content-signal fallback disabled for provider sources).

**Success Criteria:**
- `cd cli && go build ./...` → no errors
- `grep -n "func discoverFromProvider" cli/internal/tui/add_wizard.go` → return signature includes `[]addConfirmItem` and function body contains no `az.Analyze` call

---

### Task 5.5 — Wire confirm items through handleDiscoveryDone

**Files:**
- Modify: `cli/internal/tui/add_wizard_update.go`

**Depends on:** Tasks 5.1–5.4, Task 4.2

**Steps:**

1. Update `startDiscoveryCmd` (in `add_wizard.go`) to call the updated backends and populate `addDiscoveryDoneMsg.confirmItems`. Also add `analyzerToast string` to `addDiscoveryDoneMsg` and have backends that catch analyzer errors set it:

```go
type addDiscoveryDoneMsg struct {
    seq              int
    items            []addDiscoveryItem
    confirmItems     []addConfirmItem  // from Phase 4
    analyzerToast    string            // NEW — non-empty when analyzer unavailable
    err              error
    tmpDir           string
    sourceRegistry   string
    sourceVisibility string
}
```

In each backend (local, git, registry), when `azErr != nil`, instead of `return items, nil, nil`, return:
```go
return items, nil, "Content analysis unavailable — showing pattern-detected items only.", nil
// (signature updated to return toast as 3rd string value, error as 4th)
```

2. In `handleDiscoveryDone` (in `add_wizard_update.go`), after setting `m.discoveredItems`, push the analyzer toast if present, then process confirm items:

```go
    // Push analyzer unavailable toast if content-signal failed
    if msg.analyzerToast != "" {
        cmds = append(cmds, pushToast(msg.analyzerToast, toastWarn))
    }
```

```go
    // Handle triage step activation
    if len(msg.confirmItems) > 0 {
        m.hasTriageStep = true
        m.confirmItems = msg.confirmItems
        // Initialize selection: High + User pre-checked, Medium + Low unchecked
        m.confirmSelected = make(map[int]bool, len(msg.confirmItems))
        for i, ci := range msg.confirmItems {
            m.confirmSelected[i] = ci.tier == analyzer.TierHigh || ci.tier == analyzer.TierUser
        }
        m.confirmCursor = 0
        m.confirmOffset = 0
        m.confirmFocus = triageZoneItems
    } else {
        m.hasTriageStep = false
        m.confirmItems = nil
        m.confirmSelected = nil
    }
    // Rebuild shell labels now that hasTriageStep is determined
    m.shell.SetSteps(m.buildShellLabels())
    m.updateMaxStep()
```

3. Update breadcrumb click handler in `updateMouse` to use `stepForShellIndex` instead of the existing raw switch. Replace the existing block:

```go
    if step, ok := m.shell.HandleClick(msg); ok {
        target := m.stepForShellIndex(step)
        if target == addStepExecute {
            return m, nil
        }
        if target != m.step && int(target) <= int(m.maxStep) {
            m.step = target
            m.shell.SetActive(step)
            m.reviewAcknowledged = false
        }
        return m, nil
    }
```

**Success Criteria:**
- `cd cli && go build ./...` → no errors
- `cd cli && go test ./internal/tui/...` → pass

---

## Phase 6: Triage Step UI

View rendering, keyboard handler, mouse handler, responsive columns.

### Task 6.1 — viewTriage() rendering

**Files:**
- Modify: `cli/internal/tui/add_wizard_view.go`

**Depends on:** Tasks 4.1–4.4

**Steps:**

1. Add `viewTriage()` method. This is the full split-pane view. Implement in `add_wizard_view.go`:

```go
func (m *addWizardModel) viewTriage() string {
    var lines []string
    titleRow := m.renderTitleRow("Triage: detected content", true, "Next")
    lines = append(lines, titleRow)
    lines = append(lines, "")
    lines = append(lines, "  Include items to add them, or skip (skipped items reappear next time you add from this source).")
    lines = append(lines, "")

    // Compute pane dimensions
    innerW := m.width - 2             // -2 for outer border chars
    leftPct := 30
    leftW := max(18, innerW*leftPct/100)
    rightW := innerW - leftW - 1      // -1 for the split char

    // Content height: total - header(4) - shell(3) - bottom border(2)
    contentH := max(3, m.height-9)
    itemH := contentH - 2 // -2 for top/bottom border lines

    // Render left pane items
    leftLines := m.renderTriageItems(leftW-2, itemH) // -2 for left border padding
    // Render right pane preview
    rightLines := m.renderTriagePreview(rightW-2, itemH)

    // Top border: ╭─ Items ──┬─ Preview ──╮
    topBorder := mutedStyle.Render("╭─ Items " +
        strings.Repeat("─", leftW-9) + "┬─ Preview " +
        strings.Repeat("─", rightW-11) + "╮")
    lines = append(lines, topBorder)

    // Content rows
    for i := 0; i < itemH; i++ {
        left := ""
        right := ""
        if i < len(leftLines) {
            left = leftLines[i]
        }
        if i < len(rightLines) {
            right = rightLines[i]
        }
        // Pad left to leftW
        leftPadded := left + strings.Repeat(" ", max(0, leftW-2-lipgloss.Width(left)))
        row := mutedStyle.Render("│") + " " + leftPadded + mutedStyle.Render("│") +
            " " + right + mutedStyle.Render("│")
        lines = append(lines, row)
    }

    // Separator + buttons
    sep := mutedStyle.Render("├" + strings.Repeat("─", leftW) + "┴" + strings.Repeat("─", rightW) + "┤")
    lines = append(lines, sep)

    btnLine := "  " + zone.Mark("triage-cancel",
        renderButton("Cancel", false)) + "  " +
        zone.Mark("triage-back", renderButton("Back", false)) + "  " +
        zone.Mark("triage-next", renderButton("Next", true))
    lines = append(lines, btnLine)

    botBorder := mutedStyle.Render("╰" + strings.Repeat("─", innerW) + "╯")
    lines = append(lines, botBorder)

    return strings.Join(lines, "\n")
}
```

2. Add `renderTriageItems(availW, maxH int) []string` helper that applies responsive column collapse:

```go
func (m *addWizardModel) renderTriageItems(availW, maxH int) []string {
    var lines []string
    offset := m.confirmOffset
    visible := min(maxH, len(m.confirmItems)-offset)

    for i := 0; i < visible; i++ {
        idx := offset + i
        item := m.confirmItems[idx]

        checked := m.confirmSelected[idx]
        isCursor := idx == m.confirmCursor

        prefix := " "
        if isCursor {
            prefix = ">"
        }
        if checked {
            prefix = "✓"
        }

        name := item.displayName
        tierDot, tierLabel := tierDotLabel(item.tier)

        var row string
        switch {
        case availW >= 30:
            // Full: prefix + name + type + "● Label"
            typeLabel := fmt.Sprintf("%-7s", shortTypeLabel(item.itemType))
            tierStr := tierDot + " " + tierLabel
            overhead := 2 + 1 + 7 + 1 + len(tierStr) // prefix+space + type + space + tier
            nameW := max(4, availW-overhead)
            row = prefix + " " + truncate(name, nameW) + " " + typeLabel + " " + tierStr
        case availW >= 24:
            // Medium: prefix + name + "● Label"
            tierStr := tierDot + " " + tierLabel
            overhead := 2 + 1 + len(tierStr)
            nameW := max(4, availW-overhead)
            row = prefix + " " + truncate(name, nameW) + " " + tierStr
        default:
            // Narrow: prefix + name + "● X" (single letter)
            tierStr := tierDot + " " + tierAbbrev(item.tier)
            overhead := 2 + 1 + len(tierStr)
            nameW := max(4, availW-overhead)
            row = prefix + " " + truncate(name, nameW) + " " + tierStr
        }

        if isCursor {
            row = lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render(row)
        } else if !checked {
            row = mutedStyle.Render(row)
        }

        zoneID := fmt.Sprintf("triage-item-%d", idx)
        lines = append(lines, zone.Mark(zoneID, row))
    }
    return lines
}
```

3. Add helper functions:

```go
func tierDotLabel(t analyzer.ConfidenceTier) (dot, label string) {
    switch t {
    case analyzer.TierHigh:
        return lipgloss.NewStyle().Foreground(successColor).Render("●"), "High"
    case analyzer.TierMedium:
        return lipgloss.NewStyle().Foreground(primaryColor).Render("●"), "Medium"
    case analyzer.TierLow:
        return lipgloss.NewStyle().Foreground(warningColor).Render("●"), "Low"
    case analyzer.TierUser:
        return lipgloss.NewStyle().Foreground(accentColor).Render("●"), "User"
    }
    return "●", "?"
}

func tierAbbrev(t analyzer.ConfidenceTier) string {
    switch t {
    case analyzer.TierHigh:
        return "H"
    case analyzer.TierMedium:
        return "M"
    case analyzer.TierLow:
        return "L"
    case analyzer.TierUser:
        return "U"
    }
    return "?"
}

func shortTypeLabel(ct catalog.ContentType) string {
    switch ct {
    case catalog.Skills:
        return "Skill"
    case catalog.Agents:
        return "Agent"
    case catalog.Rules:
        return "Rule"
    case catalog.Hooks:
        return "Hook"
    case catalog.MCP:
        return "MCP"
    case catalog.Commands:
        return "Command"
    }
    return string(ct)
}
```

4. Add `renderTriagePreview(availW, maxH int) []string` that delegates to `m.confirmPreview`:

```go
func (m *addWizardModel) renderTriagePreview(availW, maxH int) []string {
    if len(m.confirmItems) == 0 {
        return nil
    }
    // Load preview content for focused item if not already loaded
    content := m.confirmPreview.View()
    lines := strings.Split(content, "\n")
    if len(lines) > maxH {
        lines = lines[:maxH]
    }
    // Pad/truncate each line to availW
    result := make([]string, maxH)
    for i := 0; i < maxH; i++ {
        if i < len(lines) {
            result[i] = truncate(lines[i], availW)
        } else {
            result[i] = ""
        }
    }
    return result
}
```

5. Update the discovery step view (`viewDiscovery()` in `add_wizard_view.go`) to render a provenance indicator for Auto items merged from content-signal. When an `addDiscoveryItem` has `detectionSource == "content-signal"`, append a styled `(detected)` suffix to the item name column (I6):

```go
// In the item row rendering loop inside viewDiscovery (or its renderDiscoveryRow helper):
nameSuffix := ""
if item.detectionSource == "content-signal" {
    nameSuffix = " " + mutedStyle.Render("(detected)")
}
```

Apply this wherever the item name is rendered in the discovery list rows. The suffix must be included in the width calculation so it doesn't overflow the column. If the discovery list uses a reusable render helper, add the suffix parameter there.

**Success Criteria:**
- `cd cli && go build ./...` → no errors
- TUI renders triage step without panicking when `hasTriageStep = true`
- Discovery step view shows `(detected)` suffix on items with `detectionSource == "content-signal"`

---

### Task 6.2 — Preview model wiring for triage

**Files:**
- Modify: `cli/internal/tui/add_wizard.go` (add helper)
- Modify: `cli/internal/tui/add_wizard_update.go` (update preview on cursor change)

**Depends on:** Task 6.1, Task 1.1

**Steps:**

1. Add `loadTriagePreview()` helper that reads the focused item's primary file using `catalog.ReadFileContent` with `SafeResolve` guard:

```go
func (m *addWizardModel) loadTriagePreview() {
    if m.confirmCursor >= len(m.confirmItems) {
        return
    }
    item := m.confirmItems[m.confirmCursor]
    if item.sourceDir == "" || item.path == "" {
        return
    }
    // Use SafeResolve to guard against path traversal from untrusted repo
    _, err := catalog.SafeResolve(item.sourceDir, item.path)
    if err != nil {
        m.confirmPreview = newPreviewModel()
        m.confirmPreview.lines = []string{"(preview unavailable: " + err.Error() + ")"}
        return
    }
    content, readErr := catalog.ReadFileContent(item.sourceDir, item.path, 200)
    if readErr != nil {
        m.confirmPreview = newPreviewModel()
        m.confirmPreview.lines = []string{"(preview unavailable)"}
        return
    }
    m.confirmPreview = newPreviewModel()
    m.confirmPreview.lines = strings.Split(content, "\n")
    m.confirmPreview.fileName = filepath.Base(item.path)
}
```

2. Call `loadTriagePreview()` whenever `m.confirmCursor` changes and when the triage step is first entered (in `handleDiscoveryDone` after setting `m.confirmCursor = 0`).

**Success Criteria:**
- `cd cli && go build ./...` → no errors
- `grep -n "func.*loadTriagePreview\|SafeResolve" cli/internal/tui/add_wizard.go` → returns both the function definition and the SafeResolve guard call within it

---

### Task 6.3 — Keyboard handler for triage step

**Files:**
- Modify: `cli/internal/tui/add_wizard_update.go`

**Depends on:** Task 6.1, 6.2

**Steps:**

1. Add `case addStepTriage:` to `updateKey()`:

```go
    case addStepTriage:
        return m.updateKeyTriage(msg)
```

2. Implement `updateKeyTriage()`:

```go
func (m *addWizardModel) updateKeyTriage(msg tea.KeyMsg) (*addWizardModel, tea.Cmd) {
    switch {
    case msg.Type == tea.KeyEsc:
        // Back to Discovery — clearTriageState not called here (preserve selections
        // in case user navigates forward again; hasTriageStep stays true)
        m.step = addStepDiscovery
        m.shell.SetActive(m.shellIndexForStep(addStepDiscovery))
        return m, nil

    case msg.Type == tea.KeyEnter:
        // Advance to Review (from any focus zone — no drill-in)
        m.mergeConfirmIntoDiscovery()
        m.enterReview()
        return m, nil

    case msg.Type == tea.KeyTab:
        m.confirmFocus = (m.confirmFocus + 1) % 3
        return m, nil

    case msg.Type == tea.KeyShiftTab:
        m.confirmFocus = (m.confirmFocus + 2) % 3 // wrap backward
        return m, nil

    case msg.Type == tea.KeyUp:
        if m.confirmCursor > 0 {
            m.confirmCursor--
            m.adjustTriageOffset()
            m.loadTriagePreview()
        }

    case msg.Type == tea.KeyDown:
        if m.confirmCursor < len(m.confirmItems)-1 {
            m.confirmCursor++
            m.adjustTriageOffset()
            m.loadTriagePreview()
        }

    case msg.Type == tea.KeySpace:
        // Sole toggle key — consistent with Discovery step checkboxes
        m.confirmSelected[m.confirmCursor] = !m.confirmSelected[m.confirmCursor]

    case msg.Type == tea.KeyRunes && len(msg.Runes) == 1:
        switch msg.Runes[0] {
        case 'a':
            for i := range m.confirmItems {
                m.confirmSelected[i] = true
            }
        case 'n':
            for i := range m.confirmItems {
                m.confirmSelected[i] = false
            }
        }
    }
    return m, nil
}

// adjustTriageOffset keeps the cursor visible in the items pane.
func (m *addWizardModel) adjustTriageOffset() {
    // Visible height: total - header(4) - shell(3) - border(2) = total - 9
    vh := max(3, m.height-9-2) // -2 for border
    if m.confirmCursor < m.confirmOffset {
        m.confirmOffset = m.confirmCursor
    }
    if m.confirmCursor >= m.confirmOffset+vh {
        m.confirmOffset = m.confirmCursor - vh + 1
    }
}
```

**Success Criteria:**
- `cd cli && go build ./...` → no errors
- `grep -n "func.*updateKeyTriage\|case addStepTriage" cli/internal/tui/add_wizard_update.go` → returns both the `updateKeyTriage` function definition and the dispatch `case addStepTriage:` in `updateKey()`

---

### Task 6.4 — Mouse handler for triage step

**Files:**
- Modify: `cli/internal/tui/add_wizard_update.go`

**Depends on:** Task 6.3

**Steps:**

1. Add `case addStepTriage:` to the `updateMouse` per-step switch:

```go
    case addStepTriage:
        return m.updateMouseTriage(msg)
```

2. Add `case addStepTriage:` to `updateMouseWheel`:

```go
    case addStepTriage:
        if up {
            if m.confirmCursor > 0 {
                m.confirmCursor--
                m.adjustTriageOffset()
                m.loadTriagePreview()
            }
        } else {
            if m.confirmCursor < len(m.confirmItems)-1 {
                m.confirmCursor++
                m.adjustTriageOffset()
                m.loadTriagePreview()
            }
        }
```

3. Implement `updateMouseTriage()`:

```go
func (m *addWizardModel) updateMouseTriage(msg tea.MouseMsg) (*addWizardModel, tea.Cmd) {
    // Item row clicks: focus + toggle
    for i := range m.confirmItems {
        zoneID := fmt.Sprintf("triage-item-%d", i)
        if zone.Get(zoneID).InBounds(msg) {
            m.confirmCursor = i
            m.adjustTriageOffset()
            m.loadTriagePreview()
            m.confirmSelected[i] = !m.confirmSelected[i]
            return m, nil
        }
    }

    // Preview pane click: shift focus
    if zone.Get("triage-preview").InBounds(msg) {
        m.confirmFocus = triageZonePreview
        return m, nil
    }

    // Button clicks
    if zone.Get("triage-cancel").InBounds(msg) {
        return m, func() tea.Msg { return addCloseMsg{} }
    }
    if zone.Get("triage-back").InBounds(msg) {
        return m.updateKey(tea.KeyMsg{Type: tea.KeyEsc})
    }
    if zone.Get("triage-next").InBounds(msg) {
        return m.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
    }

    return m, nil
}
```

**Success Criteria:**
- `cd cli && go build ./...` → no errors
- Every zone-marked element in `viewTriage()` has a corresponding `zone.Get().InBounds()` check

---

## Phase 7: Triage-to-Review Merge

`mergeConfirmIntoDiscovery` with idempotency. Back-navigation from Review respects `hasTriageStep`.

### Task 7.1 — mergeConfirmIntoDiscovery

**Files:**
- Modify: `cli/internal/tui/add_wizard.go`

**Depends on:** Tasks 4.1, 5.5

**Steps:**

Add the merge function (after `enterReview`):

```go
// mergeConfirmIntoDiscovery appends user-selected confirm items to the discovery
// list before entering Review. Safe to call multiple times — truncates to pre-merge
// length first (B2: idempotency guarantee for Back→Triage→Next flows).
func (m *addWizardModel) mergeConfirmIntoDiscovery() {
    // Strip any previously-merged confirm items to prevent duplicates
    m.discoveredItems = m.discoveredItems[:m.actionableCount+m.installedCount]

    for i, item := range m.confirmItems {
        if !m.confirmSelected[i] {
            continue
        }
        di := addDiscoveryItem{
            name:            item.displayName,
            displayName:     item.displayName,
            itemType:        item.itemType,
            path:            filepath.Join(item.sourceDir, item.path),
            sourceDir:       item.sourceDir,
            status:          add.StatusNew,
            detectionSource: "content-signal",
            confidence:      item.detected.Confidence,
            tier:            item.tier,
        }
        if item.detected != nil {
            ci := catalog.ContentItem{
                Name:  item.displayName,
                Type:  item.itemType,
                Path:  item.sourceDir,
                Files: []string{item.path},
            }
            di.catalogItem = &ci
            di.risks = catalog.RiskIndicators(ci)
        }
        m.discoveredItems = append(m.discoveredItems, di)
    }

    // Rebuild discovery list and auto-select merged items
    m.discoveryList = m.buildDiscoveryList()
    mergeStart := m.actionableCount + m.installedCount
    for i := mergeStart; i < len(m.discoveredItems); i++ {
        m.discoveryList.selected[i] = true
    }
}
```

**Success Criteria:**
- `cd cli && go build ./...` → no errors
- `grep -n "func.*mergeConfirmIntoDiscovery" cli/internal/tui/add_wizard.go` → returns the function definition, and body contains the idempotency truncation `m.discoveredItems[:m.actionableCount+m.installedCount]`

---

### Task 7.2 — Back-navigation from Review respects hasTriageStep

**Files:**
- Modify: `cli/internal/tui/add_wizard_update.go`

**Depends on:** Task 4.1

**Steps:**

1. In `updateKeyReview()`, change the `tea.KeyEsc` case from hardcoded `addStepDiscovery` to:

```go
    case tea.KeyEsc:
        if m.hasTriageStep {
            m.step = addStepTriage
            m.shell.SetActive(m.shellIndexForStep(addStepTriage))
        } else {
            m.step = addStepDiscovery
            m.shell.SetActive(m.shellIndexForStep(addStepDiscovery))
        }
        m.reviewAcknowledged = false
        return m, nil
```

2. In `updateMouseReview()`, find the `"add-back"` zone handler (around line 352–354) and apply the same fix:

```go
    if zone.Get("add-back").InBounds(msg) {
        if m.hasTriageStep {
            m.step = addStepTriage
            m.shell.SetActive(m.shellIndexForStep(addStepTriage))
        } else {
            m.step = addStepDiscovery
            m.shell.SetActive(m.shellIndexForStep(addStepDiscovery))
        }
        m.reviewAcknowledged = false
        return m, nil
    }
```

3. Check for any other hardcoded `m.step = addStepDiscovery` in review handlers (lines 901, etc.) and apply the same pattern where appropriate.

**Success Criteria:**
- `cd cli && go build ./...` → no errors
- `grep -n "hasTriageStep" cli/internal/tui/add_wizard_update.go` → returns matches in both `updateKeyReview()` and `updateMouseReview()` back-navigation handlers

---

## Phase 8: Navigation & Breadcrumbs

Update all shell index callers to use `stepForShellIndex`. Fix existing tests that use raw shell index values.

### Task 8.1 — Audit and fix all shell.SetActive calls

**Files:**
- Modify: `cli/internal/tui/add_wizard.go`
- Modify: `cli/internal/tui/add_wizard_update.go`

**Depends on:** Task 4.3

**Steps:**

1. Search for all `shell.SetActive(` calls in the add wizard files:

```bash
grep -n "shell.SetActive" cli/internal/tui/add_wizard*.go
```

2. For any `shell.SetActive(N)` call where `N` is a literal integer (not `shellIndexForStep(...)`), replace it with `m.shell.SetActive(m.shellIndexForStep(addStep...))`. The only legitimate literal-integer calls are those in `clearTriageState()` and `openAddWizard()` before step enums are available.

3. Verify `openAddWizard` initializes step correctly — it should call `m.shell.SetActive(0)` (Source is always index 0) or use `shellIndexForStep(addStepSource)`.

**Success Criteria:**
- `cd cli && go build ./...` → no errors
- `grep -n "shell.SetActive([0-9])" cli/internal/tui/add_wizard*.go` returns only expected literal-0 calls

---

### Task 8.2 — Update TestAddWizard forward-path test shell indices

**Files:**
- Modify: `cli/internal/tui/wizard_invariant_test.go`

**Depends on:** Task 4.1, 4.3

**Steps:**

The existing `TestAddWizard_ValidateStep_Forward` test uses `m.shell.SetActive(3)` for Review and `m.shell.SetActive(4)` for Execute. With the new enum, these must be updated:

```go
    // Step 3 (now addStepTriage skipped — no confirm items, so no triage step)
    // Step 4 -> addStepReview with shell index 3 (no-Type, no-Triage path)
    m.step = addStepReview
    m.shell.SetActive(m.shellIndexForStep(addStepReview))
    m.validateStep()

    // Step 5 -> addStepExecute
    m.reviewAcknowledged = true
    m.step = addStepExecute
    m.shell.SetActive(m.shellIndexForStep(addStepExecute))
    m.validateStep()
```

Replace all literal `shell.SetActive(N)` calls in add wizard invariant tests with `m.shellIndexForStep(addStep...)` calls.

**Success Criteria:**
- `cd cli && go test ./internal/tui/... -run TestAddWizard_ValidateStep_Forward` → pass
- `grep -n "shell.SetActive([0-9])" cli/internal/tui/wizard_invariant_test.go` → returns no matches (all literal-integer calls replaced with shellIndexForStep)

---

## Phase 9: Tests

Wizard invariant tests, golden files, integration tests.

### Task 9.1 — Wizard invariant tests for triage path

**Files:**
- Modify: `cli/internal/tui/wizard_invariant_test.go`

**Depends on:** Tasks 4.1–4.4, Phase 7

**Steps:**

Add the following test cases:

```go
func TestAddWizard_ValidateStep_TriageForward(t *testing.T) {
    t.Parallel()
    // Walk through all 6 steps with triage active
    m := openAddWizard(
        []provider.Provider{testInstallProvider("Claude Code", "claude-code", true)},
        nil, nil, "/tmp", "/tmp", "",
    )
    m.source = addSourceLocal

    // Source → Type
    m.step = addStepSource
    m.validateStep()

    m.source = addSourceLocal
    m.step = addStepType
    m.shell.SetActive(m.shellIndexForStep(addStepType))
    m.validateStep()

    // Type → Discovery
    m.typeChecks = m.buildTypeCheckList()
    m.step = addStepDiscovery
    m.discovering = true
    m.shell.SetActive(m.shellIndexForStep(addStepDiscovery))
    m.validateStep()

    // Discovery done with confirm items → Triage
    m.discovering = false
    m.hasTriageStep = true
    m.confirmItems = []addConfirmItem{
        {displayName: "test-agent", itemType: catalog.Agents,
         tier: analyzer.TierMedium, path: "agents/test.md", sourceDir: "/tmp"},
    }
    m.confirmSelected = map[int]bool{0: false}
    m.shell.SetSteps(m.buildShellLabels())
    m.step = addStepTriage
    m.shell.SetActive(m.shellIndexForStep(addStepTriage))
    m.validateStep() // must not panic

    // Triage → Review (merge first)
    m.discoveredItems = []addDiscoveryItem{
        {name: "test", itemType: catalog.Rules, status: add.StatusNew,
         underlying: &add.DiscoveryItem{Name: "test", Type: catalog.Rules}},
    }
    m.discoveryList = m.buildDiscoveryList()
    m.step = addStepReview
    m.shell.SetActive(m.shellIndexForStep(addStepReview))
    m.validateStep()

    // Review → Execute
    m.reviewAcknowledged = true
    m.step = addStepExecute
    m.shell.SetActive(m.shellIndexForStep(addStepExecute))
    m.validateStep()
}

func TestAddWizard_ValidateStep_TriageEsc(t *testing.T) {
    t.Parallel()
    // Esc from Triage goes back to Discovery (not Source)
    m := openAddWizard(nil, nil, nil, "/tmp", "/tmp", "")
    m.source = addSourceLocal
    m.typeChecks = m.buildTypeCheckList()
    m.hasTriageStep = true
    m.confirmItems = []addConfirmItem{
        {displayName: "x", itemType: catalog.Rules, tier: analyzer.TierLow,
         path: "rules/x.md", sourceDir: "/tmp"},
    }
    m.confirmSelected = map[int]bool{0: false}
    m.shell.SetSteps(m.buildShellLabels())

    m.step = addStepTriage
    m.shell.SetActive(m.shellIndexForStep(addStepTriage))
    m.validateStep()

    // Simulate Esc
    m.step = addStepDiscovery
    m.shell.SetActive(m.shellIndexForStep(addStepDiscovery))
    m.validateStep() // should not panic
}

func TestAddWizard_ValidateStep_SkipTriageWhenEmpty(t *testing.T) {
    t.Parallel()
    // No confirm items → hasTriageStep false → should go Source→Type→Discovery→Review
    m := openAddWizard(nil, nil, nil, "/tmp", "/tmp", "")
    m.source = addSourceLocal
    m.typeChecks = m.buildTypeCheckList()
    m.hasTriageStep = false

    m.step = addStepDiscovery
    m.discovering = false
    m.discoveredItems = []addDiscoveryItem{
        {name: "test", itemType: catalog.Rules, status: add.StatusNew,
         underlying: &add.DiscoveryItem{Name: "test", Type: catalog.Rules}},
    }
    m.discoveryList = m.buildDiscoveryList()
    m.step = addStepReview
    m.shell.SetActive(m.shellIndexForStep(addStepReview))
    m.validateStep() // should not panic — triage was skipped
}

func TestAddWizard_ConfirmItemsParallelArray(t *testing.T) {
    t.Parallel()
    // confirmItems and confirmSelected must stay in sync
    m := openAddWizard(nil, nil, nil, "/tmp", "/tmp", "")
    m.confirmItems = []addConfirmItem{
        {displayName: "a", tier: analyzer.TierHigh},
        {displayName: "b", tier: analyzer.TierMedium},
        {displayName: "c", tier: analyzer.TierLow},
    }
    m.confirmSelected = make(map[int]bool, len(m.confirmItems))
    for i, ci := range m.confirmItems {
        m.confirmSelected[i] = ci.tier == analyzer.TierHigh || ci.tier == analyzer.TierUser
    }
    if len(m.confirmSelected) != len(m.confirmItems) {
        t.Errorf("confirmSelected len %d != confirmItems len %d",
            len(m.confirmSelected), len(m.confirmItems))
    }
    // Verify pre-check: High checked, Medium and Low not
    if !m.confirmSelected[0] {
        t.Error("High should be pre-checked")
    }
    if m.confirmSelected[1] {
        t.Error("Medium should not be pre-checked")
    }
    if m.confirmSelected[2] {
        t.Error("Low should not be pre-checked")
    }
}

func TestAddWizard_MergeIdempotency(t *testing.T) {
    t.Parallel()
    m := openAddWizard(nil, nil, nil, "/tmp", "/tmp", "")
    m.source = addSourceLocal
    m.typeChecks = m.buildTypeCheckList()
    m.hasTriageStep = true
    m.confirmItems = []addConfirmItem{
        {displayName: "detected-agent", itemType: catalog.Agents,
         tier: analyzer.TierHigh, path: "agents/x.md",
         sourceDir: t.TempDir(),
         detected: &analyzer.DetectedItem{Confidence: 0.85}},
    }
    m.confirmSelected = map[int]bool{0: true}
    m.discoveredItems = []addDiscoveryItem{
        {name: "pattern-rule", itemType: catalog.Rules, status: add.StatusNew,
         underlying: &add.DiscoveryItem{Name: "pattern-rule", Type: catalog.Rules}},
    }
    m.actionableCount = 1
    m.installedCount = 0
    m.discoveryList = m.buildDiscoveryList()

    // First merge
    m.mergeConfirmIntoDiscovery()
    countAfterFirst := len(m.discoveredItems)
    if countAfterFirst != 2 {
        t.Errorf("after first merge: want 2 items, got %d", countAfterFirst)
    }

    // Second merge (simulate Back→Triage→Next) — must not duplicate
    m.mergeConfirmIntoDiscovery()
    if len(m.discoveredItems) != countAfterFirst {
        t.Errorf("after second merge: want %d items, got %d (idempotency broken)",
            countAfterFirst, len(m.discoveredItems))
    }
}

func TestAddWizard_ClearTriageState(t *testing.T) {
    t.Parallel()
    m := openAddWizard(nil, nil, nil, "/tmp", "/tmp", "")
    m.hasTriageStep = true
    m.confirmItems = []addConfirmItem{{displayName: "x"}}
    m.confirmSelected = map[int]bool{0: true}
    m.confirmCursor = 0
    m.maxStep = addStepReview

    m.clearTriageState()

    if m.hasTriageStep {
        t.Error("hasTriageStep should be false after clearTriageState")
    }
    if m.confirmItems != nil {
        t.Error("confirmItems should be nil after clearTriageState")
    }
    if m.maxStep != addStepDiscovery {
        t.Errorf("maxStep should be addStepDiscovery after clearTriageState, got %d", m.maxStep)
    }
}
```

**Success Criteria:**
- `cd cli && go test ./internal/tui/... -run TestAddWizard_` → pass (all new invariant tests)
- `grep -n "func TestAddWizard_ValidateStep_TriageForward\|func TestAddWizard_MergeIdempotency\|func TestAddWizard_ClearTriageState" cli/internal/tui/wizard_invariant_test.go` → returns all three function definitions

---

### Task 9.2 — stepForShellIndex unit tests

**Files:**
- Modify: `cli/internal/tui/wizard_invariant_test.go`

**Depends on:** Task 4.3

**Steps:**

Add table-driven tests covering all four permutations × all valid shell indices:

```go
func TestAddWizard_StepForShellIndex(t *testing.T) {
    t.Parallel()
    tests := []struct {
        name          string
        preFilterType catalog.ContentType
        hasTriageStep bool
        shellIdx      int
        want          addStep
    }{
        // +Type +Triage (direct mapping)
        {"+Type+Triage idx 0", "", true, 0, addStepSource},
        {"+Type+Triage idx 1", "", true, 1, addStepType},
        {"+Type+Triage idx 2", "", true, 2, addStepDiscovery},
        {"+Type+Triage idx 3", "", true, 3, addStepTriage},
        {"+Type+Triage idx 4", "", true, 4, addStepReview},
        {"+Type+Triage idx 5", "", true, 5, addStepExecute},
        // +Type -Triage
        {"+Type-Triage idx 0", "", false, 0, addStepSource},
        {"+Type-Triage idx 1", "", false, 1, addStepType},
        {"+Type-Triage idx 2", "", false, 2, addStepDiscovery},
        {"+Type-Triage idx 3", "", false, 3, addStepReview},
        {"+Type-Triage idx 4", "", false, 4, addStepExecute},
        // -Type +Triage
        {"-Type+Triage idx 0", "rules", true, 0, addStepSource},
        {"-Type+Triage idx 1", "rules", true, 1, addStepDiscovery},
        {"-Type+Triage idx 2", "rules", true, 2, addStepTriage},
        {"-Type+Triage idx 3", "rules", true, 3, addStepReview},
        {"-Type+Triage idx 4", "rules", true, 4, addStepExecute},
        // -Type -Triage
        {"-Type-Triage idx 0", "rules", false, 0, addStepSource},
        {"-Type-Triage idx 1", "rules", false, 1, addStepDiscovery},
        {"-Type-Triage idx 2", "rules", false, 2, addStepReview},
        {"-Type-Triage idx 3", "rules", false, 3, addStepExecute},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            m := openAddWizard(nil, nil, nil, "/tmp", "/tmp", tt.preFilterType)
            m.hasTriageStep = tt.hasTriageStep
            got := m.stepForShellIndex(tt.shellIdx)
            if got != tt.want {
                t.Errorf("stepForShellIndex(%d) = %d, want %d", tt.shellIdx, got, tt.want)
            }
        })
    }
}
```

**Success Criteria:**
- `cd cli && go test ./internal/tui/... -run TestAddWizard_StepForShellIndex` → pass
- `grep -n "func TestAddWizard_StepForShellIndex" cli/internal/tui/wizard_invariant_test.go` → returns the function definition, and the test table covers all 4 permutations (18 rows total)

---

### Task 9.3 — Golden file tests for triage step

**Files:**
- Modify: `cli/internal/tui/golden_test.go`
- Create: `cli/internal/tui/testdata/triage-*.golden` (generated by `-update-golden`)

**Depends on:** Phase 6

**Steps:**

Add three golden tests (80x20 minimum, 80x30 standard, 120x40 wide):

```go
func TestGolden_Triage_80x20(t *testing.T) {
    // Three items: High (checked), Medium (unchecked), Low (unchecked)
    // At minimum TUI size — verify narrow column collapse
    app := testAppSize(t, 80, 20)
    // Navigate to triage step with fixture confirm items
    // ...wizard navigation...
    requireGolden(t, "triage-items-80x20", snapshotApp(t, app))
}

func TestGolden_Triage_80x30(t *testing.T) {
    // Standard size with 5 items of varied tiers
    requireGolden(t, "triage-items-80x30", snapshotApp(t, app))
}

func TestGolden_Triage_120x40(t *testing.T) {
    // Wide layout — full columns visible
    requireGolden(t, "triage-items-120x40", snapshotApp(t, app))
}

func TestGolden_Triage_Empty(t *testing.T) {
    // Empty confirm items — should not reach triage (validateStep panics)
    // Test that navigation skips triage when hasTriageStep=false
    requireGolden(t, "triage-skipped-80x30", snapshotApp(t, app))
}
```

After implementing, run:
```bash
cd cli && go test ./internal/tui/ -update-golden
git diff internal/tui/testdata/  # review every golden change
```

**Success Criteria:**
- `cd cli && go test ./internal/tui/ -update-golden` → pass (generates baselines)
- `cd cli && go test ./internal/tui/ -run TestGolden_Triage` → pass (matches baselines)

---

### Task 9.4 — TierForMeta / metadata integration test

**Files:**
- Modify: `cli/internal/add/add_test.go`

**Depends on:** Tasks 2.1, 2.2, 3.1

**Steps:**

Add an integration test that writes an item with confidence metadata and verifies it loads back correctly:

```go
func TestAddItems_PersistsConfidenceMetadata(t *testing.T) {
    t.Parallel()
    dir := t.TempDir()
    contentDir := filepath.Join(dir, "content")
    srcDir := filepath.Join(dir, "src", "rules", "my-rule")
    os.MkdirAll(srcDir, 0755)
    os.WriteFile(filepath.Join(srcDir, "rule.md"), []byte("# My Rule\nContent here."), 0644)

    item := DiscoveryItem{
        Name:            "my-rule",
        Type:            catalog.Rules,
        Path:            filepath.Join(srcDir, "rule.md"),
        SourceDir:       srcDir,
        Status:          StatusNew,
        Confidence:      0.75,
        DetectionSource: "content-signal",
        DetectionMethod: "automatic",
    }

    results := AddItems([]DiscoveryItem{item}, AddOptions{}, contentDir, nil, "syllago-test")
    if results[0].Status == AddStatusError {
        t.Fatalf("AddItems error: %v", results[0].Error)
    }

    destDir := filepath.Join(contentDir, "rules", "my-rule")
    m, err := metadata.Load(destDir)
    if err != nil {
        t.Fatalf("Load metadata: %v", err)
    }
    if m.Confidence != 0.75 {
        t.Errorf("Confidence: got %v, want 0.75", m.Confidence)
    }
    if m.DetectionSource != "content-signal" {
        t.Errorf("DetectionSource: got %q, want content-signal", m.DetectionSource)
    }
    // Verify tier from persisted metadata matches original
    tier := analyzer.TierForMeta(m.Confidence, m.DetectionMethod)
    if tier != analyzer.TierMedium {
        t.Errorf("TierForMeta: got %q, want Medium", tier)
    }
}
```

**Success Criteria:**
- `cd cli && go test ./internal/add/... -run TestAddItems_PersistsConfidenceMetadata` → pass
- `grep -n "func TestAddItems_PersistsConfidenceMetadata" cli/internal/add/add_test.go` → returns the function definition, and the test body asserts `m.Confidence == 0.75` and calls `analyzer.TierForMeta`

---

### Task 9.5 — Backend dedup and type-filter tests

**Files:**
- Modify: `cli/internal/tui/add_wizard_test.go` (or create)

**Depends on:** Phase 5

**Steps:**

Add unit tests for the discovery backend analyzer integration:

```go
func TestDiscoverFromLocalPath_TypeFilter(t *testing.T) {
    t.Parallel()
    // Create a dir with both rules and agents content
    dir := t.TempDir()
    // ... create fixture files ...
    // Request only Rules — verify agents not in either list
    items, confirm, err := discoverFromLocalPath(dir, []catalog.ContentType{catalog.Rules}, "")
    // ...assert no agents in items or confirm...
}

func TestDiscoverFromLocalPath_AnalyzerDedup(t *testing.T) {
    t.Parallel()
    // Pattern-detected item should not appear in confirm list too
    // ...
}
```

**Success Criteria:**
- `cd cli && go test ./internal/tui/... -run TestDiscover` → pass
- `grep -n "func TestDiscoverFromLocalPath_TypeFilter\|func TestDiscoverFromLocalPath_AnalyzerDedup" cli/internal/tui/add_wizard_test.go` → returns both function definitions

---

## Phase 10: Polish & Final Verification

Build, format, golden regeneration, coverage check.

### Task 10.1 — Format and build

**Files:** No file changes.

**Depends on:** All phases complete

**Steps:**

```bash
cd cli && make fmt
cd cli && make build
```

Fix any `gofmt` issues flagged by `make fmt`. Verify binary builds without errors.

**Success Criteria:**
- `cd cli && make fmt && make build` → exit 0
- `syllago --version` → prints version (binary is up to date)

---

### Task 10.2 — Full test suite

**Files:** No file changes.

**Depends on:** Task 10.1

**Steps:**

```bash
cd cli && go test ./...
```

Fix any failing tests. Pay attention to:
- `TestAddWizard_ValidateStep_*` — enum shift may require raw integer updates
- Any golden file tests that now show different output due to step enum changes

**Success Criteria:**
- `cd cli && go test ./...` → pass (zero failures)
- `cd cli && go test ./... 2>&1 | grep -c "^---"` → returns 0 (no FAIL lines in output)

---

### Task 10.3 — Regenerate golden baselines

**Files:**
- Modify: `cli/internal/tui/testdata/*.golden` (auto-generated)

**Depends on:** Task 10.2

**Steps:**

```bash
cd cli && go test ./internal/tui/ -update-golden
git diff internal/tui/testdata/
```

Review every golden change:
- New triage goldens should look correct (split pane, items with tier dots)
- Existing install wizard goldens should be unchanged unless the tier badge was added
- Existing add wizard goldens for Review/Execute must reflect the new step enum shell labels

**Success Criteria:**
- `cd cli && go test ./internal/tui/` → pass (all goldens match)

---

### Task 10.4 — Coverage check

**Files:** No file changes.

**Depends on:** Task 10.3

**Steps:**

```bash
cd cli && go test ./internal/catalog/... -coverprofile=cov.out && go tool cover -func=cov.out | grep total
cd cli && go test ./internal/analyzer/... -coverprofile=cov.out && go tool cover -func=cov.out | grep total
cd cli && go test ./internal/add/... -coverprofile=cov.out && go tool cover -func=cov.out | grep total
cd cli && go test ./internal/tui/... -coverprofile=cov.out && go tool cover -func=cov.out | grep total
```

Each package must be ≥ 80% total coverage. If below threshold, add targeted tests for uncovered paths.

**Success Criteria:**
- All four packages report ≥ 80% total coverage
- `cd cli && go test ./internal/tui/... -coverprofile=cov.out && go tool cover -func=cov.out | grep "^total"` → percentage field is ≥ 80.0%

---

## Appendix: Complete 4-Permutation Truth Table

| preFilterType | hasTriageStep | Shell Labels | Steps |
|---|---|---|---|
| `""` (false) | true | Source Type Discovery Triage Review Execute | 6 steps |
| `""` (false) | false | Source Type Discovery Review Execute | 5 steps |
| non-empty (true) | true | Source Discovery Triage Review Execute | 5 steps |
| non-empty (true) | false | Source Discovery Review Execute | 4 steps |

`shellIndexForStep` maps step enum → label index. `stepForShellIndex` maps label index → step enum. Both methods must be consistent for all 4 permutations.

---

## Appendix: Pre-Check Default Selection

| Tier | Pre-checked | Rationale |
|------|-------------|-----------|
| `TierHigh` | ✓ Yes | Analyzer confident |
| `TierUser` | ✓ Yes | User explicitly identified this directory |
| `TierMedium` | ✗ No | Uncertain — user should review |
| `TierLow` | ✗ No | Weak signals — likely noise |

Set in `handleDiscoveryDone` when processing confirm items.

---

## Appendix: Security Decisions

| Decision | Implementation |
|---|---|
| B1/B5: Allowlist strip | `stripAnalyzerMetadata()` in `add.go` zeros confidence/detection_source/detection_method from copied packages |
| B2: Merge idempotency | `mergeConfirmIntoDiscovery()` truncates to `actionableCount+installedCount` before re-appending |
| B3: Breadcrumb mapping | `stepForShellIndex()` inverse method with 4-permutation switch |
| B4: clearTriageState | Resets all triage fields including `maxStep` and `confirmPreview` |
| B6: ReadFileContent | `filepath.Clean` + prefix check in `catalog.ReadFileContent` |
| B7: maxStep reset | `clearTriageState()` sets `m.maxStep = addStepDiscovery` before `updateMaxStep()` |
| I11: Provider pattern-only | `discoverFromProvider` returns `nil` confirm items — no analyzer call |
| I15: SafeResolve | `catalog.SafeResolve()` in `loadTriagePreview()` (Task 6.2) and `copySupportingFiles()` (Task 2.2 step 4); `.syllago.yaml` excluded from copy loop |
