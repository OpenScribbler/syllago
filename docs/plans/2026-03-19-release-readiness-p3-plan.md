# Implementation Plan: Release Readiness Phase 3 — Content + Onboarding

*Plan date: 2026-03-20*
*Design doc: docs/plans/2026-03-19-release-readiness-p3-design.md*

## Key Findings from Exploration

1. **`renderFirstRun()` already exists** in `app.go` (~line 2730) — shows getting-started guide when catalog is empty and no registries configured. The welcome card work is an enhancement of this, not a new component.
2. **`content/` is auto-discovered** — new skills/loadouts dropped into `content/skills/` and `content/loadouts/` are found by the catalog scanner. No scanner changes needed.
3. **`a.providers []provider.Provider`** is available in App, each with `Detected bool`. Trivial to check.
4. **`config paths` command already exists** — no CLI work needed for the escape hatch.
5. **`warningStyle`** exists in `styles.go` — reusable for the no-providers banner.

---

## Task 1: `syllago-quickstart` Skill

**What:** Create built-in getting-started skill using `/create-skill` workflow.

**Files to create:**
- `content/skills/syllago-quickstart/.syllago.yaml`
- `content/skills/syllago-quickstart/SKILL.md`

**Pattern to follow:** `content/skills/syllago-guide/` (`.syllago.yaml` + `SKILL.md`)

**`.syllago.yaml`:**
```yaml
name: syllago-quickstart
description: Getting started with syllago in your first 5 minutes
version: "1.0"
tags:
  - builtin
  - guide
  - syllago
```

**SKILL.md narrative — five steps:**
1. Discover what you have — `syllago add --from claude-code`
2. Browse your library — launch `syllago` TUI
3. Install to another provider — `syllago install skills/my-skill --to cursor`
4. Add a registry — `syllago registry add <git-url>`
5. Create a loadout — `syllago loadout create`

Include "Next steps" pointing at `syllago-guide` for full reference.

**Acceptance criteria:**
- `syllago list --type skills` shows `syllago-quickstart`
- `syllago inspect skills/syllago-quickstart` shows content
- TUI Skills view shows it alongside `syllago-guide`

---

## Task 2: `syllago-starter` Loadout

**What:** Built-in loadout bundling the four syllago meta-tools.

**Depends on:** Task 1 (quickstart must exist for loadout to reference it)

**Files to create:**
- `content/loadouts/claude-code/syllago-starter/.syllago.yaml`
- `content/loadouts/claude-code/syllago-starter/loadout.yaml`

**Pattern to follow:** `content/loadouts/claude-code/example-kitchen-sink-loadout/`

**loadout.yaml:**
```yaml
kind: loadout
version: 1
provider: claude-code
name: syllago-starter
description: >
  Built-in meta-tools for syllago: reference guide, quickstart,
  import workflow reference, and content authoring agent.

skills:
  - syllago-guide
  - syllago-quickstart
  - syllago-import
agents:
  - syllago-author
```

**Acceptance criteria:**
- `syllago list --type loadouts` shows `syllago-starter`
- `syllago loadout apply syllago-starter --dry-run` lists 4 items
- TUI Loadouts screen shows it

---

## Task 3: Context-Aware Welcome Card (TUI)

**What:** Replace static `renderFirstRun()` with provider-detection-aware version with three journey paths.

**File to modify:** `cli/internal/tui/app.go`

**New helper methods on App:**
- `anyProviderDetected() bool` — loops `a.providers`, returns true if any `p.Detected`
- `detectedProviderNames() []string` — returns names of detected providers

**Replace `renderFirstRun()` with journey branching:**

Priority order (first match wins):

| Priority | Condition | Journey | Shows |
|----------|-----------|---------|-------|
| 1 | `anyProviderDetected()` | A | Detected provider names + 'a' to discover + 'R' for registry |
| 2 | `len(registries) > 0` | B | Registry guidance (note: may rarely trigger since registries existing typically bypasses first-run entirely) |
| 3 | else | C | 'a' to add content + mention `syllago-starter` + '?' for help |

Note: `renderFirstRun()` is only called when library is empty. If providers are detected AND registries exist, Journey A wins (providers take priority).

**Tests to add in `app_test.go`:**
- `TestFirstRunJourneyA_ProvidersDetected` — assert detected provider name appears
- `TestFirstRunJourneyC_NoProviders` — assert `syllago-starter` mentioned
- Update existing `TestFirstRunScreenAppearsWhenEmpty` if assertions change

**Golden file impact:** `fullapp-*-empty*.golden` files will change.

---

## Task 4: No-Providers Warning Banner (TUI)

**What:** Yellow inline warning at top of content view when no providers detected.

**File to modify:** `cli/internal/tui/app.go`

**New method:**
```go
func (a App) noProvidersWarning() string {
    if a.anyProviderDetected() {
        return ""
    }
    return warningStyle.Render("! No AI coding tools detected. ...")
}
```

**Injection point:** In `View()`, prepend banner to `contentView` before `MaxHeight` constraining.

**Tests:**
- `TestNoProvidersWarningShown` — no detected providers → non-empty warning
- `TestNoProvidersWarningHiddenWhenDetected` — detected provider → empty string

**Golden file determinism:** Ensure test helpers set at least one `Detected: true` provider so banner doesn't pollute existing goldens.

---

## Task 5: No-Providers Warning (CLI)

**What:** Warning on `syllago install` and `syllago add` when target/source provider not detected.

**Files to modify:**
- `cli/cmd/syllago/install_cmd.go` — after resolver construction (~line 100)
- `cli/cmd/syllago/add_cmd.go` — after resolver construction (~line 111)

**Logic:** Call `DetectProvidersWithResolver(resolver)`, check if target/source provider is detected. If not, print warning to stderr (skip in JSON/quiet mode).

**Warning text:**
```
Warning: <provider> not detected at default locations.
If installed at a custom path, configure it:
  syllago config paths --provider <slug> --path /your/path
```

**Tests:** Stub `provider.AllProviders` with `Detected: false`, assert stderr contains warning.

---

## Task 6: Tests, Golden Files, and Integration

**What:** Final verification pass.

**Steps:**
1. Add catalog discovery test asserting `syllago-quickstart` and `syllago-starter` found with `IsBuiltin() == true`
2. Audit TUI test helpers — ensure providers have `Detected: true` for deterministic goldens
3. Regenerate golden files: `cd cli && go test ./internal/tui/ -update-golden`
4. Run full test suite: `make test`
5. Build and manual smoke test: `make build && syllago list --type skills`

---

## Task Sequence

```
Task 1 (quickstart skill) ──► Task 2 (starter loadout)
                                      │
Task 3 (welcome card TUI) ───────────┤
Task 4 (no-providers TUI) ───────────┤  (same file as T3, do sequentially)
Task 5 (no-providers CLI) ───────────┤  (independent)
                                      │
                                      ▼
                              Task 6 (tests + golden)
```

Tasks 1 and 3 can parallelize. Tasks 3 and 4 share `app.go` — do sequentially. Task 2 depends on Task 1.

---

## Edge Cases

1. **Loadout references quickstart before it exists** — Task 2 must follow Task 1
2. **Banner height at narrow terminals** — suppress below height threshold if needed
3. **Golden determinism** — test helpers must inject `Detected: true` provider
4. **Existing test assertions** — `TestFirstRunScreenAppearsWhenEmpty` may check for text that changes with new journeys; update accordingly
5. **`anyProviderDetected()` with nil providers** — returns false (correct — nil means nothing detected)
