# Cross-Source Synthesis

## What to Preserve

These are universally praised — changing them would lose the adoption advantage:

1. **Radical simplicity** — a folder with a markdown file, zero infrastructure
2. **Progressive disclosure** — ~50 tokens at startup, full load on-demand
3. **Human readability** — markdown first, anyone can audit
4. **Decentralized model** — git-native, no central authority required
5. **Token efficiency** — orders of magnitude better than MCP for equivalent capability

## What Must Change (Community Consensus)

These have broad agreement across GitHub, HN, Reddit, blog posts, and security researchers:

### Tier 1: Blocking Issues (widespread frustration, active discussion)

1. **Path standardization** — `.agents/skills/` must be the canonical path, adopted by all major tools including Claude Code
2. **Activation reliability** — the spec needs structured activation mechanisms beyond natural language description matching (globs, triggers, keywords, forced activation)
3. **Security primitives** — signing, capability declarations, sandbox requirements at the spec level
4. **Governance process** — version identifier, changelog, responsive community proposal process

### Tier 2: Missing Features (frequently requested, multiple implementations filling the gap)

5. **Distribution standard** — install, update, version pin, dependency resolution
6. **Permissions model** — declarative capability requirements (`filesystem`, `network`, `shell`)
7. **Remote discovery** — `.well-known` URI or equivalent (Cloudflare RFC in progress)
8. **MCP dependency declaration** — skills that need MCP servers can't express it
9. **Secrets/auth** — env var declaration with sensitivity flags

### Tier 3: Design Improvements (quality of life, less urgent)

10. **Single-file variant** — skip the mandatory folder-per-skill for simple skills
11. **Skill composition** — nested skills, sub-skills, progressive disclosure within a skill
12. **`allowed-tools` format normalization** — pick one YAML format, document it
13. **Conformance test suite** — verify implementations match the spec
14. **Inter-skill communication** — how skills invoke each other (especially i18n)

## What's Contentious (No Consensus)

These are actively debated with legitimate arguments on both sides:

1. **Structured vs natural language activation** — maintainer bets on LLM improvement, community wants deterministic triggers now
2. **How much metadata in SKILL.md** — token-conscious users resist additions, enterprise users want more fields
3. **Registry governance** — high curation (kills growth) vs low curation (security crisis)
4. **AGENTS.md convergence** — will these standards merge, compete, or coexist?
5. **Skills vs prompts** — "just a nice pattern for progressive disclosure" or genuine capability extension?

## Key Metrics

| Metric | Value | Source |
|--------|-------|--------|
| Adopting tools | 33+ | agentskills.io |
| Skill activation rate (passive description) | 53-79% | Vercel evals, Marc Bara study |
| Skill activation rate (directive description) | ~100% | Marc Bara 650-trial study |
| AGENTS.md activation rate | 100% | Vercel evals |
| Registry skills with security flaws | 36.82% | Snyk ToxicSkills |
| Critical security issues | 13.4% | Snyk ToxicSkills |
| Confirmed malicious payloads | 76 | Snyk ToxicSkills |
| Community proposals filed | 48 | @dacharyc (agentskills#269) |
| Community proposals answered | 1 | @dacharyc (agentskills#269) |
| Top GitHub issue upvotes | 3,020+ | anthropics/claude-code#31005 |
| Repos with AGENTS.md | 60,000+ | Linux Foundation |
| Skills on registries | 145K+ (SkillsMP) / 83K+ (skills.sh) | Various |

## Implications for Syllago

Syllago sits at a unique intersection — it's a cross-provider content sharing platform that implements the spec. Key opportunities:

1. **Path resolution** — syllago can be the tool that solves directory fragmentation by installing to `.agents/skills/` and symlinking/copying to provider-specific paths
2. **Distribution** — the spec's biggest gap is syllago's core value proposition (install, update, version, share)
3. **Format conversion** — hub-and-spoke conversion between provider-specific skill formats
4. **Security** — syllago could implement signing/verification at the distribution layer without requiring spec changes
5. **Activation** — syllago's loadout concept could include AGENTS.md integration that improves activation reliability
6. **Conformance** — syllago's test suite could become the de facto conformance test for the spec

The community is building exactly the tools syllago aims to be — craft, sklz, skillfold, add-skill are all partial solutions to problems syllago addresses holistically.
