# Rules Splitter & Monolithic-File Install — Implementation Plan

**Status:** Ready for execution
**Date:** 2026-04-24
**Scope:** V1 ship-blocking feature
**Source of truth for decisions:** [`docs/plans/2026-04-23-rules-splitter-decisions.md`](./2026-04-23-rules-splitter-decisions.md) (D1–D21, closed)

This plan translates the 21 design decisions into a bite-sized, TDD-structured task list. Every task is a Red → Green → Commit unit sized at 2–5 minutes of focused work. The design doc is the decision record; this plan is the sequence. If anything here appears to contradict the design doc, the design doc wins — treat that as a bug in this plan and fix the plan.

Feature recap: discover monolithic rule files (CLAUDE.md, AGENTS.md, GEMINI.md, .cursorrules, .clinerules, .windsurfrules) across project + home; split into atomic rules or import whole; store in library with per-rule `.history/sha256-<hex>.md` versions; install either as individual files or appended to a provider's monolithic file with exact-match uninstall and a Fresh/Clean/Modified verification scan — no in-file ownership markers.

---

## Phase index

1. [Phase 1 — Foundation](#phase-1--foundation) (D1, D2, D12, D13, D19 source fixtures)
2. [Phase 2 — Splitter core](#phase-2--splitter-core) (D3, D4, D19 expected outputs)
3. [Phase 3 — Library storage with history](#phase-3--library-storage-with-history) (D11, D13 loader, helpers + lint test)
4. [Phase 4 — Add wizard adaptation](#phase-4--add-wizard-adaptation) (D18, D4, D3)
5. [Phase 5 — Append-to-monolithic install method](#phase-5--append-to-monolithic-install-method) (D5, D6, D10, D14, D20 append)
6. [Phase 6 — Exact-match uninstall + verification scan](#phase-6--exact-match-uninstall--verification-scan) (D7, D16, D20 search, **D21 ship gate**)
7. [Phase 7 — Update flow](#phase-7--update-flow) (D17, D20 replace)
8. [Phase 8 — TUI install picker](#phase-8--tui-install-picker) (D5, D10)
9. [Phase 9 — Library "Installed" status surface](#phase-9--library-installed-status-surface) (D16 UI)
10. [Phase 10 — `split-rules-llm` skill in syllago-meta-registry](#phase-10--split-rules-llm-skill-in-syllago-meta-registry) (D9)

### Dependency graph

Linear spine: 1 → 2 → 3 → 5 → 6 → 7 → 9.

Parallelizable pairs (per design doc §"V1 phase plan"):

- **Phase 2 ∥ Phase 4** — once Phase 1's foundation lands, splitter core (2) and the wizard UI (4) touch different packages and can proceed in parallel. Phase 4 needs `[]SplitCandidate` types from Phase 2 but can stub them until merge.
- **Phase 4 ∥ Phase 6 ∥ Phase 7** — after Phase 5 ships the install method, the TUI add-wizard work (4) and the verify/uninstall work (6) and the update flow (7) are disjoint code surfaces. 7 depends on 6's `verification_state` function signature, not its full implementation.
- **Phase 10 ∥ Phases 2–9** — the `split-rules-llm` skill is an entirely separate repository and code surface. It can proceed in parallel with the whole core implementation, but V1 ship requires both Phase 10 deliverables done (skill authored + published to syllago-meta-registry) so `syllago add split-rules-llm` resolves.

Hard ship gates:

- **Phase 6** is not done until D21's 10-cell roundtrip matrix at `cli/internal/installer/roundtrip_test.go` passes (all 10 cells, no skips).
- **Phase 10** is not done until `syllago add split-rules-llm` resolves from syllago-meta-registry against a real install.

---

## Phase 1 — Foundation

Scope: D1 provenance fields on the rule metadata struct, D13 YAML schema plumbing (struct + loader), D2 per-directory discovery walk with `.git` boundary stop, D12 canonical normalization helper, and D19's 17 synthesized fixture *source* files plus `REFERENCES.md`. Expected-output halves of the fixtures are deferred to Phase 2 because the splitter's `[]SplitCandidate` shape must exist before asserting against it.

### Task 1.1: Scaffold the `canonical` package

**Depends on:** —
**Files:** `cli/internal/converter/canonical/canonical.go`, `cli/internal/converter/canonical/canonical_test.go`
**Step 1 (test):** Create `canonical_test.go` with `TestNormalize_Empty` asserting `Normalize(nil)` returns `[]byte{'\n'}` (single trailing newline per D12).
**Step 2 (expect fail):** `go test ./internal/converter/canonical/` — undefined package.
**Step 3 (implement):** Create `canonical.go`:
```go
// Package canonical provides the single normalization helper used at
// write, scan, and search time for all monolithic-file byte paths (D12).
package canonical

import "bytes"

// Normalize applies the canonical byte form per D12:
// - CRLF -> LF
// - Strip leading UTF-8 BOM
// - Exactly one trailing newline
// Trailing whitespace on lines, indentation, unicode, and casing are preserved.
func Normalize(b []byte) []byte {
    b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
    b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF})
    b = bytes.TrimRight(b, "\n")
    return append(b, '\n')
}
```
**Step 4 (pass):** `cd cli && go test ./internal/converter/canonical/` — PASS.
**Step 5 (commit):** `git commit -m "feat(canonical): scaffold Normalize helper (D12)"`

### Task 1.2: Cover the D12 normalization rules with a table-driven test

**Depends on:** 1.1
**Files:** `cli/internal/converter/canonical/canonical_test.go`
**Step 1 (test):** Add a `TestNormalize_Cases` table with these cells, each asserting `Normalize(input) == expected`:
- `crlf`: input `"a\r\nb\r\n"`, expected `"a\nb\n"`
- `bom`: input `"\xEF\xBB\xBFhello\n"`, expected `"hello\n"`
- `no_trailing_newline`: input `"hello"`, expected `"hello\n"`
- `double_trailing_newline`: input `"hello\n\n"`, expected `"hello\n"`
- `preserve_two_trailing_spaces`: input `"line  \n"`, expected `"line  \n"` (byte-equal)
- `preserve_tabs`: input `"\thello\n"`, expected `"\thello\n"`
- `preserve_unicode`: input `"café\n"` bytes, expected byte-identical (no NFC/NFD)
- `preserve_heading_case`: input `"# FOO\n"`, expected `"# FOO\n"`
- `empty_to_newline`: input `""`, expected `"\n"`

**Step 2 (expect fail if implementation is wrong):** Run tests; all should pass since 1.1's implementation follows D12 exactly. If any fails, fix `canonical.go`.
**Step 3 (implement):** Already done in 1.1. Adjust `canonical.go` only if a cell fails.
**Step 4 (pass):** `cd cli && go test ./internal/converter/canonical/ -run TestNormalize_Cases -v` — all 9 cells PASS.
**Step 5 (commit):** `git commit -m "test(canonical): table-driven D12 normalization rules"`

### Task 1.3: Add the `RuleMetadata` + `RuleVersionEntry` + `RuleSource` types

**Depends on:** —
**Files:** `cli/internal/metadata/rule.go`, `cli/internal/metadata/rule_test.go`
**Step 1 (test):** Create `rule_test.go` with `TestRuleMetadata_YAMLRoundtrip` that marshals a fully-populated `RuleMetadata` struct matching D13's example, unmarshals it, and asserts the round-trip is byte-equal.
**Step 2 (expect fail):** undefined type.
**Step 3 (implement):** Create `rule.go`:
```go
package metadata

import "time"

// RuleSource is the provenance block for a library rule (D1, D13).
type RuleSource struct {
    Provider         string    `yaml:"provider"`
    Scope            string    `yaml:"scope"` // "project" | "global"
    Path             string    `yaml:"path"`
    Format           string    `yaml:"format"`
    Filename         string    `yaml:"filename"`
    Hash             string    `yaml:"hash"` // canonical "<algo>:<64-hex>" per D11
    SplitMethod      string    `yaml:"split_method"` // h2|h3|h4|marker|single|llm
    SplitFromSection string    `yaml:"split_from_section,omitempty"`
    CapturedAt       time.Time `yaml:"captured_at"`
}

// RuleVersionEntry is one entry in the .syllago.yaml versions[] list (D13).
type RuleVersionEntry struct {
    Hash      string    `yaml:"hash"` // canonical "<algo>:<64-hex>" per D11
    WrittenAt time.Time `yaml:"written_at"`
}

// RuleMetadata is the on-disk .syllago.yaml shape for library rules (D13).
// Distinct from Meta because rule source metadata has ~9 fields that
// benefit from nesting under a source: block (D13 "Why nested source:").
type RuleMetadata struct {
    FormatVersion  int                `yaml:"format_version"`
    ID             string             `yaml:"id"`
    Name           string             `yaml:"name"`
    Description    string             `yaml:"description,omitempty"`
    Type           string             `yaml:"type"` // always "rule"
    AddedAt        time.Time          `yaml:"added_at"`
    AddedBy        string             `yaml:"added_by,omitempty"`
    Source         RuleSource         `yaml:"source"`
    Versions       []RuleVersionEntry `yaml:"versions"`
    CurrentVersion string             `yaml:"current_version"` // must match a versions[].hash
}
```
**Step 4 (pass):** `cd cli && go test ./internal/metadata/ -run TestRuleMetadata_YAMLRoundtrip` — PASS.
**Step 5 (commit):** `git commit -m "feat(metadata): add RuleMetadata schema (D1, D13)"`

### Task 1.4: Loader — reject malformed hashes

**Depends on:** 1.3
**Files:** `cli/internal/metadata/rule.go`, `cli/internal/metadata/rule_test.go`
**Step 1 (test):** Add `TestLoadRuleMetadata_RejectsMalformedHash` with cases for: missing algo prefix (`"abc123..."`), wrong algo length (`"sha256:abc"` 3 hex), extra characters (`"sha256:" + 65 hex`), colon→dash in YAML (`"sha256-abc..."`, 64 hex). Each must return an error containing the substring `"invalid hash format"`.
**Step 2 (expect fail):** `LoadRuleMetadata` undefined.
**Step 3 (implement):** Add to `rule.go`:
```go
import (
    "fmt"
    "os"
    "regexp"
    "gopkg.in/yaml.v3"
)

var canonicalHashRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// LoadRuleMetadata reads and validates a .syllago.yaml file as a RuleMetadata.
func LoadRuleMetadata(path string) (*RuleMetadata, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("reading %s: %w", path, err)
    }
    var m RuleMetadata
    if err := yaml.Unmarshal(data, &m); err != nil {
        return nil, fmt.Errorf("parsing %s: %w", path, err)
    }
    // Hash format invariant (D11).
    for i, v := range m.Versions {
        if !canonicalHashRe.MatchString(v.Hash) {
            return nil, fmt.Errorf("%s: invalid hash format in versions[%d]: %q (want sha256:<64-hex>)", path, i, v.Hash)
        }
    }
    if !canonicalHashRe.MatchString(m.CurrentVersion) {
        return nil, fmt.Errorf("%s: invalid hash format in current_version: %q (want sha256:<64-hex>)", path, m.CurrentVersion)
    }
    return &m, nil
}
```
**Step 4 (pass):** `cd cli && go test ./internal/metadata/ -run TestLoadRuleMetadata_RejectsMalformedHash` — PASS.
**Step 5 (commit):** `git commit -m "feat(metadata): validate canonical hash format on load (D11)"`

### Task 1.5: Loader — enforce `current_version` points to an existing `versions[]` entry

**Depends on:** 1.4
**Files:** `cli/internal/metadata/rule.go`, `cli/internal/metadata/rule_test.go`
**Step 1 (test):** Add `TestLoadRuleMetadata_CurrentVersionMustExist` with two cases: (a) `current_version` references a hash not in `versions[]` — expect error substring `"current_version references missing hash"`, (b) empty `versions[]` with a non-empty `current_version` — same error.
**Step 2 (expect fail):** loader does not yet check.
**Step 3 (implement):** In `LoadRuleMetadata`, after hash-format validation:
```go
found := false
for _, v := range m.Versions {
    if v.Hash == m.CurrentVersion {
        found = true
        break
    }
}
if !found {
    return nil, fmt.Errorf("%s: current_version references missing hash %q", path, m.CurrentVersion)
}
```
**Step 4 (pass):** `cd cli && go test ./internal/metadata/ -run TestLoadRuleMetadata_CurrentVersionMustExist` — PASS.
**Step 5 (commit):** `git commit -m "feat(metadata): enforce current_version → versions[] invariant (D13)"`

### Task 1.6: Scaffold the `discover` package for monolithic-rule files

**Depends on:** —
**Files:** `cli/internal/discover/discover.go`, `cli/internal/discover/discover_test.go`
**Step 1 (test):** Create `discover_test.go` with `TestDiscoverMonolithicRules_Empty` that creates an empty `t.TempDir()` and asserts `DiscoverMonolithicRules(tmp, "", []string{"CLAUDE.md"})` returns a zero-length slice, no error.
**Step 2 (expect fail):** undefined package.
**Step 3 (implement):** Create `discover.go`:
```go
// Package discover finds monolithic rule files (CLAUDE.md, AGENTS.md,
// GEMINI.md, .cursorrules, .clinerules, .windsurfrules) under a project
// root and the user's home directory (D2).
package discover

import (
    "io/fs"
    "os"
    "path/filepath"
)

// Candidate is one discovered monolithic rule file.
type Candidate struct {
    AbsPath  string // absolute path to the file
    Scope    string // "project" | "global"
    Filename string // basename (e.g. "CLAUDE.md")
}

// DiscoverMonolithicRules walks projectRoot for any filename in filenames,
// stopping at nested .git boundaries, plus checks homeDir for the same set
// at its root. Each match becomes one Candidate. Symlinks are followed.
// homeDir may be "" to skip the global scan.
func DiscoverMonolithicRules(projectRoot, homeDir string, filenames []string) ([]Candidate, error) {
    set := make(map[string]struct{}, len(filenames))
    for _, f := range filenames {
        set[f] = struct{}{}
    }
    var out []Candidate
    if projectRoot != "" {
        if err := filepath.WalkDir(projectRoot, func(p string, d fs.DirEntry, err error) error {
            if err != nil {
                return nil // skip unreadable subtree
            }
            if d.IsDir() {
                // Stop at nested .git boundaries (but not at projectRoot/.git — same dir).
                if p != projectRoot && d.Name() == ".git" {
                    return fs.SkipDir
                }
                return nil
            }
            if _, ok := set[d.Name()]; ok {
                abs, _ := filepath.Abs(p)
                out = append(out, Candidate{AbsPath: abs, Scope: "project", Filename: d.Name()})
            }
            return nil
        }); err != nil {
            return nil, err
        }
    }
    if homeDir != "" {
        for name := range set {
            candidate := filepath.Join(homeDir, name)
            if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
                abs, _ := filepath.Abs(candidate)
                out = append(out, Candidate{AbsPath: abs, Scope: "global", Filename: name})
            }
        }
    }
    return out, nil
}
```
**Step 4 (pass):** `cd cli && go test ./internal/discover/ -run TestDiscoverMonolithicRules_Empty` — PASS.
**Step 5 (commit):** `git commit -m "feat(discover): scaffold monolithic rule discovery (D2)"`

### Task 1.7: Discovery walks nested directories and returns each match

**Depends on:** 1.6
**Files:** `cli/internal/discover/discover_test.go`
**Step 1 (test):** Add `TestDiscoverMonolithicRules_NestedDirs` that creates `tmp/CLAUDE.md`, `tmp/apps/web/CLAUDE.md`, `tmp/apps/api/AGENTS.md`, then calls `DiscoverMonolithicRules(tmp, "", []string{"CLAUDE.md", "AGENTS.md"})` and asserts exactly 3 candidates, all with `Scope == "project"`, filename matches its parent dir.
**Step 2 (expect fail if logic wrong):** run test — should pass if 1.6's walk is correct.
**Step 3 (implement):** No new code expected. If a cell fails, fix 1.6.
**Step 4 (pass):** `cd cli && go test ./internal/discover/ -run TestDiscoverMonolithicRules_NestedDirs` — PASS.
**Step 5 (commit):** `git commit -m "test(discover): nested-dir multi-match coverage (D2)"`

### Task 1.8: Discovery stops at nested `.git` boundaries

**Depends on:** 1.7
**Files:** `cli/internal/discover/discover_test.go`
**Step 1 (test):** Add `TestDiscoverMonolithicRules_GitBoundary`. Layout: `tmp/CLAUDE.md` (findable), `tmp/vendor/sub/.git/` (marker dir), `tmp/vendor/sub/CLAUDE.md` (inside a nested git repo — MUST be skipped). Assert exactly 1 candidate at `tmp/CLAUDE.md`.
**Step 2 (expect fail):** run if 1.6's `.git` check is wrong.
**Step 3 (implement):** If failing, the fix belongs in 1.6's `WalkDir` closure — verify `d.Name() == ".git"` returns `fs.SkipDir`.
**Step 4 (pass):** `cd cli && go test ./internal/discover/ -run TestDiscoverMonolithicRules_GitBoundary` — PASS.
**Step 5 (commit):** `git commit -m "test(discover): nested .git boundary stop (D2)"`

### Task 1.9: Discovery includes home-directory matches with `global` scope

**Depends on:** 1.7
**Files:** `cli/internal/discover/discover_test.go`
**Step 1 (test):** Add `TestDiscoverMonolithicRules_HomeScope`. Create a fake home dir in `t.TempDir()`, write `fakeHome/CLAUDE.md`, call `DiscoverMonolithicRules("", fakeHome, []string{"CLAUDE.md"})`, assert one candidate with `Scope == "global"` and `AbsPath` pointing to `fakeHome/CLAUDE.md`.
**Step 2 (expect fail):** should already pass from 1.6.
**Step 3 (implement):** Fix 1.6 if needed.
**Step 4 (pass):** `cd cli && go test ./internal/discover/ -run TestDiscoverMonolithicRules_HomeScope` — PASS.
**Step 5 (commit):** `git commit -m "test(discover): home-dir global-scope coverage (D2)"`

### Task 1.10: Provider monolithic-filename table

**Depends on:** —
**Files:** `cli/internal/provider/monolithic.go`, `cli/internal/provider/monolithic_test.go`
**Step 1 (test):** Create `monolithic_test.go` with `TestMonolithicFilenames` asserting `MonolithicFilenames("claude-code") == []string{"CLAUDE.md"}`, `MonolithicFilenames("codex") == []string{"AGENTS.md"}`, `MonolithicFilenames("gemini-cli") == []string{"GEMINI.md"}`, `MonolithicFilenames("cursor") == []string{".cursorrules"}`, `MonolithicFilenames("cline") == []string{".clinerules"}`, `MonolithicFilenames("windsurf") == []string{".windsurfrules"}`, `MonolithicFilenames("unknown") == nil`.
**Step 2 (expect fail):** undefined function.
**Step 3 (implement):** Create `monolithic.go`:
```go
package provider

// MonolithicFilenames returns the set of monolithic rule filenames that the
// provider identified by slug authors at project or home scope (D2, §research).
// Each slug may have one or more. Returns nil for unknown slugs.
func MonolithicFilenames(slug string) []string {
    switch slug {
    case "claude-code":
        return []string{"CLAUDE.md"}
    case "codex":
        return []string{"AGENTS.md"}
    case "gemini-cli":
        return []string{"GEMINI.md"}
    case "cursor":
        return []string{".cursorrules"}
    case "cline":
        return []string{".clinerules"}
    case "windsurf":
        return []string{".windsurfrules"}
    }
    return nil
}

// AllMonolithicFilenames returns the union of monolithic filenames across
// every provider that has one. Used by discovery to build a single filename
// filter across all providers.
func AllMonolithicFilenames() []string {
    return []string{"CLAUDE.md", "AGENTS.md", "GEMINI.md", ".cursorrules", ".clinerules", ".windsurfrules"}
}
```
**Step 4 (pass):** `cd cli && go test ./internal/provider/ -run TestMonolithicFilenames` — PASS.
**Step 5 (commit):** `git commit -m "feat(provider): monolithic filename table per provider (D2)"`

### Task 1.11: Provider hints table for monolithic-install NOTE output

**Depends on:** 1.10
**Files:** `cli/internal/provider/monolithic.go`, `cli/internal/provider/monolithic_test.go`
**Step 1 (test):** Add `TestMonolithicHint` asserting:
- `MonolithicHint("codex")` → `"Codex prefers per-directory AGENTS.md files; consider installing per directory rather than as a single root file."`
- `MonolithicHint("windsurf")` → `"Windsurf has a 6KB limit on this file; the file rules format (.windsurf/rules/) is recommended for non-trivial content."`
- `MonolithicHint("claude-code")` → `""` (no hint).
**Step 2 (expect fail):** undefined function.
**Step 3 (implement):** Append to `monolithic.go`:
```go
// MonolithicHint returns a one-line, non-blocking hint for providers with
// strong conventions around monolithic-file install (D10). Empty string
// means the provider has no special guidance.
func MonolithicHint(slug string) string {
    switch slug {
    case "codex":
        return "Codex prefers per-directory AGENTS.md files; consider installing per directory rather than as a single root file."
    case "windsurf":
        return "Windsurf has a 6KB limit on this file; the file rules format (.windsurf/rules/) is recommended for non-trivial content."
    }
    return ""
}
```
**Step 4 (pass):** `cd cli && go test ./internal/provider/ -run TestMonolithicHint` — PASS.
**Step 5 (commit):** `git commit -m "feat(provider): hint table for monolithic install (D10)"`

### Task 1.12: Create fixture directory + `REFERENCES.md`

**Depends on:** —
**Files:** `cli/internal/converter/testdata/splitter/REFERENCES.md`
**Step 1 (test):** No new test; this is a doc asset.
**Step 2 (expect fail):** n/a.
**Step 3 (implement):** Write `REFERENCES.md` with the two-column table mapping each fixture filename to the reference from the design doc §"Fixture authoring guide". Structure:
```markdown
# Splitter fixture references

These fixtures are synthesized (content fabricated). Their structural shape
is modeled after the real-world references below for coverage traceability.
No third-party content is committed.

## CLAUDE.md / AGENTS.md / GEMINI.md shaped

| Fixture | Modeled after | Structural shape captured |
|---|---|---|
| h2-clean.md | saaspegasus/pegasus-docs CLAUDE.md | ~45L, 7 H2s, no preamble |
| h2-with-preamble.md | steadycursor/steadystart CLAUDE.md | ~142L, 9 H2s, 4-line preamble |
| h2-numbered-prefix.md | nammayatri/nammayatri .clinerules | ## 1., ## 2. patterns |
| h2-emoji-prefix.md | grahama1970/claude-code-mcp-enhanced CLAUDE.md | emoji-prefixed headings, slug normalization |
| h3-deep.md | kubernetes/kops AGENTS.md | 118L, 3 H2 / 11 H3 |
| h4-rare.md | payloadcms/payload CLAUDE.md | H4 splitting case |
| marker-literal.md | (synthesized) | literal custom-marker shape |
| too-small.md | victrme/Bonjourr AGENTS.md | <30L skip-split trigger |
| no-h2.md | (synthesized) | 0 H2s skip-split trigger |
| delegating-stub.md | pathintegral-institute/mcpm.sh GEMINI.md | 1-line delegation |
| table-heavy.md | DataDog/lading AGENTS.md | tables-in-content stress |
| decorative-hr.md | p33m5t3r/vibecoding/conway CLAUDE.md | standalone --- as decoration |
| must-should-may.md | pingcap/tidb AGENTS.md | mandate-language casing preservation |
| trailing-whitespace.md | (synthesized for D12) | two trailing spaces on a line |
| crlf-line-endings.md | (synthesized for D12) | CRLF throughout |
| bom-prefix.md | (synthesized for D12) | leading UTF-8 BOM |
| no-trailing-newline.md | (synthesized for D12) | missing final newline |
| import-line.md | (synthesized for D4) | @import line preservation |

## .cursorrules shaped

| Fixture | Modeled after | Structural shape captured |
|---|---|---|
| cursorrules-flat-numbered.md | level09/enferno | numbered flat list, anti-fixture for "don't split" |
| cursorrules-points-elsewhere.md | uhop/stream-json | medium, points to AGENTS.md |

## .clinerules shaped

| Fixture | Modeled after | Structural shape captured |
|---|---|---|
| clinerules-numbered-h2.md | nammayatri/nammayatri | 105L, numbered ## N. Topic H2s |

## .windsurfrules shaped

| Fixture | Modeled after | Structural shape captured |
|---|---|---|
| windsurfrules-pointer.md | SAP/fundamental-ngx | 1-line pointer — nonsensical to split |
| windsurfrules-numbered-rules.md | level09/enferno | 17 numbered rules |
```
**Step 4 (pass):** File exists; no test.
**Step 5 (commit):** `git commit -m "docs(splitter): REFERENCES.md for synthesized fixtures (D19)"`

### Tasks 1.13–1.29: Author each fixture source file (one task per fixture)

For each task below, the rhythm is:
- **Step 1 (test):** `os.Stat` on the target fixture path inside a tiny table-driven `TestFixturesPresent` lives in Phase 2; here we just author the file.
- **Step 2 (expect fail):** n/a — asset creation.
- **Step 3 (implement):** Write the fixture file with the structural properties listed.
- **Step 4 (pass):** `ls cli/internal/converter/testdata/splitter/<fixture>.md` shows the file.
- **Step 5 (commit):** one commit per fixture, message `fixture(splitter): <fixture-name> (D19)`.

Each fixture MUST begin with the HTML comment `<!-- modeled after: <reference> -->` per D19's last implication.

| # | Task | Fixture file | Required structural properties |
|---|---|---|---|
| 1.13 | Author `h2-clean.md` | `cli/internal/converter/testdata/splitter/h2-clean.md` | ≥30 lines, ≥3 H2s, no preamble, no H3 nesting |
| 1.14 | Author `h2-with-preamble.md` | `cli/internal/converter/testdata/splitter/h2-with-preamble.md` | 4-line preamble then ≥3 H2s |
| 1.15 | Author `h2-numbered-prefix.md` | `cli/internal/converter/testdata/splitter/h2-numbered-prefix.md` | H2s of shape `## 1. Foo`, `## 2. Bar`, `## 3. Baz` |
| 1.16 | Author `h2-emoji-prefix.md` | `cli/internal/converter/testdata/splitter/h2-emoji-prefix.md` | H2s of shape `## 🚀 Foo`, `## 🔧 Bar`, `## 📦 Baz` |
| 1.17 | Author `h3-deep.md` | `cli/internal/converter/testdata/splitter/h3-deep.md` | 3 H2s + ≥11 H3s nested under them |
| 1.18 | Author `h4-rare.md` | `cli/internal/converter/testdata/splitter/h4-rare.md` | H2 > H3 > H4 nesting where H4 is the meaningful split unit |
| 1.19 | Author `marker-literal.md` | `cli/internal/converter/testdata/splitter/marker-literal.md` | Content separated by literal `===SYLLAGO-SPLIT===` lines |
| 1.20 | Author `too-small.md` | `cli/internal/converter/testdata/splitter/too-small.md` | <30 lines (skip-split trigger) |
| 1.21 | Author `no-h2.md` | `cli/internal/converter/testdata/splitter/no-h2.md` | ≥30 lines but 0 H2s (skip-split trigger) |
| 1.22 | Author `delegating-stub.md` | `cli/internal/converter/testdata/splitter/delegating-stub.md` | 1-line pointer to another file |
| 1.23 | Author `table-heavy.md` | `cli/internal/converter/testdata/splitter/table-heavy.md` | Markdown tables and pipe-heavy lines mid-section |
| 1.24 | Author `decorative-hr.md` | `cli/internal/converter/testdata/splitter/decorative-hr.md` | Standalone `---` lines used decoratively between paragraphs (NOT as splits) |
| 1.25 | Author `must-should-may.md` | `cli/internal/converter/testdata/splitter/must-should-may.md` | Body prose using MUST/SHOULD/MAY in caps; splitter must not re-case |
| 1.26 | Author `trailing-whitespace.md` | `cli/internal/converter/testdata/splitter/trailing-whitespace.md` | At least one line ending with two trailing spaces then `\n` (hand-author bytes carefully) |
| 1.27 | Author `crlf-line-endings.md` | `cli/internal/converter/testdata/splitter/crlf-line-endings.md` | File written with CRLF line endings throughout. Use `os.WriteFile` via a small Go helper script or a CRLF-preserving editor. Include a comment at file top noting "CRLF throughout". |
| 1.28 | Author `bom-prefix.md` | `cli/internal/converter/testdata/splitter/bom-prefix.md` | First three bytes are `EF BB BF` (UTF-8 BOM), then normal markdown |
| 1.29 | Author `no-trailing-newline.md` | `cli/internal/converter/testdata/splitter/no-trailing-newline.md` | File does NOT end in `\n` (final byte is a non-newline character) |
| 1.30 | Author `import-line.md` | `cli/internal/converter/testdata/splitter/import-line.md` | Body contains an `@import other/rules.md` line that must pass through unchanged |
| 1.31 | Author `cursorrules-flat-numbered.md` | `cli/internal/converter/testdata/splitter/cursorrules-flat-numbered.md` | Flat numbered list (1. 2. 3.), no headings — anti-fixture |
| 1.32 | Author `cursorrules-points-elsewhere.md` | `cli/internal/converter/testdata/splitter/cursorrules-points-elsewhere.md` | Mid-sized, mixed structure, content points user to an AGENTS.md file |
| 1.33 | Author `clinerules-numbered-h2.md` | `cli/internal/converter/testdata/splitter/clinerules-numbered-h2.md` | ~105 lines, H2s of shape `## 1. Topic` through `## 5. Topic` |
| 1.34 | Author `windsurfrules-pointer.md` | `cli/internal/converter/testdata/splitter/windsurfrules-pointer.md` | 1-line pointer file |
| 1.35 | Author `windsurfrules-numbered-rules.md` | `cli/internal/converter/testdata/splitter/windsurfrules-numbered-rules.md` | ~17 numbered rules (1. through 17.), no headings |

**Task 1.36: Fixture presence sanity test**
**Depends on:** 1.13–1.35
**Files:** `cli/internal/converter/testdata_test.go`
**Step 1 (test):** Create a top-level-in-converter `TestSplitterFixturesPresent` that loops over a slice of expected filenames and asserts `os.Stat` returns no error for each at `testdata/splitter/<name>`.
**Step 2 (expect fail):** passes if 1.13–1.35 committed fully; fails loudly if any missing.
**Step 3 (implement):** slice literal with every fixture name listed above, `t.Run(name, …)` per entry.
**Step 4 (pass):** `cd cli && go test ./internal/converter/ -run TestSplitterFixturesPresent` — PASS.
**Step 5 (commit):** `git commit -m "test(splitter): fixture-presence sanity check (D19)"`

### Task 1.37: Build + fmt gate for Phase 1

**Depends on:** 1.1–1.36
**Files:** —
**Step 1 (test):** `cd cli && make fmt && make build && make test` — all PASS.
**Step 2 (expect fail):** n/a — verification only.
**Step 3 (implement):** Fix any format or vet errors surfaced.
**Step 4 (pass):** all green.
**Step 5 (commit):** only if changes were needed; otherwise skip.

---

## Phase 2 — Splitter core

Scope: implement the deterministic splitter per D3 (H2 default + H3/H4 opt-in + literal custom marker) and D4 (skip-split detection, header promotion, numbered-prefix stripping, preamble handling, `@import` preservation). Output type `SplitCandidate{Name, Description, Body, OriginalRange}`. Author the expected-output halves of Phase 1's fixtures here — tests live alongside the splitter.

### Task 2.1: Scaffold the `splitter` package with the public contract types

**Depends on:** Phase 1
**Files:** `cli/internal/splitter/splitter.go`, `cli/internal/splitter/splitter_test.go`
**Step 1 (test):** Create `splitter_test.go` with `TestSplitCandidate_ZeroValue` asserting `(SplitCandidate{}).Name == ""`.
**Step 2 (expect fail):** undefined type.
**Step 3 (implement):** Create `splitter.go`:
```go
// Package splitter splits monolithic rule files (CLAUDE.md, AGENTS.md,
// GEMINI.md, .cursorrules, .clinerules, .windsurfrules) into atomic
// SplitCandidates for library storage. Deterministic path per D3/D4;
// LLM path is a parallel producer (D9) that returns the same type.
package splitter

// SplitCandidate is one atomic rule produced by the splitter.
// Downstream pipeline (library write, install) is indifferent to which
// heuristic (H2/H3/H4/marker/single/LLM) produced the candidate.
type SplitCandidate struct {
    Name          string // slug, suitable for library dir name
    Description   string // original heading text (pre-slugify) or "" for whole-file imports
    Body          string // candidate body bytes (canonical form applied by the caller)
    OriginalRange [2]int // [start_line, end_line_exclusive) in the source file
}

// Heuristic selects the split mode.
type Heuristic int

const (
    HeuristicH2      Heuristic = iota // default — split at every ##
    HeuristicH3                       // split at every ###
    HeuristicH4                       // split at every ####
    HeuristicMarker                   // split at a literal-string line match
    HeuristicSingle                   // no split — import as single rule
)

// Options controls splitter behavior for a single call.
type Options struct {
    Heuristic     Heuristic
    MarkerLiteral string // only used when Heuristic == HeuristicMarker
}

// SkipSplitSignal is returned alongside an empty candidate slice when the
// splitter determines the file is not a good split target per D4:
// - fewer than 30 lines, OR
// - fewer than 3 H2 headings
// The wizard surfaces this as a "import as single rule" suggestion;
// the CLI errors out unless --split=single is passed.
type SkipSplitSignal struct {
    Reason string // "too_small" | "too_few_h2"
}
```
**Step 4 (pass):** `cd cli && go test ./internal/splitter/` — PASS.
**Step 5 (commit):** `git commit -m "feat(splitter): scaffold public contract (D3)"`

### Task 2.2: `Split()` entrypoint signature + skip-split detection

**Depends on:** 2.1
**Files:** `cli/internal/splitter/splitter.go`, `cli/internal/splitter/splitter_test.go`
**Step 1 (test):** Add `TestSplit_SkipSplitTooSmall` using the `too-small.md` fixture: call `Split(bytes, Options{Heuristic: HeuristicH2})`, assert returned slice is nil AND the `*SkipSplitSignal` return is non-nil with `Reason == "too_small"`. Same for `no-h2.md` with `Reason == "too_few_h2"`.
**Step 2 (expect fail):** undefined function.
**Step 3 (implement):** Append to `splitter.go`:
```go
import "bytes"

// Split returns atomic SplitCandidates according to opts, or a SkipSplitSignal
// when D4's skip-split heuristic fires (fewer than 30 lines OR fewer than 3
// H2 headings). Only one of the two returns is non-nil.
func Split(source []byte, opts Options) ([]SplitCandidate, *SkipSplitSignal) {
    if opts.Heuristic == HeuristicSingle {
        return []SplitCandidate{{
            Name:          "", // caller provides slug for whole-file import
            Description:   "",
            Body:          string(source),
            OriginalRange: [2]int{0, bytes.Count(source, []byte{'\n'}) + 1},
        }}, nil
    }
    lines := bytes.Split(source, []byte{'\n'})
    if len(lines) < 30 {
        return nil, &SkipSplitSignal{Reason: "too_small"}
    }
    h2Count := 0
    for _, ln := range lines {
        if bytes.HasPrefix(ln, []byte("## ")) && !bytes.HasPrefix(ln, []byte("### ")) {
            h2Count++
        }
    }
    if h2Count < 3 {
        return nil, &SkipSplitSignal{Reason: "too_few_h2"}
    }
    // Real split logic lives in follow-up tasks; stub returns empty success.
    return nil, nil
}
```
**Step 4 (pass):** `cd cli && go test ./internal/splitter/ -run TestSplit_SkipSplitTooSmall` — PASS.
**Step 5 (commit):** `git commit -m "feat(splitter): skip-split detection (D4)"`

### Task 2.3: H2 splitter — clean case (no preamble, no nesting)

**Depends on:** 2.2
**Files:** `cli/internal/splitter/splitter.go`, `cli/internal/splitter/splitter_h2_test.go`
**Step 1 (test):** Create `splitter_h2_test.go` with `TestSplit_H2Clean`. Load `h2-clean.md`, call `Split(body, Options{Heuristic: HeuristicH2})`, assert: 3 candidates, each `Description` matches the H2's literal text, each `Body` begins with `# <heading>\n` (header promotion per D4), each `OriginalRange` spans the expected line range for that H2.
**Step 2 (expect fail):** current stub returns `(nil, nil)`.
**Step 3 (implement):** Replace the stub's final `return nil, nil` with a real H2 walk:
```go
type section struct {
    headingLine   int
    headingText   string
    bodyStart     int
    bodyEnd       int
}
var sections []section
cursor := 0
for i, ln := range lines {
    if bytes.HasPrefix(ln, []byte("## ")) && !bytes.HasPrefix(ln, []byte("### ")) {
        if len(sections) > 0 {
            sections[len(sections)-1].bodyEnd = i
        }
        sections = append(sections, section{
            headingLine: i,
            headingText: string(bytes.TrimPrefix(ln, []byte("## "))),
            bodyStart:   i,
        })
        cursor = i
    }
}
_ = cursor
if len(sections) > 0 {
    sections[len(sections)-1].bodyEnd = len(lines)
}
out := make([]SplitCandidate, 0, len(sections))
for _, s := range sections {
    body := rebuildBody(lines[s.bodyStart:s.bodyEnd], s.headingText)
    out = append(out, SplitCandidate{
        Name:          slugify(s.headingText),
        Description:   s.headingText,
        Body:          body,
        OriginalRange: [2]int{s.headingLine, s.bodyEnd},
    })
}
return out, nil
```
Add helpers `slugify` and `rebuildBody`:
```go
import (
    "regexp"
    "strings"
)

var nonSlugRe = regexp.MustCompile(`[^a-z0-9]+`)

// slugify lowercases, strips numbered prefixes ("1. " -> ""), and replaces
// runs of non [a-z0-9] with "-". Called on the heading text only.
func slugify(heading string) string {
    h := strings.TrimSpace(heading)
    // Strip leading "N. " or "N." numbered prefix (D4).
    if m := regexp.MustCompile(`^\d+\.\s*`).FindString(h); m != "" {
        h = h[len(m):]
    }
    h = strings.ToLower(h)
    h = nonSlugRe.ReplaceAllString(h, "-")
    h = strings.Trim(h, "-")
    return h
}

// rebuildBody promotes the section heading from ## to # and returns the body
// as a single string (D4 header promotion).
func rebuildBody(sectionLines [][]byte, headingText string) string {
    if len(sectionLines) == 0 {
        return ""
    }
    var sb strings.Builder
    sb.WriteString("# ")
    sb.WriteString(strings.TrimSpace(headingText))
    sb.WriteByte('\n')
    // Append body lines after the heading.
    for _, ln := range sectionLines[1:] {
        sb.Write(ln)
        sb.WriteByte('\n')
    }
    return sb.String()
}
```
**Step 4 (pass):** `cd cli && go test ./internal/splitter/ -run TestSplit_H2Clean` — PASS.
**Step 5 (commit):** `git commit -m "feat(splitter): H2 clean split + header promotion + slugify (D3, D4)"`

### Task 2.4: H2 splitter — preamble attaches to first split

**Depends on:** 2.3
**Files:** `cli/internal/splitter/splitter.go`, `cli/internal/splitter/splitter_h2_test.go`
**Step 1 (test):** Add `TestSplit_H2WithPreamble` using `h2-with-preamble.md`. Assert the first candidate's `Body` contains the 4-line preamble text (below the promoted H1), and subsequent candidates do NOT.
**Step 2 (expect fail):** current logic drops preamble.
**Step 3 (implement):** Track a `preambleLines [][]byte` slice for lines before the first H2, then prepend them to the first section's body inside `rebuildBody` (or pass a new argument). Minimum diff:
```go
// After identifying sections, capture preamble:
var preamble [][]byte
if len(sections) > 0 && sections[0].headingLine > 0 {
    preamble = lines[0:sections[0].headingLine]
}
// Pass to the first body construction:
for i, s := range sections {
    var pre [][]byte
    if i == 0 {
        pre = preamble
    }
    body := rebuildBodyWithPreamble(lines[s.bodyStart:s.bodyEnd], s.headingText, pre)
    ...
}
```
Implement `rebuildBodyWithPreamble` that writes `# <heading>\n`, then preamble lines (if any), then body lines after the heading.
**Step 4 (pass):** `cd cli && go test ./internal/splitter/ -run TestSplit_H2WithPreamble` — PASS.
**Step 5 (commit):** `git commit -m "feat(splitter): preamble attaches to first split (D4)"`

### Task 2.5: H2 splitter — numbered-prefix stripping in slug

**Depends on:** 2.3
**Files:** `cli/internal/splitter/splitter_h2_test.go`
**Step 1 (test):** Add `TestSplit_H2NumberedPrefix` using `h2-numbered-prefix.md`. Assert: slugs are `coding-style`, not `1-coding-style`; `Description` retains the literal `"1. Coding Style"` so the user sees original text in the review step.
**Step 2 (expect fail):** should pass from 2.3's slugify.
**Step 3 (implement):** Fix `slugify` if failing.
**Step 4 (pass):** `cd cli && go test ./internal/splitter/ -run TestSplit_H2NumberedPrefix` — PASS.
**Step 5 (commit):** `git commit -m "test(splitter): numbered-prefix slug stripping (D4)"`

### Task 2.6: H2 splitter — emoji-prefix headings slugify cleanly

**Depends on:** 2.3
**Files:** `cli/internal/splitter/splitter_h2_test.go`
**Step 1 (test):** Add `TestSplit_H2EmojiPrefix` using `h2-emoji-prefix.md`. Assert emoji is stripped from slug (`🚀 Foo` → `foo`), but `Description` preserves the emoji.
**Step 2 (expect fail):** if `slugify` doesn't drop non-ASCII, it fails.
**Step 3 (implement):** The current `nonSlugRe = [^a-z0-9]+` already strips emoji because they fall outside `[a-z0-9]`. If the test fails for unicode-range reasons, tighten `slugify` to ASCII-lowercase.
**Step 4 (pass):** `cd cli && go test ./internal/splitter/ -run TestSplit_H2EmojiPrefix` — PASS.
**Step 5 (commit):** `git commit -m "test(splitter): emoji-prefix slug normalization (D4)"`

### Task 2.7: H2 splitter — `@import` lines preserved verbatim

**Depends on:** 2.3
**Files:** `cli/internal/splitter/splitter_h2_test.go`
**Step 1 (test):** Add `TestSplit_H2ImportLine` using `import-line.md`. Assert whichever split contains the `@import` line has it verbatim in its `Body` (byte-equal on that line).
**Step 2 (expect fail):** should pass — rebuildBody writes all body lines unchanged.
**Step 3 (implement):** Fix rebuildBody if failing.
**Step 4 (pass):** `cd cli && go test ./internal/splitter/ -run TestSplit_H2ImportLine` — PASS.
**Step 5 (commit):** `git commit -m "test(splitter): @import preservation (D4)"`

### Task 2.8: H2 splitter does not split on decorative `---`

**Depends on:** 2.3
**Files:** `cli/internal/splitter/splitter_h2_test.go`
**Step 1 (test):** Add `TestSplit_H2DecorativeHR` using `decorative-hr.md`. Assert: decorative `---` lines are preserved inside whichever section they fall in; no extra candidates created.
**Step 2 (expect fail):** should already pass — `---` is not an H2 line.
**Step 3 (implement):** Fix only if failing.
**Step 4 (pass):** PASS.
**Step 5 (commit):** `git commit -m "test(splitter): decorative HR not a split point (D3)"`

### Task 2.9: H2 splitter — must/should/may casing preservation

**Depends on:** 2.3
**Files:** `cli/internal/splitter/splitter_h2_test.go`
**Step 1 (test):** Add `TestSplit_H2MustShouldMay` using `must-should-may.md`. Assert words `MUST`, `SHOULD`, `MAY` appear in their original case in every candidate `Body`. (Slugs lower-case headings but bodies are byte-preserving.)
**Step 2 (expect fail):** should pass.
**Step 3 (implement):** Fix only if failing.
**Step 4 (pass):** PASS.
**Step 5 (commit):** `git commit -m "test(splitter): MUST/SHOULD/MAY casing preservation (D4)"`

### Task 2.10: H3 splitter

**Depends on:** 2.3
**Files:** `cli/internal/splitter/splitter.go`, `cli/internal/splitter/splitter_h3_test.go`
**Step 1 (test):** Create `splitter_h3_test.go` with `TestSplit_H3Deep` using `h3-deep.md`, `Options{Heuristic: HeuristicH3}`. Assert number of candidates equals the H3 count in the fixture (≥11).
**Step 2 (expect fail):** current splitter only looks at `## `.
**Step 3 (implement):** Generalize `Split` to take a heading-prefix derived from the heuristic:
```go
func headingPrefix(h Heuristic) []byte {
    switch h {
    case HeuristicH2: return []byte("## ")
    case HeuristicH3: return []byte("### ")
    case HeuristicH4: return []byte("#### ")
    }
    return nil
}
```
Replace the hard-coded `## ` checks with `headingPrefix(opts.Heuristic)` comparisons; also adjust the skip-split detection to count matches for the chosen heuristic (H3/H4 still compare against H2 count for skip-split — stick with D4's literal "fewer than 3 H2 headings" rule, which is documented only for H2 default mode; for H3/H4 opt-in, skip-split fires only on the `<30 lines` branch).
**Step 4 (pass):** `cd cli && go test ./internal/splitter/ -run TestSplit_H3Deep` — PASS.
**Step 5 (commit):** `git commit -m "feat(splitter): H3 heuristic (D3)"`

### Task 2.11: H4 splitter

**Depends on:** 2.10
**Files:** `cli/internal/splitter/splitter_h4_test.go`
**Step 1 (test):** Add `TestSplit_H4Rare` using `h4-rare.md`, `Options{Heuristic: HeuristicH4}`. Assert candidates count equals the H4 count in the fixture.
**Step 2 (expect fail):** passes from 2.10 if heading-prefix helper is wired correctly.
**Step 3 (implement):** Fix only if failing.
**Step 4 (pass):** PASS.
**Step 5 (commit):** `git commit -m "test(splitter): H4 opt-in (D3)"`

### Task 2.12: Literal-marker splitter

**Depends on:** 2.10
**Files:** `cli/internal/splitter/splitter.go`, `cli/internal/splitter/splitter_marker_test.go`
**Step 1 (test):** Create `splitter_marker_test.go` with `TestSplit_MarkerLiteral` using `marker-literal.md`, `Options{Heuristic: HeuristicMarker, MarkerLiteral: "===SYLLAGO-SPLIT==="}`. Assert candidates equal the number of marker-separated regions.
**Step 2 (expect fail):** no marker branch in `Split`.
**Step 3 (implement):** Add a marker branch inside `Split` before the heading-prefix logic:
```go
if opts.Heuristic == HeuristicMarker {
    if opts.MarkerLiteral == "" {
        return nil, nil
    }
    return splitByMarker(lines, opts.MarkerLiteral), nil
}
```
Implement `splitByMarker` that scans for exact-match lines, accumulates regions between markers, and emits one candidate per region with `Name: ""`, `Description: ""` (literal marker has no heading), `Body` = region bytes, `OriginalRange` = region boundaries. The first region (preamble before the first marker) is still emitted as a candidate.
**Step 4 (pass):** `cd cli && go test ./internal/splitter/ -run TestSplit_MarkerLiteral` — PASS.
**Step 5 (commit):** `git commit -m "feat(splitter): literal-marker heuristic (D3)"`

### Task 2.13: Delegating stub skip-split

**Depends on:** 2.2
**Files:** `cli/internal/splitter/splitter_test.go`
**Step 1 (test):** Add `TestSplit_DelegatingStub` using `delegating-stub.md`. Assert `*SkipSplitSignal{Reason: "too_small"}` returned.
**Step 2 (expect fail):** should pass.
**Step 3 (implement):** Fix only if failing.
**Step 4 (pass):** PASS.
**Step 5 (commit):** `git commit -m "test(splitter): delegating stub → skip-split (D4)"`

### Task 2.14: Table-heavy content preserved inside sections

**Depends on:** 2.3
**Files:** `cli/internal/splitter/splitter_h2_test.go`
**Step 1 (test):** Add `TestSplit_H2TableHeavy` using `table-heavy.md`. Assert: markdown tables present in the source (detect via `|---|`) are present inside whichever section contains them, byte-preserved.
**Step 2 (expect fail):** should pass.
**Step 3 (implement):** Fix only if failing.
**Step 4 (pass):** PASS.
**Step 5 (commit):** `git commit -m "test(splitter): markdown tables preserved in sections"`

### Task 2.15: Normalization-sensitive fixtures behave correctly under the splitter

**Depends on:** 2.3
**Files:** `cli/internal/splitter/splitter_norm_test.go`
**Step 1 (test):** Create `splitter_norm_test.go` with a table-driven `TestSplit_NormalizationFixtures` over `crlf-line-endings.md`, `bom-prefix.md`, `no-trailing-newline.md`, `trailing-whitespace.md`. For each: read raw bytes, call `canonical.Normalize`, then `Split` on the normalized bytes, assert the split count is correct for the fixture's structural shape (document expected counts in the fixture authoring task). Trailing-whitespace fixture: assert bodies preserve the two trailing spaces line byte-for-byte.
**Step 2 (expect fail):** may pass or fail depending on how lines are compared.
**Step 3 (implement):** If failing, route input bytes through `canonical.Normalize` inside the tests (splitter itself need not call `Normalize`; callers do — library write path handles that in Phase 3).
**Step 4 (pass):** `cd cli && go test ./internal/splitter/ -run TestSplit_NormalizationFixtures` — PASS.
**Step 5 (commit):** `git commit -m "test(splitter): normalization-sensitive fixtures under canonical input (D12)"`

### Task 2.16: `.cursorrules`/`.windsurfrules`/`.clinerules` format-specific fixtures

**Depends on:** 2.3, 2.10, 2.12
**Files:** `cli/internal/splitter/splitter_formats_test.go`
**Step 1 (test):** One subtest per format-specific fixture from Phase 1 (1.31–1.35). For `cursorrules-flat-numbered.md` and `windsurfrules-numbered-rules.md` (anti-fixtures), assert `H2` heuristic returns `*SkipSplitSignal{Reason: "too_few_h2"}`. For `clinerules-numbered-h2.md`, assert H2 produces 5 candidates with numbered-stripped slugs. For `windsurfrules-pointer.md`, assert `*SkipSplitSignal{Reason: "too_small"}`. For `cursorrules-points-elsewhere.md`, assert H2 output matches the file's actual H2 count.
**Step 2 (expect fail):** should pass if splitter is correct.
**Step 3 (implement):** Fix only if failing.
**Step 4 (pass):** PASS.
**Step 5 (commit):** `git commit -m "test(splitter): format-specific fixtures (D19)"`

### Task 2.17: Build + fmt gate for Phase 2

**Depends on:** 2.1–2.16
**Files:** —
**Step 1 (test):** `cd cli && make fmt && make build && make test` — PASS.
**Step 2–5:** As in 1.37.

---

## Phase 3 — Library storage with history

Scope: per-rule directory layout (D11), YAML schema wiring (D13), `hashToFilename` / `filenameToHash` helpers with a grep-fails-the-build lint test, load-time orphan invariant, and the in-memory `history map[string][]byte` population path. This phase is the concrete implementation of D8's storage contract (what must be stored: current canonical body + all prior canonical versions + original-format source bytes + metadata index) — D8 establishes what to store; D11 specifies the layout; this phase builds it.

### Task 3.1: Scaffold the `rulestore` package + hash helpers

**Depends on:** Phase 1 (canonical, metadata)
**Files:** `cli/internal/rulestore/hash.go`, `cli/internal/rulestore/hash_test.go`
**Step 1 (test):** Create `hash_test.go` with:
- `TestHashToFilename` — `hashToFilename("sha256:" + strings.Repeat("a", 64))` returns `"sha256-" + strings.Repeat("a", 64) + ".md"`.
- `TestFilenameToHash_Valid` — inverse roundtrip.
- `TestFilenameToHash_Malformed` — inputs missing extension, missing dash, wrong hex length, wrong algo each return an error whose `.Error()` contains `"malformed history filename"`.
**Step 2 (expect fail):** undefined package.
**Step 3 (implement):** Create `hash.go`:
```go
// Package rulestore is the on-disk persistence layer for library rules (D11).
package rulestore

import (
    "fmt"
    "regexp"
    "strings"
)

var filenameHashRe = regexp.MustCompile(`^sha256-[0-9a-f]{64}\.md$`)

// hashToFilename converts a canonical "<algo>:<hex>" hash into its
// .history/<algo>-<hex>.md filename (D11). No error return: operates
// only on already-validated canonical hashes.
func hashToFilename(hash string) string {
    // Single conversion: `:` -> `-`, append ".md".
    return strings.Replace(hash, ":", "-", 1) + ".md"
}

// filenameToHash converts a .history filename back to its canonical
// "<algo>:<hex>" form (D11). Returns an error for any malformed filename
// so the loader can fail with a specific load error.
func filenameToHash(name string) (string, error) {
    if !filenameHashRe.MatchString(name) {
        return "", fmt.Errorf("malformed history filename: %q (want sha256-<64-hex>.md)", name)
    }
    trimmed := strings.TrimSuffix(name, ".md")
    return strings.Replace(trimmed, "-", ":", 1), nil
}
```
**Step 4 (pass):** `cd cli && go test ./internal/rulestore/` — PASS.
**Step 5 (commit):** `git commit -m "feat(rulestore): hash helpers with filename conversion (D11)"`

### Task 3.2: Lint test — grep-fails-the-build for raw hash formatting

**Depends on:** 3.1
**Files:** `cli/internal/rulestore/hashlint_test.go`
**Step 1 (test):** Create `hashlint_test.go`. Walk every `.go` file under `cli/` (exclude `_test.go` and the `rulestore` package itself where the single sanctioned use lives) and fail the test if any line matches `"sha256-"` (substring) or `"\"sha256:\" +"` (substring). Record allowed call sites inside `rulestore/hash.go` and `rulestore/loader.go` (the loader's single format-validation entry point).
```go
package rulestore

import (
    "os"
    "path/filepath"
    "strings"
    "testing"
)

// TestNoRawHashFormatting enforces the D11 hash-format invariant: the
// canonical "<algo>:<hex>" string is the only thing code passes around.
// Anywhere code touches the filesystem layer it must go through
// hashToFilename / filenameToHash. This test grep-fails the build on the
// two common bug patterns:
//   (a) "sha256-" outside rulestore/hash.go (ad-hoc filename construction)
//   (b) `"sha256:" + `  outside rulestore/loader.go (ad-hoc canonical string)
func TestNoRawHashFormatting(t *testing.T) {
    root, err := filepath.Abs("../..") // cli/
    if err != nil { t.Fatal(err) }
    allowedFilenameConstruction := map[string]bool{
        filepath.Join(root, "internal/rulestore/hash.go"): true,
    }
    allowedConcat := map[string]bool{
        filepath.Join(root, "internal/rulestore/loader.go"): true,
    }
    err = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
        if err != nil || info.IsDir() { return nil }
        if !strings.HasSuffix(p, ".go") { return nil }
        if strings.HasSuffix(p, "_test.go") { return nil }
        data, err := os.ReadFile(p)
        if err != nil { return err }
        s := string(data)
        if strings.Contains(s, `"sha256-"`) && !allowedFilenameConstruction[p] {
            t.Errorf("%s: forbidden raw filename construction `\"sha256-\"` — use hashToFilename", p)
        }
        if strings.Contains(s, `"sha256:" +`) && !allowedConcat[p] {
            t.Errorf("%s: forbidden raw concat `\"sha256:\" +` — canonical hash construction belongs only in rulestore/loader.go", p)
        }
        return nil
    })
    if err != nil { t.Fatal(err) }
}
```
**Step 2 (expect fail):** may fail if pre-existing code uses the pattern — investigate and fix (it shouldn't, since this is a new feature area).
**Step 3 (implement):** Create `rulestore/loader.go` as an empty placeholder file with `package rulestore` so the allowlist path exists. Actual loader lands in 3.5.
**Step 4 (pass):** `cd cli && go test ./internal/rulestore/ -run TestNoRawHashFormatting` — PASS.
**Step 5 (commit):** `git commit -m "test(rulestore): lint forbids raw hash formatting (D11)"`

### Task 3.3: Write a canonical hash from body bytes

**Depends on:** 3.1, Phase 1 canonical
**Files:** `cli/internal/rulestore/hash.go`, `cli/internal/rulestore/hash_test.go`
**Step 1 (test):** Add `TestHashBody` asserting `HashBody([]byte("hello\n"))` returns a string matching `^sha256:[0-9a-f]{64}$` and that two equal inputs produce equal hashes.
**Step 2 (expect fail):** undefined.
**Step 3 (implement):** Add:
```go
import (
    "crypto/sha256"
    "encoding/hex"

    "github.com/OpenScribbler/syllago/cli/internal/converter/canonical"
)

// HashBody returns the canonical "<algo>:<hex>" hash of a rule body.
// The body is normalized (D12) before hashing. Callers should pass the
// body content as authored; normalization is this function's job.
func HashBody(body []byte) string {
    n := canonical.Normalize(body)
    sum := sha256.Sum256(n)
    return "sha256:" + hex.EncodeToString(sum[:])
}
```
**Step 4 (pass):** `cd cli && go test ./internal/rulestore/ -run TestHashBody` — PASS.
**Step 5 (commit):** `git commit -m "feat(rulestore): HashBody with D12 normalization (D11, D12)"`

### Task 3.4: Write a rule directory layout end-to-end

**Depends on:** 3.3, 1.3
**Files:** `cli/internal/rulestore/write.go`, `cli/internal/rulestore/write_test.go`
**Step 1 (test):** Create `write_test.go` with `TestWriteRule_CreatesLayout`. Call `WriteRule(tmp, "claude-code", "coding-style", metadata.RuleMetadata{...}, body)` and assert:
- File `tmp/claude-code/coding-style/rule.md` equals the canonical form of `body`.
- File `tmp/claude-code/coding-style/.syllago.yaml` parses into a `RuleMetadata` with exactly one `versions[]` entry whose hash equals `HashBody(body)` and `current_version` equal to that same hash.
- File `tmp/claude-code/coding-style/.history/sha256-<64hex>.md` equals the canonical form of `body` (byte-equal).
**Step 2 (expect fail):** undefined function.
**Step 3 (implement):** Create `write.go`:
```go
package rulestore

import (
    "os"
    "path/filepath"
    "time"

    "gopkg.in/yaml.v3"

    "github.com/OpenScribbler/syllago/cli/internal/converter/canonical"
    "github.com/OpenScribbler/syllago/cli/internal/metadata"
)

// WriteRule creates a new rule directory under contentRoot/<sourceProvider>/<slug>
// with rule.md + .syllago.yaml + .history/<algo>-<hex>.md per D11. Body is
// normalized once (D12) and that normalized form is what is hashed, written
// to rule.md, and written to .history. meta.Versions and meta.CurrentVersion
// are overwritten by this call to match the produced hash.
func WriteRule(contentRoot, sourceProvider, slug string, meta metadata.RuleMetadata, body []byte) error {
    canon := canonical.Normalize(body)
    hash := HashBody(canon)
    dir := filepath.Join(contentRoot, sourceProvider, slug)
    if err := os.MkdirAll(filepath.Join(dir, ".history"), 0755); err != nil {
        return err
    }
    // rule.md
    if err := os.WriteFile(filepath.Join(dir, "rule.md"), canon, 0644); err != nil {
        return err
    }
    // .history/<algo>-<hex>.md
    if err := os.WriteFile(filepath.Join(dir, ".history", hashToFilename(hash)), canon, 0644); err != nil {
        return err
    }
    // .syllago.yaml
    meta.Versions = []metadata.RuleVersionEntry{{Hash: hash, WrittenAt: time.Now().UTC()}}
    meta.CurrentVersion = hash
    if meta.FormatVersion == 0 {
        meta.FormatVersion = metadata.CurrentFormatVersion
    }
    if meta.Type == "" {
        meta.Type = "rule"
    }
    data, err := yaml.Marshal(&meta)
    if err != nil {
        return err
    }
    return os.WriteFile(filepath.Join(dir, metadata.FileName), data, 0644)
}
```
**Step 4 (pass):** `cd cli && go test ./internal/rulestore/ -run TestWriteRule_CreatesLayout` — PASS.
**Step 5 (commit):** `git commit -m "feat(rulestore): WriteRule with full D11 layout (D11, D12, D13)"`

### Task 3.5: Load a rule directory into an in-memory `Loaded` view

**Depends on:** 3.4
**Files:** `cli/internal/rulestore/loader.go`, `cli/internal/rulestore/load_test.go`
**Step 1 (test):** Create `load_test.go` with `TestLoadRule_RoundTrip` that writes a rule via `WriteRule`, then calls `LoadRule(dir)` and asserts:
- Returned `Loaded.Meta` equals what was written.
- `Loaded.History` is a `map[string][]byte` with one entry whose key equals `meta.CurrentVersion` and value is byte-equal to `canonical.Normalize(body)`.
**Step 2 (expect fail):** undefined.
**Step 3 (implement):** Create `loader.go`:
```go
package rulestore

import (
    "fmt"
    "os"
    "path/filepath"

    "github.com/OpenScribbler/syllago/cli/internal/metadata"
)

// Loaded is the in-memory form of a rule directory (D11).
// History is the direct map[canonical-hash]bytes used by scan (D16).
type Loaded struct {
    Dir     string
    Meta    metadata.RuleMetadata
    History map[string][]byte
}

// LoadRule reads a rule directory at dir, validates the orphan invariant
// (D11), and returns the Loaded view. Bodies in History are byte-equal to
// the on-disk .history/*.md files (no re-normalization on load per D21).
func LoadRule(dir string) (*Loaded, error) {
    meta, err := metadata.LoadRuleMetadata(filepath.Join(dir, metadata.FileName))
    if err != nil {
        return nil, err
    }
    historyDir := filepath.Join(dir, ".history")
    entries, err := os.ReadDir(historyDir)
    if err != nil {
        return nil, fmt.Errorf("reading history dir %s: %w", historyDir, err)
    }
    history := make(map[string][]byte, len(entries))
    seen := make(map[string]bool, len(entries))
    for _, e := range entries {
        if e.IsDir() {
            continue
        }
        hash, ferr := filenameToHash(e.Name())
        if ferr != nil {
            return nil, fmt.Errorf("%s: %w", historyDir, ferr)
        }
        data, rerr := os.ReadFile(filepath.Join(historyDir, e.Name()))
        if rerr != nil {
            return nil, rerr
        }
        history[hash] = data
        seen[hash] = true
    }
    // Orphan-invariant checks (D11):
    // (a) every versions[].hash must have a .history file.
    for _, v := range meta.Versions {
        if !seen[v.Hash] {
            return nil, fmt.Errorf("%s: missing history file for version %s (rebuild from another machine's library or remove the orphan versions[] entry)", dir, v.Hash)
        }
    }
    // (b) every history file must have a versions[] entry.
    versioned := make(map[string]bool, len(meta.Versions))
    for _, v := range meta.Versions {
        versioned[v.Hash] = true
    }
    for hash := range history {
        if !versioned[hash] {
            return nil, fmt.Errorf("%s: orphan history file for %s (add a versions[] entry or delete the file)", dir, hash)
        }
    }
    return &Loaded{Dir: dir, Meta: *meta, History: history}, nil
}
```
**Step 4 (pass):** `cd cli && go test ./internal/rulestore/ -run TestLoadRule_RoundTrip` — PASS.
**Step 5 (commit):** `git commit -m "feat(rulestore): LoadRule with orphan invariant (D11)"`

### Task 3.6: Orphan invariant — missing history file fails load

**Depends on:** 3.5
**Files:** `cli/internal/rulestore/load_test.go`
**Step 1 (test):** Add `TestLoadRule_MissingHistoryFile` that writes a valid rule, deletes the one `.history/*.md` file, and asserts `LoadRule` returns an error containing `"missing history file"`.
**Step 2 (expect fail):** should pass from 3.5.
**Step 3 (implement):** Fix if failing.
**Step 4 (pass):** PASS.
**Step 5 (commit):** `git commit -m "test(rulestore): missing history file → load error (D11)"`

### Task 3.7: Orphan invariant — orphan history file fails load

**Depends on:** 3.5
**Files:** `cli/internal/rulestore/load_test.go`
**Step 1 (test):** Add `TestLoadRule_OrphanHistoryFile` that writes a valid rule, writes an extra `.history/sha256-<other-64-hex>.md` that is not in `versions[]`, and asserts error contains `"orphan history file"`.
**Step 2 (expect fail):** should pass.
**Step 3 (implement):** Fix if failing.
**Step 4 (pass):** PASS.
**Step 5 (commit):** `git commit -m "test(rulestore): orphan history file → load error (D11)"`

### Task 3.8: Add a new canonical version to an existing rule

**Depends on:** 3.4, 3.5
**Files:** `cli/internal/rulestore/write.go`, `cli/internal/rulestore/update_test.go`
**Step 1 (test):** Create `update_test.go` with `TestAppendVersion`. Write a rule with body A, then call `AppendVersion(dir, bodyB)`, then `LoadRule(dir)`. Assert: `Meta.Versions` has length 2, `CurrentVersion` equals `HashBody(bodyB)`, `rule.md` equals `Normalize(bodyB)`, `.history/` contains both files.
**Step 2 (expect fail):** undefined.
**Step 3 (implement):** Append to `write.go`:
```go
// AppendVersion adds a new canonical version of an existing rule (D13). If
// the new body hashes to a value already in versions[], it reuses that entry
// (dedup by hash per D11) and only updates CurrentVersion + rule.md.
func AppendVersion(dir string, newBody []byte) error {
    meta, err := metadata.LoadRuleMetadata(filepath.Join(dir, metadata.FileName))
    if err != nil {
        return err
    }
    canon := canonical.Normalize(newBody)
    hash := HashBody(canon)
    // Write/overwrite rule.md.
    if err := os.WriteFile(filepath.Join(dir, "rule.md"), canon, 0644); err != nil {
        return err
    }
    // Write .history entry if new.
    historyPath := filepath.Join(dir, ".history", hashToFilename(hash))
    if _, err := os.Stat(historyPath); os.IsNotExist(err) {
        if err := os.WriteFile(historyPath, canon, 0644); err != nil {
            return err
        }
    }
    // Append versions[] entry if new.
    have := false
    for _, v := range meta.Versions {
        if v.Hash == hash {
            have = true
            break
        }
    }
    if !have {
        meta.Versions = append(meta.Versions, metadata.RuleVersionEntry{Hash: hash, WrittenAt: time.Now().UTC()})
    }
    meta.CurrentVersion = hash
    data, err := yaml.Marshal(meta)
    if err != nil {
        return err
    }
    return os.WriteFile(filepath.Join(dir, metadata.FileName), data, 0644)
}
```
**Step 4 (pass):** `cd cli && go test ./internal/rulestore/ -run TestAppendVersion` — PASS.
**Step 5 (commit):** `git commit -m "feat(rulestore): AppendVersion with dedup by hash (D11, D13)"`

### Task 3.9: Store the original source file under `.source/`

**Depends on:** 3.4
**Files:** `cli/internal/rulestore/write.go`, `cli/internal/rulestore/source_test.go`
**Step 1 (test):** Create `source_test.go` with `TestWriteRule_WithSource`. Call `WriteRuleWithSource(dir, provider, slug, meta, body, sourceFilename, sourceBytes)` and assert `dir/.source/<sourceFilename>` exists and is byte-equal to `sourceBytes`.
**Step 2 (expect fail):** undefined.
**Step 3 (implement):** Append to `write.go`:
```go
// WriteRuleWithSource is WriteRule plus captures the original source file
// bytes under .source/<filename> per D11.
func WriteRuleWithSource(contentRoot, sourceProvider, slug string, meta metadata.RuleMetadata, body []byte, sourceFilename string, sourceBytes []byte) error {
    if err := WriteRule(contentRoot, sourceProvider, slug, meta, body); err != nil {
        return err
    }
    dir := filepath.Join(contentRoot, sourceProvider, slug)
    if err := os.MkdirAll(filepath.Join(dir, ".source"), 0755); err != nil {
        return err
    }
    return os.WriteFile(filepath.Join(dir, ".source", sourceFilename), sourceBytes, 0644)
}
```
**Step 4 (pass):** `cd cli && go test ./internal/rulestore/ -run TestWriteRule_WithSource` — PASS.
**Step 5 (commit):** `git commit -m "feat(rulestore): capture original source under .source/ (D11)"`

### Task 3.10: Build + fmt gate for Phase 3

**Depends on:** 3.1–3.9
**Files:** —
**Step 1 (test):** `cd cli && make fmt && make build && make test` — PASS.

---

## Phase 4 — Add wizard adaptation

Scope: the TUI Add wizard gains multi-select discovery (D18), a heuristic step (D3), and a review step that renders N candidates from M source files (D18, D4). Per-file skip-split detection labels (D4). Validate-step invariants enforced. Zone marking for mouse parity per `.claude/rules/tui-wizard-patterns.md`. Non-interactive CLI equivalent for batched import lands in Phase 5 (alongside install flags) — Phase 4 is TUI only.

### Task 4.1: Add new wizard step for `addStepHeuristic`

**Depends on:** Phase 3 (rulestore exists), 2.1 (`splitter.Heuristic`)
**Files:** `cli/internal/tui/add_wizard.go`, `cli/internal/tui/wizard_invariant_test.go`
**Step 1 (test):** Add `TestAddWizard_HeuristicStep_InvariantDiscoveryNonEmpty` to `wizard_invariant_test.go` asserting that entering `addStepHeuristic` with no selected candidates panics `wizard invariant: addStepHeuristic entered with no selected candidates`.
**Step 2 (expect fail):** step does not exist.
**Step 3 (implement):** Insert `addStepHeuristic` in the `installStep`-style enum between `addStepDiscovery` and `addStepReview`:
```go
const (
    addStepSource addStep = iota
    addStepType
    addStepDiscovery
    addStepHeuristic
    addStepReview
    addStepExecute
)
```
Extend `validateStep()`:
```go
case addStepHeuristic:
    if len(m.selectedCandidates) == 0 {
        panic("wizard invariant: addStepHeuristic entered with no selected candidates")
    }
```
Add a `selectedCandidates []int` field plus a `chosenHeuristic splitter.Heuristic` and `markerLiteral string` fields on the wizard model (see 4.2 for population).
**Step 4 (pass):** `cd cli && go test ./internal/tui/ -run TestAddWizard_HeuristicStep_InvariantDiscoveryNonEmpty` — PASS.
**Step 5 (commit):** `git commit -m "feat(tui): addStepHeuristic with invariant (D18, D3)"`

### Task 4.2: Discovery step lists monolithic-rule candidates with multi-select

**Depends on:** 4.1, Phase 1 discover + provider tables
**Files:** `cli/internal/tui/add_wizard.go`, `cli/internal/tui/add_wizard_update.go`, `cli/internal/tui/add_wizard_view.go`, `cli/internal/tui/add_wizard_discovery_test.go`
**Step 1 (test):** Create `add_wizard_discovery_test.go` with `TestAddWizard_DiscoveryMultiSelect` that seeds a wizard at `addStepDiscovery` with three mock candidates, sends two spacebar key events at different rows, and asserts both rows are now in `selectedCandidates` and visible as `◉` in the rendered view.
**Step 2 (expect fail):** current wizard has no monolithic-rule discovery.
**Step 3 (implement):** Extend the Source step to include a new radio option "Monolithic rule files (CLAUDE.md, AGENTS.md, ...)". When chosen, the Type step is skipped (type is hard-coded to `rule`) and Discovery jumps directly to invoking `discover.DiscoverMonolithicRules(projectRoot, homeDir, provider.AllMonolithicFilenames())`. Render one row per candidate:
```
  <relative-path>   <L>L  <H>H2  [project|global]  [✓ in library]  [◉|empty]
```
Implement spacebar toggle on the focused row (both keyboard and mouse via `zone.Mark("disc-row-%d", row)` + `updateMouse` handler). Enter advances to `addStepHeuristic` after running `validateStep()`.
**Step 4 (pass):** `cd cli && go test ./internal/tui/ -run TestAddWizard_DiscoveryMultiSelect` — PASS.
**Step 5 (commit):** `git commit -m "feat(tui): monolithic-rule discovery with multi-select (D2, D18)"`

### Task 4.3: "✓ in library" indicator on discovery rows

**Depends on:** 4.2, Phase 3 rulestore
**Files:** `cli/internal/tui/add_wizard_discovery_test.go`, `cli/internal/tui/add_wizard.go`
**Step 1 (test):** Add `TestAddWizard_DiscoveryInLibraryIndicator`. Seed a tmp library with a rule whose `source.hash` equals `sha256-of-file-bytes` for one of the discovery candidates. Render the discovery step and assert that row contains the string `✓ in library`.
**Step 2 (expect fail):** current logic has no indicator.
**Step 3 (implement):** When enumerating discovery candidates, hash each file's bytes (use `rulestore.HashBody` over the raw file bytes) and compare to the set of `source.hash` values across loaded library rules; tag matches. Rendering logic appends ` ✓ in library` to the row.
**Step 4 (pass):** `cd cli && go test ./internal/tui/ -run TestAddWizard_DiscoveryInLibraryIndicator` — PASS.
**Step 5 (commit):** `git commit -m "feat(tui): discovery shows ✓ in library indicator (D18)"`

### Task 4.4: Heuristic step — radio with H2 default + H3 + H4 + marker + single

**Depends on:** 4.1
**Files:** `cli/internal/tui/add_wizard.go`, `cli/internal/tui/add_wizard_view.go`, `cli/internal/tui/add_wizard_heuristic_test.go`
**Step 1 (test):** Create `add_wizard_heuristic_test.go` with `TestAddWizard_HeuristicStepOptions` asserting the rendered step contains radio options: "By H2 (default)", "By H3", "By H4", "By literal marker", "Import as single rule"; default selection is H2.
**Step 2 (expect fail):** step not implemented.
**Step 3 (implement):** Render radio options mapped to `splitter.Heuristic{H2, H3, H4, Marker, Single}`. When marker is selected, show a text input for `MarkerLiteral`. Each option is `zone.Mark("heur-opt-<name>")`. Keyboard: up/down to move, Enter to advance, Esc to go back to Discovery.
**Step 4 (pass):** `cd cli && go test ./internal/tui/ -run TestAddWizard_HeuristicStepOptions` — PASS.
**Step 5 (commit):** `git commit -m "feat(tui): heuristic radio step (D3)"`

### Task 4.5: Review step — grouped-flat candidate list with per-file skip-split label

**Depends on:** 4.4, Phase 2 splitter
**Files:** `cli/internal/tui/add_wizard_review.go`, `cli/internal/tui/add_wizard_review_test.go`
**Step 1 (test):** Create `add_wizard_review_test.go` with `TestAddWizard_ReviewRendersCandidatesPerSource`. Seed wizard with 2 selected sources; one splits into 3 candidates, the other triggers skip-split `too_few_h2`. Render review; assert: the second source's group shows the literal label `"— will import as single rule (too few H2 headings)"`, the first source shows 3 candidate rows.
**Step 2 (expect fail):** step not implemented.
**Step 3 (implement):** In a new file `add_wizard_review.go`, for each selected source: call `splitter.Split(canonical.Normalize(bytes), Options{Heuristic: m.chosenHeuristic, MarkerLiteral: m.markerLiteral})`. On `*SkipSplitSignal`, render a header like `── <path> ─── will import as single rule (<reason>) ──` and one row with the whole-file slug. On success, render `── <path> ── <N> candidates ──` and one row per candidate with its slug + description. Mouse: `zone.Mark("review-cand-%d-%d")` per (source, candidate). Keyboard: up/down, spacebar to toggle include (default include), Enter to advance to `addStepExecute`.
**Step 4 (pass):** `cd cli && go test ./internal/tui/ -run TestAddWizard_ReviewRendersCandidatesPerSource` — PASS.
**Step 5 (commit):** `git commit -m "feat(tui): review step renders per-source groups with skip-split label (D4, D18)"`

### Task 4.6: Review step — per-rule rename override

**Depends on:** 4.5
**Files:** `cli/internal/tui/add_wizard_review.go`, `cli/internal/tui/add_wizard_review_test.go`
**Step 1 (test):** Add `TestAddWizard_ReviewRename` — focus on a candidate row, press `r`, type a new slug, press Enter; assert the candidate's `Name` is updated and the displayed slug reflects it.
**Step 2 (expect fail):** no rename path.
**Step 3 (implement):** Add a one-field rename mini-modal (`add_wizard_rename.go` pattern already exists for content — mirror that). Zone-mark the `[r] Rename` action in the footer.
**Step 4 (pass):** `cd cli && go test ./internal/tui/ -run TestAddWizard_ReviewRename` — PASS.
**Step 5 (commit):** `git commit -m "feat(tui): rename override in review step (D4)"`

### Task 4.7: Execute step — write candidates to library via `rulestore.WriteRuleWithSource`

**Depends on:** 4.5, Phase 3 rulestore
**Files:** `cli/internal/tui/add_wizard_execute.go`, `cli/internal/tui/add_wizard_execute_test.go`
**Step 1 (test):** Create `add_wizard_execute_test.go` with `TestAddWizard_ExecuteWritesRules`. Seed wizard at `addStepExecute` with 3 accepted candidates from 2 sources; run the execute command; assert 3 rule directories exist under the tmp content root with the expected slugs, each containing `rule.md`, `.syllago.yaml`, `.history/sha256-<hex>.md`, and `.source/<filename>` present on rules whose source file matches.
**Step 2 (expect fail):** no execute path.
**Step 3 (implement):** A `tea.Cmd` that loops over accepted candidates and calls `rulestore.WriteRuleWithSource(contentRoot, sourceProviderSlug, cand.Name, buildRuleMetadata(cand, source), []byte(cand.Body), source.Filename, source.Bytes)`. `buildRuleMetadata` populates `RuleSource` with `Provider`, `Scope`, `Path`, `Format` (`"claude-code"` / etc., matching the filename's provider), `Filename`, `Hash = HashBody(source.Bytes)`, `SplitMethod` (string form of heuristic), `SplitFromSection = cand.Description`, `CapturedAt = time.Now()`. Sends `addCompletedMsg` with the count on completion and a `toastMsg` on any failure.
**Step 4 (pass):** `cd cli && go test ./internal/tui/ -run TestAddWizard_ExecuteWritesRules` — PASS.
**Step 5 (commit):** `git commit -m "feat(tui): execute step writes rules via rulestore (D11, D13, D18)"`

### Task 4.8: Mouse-parity sweep across new wizard steps

**Depends on:** 4.2, 4.4, 4.5
**Files:** `cli/internal/tui/add_wizard_mouse_test.go`
**Step 1 (test):** Add three subtests (sequential, no `t.Parallel`) for click-to-toggle on a Discovery row, click-to-select on a Heuristic radio option, click-to-rename on a Review row. Each asserts the same state change as the keyboard path.
**Step 2 (expect fail):** if any interactive element lacks a zone mark, the click is a no-op.
**Step 3 (implement):** Add missing `zone.Mark` wrappers in the View functions and `zone.Get(id).InBounds(msg)` checks in `updateMouse`.
**Step 4 (pass):** `cd cli && go test ./internal/tui/ -run TestAddWizard_Mouse` — PASS.
**Step 5 (commit):** `git commit -m "test(tui): mouse parity for new wizard steps (tui-wizard-patterns)"`

### Task 4.9: Golden files for the three new steps at 60x20, 80x30, 120x40

**Depends on:** 4.2, 4.4, 4.5
**Files:** `cli/internal/tui/testdata/add-wizard-discovery-*.golden`, `add-wizard-heuristic-*.golden`, `add-wizard-review-*.golden`
**Step 1 (test):** Add `TestGoldenAddWizard_DiscoverySizes`, `TestGoldenAddWizard_HeuristicSizes`, `TestGoldenAddWizard_ReviewSizes` using `requireGolden` at three sizes each.
**Step 2 (expect fail):** no golden files yet.
**Step 3 (implement):** `cd cli && go test ./internal/tui/ -update-golden -run "TestGoldenAddWizard_(Discovery|Heuristic|Review)Sizes"` to generate, then `git diff internal/tui/testdata/` to review each.
**Step 4 (pass):** `cd cli && go test ./internal/tui/ -run "TestGoldenAddWizard_(Discovery|Heuristic|Review)Sizes"` — PASS.
**Step 5 (commit):** `git commit -m "test(tui): golden files for new add-wizard steps"`

### Task 4.10: Refresh content after add completion uses `a.refreshContent()`

**Depends on:** 4.7
**Files:** `cli/internal/tui/app_update.go`
**Step 1 (test):** Add `TestApp_HandlesAddCompletedMsgViaRefreshContent` in `app_test.go` — send an `addCompletedMsg` and assert `a.library.Items` reflects the new catalog by calling `refreshContent` (mock `catalog` or use a temp library directory and rescan).
**Step 2 (expect fail):** handler missing.
**Step 3 (implement):** In `app_update.go`, route `addCompletedMsg` through `a.rescanCatalog()` followed by `a.refreshContent()` per `.claude/rules/tui-items-rebuild.md`.
**Step 4 (pass):** PASS.
**Step 5 (commit):** `git commit -m "feat(tui): refresh content after add completion (tui-items-rebuild)"`

### Task 4.11: Telemetry enrichment for add wizard

**Depends on:** 4.7
**Files:** `cli/cmd/syllago/add_cmd.go`, `cli/internal/telemetry/catalog.go`
**Step 1 (test):** In `cli/internal/telemetry/catalog_drift_test.go` (the existing `TestGentelemetry_CatalogMatchesEnrichCalls` test) — no new test, but note it will fail until the catalog entries are added.
**Step 2 (expect fail):** run existing drift test — fails on the new keys.
**Step 3 (implement):** Inside `runAdd` (for the batched-from path added in Phase 5) and via a shared telemetry helper used by the TUI wizard too, call `telemetry.Enrich("discovery_candidate_count", n)` and `telemetry.Enrich("selected_count", k)` and `telemetry.Enrich("split_method", method)` and `telemetry.Enrich("scope", scopeTag)`. Add `PropertyDef` entries to `EventCatalog()` for each of these four keys under the `command_executed` event, with `Commands: []string{"add"}`. Run `cd cli && make gendocs` to regenerate `telemetry.json`.
**Step 4 (pass):** `cd cli && go test ./internal/telemetry/ -run TestGentelemetry_CatalogMatchesEnrichCalls` — PASS.
**Step 5 (commit):** `git commit -m "feat(telemetry): add discovery_candidate_count + selected_count + split_method + scope (D18)"`

### Task 4.12: Build + fmt + test gate for Phase 4

**Depends on:** 4.1–4.11
**Files:** —
**Step 1:** `cd cli && make fmt && make build && make test && go test ./internal/tui/ -update-golden && git diff internal/tui/testdata/` — review + commit any golden diffs that reflect real visual changes.

---

## Phase 5 — Append-to-monolithic install method

Scope: D5 append-to-end, D6 no in-file ownership, D20 byte contract for append, D14 `InstalledRuleAppend` record type + helpers, D10 per-provider hints surfaced as `NOTE:` stderr for CLI and non-blocking notes for TUI. Also introduces the CLI non-interactive batched-import form (`--from <path>` repeatable + `--split=...`) so Phase 4's flow has a callable non-TUI equivalent.

**Explicitly out of this phase's scope per D15:** scope-aware install record storage redesign (global vs project file split across all four record types). V1 inherits the existing single-`installed.json`-per-project model; D15's redesign is a separate follow-up ADR and does not block this phase or V1 ship. Users who install from project A and uninstall from project B will hit the limitation documented in D15 "Behavior in V1."

### Task 5.1: Define `InstalledRuleAppend` and extend `Installed`

**Depends on:** Phase 3
**Files:** `cli/internal/installer/installed.go`, `cli/internal/installer/installed_test.go`
**Step 1 (test):** Add `TestInstalled_RuleAppendsJSONRoundtrip` — marshal an `Installed` with one `InstalledRuleAppend`, unmarshal, assert equality.
**Step 2 (expect fail):** type undefined.
**Step 3 (implement):** Append to `installed.go`:
```go
// InstalledRuleAppend records a rule appended to a provider's monolithic file (D14).
type InstalledRuleAppend struct {
    Name        string    `json:"name"`
    LibraryID   string    `json:"libraryId"`
    Provider    string    `json:"provider"`
    TargetFile  string    `json:"targetFile"`
    VersionHash string    `json:"versionHash"` // canonical "<algo>:<hex>" per D11
    Source      string    `json:"source"`
    Scope       string    `json:"scope,omitempty"`
    InstalledAt time.Time `json:"installedAt"`
}

// (extend Installed with RuleAppends)
```
Modify the `Installed` struct:
```go
type Installed struct {
    Hooks       []InstalledHook       `json:"hooks,omitempty"`
    MCP         []InstalledMCP        `json:"mcp,omitempty"`
    Symlinks    []InstalledSymlink    `json:"symlinks,omitempty"`
    RuleAppends []InstalledRuleAppend `json:"ruleAppends,omitempty"`
}
```
**Step 4 (pass):** `cd cli && go test ./internal/installer/ -run TestInstalled_RuleAppendsJSONRoundtrip` — PASS.
**Step 5 (commit):** `git commit -m "feat(installer): InstalledRuleAppend record type (D14)"`

### Task 5.2: `FindRuleAppend` / `RemoveRuleAppend` helpers

**Depends on:** 5.1
**Files:** `cli/internal/installer/installed.go`, `cli/internal/installer/installed_test.go`
**Step 1 (test):** Add `TestInstalled_FindAndRemoveRuleAppend` — three entries, `FindRuleAppend(libID, target)` returns the right index or -1; `RemoveRuleAppend(idx)` splices.
**Step 2 (expect fail):** helpers undefined.
**Step 3 (implement):** Append:
```go
// FindRuleAppend returns the index of a rule append entry matching libraryID
// and targetFile, or -1. The (LibraryID, TargetFile) pair is unique per D14.
func (inst *Installed) FindRuleAppend(libraryID, targetFile string) int {
    for i, r := range inst.RuleAppends {
        if r.LibraryID == libraryID && r.TargetFile == targetFile {
            return i
        }
    }
    return -1
}

// RemoveRuleAppend removes the rule append entry at idx.
func (inst *Installed) RemoveRuleAppend(idx int) {
    inst.RuleAppends = append(inst.RuleAppends[:idx], inst.RuleAppends[idx+1:]...)
}
```
**Step 4 (pass):** PASS.
**Step 5 (commit):** `git commit -m "feat(installer): FindRuleAppend/RemoveRuleAppend helpers (D14)"`

### Task 5.3: Uniqueness invariant test

**Depends on:** 5.2
**Files:** `cli/internal/installer/installed_test.go`
**Step 1 (test):** Add `TestInstalled_RuleAppendsUniqueByLibraryIDAndTargetFile` — iterate over `inst.RuleAppends`, assert no two entries share a `(LibraryID, TargetFile)` pair (sanity when writer paths respect the invariant).
**Step 2 (expect fail):** Not a logic failure — this is a guard for future writers. Test passes on empty or unique slices.
**Step 3 (implement):** Test only — no code under test here.
**Step 4 (pass):** PASS.
**Step 5 (commit):** `git commit -m "test(installer): uniqueness invariant sanity for RuleAppends (D14)"`

### Task 5.4: `scope.ResolveAppendScope(targetFile, homeDir, projectRoot)` helper

**Depends on:** 5.1
**Files:** `cli/internal/installer/scope.go`, `cli/internal/installer/scope_test.go`
**Step 1 (test):** Add `TestResolveAppendScope` covering the three cases from D14: under homeDir → `"global"`, under projectRoot → `"project"`, neither → `"global"`.
**Step 2 (expect fail):** undefined.
**Step 3 (implement):** Add to `scope.go`:
```go
// ResolveAppendScope classifies a rule-append TargetFile per D14:
// "global" when under homeDir, "project" when under projectRoot,
// "global" otherwise (absolute paths outside both).
func ResolveAppendScope(targetFile, homeDir, projectRoot string) string {
    if homeDir != "" && isUnder(targetFile, homeDir) {
        return "global"
    }
    if projectRoot != "" && isUnder(targetFile, projectRoot) {
        return "project"
    }
    return "global"
}

func isUnder(path, root string) bool {
    rel, err := filepath.Rel(root, path)
    if err != nil { return false }
    return !strings.HasPrefix(rel, "..") && rel != ".."
}
```
**Step 4 (pass):** PASS.
**Step 5 (commit):** `git commit -m "feat(installer): ResolveAppendScope classifier (D14)"`

### Task 5.5: Append byte contract — empty file create

**Depends on:** 5.1, Phase 1 canonical
**Files:** `cli/internal/installer/rule_append.go`, `cli/internal/installer/rule_append_test.go`
**Step 1 (test):** Create `rule_append_test.go` with `TestAppendToTarget_EmptyFileCreates`. Call `AppendRuleToTarget(tmp/CLAUDE.md, canonicalBody)` against a non-existent file and assert the file exists and is byte-equal to `"\n" + canonicalBody` per D20.
**Step 2 (expect fail):** undefined.
**Step 3 (implement):** Create `rule_append.go`:
```go
// Package installer's rule-append path implements D20's byte contract.
package installer

import (
    "errors"
    "io/fs"
    "os"

    "github.com/OpenScribbler/syllago/cli/internal/converter/canonical"
)

// AppendRuleToTarget appends canonicalBody to targetFile per D20:
//   1. If target does not exist or is empty, write "\n<canonicalBody>".
//   2. Otherwise ensure target ends with "\n", then append "\n<canonicalBody>".
// canonicalBody must already be in D12 canonical form (single trailing \n).
func AppendRuleToTarget(targetFile string, canonicalBody []byte) error {
    existing, err := os.ReadFile(targetFile)
    if errors.Is(err, fs.ErrNotExist) {
        existing = nil
    } else if err != nil {
        return err
    }
    // Ensure canonicalBody is normalized (defensive; caller should already do this).
    cb := canonical.Normalize(canonicalBody)
    var out []byte
    if len(existing) == 0 {
        out = append([]byte{'\n'}, cb...)
    } else {
        if existing[len(existing)-1] != '\n' {
            existing = append(existing, '\n')
        }
        out = append(existing, '\n')
        out = append(out, cb...)
    }
    return os.WriteFile(targetFile, out, 0644)
}
```
**Step 4 (pass):** `cd cli && go test ./internal/installer/ -run TestAppendToTarget_EmptyFileCreates` — PASS.
**Step 5 (commit):** `git commit -m "feat(installer): AppendRuleToTarget empty-file create (D20)"`

### Task 5.6: Append byte contract — non-empty file, ends with `\n`

**Depends on:** 5.5
**Files:** `cli/internal/installer/rule_append_test.go`
**Step 1 (test):** Add `TestAppendToTarget_NonEmptyEndsWithNewline`. Seed file with `"P\n"`, call `AppendRuleToTarget(target, []byte("rule body\n"))`, assert resulting file bytes are `"P\n\nrule body\n"`.
**Step 2 (expect fail):** run — should PASS from 5.5.
**Step 3 (implement):** Fix if failing.
**Step 4 (pass):** PASS.
**Step 5 (commit):** `git commit -m "test(installer): append non-empty ends-with-\\n case (D20)"`

### Task 5.7: Append byte contract — non-empty file, missing final `\n`

**Depends on:** 5.5
**Files:** `cli/internal/installer/rule_append_test.go`
**Step 1 (test):** Add `TestAppendToTarget_NonEmptyMissingNewline`. Seed file with `"P"` (no final newline), call `AppendRuleToTarget`, assert bytes are `"P\n\nrule body\n"` (the missing `\n` was added, then the leading `\n` from the pattern).
**Step 2–4 (TDD):** iterate as needed.
**Step 5 (commit):** `git commit -m "test(installer): append repairs missing trailing newline (D20)"`

### Task 5.8: Append byte contract — mid-file three-rule sequence matches D20's worked example

**Depends on:** 5.5
**Files:** `cli/internal/installer/rule_append_test.go`
**Step 1 (test):** Add `TestAppendToTarget_SequentialThreeRules` matching D20's worked example. Seed file `"P\n"`; append bodies `"r1\n"`, `"r2\n"`, `"r3\n"` in order; assert final contents equal `"P\n\nr1\n\nr2\n\nr3\n"`.
**Step 2–4:** iterate.
**Step 5 (commit):** `git commit -m "test(installer): D20 worked example — three-rule sequential append"`

### Task 5.9: `InstallRuleAppend` high-level entry point

**Depends on:** 5.2, 5.4, 5.5, Phase 3 rulestore
**Files:** `cli/internal/installer/rule_append.go`, `cli/internal/installer/rule_append_install_test.go`
**Step 1 (test):** Create `rule_append_install_test.go` with `TestInstallRuleAppend_RecordsAndAppends`. Set up a tmp projectRoot with a syllago library rule, call `InstallRuleAppend(ctx)` where `ctx` carries (library rule `Loaded`, providerSlug, targetFile, source="manual"). Assert:
- targetFile is appended per D20.
- `installed.json` has one `InstalledRuleAppend` with correct fields, including `VersionHash == rule.Meta.CurrentVersion` and `Scope` resolved per 5.4.
**Step 2–4 (TDD):** implement:
```go
// InstallRuleAppend performs a monolithic-file append install. The library
// rule's current_version bytes are the canonical body (already normalized
// per D11's "files on disk = canonical"). D14 uniqueness: callers must
// route repeats through the update flow (D17); this function assumes
// Fresh state.
func InstallRuleAppend(projectRoot, homeDir, providerSlug, targetFile, source string, rule *rulestore.Loaded) error {
    canonBody := rule.History[rule.Meta.CurrentVersion]
    if canonBody == nil {
        return fmt.Errorf("install: no history entry for current_version %s", rule.Meta.CurrentVersion)
    }
    if err := AppendRuleToTarget(targetFile, canonBody); err != nil {
        return err
    }
    inst, err := LoadInstalled(projectRoot)
    if err != nil { return err }
    inst.RuleAppends = append(inst.RuleAppends, InstalledRuleAppend{
        Name:        rule.Meta.Name,
        LibraryID:   rule.Meta.ID,
        Provider:    providerSlug,
        TargetFile:  targetFile,
        VersionHash: rule.Meta.CurrentVersion,
        Source:      source,
        Scope:       ResolveAppendScope(targetFile, homeDir, projectRoot),
        InstalledAt: time.Now().UTC(),
    })
    return SaveInstalled(projectRoot, inst)
}
```
**Step 5 (commit):** `git commit -m "feat(installer): InstallRuleAppend entry point (D5, D14, D20)"`

### Task 5.10: CLI — add `--from <path>` repeatable + `--split` flag to `syllago add`

**Depends on:** 5.9, Phase 2 splitter, Phase 3 rulestore
**Files:** `cli/cmd/syllago/add_cmd.go`, `cli/cmd/syllago/add_cmd_monolithic_test.go`
**Step 1 (test):** Create `add_cmd_monolithic_test.go` with:
- `TestAdd_FromMonolithicFile_H2` — seeds tmp with a fixture file, runs `syllago add --from <path> --split=h2`, asserts expected rules appear in the library.
- `TestAdd_FromMonolithicFile_SkipSplitErrorsWithoutSingleFlag` — uses `too-small.md`, asserts error mentions `--split=single`.
- `TestAdd_FromMonolithicFile_Single` — same fixture plus `--split=single`, asserts one rule written.
- `TestAdd_LLMSplitWithoutSkill_ErrorsWithInstallPointer` — `--split=llm` without the `split-rules-llm` skill installed, asserts error message contains `syllago add split-rules-llm`.
**Step 2 (expect fail):** no `--from <path>` semantics (the existing `--from` is a provider slug).
**Step 3 (implement):** In `add_cmd.go`, treat `--from` as follows: if the value exists as a file, it is a monolithic source path; otherwise it is a provider slug (existing behavior). Change the registration:
```go
addCmd.Flags().StringArray("from", nil, "Provider to add from, or path to a monolithic rule file (repeatable for files)")
addCmd.Flags().String("split", "", "Splitter heuristic: h2|h3|h4|marker:<literal>|single|llm (monolithic --from only)")
```
Adapt `runAdd` to detect the two modes. For monolithic mode, iterate inputs: parse `--split`, call `splitter.Split` over each file's canonical bytes, on `*SkipSplitSignal` without `--split=single` error with the list of affected files and the suggestion. On `--split=llm`, call a new `splitterllm.Available()` helper (Phase 10 provides the real one; here stub it to return an error `"install split-rules-llm from syllago-meta-registry: syllago add split-rules-llm"`). On success, for each candidate call `rulestore.WriteRuleWithSource(...)` using the inferred source provider slug from the filename (`CLAUDE.md → claude-code`, etc. — reuse `provider.MonolithicFilenames` inverse map).
**Step 4 (pass):** `cd cli && go test ./cmd/syllago/ -run TestAdd_FromMonolithicFile` — PASS.
**Step 5 (commit):** `git commit -m "feat(cli): add --from <path> + --split for monolithic sources (D3, D4, D9, D18)"`

### Task 5.11: CLI — install path picks append method via `--method`

**Depends on:** 5.9
**Files:** `cli/cmd/syllago/install_cmd.go`, `cli/cmd/syllago/install_cmd_append_test.go`
**Step 1 (test):** Create `install_cmd_append_test.go` with `TestInstall_MethodAppend_WritesMonolithicFile` — seed a library rule, seed a tmp project, run `syllago install rules/foo --to claude-code --method=append`, assert: `projectRoot/CLAUDE.md` exists and contains the rule body; `installed.json` has a corresponding `ruleAppends[]` entry.
**Step 2 (expect fail):** `--method` flag does not exist.
**Step 3 (implement):** Register `--method string` ("file" | "append"; default "file"). On `append`, use the provider's first monolithic filename via `provider.MonolithicFilenames(slug)` and call `installer.InstallRuleAppend(...)`. Print the hint via `provider.MonolithicHint(slug)` to `output.ErrWriter` as `NOTE: <hint>` per D10 (suppress when `--quiet`).
**Step 4 (pass):** PASS.
**Step 5 (commit):** `git commit -m "feat(cli): install --method=append for monolithic rules (D5, D10, D14)"`

### Task 5.12: NOTE stderr suppressed by `--quiet` per D10

**Depends on:** 5.11
**Files:** `cli/cmd/syllago/install_cmd_append_test.go`
**Step 1 (test):** Add `TestInstall_MethodAppend_QuietSuppressesNote` — run with `--method=append --quiet` against windsurf; assert stderr does NOT contain `"NOTE:"`.
**Step 2 (expect fail):** whether `--quiet` already exists globally; add it if not (likely already exists as a root flag — verify). If it exists, wire the hint print through it.
**Step 3 (implement):** Guard the print with `if !output.Quiet`.
**Step 4 (pass):** PASS.
**Step 5 (commit):** `git commit -m "feat(cli): --quiet suppresses monolithic install NOTE (D10)"`

### Task 5.13: Build + fmt + gendocs + test gate for Phase 5

**Depends on:** 5.1–5.12
**Files:** —
**Step 1:** `cd cli && make fmt && make build && make gendocs && make test` — PASS. Commit any `telemetry.json` / `commands.json` changes separately.

---

## Phase 6 — Exact-match uninstall + verification scan

Scope: D16 verification scan (mtime cache, `(State, Reason)` per (LibraryID, TargetFile), `match_set` projection, `warnings[]`) using D20's leading-`\n` byte pattern; D7 explicit uninstall semantics; D20 search-and-remove with zero/multiple-match fall-through to D17 Modified path (Phase 7 handles the decision surface; Phase 6 only exposes the state). **Ship gate: D21's 10-cell roundtrip matrix at `cli/internal/installer/roundtrip_test.go` must pass before this phase is considered done.**

### Task 6.1: Scaffold the `installcheck` package + `VerificationState` types

**Depends on:** Phase 3, Phase 5 (`InstalledRuleAppend`)
**Files:** `cli/internal/installcheck/state.go`, `cli/internal/installcheck/state_test.go`
**Step 1 (test):** Trivial zero-value test.
**Step 2 (expect fail):** undefined.
**Step 3 (implement):** Create `state.go`:
```go
// Package installcheck implements D16's rule-append verification scan:
// cross-references installed.json.RuleAppends against actual bytes on disk
// to produce (State, Reason) tuples consumed by D17 and the TUI.
package installcheck

// State is the per-record verification state (D16).
type State int

const (
    StateFresh State = iota // no RuleAppend record exists — install proceeds
    StateClean              // record exists AND recorded VersionHash bytes are in TargetFile
    StateModified           // record exists BUT recorded bytes are not found
)

// Reason carries the divergence type when State is Modified (D16).
type Reason int

const (
    ReasonNone       Reason = iota
    ReasonEdited            // file present, bytes don't match
    ReasonMissing           // ENOENT
    ReasonUnreadable        // EACCES/EIO/other I/O error or bad read
)

// PerTargetState is the value stored per (LibraryID, TargetFile) pair.
type PerTargetState struct {
    State  State
    Reason Reason
}

// VerificationResult is the output of a single scan (D16 "Input -> output contract").
type VerificationResult struct {
    PerRecord map[RecordKey]PerTargetState   // one tuple per RuleAppend record
    MatchSet  map[string][]string            // LibraryID -> TargetFiles that are Clean (column projection)
    Warnings  []string                       // surfaced to stderr (CLI) / toast (TUI)
}

// RecordKey is the composite key (LibraryID, TargetFile).
type RecordKey struct {
    LibraryID  string
    TargetFile string
}
```
**Step 4 (pass):** PASS.
**Step 5 (commit):** `git commit -m "feat(installcheck): state + result types (D16)"`

### Task 6.2: `Scan` happy path — Fresh (no record) and Clean (record + bytes match)

**Depends on:** 6.1, Phase 1 canonical
**Files:** `cli/internal/installcheck/scan.go`, `cli/internal/installcheck/scan_test.go`
**Step 1 (test):** Add `TestScan_Clean`. Seed a library rule with one history entry, install it into a target file via `AppendRuleToTarget`, add the matching record to `Installed.RuleAppends`, then call `Scan(inst, library)`. Assert:
- `result.PerRecord[{libID, target}] == {StateClean, ReasonNone}`.
- `result.MatchSet[libID]` contains `target`.
**Step 2 (expect fail):** undefined.
**Step 3 (implement):** Create `scan.go`:
```go
package installcheck

import (
    "bytes"
    "errors"
    "fmt"
    "io/fs"
    "os"

    "github.com/OpenScribbler/syllago/cli/internal/converter/canonical"
    "github.com/OpenScribbler/syllago/cli/internal/installer"
    "github.com/OpenScribbler/syllago/cli/internal/rulestore"
)

// Scan runs D16's verification algorithm against the provided install
// record and library. Library indexes rules by LibraryID (the .syllago.yaml
// id) and provides history as map[hash][]byte (canonical bytes per D11/D12).
func Scan(inst *installer.Installed, library map[string]*rulestore.Loaded) *VerificationResult {
    out := &VerificationResult{
        PerRecord: map[RecordKey]PerTargetState{},
        MatchSet:  map[string][]string{},
    }
    // Group records by unique TargetFile (D16 pseudocode).
    byTarget := map[string][]installer.InstalledRuleAppend{}
    for _, r := range inst.RuleAppends {
        byTarget[r.TargetFile] = append(byTarget[r.TargetFile], r)
    }
    for targetFile, records := range byTarget {
        stat, statErr := os.Stat(targetFile)
        if errors.Is(statErr, fs.ErrNotExist) {
            for _, r := range records {
                out.PerRecord[RecordKey{r.LibraryID, targetFile}] = PerTargetState{StateModified, ReasonMissing}
            }
            continue
        }
        if statErr != nil {
            for _, r := range records {
                out.PerRecord[RecordKey{r.LibraryID, targetFile}] = PerTargetState{StateModified, ReasonUnreadable}
            }
            out.Warnings = append(out.Warnings, fmt.Sprintf("verify %s: %s", targetFile, statErr))
            continue
        }
        _ = stat
        raw, readErr := os.ReadFile(targetFile)
        if readErr != nil {
            for _, r := range records {
                out.PerRecord[RecordKey{r.LibraryID, targetFile}] = PerTargetState{StateModified, ReasonUnreadable}
            }
            out.Warnings = append(out.Warnings, fmt.Sprintf("verify %s: %s", targetFile, readErr))
            continue
        }
        normalizedTarget := canonical.Normalize(raw)
        for _, r := range records {
            rule, ok := library[r.LibraryID]
            if !ok {
                out.PerRecord[RecordKey{r.LibraryID, targetFile}] = PerTargetState{StateModified, ReasonEdited}
                out.Warnings = append(out.Warnings, fmt.Sprintf("verify %s: no library rule for %s", targetFile, r.LibraryID))
                continue
            }
            body := rule.History[r.VersionHash]
            if body == nil {
                // Defensive orphan record (D11's load-time invariant should prevent this).
                out.PerRecord[RecordKey{r.LibraryID, targetFile}] = PerTargetState{StateModified, ReasonEdited}
                out.Warnings = append(out.Warnings, fmt.Sprintf("verify %s: orphan record for %s (no history file for %s)", targetFile, r.LibraryID, r.VersionHash))
                continue
            }
            pattern := append([]byte{'\n'}, canonical.Normalize(body)...)
            if bytes.Contains(normalizedTarget, pattern) {
                out.PerRecord[RecordKey{r.LibraryID, targetFile}] = PerTargetState{StateClean, ReasonNone}
                out.MatchSet[r.LibraryID] = append(out.MatchSet[r.LibraryID], targetFile)
            } else {
                out.PerRecord[RecordKey{r.LibraryID, targetFile}] = PerTargetState{StateModified, ReasonEdited}
            }
        }
    }
    return out
}
```
**Step 4 (pass):** `cd cli && go test ./internal/installcheck/ -run TestScan_Clean` — PASS.
**Step 5 (commit):** `git commit -m "feat(installcheck): Scan happy-path Clean state (D16, D20)"`

### Task 6.3: Scan — Modified/edited (record exists, bytes diverge)

**Depends on:** 6.2
**Files:** `cli/internal/installcheck/scan_test.go`
**Step 1 (test):** Add `TestScan_ModifiedEdited` — install, then mutate the target file to remove one character from the appended block. Assert `{StateModified, ReasonEdited}` and `MatchSet` is empty for that libID.
**Step 2–4 (TDD):** should already pass from 6.2.
**Step 5 (commit):** `git commit -m "test(installcheck): Modified/edited state (D16)"`

### Task 6.4: Scan — Modified/missing (ENOENT)

**Depends on:** 6.2
**Files:** `cli/internal/installcheck/scan_test.go`
**Step 1 (test):** Add `TestScan_ModifiedMissing` — install, then `os.Remove(target)`. Assert `{StateModified, ReasonMissing}`, no warning (ENOENT is expected per D16).
**Step 2–4:** iterate.
**Step 5 (commit):** `git commit -m "test(installcheck): Modified/missing ENOENT (D16)"`

### Task 6.5: Scan — Modified/unreadable (permission denied)

**Depends on:** 6.2
**Files:** `cli/internal/installcheck/scan_test.go`
**Step 1 (test):** Add `TestScan_ModifiedUnreadable` — install, chmod target to `0000`. Assert `{StateModified, ReasonUnreadable}` and a warning containing the target path. Use `t.Cleanup` to restore permissions so the temp dir can be cleaned.
**Step 2–4:** iterate.
**Step 5 (commit):** `git commit -m "test(installcheck): Modified/unreadable I/O error (D16)"`

### Task 6.6: mtime cache — repeat scans hit the cache

**Depends on:** 6.2
**Files:** `cli/internal/installcheck/cache.go`, `cli/internal/installcheck/cache_test.go`
**Step 1 (test):** Create `cache_test.go` with `TestScan_MtimeCacheSkipsRead` — install + scan once, install a `counting fs` hook that counts `ReadFile` calls (inject via a package-private `readFile` func var), scan again with the same mtime+size, assert the second call does NOT increment the read count.
**Step 2 (expect fail):** no cache yet.
**Step 3 (implement):** In `cache.go`:
```go
package installcheck

import "sync"

type mtimeCacheEntry struct {
    mtime int64
    size  int64
    state map[string]PerTargetState // libID -> state
}

// cache is process-local per D16 "no on-disk cache file".
var (
    cacheMu sync.Mutex
    cache   = map[string]mtimeCacheEntry{}
)

// InvalidateCache is called from install/uninstall paths that touch target.
func InvalidateCache(target string) {
    cacheMu.Lock()
    defer cacheMu.Unlock()
    delete(cache, target)
}
```
Thread cache use into `scan.go`: before reading, check `cache[targetFile]` against the `stat` mtime+size; on hit, reuse the cached `per_target_state`. On read, write through to cache.
**Step 4 (pass):** `cd cli && go test ./internal/installcheck/ -run TestScan_MtimeCacheSkipsRead` — PASS.
**Step 5 (commit):** `git commit -m "feat(installcheck): mtime cache skips repeat reads (D16)"`

### Task 6.7: `Uninstall` high-level entry point — exact match success

**Depends on:** 5.9, 6.2
**Files:** `cli/internal/installer/rule_append.go`, `cli/internal/installer/rule_append_uninstall_test.go`
**Step 1 (test):** Create `rule_append_uninstall_test.go` with `TestUninstallRuleAppend_ExactMatch`. Install via `InstallRuleAppend`, then call `UninstallRuleAppend(projectRoot, libID, targetFile, library)`. Assert:
- Target file bytes are byte-equal to the pre-install snapshot.
- `installed.json` no longer contains the matching record.
**Step 2–4 (TDD):** implement in `rule_append.go`:
```go
// UninstallRuleAppend searches targetFile for any historical version's bytes
// (canonical "\n<body>" pattern per D20) and removes the matched range if
// found exactly once. Zero or multiple matches return an error that callers
// should surface as D17 Modified state — actual decision routing lives in
// the Update flow (Phase 7).
func UninstallRuleAppend(projectRoot, libraryID, targetFile string, library map[string]*rulestore.Loaded) error {
    inst, err := LoadInstalled(projectRoot)
    if err != nil { return err }
    idx := inst.FindRuleAppend(libraryID, targetFile)
    if idx < 0 {
        return fmt.Errorf("no rule-append record for %s at %s", libraryID, targetFile)
    }
    record := inst.RuleAppends[idx]
    // Read target. ENOENT uninstall semantics (D7): silent success + drop record.
    raw, err := os.ReadFile(targetFile)
    if errors.Is(err, fs.ErrNotExist) {
        inst.RemoveRuleAppend(idx)
        return SaveInstalled(projectRoot, inst)
    }
    if err != nil {
        // D7: unreadable target errors out and leaves record intact.
        return fmt.Errorf("reading %s: %w", targetFile, err)
    }
    rule := library[libraryID]
    if rule == nil {
        return fmt.Errorf("no library rule for %s", libraryID)
    }
    normalized := canonical.Normalize(raw)
    // Try the full history in reverse order (newest first) — D7 full-history search.
    for i := len(rule.Meta.Versions) - 1; i >= 0; i-- {
        body := rule.History[rule.Meta.Versions[i].Hash]
        pattern := append([]byte{'\n'}, canonical.Normalize(body)...)
        count := bytes.Count(normalized, pattern)
        if count == 1 {
            // Splice out.
            cut := bytes.Index(normalized, pattern)
            out := append([]byte{}, normalized[:cut]...)
            out = append(out, normalized[cut+len(pattern):]...)
            if err := os.WriteFile(targetFile, out, 0644); err != nil {
                return err
            }
            installcheck.InvalidateCache(targetFile)
            inst.RemoveRuleAppend(idx)
            return SaveInstalled(projectRoot, inst)
        }
        if count > 1 {
            return fmt.Errorf("multiple matches for rule %s in %s — resolve via update flow", libraryID, targetFile)
        }
    }
    return fmt.Errorf("rule %s not found in %s — file may have been edited", libraryID, targetFile)
    _ = record
}
```
**Step 5 (commit):** `git commit -m "feat(installer): UninstallRuleAppend with full-history search (D7, D20)"`

### Task 6.8: Uninstall — missing target file silent-succeeds + drops record

**Depends on:** 6.7
**Files:** `cli/internal/installer/rule_append_uninstall_test.go`
**Step 1 (test):** `TestUninstallRuleAppend_MissingTargetFileSucceeds` — install, delete target, call Uninstall, assert no error, record is removed.
**Step 2–4:** should PASS from 6.7.
**Step 5 (commit):** `git commit -m "test(installer): uninstall ENOENT drops record (D7)"`

### Task 6.9: Uninstall — unreadable target errors + preserves record

**Depends on:** 6.7
**Files:** `cli/internal/installer/rule_append_uninstall_test.go`
**Step 1 (test):** `TestUninstallRuleAppend_UnreadableTargetPreservesRecord` — install, chmod target 0000, call Uninstall, assert error contains `"reading"`, record still present.
**Step 2–4:** should PASS.
**Step 5 (commit):** `git commit -m "test(installer): uninstall EACCES preserves record (D7)"`

### Task 6.10: Uninstall — zero matches returns a "not found, edited" error

**Depends on:** 6.7
**Files:** `cli/internal/installer/rule_append_uninstall_test.go`
**Step 1 (test):** `TestUninstallRuleAppend_ZeroMatches` — install, mutate target so the block is gone, call Uninstall, assert error contains `"file may have been edited"`.
**Step 5 (commit):** `git commit -m "test(installer): uninstall zero-match → edited error (D20)"`

### Task 6.11: Uninstall — multiple matches returns a "multiple matches" error

**Depends on:** 6.7
**Files:** `cli/internal/installer/rule_append_uninstall_test.go`
**Step 1 (test):** `TestUninstallRuleAppend_MultipleMatches` — install, then manually `os.WriteFile(target, existing+"\n"+block+"\n"+block, 0644)` to duplicate the block. Call Uninstall, assert error contains `"multiple matches"`.
**Step 5 (commit):** `git commit -m "test(installer): uninstall multi-match defers to update flow (D20)"`

### Task 6.12: CLI — wire `syllago uninstall rules/foo` through `UninstallRuleAppend` for monolithic-installed rules

**Depends on:** 6.7
**Files:** `cli/cmd/syllago/uninstall_cmd.go`, `cli/cmd/syllago/uninstall_cmd_monolithic_test.go`
**Step 1 (test):** `TestUninstall_MonolithicRule_Roundtrip` — install with `--method=append`, uninstall, assert target is byte-equal to pre-install.
**Step 2–4:** iterate — detect when the target is a monolithic-append record and route to `UninstallRuleAppend` instead of the symlink path.
**Step 5 (commit):** `git commit -m "feat(cli): uninstall routes monolithic rules via D7 search (D7)"`

### Task 6.13: D21 ship gate — 10-cell roundtrip matrix at `cli/internal/installer/roundtrip_test.go`

**Depends on:** 6.2, 6.7
**Files:** `cli/internal/installer/roundtrip_test.go`
**Step 1 (test):** Create `roundtrip_test.go` with a single table-driven `TestRoundtrip_NormalizationChain` that enumerates 10 cells:
```go
fixtures := []string{"h2-clean.md", "crlf-line-endings.md", "bom-prefix.md", "no-trailing-newline.md", "trailing-whitespace.md"}
preStates := []string{"empty", "non-empty"}
```
Each cell runs the 8-step roundtrip chain from D21:
1. Load fixture bytes from `cli/internal/converter/testdata/splitter/<name>`.
2. Run `splitter.Split(canonical.Normalize(bytes), Options{Heuristic: splitter.HeuristicSingle})` → one `SplitCandidate`.
3. Normalize candidate body (already normalized via step 2's input).
4. `rulestore.WriteRuleWithSource(tmpLibrary, "claude-code", "fixture", metadata, []byte(cand.Body), filename, bytes)`.
5. `rulestore.LoadRule(ruleDir)` → `Loaded`.
6. For `pre=empty`: target file path exists but is empty (`os.WriteFile(target, nil, 0644)`); for `pre=non-empty`: seed with `"preamble content\nmore preamble\n"`. Snapshot pre-install bytes. Call `installer.InstallRuleAppend(projectRoot, home, "claude-code", target, "manual", loaded)`.
7. `installcheck.Scan(inst, library)` → assert `PerRecord[{libID, target}] == {StateClean, ReasonNone}`.
8. `installer.UninstallRuleAppend(projectRoot, libID, target, library)` → assert `os.ReadFile(target)` is byte-equal to the pre-install snapshot.
**Step 2 (expect fail):** likely some cells fail — specifically CRLF/BOM cases where a normalization layer is inconsistent between write, load, scan, and search. Debug by asserting at intermediate steps.
**Step 3 (implement):** Fix whichever path is applying normalization differently from D12's contract. The bug is almost certainly one of:
  (a) load path re-normalizes (D21 says it MUST NOT — bytes are already canonical on disk).
  (b) scan pattern forgets the leading `\n`.
  (c) write path re-encodes CRLF somewhere.
Fix at the root — do not paper over with double-normalization.
**Step 4 (pass):** all 10 cells pass.
**Step 5 (commit):** `git commit -m "test(installer): D21 10-cell normalization-chain roundtrip (ship gate)"`

### Task 6.14: Build + fmt + test gate for Phase 6 (includes D21)

**Depends on:** 6.1–6.13
**Files:** —
**Step 1:** `cd cli && make fmt && make build && make test` — all PASS, including D21. If any D21 cell fails, Phase 6 is NOT done per the design doc's ship gate.

---

## Phase 7 — Update flow

Scope: D17's explicit-decision routing. CLI `--on-clean` and `--on-modified` flags, TUI `installUpdateModal` and `installModifiedModal`, Replace-in-place per D20 byte splice. Depends on Phase 6's `VerificationResult` shape.

### Task 7.1: CLI flags `--on-clean` + `--on-modified` registered on install command

**Depends on:** Phase 6
**Files:** `cli/cmd/syllago/install_cmd.go`, `cli/cmd/syllago/install_cmd_flags_test.go`
**Step 1 (test):** Create `install_cmd_flags_test.go` with `TestInstall_OnCleanFlagAccepted` (flag takes values `replace|skip`, others error) and `TestInstall_OnModifiedFlagAccepted` (flag takes `drop-record|append-fresh|keep`, others error).
**Step 2 (expect fail):** flags do not exist.
**Step 3 (implement):**
```go
installCmd.Flags().String("on-clean", "", "Action when rule is already installed cleanly: replace|skip")
installCmd.Flags().String("on-modified", "", "Action when install record is stale: drop-record|append-fresh|keep")
```
Validate the value in `runInstall`; error with the exact messages from D17's flag table (e.g., `"rule already installed at clean state; specify --on-clean=replace|skip"`).
**Step 4 (pass):** PASS.
**Step 5 (commit):** `git commit -m "feat(cli): --on-clean and --on-modified flags (D17)"`

### Task 7.2: Install command routes through verification before mutating

**Depends on:** 7.1, Phase 6
**Files:** `cli/cmd/syllago/install_cmd.go`, `cli/cmd/syllago/install_cmd_update_test.go`
**Step 1 (test):** Create `install_cmd_update_test.go` with `TestInstall_CleanState_RequiresOnCleanFlagWhenNonInteractive`. Install once, then attempt to install the same rule into the same target non-interactively without `--on-clean`; assert exit code != 0 and stderr contains the exact message `"rule already installed at clean state; specify --on-clean=replace|skip"`.
**Step 2 (expect fail):** install just overwrites or appends naively.
**Step 3 (implement):** In `runInstall`, for `--method=append`, build the library map, run `installcheck.Scan`, inspect `result.PerRecord[{libID, target}]`. Branch:
- `StateFresh` or no key present → call `InstallRuleAppend`.
- `StateClean` + no `--on-clean` and not interactive → error above.
- `StateClean` + `--on-clean=skip` → print `"skipping: already installed"` and exit 0.
- `StateClean` + `--on-clean=replace` → call `installer.ReplaceRuleAppend(...)` (added in 7.4).
- `StateModified` + no `--on-modified` and not interactive → error `"rule install record is stale; specify --on-modified=drop-record|append-fresh|keep"`.
- `StateModified` + `--on-modified=drop-record` → remove the install record, no file change.
- `StateModified` + `--on-modified=append-fresh` → call `InstallRuleAppend` (creates file if missing per D20).
- `StateModified` + `--on-modified=keep` → no-op, exit 0.
**Step 4 (pass):** PASS.
**Step 5 (commit):** `git commit -m "feat(cli): install routes through D16 verification with D17 decisions"`

### Task 7.3: Non-interactive error messages match D17 exactly for all three Modified reasons

**Depends on:** 7.2
**Files:** `cli/cmd/syllago/install_cmd_update_test.go`
**Step 1 (test):** Table-driven `TestInstall_ModifiedState_NonInteractiveErrors` with three subtests — reason=edited, reason=missing, reason=unreadable. All three fail without the flag with exactly `"rule install record is stale; specify --on-modified=drop-record|append-fresh|keep"`.
**Step 2–4:** PASS from 7.2.
**Step 5 (commit):** `git commit -m "test(cli): non-interactive install errors identical across Modified reasons (D17)"`

### Task 7.4: `ReplaceRuleAppend` — in-place splice per D20

**Depends on:** 6.7
**Files:** `cli/internal/installer/rule_append.go`, `cli/internal/installer/rule_append_replace_test.go`
**Step 1 (test):** Create `rule_append_replace_test.go` with `TestReplaceRuleAppend_InPlace`. Install body A into a target with `"PRE\n"`, append body B via a second rule so target is `"PRE\n\nA\n\nB\n"`, then call `ReplaceRuleAppend(libA, newBodyA2)`. Assert target is `"PRE\n\nA2\n\nB\n"` (byte-for-byte).
**Step 2–4 (TDD):** implement:
```go
// ReplaceRuleAppend splices the recorded version's block in-place (D20 Replace).
// The caller must have already verified D16 State == Clean; this function
// re-runs the byte search at execute time (no offset caching).
func ReplaceRuleAppend(projectRoot, libraryID, targetFile string, newBody []byte, library map[string]*rulestore.Loaded) error {
    inst, err := LoadInstalled(projectRoot)
    if err != nil { return err }
    idx := inst.FindRuleAppend(libraryID, targetFile)
    if idx < 0 {
        return fmt.Errorf("no rule-append record for %s at %s", libraryID, targetFile)
    }
    raw, err := os.ReadFile(targetFile)
    if err != nil { return err }
    normalized := canonical.Normalize(raw)
    rule := library[libraryID]
    recorded := rule.History[inst.RuleAppends[idx].VersionHash]
    oldPattern := append([]byte{'\n'}, canonical.Normalize(recorded)...)
    if bytes.Count(normalized, oldPattern) != 1 {
        return fmt.Errorf("replace: expected exactly one match for recorded version in %s", targetFile)
    }
    newPattern := append([]byte{'\n'}, canonical.Normalize(newBody)...)
    cut := bytes.Index(normalized, oldPattern)
    out := append([]byte{}, normalized[:cut]...)
    out = append(out, newPattern...)
    out = append(out, normalized[cut+len(oldPattern):]...)
    if err := os.WriteFile(targetFile, out, 0644); err != nil { return err }
    installcheck.InvalidateCache(targetFile)
    newHash := rulestore.HashBody(newBody)
    inst.RuleAppends[idx].VersionHash = newHash
    return SaveInstalled(projectRoot, inst)
}
```
Caller must also ensure the new body's hash is in the library's history (Phase 3's `AppendVersion`). For Phase 7, wire that in `runInstall` before calling `ReplaceRuleAppend`.
**Step 5 (commit):** `git commit -m "feat(installer): ReplaceRuleAppend in-place splice (D17, D20)"`

### Task 7.5: Telemetry — `verification_state` + `decision_action` enrichment

**Depends on:** 7.2
**Files:** `cli/cmd/syllago/install_cmd.go`, `cli/internal/telemetry/catalog.go`
**Step 1 (test):** Will be caught by the existing drift test.
**Step 2 (expect fail):** drift test fails until catalog updated.
**Step 3 (implement):** After the decision branch in `runInstall`, call:
```go
telemetry.Enrich("verification_state", stateString) // "fresh" | "clean" | "modified"
telemetry.Enrich("decision_action", actionString)   // "proceed" | "replace" | "skip" | "drop_record" | "append_fresh" | "keep"
```
Add the two `PropertyDef` entries to `EventCatalog()` under `command_executed` with `Commands: []string{"install"}`. Run `cd cli && make gendocs`.
**Step 4 (pass):** `cd cli && go test ./internal/telemetry/` — PASS.
**Step 5 (commit):** `git commit -m "feat(telemetry): verification_state + decision_action (D17)"`

### Task 7.6: TUI — `installUpdateModal` for Case A (Clean)

**Depends on:** Phase 6, Phase 8 scaffolding available (or stub method picker for now)
**Files:** `cli/internal/tui/install_update_modal.go`, `cli/internal/tui/install_update_modal_test.go`
**Step 1 (test):** Create `install_update_modal_test.go` with `TestInstallUpdateModal_Renders` — render modal with `recordedShortHash = "sha256:abc123abcdef"` and `newShortHash = "sha256:def456abcdef"`, assert rendered text contains `"This rule is already installed at:"`, `"Replace with current version"`, `"Skip (leave file unchanged)"`, and that `[Enter]` is bound to Replace (default per D17).
**Step 2 (expect fail):** modal does not exist.
**Step 3 (implement):** Create `install_update_modal.go` following the modal pattern from `.claude/rules/tui-modals.md` (manual box-drawing borders, `modalWidth = 56`, accent color). Two focusable buttons via `renderModalButtons`. `zone.Mark` on each option. Emits `installUpdateDecisionMsg{action: "replace"|"skip"}` on Enter/Esc. Default focus is Replace.
**Step 4 (pass):** PASS.
**Step 5 (commit):** `git commit -m "feat(tui): installUpdateModal for Clean state (D17 Case A)"`

### Task 7.7: TUI — `installModifiedModal` for Case B (all 3 reasons)

**Depends on:** 7.6
**Files:** `cli/internal/tui/install_modified_modal.go`, `cli/internal/tui/install_modified_modal_test.go`
**Step 1 (test):** Three subtests in `install_modified_modal_test.go` — one per reason (`edited`, `missing`, `unreadable`). Each renders the modal and asserts:
- `edited` modal contains `"...but the file no longer contains the version we recorded."`, default focus on "Drop stale install record".
- `missing` modal contains `"...but that file no longer exists."`, default focus on "Drop stale install record".
- `unreadable` modal contains `"...but we couldn't read the file:"`, default focus on "Keep (leave record + file unchanged)".
**Step 2–4 (TDD):** implement the modal with three options: Drop, Append fresh, Keep — each `zone.Mark`'d. Selection default swaps per reason per D17 table.
**Step 5 (commit):** `git commit -m "feat(tui): installModifiedModal with reason-varying defaults (D17 Case B)"`

### Task 7.8: TUI install command wires verification → modal routing

**Depends on:** 7.6, 7.7, Phase 6
**Files:** `cli/internal/tui/install.go`, `cli/internal/tui/install_update.go`, `cli/internal/tui/install_update_routing_test.go`
**Step 1 (test):** `TestInstallWizard_RoutesCleanThroughUpdateModal` — mock a library rule in Clean state, run wizard to Review step, press Enter (install). Assert `installUpdateModal` is displayed before any file mutation.
**Step 2–4 (TDD):** before the final `installResultMsg` dispatch in the review confirmation, run `installcheck.Scan` and route to the appropriate modal. The modal's decision message ultimately produces the same `installResultMsg` plus a `decisionAction` field consumed by the downstream install executor.
**Step 5 (commit):** `git commit -m "feat(tui): install wizard routes through update/modified modals (D17)"`

### Task 7.9: Golden files for the two modals

**Depends on:** 7.6, 7.7
**Files:** `cli/internal/tui/testdata/install-update-modal-*.golden`, `install-modified-modal-*.golden`
**Step 1 (test):** Add `TestGoldenInstallUpdateModal_Sizes` and `TestGoldenInstallModifiedModal_Reasons_Sizes`.
**Step 2–4:** `-update-golden` to generate, review, ensure sizes 60x20 / 80x30 / 120x40 render cleanly.
**Step 5 (commit):** `git commit -m "test(tui): golden files for install update/modified modals"`

### Task 7.10: Build + fmt + gendocs + test gate for Phase 7

**Depends on:** 7.1–7.9
**Step 1:** `cd cli && make fmt && make build && make gendocs && make test`.

---

## Phase 8 — TUI install picker

Scope: the TUI install wizard presents two choices for rule install — individual-file install (existing path) and monolithic append (new path) — with per-provider `MonolithicHint` as a non-blocking note in the Review step.

### Task 8.1: `installStepMethod` now distinguishes "file" vs "append" for rules

**Depends on:** Phase 5
**Files:** `cli/internal/tui/install.go`, `cli/internal/tui/install_view.go`, `cli/internal/tui/install_method_test.go`
**Step 1 (test):** Create `install_method_test.go` with `TestInstallWizard_MethodStep_OffersFileAndAppendForRules`. Seed wizard with a rule item and a provider that supports monolithic files; assert both options are rendered with `[Enter]` selecting the first.
**Step 2 (expect fail):** current method step doesn't offer append.
**Step 3 (implement):** In `install_view.go`'s method-step render, when `catalog.ContentType(item) == catalog.Rules` AND `provider.MonolithicFilenames(slug) != nil`, add the "Append to <filename>" option alongside the existing file-install option. `zone.Mark` both. For providers without a monolithic file (rare for rules), hide the append option.
**Step 4 (pass):** PASS.
**Step 5 (commit):** `git commit -m "feat(tui): install method picker adds append option for rules (D5)"`

### Task 8.2: Review step renders the `MonolithicHint` when method=append

**Depends on:** 8.1, Phase 1 provider hints
**Files:** `cli/internal/tui/install_view.go`, `cli/internal/tui/install_view_test.go`
**Step 1 (test):** `TestInstallWizard_ReviewShowsMonolithicHint` — set method=append and provider=codex; render review; assert view contains `"Codex prefers per-directory AGENTS.md files"` formatted as a note (muted color, no block icon).
**Step 2–4:** render a small one-line note above the Install button when `provider.MonolithicHint(slug) != ""`.
**Step 5 (commit):** `git commit -m "feat(tui): review shows monolithic install hint (D10)"`

### Task 8.3: Mouse parity + golden files for the updated method + review steps

**Depends on:** 8.1, 8.2
**Files:** `cli/internal/tui/install_mouse_test.go`, `cli/internal/tui/testdata/install-method-append-*.golden`
**Step 1 (test):** Click "Append to CLAUDE.md" option in method step. Assert same transition as keyboard Enter on that row. Golden files at 3 sizes for the new option.
**Step 2–4:** `-update-golden`, review.
**Step 5 (commit):** `git commit -m "test(tui): mouse + goldens for method=append (D5, D10)"`

### Task 8.4: Build + fmt + test gate for Phase 8

**Step 1:** `cd cli && make fmt && make build && make test`.

---

## Phase 9 — Library "Installed" status surface

Scope: wire Phase 6's `VerificationResult` into the TUI library column as a binary Installed/Not-Installed summary (D16 "column is binary"), and into the metadata panel as a per-target breakdown with Reason strings (`Clean`, `Modified · edited`, `Modified · missing`, `Modified · unreadable`).

### Task 9.1: Library column reads `MatchSet` from `installcheck.Scan`

**Depends on:** Phase 6
**Files:** `cli/internal/tui/library.go`, `cli/internal/tui/library_installed_test.go`
**Step 1 (test):** Create `library_installed_test.go` with `TestLibrary_InstalledColumnBinary`. Seed a tmp library + installed.json with 2 rules: one installed cleanly into the project CLAUDE.md, one never installed. Call `library.SetItems(catalog.Items, scanResult)`; assert row 0's "Installed" column contains `"✓"` (or whatever the existing style uses — check `styles.go`'s `successColor` usage); row 1 contains blank / `"-"`.
**Step 2–4 (TDD):** extend `libraryModel.items` or a parallel `verificationResult *installcheck.VerificationResult` field; in View, check `scanResult.MatchSet[item.LibraryID]` — non-empty = Installed, else Not Installed. Use existing Installed column styling (match hooks/MCP pattern).
**Step 5 (commit):** `git commit -m "feat(tui): library Installed column for rules uses D16 MatchSet (D16)"`

### Task 9.2: Metadata panel — per-target status breakdown

**Depends on:** 9.1
**Files:** `cli/internal/tui/metapanel.go`, `cli/internal/tui/metapanel_rule_test.go`
**Step 1 (test):** Create `metapanel_rule_test.go` with `TestMetaPanel_RuleShowsPerTargetStatus`. Three records for one rule: target A Clean, target B Modified/edited, target C Modified/missing. Render metapanel; assert each target is listed with its status (`"Clean"`, `"Modified · edited"`, `"Modified · missing"`).
**Step 2–4 (TDD):** extend metapanel's render path for content type `rule` with a new subsection "Installed at:" that lists one line per `InstalledRuleAppend` record + its looked-up `PerTargetState`.
**Step 5 (commit):** `git commit -m "feat(tui): metapanel shows per-target rule install status (D16)"`

### Task 9.3: Rescan hook propagates verification result

**Depends on:** 9.1
**Files:** `cli/internal/tui/app.go`, `cli/internal/tui/app_update.go`, `cli/internal/tui/rescan_test.go`
**Step 1 (test):** `TestApp_RescanUpdatesVerificationState` — modify a target file behind the app's back, press `R`, assert library column transitions from Installed → Not Installed in the rendered view.
**Step 2–4 (TDD):** inside `rescanCatalog()`, after rebuilding the catalog, call `installcheck.Scan(inst, library)`; store the result on the App; pass it to `library.SetItems(...)` and `metapanel.SetVerification(...)`; invoke `a.refreshContent()` per `.claude/rules/tui-items-rebuild.md`.
**Step 5 (commit):** `git commit -m "feat(tui): rescan re-runs verification scan (D16)"`

### Task 9.4: Golden files for the new Installed column + metapanel breakdown

**Depends on:** 9.1, 9.2
**Files:** `cli/internal/tui/testdata/library-installed-rules-*.golden`, `metapanel-rule-installed-*.golden`
**Step 1 (test):** Add golden tests at 3 sizes.
**Step 2–4:** `-update-golden`, review.
**Step 5 (commit):** `git commit -m "test(tui): golden files for rule Installed column + metapanel"`

### Task 9.5: Build + fmt + test gate for Phase 9

**Step 1:** `cd cli && make fmt && make build && make test`.

---

## Phase 10 — `split-rules-llm` skill in syllago-meta-registry

Scope: author the `split-rules-llm` skill as a first-class content item under the separate `syllago-meta-registry` repository and publish it so `syllago add split-rules-llm` resolves from the default meta-registry. Both pieces — authoring + publishing — are hard V1 ship gates; without them, `--split=llm` in the binary is a dead flag.

The skill's runtime contract: it reads a monolithic source file, calls the user's LLM (via whatever shell command the harness configures), and emits a JSON array matching `splitter.SplitCandidate` so the wizard's Review step and the CLI's `runAdd` code can consume it through the same pipeline as the deterministic splitter.

### Task 10.1: Add a skill discovery function in the CLI — `splitterllm.Available()`

**Depends on:** Phase 3
**Files:** `cli/internal/splitterllm/available.go`, `cli/internal/splitterllm/available_test.go`
**Step 1 (test):** Create `available_test.go` with:
- `TestAvailable_NotInstalled` — no `split-rules-llm` skill in the library; `Available(catalogItems)` returns `false, "install `split-rules-llm` from syllago-meta-registry: syllago add split-rules-llm"`.
- `TestAvailable_Installed` — seed a skill with canonical name `split-rules-llm`; returns `true, ""`.
**Step 2 (expect fail):** undefined.
**Step 3 (implement):** Create `available.go`:
```go
// Package splitterllm detects the presence of the split-rules-llm skill
// (authored under syllago-meta-registry per D9) and provides the install
// pointer when absent.
package splitterllm

import "github.com/OpenScribbler/syllago/cli/internal/catalog"

// SkillName is the canonical name of the LLM-split skill in the user's library.
const SkillName = "split-rules-llm"

// Available returns whether the split-rules-llm skill is installed in the
// user's library. When not installed, the second return value is the one-line
// install pointer per D9 non-interactive behavior.
func Available(items []catalog.ContentItem) (bool, string) {
    for _, it := range items {
        if it.Type == catalog.Skills && it.Name == SkillName {
            return true, ""
        }
    }
    return false, "install `split-rules-llm` from syllago-meta-registry: syllago add split-rules-llm"
}
```
**Step 4 (pass):** PASS.
**Step 5 (commit):** `git commit -m "feat(splitterllm): Available detector + install pointer (D9)"`

### Task 10.2: CLI `--split=llm` calls the skill and parses its output

**Depends on:** 10.1, 5.10
**Files:** `cli/internal/splitterllm/invoke.go`, `cli/internal/splitterllm/invoke_test.go`
**Step 1 (test):** Create `invoke_test.go` with `TestInvoke_ParsesJSONArray` — seed a fake skill that echoes a JSON array of three `SplitCandidate` objects (use a shell script fixture in `testdata/`). Call `Invoke(skillPath, sourceBytes)`; assert three candidates are returned.
**Step 2 (expect fail):** undefined.
**Step 3 (implement):** Create `invoke.go`:
```go
package splitterllm

import (
    "bytes"
    "encoding/json"
    "fmt"
    "os/exec"

    "github.com/OpenScribbler/syllago/cli/internal/splitter"
)

// Invoke runs the split-rules-llm skill and parses its stdout as a JSON
// array of SplitCandidate per D9 structured output contract.
func Invoke(skillCommand string, source []byte) ([]splitter.SplitCandidate, error) {
    cmd := exec.Command(skillCommand)
    cmd.Stdin = bytes.NewReader(source)
    out, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("invoking split-rules-llm skill: %w", err)
    }
    var cands []splitter.SplitCandidate
    if err := json.Unmarshal(out, &cands); err != nil {
        return nil, fmt.Errorf("parsing split-rules-llm output: %w", err)
    }
    return cands, nil
}
```
**Step 4 (pass):** PASS.
**Step 5 (commit):** `git commit -m "feat(splitterllm): Invoke wraps skill as structured producer (D9)"`

### Task 10.3: Wire `--split=llm` through `runAdd`

**Depends on:** 5.10, 10.1, 10.2
**Files:** `cli/cmd/syllago/add_cmd.go`, `cli/cmd/syllago/add_cmd_llm_test.go`
**Step 1 (test):** Create `add_cmd_llm_test.go` with:
- `TestAdd_SplitLLM_MissingSkillErrors` — no skill; `syllago add --from <path> --split=llm` errors with the exact install-pointer message.
- `TestAdd_SplitLLM_SkillInstalled_WritesRules` — stub skill installed; asserts candidate rules land in the library.
**Step 2–4 (TDD):** in `runAdd`, when `--split=llm`, call `splitterllm.Available(catalog.Items)`; error if not; otherwise `splitterllm.Invoke(skillExecPath, sourceBytes)` and feed the resulting `[]SplitCandidate` through the same write path as the deterministic splitter.
**Step 5 (commit):** `git commit -m "feat(cli): --split=llm calls the split-rules-llm skill (D9)"`

### Task 10.4: Author `split-rules-llm/SKILL.md` in syllago-meta-registry

**Depends on:** 10.3
**Files:** `<syllago-meta-registry>/skills/split-rules-llm/SKILL.md`, `<syllago-meta-registry>/skills/split-rules-llm/prompt.md`, `<syllago-meta-registry>/skills/split-rules-llm/.syllago.yaml`
**Step 1 (test):** In the meta-registry repo, add a smoke test that shells out to the skill with a fixed input from `testdata/h2-clean.md` and asserts the JSON array it emits has 3 entries each with `Name`, `Description`, `Body`, `OriginalRange`.
**Step 2 (expect fail):** skill doesn't exist yet.
**Step 3 (implement):** Author the three files:
- `SKILL.md`: describes what the skill does, input format (monolithic markdown on stdin), output format (JSON array `[{Name, Description, Body, OriginalRange}]`). Declares its invocation contract and token expectations.
- `prompt.md`: the prompt template the skill sends to the user's LLM. Inputs placeholder `{SOURCE}` replaced with the file contents. Output spec: "Respond with a JSON array and nothing else. Each element MUST have fields Name, Description, Body, OriginalRange."
- `.syllago.yaml`: metadata with `name: split-rules-llm`, `type: skill`, standard fields.
**Step 4 (pass):** skill smoke test PASS.
**Step 5 (commit):** in syllago-meta-registry: `git commit -m "feat(skills): split-rules-llm — authored per syllago D9"`

### Task 10.5: Publish `split-rules-llm` to the default syllago-meta-registry

**Depends on:** 10.4
**Files:** registry index / manifest under syllago-meta-registry (format matches existing published skills)
**Step 1 (test):** On a fresh machine (or clean clone), run `syllago add split-rules-llm` and assert the skill lands in the user's library.
**Step 2 (expect fail):** the skill isn't in the registry index yet.
**Step 3 (implement):** Publish via whatever the syllago-meta-registry publish flow requires (commit to default branch + tag, or explicit manifest update — check the meta-registry's README). Ensure the canonical name `split-rules-llm` resolves.
**Step 4 (pass):** end-to-end `syllago add split-rules-llm` works from a clean syllago install.
**Step 5 (commit):** meta-registry: `git commit -m "chore(publish): split-rules-llm available via syllago add"`

### Task 10.6: V1 ship gate — end-to-end integration test

**Depends on:** 10.5
**Files:** `cli/cmd/syllago/add_cmd_llm_e2e_test.go`
**Step 1 (test):** Create `add_cmd_llm_e2e_test.go` gated behind `SYLLAGO_TEST_NETWORK=1`. The test:
1. Runs `syllago add split-rules-llm` against the real meta-registry.
2. Runs `syllago add --from cli/internal/converter/testdata/splitter/h2-clean.md --split=llm` using the newly-installed skill.
3. Asserts candidate rules appear in the library.
**Step 2 (expect fail):** without 10.5 complete, step 1 fails.
**Step 3 (implement):** Add the test with the env-var gate:
```go
if os.Getenv("SYLLAGO_TEST_NETWORK") == "" {
    t.Skip("set SYLLAGO_TEST_NETWORK=1 to run network-dependent tests")
}
```
**Step 4 (pass):** `SYLLAGO_TEST_NETWORK=1 go test ./cmd/syllago/ -run TestAdd_SplitLLM_E2E` — PASS.
**Step 5 (commit):** `git commit -m "test(cli): E2E split-rules-llm via real meta-registry (D9 ship gate)"`

### Task 10.7: Build + fmt + gendocs + test gate for Phase 10

**Depends on:** 10.1–10.6
**Step 1:** `cd cli && make fmt && make build && make gendocs && make test`. V1 is callable only after 10.5 + 10.6 are green.

---

## Completion criteria — "V1 is callable"

All of the following must be true, in order:

1. Every task in Phases 1–9 committed with green `make test` at each phase's gate.
2. D21's 10-cell roundtrip matrix (Task 6.13) passes — zero skips.
3. `syllago add split-rules-llm` resolves against the real syllago-meta-registry (Task 10.5 + 10.6).
4. `telemetry.json` and `commands.json` regenerated via `make gendocs` and committed.
5. `cd cli && make fmt && make build && make test` on the main branch is green.

If any one of these is missing, V1 is not callable per the design doc's ship gates.
