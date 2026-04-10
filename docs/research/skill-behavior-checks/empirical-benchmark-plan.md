# Empirical Skill Loading Benchmark Plan

**Date:** 2026-03-31
**Goal:** Empirically test all 28 checks from [agentskillimplementation.com/checks](https://agentskillimplementation.com/checks/) across 12 agents using the official benchmark skills, then PR results back to the community repo
**Benchmark repo:** `~/.local/src/agent-skill-implementation/` (cloned from `agent-ecosystem/agent-skill-implementation`)
**Prior work:** `00-behavior-matrix.md` — source code inspection of 14 checks (not empirical). This plan replaces that approach with actual runtime testing using the official canary phrase methodology.

---

## What Changed from the Original Plan

The original research plan (`00-behavior-matrix.md`) used AI subagents reading source code and docs — valid for structural questions but not empirical. Dakri (the checklist author) correctly flagged that source-code-derived findings are less trustworthy for runtime behavior.

The benchmark repo provides:
- **17 pre-built test skills** with canary phrases embedded in specific files
- **28 checks** (not 14 — we only covered a subset before) across 9 categories
- **Canary phrase methodology** — ask the model if it knows a specific phrase to determine what was loaded, no internal state inspection needed
- **Contribution template** at `platform-loading-implementation/template.md`
- **Check-to-skill mapping** telling you exactly which skill to use for each check

This makes nearly all checks empirically testable from the outside.

---

## The 28 Checks

### Category 1: Loading Timing (3 checks)
| Check | Skill | Automatable? | Method |
|---|---|---|---|
| `discovery-reading-depth` | `probe-loading` | Yes | Ask "Do you know CARDINAL-ZEBRA-7742?" before activating. |
| `activation-loading-scope` | `probe-loading` | Yes | Activate, then check 5 canary phrases (PELICAN-MANGO-3391, FALCON-QUARTZ-8819, OSPREY-COBALT-5567, HERON-AMBER-2204, CRANE-TOPAZ-6638). |
| `eager-link-resolution` | `probe-linked-resources` | Yes | Activate, check PARROT-SILVER-4412, TOUCAN-BRONZE-9931, EAGLE-COPPER-1178. Unlinked file (EAGLE) distinguishes link-following from bulk loading. |

### Category 2: Directory Recognition (3 checks)
| Check | Skill | Automatable? | Method |
|---|---|---|---|
| `recognized-directory-set` | `probe-loading` | Yes | Activate, ask what directories/files the model is aware of. |
| `directory-naming-divergence` | `probe-nonstandard-dirs` | Yes | Activate, check if SWIFT-OPAL-8156 (in `resources/` not `references/`) is visible. |
| `unrecognized-directory-handling` | `probe-nonstandard-dirs` | Yes | Check canaries ROBIN-JADE-3847, WREN-PEARL-6293, SWIFT-OPAL-8156 for evals/, templates/, resources/. |

### Category 3: Resource Access Patterns (4 checks)
| Check | Skill | Automatable? | Method |
|---|---|---|---|
| `resource-enumeration-behavior` | `probe-loading` | Yes | Activate, check if all 3 reference files are listed (including unreferenced OSPREY-COBALT-5567). |
| `path-resolution-base` | `probe-linked-resources` | Yes | Activate, have model read files by relative path, observe what resolves. |
| `cross-skill-resource-shadowing` | `probe-shadow-alpha` + `beta` | Yes | Activate both, read `references/API.md` from each. STORK-CORAL-4471 vs EGRET-SLATE-8823. |
| `path-traversal-boundary` | `probe-traversal` | Yes | Activate, attempt three reads: `../probe-loading/SKILL.md` (sibling skill), `../README.md` (parent dir), `../../loading-behavior.md` (two levels up). |

### Category 4: Content Presentation (3 checks)
| Check | Skill | Automatable? | Method |
|---|---|---|---|
| `frontmatter-handling` | `probe-loading` | Yes | Activate, ask if model sees allowed-tools, compatibility, metadata fields. |
| `metadata-value-edge-cases` | `probe-metadata-values` | Yes | Activate (does it load at all?), check THRUSH-FLINT-8294, ask which metadata keys survived. |
| `content-wrapping-format` | `probe-loading` | Semi | Activate, ask model to describe how content was presented. Relies on model self-report. |

### Category 5: Lifecycle Management (3 checks)
| Check | Skill | Automatable? | Method |
|---|---|---|---|
| `reactivation-deduplication` | `probe-loading` | Yes | Activate, have a brief conversation, then activate again. Ask the model if it sees the skill instructions twice in its context. |
| `reactivation-freshness` | `probe-loading` | Semi | Activate, verify CARDINAL-ZEBRA-7742 is present, then edit SKILL.md to replace it with a new canary (e.g., CHANGED-PHRASE-0001). Re-activate in same session, check which canary the model reports. Requires file edit mid-test. |
| `context-compaction-protection` | `probe-loading` | Slow | Activate, generate long conversation, check if CARDINAL-ZEBRA-7742 survives. Needs many messages. |

### Category 6: Access Control (2 checks)
| Check | Skill | Automatable? | Method |
|---|---|---|---|
| `trust-gating-behavior` | Any | Manual | Install in fresh/untrusted repo. Observe if agent prompts for approval. |
| `compatibility-field-behavior` | `probe-compatibility` | Yes | Activate, follow instructions. Test on non-Claude agents for the "Designed for Claude Code" text. |

### Category 7: Structural Edge Cases (2 checks)
| Check | Skill | Automatable? | Method |
|---|---|---|---|
| `nested-skill-discovery` | `probe-deep-nesting` | Yes | Install, check if `nested-skill` (HAWK-ONYX-5534) appears in available skills list. |
| `resource-nesting-depth` | `probe-deep-nesting` | Yes | Activate, read files at depths 1-3. DOVE-GARNET-1029, LARK-RUBY-4483, OWL-EMERALD-7756, FINCH-SAPPHIRE-2098. |

### Category 8: Skill-to-Skill Invocation (4 checks)
| Check | Skill | Automatable? | Method |
|---|---|---|---|
| `cross-skill-invocation` | `invoke-alpha` + `beta` | Yes | Activate alpha (confirm IBIS-RUST-3310 loaded), check if TERN-MOSS-6647 (beta) appears. |
| `invocation-depth-limit` | `invoke-alpha` + `beta` + `gamma` | Yes | Activate alpha (IBIS-RUST-3310), let chain run. Does TERN-MOSS-6647 (beta) and JAY-TEAL-9984 (gamma) appear? |
| `circular-invocation-handling` | `probe-circular-alpha` + `beta` | Yes | Activate alpha. Count occurrences of KITE-ONYX-2251 and WREN-SLATE-7738. |
| `invocation-language-sensitivity` | `invoke-alpha` chain | Semi | Run chain in English, then repeat in Japanese (e.g., "呼び出しチェーンを開始してください"). Compare success rates across multiple runs. |

### Category 9: Skill Dependencies (4 checks)
| Check | Skill | Automatable? | Method |
|---|---|---|---|
| `informal-dependency-resolution` | `invoke-alpha` + `beta` | Yes | Same as cross-skill-invocation (prose-expressed dependency). |
| `missing-dependency-behavior` | `probe-missing-dep` | Yes | Activate, confirm GULL-IRON-4492 is known (skill body loaded). Skill references nonexistent `nonexistent-formatter`. Observe failure mode. |
| `nonstandard-dependency-fields` | `probe-nonstandard-fields` | Yes | Activate. Has `requires` and `depends-on` in frontmatter. Check if acted on. |
| `cross-scope-dependency` | `probe-cross-scope` + `probe-loading` | Semi | Step 1: Install probe-cross-scope at project level, probe-loading at user level. Activate probe-cross-scope, confirm CRANE-STEEL-1163 is known (skill body loaded), check if it can invoke probe-loading across scopes. Step 2: Remove probe-loading from user level, re-test to observe failure mode. |

### Automability Summary

| Category | Count | Checks |
|---|---|---|
| **Fully automatable** | 22 | All of Cat 1-3 (10), frontmatter-handling, metadata-value-edge-cases, reactivation-deduplication, compatibility-field-behavior, nested-skill-discovery, resource-nesting-depth, cross-skill-invocation, invocation-depth-limit, circular-invocation-handling, informal-dependency-resolution, missing-dependency-behavior, nonstandard-dependency-fields |
| **Semi-auto** (needs file edit, multi-run, or multi-scope) | 4 | content-wrapping-format (model self-report), reactivation-freshness (file edit mid-test), invocation-language-sensitivity (multi-run + multi-language), cross-scope-dependency (multi-scope setup) |
| **Slow** (needs long conversation) | 1 | context-compaction-protection |
| **Manual** (requires observation) | 1 | trust-gating-behavior |
| **Total** | **28** | |

**Bottom line: 22 of 28 checks can be fully automated with a prompt script. 4 more can be semi-automated. 1 is slow but scriptable. Only trust-gating requires manual observation.**

---

## Agent Coverage & Cost

### Tier 1: Free, no catches — test first

| Agent | How to Run Free | Install Method |
|---|---|---|
| Gemini CLI | 1,000 req/day free with Google account | `npm install -g @google/gemini-cli`; place skills in `.gemini/skills/` |
| Cline | Open source VS Code extension + Google AI Studio free API key (Gemini Flash) | VS Code extension; place skills in `.cline/skills/` or `.agents/skills/` |
| Roo Code | Open source VS Code extension + Google AI Studio free API key | VS Code extension; place skills in `.roo/skills/` or `.agents/skills/` |
| OpenCode | Open source CLI + Google AI Studio free API key | CLI install; place skills in `.agents/skills/` |
| Amp | $10/day free credits (ad-supported) | CLI or VS Code; place skills in `.agents/skills/` |

### Tier 2: Free tier, enough for benchmarking

| Agent | Free Allowance | Enough for 28 checks? |
|---|---|---|
| GitHub Copilot | 50 premium requests/mo | Tight. Run each check in minimal prompts (2-3 per check = ~100 prompts). May need 2 months or paid. |
| Cursor | 50 slow premium requests/mo | Same constraint. Prioritize highest-value checks. |
| Windsurf | 25 credits/mo | Very tight. May need to split across months or pay. |
| Kiro | 50 credits/mo + 500 bonus on signup | Bonus credits make this doable in month 1. |
| Junie CLI | 1-week free trial + BYOK | BYOK with Google AI Studio key works post-trial. |

### Tier 3: Requires payment

| Agent | Min Cost | Notes |
|---|---|---|
| Claude Code | $20/mo Pro (already have) | Full access. |
| Codex CLI | $20/mo ChatGPT Plus or OpenAI API credits | Check if limited-time free access still works. |

### Recommended Execution Order

1. **Claude Code** (already have, know it best — use as baseline)
2. **Gemini CLI** (free, open source, generous limits)
3. **Cline** (free, BYOK, VS Code extension)
4. **Roo Code** (free, BYOK, similar to Cline)
5. **OpenCode** (free, BYOK, CLI)
6. **Amp** (free ad-supported)
7. **Kiro** (500 bonus credits on signup)
8. **Junie CLI** (free trial + BYOK)
9. **GitHub Copilot** (free tier, 50 requests)
10. **Cursor** (free tier, 50 requests)
11. **Windsurf** (free tier, 25 credits — prioritize checks)
12. **Codex CLI** (pay or free trial)

---

## Test Execution Strategy

### Setup (one-time)

1. Clone the benchmark repo to a test project directory
2. For each agent, install all 17 benchmark skills at the appropriate location:
   - CLI agents: `.agents/skills/` or agent-specific dir
   - IDE agents: workspace `.agents/skills/` or `.cursor/skills/` etc.
3. Create a Google AI Studio API key for BYOK agents
4. Set up accounts for Tier 2 agents (Kiro, Copilot free, etc.)

### Per-Agent Test Run

For each agent, run 28 checks following the check-to-skill mapping from the benchmark README. Key rules from the contribution template:

- **Run each check in a separate session** (prior checks leave context that contaminates later results)
- **Distinguish platform-level vs model-level behavior** (deterministic harness behavior vs probabilistic model behavior)
- **Record fallback behavior** (when default behavior doesn't surface content, does the agent self-recover, require user prompt, or have no fallback?)
- **Note the specific model used** (behavior may vary by model within the same agent)
- **Run probabilistic checks multiple times** (model-level behaviors need 3-5 runs for confidence). The checks most likely to show model-level variation: `content-wrapping-format` (model self-report), `cross-skill-invocation` (model chooses to chain), `invocation-language-sensitivity` (model interpretation), `informal-dependency-resolution` (model follows prose), `missing-dependency-behavior` (model failure mode)
- **Record fallback behavior for every check** — when the default behavior doesn't surface content, does the agent: (a) self-recover by reading files on its own, (b) require the user to prompt it explicitly, or (c) have no fallback at all? This is required by the contribution template
- **Use the template's status taxonomy** for each check: `Observed` (tested, have a finding), `Inconclusive` (tested, ambiguous or inconsistent across runs), or `Not tested` (haven't tested yet)
- **Record platform-level vs model-level** for each check — is the behavior enforced by the harness (deterministic) or decided by the model (probabilistic)?
- **Record platform version, check list version, and model used** in the Platform Details table at the top of each output file

### Prompt Script (for automatable checks)

A minimal prompt script per check. Each check = one fresh session with these steps:

```
1. Start fresh session in test project directory
2. [Pre-activation checks, if any — e.g., "Do you know CARDINAL-ZEBRA-7742?"]
3. Activate the target skill (agent-specific command)
4. Ask the check-specific questions
5. Record: observation, evidence, platform-vs-model, fallback behavior
```

For the 22 fully automatable checks, this could be scripted as a prompt file per check that gets pasted into each agent.

### Output Format

One file per agent following `platform-loading-implementation/template.md`:
- File: `platform-loading-implementation/<agent-slug>.md`
- Fill in all 28 check sections
- PR to `agent-ecosystem/agent-skill-implementation`

---

## Relationship to Prior Research

The source-code-inspection research (`00-behavior-matrix.md`) remains valuable as:
1. **Predictions** — the empirical tests will confirm or correct each finding
2. **Context** — knowing the source code helps interpret ambiguous empirical results
3. **Coverage** — some structural questions (Q4: recognized directory set, Q8: content wrapping format) are better answered by source code than canary phrases

After empirical testing, we should update `00-behavior-matrix.md` with a reconciliation column: did the empirical result match the source-code prediction?

---

## Deliverables

1. **12 platform files** in contribution template format — one per agent
2. **PR to agent-ecosystem/agent-skill-implementation** with results
3. **Reconciliation doc** comparing empirical results to source-code predictions
4. **Updated `00-behavior-matrix.md`** with empirical evidence levels added
