# Solutions: Governance

## Proposal 1: Minimal Visibility (dacharyc, #269)

Three deliberately minimal asks:

1. **Spec version identifier** — e.g. `spec-version: 2026.03`, changed on normative changes
2. **CHANGELOG.md** — normative changes only, bar: "would an implementer need to update their code?"
3. **Label for spec-altering PRs** — e.g. `spec-change` or `normative`

Explicitly NOT proposing: RFC process, governance board, breaking change review periods, community approval gates.

**Sharp criticism:** "The community has filed 48 proposals in Discussions. Of those, 1 has been answered." Contribution model "routes community input to a channel where it isn't actioned, while normative changes bypass that channel entirely."

**Maintainer response (@jonathanhefner):** Pushed back on examples but agreed in principle: "For the most part, no objections. I agree that spec changes should be visible."

**Status:** No CHANGELOG.md, version identifier, or PR labels implemented.

## Proposal 2: Foundation Governance (yordis, #59)

Statement, not proposal: "Anthropic decides the direction here, and until it is not part of something like Linux Foundation, or some sort of governance; it is a Claude Code or Anthropic thing."

No formal proposal to donate to AAIF exists in the repo.

## The MCP Template

MCP provides a clear governance model Agent Skills could follow:

**Before AAIF:** Anthropic-controlled (identical to Agent Skills now)

**After AAIF donation:**
- Specification Enhancement Proposal (SEP) process
- Working Groups for different protocol areas, regular open meetings
- Core Maintainers retain strategy, delegate domain authority
- Contributor Ladder (planned)
- Structured versioning (MCP 1.1, 2.0)
- 2026 roadmap lists "Governance Maturation" as top-4 priority

## AAIF (Agentic AI Foundation)

**Announced:** Dec 9, 2025 by Linux Foundation

**Projects:** MCP (Anthropic), AGENTS.md (OpenAI), goose (Block)

**Agent Skills is NOT an AAIF project.** Despite some third-party analysis listing it alongside MCP and AGENTS.md, no formal donation has occurred. Anthropic is a platinum AAIF member. Why Agent Skills hasn't followed MCP into AAIF is conspicuously unaddressed.

**Governance model:**
- Directed fund under Linux Foundation
- Governing Board chaired by David Nalley (AWS)
- Platinum: AWS, Anthropic, Block, Bloomberg, Cloudflare, Google, Microsoft, OpenAI
- 146+ member organizations
- No single member gets unilateral control
- Project inclusion based on adoption, quality, and community health

## AGENTS.md Convergence

**They are NOT merging.** Complementary:
- AGENTS.md = project context, always loaded (passive)
- Agent Skills = domain knowledge, on-demand (active/progressive disclosure)

No one is working on formal alignment. Both under broad AAIF umbrella (though Agent Skills' status unclear).

## Current CONTRIBUTING.md Process

- Documentation improvements: welcome via PR
- Bug reports: open an issue
- Proposals/questions: start a Discussion (NOT issues)
- Reference library (skills-ref/): **not accepting contributions**
- Major architectural changes: **not accepting** ("still iterating on core specification")
- AI disclosure required
- Logo/ecosystem listings: reviewed by "the Anthropic team"
- No RFC process, no review timeline, no escalation path

Quote: "We maintain a high bar for additions to the spec -- it is much easier to add things to a specification than to remove them."

## The Asymmetry

MCP (Anthropic-originated) has: formal SEP process, working groups, contributor ladder, structured versioning under AAIF.

Agent Skills (also Anthropic-originated) has: none of the above. 57 community discussions, unversioned spec, no changelog, proposals go unactioned.

The asymmetry is striking and publicly unexplained.
