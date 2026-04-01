# Reddit, Hacker News & Forum Sentiment

## Common Complaints

### Activation Unreliability (Dominant Theme)

- **msp26** (HN): "half the time Claude Code still doesn't invoke it even."
- **joebates** (HN): "about 5-10% of the time, it will not use the skill."
- **SOLAR_FIELDS** (HN): "way more often than not, i remind the agent that the skill exists before it does anything"
- **themoose8** (HN): "the 'Skills' feature set as implemented...is pretty bogus compared to just front loading the AGENTS.md file."
- Marc Bara's 650-trial study identified two distinct problems: activation failure (skill not invoked) and execution failure (skill loads but steps are skipped). Execution failure is "more dangerous because an activation failure is at least detectable in session logs, but an execution failure produces output that appears to have followed the process" when it hasn't.

### Standard Instability / Churn

On the "Skill.md: An open standard" HN thread:
- **lovich**: "a 6 day turnaround time from announcement to deprecation is unacceptable for a standard" (referring to install.md -> skill.md rename)
- **ClassAndBurn**: "The lack of conviction in your design does not inspire confidence"
- **petcat**: "I feel like there is a new one of these everyday"
- **RadiozRadioz**: worried about random "rugpulls"

### Security Fears

- **JohnMakin** (HN): agents "just go without considering consequences like job termination or organizational harm"
- **skybrian** drew parallels to npm supply chain attacks, questioning how to build trustworthy skill ecosystems
- Snyk found prompt injection in 36% of skills audited and vulnerabilities in 26%

## Desired Changes

1. **Reliable automatic activation** — without manual reminders or hooks
2. **Cross-platform directory standardization** — `.agents/skills/` everywhere
3. **Better decision frameworks** — when to use skills vs MCP vs AGENTS.md vs custom prompts
4. **Dependency management and versioning** — "How do skills handle versioning? Enable cross-project sharing?" (verdverm, HN)
5. **Skill marketplace / registry** — unified, curated (Dshadowzh, HN)
6. **Conflict detection** — when skills are edited simultaneously across tools (prateeksi, HN)
7. **Team-level sharing** beyond individual repo installs

## Confusion Points

### Skills vs Rules vs AGENTS.md vs CLAUDE.md

Users consistently struggle to understand when each applies. Cursor forum has multiple threads asking "How should we distinguish between these two features?" A Substack post was titled "Instructions.md vs Skills.md vs Agent.md vs Agents.md."

### Skills vs MCP

People conflate the two since both extend agent capabilities. Consensus when found: MCP for connectivity, Skills for expertise. But not obvious without explanation.

### What Makes a Skill Different from a Prompt?

- **jairtrejo** (HN): "I feel the term 'Agent Skills' is getting a bit mystical, when in reality it's just a nice pattern for progressive disclosure."
- When Anthropic's example skill was just a SKILL.md file with instructions, it confused people.

### Where to Put Skills

Different agents look in different directories. Cursor forum users ask whether `.claude/skills/` works with Cursor. Claude Code users ask whether `.agents/skills/` works (it was removed).

### Premature Documentation

Cursor staff acknowledged skills docs were published before the feature worked: "Unfortunately, even though it was added to the docs, it wasn't quite ready for primetime." Users tried to follow those docs and hit dead ends.

## Comparisons to Alternatives

### Skills vs AGENTS.md

- Vercel eval: AGENTS.md outperformed skills. Mechanism: AGENTS.md includes "the whole index for the docs...so there's nothing to 'do', the skill output is already in context." (OJFord, HN)
- **motoboi** (HN): skills require RL training data that doesn't exist: "There is not enough training samples displaying that behavior."
- Emerging view: AGENTS.md better for always-on project context, Skills better for on-demand procedural knowledge — but activation problem undermines Skills advantage.

### Skills vs MCP

- **cra.mr** (David Cramer): maintains both, calling "X is all you need" mentality misguided. MCP handles OAuth, permissions, remote services. Skills encode domain expertise.
- One HN commenter: "I have seen ~10 IQ points drop with each MCP I added" — switched to skills
- CLI + Skills beat MCP by 17 points in one benchmark, with 33% token savings

### Skills vs .cursorrules

Always loaded (consuming context), project-specific, Cursor-only. Skills are on-demand, portable, cross-platform. Cursor forum: "coexist and serve different purposes."

### Skills vs Plain Docs

- **CuriouslyC** (HN): just create README files with links
- **iainmerrick**: "just write instructions in English in any old format" is sufficient
- Counter: Skills provide progressive disclosure (~100 tokens at startup vs full doc load)

## Positive Reception

1. **Progressive disclosure valued** — **smithkl42** (HN): with 20+ capabilities, skills enable "progressive disclosure rather than loading everything upfront"
2. **Spec is simple** — "a deliciously tiny specification." Implemented in ~100 lines of Python (jairtrejo, HN)
3. **Benchmarks compelling** — Haiku 4.5 with Skills (27.7%) beats Opus 4.5 without (22.0%)
4. **Cross-platform portability promise** — 30+ tools adopting same spec is meaningful
5. **Tribal knowledge encoding** — teams encode architectural patterns and best practices, reducing code review overhead
6. **Community building real things** — 450+ medical research skills, 220+ developer skills, marketing skills, multiple management tools

## Key HN Threads

- [Agent Skills](https://news.ycombinator.com/item?id=46871173)
- [Skill.md open standard](https://news.ycombinator.com/item?id=46723183)
- [Skills for organizations](https://news.ycombinator.com/item?id=46315414)
- [Agent Skills Security Database](https://news.ycombinator.com/item?id=47402118)
- [Agent Skills Benchmark](https://news.ycombinator.com/item?id=47053217)
- [AGENTS.md Outperforms Skills](https://news.ycombinator.com/item?id=46809708)
- [Agent Skills in 100 Lines of Python](https://news.ycombinator.com/item?id=46634563)
