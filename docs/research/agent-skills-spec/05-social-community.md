# Social Media & Community Channels

## Tool Author Perspectives

### Will Larson (Infrastructure Leader)

Initial skeptic → convert. Built internal agent with skills after two problems forced reconsideration: managing reusable snippets and preventing irrelevant context. "For something that I was initially deeply skeptical about, I now wish I had implemented skills much earlier." Built own implementation to stay platform-agnostic. Top gaps: no sub-skill loading, no sandboxed script execution.

### Tristan Handy (dbt Labs)

Skills encode "hundreds, maybe thousands of hours of collective human experience" in 12kb of markdown. Deployed migration skill that worked end-to-end autonomously. But notes ecosystem is thin — only 8 dbt-related skills exist with huge gaps.

### Kaushik Gopal (Instacart)

First reaction: "So what?" — noting AGENTS.md, slash commands, nested instructions, and MCPs already attempt to define reusable agent workflows. Changed mind based on *how* Anthropic approached the problem, not what it does.

### Google/Gemini CLI Team

Previewed experimental support in v0.23.0, "actively looking for feedback." Daniel Strebel (Google Cloud) framed skills as solving "context bloat," positioning lazy-loading as the key innovation.

### Sebastian Ramirez (@tiangolo, FastAPI)

Shipped library-bundled skills in FastAPI 0.133.1 before spec existed for it. Concern about overhead: "just defining the specific conventional directory to be used would be enough... Removing the need for a config file would make it easier and faster to adopt. Just Markdown files included in the package."

### Simon Willison

Called skills potentially "a bigger deal than MCP" due to token efficiency. But described spec as "quite heavily under-specified."

## User Frustrations

### Anthropic Credibility Gap

The AGENTS.md / `.agents/skills/` controversy is the single most discussed community grievance. 3,020+ upvotes, 224 comments, zero Anthropic response. Undermines trust that Anthropic will steward the spec in good faith. A user reported the feature *worked* and was then *removed*.

### IDE Support Gaps

VS Code has preview support. JetBrains IDE support described as nonexistent. Visual Studio 2026 users report being unable to use skills from Copilot Chat despite the extension allowing skill management.

### Registry Fragmentation

Three competing registries with no clear winner:
- SkillsMP: 96K+ skills, zero security audit
- Skills.sh/Vercel: 83K+ skills, 8M+ installs, better signal
- ClawHub: now defunct after malware crisis

npm took a decade to reach 350K packages; skills ecosystem did it in two months. Portability of SKILL.md is "great for distribution. It's terrible for containment."

## Community Debates

### Skills vs MCP: Complementary or Competing?

Official narrative: "complementary layers." Simon Willison challenged this. Community remains divided on convergence vs distinction.

### Open Standard or Anthropic-Controlled?

The New Stack framed skills as "Anthropic's Next Bid to Define AI Standards." Community worried about single-vendor influence. Spec living in `agentskills/agentskills` (independent org) is positive, but AGENTS.md (Linux Foundation/AAIF) creates parallel governance dynamic.

### Registry Curation Level

Post-ClawHub collapse: heated debate. SpecWeave advocates mandatory three-tier verification. Others argue over-curation kills growth. Core question: "The real question isn't 'is this skill safe?' but 'what's the blast radius when a bad one gets through?'" (Matthew Hou)

### Problem-Shaping vs Skill Quality

Recurring HN/dev.to debate: are skills useful, or do they paper over the real challenge of problem decomposition? "If you cannot decompose the problem clearly enough, the Agent will consistently produce outputs in the wrong direction. This isn't the Agent's fault — it's a problem-shaping problem."

## Wishlist Items

1. **Sub-skill / progressive disclosure within a skill** — Will Larson's top request
2. **Skill signing and provenance** — OWASP published "Agentic Skills Top 10," Snyk demonstrated 3 lines of markdown can exfiltrate SSH keys
3. **Sandboxed script execution** — no spec-level sandbox model, each client implements differently
4. **Richer metadata** — dbt community: "SKILL.md format misses a lot of metadata and nuances for multimodality, secrets, etc."
5. **Permissions model** — no declarative way to express required permissions; Microsoft added `require_script_approval` client-side
6. **Skill versioning** — no version field in spec
7. **Cross-client YAML compatibility** — no conformance test suite exists

## Spec Direction Concerns

### "Deliciously Tiny" vs Dangerously Under-specified

Central tension. Some see minimalism as a feature providing flexibility. Others see a spec that punts on every hard problem while the ecosystem races ahead on sand.

### npm Replay at 10x Speed

Skills ecosystem replaying npm's supply chain security crisis with higher stakes — agents have broader system access than build tools. The spec says nothing about security. OWASP, Snyk, Cisco filling the gap retroactively.

### Convergence Risk with AGENTS.md

AGENTS.md (Linux Foundation/AAIF) and Agent Skills (agentskills.io) address overlapping concerns. Community asks: will these converge, compete, or further fragment? 60,000+ repos already have AGENTS.md files.
