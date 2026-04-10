# Solutions: Path/Directory Fragmentation

## Solution 1: `.agents/skills/` Convention (De Facto Winner)

**Proposed by:** @douglascamata (Dec 2025), refined by community

**What:** Single vendor-neutral directory at both scopes:
- Project: `.agents/skills/`
- User: `~/.agents/skills/`

**Adoption (15+ tools):** Codex (OpenAI confirmed Feb 2026), Gemini CLI, VS Code/Copilot, GitHub, OpenCode, Amp, Kimi CLI, Windsurf, Antigravity (Google), Mistral Vibe, Augment Code, Roo Code, OpenClaw, Craft Agents

**Holdout:** Claude Code still only scans `.claude/skills/`

**PR #174:** Adds to spec: "It's recommended that adopters automatically load skills from `~/.agents/skills` and `.agents/skills`." **Still OPEN — no merge despite massive adoption.**

**Integration guide:** Already mentions `.agents/skills/` as convention, but NOT in the formal specification.

**@douglascamata's frustration:** "Honestly I'm already fairly confident this proposal will not be incorporated into the spec even with it already being the de-facto standard, which is sad."

## Solution 2: XDG-Compliant Paths (Sub-debate)

For user-level skills: `~/.config/agents/skills/` vs `~/.agents/skills/`

**@jonathanhefner (maintainer):** "if the specification mandates particular paths for user-level directories, I **strongly** feel that we should abide by the XDG standard."

Split in practice:
- XDG: Amp, Kimi CLI (`~/.config/agents/skills/`)
- Non-XDG: Codex, OpenCode (`~/.agents/skills/`)

## Solution 3: Vercel `npx skills` (Tooling Abstraction)

**Repo:** vercel-labs/skills

Maps skills to each agent's native path via hardcoded agent registry (30+ agents). Distinguishes "universal agents" (using `.agents/skills/`) from "non-universal agents" (vendor-specific). Creates symlinks from canonical copy.

Pragmatic bridge, not a standard. Requires constant maintenance.

## Solution 4: `~/.skills` with `AGENT_SKILLS_PATH` Override

**Proposed by:** @vmalyi (PR #120). Closed in favor of #15.

## Solution 5: `.well-known` URI for Remote Discovery

**Proposed by:** @jonathanhefner (maintainer) in PR #254

Based on Cloudflare's RFC. Publishers host `/.well-known/agent-skills/index.json`. Mintlify already implements it. Orthogonal to local filesystem path question.

## Solution 6: AGENTS.md Directory Expansion

**agentsmd/agents.md#9:** Proposes `.agents/` directory for modular instruction files, aligning with the broader `.agents/` namespace.

## Current State

| Approach | Status | In Spec? | Adoption |
|----------|--------|----------|----------|
| `.agents/skills/` convention | De facto standard | In guide only | 15+ tools |
| XDG `~/.config/agents/skills/` | Sub-debate | No | 2-3 tools |
| Vercel `npx skills` | Shipping | N/A (tooling) | Thousands of installs |
| `.well-known` URI | PR open | Proposed | Mintlify |
| PR #174 (add to spec) | Open, no merge | Proposed | N/A |
