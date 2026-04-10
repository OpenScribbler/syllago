# Implementation Plan: OpenScribbler/syllago-meta-registry

**Design doc:** `docs/plans/2026-03-26-syllago-meta-registry-design.md`
**Bead:** syllago-5wdcb

---

## Task 1: Scaffold the registry repo

**Run from `~/.local/src/`:**
```bash
syllago registry create --new syllago-meta-registry \
  --description "Official syllago meta-registry — skills, agents, loadouts, and content for syllago usage and management"
```

Then in the new `~/.local/src/syllago-meta-registry/`:
- Delete `skills/hello-world/` and `rules/claude-code/example-rule/`
- Edit `registry.yaml`: set version to `1.0.0`, add `maintainers: [OpenScribbler]`, add `visibility: public`
- Update `README.md` with description of the meta-registry purpose
- Update `CONTRIBUTING.md` to remove references to deleted examples

---

## Task 2: Copy and fix meta-content

Copy from `content/` in the syllago repo to the registry:

### Skills (4 items)
```bash
cp -r content/skills/syllago-guide       ../syllago-meta-registry/skills/
cp -r content/skills/syllago-import      ../syllago-meta-registry/skills/
cp -r content/skills/syllago-quickstart  ../syllago-meta-registry/skills/
cp -r content/skills/syllago-provider-audit ../syllago-meta-registry/skills/
```

### Agent (1 item)
```bash
cp -r content/agents/syllago-author ../syllago-meta-registry/agents/
```

### Loadouts (12 items)
```bash
for prov in amp claude-code cline codex copilot-cli cursor gemini-cli kiro opencode roo-code windsurf zed; do
  mkdir -p ../syllago-meta-registry/loadouts/$prov
  cp -r content/loadouts/$prov/syllago-starter ../syllago-meta-registry/loadouts/$prov/
done
```

### Fix syllago-guide typo
In `skills/syllago-guide/.syllago.yaml`, fix line 4:
```
description: Complete reference for using syllago to manage AI coding tool contentasdfasdfasdf
```
→
```
description: Complete reference for using syllago to manage AI coding tool content
```

### Create missing `.syllago.yaml` files

**`skills/syllago-provider-audit/.syllago.yaml`:**
```yaml
name: syllago-provider-audit
description: Structured research workflow for auditing AI coding tool providers
version: "1.0"
tags:
  - official
  - audit
  - syllago
```

**All 12 loadout `.syllago.yaml` files** (in each `loadouts/<provider>/syllago-starter/`):
```yaml
id: syllago-starter
name: syllago-starter
description: Official syllago meta-tools for getting started
tags:
  - official
  - starter
  - syllago
```

### Tag changes
In ALL existing `.syllago.yaml` files, replace `builtin` tag with `official`:
- `skills/syllago-guide/.syllago.yaml`
- `skills/syllago-import/.syllago.yaml`
- `skills/syllago-quickstart/.syllago.yaml`
- `agents/syllago-author/.syllago.yaml`

---

## Task 3: Create Registry Creator skill

**Create `skills/syllago-registry-creator/SKILL.md`** in the registry:

Frontmatter:
```yaml
---
name: syllago-registry-creator
description: Create and publish your own syllago content registry
---
```

Body: Skill instructions with two modes:
- **Quick mode** — Ask for name, description, content types to include. Run `syllago registry create --new <name> --description "<desc>"`. Print next steps (add remote, push).
- **Guided mode** — Educational walkthrough: what is a registry, content types explained (universal: skills/agents/MCP vs provider-specific: rules/hooks/commands), loadouts for bundling, organization advice, scaffold with commentary, next steps (push to GitHub, add to projects, share with team).

Mode selection prompt at start: "How would you like to create your registry?"
- "Quick — I know what I'm doing"
- "Guided — Walk me through it"

**Create `skills/syllago-registry-creator/.syllago.yaml`:**
```yaml
name: syllago-registry-creator
description: Create and publish your own syllago content registry
version: "1.0"
tags:
  - official
  - registry
  - syllago
```

---

## Task 4: Update starter loadouts

Add `syllago-registry-creator` to skills list in all 12 `loadout.yaml` files.

**For providers WITH agents** (claude-code, codex, copilot-cli, cursor, gemini-cli, kiro, opencode, roo-code):
```yaml
kind: loadout
version: 1
provider: <provider>
name: syllago-starter
description: >
  Official syllago meta-tools: reference guide, quickstart,
  import workflow, registry creator, and content authoring agent.

skills:
  - syllago-guide
  - syllago-quickstart
  - syllago-import
  - syllago-registry-creator
agents:
  - syllago-author
```

**For providers WITHOUT agents** (amp, cline, windsurf, zed):
```yaml
kind: loadout
version: 1
provider: <provider>
name: syllago-starter
description: >
  Official syllago meta-tools: reference guide, quickstart,
  import workflow, and registry creator.

skills:
  - syllago-guide
  - syllago-quickstart
  - syllago-import
  - syllago-registry-creator
```

Also update descriptions: "Built-in meta-tools" → "Official syllago meta-tools".

---

## Task 5: Wire up first-boot default registry

### 5a. `cli/internal/registry/registry.go`

Add exported constant before `KnownAliases`:
```go
// OfficialRegistryURL is the default syllago meta-registry.
const OfficialRegistryURL = "https://github.com/OpenScribbler/syllago-meta-registry.git"
```

Update `KnownAliases` comment and value:
```go
// KnownAliases maps short names to full git URLs.
var KnownAliases = map[string]string{
	"syllago": OfficialRegistryURL,
}
```

### 5b. `cli/cmd/syllago/init_wizard.go`

**Constants** (line 20-24) — add `registryOptOfficial`, shift others:
```go
const (
	registryOptOfficial = 0
	registryOptAdd      = 1
	registryOptCreate   = 2
	registryOptSkip     = 3
)
```

**Add import** for `registry` package:
```go
"github.com/OpenScribbler/syllago/cli/internal/registry"
```

**Update()** — `stepRegistry` case:

Cursor bounds (line 121):
```go
if w.registryCursor < 3 {
```

Enter handler (line 124-147) — add new case before existing ones:
```go
case registryOptOfficial:
	w.registryAction = "add"
	w.registryURL = registry.OfficialRegistryURL
	w.done = true
```

Existing `registryOptAdd` and `registryOptCreate` cases stay the same (just renumbered constants).

**View()** — `stepRegistry` case (line 222):
```go
options := []string{
	"Add the official syllago meta-registry (Recommended)",
	"Add a custom registry URL",
	"Create a new registry",
	"Skip for now",
}
```

### 5c. `cli/internal/registry/registry_test.go`

Replace `TestExpandAlias_KnownAliasTableIsEmpty` (lines 34-38):
```go
func TestExpandAlias_OfficialAlias(t *testing.T) {
	url, expanded := ExpandAlias("syllago")
	if !expanded {
		t.Fatal("expected syllago alias to expand")
	}
	if url != OfficialRegistryURL {
		t.Errorf("got %q, want %q", url, OfficialRegistryURL)
	}
}
```

### 5d. `cli/cmd/syllago/init_test.go`

**`TestInitWizard_EnterMarksDone`** (line 238) — Skip moved from index 2 to 3:
```go
// Move cursor to "Skip for now" (index 3) and confirm
w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
```

**`TestInitWizard_SkipRegistryMarksDone`** (line 349) — same fix:
```go
// Move cursor to "Skip for now" (index 3) and select it
w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
w, _ = w.Update(tea.KeyMsg{Type: tea.KeyDown})
w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})
```

**Add new test — `TestInitWizard_OfficialRegistryOption`:**
```go
func TestInitWizard_OfficialRegistryOption(t *testing.T) {
	detected := []provider.Provider{{Name: "Claude Code", Slug: "claude-code", Detected: true}}
	w := newInitWizard(detected, detected)

	// Advance past provider step
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Option 0 is "Add the official syllago meta-registry" — just press Enter
	w, _ = w.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !w.done {
		t.Error("selecting official registry should mark wizard as done")
	}
	if w.registryAction != "add" {
		t.Errorf("registryAction should be 'add', got %q", w.registryAction)
	}
	if w.registryURL != registry.OfficialRegistryURL {
		t.Errorf("registryURL should be %q, got %q", registry.OfficialRegistryURL, w.registryURL)
	}
}
```

### Verify: `cd cli && make test`

---

## Task 6: Migrate converter test fixtures

### 6a. Create testdata directory and copy fixtures:
```bash
mkdir -p cli/internal/converter/testdata/skills/example-kitchen-sink-skill
mkdir -p cli/internal/converter/testdata/agents/example-kitchen-sink-agent
mkdir -p cli/internal/converter/testdata/rules/cursor/example-kitchen-sink-rules
mkdir -p cli/internal/converter/testdata/commands/claude-code/example-kitchen-sink-commands

cp content/skills/example-kitchen-sink-skill/SKILL.md \
   cli/internal/converter/testdata/skills/example-kitchen-sink-skill/
cp content/agents/example-kitchen-sink-agent/AGENT.md \
   cli/internal/converter/testdata/agents/example-kitchen-sink-agent/
cp content/rules/cursor/example-kitchen-sink-rules/rule.mdc \
   cli/internal/converter/testdata/rules/cursor/example-kitchen-sink-rules/
cp content/commands/claude-code/example-kitchen-sink-commands/command.md \
   cli/internal/converter/testdata/commands/claude-code/example-kitchen-sink-commands/
```

### 6b. Update `kitchen_sink_coverage_test.go`

Replace `repoRoot()` helper with `testdataDir()`:
```go
func testdataDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "testdata")
}
```

Update all 4 test functions to use `testdataDir(t)` instead of `repoRoot(t)`:
- Line 48: `filepath.Join(root, "content", "skills", ...)` → `filepath.Join(td, "skills", ...)`
- Line 70: `filepath.Join(root, "content", "agents", ...)` → `filepath.Join(td, "agents", ...)`
- Line 90: `filepath.Join(root, "content", "rules", ...)` → `filepath.Join(td, "rules", ...)`
- Line 110: `filepath.Join(root, "content", "commands", ...)` → `filepath.Join(td, "commands", ...)`

### 6c. Update `kitchen_sink_roundtrip_test.go`

Same change — use `testdataDir(t)` instead of `repoRoot(t)`:
- Line 34: `filepath.Join(root, "content", "skills", ...)` → `filepath.Join(td, "skills", ...)`
- Line 171: `filepath.Join(root, "content", "agents", ...)` → `filepath.Join(td, "agents", ...)`

### Verify: `cd cli && go test ./internal/converter/... -v`

---

## Task 7: Clean up `content/` in main repo

### Delete meta-content (moved to registry):
```bash
rm -rf content/skills/syllago-guide
rm -rf content/skills/syllago-import
rm -rf content/skills/syllago-quickstart
rm -rf content/skills/syllago-provider-audit
rm -rf content/agents/syllago-author
rm -rf content/loadouts
```

### Delete example/demo content:
```bash
rm -rf content/skills/example-*
rm -rf content/agents/example-*
rm -rf content/mcp/example-*
rm -rf content/rules/*/example-*
rm -rf content/hooks/claude-code/*
rm -rf content/commands/*/example-*
```

### Delete dropped content types:
```bash
rm -rf content/prompts
rm -rf content/apps
```

### Delete empty type directories:
```bash
rmdir content/skills content/agents content/mcp content/rules/* content/rules \
      content/hooks/* content/hooks content/commands/* content/commands 2>/dev/null || true
```

### What remains:
- `content/.syllago/` (project config)
- `content/local/` (user-created content, gitignored)

### Verify: `cd cli && make test` (full test suite)

---

## Task 8: Build and verify

```bash
cd cli && make fmt && make build && make test
```

Manual verification:
1. `syllago init` — shows 4 registry options
2. Option 0 adds official meta-registry URL to config
3. `syllago registry add syllago` — alias works

---

## Task 9: Commit, push registry, push main

### Registry repo:
```bash
cd ~/.local/src/syllago-meta-registry
git add -A && git commit -m "Initial registry: 5 skills, 1 agent, 12 loadouts"
```
Create `OpenScribbler/syllago-meta-registry` on GitHub (public), add remote, push.

### Main repo:
```bash
cd ~/.local/src/syllago
# Stage all changes (init wizard, tests, converter testdata, content cleanup)
git add <files>
git commit -m "feat: wire up official syllago-meta-registry as default on first boot

- Add OfficialRegistryURL constant and 'syllago' alias in KnownAliases
- Add 4th init wizard option: 'Add the official syllago meta-registry (Recommended)'
- Migrate converter kitchen-sink fixtures to testdata/ (no more content/ dependency)
- Remove meta-content and example content from content/ (now in registry)
- Delete prompts/ and apps/ content type directories (no longer supported)"
```

### Final verification:
```bash
syllago registry add https://github.com/OpenScribbler/syllago-meta-registry.git
syllago registry items
# Expected: 5 skills + 1 agent + 12 loadouts = 18 items
syllago registry remove OpenScribbler/syllago-meta-registry
```

---

## Execution Order

| Task | Depends On | Parallel? |
|------|-----------|-----------|
| 1. Scaffold registry | — | — |
| 2. Copy meta-content | 1 | — |
| 3. Registry Creator skill | 2 | — |
| 4. Update loadouts | 3 | — |
| 5. Wire up init wizard | — | Yes, parallel with 2-4 |
| 6. Migrate converter fixtures | — | Yes, parallel with 2-4 |
| 7. Clean up content/ | 2, 4, 6 | — |
| 8. Build and verify | 5, 7 | — |
| 9. Commit and push | 8 | — |

Tasks 5 and 6 are independent of registry work and can run in parallel with tasks 2-4.

---

## Notes from Parity Check

**No code action needed:**
- `installBuiltins()` in `init.go` (lines 219-285) becomes a harmless no-op when all `builtin`-tagged content is removed. No code change required — it gracefully handles zero items.

**Follow-up bead (post-implementation):**
- Existing users who already ran `syllago init` won't have the official meta-registry. They need to run `syllago registry add syllago` manually. Consider: release notes mention, and/or a `syllago doctor` hint when the official registry is not configured.
