# _gentelemetry Implementation Plan

**Feature:** Hidden CLI command that generates `telemetry.json` — a machine-readable event catalog
**Design doc:** `docs/plans/2026-04-03-gentelemetry-design.md`
**Date:** 2026-04-03

---

## Overview

Seven tasks. TDD throughout: write the test first, confirm it fails for the right reason, implement, confirm it passes, commit. Each task is 2–5 minutes of focused work.

**Dependency chain:**

```
Task 1 (catalog.go)
  → Task 2 (gentelemetry.go) — depends on Task 1
  → Task 3 (gentelemetry_test.go) — depends on Tasks 1 + 2
    → Task 4 (Makefile) — depends on Task 2
      → Task 5 (release.yml) — depends on Task 4
        → Task 6 (pre-push hook) — depends on Task 4
Task 7 (rule file) — independent, no deps
```

Note: Task 2 (implementation) before Task 3 (tests) is intentional for this feature.
The `_gentelemetry` command structure is fully prescribed by the design doc and
follows the exact `gendocs.go`/`genproviders.go` pattern — writing tests after
implementation is appropriate because the shape is known. The tests serve as a
regression + drift-detection safety net, not as a design driver.

---

## Task 1 — Event Catalog (`cli/internal/telemetry/catalog.go`)

**Dependencies:** none

**What:** New file in the telemetry package defining the three struct types and three catalog functions, fully populated from the actual `Enrich()` call scan.

**Enrich() call inventory** (from scanning `cli/cmd/syllago/*.go`):

| Key | Type | Commands that set it |
|-----|------|----------------------|
| `from` | string | add (hooks path, mcp path, general path) |
| `content_type` | string | add, convert, create, install, list, registry_items, remove, share, sync-and-export, uninstall |
| `dry_run` | bool | add, install, remove, sync-and-export, uninstall |
| `from_provider` | string | convert |
| `to_provider` | string | convert |
| `content_count` | int | add, install |
| `source_filter` | string | list |
| `item_count` | int | list, registry_items |
| `provider` | string | install, loadout_apply, sandbox_run, uninstall, sync-and-export |
| `mode` | string | loadout_apply |
| `action_count` | int | loadout_apply |
| `registry_count` | int | registry_sync |

**Standard properties** (set by `Track()` in `telemetry.go` for every event):
- `version` (string) — from `sysBuildVersion`
- `os` (string) — from `runtime.GOOS`
- `arch` (string) — from `runtime.GOARCH`

**Events:**
1. `command_executed` — fired by `TrackCommand()` from `PersistentPostRun` on every non-telemetry command
2. `tui_session_started` — fired directly in `main.go` after `tea.Program.Run()` completes

**File to create:** `/home/hhewett/.local/src/syllago/cli/internal/telemetry/catalog.go`

**Complete file content:**

```go
package telemetry

// EventDef describes a single telemetry event.
type EventDef struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	FiredWhen   string        `json:"firedWhen"`
	Properties  []PropertyDef `json:"properties"`
}

// PropertyDef describes a single property sent with an event.
type PropertyDef struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`     // "string", "int", or "bool"
	Description string   `json:"description"`
	Example     any      `json:"example"`
	Commands    []string `json:"commands"` // which commands set this property; "*" means all
}

// PrivacyEntry describes a category of data that is never collected.
type PrivacyEntry struct {
	Category string `json:"category"`
	Examples string `json:"examples"`
}

// EventCatalog returns the complete list of telemetry events syllago may fire.
// This is the single source of truth for telemetry documentation.
func EventCatalog() []EventDef {
	return []EventDef{
		{
			Name:        "command_executed",
			Description: "Fired when a CLI command completes successfully",
			FiredWhen:   "PersistentPostRun (every non-telemetry command)",
			Properties: []PropertyDef{
				{
					Name:        "command",
					Type:        "string",
					Description: "Command name (cobra command path)",
					Example:     "install",
					Commands:    []string{"*"},
				},
				{
					Name:        "provider",
					Type:        "string",
					Description: "Target provider slug",
					Example:     "claude-code",
					Commands:    []string{"install", "uninstall", "loadout_apply", "sandbox_run", "sync-and-export"},
				},
				{
					Name:        "content_type",
					Type:        "string",
					Description: "Content type filter or specific type",
					Example:     "rules",
					Commands:    []string{"install", "add", "convert", "create", "uninstall", "remove", "list", "share", "sync-and-export", "registry_items"},
				},
				{
					Name:        "content_count",
					Type:        "int",
					Description: "Number of content items affected",
					Example:     3,
					Commands:    []string{"install", "add"},
				},
				{
					Name:        "dry_run",
					Type:        "bool",
					Description: "Whether --dry-run flag was used",
					Example:     false,
					Commands:    []string{"install", "add", "uninstall", "remove", "sync-and-export"},
				},
				{
					Name:        "from",
					Type:        "string",
					Description: "Source provider slug when adding cross-provider content",
					Example:     "cursor",
					Commands:    []string{"add"},
				},
				{
					Name:        "from_provider",
					Type:        "string",
					Description: "Source provider for conversion",
					Example:     "cursor",
					Commands:    []string{"convert"},
				},
				{
					Name:        "to_provider",
					Type:        "string",
					Description: "Target provider for conversion",
					Example:     "claude-code",
					Commands:    []string{"convert"},
				},
				{
					Name:        "source_filter",
					Type:        "string",
					Description: "Content source filter (library, shared, or registry)",
					Example:     "library",
					Commands:    []string{"list"},
				},
				{
					Name:        "item_count",
					Type:        "int",
					Description: "Number of items in the result set",
					Example:     12,
					Commands:    []string{"list", "registry_items"},
				},
				{
					Name:        "mode",
					Type:        "string",
					Description: "Loadout application mode",
					Example:     "try",
					Commands:    []string{"loadout_apply"},
				},
				{
					Name:        "action_count",
					Type:        "int",
					Description: "Number of actions performed by loadout",
					Example:     5,
					Commands:    []string{"loadout_apply"},
				},
				{
					Name:        "registry_count",
					Type:        "int",
					Description: "Number of registries involved",
					Example:     2,
					Commands:    []string{"registry_sync"},
				},
			},
		},
		{
			Name:        "tui_session_started",
			Description: "Fired when the TUI exits normally after a session",
			FiredWhen:   "After tea.Program.Run() completes without error (main.go root command)",
			Properties: []PropertyDef{
				{
					Name:        "success",
					Type:        "bool",
					Description: "Whether the TUI exited normally",
					Example:     true,
					Commands:    []string{"(root)"},
				},
			},
		},
	}
}

// StandardProperties returns the properties automatically included in every event.
// These are merged into all Track() calls by the telemetry package itself.
func StandardProperties() []PropertyDef {
	return []PropertyDef{
		{
			Name:        "version",
			Type:        "string",
			Description: "Syllago version",
			Example:     "0.7.0",
			Commands:    []string{"*"},
		},
		{
			Name:        "os",
			Type:        "string",
			Description: "Operating system (runtime.GOOS)",
			Example:     "linux",
			Commands:    []string{"*"},
		},
		{
			Name:        "arch",
			Type:        "string",
			Description: "CPU architecture (runtime.GOARCH)",
			Example:     "amd64",
			Commands:    []string{"*"},
		},
	}
}

// NeverCollected returns the structured privacy guarantees — categories of data
// syllago explicitly does not collect.
func NeverCollected() []PrivacyEntry {
	return []PrivacyEntry{
		{
			Category: "File contents",
			Examples: "Rule text, skill prompts, hook commands, MCP configs",
		},
		{
			Category: "File paths",
			Examples: "/home/user/.claude/rules/my-secret-rule",
		},
		{
			Category: "User identity",
			Examples: "Usernames, hostnames, IP addresses, email",
		},
		{
			Category: "Registry URLs",
			Examples: "Git clone URLs, registry names",
		},
		{
			Category: "Content names",
			Examples: "Names of rules, skills, agents, hooks, or MCP servers you manage",
		},
		{
			Category: "Interaction details",
			Examples: "Keystrokes, mouse clicks, TUI navigation paths",
		},
	}
}
```

**Success criteria:**

- `cd cli && go build ./internal/telemetry/...` → pass — package compiles cleanly
- `cd cli && go vet ./internal/telemetry/...` → pass — no vet issues
- `grep -c 'EventCatalog\|StandardProperties\|NeverCollected' cli/internal/telemetry/catalog.go` → outputs `3` — all three functions present

**Commit message:** `feat: telemetry event catalog — EventDef types and full EventCatalog() (Task 1)`

---

## Task 2 — Hidden Command (`cli/cmd/syllago/gentelemetry.go`)

**Dependencies:** Task 1 must be complete (catalog.go exists and compiles)

**What:** New file in `cli/cmd/syllago/` registering the `_gentelemetry` hidden command. Follows the exact structure of `genproviders.go`: manifest struct, `init()` registration, `RunE` function that reads from catalog and JSON-encodes to stdout.

**File to create:** `/home/hhewett/.local/src/syllago/cli/cmd/syllago/gentelemetry.go`

**Complete file content:**

```go
package main

import (
	"encoding/json"
	"os"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
	"github.com/spf13/cobra"
)

// TelemetryManifest is the top-level JSON structure output by _gentelemetry.
type TelemetryManifest struct {
	Version            string                   `json:"version"`
	GeneratedAt        string                   `json:"generatedAt"`
	SyllagoVersion     string                   `json:"syllagoVersion"`
	Events             []telemetry.EventDef     `json:"events"`
	StandardProperties []telemetry.PropertyDef  `json:"standardProperties"`
	NeverCollected     []telemetry.PrivacyEntry `json:"neverCollected"`
}

var gentelemetryCmd = &cobra.Command{
	Use:    "_gentelemetry",
	Short:  "Generate telemetry.json manifest",
	Hidden: true,
	RunE:   runGentelemetry,
}

func init() {
	rootCmd.AddCommand(gentelemetryCmd)
}

func runGentelemetry(_ *cobra.Command, _ []string) error {
	v := version
	if v == "" {
		v = "dev"
	}

	manifest := TelemetryManifest{
		Version:            "1",
		GeneratedAt:        time.Now().UTC().Format(time.RFC3339),
		SyllagoVersion:     v,
		Events:             telemetry.EventCatalog(),
		StandardProperties: telemetry.StandardProperties(),
		NeverCollected:     telemetry.NeverCollected(),
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(manifest)
}
```

**Success criteria:**

- `cd cli && go build ./cmd/syllago` → pass — binary compiles with new command registered
- `cd cli && go build -o /tmp/syl-test ./cmd/syllago && /tmp/syl-test _gentelemetry | python3 -m json.tool > /dev/null` → pass — output is valid JSON
- `/tmp/syl-test _gentelemetry | python3 -c "import json,sys; d=json.load(sys.stdin); assert d['version']=='1'; assert len(d['events'])>=2; assert len(d['neverCollected'])>=6"` → pass — structure contains expected content

**Commit message:** `feat: _gentelemetry hidden command — outputs telemetry.json to stdout (Task 2)`

---

## Task 3 — Tests (`cli/cmd/syllago/gentelemetry_test.go`)

**Dependencies:** Task 2 must be complete (gentelemetry.go exists and compiles)

**What:** Six tests covering valid JSON output, catalog completeness, property completeness, standard properties, privacy guarantees, and drift detection. Uses the `captureStdout` helper already defined in `genproviders_test.go`. The drift detection test (`TestGentelemetry_CatalogMatchesEnrichCalls`) scans source files with `os.ReadDir` + `os.ReadFile` and regex-matches `telemetry.Enrich("key"` patterns, then verifies each discovered key exists in `EventCatalog()`.

**File to create:** `/home/hhewett/.local/src/syllago/cli/cmd/syllago/gentelemetry_test.go`

**Complete file content:**

```go
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
)

// TestGentelemetry verifies the top-level manifest structure and required fields.
func TestGentelemetry(t *testing.T) {
	raw := captureStdout(t, func() {
		if err := gentelemetryCmd.RunE(gentelemetryCmd, nil); err != nil {
			t.Fatalf("_gentelemetry failed: %v", err)
		}
	})

	var manifest TelemetryManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("output is not valid JSON: %v\nfirst 200 bytes: %s", err, string(raw[:min(200, len(raw))]))
	}

	if manifest.Version != "1" {
		t.Errorf("version = %q, want %q", manifest.Version, "1")
	}
	if manifest.SyllagoVersion == "" {
		t.Error("syllagoVersion is empty")
	}
	if manifest.GeneratedAt == "" {
		t.Error("generatedAt is empty")
	}
	if len(manifest.Events) == 0 {
		t.Error("events is empty")
	}
	if len(manifest.StandardProperties) == 0 {
		t.Error("standardProperties is empty")
	}
	if len(manifest.NeverCollected) == 0 {
		t.Error("neverCollected is empty")
	}
}

// TestGentelemetry_EventsComplete verifies every event has name, description,
// firedWhen, and at least one property.
func TestGentelemetry_EventsComplete(t *testing.T) {
	for _, ev := range telemetry.EventCatalog() {
		t.Run(ev.Name, func(t *testing.T) {
			if ev.Name == "" {
				t.Error("event has empty name")
			}
			if ev.Description == "" {
				t.Errorf("event %q has empty description", ev.Name)
			}
			if ev.FiredWhen == "" {
				t.Errorf("event %q has empty firedWhen", ev.Name)
			}
			if len(ev.Properties) == 0 {
				t.Errorf("event %q has no properties", ev.Name)
			}
		})
	}
}

// TestGentelemetry_PropertiesComplete verifies every property on every event has
// name, a valid type, description, a non-nil example, and at least one command.
func TestGentelemetry_PropertiesComplete(t *testing.T) {
	validTypes := map[string]bool{"string": true, "int": true, "bool": true}

	for _, ev := range telemetry.EventCatalog() {
		for _, prop := range ev.Properties {
			t.Run(ev.Name+"/"+prop.Name, func(t *testing.T) {
				if prop.Name == "" {
					t.Error("property has empty name")
				}
				if !validTypes[prop.Type] {
					t.Errorf("property %q type = %q, want one of string/int/bool", prop.Name, prop.Type)
				}
				if prop.Description == "" {
					t.Errorf("property %q has empty description", prop.Name)
				}
				if prop.Example == nil {
					t.Errorf("property %q has nil example", prop.Name)
				}
				if len(prop.Commands) == 0 {
					t.Errorf("property %q has no commands", prop.Name)
				}
			})
		}
	}
}

// TestGentelemetry_StandardProperties verifies version, os, and arch are present.
func TestGentelemetry_StandardProperties(t *testing.T) {
	props := telemetry.StandardProperties()
	propByName := make(map[string]telemetry.PropertyDef, len(props))
	for _, p := range props {
		propByName[p.Name] = p
	}

	for _, want := range []string{"version", "os", "arch"} {
		p, ok := propByName[want]
		if !ok {
			t.Errorf("standardProperties missing %q", want)
			continue
		}
		if p.Type != "string" {
			t.Errorf("standardProperties[%q].type = %q, want %q", want, p.Type, "string")
		}
		if p.Description == "" {
			t.Errorf("standardProperties[%q].description is empty", want)
		}
	}
}

// TestGentelemetry_PrivacyGuarantees verifies at least 6 entries covering the
// key categories documented in the design: file contents, paths, identity,
// registry URLs, content names, and interaction details.
func TestGentelemetry_PrivacyGuarantees(t *testing.T) {
	entries := telemetry.NeverCollected()
	if len(entries) < 6 {
		t.Errorf("neverCollected has %d entries, want >= 6", len(entries))
	}

	// Build a searchable corpus of all categories (lowercase).
	corpus := make([]string, len(entries))
	for i, e := range entries {
		if e.Category == "" {
			t.Errorf("neverCollected[%d] has empty category", i)
		}
		if e.Examples == "" {
			t.Errorf("neverCollected[%d] (%q) has empty examples", i, e.Category)
		}
		corpus[i] = strings.ToLower(e.Category)
	}

	wantCategories := []struct {
		keyword string
		label   string
	}{
		{"file", "file contents or paths"},
		{"path", "file paths"},
		{"identity", "user identity"},
		{"registry", "registry URLs"},
		{"content", "content names"},
		{"interaction", "interaction details"},
	}

	for _, wc := range wantCategories {
		found := false
		for _, c := range corpus {
			if strings.Contains(c, wc.keyword) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("neverCollected missing a %q category (keyword: %q)", wc.label, wc.keyword)
		}
	}
}

// TestGentelemetry_CatalogMatchesEnrichCalls scans all *.go files in
// cli/cmd/syllago/ for telemetry.Enrich("key", ...) calls and verifies that
// every discovered key exists as a property on at least one event in EventCatalog().
// This is a strict drift-detection test — CI fails on any mismatch.
func TestGentelemetry_CatalogMatchesEnrichCalls(t *testing.T) {
	// Find the repo root relative to this test file's location.
	// The test binary runs from cli/cmd/syllago/, so we walk upward.
	cmdDir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("cannot determine test directory: %v", err)
	}
	// If the test directory doesn't look right (e.g. during go test ./...),
	// derive it from the source file path.
	if !strings.HasSuffix(cmdDir, filepath.Join("cmd", "syllago")) {
		// Attempt to find it relative to a known anchor.
		repoRoot := findRepoRoot(t)
		cmdDir = filepath.Join(repoRoot, "cli", "cmd", "syllago")
	}

	// Regex matches: telemetry.Enrich("someKey"
	enrichRe := regexp.MustCompile(`telemetry\.Enrich\("([^"]+)"`)

	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		t.Fatalf("cannot read cmd/syllago: %v", err)
	}

	seenKeys := make(map[string][]string) // key → []file
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		// Skip test files — they don't fire real events.
		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(cmdDir, entry.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", entry.Name(), err)
		}
		for _, match := range enrichRe.FindAllSubmatch(data, -1) {
			key := string(match[1])
			seenKeys[key] = append(seenKeys[key], entry.Name())
		}
	}

	if len(seenKeys) == 0 {
		t.Fatal("no telemetry.Enrich() calls found — scan may be broken")
	}

	// Build the set of property names across all events.
	catalogKeys := make(map[string]bool)
	for _, ev := range telemetry.EventCatalog() {
		for _, prop := range ev.Properties {
			catalogKeys[prop.Name] = true
		}
	}

	// Every key found in source must appear in the catalog.
	for key, files := range seenKeys {
		if !catalogKeys[key] {
			t.Errorf("telemetry.Enrich(%q) found in %v but %q is not in EventCatalog()", key, files, key)
		}
	}
}

// findRepoRoot walks upward from the working directory to find the repo root
// by looking for the .git directory.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (.git directory)")
		}
		dir = parent
	}
}
```

**Success criteria:**

- `cd cli && go test ./cmd/syllago/ -run TestGentelemetry -v` → pass — all six tests pass
- `cd cli && go test ./cmd/syllago/ -run TestGentelemetry_CatalogMatchesEnrichCalls -v` → pass — drift detection finds all 12 Enrich() keys and all are in the catalog
- `cd cli && go test ./cmd/syllago/ -run TestGentelemetry_PrivacyGuarantees -v` → pass — all 6 privacy keyword categories matched
- `cd cli && go test ./cmd/syllago/ -run TestGentelemetry_PropertiesComplete -v` → pass — all properties have valid types (string/int/bool), non-empty descriptions, non-nil examples

**Commit message:** `test: gentelemetry tests — structure, completeness, privacy, drift detection (Task 3)`

---

## Task 4 — Makefile Update

**Dependencies:** Task 2 must be complete (binary can execute `_gentelemetry`)

**What:** Add `_gentelemetry > telemetry.json` to the `gendocs` target in `cli/Makefile`. One line change.

**File to modify:** `/home/hhewett/.local/src/syllago/cli/Makefile`

**Current `gendocs` target:**

```makefile
gendocs: build
	./$(OUTPUT) _gendocs > commands.json
	./$(OUTPUT) _genproviders > providers.json
```

**New `gendocs` target:**

```makefile
gendocs: build
	./$(OUTPUT) _gendocs > commands.json
	./$(OUTPUT) _genproviders > providers.json
	./$(OUTPUT) _gentelemetry > telemetry.json
```

**Success criteria:**

- `cd cli && make gendocs` → pass — runs without error; `commands.json`, `providers.json`, and `telemetry.json` all exist in `cli/`
- `cd cli && python3 -m json.tool telemetry.json > /dev/null` → pass — `telemetry.json` is valid JSON
- `cd cli && grep -c '"events"' telemetry.json` → outputs `1` — events key present in output

**Commit message:** `build: add _gentelemetry to gendocs Makefile target (Task 4)`

---

## Task 5 — Release Workflow Update

**Dependencies:** Task 4 must be complete (Makefile change done; confirms the command works end-to-end)

**What:** Extend `.github/workflows/release.yml` with three changes:
1. Add `./syllago-gendocs _gentelemetry > telemetry.json` to the Generate step
2. Add `telemetry.json` to the `sha256sum` command
3. Add `telemetry.json` to both `gh release create` commands

**File to modify:** `/home/hhewett/.local/src/syllago/.github/workflows/release.yml`

**Change 1 — Generate step** (after `providers.json` line):

Old:
```yaml
      - name: Generate commands.json and providers.json
        working-directory: cli
        run: |
          LDFLAGS="-X main.version=${VERSION}"
          go build -ldflags "$LDFLAGS" -o syllago-gendocs ./cmd/syllago
          ./syllago-gendocs _gendocs > commands.json
          ./syllago-gendocs _genproviders > providers.json
          rm -f syllago-gendocs
```

New:
```yaml
      - name: Generate commands.json, providers.json, and telemetry.json
        working-directory: cli
        run: |
          LDFLAGS="-X main.version=${VERSION}"
          go build -ldflags "$LDFLAGS" -o syllago-gendocs ./cmd/syllago
          ./syllago-gendocs _gendocs > commands.json
          ./syllago-gendocs _genproviders > providers.json
          ./syllago-gendocs _gentelemetry > telemetry.json
          rm -f syllago-gendocs
```

**Change 2 — Checksums step** (add `telemetry.json`):

Old:
```yaml
          sha256sum syllago-linux-amd64 syllago-linux-arm64 \
            syllago-darwin-amd64 syllago-darwin-arm64 \
            syllago-windows-amd64.exe syllago-windows-arm64.exe \
            commands.json providers.json sbom.spdx.json \
            > checksums.txt
```

New:
```yaml
          sha256sum syllago-linux-amd64 syllago-linux-arm64 \
            syllago-darwin-amd64 syllago-darwin-arm64 \
            syllago-windows-amd64.exe syllago-windows-arm64.exe \
            commands.json providers.json telemetry.json sbom.spdx.json \
            > checksums.txt
```

**Change 3 — Both `gh release create` commands** (add `telemetry.json` after `providers.json`):

Old (both blocks):
```
            commands.json providers.json sbom.spdx.json \
```

New (both blocks):
```
            commands.json providers.json telemetry.json sbom.spdx.json \
```

**Success criteria:**

- `grep 'telemetry.json' .github/workflows/release.yml | wc -l` → outputs `4` — telemetry.json appears in generate, checksums, and both release blocks
- `grep '_gentelemetry' .github/workflows/release.yml` → pass — generate step includes the command
- `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))"` → pass — YAML is valid (or use `yq` if available)

**Commit message:** `ci: add telemetry.json to release workflow — generate, checksum, and release assets (Task 5)`

---

## Task 6 — Pre-push Hook Update

**Dependencies:** Task 4 must be complete (binary produces `telemetry.json` via `_gentelemetry`)

**What:** Extend `.git/hooks/pre-push` to also verify `telemetry.json` freshness. Follows the exact same pattern as the existing `commands.json` freshness check: build a temp binary, run `_gentelemetry`, strip volatile fields, diff against committed file, fail if stale.

**File to modify:** `/home/hhewett/.local/src/syllago/.git/hooks/pre-push`

**Add after the existing `commands.json` freshness block** (after `echo "commands.json up to date."`):

```sh
# --- telemetry.json freshness ---
echo "Checking telemetry.json freshness..."
TMPBIN2=$(mktemp)
if go build -o "$TMPBIN2" ./cmd/syllago 2>/dev/null; then
    "$TMPBIN2" _gentelemetry > telemetry.json.tmp 2>/dev/null
    rm -f "$TMPBIN2"

    # Strip volatile fields before comparing
    grep -v '"generatedAt"' telemetry.json | grep -v '"syllagoVersion"' > telemetry.json.stable
    grep -v '"generatedAt"' telemetry.json.tmp | grep -v '"syllagoVersion"' > telemetry.json.tmp.stable

    if ! diff -q telemetry.json.stable telemetry.json.tmp.stable >/dev/null 2>&1; then
        echo "" >&2
        echo "Error: telemetry.json is stale. Regenerate it:" >&2
        echo "  cd cli && make gendocs" >&2
        rm -f telemetry.json.tmp telemetry.json.stable telemetry.json.tmp.stable
        exit 1
    fi
    rm -f telemetry.json.tmp telemetry.json.stable telemetry.json.tmp.stable
    echo "telemetry.json up to date."
else
    rm -f "$TMPBIN2"
    echo "Warning: could not build for telemetry.json check, skipping" >&2
fi
```

Note: The existing hook already does `cd "$REPO_ROOT/cli" || exit 1` before the `commands.json` block. The new block runs in the same working directory — no additional `cd` needed.

**Success criteria:**

- `cd cli && make gendocs` then `git stash` then modifying `cli/internal/telemetry/catalog.go` to add a dummy event then running `.git/hooks/pre-push` manually → produces `Error: telemetry.json is stale` and exits 1
- With a fresh `make gendocs` and no catalog changes: `.git/hooks/pre-push` → exits 0 with `telemetry.json up to date.` printed
- `head -1 .git/hooks/pre-push` → `#!/bin/sh` — file is still executable shell

**Commit message:** `build: extend pre-push hook to check telemetry.json freshness (Task 6)`

---

## Task 7 — Rule File (`.claude/rules/telemetry-enrichment.md`)

**Dependencies:** none (independent; can be done at any point)

**What:** A rule file scoped to CLI command files. Reminds the developer to add `telemetry.Enrich()` calls when adding new commands or new flags, and to update `catalog.go` if introducing a new property key. Follows the ADR awareness system's advisory pattern.

**File to create:** `/home/hhewett/.local/src/syllago/.claude/rules/telemetry-enrichment.md`

**Complete file content:**

```markdown
# Telemetry Enrichment Rule

**Scope:** `cli/cmd/syllago/*_cmd.go`, `cli/cmd/syllago/*.go` files with `RunE` functions

---

## When adding or modifying a CLI command

1. **Add `telemetry.Enrich()` calls for relevant properties** in the `RunE` function, before it returns. Track:
   - Provider slugs (`provider`, `from`, `from_provider`, `to_provider`)
   - Content type string (`content_type`)
   - Counts (`content_count`, `item_count`, `action_count`, `registry_count`)
   - Boolean flags (`dry_run`)
   - Mode strings (`mode`, `source_filter`)

   Never enrich with: file contents, file paths, content names, registry URLs, usernames, or any PII.

2. **If the property key is new** (not already in `EventCatalog()` in `cli/internal/telemetry/catalog.go`):
   - Add a `PropertyDef` entry to the relevant event in `EventCatalog()`
   - The drift-detection test `TestGentelemetry_CatalogMatchesEnrichCalls` will fail CI if you forget this step

3. **Regenerate `telemetry.json`** after any catalog change:
   ```
   cd cli && make gendocs
   ```
   The pre-push hook blocks pushes with stale `telemetry.json`.

## Quick reference

| Property key     | Type   | When to use                                          |
|------------------|--------|------------------------------------------------------|
| `provider`       | string | Target provider slug (install, uninstall, apply)     |
| `from`           | string | Source provider slug for add/import commands         |
| `from_provider`  | string | Source provider for convert                          |
| `to_provider`    | string | Target provider for convert                          |
| `content_type`   | string | Content type filter or selected type                 |
| `content_count`  | int    | Number of items installed/added                      |
| `item_count`     | int    | Number of items in a list result                     |
| `action_count`   | int    | Number of actions in a loadout result                |
| `registry_count` | int    | Number of registries involved                        |
| `dry_run`        | bool   | Whether --dry-run was used                           |
| `mode`           | string | Operational mode (e.g. "try" for loadout)            |
| `source_filter`  | string | Source filter (library, shared, registry)            |

## Example

```go
func runMyCommand(cmd *cobra.Command, args []string) error {
    // ... command logic ...
    telemetry.Enrich("provider", providerSlug)
    telemetry.Enrich("content_type", typeStr)
    telemetry.Enrich("content_count", len(installed))
    return nil
}
```

`TrackCommand()` fires automatically from `PersistentPostRun` — you do not call it yourself.
```

**Success criteria:**

- `ls .claude/rules/telemetry-enrichment.md` → pass — file exists
- `grep -c 'telemetry.Enrich\|catalog.go\|make gendocs' .claude/rules/telemetry-enrichment.md` → outputs `3` — all three reminder triggers are present
- `grep 'content_type\|dry_run\|provider' .claude/rules/telemetry-enrichment.md | wc -l` → outputs >= 3 — quick reference table is populated

**Commit message:** `docs: telemetry-enrichment rule — reminds developers to enrich and update catalog (Task 7)`

---

## Full Execution Order

```
Task 7  — rule file (any time, no deps)
Task 1  — catalog.go
Task 2  — gentelemetry.go          (needs Task 1)
Task 3  — gentelemetry_test.go     (needs Task 2)
Task 4  — Makefile                 (needs Task 2)
Task 5  — release.yml              (needs Task 4)
Task 6  — pre-push hook            (needs Task 4)
```

Optimal parallelism: Task 7 + Task 1 can start simultaneously. Tasks 2–6 are a strict chain.

## Verification After All Tasks

```
cd cli && make build                             # binary rebuilds cleanly
cd cli && go test ./internal/telemetry/...       # catalog package tests pass
cd cli && go test ./cmd/syllago/ -run TestGentelemetry  # all 6 gentelemetry tests pass
cd cli && make gendocs                           # generates commands.json, providers.json, telemetry.json
python3 -m json.tool cli/telemetry.json > /dev/null  # telemetry.json is valid JSON
grep '"events"' cli/telemetry.json               # events key present
grep '"neverCollected"' cli/telemetry.json       # privacy section present
```

---

## Notes on the Drift Detection Approach

`TestGentelemetry_CatalogMatchesEnrichCalls` scans source files with `os.ReadDir` + `os.ReadFile` rather than importing or executing the commands being scanned. This is intentional:

- **Why not import:** The command files are in `package main` and use package-level state (cobra commands, flags). Importing them from a test would cause registration conflicts.
- **Why not subprocess:** Would require the binary to be pre-built, adding a fragile build dependency to the test.
- **Why regex scan:** The regex `telemetry\.Enrich\("([^"]+)"` is narrow enough to avoid false positives (Enrich is only called with string literals in this codebase). It's the same approach used in `TestGenproviders_HookEventCategoryCompleteness` which scans the `hookEventCategory` map against `converter.HookEvents`.

The test uses `findRepoRoot()` to locate `cmd/syllago/` regardless of where `go test` is invoked from, making it robust whether run as `go test ./cmd/syllago/` or `go test ./...` from `cli/`.
