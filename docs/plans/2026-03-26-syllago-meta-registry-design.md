# Plan: Create OpenScribbler/syllago-meta-registry

**Bead:** syllago-5wdcb

## Context

Syllago's own meta-content (skills that teach AI tools how to use syllago, starter loadouts, authoring agent) currently lives in `content/` inside the main repo. This creates a chicken-and-egg problem: the content management tool bundles its own content as flat files instead of using its own registry system.

This work extracts that meta-content into a standalone public registry at `OpenScribbler/syllago-meta-registry`, wires it up as the default registry on first boot, adds a Registry Creator skill, and cleans up the main repo's `content/` directory.

## Decisions Made

- **First boot:** Prompt during init with default-yes ("Add the official syllago meta-registry? [Y/n]")
- **Scaffolding:** Use `syllago registry create --new` (dogfood)
- **Demo content:** Delete all fake example content from `content/` — the registry serves as the real example
- **Prompts/Apps:** Delete these directories — content types no longer supported
- **Tags:** Replace `builtin` with `official` in migrated content
- **Registry Creator skill:** Two modes — Quick (power users) and Guided (first-timers with educational walkthrough)

## Audit Findings (Sanity Check)

Findings from plan audit against codebase — addressed inline in each phase:

| # | Severity | Finding | Resolution |
|---|----------|---------|------------|
| 1 | BLOCKER | Converter tests (`kitchen_sink_coverage_test.go`, `kitchen_sink_roundtrip_test.go`) read `content/example-kitchen-sink-*` via hardcoded paths — Phase 6 would break 8+ tests | Move test fixtures to `cli/internal/converter/testdata/` before deleting |
| 2 | BLOCKER | Init wizard tests (`TestInitWizard_EnterMarksDone`, `TestInitWizard_SkipRegistryMarksDone`) navigate to "Skip" with 2× Down; adding option 0 shifts Skip to index 3 | Update tests to press Down 3× for Skip |
| 3 | FIXED | Item count wrong: 5 skills + 1 agent + 12 loadouts = 18, not 19 | Corrected in verification section |
| 4 | FIXED | Plan claimed claude-code loadout has `.syllago.yaml` — none of the 12 do | All 12 need `.syllago.yaml` created |
| 5 | WARNING | No migration path for existing `syllago init` users | Document in release notes; consider `syllago doctor` hint |
| 6 | WARNING | Loadout descriptions vary by provider (claude-code has agent, others don't) | Per-provider description updates in Phase 4 |

---

## Phase 1: Scaffold Registry Repo

**What:** Create `syllago-meta-registry` using syllago's own tooling.

**Steps:**
1. From `~/.local/src/`, run `syllago registry create --new syllago-meta-registry --description "Official syllago meta-registry — skills, agents, loadouts, and content for syllago usage and management"`
2. Delete generated example content (`skills/hello-world/`, `rules/claude-code/example-rule/`)
3. Update `registry.yaml` with version 1.0.0, maintainers, visibility

**Result:** Empty registry structure ready to populate.

---

## Phase 2: Populate with Meta-Content

**What:** Copy existing syllago meta-content from `content/` into the registry.

**Content to copy:**

| Item | Type | Has .syllago.yaml? |
|------|------|--------------------|
| `syllago-guide` | skill | YES (fix `asdfasdfasdf` typo in description) |
| `syllago-import` | skill | YES |
| `syllago-quickstart` | skill | YES |
| `syllago-provider-audit` | skill | NO — create |
| `syllago-author` | agent | YES |
| 12× `syllago-starter` | loadout (amp, claude-code, cline, codex, copilot-cli, cursor, gemini-cli, kiro, opencode, roo-code, windsurf, zed) | None have `.syllago.yaml` — create for all 12 |

**Tag changes:** In all `.syllago.yaml` files, replace `builtin` tag with `official`.

**Missing `.syllago.yaml` to create:**

`skills/syllago-provider-audit/.syllago.yaml`:
```yaml
name: syllago-provider-audit
description: Structured research workflow for auditing AI coding tool providers
version: "1.0"
tags:
  - official
  - audit
  - syllago
```

For all 12 loadouts (same pattern, per provider):
```yaml
id: syllago-starter
name: syllago-starter
description: Official syllago meta-tools for getting started
tags:
  - official
  - starter
  - syllago
```

---

## Phase 3: Create Registry Creator Skill

**What:** New skill that helps users create their own registries with adaptive depth.

**Files:**
- `skills/syllago-registry-creator/SKILL.md`
- `skills/syllago-registry-creator/.syllago.yaml`

**Skill behavior — mode selection:**
- On invocation, ask: "How would you like to create your registry?"
  - **Quick** — "I know what I'm doing" → name, desc, content types, scaffold, done
  - **Guided** — "First time, walk me through it" → educational walkthrough explaining registries, content types (universal vs provider-specific), organization strategy, scaffold with commentary, next steps (push to GitHub, share with team)

**`.syllago.yaml`:**
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

## Phase 4: Update Starter Loadouts

**What:** Add `syllago-registry-creator` to all 12 starter loadout skill lists.

**Change per loadout.yaml:**
```yaml
skills:
  - syllago-guide
  - syllago-quickstart
  - syllago-import
  - syllago-registry-creator    # NEW
```

Also update the `description` field to mention registry creation.

---

## Phase 5: Wire Up First-Boot Default Registry (parallel with 2-4)

**What:** Modify `syllago init` wizard to offer the official meta-registry as the default option.

### Files to modify:

**`cli/internal/registry/registry.go`**
- Add constant: `OfficialRegistryURL = "https://github.com/OpenScribbler/syllago-meta-registry.git"`
- Populate `KnownAliases`: `"syllago" → OfficialRegistryURL`

**`cli/cmd/syllago/init_wizard.go`**
- Add `registryOptOfficial = 0`, shift existing constants up by 1
- New options in View():
  ```
  [0] Add the official syllago meta-registry (Recommended)
  [1] Add a custom registry URL
  [2] Create a new registry
  [3] Skip for now
  ```
- Handle `registryOptOfficial` in Update(): set `registryAction = "add"`, `registryURL = registry.OfficialRegistryURL`, `done = true`
- Update cursor bounds (`w.registryCursor < 3` → `< 4`)

**`cli/internal/registry/registry_test.go`**
- Replace `TestExpandAlias_KnownAliasTableIsEmpty` with `TestExpandAlias_OfficialAlias` that verifies the `syllago` alias expands correctly

**`cli/cmd/syllago/init_test.go`** (BLOCKER fix)
- `TestInitWizard_SkipRegistryMarksDone` (line ~349): currently presses Down 2× to reach Skip — update to 3× Down
- `TestInitWizard_EnterMarksDone` (line ~238): same fix — update Down presses to account for new option 0

**New test in `cmd/syllago/` or init_wizard_test.go:**
- Verify selecting option 0 sets correct registryURL and registryAction

---

## Phase 6: Clean Up `content/` in Main Repo

**What:** Remove all migrated meta-content and dead example content.

### Step 1: Migrate converter test fixtures (BLOCKER fix)

Before deleting example content, move the kitchen-sink fixtures used by converter tests:

**Files to move to `cli/internal/converter/testdata/`:**
- `content/skills/example-kitchen-sink-skill/SKILL.md`
- `content/agents/example-kitchen-sink-agent/AGENT.md`
- `content/rules/cursor/example-kitchen-sink-rules/rule.mdc`
- `content/commands/claude-code/example-kitchen-sink-commands/command.md`

**Tests to update:**
- `cli/internal/converter/kitchen_sink_coverage_test.go` — update paths from `content/` to `testdata/`
- `cli/internal/converter/kitchen_sink_roundtrip_test.go` — update paths from `content/` to `testdata/`

Run `go test ./internal/converter/...` to verify after migration.

### Step 2: Delete content

**Delete entirely:**
- `content/skills/syllago-guide/`
- `content/skills/syllago-import/`
- `content/skills/syllago-quickstart/`
- `content/skills/syllago-provider-audit/`
- `content/agents/syllago-author/`
- `content/loadouts/` (all 12 provider starter loadouts)
- `content/prompts/` (content type dropped)
- `content/apps/` (content type dropped)
- All remaining `example-*` content (skills, agents, MCP, rules, hooks, commands)
- Empty type directories (`content/skills/`, `content/agents/`, `content/mcp/`, `content/rules/`, `content/hooks/`, `content/commands/`)

**What remains in `content/`:**
- `content/.syllago/` (project config)
- `content/local/` (user-created content, not tracked in git)

**`installBuiltins()` in init.go:** Becomes a no-op (no `builtin`-tagged items remain). Leave in place — harmless and handles edge cases where someone has local builtin content.

---

## Phase 7: Push to GitHub

**Steps:**
1. Create `OpenScribbler/syllago-meta-registry` on GitHub (public)
2. Add remote and push from local scaffold
3. Verify: `syllago registry add https://github.com/OpenScribbler/syllago-meta-registry.git` works from a test project
4. Verify: `syllago registry items` shows all expected content

---

## Task Dependencies

```
Phase 1 (scaffold) → Phase 2 (populate) → Phase 3 (creator skill) → Phase 4 (update loadouts)
                                                                              ↓
Phase 5 (init wizard) ──────── can run in parallel with 2-4 ────────→ Phase 6 (cleanup)
                                                                              ↓
                                                                      Phase 7 (push)
```

---

## Verification

1. `make build && make test` — all CLI tests pass after Phase 5 changes
2. `syllago init` — shows 4 registry options, option 0 auto-adds official meta-registry
3. `syllago registry add syllago` — alias expands and clones successfully
4. `syllago registry items` — shows 5 skills + 1 agent + 12 loadouts (18 items total)
5. `syllago registry remove OpenScribbler/syllago-meta-registry` — clean removal
6. Content in main repo `content/` is minimal (just config + local)

## Key Files

| File | Change |
|------|--------|
| `cli/internal/registry/registry.go` | Add `OfficialRegistryURL`, populate `KnownAliases` |
| `cli/cmd/syllago/init_wizard.go` | Add 4th registry option (official, recommended) |
| `cli/cmd/syllago/init_test.go` | Fix wizard navigation tests (Down 2× → 3× for Skip) |
| `cli/internal/registry/registry_test.go` | Update alias test |
| `cli/internal/converter/kitchen_sink_coverage_test.go` | Move fixture paths to `testdata/` |
| `cli/internal/converter/kitchen_sink_roundtrip_test.go` | Move fixture paths to `testdata/` |
| `content/` | Delete migrated + example content |
