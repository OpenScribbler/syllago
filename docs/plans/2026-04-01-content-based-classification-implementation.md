# Content-Based Classification — Implementation Plan

**Design doc:** `docs/plans/2026-04-01-content-based-classification-design.md`
**Date:** 2026-04-01

---

## Phase 1: Security Foundations

### Task 1.1 — Path traversal fix in resolveHookScript (B1)

**Files:**
- `cli/internal/analyzer/detector_cc.go` — add containment check to `resolveHookScript`
- `cli/internal/analyzer/detector_cc_test.go` — add security tests

**Why first:** B1 is an existing bug (unimplemented path containment) that affects production code before any new features land. Fixing it first means every subsequent hook-related test runs against a safe baseline.

**Test first (`detector_cc_test.go`):**

```go
func TestResolveHookScript_PathTraversal(t *testing.T) {
    t.Parallel()
    root := t.TempDir()
    // Create a real file outside repo root to verify stat doesn't help attacker.
    outsideFile := filepath.Join(t.TempDir(), "evil.sh")
    os.WriteFile(outsideFile, []byte("#!/bin/sh\nrm -rf /\n"), 0755)

    cases := []struct {
        name    string
        command string
        want    string // empty = rejected
    }{
        {"traversal dots", "bash ../../evil.sh", ""},
        {"absolute path", "/etc/passwd", ""},
        {"absolute path in command", "bash /etc/passwd", ""},
        {"clean relative inside repo", "bash hooks/validate.sh", "hooks/validate.sh"},
        {"no path token", "echo done", ""},
        {"double-slash absolute", "bash //etc/evil.sh", ""},
    }
    setupFile(t, root, "hooks/validate.sh", "#!/bin/sh\n")

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            got := resolveHookScript(tc.command, root)
            if got != tc.want {
                t.Errorf("resolveHookScript(%q, root) = %q, want %q", tc.command, got, tc.want)
            }
        })
    }
}

func TestResolveHookScript_SymlinkEscape(t *testing.T) {
    t.Parallel()
    root := t.TempDir()
    outside := t.TempDir()
    os.WriteFile(filepath.Join(outside, "evil.sh"), []byte("#!/bin/sh\n"), 0755)
    // Create symlink inside repo pointing outside.
    os.Symlink(filepath.Join(outside, "evil.sh"), filepath.Join(root, "hooks", "evil.sh"))
    os.MkdirAll(filepath.Join(root, "hooks"), 0755)
    os.Symlink(filepath.Join(outside, "evil.sh"), filepath.Join(root, "hooks", "escape.sh"))

    got := resolveHookScript("bash hooks/escape.sh", root)
    if got != "" {
        t.Errorf("expected symlink escape to be rejected, got %q", got)
    }
}
```

**Implementation (`detector_cc.go` — replace `resolveHookScript`):**

```go
// resolveHookScript extracts a script path from a hook command string.
// Returns the relative path if the file exists within repoRoot, empty string otherwise.
// Rejects absolute paths, path traversal (../), and symlinks that escape the repo root.
func resolveHookScript(command, repoRoot string) string {
    scriptExts := map[string]bool{
        ".sh": true, ".py": true, ".js": true, ".ts": true, ".rb": true, ".bash": true,
    }

    tokens := strings.Fields(command)
    for _, token := range tokens {
        // Reject absolute paths immediately — no stat needed.
        if filepath.IsAbs(token) {
            continue
        }
        // Reject tokens with traversal components before joining.
        cleaned := filepath.Clean(token)
        if strings.HasPrefix(cleaned, "..") {
            continue
        }
        hasPathSep := strings.Contains(token, "/") || strings.Contains(token, "\\")
        hasScriptExt := scriptExts[filepath.Ext(token)]
        if !hasPathSep && !hasScriptExt {
            continue
        }
        abs := filepath.Join(repoRoot, cleaned)
        // Resolve symlinks to verify real path stays within repo root.
        resolved, err := filepath.EvalSymlinks(abs)
        if err != nil {
            continue // file doesn't exist or symlink broken
        }
        resolvedRoot, err := filepath.EvalSymlinks(repoRoot)
        if err != nil {
            continue
        }
        if !strings.HasPrefix(resolved+string(filepath.Separator), resolvedRoot+string(filepath.Separator)) {
            continue // symlink escapes repo root
        }
        rel, err := filepath.Rel(resolvedRoot, resolved)
        if err != nil || strings.HasPrefix(rel, "..") {
            continue
        }
        return filepath.ToSlash(rel)
    }
    return ""
}
```

### Success Criteria

```
command → pass|fail — description
cd cli && go test ./internal/analyzer/ -run TestResolveHookScript_PathTraversal → pass — traversal inputs return empty string
cd cli && go test ./internal/analyzer/ -run TestResolveHookScript_SymlinkEscape → pass — symlink escaping repo root is rejected
cd cli && go test ./internal/analyzer/ → pass — no existing tests broken
cd cli && make build → pass — binary compiles
```

---

### Task 1.2 — External content sanitization boundary (B2)

**Files:**
- `cli/internal/analyzer/sanitize.go` — new file with `SanitizeItem` function
- `cli/internal/analyzer/sanitize_test.go` — new test file
- `cli/internal/analyzer/analyzer.go` — call `SanitizeItem` after classify, before dedup

**Why:** Every `DetectedItem` string field that originates from external repo content must be stripped of C0/C1 control chars and truncated before reaching display, audit log, or dedup. This is a one-time boundary — all callers inherit it.

**DetectedItem string fields that require sanitization** (exhaustive list from `types.go`):
- `Name` — max 80 chars
- `DisplayName` — max 80 chars
- `Description` — max 200 chars
- `Path` — max 256 chars, also validate no null bytes
- `ConfigSource` — max 256 chars
- `HookEvent` — max 64 chars
- `Provider` — max 64 chars
- `InternalLabel` — max 64 chars
- `ContentHash` — max 64 chars (hex only, strip non-hex)
- `Scripts` elements — each max 256 chars
- `References` elements — each max 256 chars
- `Providers` elements — each max 256 chars

**Test first (`sanitize_test.go`):**

```go
package analyzer

import (
    "strings"
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestSanitizeItem_StripsControlChars(t *testing.T) {
    t.Parallel()
    item := &DetectedItem{
        Name:        "skill\x1b[31mred\x1b[0m",
        Description: "desc\x00with\x07bells",
        Path:        "skills/foo\x01bar/SKILL.md",
        Provider:    "claude-code",
        Type:        catalog.Skills,
        Confidence:  0.90,
    }
    SanitizeItem(item)
    if strings.ContainsAny(item.Name, "\x00\x01\x07\x1b") {
        t.Errorf("Name still contains control chars: %q", item.Name)
    }
    if strings.ContainsAny(item.Description, "\x00\x01\x07\x1b") {
        t.Errorf("Description still contains control chars: %q", item.Description)
    }
    if strings.ContainsAny(item.Path, "\x00\x01") {
        t.Errorf("Path still contains control chars: %q", item.Path)
    }
}

func TestSanitizeItem_TruncatesLongFields(t *testing.T) {
    t.Parallel()
    item := &DetectedItem{
        Name:        strings.Repeat("a", 200),
        Description: strings.Repeat("b", 500),
        Path:        strings.Repeat("c", 400),
        Type:        catalog.Skills,
        Confidence:  0.90,
    }
    SanitizeItem(item)
    if len([]rune(item.Name)) > 80 {
        t.Errorf("Name not truncated: len=%d", len(item.Name))
    }
    if !strings.HasSuffix(item.Name, "…") {
        t.Errorf("Name missing ellipsis: %q", item.Name)
    }
    if len([]rune(item.Description)) > 200 {
        t.Errorf("Description not truncated: len=%d", len(item.Description))
    }
    if len([]rune(item.Path)) > 256 {
        t.Errorf("Path not truncated: len=%d", len(item.Path))
    }
}

func TestSanitizeItem_ScriptSlices(t *testing.T) {
    t.Parallel()
    item := &DetectedItem{
        Type:       catalog.Hooks,
        Confidence: 0.90,
        Scripts:    []string{"hooks/validate.sh\x1b[", strings.Repeat("x", 300)},
        References: []string{"ref\x00bad"},
        Providers:  []string{"provider\x07path"},
    }
    SanitizeItem(item)
    for i, s := range item.Scripts {
        if strings.ContainsAny(s, "\x00\x01\x07\x1b") {
            t.Errorf("Scripts[%d] contains control chars: %q", i, s)
        }
        if len([]rune(s)) > 256 {
            t.Errorf("Scripts[%d] not truncated", i)
        }
    }
    for i, s := range item.References {
        if strings.ContainsAny(s, "\x00\x01\x07\x1b") {
            t.Errorf("References[%d] contains control chars: %q", i, s)
        }
    }
    for i, s := range item.Providers {
        if strings.ContainsAny(s, "\x00\x01\x07\x1b") {
            t.Errorf("Providers[%d] contains control chars: %q", i, s)
        }
    }
}

func TestSanitizeItem_MaliciousFilenameInStructuredFields(t *testing.T) {
    t.Parallel()
    // Simulates a repo with a filename designed to inject into audit log JSON.
    item := &DetectedItem{
        Name: `legit","event_type":"hook.execute","exit_code":0,"x":"`,
        Path: `skills/foo\u0000bar`,
        Type: catalog.Skills,
    }
    SanitizeItem(item)
    if strings.Contains(item.Name, `"event_type"`) {
        t.Errorf("Name contains JSON injection payload after sanitize: %q", item.Name)
    }
}
```

**Implementation (`sanitize.go`):**

```go
package analyzer

import (
    "strings"
    "unicode"
)

// Field length limits for DetectedItem string fields.
const (
    maxNameLen          = 80
    maxDescriptionLen   = 200
    maxPathLen          = 256
    maxShortFieldLen    = 64
)

// SanitizeItem strips C0/C1 control characters from all string fields of a
// DetectedItem and truncates fields that exceed display length limits.
// Must be called after Classify, before dedup or audit writes.
func SanitizeItem(item *DetectedItem) {
    item.Name = sanitizeField(item.Name, maxNameLen)
    item.DisplayName = sanitizeField(item.DisplayName, maxNameLen)
    item.Description = sanitizeField(item.Description, maxDescriptionLen)
    item.Path = sanitizeField(item.Path, maxPathLen)
    item.ConfigSource = sanitizeField(item.ConfigSource, maxPathLen)
    item.HookEvent = sanitizeField(item.HookEvent, maxShortFieldLen)
    item.Provider = sanitizeField(item.Provider, maxShortFieldLen)
    item.InternalLabel = sanitizeField(item.InternalLabel, maxShortFieldLen)
    // ContentHash: strip non-hex characters, keep max 64 chars.
    item.ContentHash = sanitizeHex(item.ContentHash)
    item.Scripts = sanitizeSlice(item.Scripts, maxPathLen)
    item.References = sanitizeSlice(item.References, maxPathLen)
    item.Providers = sanitizeSlice(item.Providers, maxPathLen)
}

// sanitizeField strips C0/C1 control chars (except \t and \n) and truncates to maxLen runes.
func sanitizeField(s string, maxLen int) string {
    if s == "" {
        return s
    }
    var b strings.Builder
    for _, r := range s {
        if isAllowedRune(r) {
            b.WriteRune(r)
        }
    }
    clean := b.String()
    runes := []rune(clean)
    if len(runes) > maxLen {
        return string(runes[:maxLen-1]) + "…"
    }
    return clean
}

// sanitizeSlice applies sanitizeField to each element.
func sanitizeSlice(ss []string, maxLen int) []string {
    for i, s := range ss {
        ss[i] = sanitizeField(s, maxLen)
    }
    return ss
}

// sanitizeHex strips non-hex characters and truncates to 64 chars.
func sanitizeHex(s string) string {
    var b strings.Builder
    for _, r := range s {
        if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
            b.WriteRune(r)
        }
    }
    result := b.String()
    if len(result) > 64 {
        return result[:64]
    }
    return result
}

// isAllowedRune returns true for runes that are safe for display and structured output.
// Allows printable characters, tab, and newline. Strips C0 (0x00–0x1F except \t,\n)
// and C1 (0x80–0x9F) control characters.
func isAllowedRune(r rune) bool {
    if r == '\t' || r == '\n' {
        return true
    }
    if r < 0x20 { // C0 control chars
        return false
    }
    if r >= 0x80 && r <= 0x9F { // C1 control chars
        return false
    }
    return unicode.IsPrint(r) || r == ' '
}
```

**Wire into `analyzer.go`** — in the classify loop, after the `item.References = ResolveReferences(...)` call, add:

```go
SanitizeItem(item)
```

The ordering must be: `Classify` → `SanitizeItem` → (later) audit writes → `DeduplicateItems`.

### Success Criteria

```
command → pass|fail — description
cd cli && go test ./internal/analyzer/ -run TestSanitizeItem → pass — all sanitize cases pass
cd cli && go test ./internal/analyzer/ -run TestSanitizeItem_MaliciousFilenameInStructuredFields → pass — JSON injection in Name is neutralized
cd cli && go test ./internal/analyzer/ → pass — existing tests unaffected
cd cli && make build → pass — binary compiles
```

---

### Task 1.3 — Global README exclusion (Component 4)

**Files:**
- `cli/internal/analyzer/match.go` — add exclusion check in `MatchPatterns`
- `cli/internal/analyzer/match_test.go` — add exclusion tests

**Why:** README.md is the most common false positive across all detectors. A single exclusion point in `MatchPatterns` protects all 11 existing detectors and the upcoming content-signal detector simultaneously.

**Test first (`match_test.go` additions):**

```go
func TestMatchPatterns_GlobalREADMEExclusion(t *testing.T) {
    t.Parallel()
    // README.md at top-level would match agents/*.md if placed in agents/
    // but the global exclusion must fire for standard README filenames.
    excluded := []string{
        "README.md",
        "CHANGELOG.md",
        "LICENSE.md",
        "CONTRIBUTING.md",
        "CODE_OF_CONDUCT.md",
        "agents/README.md",
        "skills/foo/README.md",
    }
    notExcluded := []string{
        "CLAUDE.md",
        "GEMINI.md",
        "AGENTS.md",
        "agents/my-agent.md",
    }
    dets := []ContentDetector{&TopLevelDetector{}}

    for _, path := range excluded {
        matches := MatchPatterns([]string{path}, dets)
        if len(matches) > 0 {
            t.Errorf("path %q should be excluded but got %d matches", path, len(matches))
        }
    }
    for _, path := range notExcluded {
        // Only test that exclusion does NOT fire — not all paths will match patterns.
        // Just ensure calling MatchPatterns with these paths doesn't panic.
        _ = MatchPatterns([]string{path}, dets)
    }
}
```

**Implementation (`match.go`):**

Add before the existing matches loop:

```go
// globalExcludedBasenames are filenames excluded from all detectors regardless of path.
// Prevents false positives from documentation files that happen to match content patterns.
// Exception: CLAUDE.md, GEMINI.md, AGENTS.md are NOT excluded — they are legitimate content.
var globalExcludedBasenames = map[string]bool{
    "README.md":          true,
    "CHANGELOG.md":       true,
    "LICENSE.md":         true,
    "CONTRIBUTING.md":    true,
    "CODE_OF_CONDUCT.md": true,
}
```

In `MatchPatterns`, add as first filter inside the `paths` loop:

```go
if globalExcludedBasenames[filepath.Base(p)] {
    continue
}
```

### Success Criteria

```
command → pass|fail — description
cd cli && go test ./internal/analyzer/ -run TestMatchPatterns_GlobalREADMEExclusion → pass — excluded basenames produce zero matches
cd cli && go test ./internal/analyzer/ → pass — existing match tests unaffected (agents/my-agent.md still matches)
```

---

### Task 1.4 — Vocabulary hardcoded in binary (B4)

**Files:**
- `cli/internal/analyzer/vocabulary.go` — new file with hardcoded hook event names and fingerprint vocabulary

**Why:** B4 requires classification-critical vocabulary to live in the binary, not loaded from converter packages. Centralizing it here also makes the content-signal detector in Phase 2 straightforward to implement.

**Implementation (`vocabulary.go`):**

```go
package analyzer

// knownHookEventNames contains all known hook event names across all providers.
// These are hardcoded in the binary — never loaded from converter packages.
// Converters contribute display metadata only; scoring vocabulary is immutable.
var knownHookEventNames = map[string]bool{
    // Claude Code
    "PreToolUse":  true,
    "PostToolUse": true,
    "SessionStart": true,
    "SessionEnd":  true,
    // Gemini
    "BeforeTool": true,
    "AfterTool":  true,
    // Windsurf
    "BeforeRun": true,
    "AfterRun":  true,
    // Generic / syllago canonical
    "before_tool_execute": true,
    "after_tool_execute":  true,
    "session_start":       true,
    "session_end":         true,
    "pre_run_command":     true,
    "post_run_command":    true,
    "pre_read_code":       true,
    "pre_write_code":      true,
}

// directoryKeywords are directory name substrings that indicate AI content presence.
// Used by the content-signal detector's pre-filter.
var directoryKeywords = []string{
    "agent", "skill", "rule", "hook", "command",
    "mcp", "prompt", "steering", "pack", "workflow",
}

// contentSignalExtensions are the only file extensions the content-signal detector inspects.
var contentSignalExtensions = map[string]bool{
    ".md":   true,
    ".yaml": true,
    ".yml":  true,
    ".json": true,
    ".toml": true,
}
```

**Test (`vocabulary_test.go`):**

```go
package analyzer

import "testing"

func TestKnownHookEventNames_NotEmpty(t *testing.T) {
    t.Parallel()
    if len(knownHookEventNames) == 0 {
        t.Fatal("knownHookEventNames must not be empty")
    }
    // Spot check canonical names are present.
    required := []string{"PreToolUse", "PostToolUse", "SessionStart", "before_tool_execute"}
    for _, name := range required {
        if !knownHookEventNames[name] {
            t.Errorf("knownHookEventNames missing required entry %q", name)
        }
    }
}

func TestDirectoryKeywords_NotEmpty(t *testing.T) {
    t.Parallel()
    if len(directoryKeywords) == 0 {
        t.Fatal("directoryKeywords must not be empty")
    }
}
```

### Success Criteria

```
command → pass|fail — description
cd cli && go test ./internal/analyzer/ -run TestKnownHookEventNames → pass — vocabulary is present and complete
cd cli && go test ./internal/analyzer/ → pass — all existing tests pass
```

---

## Phase 2: Content-Signal Detector

### Task 2.1 — Signal scoring unit tests (write tests first)

**Files:**
- `cli/internal/analyzer/detector_content_signal_test.go` — new file, all signal scoring tests

**Why TDD here:** The scoring logic is the most complex piece. Writing tests first forces exact specification of weights, thresholds, and edge cases before any production code exists.

```go
package analyzer

import (
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestContentSignalDetector_PreFilter_ExtensionReject(t *testing.T) {
    t.Parallel()
    d := &ContentSignalDetector{}
    // .go files should be rejected by pre-filter.
    if d.passesPreFilter("agents/foo/bar.go", false) {
        t.Error("expected .go to be rejected by pre-filter")
    }
    if d.passesPreFilter("agents/foo/bar.rs", false) {
        t.Error("expected .rs to be rejected by pre-filter")
    }
}

func TestContentSignalDetector_PreFilter_ExtensionAccept(t *testing.T) {
    t.Parallel()
    d := &ContentSignalDetector{}
    exts := []string{".md", ".yaml", ".yml", ".json", ".toml"}
    for _, ext := range exts {
        path := "agents/foo/bar" + ext
        if !d.passesPreFilter(path, false) {
            t.Errorf("expected %q to pass pre-filter", path)
        }
    }
}

func TestContentSignalDetector_PreFilter_DirectoryKeyword(t *testing.T) {
    t.Parallel()
    d := &ContentSignalDetector{}
    // With userDirected=false, directory name keyword is required.
    if !d.passesPreFilter("skills/foo/bar.md", false) {
        t.Error("path with 'skills' keyword should pass")
    }
    if d.passesPreFilter("docs/api/overview.md", false) {
        t.Error("path with no keyword should fail without userDirected")
    }
    // With userDirected=true, directory keyword not required.
    if !d.passesPreFilter("docs/api/overview.md", true) {
        t.Error("userDirected path should bypass directory keyword check")
    }
}

func TestContentSignalDetector_ScoreFilename_SKILL(t *testing.T) {
    t.Parallel()
    d := &ContentSignalDetector{}
    signals := d.scoreFilename("SKILL.md", catalog.Skills)
    total := sumWeights(signals)
    if total < 0.25 {
        t.Errorf("SKILL.md filename should score >= 0.25 for skills, got %.2f", total)
    }
}

func TestContentSignalDetector_ScoreFilename_AgentYAML(t *testing.T) {
    t.Parallel()
    d := &ContentSignalDetector{}
    signals := d.scoreFilename("my-agent.agent.yaml", catalog.Agents)
    total := sumWeights(signals)
    if total < 0.20 {
        t.Errorf("*.agent.yaml should score >= 0.20 for agents, got %.2f", total)
    }
}

func TestContentSignalDetector_ScoreJSON_MCPServers(t *testing.T) {
    t.Parallel()
    d := &ContentSignalDetector{}
    data := []byte(`{"mcpServers": {"myserver": {"command": "npx"}}}`)
    signals, ct := d.scoreJSON(data)
    if ct != catalog.MCP {
        t.Errorf("mcpServers JSON should detect MCP type, got %v", ct)
    }
    total := sumWeights(signals)
    if total < 0.30 {
        t.Errorf("mcpServers signal should score >= 0.30, got %.2f", total)
    }
}

func TestContentSignalDetector_ScoreJSON_HooksWiring(t *testing.T) {
    t.Parallel()
    d := &ContentSignalDetector{}
    data := []byte(`{
        "hooks": {
            "PreToolUse": [{"command": "bash hooks/lint.sh"}],
            "PostToolUse": [{"command": "echo done"}]
        }
    }`)
    signals, ct := d.scoreJSON(data)
    if ct != catalog.Hooks {
        t.Errorf("hooks JSON with event names should detect Hooks type, got %v", ct)
    }
    total := sumWeights(signals)
    if total < 0.25 {
        t.Errorf("hooks wiring should score >= 0.25, got %.2f", total)
    }
}

func TestContentSignalDetector_ScoreJSON_HooksOnlyOneEvent(t *testing.T) {
    t.Parallel()
    d := &ContentSignalDetector{}
    // Only one known event name — must NOT classify as hooks (needs ≥2).
    data := []byte(`{"hooks": {"PreToolUse": [], "onSave": []}}`)
    _, ct := d.scoreJSON(data)
    if ct == catalog.Hooks {
        t.Error("single known event name should not trigger hooks classification")
    }
}

func TestContentSignalDetector_ScoreFrontmatter_CommandFields(t *testing.T) {
    t.Parallel()
    d := &ContentSignalDetector{}
    data := []byte("---\nallowed-tools: [Bash, Read]\nargument-hint: \"<filename>\"\n---\nDo something.\n")
    signals, ct := d.scoreFrontmatter(data)
    if ct != catalog.Commands {
        t.Errorf("allowed-tools + argument-hint should detect Commands, got %v", ct)
    }
    total := sumWeights(signals)
    if total < 0.35 {
        t.Errorf("command frontmatter signals should total >= 0.35, got %.2f", total)
    }
}

func TestContentSignalDetector_ScoreFrontmatter_RuleFields(t *testing.T) {
    t.Parallel()
    d := &ContentSignalDetector{}
    data := []byte("---\nalwaysApply: true\nglobs: [\"*.go\"]\n---\nRule content.\n")
    signals, ct := d.scoreFrontmatter(data)
    if ct != catalog.Rules {
        t.Errorf("alwaysApply + globs should detect Rules, got %v", ct)
    }
    _ = signals
}

func TestContentSignalDetector_FinalScore_Floor(t *testing.T) {
    t.Parallel()
    d := &ContentSignalDetector{}
    // Base 0.40 + directory keyword 0.10 = 0.50 — must be dropped (below 0.55 floor).
    score := d.computeScore([]signalEntry{{weight: 0.10}})
    if score >= 0.55 {
        t.Errorf("base(0.40)+keyword(0.10)=0.50 must be below 0.55 floor, got %.2f", score)
    }
}

func TestContentSignalDetector_FinalScore_Cap(t *testing.T) {
    t.Parallel()
    d := &ContentSignalDetector{}
    // Many signals should be capped at 0.70.
    score := d.computeScore([]signalEntry{
        {weight: 0.25},
        {weight: 0.25},
        {weight: 0.20},
        {weight: 0.15},
        {weight: 0.10},
    })
    if score > 0.70 {
        t.Errorf("score should be capped at 0.70, got %.2f", score)
    }
}

func TestContentSignalDetector_TypePriority_CommandsOverSkills(t *testing.T) {
    t.Parallel()
    d := &ContentSignalDetector{}
    // Commands have higher specificity than Skills in tie-breaking.
    priority := d.typePriority(catalog.Commands)
    skillPriority := d.typePriority(catalog.Skills)
    if priority >= skillPriority {
        t.Errorf("Commands should have lower priority value (higher specificity) than Skills: commands=%d skills=%d", priority, skillPriority)
    }
}

func TestContentSignalDetector_GlobalREADMEExcluded(t *testing.T) {
    t.Parallel()
    root := t.TempDir()
    setupFile(t, root, "agents/README.md", "# Agents\nThis is documentation.\n")
    d := &ContentSignalDetector{}
    items, err := d.ClassifyUnmatched([]string{"agents/README.md"}, root, nil)
    if err != nil {
        t.Fatalf("ClassifyUnmatched error: %v", err)
    }
    if len(items) > 0 {
        t.Errorf("README.md in agents/ should be excluded, got %d items", len(items))
    }
}

// sumWeights is a test helper.
func sumWeights(signals []signalEntry) float64 {
    var total float64
    for _, s := range signals {
        total += s.weight
    }
    return total
}
```

### Success Criteria

```
command → pass|fail — description
cd cli && go test ./internal/analyzer/ -run TestContentSignalDetector → fail — tests exist, no implementation yet (expected at this stage)
cd cli && go test ./internal/analyzer/ -run "^(TestAnalyzer|TestClaudeCode|TestTopLevel|TestDedup)" → pass — existing tests unaffected by new test file
```

---

### Task 2.2 — ContentSignalDetector implementation

**Files:**
- `cli/internal/analyzer/detector_content_signal.go` — new file

**Implementation:**

```go
package analyzer

import (
    "encoding/json"
    "path/filepath"
    "strings"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
    "github.com/tidwall/gjson"
)

// signalEntry records one matched signal and its weight contribution.
type signalEntry struct {
    signal string
    weight float64
}

// contentSignalBase is the starting score for any file that passes the pre-filter.
const contentSignalBase = 0.40

// contentSignalFloor is the minimum score required to emit an item.
const contentSignalFloor = 0.55

// contentSignalCap is the maximum score content-signal items can achieve.
const contentSignalCap = 0.70

// ContentSignalDetector classifies files that pattern-based detectors did not match.
// It uses weighted signal scoring against static fingerprints only.
type ContentSignalDetector struct{}

func (d *ContentSignalDetector) ProviderSlug() string { return "content-signal" }

// Patterns returns empty — the content-signal detector does not participate in MatchPatterns.
// It operates on unmatched files via ClassifyUnmatched.
func (d *ContentSignalDetector) Patterns() []DetectionPattern { return nil }

// Classify satisfies ContentDetector but is not used by this detector.
func (d *ContentSignalDetector) Classify(path, repoRoot string) ([]*DetectedItem, error) {
    return nil, nil
}

// ClassifyUnmatched inspects files not matched by any pattern-based detector.
// unmatchedPaths are relative to repoRoot. scanAsPaths maps relative path prefixes
// to their user-specified content type (bypasses directory keyword filter).
func (d *ContentSignalDetector) ClassifyUnmatched(
    unmatchedPaths []string,
    repoRoot string,
    scanAsPaths map[string]catalog.ContentType,
) ([]*DetectedItem, error) {
    var items []*DetectedItem

    for _, p := range unmatchedPaths {
        // Global README exclusion applies to content-signal detector too.
        if globalExcludedBasenames[filepath.Base(p)] {
            continue
        }

        userDirected := false
        var typeHint catalog.ContentType
        for prefix, ct := range scanAsPaths {
            if strings.HasPrefix(filepath.ToSlash(p), filepath.ToSlash(prefix)) {
                userDirected = true
                typeHint = ct
                break
            }
        }

        if !d.passesPreFilter(p, userDirected) {
            continue
        }

        item := d.classifyFile(p, repoRoot, userDirected, typeHint)
        if item != nil {
            items = append(items, item)
        }
    }
    return items, nil
}

// passesPreFilter returns true if the file should be inspected for content signals.
// userDirected bypasses the directory keyword requirement.
func (d *ContentSignalDetector) passesPreFilter(path string, userDirected bool) bool {
    ext := strings.ToLower(filepath.Ext(path))
    if !contentSignalExtensions[ext] {
        return false
    }
    if userDirected {
        return true
    }
    // Check that at least one path component contains a known keyword.
    normalized := filepath.ToSlash(path)
    parts := strings.Split(normalized, "/")
    for _, part := range parts[:len(parts)-1] { // skip filename
        lower := strings.ToLower(part)
        for _, kw := range directoryKeywords {
            if strings.Contains(lower, kw) {
                return true
            }
        }
    }
    return false
}

// classifyFile reads and scores a single file, returning a DetectedItem or nil.
func (d *ContentSignalDetector) classifyFile(
    path, repoRoot string,
    userDirected bool,
    typeHint catalog.ContentType,
) *DetectedItem {
    ext := strings.ToLower(filepath.Ext(path))
    absPath := filepath.Join(repoRoot, path)

    var data []byte
    var readErr error
    if ext == ".json" {
        data, readErr = readFileLimited(absPath, limitJSON)
    } else {
        data, readErr = readFileLimited(absPath, limitMarkdown)
    }
    if readErr != nil || len(data) == 0 {
        return nil
    }

    // Score against all types and pick the winner.
    type typeScore struct {
        ct      catalog.ContentType
        signals []signalEntry
        score   float64
    }

    var best typeScore
    allTypes := []catalog.ContentType{
        catalog.Commands, catalog.Skills, catalog.Agents,
        catalog.Hooks, catalog.MCP, catalog.Rules,
    }

    if typeHint != "" {
        // User-directed: only score against the specified type.
        allTypes = []catalog.ContentType{typeHint}
    }

    for _, ct := range allTypes {
        var signals []signalEntry

        // Filename fingerprints.
        signals = append(signals, d.scoreFilename(filepath.Base(path), ct)...)

        // Directory keyword signals.
        signals = append(signals, d.scoreDirectory(path)...)

        // Content-based signals.
        if ext == ".json" {
            jsonSigs, detectedType := d.scoreJSON(data)
            if detectedType == ct {
                signals = append(signals, jsonSigs...)
            }
        } else {
            fmSigs, detectedType := d.scoreFrontmatter(data)
            if detectedType == ct || detectedType == "" {
                signals = append(signals, fmSigs...)
            }
        }

        score := d.computeScore(signals)
        if score > best.score || (score == best.score && d.typePriority(ct) < d.typePriority(best.ct)) {
            best = typeScore{ct, signals, score}
        }
    }

    if best.ct == "" || best.score < contentSignalFloor {
        return nil
    }

    confidence := best.score
    if userDirected {
        confidence = min(confidence+0.20, 0.85)
    }

    name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
    item := &DetectedItem{
        Name:          name,
        Type:          best.ct,
        Provider:      "content-signal",
        Path:          path,
        ContentHash:   hashBytes(data),
        Confidence:    confidence,
        InternalLabel: "content-signal",
    }

    if fm := parseFrontmatterBasic(data); fm != nil {
        item.DisplayName = fm.name
        item.Description = fm.description
    }

    return item
}

// scoreFilename returns signals based on filename patterns for a given content type.
func (d *ContentSignalDetector) scoreFilename(filename string, ct catalog.ContentType) []signalEntry {
    lower := strings.ToLower(filename)
    var signals []signalEntry

    switch ct {
    case catalog.Skills:
        if filename == "SKILL.md" {
            signals = append(signals, signalEntry{"filename_SKILL.md", 0.25})
        }
    case catalog.Agents:
        if filename == "AGENT.md" {
            signals = append(signals, signalEntry{"filename_AGENT.md", 0.25})
        }
        if strings.HasSuffix(lower, ".agent.yaml") || strings.HasSuffix(lower, ".agent.md") {
            signals = append(signals, signalEntry{"filename_agent_extension", 0.20})
        }
    }
    return signals
}

// scoreDirectory returns directory-context signals for the file path.
func (d *ContentSignalDetector) scoreDirectory(path string) []signalEntry {
    var signals []signalEntry
    normalized := filepath.ToSlash(path)
    parts := strings.Split(normalized, "/")
    if len(parts) < 2 {
        return nil
    }
    dirs := parts[:len(parts)-1]

    for i, part := range dirs {
        lower := strings.ToLower(part)
        for _, kw := range directoryKeywords {
            if strings.Contains(lower, kw) {
                if i == 0 {
                    signals = append(signals, signalEntry{"directory_keyword_" + kw, 0.10})
                } else {
                    signals = append(signals, signalEntry{"subdirectory_keyword_" + kw, 0.05})
                }
                break
            }
        }
    }
    return signals
}

// scoreJSON scores a JSON file and returns signals + detected content type.
func (d *ContentSignalDetector) scoreJSON(data []byte) ([]signalEntry, catalog.ContentType) {
    if !json.Valid(data) {
        return nil, ""
    }

    // MCP: top-level mcpServers key.
    if gjson.GetBytes(data, "mcpServers").IsObject() {
        return []signalEntry{{"json_mcpServers", 0.30}}, catalog.MCP
    }

    // Hooks: hooks key with ≥2 known event name subkeys.
    hooks := gjson.GetBytes(data, "hooks")
    if hooks.IsObject() {
        var matchCount int
        hooks.ForEach(func(key, _ gjson.Result) bool {
            if knownHookEventNames[key.String()] {
                matchCount++
            }
            return true
        })
        if matchCount >= 2 {
            return []signalEntry{{"json_hooks_event_names", 0.25}}, catalog.Hooks
        }
    }

    return nil, ""
}

// scoreFrontmatter scores YAML frontmatter and returns signals + detected content type.
func (d *ContentSignalDetector) scoreFrontmatter(data []byte) ([]signalEntry, catalog.ContentType) {
    // Parse frontmatter keys using a simple scanner (no full YAML parse).
    keys := parseFrontmatterKeys(data)
    if len(keys) == 0 {
        return nil, ""
    }

    var signals []signalEntry
    typeCounts := make(map[catalog.ContentType]float64)

    for _, k := range keys {
        switch k {
        case "allowed-tools":
            signals = append(signals, signalEntry{"frontmatter_allowed-tools", 0.20})
            typeCounts[catalog.Commands] += 0.20
        case "argument-hint":
            signals = append(signals, signalEntry{"frontmatter_argument-hint", 0.15})
            typeCounts[catalog.Commands] += 0.15
        case "alwaysApply":
            signals = append(signals, signalEntry{"frontmatter_alwaysApply", 0.15})
            typeCounts[catalog.Rules] += 0.15
        case "globs":
            signals = append(signals, signalEntry{"frontmatter_globs", 0.10})
            typeCounts[catalog.Rules] += 0.10
        }
    }

    // Pick the type with most signal weight.
    var bestType catalog.ContentType
    var bestWeight float64
    for ct, w := range typeCounts {
        if w > bestWeight {
            bestWeight = w
            bestType = ct
        }
    }

    return signals, bestType
}

// parseFrontmatterKeys extracts YAML frontmatter key names from file data.
func parseFrontmatterKeys(data []byte) []string {
    s := string(data)
    if !strings.HasPrefix(strings.TrimSpace(s), "---") {
        return nil
    }
    rest := s[strings.Index(s, "---")+3:]
    end := strings.Index(rest, "\n---")
    if end < 0 {
        return nil
    }
    block := rest[:end]
    var keys []string
    for _, line := range strings.Split(block, "\n") {
        if k, _, ok := strings.Cut(line, ":"); ok {
            k = strings.TrimSpace(k)
            if k != "" && !strings.HasPrefix(k, "#") {
                keys = append(keys, k)
            }
        }
    }
    return keys
}

// computeScore returns base + sum of signal weights, capped at contentSignalCap.
func (d *ContentSignalDetector) computeScore(signals []signalEntry) float64 {
    total := contentSignalBase
    for _, s := range signals {
        total += s.weight
    }
    if total > contentSignalCap {
        return contentSignalCap
    }
    return total
}

// typePriority returns a rank for tie-breaking. Lower = higher priority.
// Commands > Skills > Agents > Hooks > MCP > Rules.
func (d *ContentSignalDetector) typePriority(ct catalog.ContentType) int {
    switch ct {
    case catalog.Commands:
        return 0
    case catalog.Skills:
        return 1
    case catalog.Agents:
        return 2
    case catalog.Hooks:
        return 3
    case catalog.MCP:
        return 4
    case catalog.Rules:
        return 5
    default:
        return 99
    }
}

// Note: min() is a Go 1.21+ built-in and does not need to be defined.
```

### Success Criteria

```
command → pass|fail — description
cd cli && go test ./internal/analyzer/ -run TestContentSignalDetector → pass — all signal scoring unit tests pass
cd cli && go test ./internal/analyzer/ -run TestContentSignalDetector_GlobalREADMEExcluded → pass — README in agents/ not classified
cd cli && go test ./internal/analyzer/ → pass — no regressions
cd cli && make build → pass — binary compiles
```

---

### Task 2.3 — Wire ContentSignalDetector into analyzer pipeline

**Files:**
- `cli/internal/analyzer/analyzer.go` — add fallback pass after MatchPatterns
- `cli/internal/analyzer/analyzer_test.go` — integration tests for fallback detection

**Why:** The detector exists but is not yet called. The pipeline must: collect matched paths → compute unmatched paths → pass unmatched to `ContentSignalDetector.ClassifyUnmatched` → merge results.

**Test first (`analyzer_test.go` additions):**

```go
func TestAnalyzer_ContentSignalFallback_PAIStyle(t *testing.T) {
    t.Parallel()
    root := t.TempDir()
    // PAI-style layout: Packs/<name>/SKILL.md
    setupFile(t, root, "Packs/redteam-skill/SKILL.md",
        "---\nname: Red Team Skill\ndescription: Security testing\n---\nContent.\n")
    setupFile(t, root, "Packs/coding-skill/SKILL.md",
        "---\nname: Coding Skill\ndescription: Writes code\n---\nContent.\n")

    a := New(DefaultConfig())
    result, err := a.Analyze(root)
    if err != nil {
        t.Fatalf("Analyze error: %v", err)
    }
    // Both should be in Confirm (0.55-0.70 range, content-signal bucket).
    total := len(result.Auto) + len(result.Confirm)
    if total < 2 {
        t.Errorf("expected ≥2 detected items for PAI-style layout, got %d", total)
    }
    for _, item := range result.AllItems() {
        if item.Provider == "content-signal" && item.Type != catalog.Skills {
            t.Errorf("content-signal item %q should be Skills, got %v", item.Name, item.Type)
        }
    }
}

func TestAnalyzer_ContentSignalFallback_BMADStyle(t *testing.T) {
    t.Parallel()
    root := t.TempDir()
    // BMAD-style: src/bmm/agents/*.agent.yaml
    setupFile(t, root, "src/bmm/agents/orchestrator.agent.yaml",
        "name: Orchestrator\ndescription: Main orchestrator agent\n")
    setupFile(t, root, "src/bmm/agents/analyst.agent.yaml",
        "name: Analyst\ndescription: Analysis agent\n")

    a := New(DefaultConfig())
    result, err := a.Analyze(root)
    if err != nil {
        t.Fatalf("Analyze error: %v", err)
    }
    total := len(result.Auto) + len(result.Confirm)
    if total < 2 {
        t.Errorf("expected ≥2 detected items for BMAD-style layout, got %d", total)
    }
}

func TestAnalyzer_ContentSignalFallback_NoDoubleClassify(t *testing.T) {
    t.Parallel()
    root := t.TempDir()
    // Standard CC layout — should be handled by CC detector, NOT content-signal.
    setupFile(t, root, ".claude/agents/my-agent.md",
        "---\nname: My Agent\n---\nAgent body.\n")

    a := New(DefaultConfig())
    result, err := a.Analyze(root)
    if err != nil {
        t.Fatalf("Analyze error: %v", err)
    }
    for _, item := range result.AllItems() {
        if item.Provider == "content-signal" {
            t.Errorf("pattern-matched file %q should not be classified by content-signal detector", item.Path)
        }
    }
}
```

**Implementation changes to `analyzer.go`:**

After the `MatchPatterns` call, collect matched paths, then call fallback:

```go
// Step 2: Pattern matching.
candidates := MatchPatterns(walkResult.Paths, a.detectors)

// Build set of matched paths for fallback exclusion.
matchedPaths := make(map[string]bool, len(candidates))
for _, c := range candidates {
    matchedPaths[c.Path] = true
}

// Collect unmatched paths for content-signal fallback.
var unmatchedPaths []string
for _, p := range walkResult.Paths {
    if !matchedPaths[p] {
        unmatchedPaths = append(unmatchedPaths, p)
    }
}
```

Then after the classify loop (before dedup), add:

```go
// Step 3b: Content-signal fallback for unmatched files.
if !a.config.Strict {
    csd := &ContentSignalDetector{}
    fallbackItems, fallbackErr := csd.ClassifyUnmatched(unmatchedPaths, repoRoot, a.config.ScanAsPaths)
    if fallbackErr != nil {
        result.Warnings = append(result.Warnings, "content-signal fallback: "+fallbackErr.Error())
    }
    for _, item := range fallbackItems {
        if item == nil {
            continue
        }
        SanitizeItem(item)
        allItems = append(allItems, item)
    }
}
```

Add `Strict bool` and `ScanAsPaths map[string]catalog.ContentType` to `AnalysisConfig`:

```go
type AnalysisConfig struct {
    AutoThreshold float64
    SkipThreshold float64
    ExcludeDirs   []string
    SymlinkPolicy string
    Strict        bool                           // disables content-signal fallback
    ScanAsPaths   map[string]catalog.ContentType // user-directed: path prefix → type
}
```

### Success Criteria

```
command → pass|fail — description
cd cli && go test ./internal/analyzer/ -run TestAnalyzer_ContentSignalFallback → pass — PAI and BMAD style layouts produce ≥2 items each
cd cli && go test ./internal/analyzer/ -run TestAnalyzer_ContentSignalFallback_NoDoubleClassify → pass — CC-matched files not re-classified by content-signal
cd cli && go test ./internal/analyzer/ → pass — no regressions
cd cli && make build → pass — binary compiles
```

---

## Phase 3: Quick-Win Patterns + README Exclusion Completion

### Task 3.1 — Add quick-win patterns to TopLevelDetector

**Files:**
- `cli/internal/analyzer/detector_toplevel.go` — add 4 new patterns
- `cli/internal/analyzer/detector_toplevel_test.go` — add tests for new patterns

**Test first:**

```go
func TestTopLevelDetector_QuickWinPatterns(t *testing.T) {
    t.Parallel()
    cases := []struct {
        path       string
        wantType   catalog.ContentType
        wantMinConf float64
    }{
        {"agents/coding/examples/helper.md", catalog.Agents, 0.75},
        {"examples/agents/my-agent.md", catalog.Agents, 0.70},
        {"examples/skills/my-skill/SKILL.md", catalog.Skills, 0.70},
        {"examples/commands/run-tests.md", catalog.Commands, 0.70},
    }

    d := &TopLevelDetector{}
    for _, tc := range cases {
        t.Run(tc.path, func(t *testing.T) {
            ct, conf, _ := d.matchPattern(tc.path)
            if ct != tc.wantType {
                t.Errorf("matchPattern(%q) type = %v, want %v", tc.path, ct, tc.wantType)
            }
            if conf < tc.wantMinConf {
                t.Errorf("matchPattern(%q) conf = %.2f, want >= %.2f", tc.path, conf, tc.wantMinConf)
            }
        })
    }
}
```

**Implementation** — add to `TopLevelDetector.Patterns()`:

```go
// Quick-win patterns for common non-standard layouts.
{Glob: "agents/*/*/*.md", ContentType: catalog.Agents, Confidence: 0.75},
{Glob: "examples/agents/*.md", ContentType: catalog.Agents, Confidence: 0.70},
{Glob: "examples/skills/*/SKILL.md", ContentType: catalog.Skills, Confidence: 0.70},
{Glob: "examples/commands/*.md", ContentType: catalog.Commands, Confidence: 0.70},
```

Update `TestTopLevelDetector_Patterns` count if it checks `len(pats)`.

### Success Criteria

```
command → pass|fail — description
cd cli && go test ./internal/analyzer/ -run TestTopLevelDetector_QuickWinPatterns → pass — all 4 new patterns match correctly
cd cli && go test ./internal/analyzer/ → pass — no regressions
```

---

## Phase 4: User-Directed Discovery (--scan-as)

### Task 4.1 — --scan-as flag parsing and validation

**Files:**
- `cli/cmd/syllago/generate_manifest.go` — add `--scan-as` and `--strict` flags
- `cli/cmd/syllago/generate_manifest_test.go` — add tests for flag parsing

**Test first:**

```go
func TestGenerateManifest_ScanAsFlag_ValidFormats(t *testing.T) {
    t.Parallel()
    cases := []struct {
        input   string
        wantErr bool
    }{
        {"skills:Packs/", false},
        {"agents:src/bmm/agents/", false},
        {"rules:governance/", false},
        {"invalid-type:path/", true},
        {"skills", true},            // missing colon
        {"skills:", true},           // empty path
        {":path/", true},            // empty type
        {"skills:Packs/ agents:X", true}, // space in single value (should be two flags)
    }
    for _, tc := range cases {
        t.Run(tc.input, func(t *testing.T) {
            _, err := parseScanAsFlag(tc.input)
            if (err != nil) != tc.wantErr {
                t.Errorf("parseScanAsFlag(%q) error=%v, wantErr=%v", tc.input, err, tc.wantErr)
            }
        })
    }
}

func TestGenerateManifest_ScanAsFlag_ConflictRejected(t *testing.T) {
    t.Parallel()
    entries := []string{"skills:Packs/", "agents:Packs/"}
    _, err := validateScanAsEntries(entries)
    if err == nil {
        t.Error("expected error for conflicting type hints on same path")
    }
}

func TestGenerateManifest_ScanAsFlag_PathNotFound(t *testing.T) {
    t.Parallel()
    root := t.TempDir()
    entries := map[string]catalog.ContentType{"nonexistent/": catalog.Skills}
    err := validateScanAsPaths(entries, root)
    if err == nil {
        t.Error("expected error for nonexistent scan-as path")
    }
}
```

**Implementation** — add to `generate_manifest.go`:

```go
var generateManifestScanAs []string
var generateManifestStrict bool

func init() {
    generateManifestCmd.Flags().BoolVar(&generateManifestForce, "force", false, "overwrite existing registry.yaml")
    generateManifestCmd.Flags().StringArrayVar(&generateManifestScanAs, "scan-as", nil,
        "scan path as content type: type:path (e.g. skills:Packs/)")
    generateManifestCmd.Flags().BoolVar(&generateManifestStrict, "strict", false,
        "disable content-signal fallback; fatal on missing scan-as paths")
    // ...
}

// parseScanAsFlag parses a single "type:path" value.
func parseScanAsFlag(s string) (struct{ ct catalog.ContentType; path string }, error) {
    typePart, pathPart, ok := strings.Cut(s, ":")
    if !ok {
        return struct{ ct catalog.ContentType; path string }{}, fmt.Errorf("--scan-as %q: expected type:path format", s)
    }
    typePart = strings.TrimSpace(typePart)
    pathPart = strings.TrimSpace(pathPart)
    if typePart == "" {
        return struct{ ct catalog.ContentType; path string }{}, fmt.Errorf("--scan-as %q: type is empty", s)
    }
    if pathPart == "" {
        return struct{ ct catalog.ContentType; path string }{}, fmt.Errorf("--scan-as %q: path is empty", s)
    }
    ct := catalog.ContentType(typePart)
    if !analyzer.IsValidContentType(ct) {
        return struct{ ct catalog.ContentType; path string }{}, fmt.Errorf("--scan-as %q: unknown content type %q (valid: skills, agents, commands, rules, hooks, mcp)", s, typePart)
    }
    return struct{ ct catalog.ContentType; path string }{ct, pathPart}, nil
}

// validateScanAsEntries parses all --scan-as values and checks for conflicting paths.
func validateScanAsEntries(entries []string) (map[string]catalog.ContentType, error) {
    result := make(map[string]catalog.ContentType, len(entries))
    for _, e := range entries {
        parsed, err := parseScanAsFlag(e)
        if err != nil {
            return nil, err
        }
        if existing, ok := result[parsed.path]; ok && existing != parsed.ct {
            return nil, fmt.Errorf("--scan-as: conflicting type hints for path %q: %v vs %v", parsed.path, existing, parsed.ct)
        }
        result[parsed.path] = parsed.ct
    }
    return result, nil
}

// validateScanAsPaths checks that all scan-as paths exist within repoRoot.
func validateScanAsPaths(paths map[string]catalog.ContentType, repoRoot string) error {
    for p := range paths {
        abs := filepath.Join(repoRoot, p)
        if _, err := os.Stat(abs); err != nil {
            return fmt.Errorf("--scan-as path %q not found: %w", p, err)
        }
    }
    return nil
}
```

Wire into `runGenerateManifest`:

```go
scanAsPaths, err := validateScanAsEntries(generateManifestScanAs)
if err != nil {
    return err
}
if err := validateScanAsPaths(scanAsPaths, absDir); err != nil {
    return err
}

cfg := analyzer.DefaultConfig()
cfg.ScanAsPaths = scanAsPaths
cfg.Strict = generateManifestStrict
```

### Success Criteria

```
command → pass|fail — description
cd cli && go test ./cmd/syllago/ -run TestGenerateManifest_ScanAsFlag → pass — valid formats parse, invalid rejected
cd cli && go test ./cmd/syllago/ -run TestGenerateManifest_ScanAsFlag_ConflictRejected → pass — duplicate paths with different types error
cd cli && make build → pass — binary compiles
```

---

### Task 4.2 — I6: Interactive fallback threshold + interactive CLI discovery surface

**Files:**
- `cli/internal/analyzer/analyzer.go` — expose `ShouldTriggerInteractiveFallback(result)` helper
- `cli/cmd/syllago/generate_manifest.go` — interactive fallback prompt when result has ≤5 items in non-strict, non-json mode
- `cli/internal/analyzer/analyzer_test.go` — threshold tests

**Why:** Design specifies that when the analyzer returns ≤5 items in interactive mode (I6, raised from ≤2 in first panel), the CLI presents a filtered directory tree for the user to select directories and assign content types. Without this, non-standard repos get zero useful output with no guidance.

**Threshold (I6):** 5 items (not 2 — raised in first panel review).

**Implementation sketch:**

```go
// ShouldTriggerInteractiveFallback returns true if the result is sparse enough to
// warrant user-directed discovery. Only triggered in interactive (non-strict, non-json) mode.
func ShouldTriggerInteractiveFallback(result *AnalysisResult) bool {
    return len(result.AllItems()) <= 5
}
```

Interactive CLI flow in `runGenerateManifest` (when `--strict` is false, `--json` is false, and `ShouldTriggerInteractiveFallback` returns true):

1. Print top-level directory tree (filtered: skip Walk-excluded dirs like `node_modules`, `vendor`, `.git`, hidden dirs unless named)
2. Prompt user to select directories with `survey` or `bubbletea` inline prompt (reuse TUI survey patterns)
3. Prompt for content type per selected directory
4. Each selection → add to `ScanAsPaths` → re-run `Analyze` with updated config
5. Propose saving selections to `.syllago.yaml` (confirm prompt)
6. If confirmed, call `SaveScanAsConfig`

**Test:**

```go
func TestShouldTriggerInteractiveFallback(t *testing.T) {
    t.Parallel()
    cases := []struct {
        itemCount int
        want      bool
    }{
        {0, true},
        {3, true},
        {5, true},
        {6, false},
        {20, false},
    }
    for _, tc := range cases {
        result := &AnalysisResult{}
        for range tc.itemCount {
            result.Confirm = append(result.Confirm, &DetectedItem{})
        }
        got := ShouldTriggerInteractiveFallback(result)
        if got != tc.want {
            t.Errorf("itemCount=%d: got %v, want %v", tc.itemCount, got, tc.want)
        }
    }
}
```

**Note:** Full interactive directory tree picker implementation is complex. The threshold helper and CLI prompt wiring should land in this phase. A polished TUI tree picker may be deferred to a TUI polish phase if the team decides to use a simpler numbered-list prompt for v1.

### Success Criteria

```
command → pass|fail — description
cd cli && go test ./internal/analyzer/ -run TestShouldTriggerInteractiveFallback → pass — threshold at 5 items
cd cli && make build → pass — binary compiles
```

---

## Phase 5: Strict Mode + Structured Output

### Task 5.1 — --strict mode enforcement

**Files:**
- `cli/internal/analyzer/analyzer.go` — enforce strict mode behaviors
- `cli/internal/analyzer/analyzer_test.go` — strict mode tests
- `cli/cmd/syllago/generate_manifest.go` — add `--no-config` flag

**Why:** Design specifies `--strict --no-config` suppresses config-file entries entirely. `--strict` alone still honors `.syllago.yaml` scan-as entries (with path validation). `--no-config` is the escape hatch for CI pipelines that want fully explicit control.

**Test first:**

```go
func TestAnalyzer_StrictMode_DisablesFallback(t *testing.T) {
    t.Parallel()
    root := t.TempDir()
    setupFile(t, root, "Packs/redteam/SKILL.md", "---\nname: Red Team\n---\nContent.\n")

    cfg := DefaultConfig()
    cfg.Strict = true
    a := New(cfg)
    result, err := a.Analyze(root)
    if err != nil {
        t.Fatalf("Analyze error: %v", err)
    }
    // In strict mode, content-signal fallback is disabled, so PAI-style layout = 0 items.
    for _, item := range result.AllItems() {
        if item.Provider == "content-signal" {
            t.Errorf("strict mode should disable content-signal fallback, got item %q", item.Name)
        }
    }
}

func TestAnalyzer_StrictMode_500CapFatal(t *testing.T) {
    t.Parallel()
    root := t.TempDir()
    // Simulate 501 unmatched files in an agent-keyword directory.
    // In strict mode this must return a non-nil error.
    for i := range 501 {
        setupFile(t, root, fmt.Sprintf("agents/item-%04d.md", i), "# Item\nContent.\n")
    }
    cfg := DefaultConfig()
    cfg.Strict = true
    a := New(cfg)
    _, err := a.Analyze(root)
    if err == nil {
        t.Error("strict mode: expected fatal error when content-signal candidates exceed 500-file cap")
    }
}

func TestAnalyzer_StrictMode_ConfigFileScanAsPathNotFound(t *testing.T) {
    t.Parallel()
    root := t.TempDir()
    // .syllago.yaml references a path that does not exist.
    os.WriteFile(filepath.Join(root, ".syllago.yaml"),
        []byte("scan-as:\n  - type: skills\n    path: nonexistent/\n"), 0644)

    cfg := DefaultConfig()
    cfg.Strict = true
    a := New(cfg)
    _, err := a.Analyze(root)
    if err == nil {
        t.Error("strict mode: expected fatal error for missing config-file scan-as path")
    }
}

func TestAnalyzer_NoConfig_IgnoresProjectConfig(t *testing.T) {
    t.Parallel()
    root := t.TempDir()
    os.WriteFile(filepath.Join(root, ".syllago.yaml"),
        []byte("scan-as:\n  - type: skills\n    path: Packs/\n"), 0644)
    setupFile(t, root, "Packs/my-skill/SKILL.md", "---\nname: My Skill\n---\nContent.\n")

    cfg := DefaultConfig()
    cfg.NoConfig = true // suppress .syllago.yaml loading
    a := New(cfg)
    result, err := a.Analyze(root)
    if err != nil {
        t.Fatalf("Analyze error: %v", err)
    }
    // Without config, Packs/ is not a registered path — no user-directed boost.
    // Items may still appear via content-signal (SKILL.md filename), but provider
    // should be "content-signal" not user-directed with elevated confidence.
    for _, item := range result.AllItems() {
        if item.Provider == "content-signal" && item.Confidence > 0.70 {
            t.Errorf("--no-config: item %q has elevated confidence %.2f suggesting config was applied", item.Name, item.Confidence)
        }
    }
}
```

**Implementation** — add `NoConfig bool` to `AnalysisConfig`:

```go
type AnalysisConfig struct {
    AutoThreshold float64
    SkipThreshold float64
    ExcludeDirs   []string
    SymlinkPolicy string
    Strict        bool                           // disables content-signal fallback; fatal on cap breach
    NoConfig      bool                           // suppress .syllago.yaml loading (use with --strict for full explicit control)
    ScanAsPaths   map[string]catalog.ContentType // user-directed: path prefix → type
    DebugSkips    bool
    AuditLogger   *audit.Logger
}
```

In `analyzer.go`, before applying `ScanAsPaths`, validate config-file paths in strict mode:

```go
// In strict mode, config-file scan-as paths must exist (same as CLI flag paths).
if a.config.Strict {
    for p := range a.config.ScanAsPaths {
        abs := filepath.Join(repoRoot, p)
        if _, err := os.Stat(abs); err != nil {
            return nil, fmt.Errorf("strict mode: config scan-as path %q not found: %w", p, err)
        }
    }
}
```

In `analyzer.go`, modify the strict mode candidate cap handling:

```go
// Cap content-signal candidates.
const maxContentSignalCandidates = 500
if !a.config.Strict && len(unmatchedPaths) > maxContentSignalCandidates {
    capMsg := fmt.Sprintf("content-signal: %d unmatched files exceeds 500-file cap; scanned first 500. Use --scan-as to target specific directories.", len(unmatchedPaths))
    result.Warnings = append(result.Warnings, capMsg)
    unmatchedPaths = unmatchedPaths[:maxContentSignalCandidates]
} else if a.config.Strict && len(unmatchedPaths) > maxContentSignalCandidates {
    return nil, fmt.Errorf("strict mode: content-signal candidate count %d exceeds 500-file cap; use --scan-as to target directories explicitly", len(unmatchedPaths))
}
```

Add `--no-config` to `generate_manifest.go`:

```go
var generateManifestNoConfig bool

generateManifestCmd.Flags().BoolVar(&generateManifestNoConfig, "no-config", false,
    "suppress .syllago.yaml loading (use with --strict for fully explicit CI control)")
```

Wire `NoConfig` into `runGenerateManifest`: when `generateManifestNoConfig` is true, skip the `LoadScanAsConfig` call and use only CLI-flag paths.

### Success Criteria

```
command → pass|fail — description
cd cli && go test ./internal/analyzer/ -run TestAnalyzer_StrictMode_DisablesFallback → pass — content-signal items absent in strict mode
cd cli && go test ./internal/analyzer/ -run TestAnalyzer_StrictMode_500CapFatal → pass — 501 candidates returns error in strict mode
cd cli && go test ./internal/analyzer/ -run TestAnalyzer_StrictMode_ConfigFileScanAsPathNotFound → pass — missing config-file path is fatal in strict mode
cd cli && go test ./internal/analyzer/ -run TestAnalyzer_NoConfig_IgnoresProjectConfig → pass — .syllago.yaml not applied when NoConfig=true
cd cli && go test ./internal/analyzer/ → pass — no regressions
cd cli && make build → pass — binary compiles
```

---

### Task 5.2 — I8: JSON cap warning + I10: Structured failure JSON

**Files:**
- `cli/cmd/syllago/generate_manifest.go` — emit JSON-parseable cap warning and structured error

**Test first:**

```go
func TestGenerateManifest_JSONCapWarning(t *testing.T) {
    // When --json is set and cap warning is present, warning must be JSON-parseable.
    // Tested via output format check.
    t.Parallel()
    // This is validated in integration; unit test checks the warning format helper.
    msg := formatCapWarning(600, false)
    var v map[string]interface{}
    if err := json.Unmarshal([]byte(msg), &v); err != nil {
        t.Errorf("cap warning is not valid JSON: %v — %s", err, msg)
    }
    if v["type"] != "cap_warning" {
        t.Errorf("cap warning missing type field: %v", v)
    }
}
```

**Implementation** — add to `generate_manifest.go`:

```go
// formatCapWarning returns a JSON-parseable warning string for output.JSON mode,
// or a human-readable string for normal mode.
func formatCapWarning(total int, jsonMode bool) string {
    if jsonMode {
        b, _ := json.Marshal(map[string]interface{}{
            "type":    "cap_warning",
            "message": fmt.Sprintf("content-signal candidate count %d exceeds 500-file cap", total),
            "total":   total,
            "capped":  500,
            "hint":    "use --scan-as to target specific directories",
        })
        return string(b)
    }
    return fmt.Sprintf("warning: content-signal: %d files exceeded 500-file cap; use --scan-as to target directories", total)
}
```

### Success Criteria

```
command → pass|fail — description
cd cli && go test ./cmd/syllago/ -run TestGenerateManifest_JSONCapWarning → pass — cap warning is valid JSON in JSON mode
cd cli && make build → pass — binary compiles
```

---

## Phase 6: Audit Signal Traces

### Task 6.1 — Add signal trace to audit log (I5)

**Files:**
- `cli/internal/audit/audit.go` — add `EventContentSignalClassify` event type + `SignalTrace` fields
- `cli/internal/audit/audit_test.go` — add signal trace test
- `cli/internal/analyzer/analyzer.go` — emit audit event after content-signal classify

**Test first (`audit_test.go`):**

```go
func TestLogger_SignalTrace(t *testing.T) {
    t.Parallel()
    var buf bytes.Buffer
    l := NewLoggerWriter(&buf)
    err := l.Log(Event{
        EventType: EventContentSignalClassify,
        ItemName:  "redteam",
        ItemType:  "skills",
        ContentSignalFile: "Packs/redteam/SKILL.md",
        ContentSignalConfidence: 0.65,
        ContentSignalBucket: "confirm",
        ContentSignalSource: "content-signal",
        ContentSignalStaticSignals: []SignalTrace{
            {Signal: "filename_SKILL.md", Weight: 0.25},
            {Signal: "directory_keyword_pack", Weight: 0.10},
        },
    })
    if err != nil {
        t.Fatalf("Log error: %v", err)
    }
    line := buf.String()
    if !strings.Contains(line, `"event_type":"content-signal.classify"`) {
        t.Errorf("missing event_type in: %s", line)
    }
    if !strings.Contains(line, `"filename_SKILL.md"`) {
        t.Errorf("missing signal name in: %s", line)
    }
    if !strings.Contains(line, `"confidence":0.65`) {
        t.Errorf("missing confidence in: %s", line)
    }
}
```

**Implementation** — add to `audit.go`:

```go
const EventContentSignalClassify EventType = "content-signal.classify"

// SignalTrace records one matched signal in a content-signal classification.
type SignalTrace struct {
    Signal string  `json:"signal"`
    Weight float64 `json:"weight"`
}

// DynamicSignalTrace records a frontmatter field matched against converter registry.
type DynamicSignalTrace struct {
    Field    string `json:"field"`
    Provider string `json:"provider"`
    Matched  bool   `json:"matched"`
}
```

Add fields to `Event`:

```go
// Content-signal classification fields (for EventContentSignalClassify)
ContentSignalFile            string               `json:"file,omitempty"`
ContentSignalConfidence      float64              `json:"confidence,omitempty"`
ContentSignalBucket          string               `json:"bucket,omitempty"`
ContentSignalSource          string               `json:"source,omitempty"`
ContentSignalStaticSignals   []SignalTrace        `json:"signals_static,omitempty"`
ContentSignalDynamicSignals  []DynamicSignalTrace `json:"signals_dynamic_informational,omitempty"`
```

**Wire into analyzer.go** — after `ClassifyUnmatched` returns items, if an audit logger is available:

Add `AuditLogger *audit.Logger` to `AnalysisConfig`. Then after fallback classify loop:

```go
if a.config.AuditLogger != nil && item.Provider == "content-signal" {
    bucket := "confirm"
    _ = a.config.AuditLogger.Log(audit.Event{
        EventType:               audit.EventContentSignalClassify,
        ItemName:                item.Name,
        ItemType:                string(item.Type),
        ContentSignalFile:       item.Path,
        ContentSignalConfidence: item.Confidence,
        ContentSignalBucket:     bucket,
        ContentSignalSource:     "content-signal",
    })
}
```

Note: Signal traces require passing `[]signalEntry` through to the item. Add `SignalTrace []signalEntry` as an unexported field on `DetectedItem` for audit use, populated by `classifyFile`.

### Success Criteria

```
command → pass|fail — description
cd cli && go test ./internal/audit/ -run TestLogger_SignalTrace → pass — signal trace round-trips through JSON correctly
cd cli && go test ./internal/analyzer/ → pass — no regressions from AuditLogger field addition
cd cli && make build → pass — binary compiles
```

---

## Phase 7: Confirm UI Confidence Tiers

### Task 7.1 — Confidence tier labels for confirm bucket (I1)

**Files:**
- `cli/internal/analyzer/types.go` — add `ConfidenceTier` type and `TierForItem` function
- `cli/internal/analyzer/types_test.go` — add tier classification tests

**Why separate from TUI:** The tier logic belongs in the analyzer package where confidence values live, not in TUI rendering. TUI reads the tier and applies visual treatment.

**Test first (`types_test.go`):**

```go
func TestConfidenceTier(t *testing.T) {
    t.Parallel()
    cases := []struct {
        item     *DetectedItem
        wantTier ConfidenceTier
        wantLabel string
    }{
        {&DetectedItem{Confidence: 0.57, Provider: "content-signal"}, TierLow, "Low confidence"},
        {&DetectedItem{Confidence: 0.65, Provider: "content-signal"}, TierMedium, "Medium confidence"},
        {&DetectedItem{Confidence: 0.72, Provider: "content-signal"}, TierHigh, "High confidence"},
        // User-asserted zero-signal: base 0.40 + boost 0.20 = 0.60, zero static signals.
        {&DetectedItem{Confidence: 0.60, Provider: "content-signal", InternalLabel: "user-asserted-zero-signal"}, TierUserAsserted, "User-asserted, no content signals"},
        // Pattern-detected items in confirm bucket (hooks, MCP) — no tier label.
        {&DetectedItem{Confidence: 0.90, Provider: "claude-code", Type: catalog.Hooks}, TierNone, ""},
    }
    for _, tc := range cases {
        t.Run(fmt.Sprintf("%.2f-%s", tc.item.Confidence, tc.item.Provider), func(t *testing.T) {
            tier := TierForItem(tc.item)
            if tier != tc.wantTier {
                t.Errorf("TierForItem() = %v, want %v", tier, tc.wantTier)
            }
            if tc.wantLabel != "" && tier.Label() != tc.wantLabel {
                t.Errorf("Label() = %q, want %q", tier.Label(), tc.wantLabel)
            }
        })
    }
}
```

**Implementation** — add to `types.go`:

```go
// ConfidenceTier categorizes content-signal items for confirm UI display.
type ConfidenceTier int

const (
    TierNone         ConfidenceTier = iota // non-content-signal items
    TierLow                                // 0.55–0.60
    TierMedium                             // 0.60–0.70
    TierHigh                               // 0.70–0.85
    TierUserAsserted                       // user-directed with zero content signals
)

// Label returns the display label for a confidence tier.
func (t ConfidenceTier) Label() string {
    switch t {
    case TierLow:
        return "Low confidence"
    case TierMedium:
        return "Medium confidence"
    case TierHigh:
        return "High confidence"
    case TierUserAsserted:
        return "User-asserted, no content signals"
    default:
        return ""
    }
}

// TierForItem returns the confidence tier for a detected item.
// Only content-signal items get tier labels; pattern-detected items return TierNone.
func TierForItem(item *DetectedItem) ConfidenceTier {
    if item.Provider != "content-signal" {
        return TierNone
    }
    if item.InternalLabel == "user-asserted-zero-signal" {
        return TierUserAsserted
    }
    switch {
    case item.Confidence < 0.60:
        return TierLow
    case item.Confidence < 0.70:
        return TierMedium
    default:
        return TierHigh
    }
}
```

**Mark user-asserted zero-signal items** in `ContentSignalDetector.classifyFile`:

When `userDirected=true` and `best.score == contentSignalBase` (no static signals fired), set `InternalLabel = "user-asserted-zero-signal"` on the item.

### Success Criteria

```
command → pass|fail — description
cd cli && go test ./internal/analyzer/ -run TestConfidenceTier → pass — all tier classifications correct
cd cli && go test ./internal/analyzer/ -run TestConfidenceTier/0.60 → pass — user-asserted zero-signal gets TierUserAsserted, not TierMedium
cd cli && go test ./internal/analyzer/ → pass — no regressions
```

---

### Task 7.2 — Wire confidence tier labels into confirm UI (TUI + CLI)

**Files:**
- `cli/internal/tui/install.go` (or equivalent confirm wizard step) — read `TierForItem()`, render tier label with color coding
- `cli/cmd/syllago/generate_manifest.go` — render tier label in text output confirm list

**Why:** Task 7.1 creates the `ConfidenceTier` data model. Without wiring it into the confirm UI, users see an undifferentiated list — the design explicitly requires visual differentiation (I1). This task closes the rendering gap.

**Visual specification (from design):**

| Score Range | Label | Color |
|-------------|-------|-------|
| 0.55–0.60 | Low confidence | Yellow |
| 0.60–0.70 | Medium confidence | Cyan |
| 0.70–0.85 | High confidence | Green |
| User-asserted, zero signals | "User-asserted, no content signals" | Yellow (special label) |

**TUI implementation sketch:**

In the confirm wizard step's item rendering function, after resolving `item.Name` and `item.Type`:

```go
tier := analyzer.TierForItem(item)
if label := tier.Label(); label != "" {
    // Render tier label with appropriate color below item name.
    // Use Flexoki yellow for Low/UserAsserted, cyan for Medium, green for High.
    tierStyle := styles.WarningStyle // yellow default
    if tier == analyzer.TierMedium {
        tierStyle = styles.InfoStyle // cyan
    } else if tier == analyzer.TierHigh {
        tierStyle = styles.SuccessStyle // green
    }
    row += "\n  " + tierStyle.Render(label)
}
```

**CLI text output sketch:**

In the confirm list printer (used by non-interactive `--json=false` output):

```go
label := analyzer.TierForItem(item).Label()
if label != "" {
    fmt.Fprintf(w, "  [%s]\n", label)
}
```

**Test:**

```go
// TUI golden test — update after implementation:
// cd cli && go test ./internal/tui/ -update-golden -run TestGolden_ConfirmStep_ContentSignalItems

func TestTierForItem_LowConfidenceYellow(t *testing.T) {
    // Verify that a 0.57-confidence content-signal item maps to TierLow.
    // TUI color verification handled by golden test.
    item := &analyzer.DetectedItem{Confidence: 0.57, Provider: "content-signal"}
    if analyzer.TierForItem(item) != analyzer.TierLow {
        t.Errorf("expected TierLow for 0.57 content-signal item")
    }
}
```

**Note:** TUI golden file baseline must be regenerated after this task: `cd cli && go test ./internal/tui/ -update-golden`

### Success Criteria

```
command → pass|fail — description
cd cli && go test ./internal/analyzer/ -run TestTierForItem_LowConfidenceYellow → pass — tier mapping correct
cd cli && go test ./internal/tui/ -run TestGolden_ConfirmStep_ContentSignalItems → pass — golden file matches rendered output with tier labels
cd cli && make build → pass — binary compiles
```

---

## Phase 8: Scan-As Config Persistence

### Task 8.1 — scan-as YAML config loading

**Files:**
- `cli/internal/analyzer/scanconfig.go` — new file for `ScanAsConfig` type and YAML load/save
- `cli/internal/analyzer/scanconfig_test.go` — new test file

**The `.syllago.yaml` format:**

```yaml
scan-as:
  - type: skills
    path: Packs/
  - type: agents
    path: src/bmm/agents/
```

**Test first (`scanconfig_test.go`):**

```go
package analyzer

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestLoadScanAsConfig_Valid(t *testing.T) {
    t.Parallel()
    root := t.TempDir()
    content := "scan-as:\n  - type: skills\n    path: Packs/\n  - type: agents\n    path: src/bmm/agents/\n"
    os.WriteFile(filepath.Join(root, ".syllago.yaml"), []byte(content), 0644)

    cfg, err := LoadScanAsConfig(root)
    if err != nil {
        t.Fatalf("LoadScanAsConfig error: %v", err)
    }
    if len(cfg.ScanAs) != 2 {
        t.Fatalf("expected 2 entries, got %d", len(cfg.ScanAs))
    }
    if cfg.ScanAs[0].Type != catalog.Skills {
        t.Errorf("entry[0].Type = %v, want Skills", cfg.ScanAs[0].Type)
    }
    if cfg.ScanAs[0].Path != "Packs/" {
        t.Errorf("entry[0].Path = %q, want Packs/", cfg.ScanAs[0].Path)
    }
}

func TestLoadScanAsConfig_Missing(t *testing.T) {
    t.Parallel()
    root := t.TempDir()
    cfg, err := LoadScanAsConfig(root)
    if err != nil {
        t.Fatalf("LoadScanAsConfig should succeed on missing file: %v", err)
    }
    if len(cfg.ScanAs) != 0 {
        t.Errorf("expected empty config for missing file, got %d entries", len(cfg.ScanAs))
    }
}

func TestSaveScanAsConfig_RoundTrip(t *testing.T) {
    t.Parallel()
    root := t.TempDir()
    cfg := &ScanAsConfig{
        ScanAs: []ScanAsEntry{
            {Type: catalog.Skills, Path: "Packs/"},
        },
    }
    if err := SaveScanAsConfig(root, cfg); err != nil {
        t.Fatalf("SaveScanAsConfig error: %v", err)
    }
    loaded, err := LoadScanAsConfig(root)
    if err != nil {
        t.Fatalf("LoadScanAsConfig after save error: %v", err)
    }
    if len(loaded.ScanAs) != 1 || loaded.ScanAs[0].Path != "Packs/" {
        t.Errorf("round-trip failed: %+v", loaded.ScanAs)
    }
}

func TestLoadScanAsConfig_InvalidType(t *testing.T) {
    t.Parallel()
    root := t.TempDir()
    content := "scan-as:\n  - type: bogus\n    path: Packs/\n"
    os.WriteFile(filepath.Join(root, ".syllago.yaml"), []byte(content), 0644)
    _, err := LoadScanAsConfig(root)
    if err == nil {
        t.Error("expected error for invalid content type in .syllago.yaml")
    }
}
```

**Implementation (`scanconfig.go`):**

```go
package analyzer

import (
    "errors"
    "io/fs"
    "os"
    "path/filepath"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
    "gopkg.in/yaml.v3"
)

// ScanAsEntry is one path-to-type mapping in .syllago.yaml.
type ScanAsEntry struct {
    Type catalog.ContentType `yaml:"type"`
    Path string              `yaml:"path"`
}

// ScanAsConfig is the parsed content of .syllago.yaml.
type ScanAsConfig struct {
    ScanAs []ScanAsEntry `yaml:"scan-as"`
}

const scanAsConfigFile = ".syllago.yaml"

// LoadScanAsConfig reads .syllago.yaml from repoRoot.
// Returns an empty config if the file does not exist.
func LoadScanAsConfig(repoRoot string) (*ScanAsConfig, error) {
    path := filepath.Join(repoRoot, scanAsConfigFile)
    data, err := os.ReadFile(path)
    if errors.Is(err, fs.ErrNotExist) {
        return &ScanAsConfig{}, nil
    }
    if err != nil {
        return nil, err
    }

    // Intermediate struct with string type for validation.
    var raw struct {
        ScanAs []struct {
            Type string `yaml:"type"`
            Path string `yaml:"path"`
        } `yaml:"scan-as"`
    }
    if err := yaml.Unmarshal(data, &raw); err != nil {
        return nil, err
    }

    cfg := &ScanAsConfig{}
    for _, e := range raw.ScanAs {
        ct := catalog.ContentType(e.Type)
        if !IsValidContentType(ct) {
            return nil, fmt.Errorf(".syllago.yaml: unknown content type %q (valid: skills, agents, commands, rules, hooks, mcp)", e.Type)
        }
        cfg.ScanAs = append(cfg.ScanAs, ScanAsEntry{Type: ct, Path: e.Path})
    }
    return cfg, nil
}

// IsValidContentType checks whether ct is a recognized content type.
// Exported for use by CLI commands in package main.
func IsValidContentType(ct catalog.ContentType) bool {
    for _, valid := range catalog.AllContentTypes() {
        if ct == valid {
            return true
        }
    }
    return false
}

// SaveScanAsConfig writes cfg to .syllago.yaml in repoRoot.
// Uses atomic write (temp file + rename).
func SaveScanAsConfig(repoRoot string, cfg *ScanAsConfig) error {
    // Convert to string-keyed representation for YAML marshal.
    type rawEntry struct {
        Type string `yaml:"type"`
        Path string `yaml:"path"`
    }
    var raw struct {
        ScanAs []rawEntry `yaml:"scan-as"`
    }
    for _, e := range cfg.ScanAs {
        raw.ScanAs = append(raw.ScanAs, rawEntry{Type: string(e.Type), Path: e.Path})
    }
    data, err := yaml.Marshal(raw)
    if err != nil {
        return err
    }
    dest := filepath.Join(repoRoot, scanAsConfigFile)
    tmp := dest + ".tmp"
    if err := os.WriteFile(tmp, data, 0644); err != nil {
        return err
    }
    return os.Rename(tmp, dest)
}

// ToPathMap converts ScanAsConfig entries to the map format used by AnalysisConfig.
func (c *ScanAsConfig) ToPathMap() map[string]catalog.ContentType {
    m := make(map[string]catalog.ContentType, len(c.ScanAs))
    for _, e := range c.ScanAs {
        m[e.Path] = e.Type
    }
    return m
}
```

### Success Criteria

```
command → pass|fail — description
cd cli && go test ./internal/analyzer/ -run TestLoadScanAsConfig → pass — load, missing, invalid type cases all pass
cd cli && go test ./internal/analyzer/ -run TestSaveScanAsConfig_RoundTrip → pass — save and reload produces same entries
cd cli && go test ./internal/analyzer/ → pass — no regressions
```

---

### Task 8.2 — Wire .syllago.yaml into generate manifest command

**Files:**
- `cli/cmd/syllago/generate_manifest.go` — load `.syllago.yaml` and merge with CLI flags

**Test first:**

```go
func TestRunGenerateManifest_LoadsProjectConfig(t *testing.T) {
    // Not parallel — mutates generateManifestCmd flags global state.
    root := t.TempDir()
    os.MkdirAll(filepath.Join(root, "Packs", "my-skill"), 0755)
    os.WriteFile(filepath.Join(root, ".syllago.yaml"), []byte("scan-as:\n  - type: skills\n    path: Packs/\n"), 0644)
    os.WriteFile(filepath.Join(root, "Packs", "my-skill", "SKILL.md"), []byte("---\nname: My Skill\n---\nContent.\n"), 0644)

    generateManifestCmd.Flags().Set("force", "true")
    t.Cleanup(func() { generateManifestCmd.Flags().Set("force", "false") })

    var buf bytes.Buffer
    generateManifestCmd.SetOut(&buf)
    generateManifestCmd.SetErr(&buf)

    err := generateManifestCmd.RunE(generateManifestCmd, []string{root})
    if err != nil {
        t.Fatalf("RunE error: %v", err)
    }
    out := buf.String()
    if !strings.Contains(out, "item") {
        t.Errorf("expected items in output, got: %s", out)
    }
}
```

**Implementation** — in `runGenerateManifest`, after resolving `absDir`:

```go
// Load project scan-as config.
projectScanAs, err := analyzer.LoadScanAsConfig(absDir)
if err != nil {
    return fmt.Errorf("loading .syllago.yaml: %w", err)
}

// Merge: CLI flags are additive on top of config file.
allScanAsPaths := projectScanAs.ToPathMap()
for path, ct := range scanAsPaths { // from --scan-as flags
    if existing, ok := allScanAsPaths[path]; ok && existing != ct {
        return fmt.Errorf("--scan-as %v:%q conflicts with .syllago.yaml entry %v:%q", ct, path, existing, path)
    }
    allScanAsPaths[path] = ct
}

cfg := analyzer.DefaultConfig()
cfg.ScanAsPaths = allScanAsPaths
cfg.Strict = generateManifestStrict
```

### Success Criteria

```
command → pass|fail — description
cd cli && go test ./cmd/syllago/ -run TestRunGenerateManifest_LoadsProjectConfig → pass — .syllago.yaml scan-as entries are applied
cd cli && make build → pass — binary compiles
cd cli && make fmt → pass — no formatting issues
```

---

## Phase 9: Integration Testing + Real-World Validation

### Task 9.1 — Full pipeline integration tests

**Files:**
- `cli/internal/analyzer/integration_test.go` — new file with end-to-end scenarios

```go
package analyzer

import (
    "testing"

    "github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// TestIntegration_PAIStyleLayout simulates the PAI repo structure.
// PAI uses Packs/<name>-skill/SKILL.md for all skills.
func TestIntegration_PAIStyleLayout(t *testing.T) {
    t.Parallel()
    root := t.TempDir()
    skills := []string{
        "Packs/redteam-skill/SKILL.md",
        "Packs/coding-skill/SKILL.md",
        "Packs/research-skill/SKILL.md",
        "Packs/writing-skill/SKILL.md",
        "Packs/analysis-skill/SKILL.md",
    }
    for _, s := range skills {
        setupFile(t, root, s, "---\nname: A Skill\ndescription: Does things\n---\nContent.\n")
    }

    a := New(DefaultConfig())
    result, err := a.Analyze(root)
    if err != nil {
        t.Fatalf("Analyze error: %v", err)
    }
    var skillCount int
    for _, item := range result.AllItems() {
        if item.Type == catalog.Skills {
            skillCount++
        }
    }
    if skillCount < 5 {
        t.Errorf("PAI-style: expected ≥5 skills detected, got %d", skillCount)
    }
    // All content-signal items must land in Confirm, never Auto.
    for _, item := range result.Auto {
        if item.Provider == "content-signal" {
            t.Errorf("content-signal item %q must not be in Auto bucket", item.Name)
        }
    }
}

// TestIntegration_BMADStyleLayout simulates BMAD's agent layout.
func TestIntegration_BMADStyleLayout(t *testing.T) {
    t.Parallel()
    root := t.TempDir()
    agents := []string{
        "src/bmm/agents/orchestrator.agent.yaml",
        "src/bmm/agents/analyst.agent.yaml",
        "src/bmm/agents/writer.agent.yaml",
        "src/bmm/agents/reviewer.agent.yaml",
        "src/bmm/agents/planner.agent.yaml",
    }
    for _, a := range agents {
        setupFile(t, root, a, "name: Agent\ndescription: Does work\n")
    }

    an := New(DefaultConfig())
    result, err := an.Analyze(root)
    if err != nil {
        t.Fatalf("Analyze error: %v", err)
    }
    var agentCount int
    for _, item := range result.AllItems() {
        if item.Type == catalog.Agents {
            agentCount++
        }
    }
    if agentCount < 5 {
        t.Errorf("BMAD-style: expected ≥5 agents detected, got %d", agentCount)
    }
}

// TestIntegration_NoFalsePositivesOnKnownGoodLayouts verifies existing detectors
// still work correctly and content-signal doesn't add duplicate items.
func TestIntegration_NoFalsePositivesOnKnownGoodLayouts(t *testing.T) {
    t.Parallel()
    root := t.TempDir()
    // Standard CC layout.
    setupFile(t, root, ".claude/agents/my-agent.md", "---\nname: My Agent\n---\nAgent body.\n")
    setupFile(t, root, ".claude/skills/my-skill/SKILL.md", "---\nname: My Skill\n---\nContent.\n")
    setupFile(t, root, ".claude/commands/run.md", "---\nallowed-tools: [Bash]\n---\nRun tests.\n")
    setupFile(t, root, "README.md", "# Project\nDocumentation.\n")
    setupFile(t, root, "CHANGELOG.md", "## v1.0.0\nInitial release.\n")

    a := New(DefaultConfig())
    result, err := a.Analyze(root)
    if err != nil {
        t.Fatalf("Analyze error: %v", err)
    }

    // README.md and CHANGELOG.md must not appear.
    for _, item := range result.AllItems() {
        if item.Name == "README" || item.Name == "CHANGELOG" {
            t.Errorf("false positive: %q should not be classified as %v", item.Name, item.Type)
        }
    }

    // No content-signal duplicates for CC-matched files.
    seen := make(map[string][]string)
    for _, item := range result.AllItems() {
        seen[item.Name] = append(seen[item.Name], item.Provider)
    }
    for name, providers := range seen {
        if len(providers) > 1 {
            t.Errorf("item %q detected by multiple providers: %v", name, providers)
        }
    }
}

// TestIntegration_MCPAndHookSignals verifies JSON wiring file detection.
func TestIntegration_MCPAndHookSignals(t *testing.T) {
    t.Parallel()
    root := t.TempDir()
    // Non-standard MCP config.
    setupFile(t, root, "config/mcp-servers.json",
        `{"mcpServers": {"myserver": {"command": "npx", "args": ["-y", "@myserver/mcp"]}}}`)
    // Non-standard hooks wiring.
    setupFile(t, root, "hooks/wiring.json",
        `{"hooks": {"PreToolUse": [{"command": "bash hooks/lint.sh"}], "PostToolUse": [{"command": "echo done"}]}}`)

    a := New(DefaultConfig())
    result, err := a.Analyze(root)
    if err != nil {
        t.Fatalf("Analyze error: %v", err)
    }
    var mcpCount, hookCount int
    for _, item := range result.AllItems() {
        switch item.Type {
        case catalog.MCP:
            mcpCount++
        case catalog.Hooks:
            hookCount++
        }
    }
    if mcpCount == 0 {
        t.Error("expected MCP item from non-standard mcp-servers.json")
    }
    if hookCount == 0 {
        t.Error("expected Hooks item from non-standard hooks wiring JSON")
    }
}

// TestIntegration_ScanAsPathBypass tests user-directed discovery bypasses pre-filter.
func TestIntegration_ScanAsPathBypass(t *testing.T) {
    t.Parallel()
    root := t.TempDir()
    // Files in a directory with NO keyword in name — normally pre-filter would reject.
    setupFile(t, root, "library/catalog/item-one.md", "---\nname: Item One\n---\nContent.\n")
    setupFile(t, root, "library/catalog/item-two.md", "---\nname: Item Two\n---\nContent.\n")

    cfg := DefaultConfig()
    cfg.ScanAsPaths = map[string]catalog.ContentType{
        "library/catalog/": catalog.Skills,
    }
    a := New(cfg)
    result, err := a.Analyze(root)
    if err != nil {
        t.Fatalf("Analyze error: %v", err)
    }
    var found int
    for _, item := range result.AllItems() {
        if item.Type == catalog.Skills {
            found++
        }
    }
    if found < 2 {
        t.Errorf("expected ≥2 skills from user-directed scan, got %d", found)
    }
    // User-directed items should have elevated confidence (> 0.55).
    for _, item := range result.AllItems() {
        if item.Provider == "content-signal" && item.Confidence <= 0.55 {
            t.Errorf("user-directed item %q confidence %.2f should be elevated", item.Name, item.Confidence)
        }
    }
}
```

### Success Criteria

```
command → pass|fail — description
cd cli && go test ./internal/analyzer/ -run TestIntegration_PAIStyleLayout → pass — ≥5 skills detected from PAI layout
cd cli && go test ./internal/analyzer/ -run TestIntegration_BMADStyleLayout → pass — ≥5 agents detected from BMAD layout
cd cli && go test ./internal/analyzer/ -run TestIntegration_NoFalsePositivesOnKnownGoodLayouts → pass — README.md not classified, no duplicate providers
cd cli && go test ./internal/analyzer/ -run TestIntegration_MCPAndHookSignals → pass — MCP and Hooks detected from non-standard JSON
cd cli && go test ./internal/analyzer/ -run TestIntegration_ScanAsPathBypass → pass — user-directed scan finds items in non-keyword directory
cd cli && go test ./internal/analyzer/ → pass — full suite passes
cd cli && make build → pass — binary compiles
```

---

### Task 9.2 — I9: --debug-skips flag

**Files:**
- `cli/cmd/syllago/generate_manifest.go` — add `--debug-skips` flag
- `cli/internal/analyzer/types.go` — add `SkipReason` type
- `cli/internal/analyzer/analyzer.go` — collect skip reasons when flag is set

**The four skip cases (I10):**
- `pre_filter_excluded` — file rejected by extension or directory keyword filter
- `below_threshold` — file passed pre-filter but scored < 0.55
- `locked_conflict` — file's type conflicts with locked org-config entry
- `walk_skipped` — file excluded by Walk (node_modules, vendor, etc.)

**Implementation sketch** — add to `AnalysisConfig`:

```go
DebugSkips bool // collect per-file skip reasons for --debug-skips output
```

Add to `AnalysisResult`:

```go
SkipReasons []SkipEntry // populated when DebugSkips=true
```

Where:

```go
type SkipEntry struct {
    Path   string `json:"path"`
    Reason string `json:"reason"` // one of: pre_filter_excluded, below_threshold, locked_conflict, walk_skipped
}
```

In the content-signal detector's `ClassifyUnmatched`, when `DebugSkips` is propagated, append a `SkipEntry` for each rejected file. In `runGenerateManifest`, if `--debug-skips` is set, print skip entries to stderr (or JSON if `--json`).

**Test:**

```go
func TestAnalyzer_DebugSkips_CollectsPreFilterExclusions(t *testing.T) {
    t.Parallel()
    root := t.TempDir()
    setupFile(t, root, "docs/overview.md", "# Overview\nDocumentation.\n") // no keyword
    setupFile(t, root, "agents/helper.go", "package agents\n")             // wrong extension

    cfg := DefaultConfig()
    cfg.DebugSkips = true
    a := New(cfg)
    result, err := a.Analyze(root)
    if err != nil {
        t.Fatalf("Analyze error: %v", err)
    }
    var preFilterSkips int
    for _, s := range result.SkipReasons {
        if s.Reason == "pre_filter_excluded" {
            preFilterSkips++
        }
    }
    if preFilterSkips == 0 {
        t.Error("expected pre_filter_excluded skip entries with debug-skips enabled")
    }
}
```

### Success Criteria

```
command → pass|fail — description
cd cli && go test ./internal/analyzer/ -run TestAnalyzer_DebugSkips → pass — skip reasons collected when DebugSkips=true
cd cli && go test ./internal/analyzer/ → pass — full suite passes
cd cli && make build → pass — binary compiles
cd cli && make fmt → pass — no formatting issues
```

---

## Scope Boundaries

### Org-Config (I3) — Deferred

**Design requirement I3** (org-config support for `--scan-as` mappings with `locked` and `deny-scan-as`) is **not implemented in this plan**. Rationale:

- Second panel review explicitly noted: "Org-config deployment out of scope for v1 — operators provision via existing MDM/dotfiles toolchain."
- The layered-config model (org baseline → project additive → CLI additive) is architecturally sound but requires IT-managed deployment tooling not available at v1 launch.
- The `locked_conflict` skip case in I10/`--debug-skips` will be stubbed as a recognized skip reason without enforcement (since locked entries require org-config loading).

**Deferred to follow-up feature:** `feat(config): org-config.yaml with locked scan-as and deny-scan-as enforcement`

### Correction Workflow — Deferred

**Design panel note** specifies `syllago manifest edit --remove <path>` as the correction workflow for wrong approvals. This requires a new `manifest edit` CLI subcommand not in the current CLI surface.

**Deferred to follow-up feature:** `feat(manifest): manifest edit --remove for correcting wrong approvals`

### User/Org Config Boundary — Documented

Per design: `locked` mappings live in org-config (`~/.syllago/org-config.yaml`), `ignore` entries (rejected confirm items) live in user-config (`.syllago.yaml`). These cannot conflict by design — org-config is read-only from the project's perspective. This boundary is enforced at v1 by the absence of org-config loading; project config is the top layer.

## Commit Checkpoints

Each phase should be committed separately. Suggested commit messages:

- Phase 1: `fix(analyzer): path containment in resolveHookScript, sanitize all DetectedItem string fields`
- Phase 2: `feat(analyzer): ContentSignalDetector with weighted signal scoring`
- Phase 3: `feat(analyzer): quick-win patterns for examples/ and nested agent dirs`
- Phase 4: `feat(manifest): --scan-as flag for user-directed discovery`
- Phase 5: `feat(manifest): --strict mode, --no-config flag, and JSON-parseable cap warnings`
- Phase 6: `feat(audit): content-signal classification trace events`
- Phase 7: `feat(analyzer): ConfidenceTier type and confirm UI tier label rendering`
- Phase 8: `feat(config): .syllago.yaml scan-as persistence`
- Phase 9: `test(analyzer): full integration test suite for content-signal fallback`

Before each commit: `cd cli && make fmt && make test && make build`

---

## Dependencies Between Tasks

```
1.1 (resolveHookScript) → standalone (no deps)
1.2 (sanitize) → standalone (no deps)
1.3 (README exclusion) → standalone (no deps)
1.4 (vocabulary) → standalone, must complete before 2.2

2.1 (signal tests) → requires 1.4 (vocabulary.go must exist for test compilation)
2.2 (ContentSignalDetector impl) → requires 2.1, 1.4
2.3 (wire into pipeline) → requires 2.2, AnalysisConfig.Strict + ScanAsPaths fields

3.1 (quick-win patterns) → standalone (parallel with Phase 2)

4.1 (--scan-as flag) → requires 2.3 (ScanAsPaths in AnalysisConfig)
4.2 (interactive fallback threshold + CLI surface) → requires 4.1, 8.2 (SaveScanAsConfig)

5.1 (strict mode + --no-config) → requires 2.3 (Strict in AnalysisConfig), 8.1 (for config-file path validation)
5.2 (JSON warnings) → requires 5.1

6.1 (audit traces) → requires 2.2 (signalEntry type), 2.3 (AuditLogger in config)

7.1 (confidence tiers) → requires 2.2 (InternalLabel "user-asserted-zero-signal")
7.2 (TUI + CLI tier rendering) → requires 7.1 (TierForItem function)

8.1 (scan-as YAML) → requires catalog.ParseContentType (existing)
8.2 (wire YAML into manifest) → requires 8.1, 4.1

9.1 (integration tests) → requires all phases complete
9.2 (debug-skips) → requires 2.2 (ClassifyUnmatched), 2.3 (DebugSkips in config)
```

---

## Coverage Targets

Per-package minimums (80% minimum, 95% aspirational):

| Package | New files | Critical paths to cover |
|---------|-----------|------------------------|
| `analyzer` | `sanitize.go`, `vocabulary.go`, `detector_content_signal.go`, `scanconfig.go` | All signal branches, pre-filter logic, score capping, tier classification |
| `cmd/syllago` | additions to `generate_manifest.go` | Flag parsing, conflict detection, path validation |
| `audit` | additions to `audit.go` | Signal trace serialization |

Check after each phase: `cd cli && go test ./internal/analyzer/... -coverprofile=cov.out && go tool cover -func=cov.out | grep total`
