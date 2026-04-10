# Top Pain Points (Ranked by Frequency)

## 1. Anthropic Doesn't Follow Its Own Standard

The single loudest complaint across all sources. GitHub issue anthropics/claude-code#31005 has **3,020+ upvotes, 224 comments, zero Anthropic response after 7 months.** The spec defines `.agents/skills/` as the standard directory, but Claude Code forces `.claude/skills/`.

Key quotes:
- "Anthropic created the Agent Skills open standard, and the spec itself defines .agents/skills/ as the standard directory. Your own docs state: 'Claude Code skills follow the Agent Skills open standard.' Yet Claude Code itself doesn't follow the standard it created."
- "The fragmentation is brutal. I have 12 agent definitions that need to live somewhere standard, and right now it's all locked into .claude/ with no portability." — @Koroqe
- ".agents/skills worked on version 1.0117. Why did you remove it?" — @b3nk3
- "We love Claude Code. We pay for Claude Code. We just want it to follow the standards it created."
- "not just ironic, it's self-serving" / "the exact kind of vendor lock-in MCP was designed to eliminate"

**Sources:** anthropics/claude-code#31005, anthropics/claude-code#6235, HN threads

## 2. Skills Don't Trigger Reliably

The #1 technical pain point. Multiple independent data points:

- Vercel's evals: skills achieve **53-79% activation** vs AGENTS.md's **100%**
- 650-trial empirical study (Marc Bara): default passive descriptions achieve only ~77% activation; directive-style ("ALWAYS invoke...") achieves 100%
- Claude self-diagnosed: "My default mode always wins because it requires less cognitive effort and activates automatically"
- User reports across Claude Code, Gemini CLI, and Cursor of needing to explicitly remind agents

Key quotes:
- "properly configured skills being systematically ignored... It's necessary to explicitly mention the skill in the user prompt for it to be applied." — @EricGT (agentskills#57)
- "Gemini does not use custom skills and sub-agents on its own, basically at all." — Gemini CLI#21968
- "half the time Claude Code still doesn't invoke it even" — msp26, HN
- "way more often than not, i remind the agent that the skill exists before it does anything" — SOLAR_FIELDS, HN
- "the 'Skills' feature set as implemented...is pretty bogus compared to just front loading the AGENTS.md file." — themoose8, HN

Spec maintainer position: "I'm reluctant to introduce more structured ways of representing the activation conditions for skills. The hope is that a natural language description should suffice, particularly as LLMs continue to get more intelligent." — @klazuka (agentskills#57)

**Sources:** agentskills#57, gemini-cli#21968, anthropics/claude-code#19308, #20986, Medium (Marc Bara), HN threads

## 3. Security Is a Crisis

Snyk's ToxicSkills audit: **36.82% of skills** (1,467/3,984) on registries had security flaws, **13.4% critical**. ClawHub had **20% malicious packages** (800+) before shutting down. 76 confirmed malicious payloads identified. Researchers demonstrated invisible Unicode Tag injection in SKILL.md files.

The spec contains **zero normative security language** — no signing, no sandboxing, no permissions model, no trust boundaries.

Key quotes:
- "100% of confirmed malicious skills contain malicious code patterns, while 91% simultaneously employ prompt injection techniques" — Snyk
- "every time there's a new technology, we abandon the security practices that we had before" — 4ppsec, HN
- "npm distributed discrete artifacts. Agent skills distribute behaviors into continuously-running systems with persistent identity and state. This fundamentally changes the threat model." — Kalpaka
- "There's no way to grant a skill limited permissions (e.g., 'read-only filesystem' or 'network access only to api.trello.com')" — Nils Friedrichs

Responses: OWASP created "Agentic Skills Top 10," Snyk published ToxicSkills research, Cisco released skill scanner, Red Hat published threat analysis.

**Sources:** Snyk ToxicSkills, OWASP, Grith.ai, Red Hat, Pluto Security, Embrace The Red, HN threads

## 4. Path/Directory Fragmentation

GitHub issue agentskills#15 (104 comments) — every tool uses a different path:
- `.claude/skills/` (Claude Code)
- `.codex/skills/` (OpenAI Codex)
- `.gemini/skills/` (Gemini CLI)
- `.github/skills/` (GitHub Copilot)
- `.agents/skills/` (proposed standard)
- `.opencode/skill/` (OpenCode)

Symlinks (the community workaround) don't work everywhere — Codex ignores symlinked directories entirely. Roo Code has bugs with symbolic linked files.

Key quotes:
- "I agree, this is frustrating. It creates a vendor lock-in that is more of a nuisance than lock-in because it can be circumvented by symlinks / copying the skills folder across folders." — @mderazon
- "The last thing we need is every coding agent provider having a different directory location." — @TabishB
- "This copies (duplicates) the skills across folders, to me that's horrible" — @mderazon (on Vercel's add-skill workaround)
- OpenAI's @gverma-openai: "We agree on the general problem: when skills are distributed on file systems, sharing skills across agents is hard, and pushing the burden of symlinking or duplication onto users isn't a scalable solution."

Emerging consensus around `.agents/skills/` but no spec change yet.

**Sources:** agentskills#15, various tool repos

## 5. No Distribution, Versioning, or Dependency Story

Multiple competing tools emerged (craft, sklz, skillfold, add-skill, ai-agent-skills) because the spec doesn't address installation, updates, versioning, or composition.

- No semver enforcement, no lock files, no dependency declarations
- `metadata.version` is optional and freeform (purely informational)
- Skills can't declare dependencies on other skills or MCP servers
- No standard for remote discovery (`.well-known` RFC in progress)

Key quotes:
- "I have been building skills that depend on other skills from different repos, and kept running into the same wall: there's no standard way for users to install my skill and everything it needs." — @erdemtuna (agentskills#100)
- "there are a lot of things to consider including security and identity. Skills are a decentralized concept with no central naming authority." — @klazuka
- "How do skills handle versioning? Enable cross-project sharing? Provide dependency management?" — verdverm, HN

**Sources:** agentskills#100, #110, #46, discussions#210, #243, HN threads
