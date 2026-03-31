# Syllago Benchmark Installer - Design Document

**Goal:** Use syllago to install the 17 agentskillimplementation.com benchmark skills across 12 AI coding agents, dogfooding syllago's cross-provider content distribution pipeline.

**Decision Date:** 2026-03-31

---

## Problem Statement

We need to install 17 benchmark skills (from `agent-ecosystem/agent-skill-implementation`) to 12 AI coding agents to run empirical behavioral checks. Today this requires manually copying SKILL.md directories to each agent's skill location — 204 copy operations (17 x 12). Syllago already supports skills as a universal content type and all 12 agents as providers. The gap is: no single command installs to all providers, and the benchmark repo isn't set up as a syllago registry.

This is a perfect dogfooding opportunity: if syllago can't make this easy, it's failing at its core value proposition.

## Proposed Solution

Four layers, each building on the last:

### Layer 1: `--to-all` flag on `syllago install`

Add a `--to-all` flag that installs content to every detected provider in one command.

**Current:**
```bash
# Must repeat for each of 12 providers
syllago install --type skills --to claude-code
syllago install --type skills --to cursor
syllago install --type skills --to windsurf
# ... 9 more times
```

**Proposed:**
```bash
syllago install --type skills --to-all
```

**Implementation:** In `cli/cmd/syllago/install_cmd.go`, when `--to-all` is set:
1. Call `provider.DetectProvidersWithResolver(cfg)` to get installed agents
2. Loop over results, call `installer.Install()` for each
3. Report per-provider results: success count, skip count (already installed), failures
4. If `--type` is set, filter to that content type. If a specific item name is given, install just that item to all providers.

**Edge cases:**
- Provider not detected (not installed) → skip with message
- Content type not supported by provider → skip silently (e.g., hooks format differs)
- Install failure on one provider → continue with others, report at end
- `--to-all` + `--to <provider>` → error, mutually exclusive

**Scope:** ~30 lines of Go in the install command + flag registration. No new packages.

### Layer 2: Benchmark repo as a syllago registry

Register `agent-ecosystem/agent-skill-implementation` as a syllago registry so the 17 benchmark skills can be imported via syllago's standard add flow.

**Steps:**
```bash
# One-time setup
syllago registry add agent-ecosystem/agent-skill-implementation

# Import all 17 benchmark skills to library
syllago add skills --all --from agent-skill-implementation

# Install to all agents
syllago install --type skills --to-all
```

**What this tests:**
- `syllago registry add` — git clone + index
- `syllago add` — discovery + canonicalization of SKILL.md files
- Frontmatter parsing for all 17 skills (name, description, metadata fields)
- Content types that are universal (skills) distribute without conversion
- `--to-all` (Layer 1) for the actual installation

**Registry structure concern:** The benchmark repo's skills are in `benchmark-skills/` not the standard `.agents/skills/` path. Syllago's scanner may need to handle this — either via a registry config that specifies the content root, or by the scanner being flexible enough to find SKILL.md files in non-standard locations. The scanner already does recursive SKILL.md discovery, so this may just work.

### Layer 3: Workflow doc

A step-by-step guide in `docs/research/skill-behavior-checks/` that ties the benchmark plan to syllago:

1. Prerequisites (syllago installed, Google AI Studio key for BYOK agents, agent accounts)
2. Registry setup (`syllago registry add`)
3. Import benchmark skills (`syllago add`)
4. Verify library contents (`syllago list --type skills`)
5. Install to all agents (`syllago install --type skills --to-all`)
6. Per-agent verification (confirm skills appear in each agent's skill list)
7. Run the 28 checks (reference the empirical benchmark plan)

### Layer 4: Multi-provider loadout (roadmap bead only)

Create a bead for future work and add to roadmap files. The feature would allow:

```yaml
kind: loadout
version: 2
name: "benchmark-skills"
description: "Agent Skills benchmark suite for all providers"
providers:
  - claude-code
  - cursor
  - windsurf
  - gemini-cli
  # ... all detected, or "all"
skills:
  - probe-loading
  - probe-linked-resources
  # ... all 17
```

This is deferred because `--to-all` covers the immediate need. Multi-provider loadouts are the right long-term solution for "distribute this bundle to my whole tool fleet."

## Architecture

### Data Flow

```
benchmark repo (GitHub)
    ↓ syllago registry add
~/.syllago/registries/agent-skill-implementation/  (git clone)
    ↓ syllago add skills --all --from agent-skill-implementation
~/.syllago/content/skills/{17 benchmark skills}/   (canonical library)
    ↓ syllago install --type skills --to-all
~/.claude/.agents/skills/probe-loading/            (symlink)
~/.cursor/.agents/skills/probe-loading/            (symlink)
~/.windsurf/.agents/skills/probe-loading/          (symlink)
~/.config/gemini-cli/skills/probe-loading/         (symlink)
... (12 providers total)
```

### Files Modified

| File | Change |
|------|--------|
| `cli/cmd/syllago/install_cmd.go` | Add `--to-all` flag, detection loop, per-provider reporting |
| `cli/internal/installer/installer.go` | Possibly: `InstallToAll()` helper that wraps the loop |
| `docs/research/skill-behavior-checks/workflow-syllago-install.md` | New: workflow doc |
| `README.md` | Roadmap update: multi-provider loadout |

## Key Decisions

| Decision | Choice | Reasoning |
|----------|--------|-----------|
| `--to-all` vs `--to all` | `--to-all` (boolean flag) | `--to` takes a string value; overloading "all" as a magic string is fragile. Separate flag is unambiguous. |
| Symlink vs copy for benchmark | Symlink (default) | Benchmark skills are read-only test fixtures. Symlinks save disk and make updates instant if the registry is re-synced. |
| Registry vs local add | Registry | Tests the full pipeline. Local `--from shared` would skip registry indexing, which we want to dogfood. |
| Multi-provider loadout | Deferred (roadmap bead) | `--to-all` solves the immediate need. Loadout v2 is a bigger design with its own trade-offs (provider-specific overrides, partial install, rollback). |

## Success Criteria

1. `syllago registry add agent-ecosystem/agent-skill-implementation` succeeds
2. `syllago add skills --all --from agent-skill-implementation` imports all 17 benchmark skills
3. `syllago install --type skills --to-all` installs to every detected provider
4. Each agent can discover and list the benchmark skills (verified manually per agent)
5. The canary phrase test works: open Claude Code, ask "Do you know CARDINAL-ZEBRA-7742?" — the skill is discoverable

## Open Questions

1. **Scanner path flexibility** — Will the scanner find SKILL.md files under `benchmark-skills/` in the registry? Or does it only scan `.agents/skills/`? Need to check `scanner.go` scan paths for registries.
2. **Project-level vs user-level install** — The benchmark plan says to install at project level (in the test project directory). `--to-all` currently implies user-level install. Should we support `--to-all --scope project`?
3. **Idempotency** — If skills are already installed, does `--to-all` skip or overwrite? Current behavior is skip-if-exists for symlinks. This is fine.

---

## Next Steps

Ready for implementation planning with `/develop` plan stage.
