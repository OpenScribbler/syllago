# GitHub Issues & Discussions

## Critical Issues

### No Standard Skills Directory (agentskills#15, 104 comments)

The most-discussed issue in the repo. Every tool has its own path. Emerging consensus around `.agents/skills/` but no spec change.

- @mderazon: "I agree, this is frustrating. It creates a vendor lock-in for specific agent that is more of a nuisance than lock-in because it can be hacked to be circumvented by symlinks / copying the skills folder across .gemini / .claude / .github folders."
- @TabishB: "The last thing we need is every coding agent provider having a different directory location that we need to either copy over or create symlinks for."
- @douglascamata: "Antigravity now has skill support at: `~/.gemini/antigravity/skills` or `<workspace-root>/.agent/skills/`. This is becoming a nice mess. :/"
- @gverma-openai (OpenAI): "We agree on the general problem: when skills are distributed on file systems, sharing skills across agents is hard, and pushing the burden of symlinking or duplication onto users isn't a scalable solution."

### Skills Get Ignored (agentskills#57, gemini-cli#21968)

- @EricGT: "properly configured skills being systematically ignored... It's necessary to explicitly mention the skill in the user prompt for it to be applied."
- Gemini CLI internal: "Gemini does not use custom skills and sub-agents on its own, basically at all. It will if I instruct it to explicitly, but won't when it's doing something very related."
- @ralfstrobel: "I also had to add explicit instructions to CLAUDE.md indicating that contextually discovered skills are highly relevant and should always be invoked. Otherwise they would often be ignored."

### Gemini CLI Implementation Breaks Interop (gemini-cli#15895)

Detailed report: "Gemini CLI's current Skills implementation treats skills as 'tools with documentation' rather than methodologies with structured resources." Specific problems:
- No distinction between `scripts/`, `references/`, and `assets/` directories
- No progressive disclosure — dumps ALL resources at Level 2
- Skills written for Claude Code don't work in Gemini CLI without modification

### Skill-to-Skill Invocation Undefined (agentskills#95)

- @ShotaOki: skill-to-skill invocation works in English but fails ~90% of the time in Japanese: "After investigating more carefully, I discovered that this behavior only happens with Japanese prompts."
- The spec says nothing about whether skills can invoke other skills.

### `allowed-tools` Format Ambiguity (agentskills#144)

Analysis of 256 community skills found 3 different YAML formats in use: space-delimited string, inline YAML sequence `[...]`, and block YAML sequence `- item`. "A strict string-typed parser rejects valid skills that use list syntax." 14% of affected skills.

## Feature Requests

### Skill Dependencies and Distribution (agentskills#100, #110, discussions#210, #243)

Most-requested category. Multiple independent tools emerged (craft, sklz, skillfold).

- @erdemtuna: "I have been building skills that depend on other skills from different repos, and kept running into the same wall: there's no standard way for users to install my skill and everything it needs."
- @klazuka: "I would like to have a solution to this problem, but there are a lot of things to consider including security and identity."

### Versioning and Locking (agentskills#46)

- @EricGT: "text in SKILL.md not directly needed in the prompt should not be in SKILL.md... I currently have 48 skills."
- @jonathanhefner: "I would be more inclined to define versioning as part of the distribution mechanism rather than SKILL.md."
- Consensus forming: versioning belongs in distribution/manifest layer, not SKILL.md.

### Secrets and Authentication (agentskills#86, discussions#173)

- @keithagroves: proposed declaring env vars in frontmatter with `secret: true` flag
- @ashwinhprasad: "Without this, skills are effectively limited to non-authenticated or local-only integrations."
- @jonathanhefner (security concern): "One downside of that approach is malicious skill scripts also have access to those env vars."

### MCP Server Dependencies (agentskills#21, discussions#195)

No standard way to declare MCP server requirements. @klazuka: "MCP servers are distributed in many ways, and so it's hard to uniquely refer to a particular MCP server." Workaround: mention MCP server in prose.

### Capability/Permissions Declarations (discussions#181)

Driven by scam skills on OpenClaw. Proposed capabilities vocabulary: `shell`, `filesystem`, `network`, `browser`. Smart distinction between required and optional.

### `.well-known` URI for Remote Discovery (agentskills#255, Cloudflare RFC)

Active collaboration between Cloudflare, agentskills maintainers, and community. PR #254 in progress. Multiple tools expressing interest.

### Skill Parameterization (discussions#246)

@jackchuka built skillctx to solve at tooling layer. @ianscrivener pushes back on JSON Schema: "less tokens are better!" — prefers letting LLM read config naturally.

## Design Concerns

### Governance Gap (agentskills#59, discussions#269)

- @yordis: "Anthropic decides the direction here, and until it is not part of something like Linux Foundation, or some sort of governance; it is a Claude Code or Anthropic thing."
- @dacharyc (skill-validator maintainer): "The community has filed 48 proposals in Discussions (as directed by CONTRIBUTING.md). Of those, 1 has been answered. Multiple proposals touch on versioning and interoperability but none have resulted in spec changes. Meanwhile, spec-altering PRs from the maintainer continue to land without going through the Discussions process."
- @dacharyc requested: version identifier, changelog for normative changes, labels on spec-altering PRs. @jonathanhefner agreed in principle.

### SKILL.md Filename Rigidity (agentskills#30)

- @marcusquinn: "I think someone, desperate to be the person that made a new standard... hastily solidifying this standard... didn't consider the unnecessary overhead."
- @klazuka acknowledged: "I agree that wrapping a single SKILL.md file in a directory with no other files is clumsy and noisy. We are thinking about whether it makes sense to introduce a single-file variant."

### Natural Language Activation Insufficient (agentskills#57, #64)

Core philosophical split:
- Community: structured activation mechanisms needed (globs, triggers, keywords)
- Maintainer: "natural language should suffice as LLMs get more intelligent"

### Context Bloat (agentskills#46, discussions#252)

- @EricGT (48+ skills): pushes back on token-consuming metadata additions
- @ianscrivener: "this agent skill spec should be fit-for-purpose for mobile phones, edge devices, and even micro processors"
- Recurring tension: every proposed feature adds tokens

### Skill Composition Undefined (gemini-cli#22420)

When project-local and global skills share names, Gemini CLI treats it as a "conflict" and overrides. Users want composition/layering. Currently requires duplicating entire global skill contents.

## Implementation Pain Points

- **Each implementer interprets the spec differently** — directory structure, frontmatter schema, `allowed-tools` format, progressive disclosure
- **Windows path resolution** — OpenCode on Windows doesn't resolve `~/.agents/skills/` correctly (opencode#17007)
- **Symlinks not universally supported** — Codex ignores them, Roo Code has bugs with them
- **Eager loading wastes context** — Roo Code#10393: agent read every SKILL.md instead of just showing metadata
- **FastAPI's @tiangolo**: shipped library-bundled skills before spec existed, concerned about overhead: "just defining the specific conventional directory to be used would be enough... Removing the need for a config file would make it easier and faster to adopt."
