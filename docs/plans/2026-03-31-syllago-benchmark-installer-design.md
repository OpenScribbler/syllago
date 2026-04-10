# Syllago Benchmark Installer - Design Document

**Goal:** Use syllago to install the 17 agentskillimplementation.com benchmark skills across 8 AI coding agents (the ones syllago currently supports with user-scope skill install), dogfooding syllago's cross-provider content distribution pipeline.

**Decision Date:** 2026-03-31

---

## Problem Statement

We need to install 17 benchmark skills (from `agent-ecosystem/agent-skill-implementation`) to AI coding agents to run empirical behavioral checks. Syllago supports 12 providers, of which 10 overlap with the benchmark's 12 agents. Of those 10, 2 are excluded for technical reasons: Cline (no skills support — `SupportsType` returns false) and Kiro (project-scope only — `InstallDir` returns `ProjectScopeSentinel`). Two more benchmark agents have no syllago provider at all: Junie CLI and GitHub Copilot VS Code extension (syllago has `copilot-cli` for the CLI tool, not the extension). Zed is supported by syllago but not in the benchmark.

That gives us **8 agents** x 17 skills = 136 install operations that syllago should handle. Today this requires manually copying SKILL.md directories to each agent's skill location. Syllago already supports skills as a universal content type and all 8 target agents as providers. The gap is: no single command installs to all providers, and the benchmark repo isn't set up as a syllago registry.

This is a perfect dogfooding opportunity: if syllago can't make this easy, it's failing at its core value proposition.

### Agent Coverage

| Agent | Syllago Slug | Installed? | In Benchmark? |
|---|---|---|---|
| Claude Code | `claude-code` | Yes | Yes |
| Gemini CLI | `gemini-cli` | Yes | Yes |
| Cursor | `cursor` | Yes | Yes |
| Amp | `amp` | Yes | Yes |
| Windsurf | `windsurf` | No | Yes |
| Roo Code | `roo-code` | No | Yes |
| OpenCode | `opencode` | No | Yes |
| Codex | `codex` | No | Yes |

**Excluded:**
- Cline — syllago's Cline provider does not support skills (`SupportsType` returns false for `Skills`; Cline only supports rules, hooks, MCP)
- Kiro — returns `ProjectScopeSentinel` for skills; installer refuses user-scope install. Kiro maps skills to project-level steering files in `.kiro/steering/`, which is a different concept than the benchmark expects
- Junie CLI — no syllago provider
- GitHub Copilot (VS Code extension) — syllago has `copilot-cli`, not the extension
- Zed — syllago supports it but it's not in the benchmark

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

**Scope:** ~50 lines of Go. Requires: (1) removing `MarkFlagRequired("to")` and adding runtime mutual-exclusion validation in `RunE`, (2) extracting a small `installToProvider` helper from the current interleaved install logic, (3) the detection loop + per-provider reporting. No new packages.

**Note on WSL:** Providers with Windows-side config dirs (e.g., Cursor at `/mnt/c/Users/.../`) will automatically use copy instead of symlink — `IsWindowsMount()` in `symlink.go` handles this transparently. The "symlink default" in the design applies to Linux-native providers only.

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
- Frontmatter parsing for all 17 skills (name, description, metadata, allowed-tools, compatibility, etc.)
- **Skill conversion pipeline** — skills are NOT just copied verbatim. Each provider gets a different SKILL.md with provider-appropriate frontmatter. Claude Code keeps the full superset (allowed-tools, context, agent, model, hooks, etc.). Gemini/Windsurf/Amp get only name+description. Cursor/Copilot/Kiro/OpenCode get intermediate subsets. Unsupported fields are embedded as prose behavioral notes. Tool names are translated between canonical and provider-native names via `TranslateTools()`. This is real hub-and-spoke conversion, not passthrough.
- `--to-all` (Layer 1) for the actual installation

**Converter coverage per provider:**

| Provider | Frontmatter Fields Kept | Unsupported Fields Treatment |
|---|---|---|
| Claude Code | Full superset (13+ fields) | N/A — canonical source |
| Cursor | name, description, license, compatibility, metadata, disable-model-invocation | Prose notes + tool name translation + hook warnings |
| Copilot CLI | name, description, license, argument-hint, user-invocable, disable-model-invocation | Prose notes + hook warnings |
| Kiro | name, description, license, compatibility, metadata | Prose notes + hook warnings |
| OpenCode | name, description, license, compatibility, metadata | Prose notes |
| Gemini CLI | name, description | Prose notes + tool name translation + hook warnings |
| Windsurf | name, description | Prose notes + hook warnings |
| Amp | name, description | Prose notes |
| Roo Code | name, description | Prose notes |
| Codex | Full superset (falls through to Claude Code renderer — no dedicated Codex case) | N/A — receives full CC frontmatter |

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
~/.claude/skills/probe-loading/                    (symlink → claude-code)
~/.cursor/skills/probe-loading/                    (symlink → cursor)
~/.codeium/windsurf/skills/probe-loading/          (symlink → windsurf)
~/.gemini/skills/probe-loading/                    (symlink → gemini-cli)
~/.config/agents/skills/probe-loading/             (symlink → amp; shared cross-provider dir)
~/.roo/skills/probe-loading/                       (symlink → roo-code)
~/.config/opencode/skills/probe-loading/           (symlink → opencode)
~/.agents/skills/probe-loading/                    (symlink → codex; shared cross-provider dir)
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

## Model Selection Per Agent

Most of the 28 checks test **platform behavior** (does the harness load this file?), not model intelligence. The canary phrase approach ("do you know CARDINAL-ZEBRA-7742?") works with any model that can read its context and answer yes/no — Haiku-class models are fine.

### Model-Sensitive Checks (6 of 28)

These checks depend on the model's ability to follow prose instructions, chain activations, or handle edge cases. Results may differ between cheap and capable models:

| Check | Why Model Matters |
|---|---|
| `cross-skill-invocation` | Model must follow prose instruction to activate another skill |
| `invocation-depth-limit` | Model must chain A→B→C |
| `invocation-language-sensitivity` | Model interprets non-English instructions |
| `circular-invocation-handling` | Model must recognize or fail to recognize loops |
| `informal-dependency-resolution` | Model follows prose dependency |
| `missing-dependency-behavior` | Model's failure mode when skill doesn't exist |

**Strategy:** Run all 28 checks with the cheapest available model first (platform behavior baseline). Then re-run only the 6 model-sensitive checks with a capable model to capture the delta. Record both results per the contribution template's "platform-level vs model-level" distinction.

### Per-Agent Model Configuration

| Agent | Installed? | Cheapest Model | Cost | Config |
|---|---|---|---|---|
| **Claude Code** | Yes | Haiku 4.5 | $0 (Pro subscription) | `claude --model haiku` or `/model haiku` in session |
| **Gemini CLI** | Yes | Gemini 2.5 Flash | $0 (free tier) | `gemini --model gemini-2.5-flash` |
| **Cursor** | Yes | Free tier model | $0 (50 slow requests) | Free plan auto-selects |
| **Amp** | Yes | Frontier models (ad-supported) | $0 | Free tier, no model selection |
| **Roo Code** | Not installed | Gemini Flash via Google AI Studio | $0 | Settings → API Provider → Google → gemini-2.0-flash |
| **OpenCode** | Not installed | Gemini Flash via Google AI Studio | $0 | Config → provider: google, model: gemini-2.0-flash |
| **Windsurf** | Not installed | Free tier model | $0 (25 credits) | Free plan auto-selects |
| **Codex CLI** | Not installed | codex-mini | ~$0.10 total | `codex --model codex-mini-latest` |

### Shared API Key Setup

The 2 BYOK agents (Roo Code, OpenCode) accept a Google AI Studio API key. One key, two agents:

1. Get a free API key from [aistudio.google.com](https://aistudio.google.com/)
2. Configure each BYOK agent to use `gemini-2.0-flash` with that key
3. Cost: $0 — Google AI Studio free tier covers thousands of requests/day

### Total Benchmark Cost

| Category | Agents | Cost |
|---|---|---|
| Already paying / free | Claude Code, Gemini CLI, Cursor, Amp | $0 |
| BYOK with free API key | Roo Code, OpenCode | $0 |
| Free tiers | Windsurf | $0 |
| Paid auth required | Codex CLI | ~$0.10 |
| **Total for 8 agents x 28 checks** | | **~$0.10** |

## Open Questions

1. **Scanner path flexibility** — Will the scanner find SKILL.md files under `benchmark-skills/` in the registry? Or does it only scan `.agents/skills/`? Need to check `scanner.go` scan paths for registries.
2. **Project-level vs user-level install** — The benchmark plan says to install at project level (in the test project directory). `--to-all` currently implies user-level install. Should we support `--to-all --scope project`?
3. **Idempotency** — If skills are already installed, does `--to-all` skip or overwrite? Current behavior is skip-if-exists for symlinks. This is fine.

---

## Next Steps

Ready for implementation planning with `/develop` plan stage.
