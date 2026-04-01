# Solutions: Distribution, Versioning & Dependencies

## CLI Installers

### Vercel `skills` CLI (DOMINANT TOOL)

**Repo:** vercel-labs/skills | **Stats:** 8M+ installs via skills.sh

How it works: Clones git repos, discovers SKILL.md files, creates symlinks from canonical copy to each agent's directory. Supports 45+ agents. Distinguishes "universal" (`.agents/skills/`) from "non-universal" (vendor-specific) agents.

Commands: `npx skills add <repo>`, `npx skills find`, `npx skills check`, `npx skills update`, `npx skills remove`

Versioning: Compares local vs remote HEAD. No semver pinning.

### agent-skills-cli (Karanjot Singh)

`npm install -g agent-skills-cli`. Installs from SkillsMP, GitHub, private git, npm. Claims 175K+ skills, 45 agents. Features: frozen installs from lockfile, skill composition, quality scoring, context budget management, conflict detection.

### OpenSkills (numman-ali)

Generates `<available_skills>` XML in AGENTS.md, matching Claude Code's format. Bridges non-Claude agents into skills ecosystem.

## Package Managers

### sklz (Alisson Steffens)

npm-like versioning without vendoring. Register git repos as sources, install skills, only `sklz.json` committed. `.agents/skills/` is gitignored. Teammates run `sklz install`.

```json
{
  "sklz": {
    "button-spec": {
      "repo": "https://github.com/my-org/design-skills.git",
      "version": "1.2.0",
      "commit": "a1b2c3d",
      "tags": ["design-system", "ui"]
    }
  }
}
```

Named `sklz.json` deliberately — agents confuse `skills.json` with skill content.

### Paks (Stakpak) — MOST FULLY-FEATURED

Rust CLI with its own web registry (registry.paks.dev). Full lifecycle: create, validate, install, publish.

```yaml
version: 1.0.0
authors:
  - Your Name <you@example.com>
dependencies:
  - name: base-skill
    version: ">=1.0.0"
```

Full semver with `--bump patch/minor/major`. Scoped installs (global vs project). Auth via `paks login`. Homebrew installable.

### skillpm (sbroenne)

~630 lines, 3 runtime deps. Scans `node_modules/` for packages containing `skills/*/SKILL.md`. Wires to agent directories. Configures MCP servers declared by skills. Workspace-aware for monorepos.

Philosophy: "Why build a custom registry when npm already has one?"

## npm-Based Distribution

### antfu/skills-npm (Anthony Fu)

Package authors include `skills/` directory in npm package. `skills-npm` scans `node_modules/**/skills/*/SKILL.md`, creates symlinks. Runs as `prepare` script.

**Key insight:** Version-aligned — `npm update my-tool` updates tool AND skills together.

**Real adoption:** Slidev, eslint-vitest-rule-tester, Vite devtools, VueUse ship bundled skills.

### npm-agentskills (onmax, agentskills#81)

Uses `package.json` field:
```json
{ "agentskills": { "skills": [{ "name": "my-skill", "path": "./skills/my-skill" }] } }
```

Extensible to Cargo.toml, pyproject.toml, Deno.

### tiangolo Library-Bundled Skills (FastAPI)

Libraries include `.agents/skills/<library-name>/SKILL.md` inside package. Skills auto-update with `pip install --upgrade`. Shipped in FastAPI 0.133.1.

"Just defining the specific conventional directory to be used would be enough... Just Markdown files included in the package."

## Spec-Level Proposals

### Discussion #210: skills.json Manifest (THE COMPREHENSIVE PROPOSAL)

```json
{
  "schema_version": 1,
  "name": "code-quality",
  "version": "1.0.0",
  "skills": ["./skills/lint-check", "./skills/review-pr"],
  "dependencies": {
    "git-operations": "github.com/example/git-skills@v1.0.0"
  }
}
```

Git URLs as identity (like Go modules). Minimum version selection, cycle detection, collision detection. Lockfile with SHA-256 digests. SKILL.md stays untouched — `skills.json` is for tooling only.

### Discussion #243: Language-Agnostic Manifest

Overlaps with #210. Emphasizes: manifest must NOT be named `skills.json` (agents try to read it). References sklz as proof-of-concept.

### Issue #255: `.well-known` URI Discovery (MAINTAINER-ENDORSED)

Adopt Cloudflare's RFC. `/.well-known/agent-skills/index.json`:
```json
{
  "skills": [{
    "name": "code-review",
    "type": "skill-md",
    "url": "/.well-known/agent-skills/code-review/SKILL.md",
    "digest": "sha256:c4d5e6f7..."
  }]
}
```

Mintlify already implements. PR #254 in progress.

### Issue #110: Dependencies with Version Validation

```yaml
requires:
  - skill: environment-selector
    version: "1.2.0"
```

Also proposes `test` field for portable skill testing. Maintainer direction: moved to distribution layer.

### Issue #46: Versioning/Locking

Versioning belongs in distribution layer, not spec. Consensus forming.

## Parameterization & Composition

### skillctx (jackchuka)

Replaces hardcoded values with `{placeholders}`, resolved from shared config. Migration: `/skillctx-ify my-skill` scans for hardcoded values.

```json
{
  "vars": {
    "identity": { "github_username": "alice" },
    "slack": { "standup_channel_id": "CABC123DEF" }
  }
}
```

### Skillfold (byronxlg)

**Compiler**, not package manager. Declare pipelines in `skillfold.yaml` (agents, state schema, flow transitions). Validates type matching, reachability, cycle exits. Outputs native SKILL.md files. "TypeScript for agent pipelines."

## Registries

| Registry | Skills | Features |
|----------|--------|----------|
| skills.sh (Vercel) | 83K+ | Leaderboard, telemetry-fed, 8M+ installs |
| SkillsMP | 351K+ | GitHub crawler, AI semantic search, min 2 stars |
| SkillHub (iFlytek) | Self-hosted | Enterprise, RBAC, full semver, rollback |
| ClawHub/OpenClaw | 13K+ | Vector search, versioning (security crisis) |

## Ecosystem Split

Two philosophies emerging:
1. **Git-centric** (Vercel skills, sklz, #210) — skills in git repos, versioned by tags/commits, no central registry
2. **Package-manager-centric** (npm-agentskills, antfu/skills-npm, skillpm, Paks) — piggyback on npm/PyPI/Cargo

Maintainers lean toward `.well-known` for remote discovery, keeping manifest/dependency as separate distribution layer concern.
