# Adversarial Review Panel: Metadata Convention v0.1

**Date:** 2026-04-01
**Format:** 6 adversarial personas reviewing metadata_convention.md
**Framing:** Author has no clout, no fame, just access to an LLM like everyone else

---

## Panel Members

| Persona | Role | Disposition |
|---------|------|-------------|
| **Jordan** | W3C/IETF spec maintainer, 15 years | Skeptical of newcomers, respects only things that ship |
| **Dr. Priya Anand** | Ex-Google Project Zero, AI security consultancy | Zero patience for security theater |
| **Marcus Chen** | OSS maintainer, 50K+ stars, adopted Agent Skills | Cynical about specs from non-shippers |
| **Sarah Park** | VP Engineering, Fortune 500 finserv, 3K developers | Needs governance, compliance, audit trails |
| **Kai Nakamura** | Ex-DeepMind, autonomous agent infrastructure | Thinks SKILL.md is a transitional artifact |
| **Aisha Williams** | Senior dev advocate, maintains 30+ skills daily | Voice of the actual user |

---

## JORDAN — Spec Maintainer (15 years W3C/IETF)

### What's Actually Good

- **Sidecar file decision** is the best architectural call in the document. "No convention-based discovery" rule shows someone who understands that implicit behavior is where interop dies.
- **Content hash algorithm** specified with unusual care — byte-level replacement, CRLF normalization, UTF-8 paths. "I have personally seen W3C specifications ship without CRLF normalization and spend three years fixing it."
- **Self-attestation disclosure** (asymmetric enforcement) is genuinely clever.
- **Behavioral research** is real — 12 agents, 14 checks, validated against source code.

### What's Fatally Flawed

- **Unsigned attestation is almost meaningless.** "An unsigned JSON file in a git repository is just... a claim stored in a different location." Aviation doesn't ship aircraft with "we will add the bolts in v2." **Ship attestation and signing together or don't ship attestation at all.**
- **Graduation threshold is too low and circular.** 3 agents + 2 registries where syllago is one tool and registries don't exist yet. W3C requires two independent interoperable implementations. OpenTelemetry requires production deployments at scale.

### What Should Be Thrown Out

- **Durability.** 1/12 agents has compaction protection. "Specifying a field that 11/12 agents will silently ignore is not a convention — it is wishful thinking." Remove it, bring back when 3 agents implement it.
- **Tags.** Freeform unconstrained tags become SEO spam within months. Every major registry learned this. Either add limits in v0.1 or remove.

### What's Missing

- **Conformance test suite.** Without it, "MUST" is a polite suggestion.
- **Error handling.** What happens when hash fails? When expectations aren't met? Recovery behavior unspecified.
- **Version negotiation.** Exact-string-match on `metadata_spec` means v0.1 agent ignores ALL v0.2 fields. "Content negotiation is a solved problem."
- **Unified threat model.** Security concerns scattered, never presented as attack vectors + mitigations.

### Where Lack of Experience Shows

- **Simulated persona reviews are not peer review.** "Running your draft through five AI-generated personas is not peer review. It is autocomplete with extra steps."
- **"Companion convention" framing** is a bid to become the real spec, staged as a polite companion. Should be honest about what it is.

### What Would Make Him Take It Seriously

1. Ship the reference implementation with real content flowing through it
2. Get real external review from package registry maintainers (npm, crates.io, PyPI)
3. Solve the hard problems before publishing (signing, version negotiation, error recovery)

### Cross-Industry Insight

"Look at pharmaceutical regulation: every ingredient has a chain of custody with cryptographic seals at each handoff, not 'we will add the seals later.' Look at aviation safety: you don't get to ship the airframe and add the stress analysis in the next version."

**Bottom line:** "This is the most thorough first-draft companion spec I have seen from someone without standards experience. But it has the characteristic weakness of a solo author working with AI tools: it is extremely well-polished on the surface while leaving the hardest engineering problems unresolved underneath. Polish is not maturity."

---

## DR. PRIYA ANAND — Security Researcher (Ex-Project Zero)

### What's Sound Security Engineering

- **Content hash algorithm** is well-engineered. "Most specs get this wrong on the first pass."
- **Immutable versions** — table stakes, glad it's normative.
- **"Attestation means authenticity, not safety"** — "the single most important sentence in the proposal." Mirrors FDA: approved facility ≠ safe pill.
- **Asymmetric denouncement propagation** in v0.3 roadmap is sophisticated. Every supply chain (diamonds, aviation, pharma) propagates revocation faster than endorsement.
- **Three-tier trust model** mirrors FDA DSCSA: serialization → transaction verification → system interoperability.

### What's Security Theater

- **Unsigned attestation records.** "If I compromise a registry's git access, I can forge any attestation record I want." Compare to FAA 8130-3 form: the paper is worthless without the inspector's signature. "v0.1 provides structured provenance claims, not attestations. Call them what they are."
- **Self-attestation hostname heuristic** catches honest mistakes, not adversaries. "This is the Kimberley Process problem: works when participants play by rules, fails when it matters."

### Attack Vectors Missed

1. **Typosquatting and name confusion.** No skill name reservation. `code-review` vs `code_review` vs `codereview`.
2. **Dependency confusion for expectations.** A skill requiring `git >= 99.0` could prompt installing attacker binary.
3. **Post-attestation content modification.** No integrity monitoring after installation. Aviation has "back to birth" traceability.
4. **Prompt injection through metadata fields.** Author names, tags rendered by tooling — are they sanitized?

### Demands Before Production

1. Don't ship "attestation" in any UI until v0.2 signing lands
2. Normative MUST for script_hash verification before execution
3. Skill naming policy addressing typosquatting
4. Post-install integrity checking (`syllago audit`)
5. Sanitize all metadata strings before rendering

**Bottom line:** "Better security engineering than I expected from a non-security-engineer. Fix the terminology for v0.1, ship signing in v0.2 on the promised timeline, and make script_hash verification normative. Do those three things and this is a credible foundation."

---

## MARCUS CHEN — OSS Maintainer (50K+ stars)

### Implementation Cost

- Basic sidecar + triggers: 2-3 days
- Full provenance + attestation: another week
- The `~>` semver operator requires a bespoke parser — no standard library in Go, Rust, or Python implements it
- Every optional field with structured sub-objects is a new class of parse error

### The "Optional" Lie

"Registries start requiring version and source_repo. Then popular registries all require it. Then 'optional' means 'optional if you never want anyone to find your skill.'" The spec hints at this already. Just own it.

### What He Actually Needs

Three things: **`mode`**, **`activation_file_globs`**, and **`commands`**. That's it. Implementable in a day. Solves 80% of activation reliability.

"The `mode` field is genuinely the highest-value thing in this entire proposal."

### What He'd Strip

Kill `expectations` (no verification mechanism), `durability` (1 agent supports it), `supported_agents` (goes stale), `tags` (registries impose their own). That leaves: `metadata_spec`, `provenance`, `triggers`, `attestation`. Four sections instead of nine.

### The ID3 Tag Lesson

"SKILL.md is ID3v1 — minimal, universally supported. This proposal is heading toward ID3v2 — extensible, powerful, likely to be implemented inconsistently for years. The sidecar pattern is smarter than embedding, but the fragmentation risk is the same."

Food industry model: Nutrition Facts labels are rigid — you can't add custom fields. The rigidity is a feature.

**Bottom line:** "Ship the minimum: `mode`, `activation_file_globs`, `commands`, `version`, `source_repo`, `content_hash`, and `authors`. Seven fields across two sections. Get adoption. Then grow."

---

## SARAH PARK — VP Engineering, Fortune 500

### Does It Solve Governance?

Partially. Provenance model (version pinning, content hashing, immutable versions, source commit binding) answers the CISO's question: "what version of which skill is running on which machine?" Content hash algorithm is well-specified enough for compliance tooling.

### What's Missing for Enterprise

1. **Approval workflow hooks.** Convention tracks provenance and attestation but not authorization. Pharmaceutical GxP requires documented approval chains. Need custom claim types (extensible claims array enables this).
2. **SBOM integration.** `derived_from` is informed by CycloneDX/SPDX but no defined mapping. Need normative CycloneDX/SPDX equivalence appendix.
3. **Policy-as-code support.** Need to express "no skills with `mode: always` unless approved by security." Trust states are right primitives but no policy expression layer.

### Attestation Assessment

- "Honest, which is more important than being complete"
- Self-attestation disclosure is clever
- Expiry model (mandatory) is something she wishes more ecosystems had. Mirrors SWIFT re-certification, DFARS compliance.
- But unsigned = advisory only, can't present to auditors

### Can Build Approval Workflow?

Yes. Extensible claims array + sidecar pattern provide right extension points. Internal workflow: verify → check policy → route to approver → issue corporate attestation.

### CISO's Verdict

"Provenance and hashing are solid. Immutable versions close the left-pad vector. But need signing before regulated workloads, and denouncement propagation before trusting ecosystem signals."

**Bottom line:** "Ship v0.1. Prioritize signing (v0.2) over federation (v0.3). Add CycloneDX/SPDX mapping appendix — that single addition would cut my integration timeline in half."

---

## KAI NAKAMURA — AI-Native Futurist (Ex-DeepMind)

### What Survives 2028

- **Provenance and attestation.** Content hashing, source commit binding, attestation chain are durable regardless of who produces artifacts.
- **`derived_from` vocabulary.** "More forward-looking than the authors may realize." When agents generate skills by composing existing ones, the lineage graph becomes critical.

### What Doesn't Survive

- **Triggers system.** "The most 2026 artifact in the proposal." Activation reliability climbing toward 95%+ with description alone. File globs will feel like hardcoded if-statements in a world that reasons about capability matching.
- **Token efficiency argument driving the sidecar.** Context windows doubled 200K→1M in 12 months. 4 lines vs 34 lines will be noise within 18 months.

### What an AI-Native Format Would Look Like

1. **Self-describing through embeddings, not keywords.** Dense vector representation instead of `tags: ["testing"]`. Skill matching via cosine similarity.
2. **Capability interfaces, not descriptions.** Typed inputs/outputs, explicit contracts (MCP already shows this).
3. **Composability primitives.** Like immune system V(D)J recombination — small capability fragments agents recombine, not monolithic documents.

### Where It Thinks Too Small

- `expectations` solves yesterday's problem. Agents are moving toward environment-aware execution.
- Should think about **capability negotiation protocols** — agents and skills as participants in a capability marketplace. Mechanism design from game theory.

**Bottom line:** "The proposal is good infrastructure for 2026. The question is whether you are building a bridge or a destination. If bridge, ship quickly and plan for obsolescence. If destination, the format needs to be fundamentally more machine-native than YAML sidecar files."

---

## AISHA WILLIAMS — Daily Skill Author (30+ skills)

### Does It Make Life Better?

Better, conditionally. Addresses activation and portability without breaking existing skills. But the surface area is intimidating — "this reads like a spec designed for registries and enterprise security teams, not for someone who writes a skill on Friday afternoon."

### Triggers: The Section That Matters

"This is the section that made me lean forward." `activation_file_globs` and `mode` would be genuine quality-of-life improvements. "The `mode` field alone would save me hours."

### Two Files Per Skill

Not a dealbreaker IF tooling generates the sidecar. "If I have to hand-write SKILL.meta.yaml, I will resent it. If `syllago init` scaffolds it, it is invisible plumbing."

**The design principle that should be front and center:** The convention is designed to be tool-populated, not hand-authored.

### Would She Fill It Out?

By hand: no, except triggers. With tooling: yes, 2 minutes per skill.

### Her Minimum

5 fields: `triggers.mode`, `triggers.activation_file_globs`, `triggers.commands`, `provenance.version`, `provenance.source_repo`.

### Key Concerns

- Attestation is premature for her use case (15-person team sharing internally)
- 8 unresolved open questions suggest another round needed before Draft
- Convention versioning (open question 2) is a real problem — adding a v0.2 field breaks all v0.1 tooling

**Bottom line:** "The triggers section alone justifies this convention's existence. Ship triggers and basic provenance. Make the tooling generate everything. Separate registry/enterprise concerns so skill authors aren't intimidated by a 440-line spec."

---

## CROSS-PANEL CONSENSUS

### Universal Agreement (6/6)

- **`mode` field is the single highest-value thing.** Every persona wants it.
- **Content hash algorithm is well-engineered.** Even the security researcher and spec maintainer approve.
- **The sidecar pattern is architecturally sound** for current constraints.
- **The research basis is genuine** and unusually thorough for a first-time spec.

### Strong Agreement (4-5/6)

- **Triggers are the immediate adoption driver** (all except Kai, who thinks they'll be obsolete)
- **Unsigned attestation should not be called "attestation" in v0.1** (Jordan, Priya, Sarah agree; Marcus doesn't care; Aisha doesn't need it yet)
- **Strip to minimum for v0.1** — provenance + triggers core, everything else optional/deferred

### The Split

- **Enterprise needs** (Sarah) vs **keep it simple** (Marcus, Aisha) — attestation, expectations, SBOM integration
- **Bridge vs destination** (Kai) — is the sidecar YAML pattern forward-compatible with AI-native futures?
- **Ship signing now** (Jordan, Priya) vs **ship v0.1 fast, signing in v0.2** (Sarah, Marcus, Aisha)

### Ideas From Outside Software

| Industry | Insight | Applicable To |
|----------|---------|---------------|
| **Pharmaceutical** (FDA DSCSA) | Serialization → verification → interoperability, each layer deployed years apart | Trust layer model matches perfectly |
| **Aviation** (FAA 8130-3) | Unsigned forms are worthless without inspector signature | Attestation needs signing |
| **Food industry** (Nutrition Facts) | Rigid format, no custom fields = universal adoption | Converge on minimal rigid set, not extensible framework |
| **Music** (ID3 tags) | v1→v2 took a decade, still fragmented | Risk of overengineering the extension |
| **Photography** (Lightroom XMP) | Sidecar works when auto-generated, not hand-authored | Tooling is prerequisite, not nice-to-have |
| **Recipes** (Schema.org JSON-LD) | Only works because WordPress plugins generate it | Same as XMP — tooling generates, humans don't |
| **Diamonds** (Kimberley Process) | Works when participants play by rules, fails when it matters | Hostname heuristic catches honest mistakes, not adversaries |
| **Biology** (immune system V(D)J) | Combinatorial recombination from finite alphabet | Composable skill fragments > monolithic documents |
| **Game theory** (mechanism design) | Capability marketplace with bid/ask | Future: agents negotiate capabilities dynamically |
| **Manufacturing** (ISO 28000) | Verification contemporaneous with claim | Don't defer signing |

### The Minimum Viable Convention (Panel Consensus)

If forced to agree on what ships first:

1. `metadata_spec` — convention version identifier
2. `provenance.version` — semver
3. `provenance.source_repo` — canonical source
4. `provenance.content_hash` — integrity verification
5. `provenance.authors` — attribution
6. `triggers.mode` — auto/manual/always
7. `triggers.activation_file_globs` — deterministic activation
8. `triggers.commands` — explicit invocation

**8 fields. Two sections. Ship it.**
