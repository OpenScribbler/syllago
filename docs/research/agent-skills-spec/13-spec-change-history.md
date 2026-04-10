# Spec Change History: Has the Community Ever Changed the Spec?

**Date researched:** 2026-04-01

## The Answer: No.

Zero community contributions have been adopted into the formal specification. The spec has been semantically unchanged since its initial publication on 2025-12-18.

## Spec File (specification.mdx) Commit History

12 total commits to the specification file:

| Date | Author | PR | What Changed |
|------|--------|----|-------------|
| 2025-12-18 | Eric Harmeling (Anthropic) | #1 | Initial spec creation |
| 2025-12-18 | Eric Harmeling (Anthropic) | #3 | Document reference SDK |
| 2026-03-06 | Jonathan Hefner (maintainer) | #216 | Formatting cleanup, code annotations, directory tree. **No normative changes.** |
| 2026-03-10 | **Koichi ITO** (community — ESM, Inc.) | #222 | Fix example `description` to include "when to use" guidance. **1 line in 3 files. Example wording, not normative.** |
| 2026-03-16 | Jonathan Hefner (maintainer) | #253 | Add runtime version example to `compatibility` field |

All other commits are formatting, infrastructure, or Anthropic-internal.

**Last normative spec change:** Never. No new fields, no structural changes, no community-proposed features merged.

**Releases published:** Zero. No version numbers, no tags, no changelog.

## The Contribution Funnel

```
Community files spec PR
    ↓
Closed with "please open a Discussion per CONTRIBUTING.md"
    ↓
Discussion sits without formal acceptance or rejection
    ↓
Nothing happens
```

### Community Spec PRs — All Closed Without Merge

| PR | Author | Proposal | Closed | Response |
|----|--------|----------|--------|----------|
| #13 | edu-ap | Type-safe skill composition | Closed | Never merged |
| #29 | edu-ap | Type-safe skill composition (attempt 2) | Closed | Never merged |
| #52 | edu-ap | Type-safe skill composition (attempt 3) | Closed | Never merged |
| #89 | edu-ap | Skill composition (attempt 4) | Still open | No response |
| #120 | vmalyi | Universal skill directory (`~/.skills`) | Closed 2026-02-13 | No comments |
| #171 | orlyjamie | `capabilities` field | Closed 2026-02-22 | → Discussion |
| #172 | iwasrobbed | `credentials` field | Closed 2026-02-22 | → Discussion |
| #174 | douglascamata | `.agents/skills/` in spec | **Still open** | No merge despite 15+ tools adopting |
| #228 | voodootikigod | `product-version` field | Closed 2026-03-12 | → Discussion |
| #231 | voodootikigod | Feature flags | Closed 2026-03-12 | → Discussion |
| #277 | jcmuller | Claude Code fields in validator | Closed 2026-03-30 | "Requires significant discussion" |
| #281 | yejiming | YAML lists for `allowed-tools` | Closed 2026-03-30 | "Requires significant discussion" |
| #267 | byronxlg | Ecosystem tools page | Closed 2026-03-22 | No details |

### Discussions: The Proposal Graveyard

| Metric | Count |
|--------|-------|
| Total discussions filed | 57 |
| Formally "answered" (Q&A mechanism) | **1** |
| With maintainer responses | 22 |
| With zero comments | 11 |
| Community-only comments (no maintainer) | 23 |
| Resulting in spec changes | **0** |

Note: The Ideas category was configured as non-answerable in GitHub's discussion system, so discussions structurally *cannot* be marked "answered." The "1 answered" stat refers to the only Q&A-category discussion.

## What Community CAN Contribute

Of 26 community-authored merged PRs, **24 are logo additions** to the marketing carousel. The community can declare adoption. They cannot change the spec.

| Category | Community PRs Merged |
|----------|---------------------|
| Logo carousel additions | 24 |
| Spec example wording fix | 1 (koic #222) |
| Docs/guides contributions | 2 |
| Substantive spec changes | **0** |

## Who Controls the Spec

| Person | Affiliation | Role | Merged PRs |
|--------|------------|------|------------|
| jonathanhefner | agentskills org MEMBER | Sole active maintainer, most discussion responses, all PR merges | 20 |
| ericharmeling | Anthropic (listed) | Created repo + initial docs | 7 |
| maheshmurag | Product @ Anthropic | LICENSE, minor edits | 2 |
| klazuka | Anthropic (listed) | Added skills-ref library | 2 |

Total Anthropic-affiliated merged PRs: 29 of 55 (53%). Remaining 26 are almost all logos.

## CONTRIBUTING.md Gatekeeping (verbatim quotes)

> "We maintain a high bar for additions to the spec -- it is much easier to add things to a specification than to remove them."

> "We're still iterating on the core specification. Large-scale redesigns are premature."

> "We're still determining the direction for the reference library and are not accepting code contributions to it at this time."

## The MCP Asymmetry

MCP (also Anthropic-originated) was donated to AAIF (Linux Foundation) and now has:
- Specification Enhancement Proposal (SEP) process
- Working Groups with regular open meetings
- Contributor Ladder (planned)
- Structured versioning (MCP 1.1, 2.0)
- 146+ member organizations

Agent Skills has: none of the above. Why the difference has not been publicly explained by Anthropic.

## Implications

The spec is functionally a **read-only Anthropic document** presented as an open standard. The community can:
- Adopt it (add logos) ✓
- Discuss it (file proposals) ✓
- Change it ✗

This means:
1. Solutions to the top 5 pain points cannot come from the spec itself under current governance
2. The ecosystem is already routing around the spec's gaps (Vercel skills, Paks, Grith.ai, etc.)
3. Any tool building on the spec must solve gaps at its own layer rather than waiting for spec evolution
4. The governance bottleneck may be the single biggest strategic risk to the spec's long-term relevance
