# Community Signals & Technical Trust Strategy for Agent Skills

**Date:** 2026-04-01
**Status:** Revised after 3-round adversarial panel (file 16 → killed, this replaces it)
**Context:** Builds on research files 01-16, vouch research, web-of-trust history, viral adoption patterns, 3-round panel review

## What The Panel Killed

The v1 strategy (file 16) proposed a "social-first trust" model — vouches as the adoption driver, technical trust as invisible plumbing. The adversarial panel (6 personas, 3 rounds) unanimously rejected the framing:

1. **"Social trust" framing is dangerous.** Users WILL treat community signals as security guarantees regardless of architecture. Calling it "trust" accelerates this. (Miriam, all 6 agreed)
2. **No skin in the game for vouchers.** Without consequences for bad vouches, they're cheap talk with zero information content. (Dmitri)
3. **Vouch-to-version binding was unresolved.** A vouch for v1.0 that displays on v2.0 is a vulnerability, not a feature. (Rena, all 6 agreed)
4. **GitHub identity dependency excludes global devs.** "0 vouches" as alarming punishes underserved communities. (Kwame)
5. **Enterprise can't use social consensus for compliance.** Authority-based trust ≠ popularity-based trust. (Tomoko)
6. **Cold start plan was fantasy.** Assumed users/network effects that don't exist. (Jake)
7. **Let's Encrypt comparison was cherry-picked.** Let's Encrypt's social layer (domain validation) was automated, not manual. (Jake)

## What Survived

Five things the panel kept:

1. **Denouncement networks** — shared blocklists propagating faster than endorsement
2. **Content-hash-bound vouches** — vouch for a specific artifact (hash), not a person or version
3. **Behavioral scanning** — catches what community signals can't
4. **The `.td` format** — dead simple, parseable by anything
5. **"Community signals" framing** — helpful for discovery, explicitly NOT called trust

## Core Thesis (Revised)

The Agent Skills ecosystem has a security crisis (36.82% flawed, 76 confirmed malicious) and a discovery problem (145K+ skills, no quality signal). These are **two different problems requiring two different solutions:**

- **Security** is solved by technical verification: content hashing, behavioral scanning, denouncement feeds, and eventually signing. This is where real protection lives. No shortcuts.
- **Discovery** is aided by community signals: vouches, usage data, reasons. This helps developers FIND good skills. It does not make skills SAFE.

These two systems are complementary but must never be conflated — in architecture, in UI, or in language.

## Architecture

### Technical Verification (Security)

This is the load-bearing layer. It provides actual guarantees.

#### Content Hashing

Every skill gets a SHA-256 content hash computed at publish time. Verified at install time. If the hash doesn't match, the install fails. No override. This is the minimum security floor — it ships in v1.

The hash covers all files in the skill directory, with defined behavior for:
- Binary files (hashed as-is, no line-ending normalization)
- Symlinks (resolved if target is within skill directory, excluded if external)
- The hash field itself (blanked with exact regex before computation)

Reference implementation + 20 test vectors required before any tool can claim conformance.

#### Behavioral Scanning

Automated analysis of skill content for suspicious patterns:
- Shell commands (especially `curl | bash`, encoded payloads, environment variable exfiltration)
- Network requests (URLs, IP addresses, webhook endpoints)
- Obfuscated content (Base64 blobs, Unicode tag injection, zero-width characters)
- File system access patterns (reading outside skill directory, accessing credentials)

Scanning runs at `syllago install` and can be run standalone via `syllago scan`. Results are cached by content hash — scan once, result is valid for that exact content.

This is NOT a guarantee of safety. It catches known patterns. A sufficiently clever attacker can bypass it. But it catches the 36.82% of skills with obvious flaws.

#### Denouncement Network

The mechanism for propagating "this is bad" across the ecosystem. Protected against weaponization via a tiered model:

**Tier 1: Concern** (low stakes, triggers review)
- Anyone can file
- Requires a reason (free text, minimum 20 characters)
- Does NOT propagate to other registries
- Does NOT affect install flow — no user-visible impact
- Visible to the skill author, who can respond publicly
- Author response is recorded alongside the concern
- Think: "hey, I noticed something" — a GitHub issue, not a restraining order

**Tier 2: Denouncement** (high stakes, propagates after delay)
- Requires supporting evidence: scan results, specific file + line numbers, reproduction steps, or link to external analysis (e.g., Snyk report)
- Requires the denouncer to have an established identity (account age > 90 days, has published or vouched for at least 1 skill)
- **72-hour dispute window** before propagation to subscribing registries
- During dispute window: author can respond with counter-evidence. If disputed, propagation pauses until third-party review (registry maintainer OR 3 independent reviewers with established identities)
- After 72 hours with no dispute: propagates to subscribing registries automatically
- Think: npm security advisory with due process

**Tier 3: Emergency** (critical threat, immediate action)
- Triggered ONLY by automated detection: behavioral scanner malware signature match, known-malicious content hash, active exploit pattern
- Propagates immediately with no delay
- Skill is quarantined (install blocked, existing installs warned but not removed)
- Human review required within 48 hours to sustain quarantine
- If not confirmed by human review, quarantine auto-expires and author is notified
- Think: antivirus quarantine — act first, verify fast

**Protection against weaponized denouncement:**

| Attack | Defense |
|--------|---------|
| Competitor files false Tier 2 denouncement | Evidence requirement filters drive-by attacks. 72-hour delay gives author time to dispute. Denouncer's identity is public — weaponization creates a visible track record. |
| Coordinated denouncement campaign (multiple accounts) | Established identity requirement (90+ days, published content) raises the cost. Pattern detection: multiple denouncements from accounts with mutual follower relationships flagged for review. |
| Denouncement of a popular skill to cause chaos | Tier 2 only pauses installs during dispute window if the author doesn't respond. If the author disputes within 72 hours, propagation is held. Emergency (Tier 3) requires automated scanner match, not human filing. |
| Repeated frivolous concerns (Tier 1 spam) | Rate limiting: max 5 concerns per account per week. Tier 1 has no user-visible impact so the incentive to spam is low. |
| Denouncing to eliminate competition before a launch | All denouncements are public and attributable. Transparent history means the denouncer's pattern is visible. A history of denouncing competitors is itself a reputational signal. |

**Denouncement data format (`.td` compatible):**
```
# DENOUNCED.td
# Format: -platform:username  content_hash  reason
-github:badactor  sha256:abc123  Obfuscated shell commands exfiltrating env vars
-github:slopbot   sha256:def456  AI-generated skill with hardcoded credentials
```

Registries can subscribe to each other's denouncement feeds. A registry opts in to trust another registry's denouncements — this is explicit, not automatic. The `.td` format makes it trivially parseable.

### Community Signals (Discovery)

This is the human-readable layer. It helps developers find good skills. It is explicitly NOT a security system and must never be presented as one.

#### Content-Hash-Bound Vouches

A vouch is bound to a specific content hash, not a version number, not an author, not a skill name. This resolves the panel's #1 concern:

```
$ syllago vouch community/prompt-engineering --reason "I use this daily"
  ✓ Vouch recorded for community/prompt-engineering@sha256:abc123 (v2.1.0)
  Your identity: github:alice (verified via OIDC)
```

**What this means in practice:**
- When the author publishes v2.2.0 with a new hash, the vouch does NOT carry over
- The install prompt shows "Vouched for this version: 3" and "Vouched for previous versions: 9"
- Users can see that a skill has a long history of vouches across versions (continuity signal) while understanding that THIS specific version has fewer (recency signal)
- A compromised account that vouches for a malicious update is bounded — old vouches for old hashes are still valid, the new vouch is only for the new content

**Vouch cost function (addressing Dmitri's cheap talk critique):**
- When a skill you vouched for receives a Tier 2 denouncement, your vouch weight decreases
- When skills you vouch for remain healthy over time, your vouch weight increases
- Vouch weight is not displayed as a number — it affects ordering ("Vouched by @alice" appears higher than "Vouched by @newuser") but is not gameable because the algorithm isn't published
- This creates real but mild consequences: bad vouches make your future vouches less prominent

#### What Vouch Data Looks Like

```
# VOUCHES.td (in a registry or collection)
# Format: platform:username  content_hash  "reason"
github:alice  sha256:abc123  "I use this daily for all my Claude projects"
github:bob    sha256:abc123  "Solid activation triggers, works reliably"
github:carol  sha256:xyz789  "v2.0 is even better than v1"
```

#### What Users See

```
$ syllago install community/prompt-engineering

  prompt-engineering v2.1.0 by @swyx
  ─────────────────────────────
  Scan: ✓ clean (no suspicious patterns)
  Hash: sha256:abc123 (verified)
  ─────────────────────────────
  Community: 3 vouches for this version, 9 for previous
    "I use this daily" — @alice
    "Solid activation triggers" — @bob

  Install? [Y/n]
```

**Critical design choice:** Security signals (scan result, hash verification) appear ABOVE community signals (vouches). They are visually separated. The word "trust" does not appear anywhere. Scan and hash are presented as facts. Vouches are presented as opinions.

Compare with a skill that has issues:

```
$ syllago install sketchy/data-grabber

  data-grabber v1.0.0 by @unknown_account
  ─────────────────────────────
  Scan: ⚠ 2 warnings (shell commands detected, network access)
  Hash: sha256:def456 (verified)
  Concern: 1 open (filed by @security-researcher)
  ─────────────────────────────
  Community: 0 vouches

  Review scan details? [y/N]
```

The absence of vouches is presented neutrally — "0 vouches" not "⚠ 0 vouches." A new skill by a new developer with a clean scan should not feel alarming. It should feel new. (This addresses Kwame's concern about punishing underserved communities.)

#### Discovery, Not Trust

The community signals layer helps answer:
- "Which skills do experienced developers use for X?"
- "Has anyone actually used this skill and found it good?"
- "Is this skill actively used or abandoned?"

It does NOT answer:
- "Is this skill safe?"
- "Has this skill been audited?"
- "Can I deploy this in a regulated environment?"

This distinction must be maintained in:
- UI copy (never use the word "trust" for vouches)
- Documentation (explicit section explaining what vouches mean and don't mean)
- API design (vouch endpoints and security endpoints are separate, not mixed)

## What This Does NOT Include (Explicitly Scoped Out)

The panel and prior research identified these as important but separate concerns. They are not part of this strategy and should not be added to it:

1. **Cryptographic signing** — important, ships later, separate design doc
2. **Enterprise policy layers** — authority-based trust is a different system from community signals. Enterprise needs its own design that builds on the technical verification layer, NOT on community signals.
3. **Activation reliability** — the `mode`, `file_globs`, `commands` fields are a separate concern (per-skill metadata in SKILL.md frontmatter). Addressed by syllago's hub-and-spoke conversion, not by this strategy.
4. **Distribution / versioning** — collection manifests, dependency resolution, version pinning. Separate design.
5. **Spec governance** — the Agent Skills spec's governance problems are someone else's to solve. This strategy routes around the spec entirely.

## Identity

The v1 strategy was GitHub-only. The panel flagged this as exclusionary.

**Revised approach:** Support multiple identity providers from the start:
- GitHub (primary, largest developer population)
- GitLab (significant open source community)
- Additional providers added based on demand

Vouch and denouncement records use the `platform:username` format from vouch's `.td` spec, which already supports this:
```
github:alice
gitlab:bob
```

The system does NOT weight identities by platform social metrics (follower count, org membership). A vouch from `gitlab:developer_in_lagos` has the same base weight as a vouch from `github:silicon_valley_influencer`. Weight only changes based on vouch track record (the cost function described above).

## Implementation Priority

Based on what provides real security value vs what provides discovery value:

**Ship first (v1):**
1. Content hashing — computed at publish, verified at install
2. Behavioral scanning — `syllago scan` and auto-scan at install
3. Denouncement feed — `.td` format, tiered model, dispute process

These three provide genuine security improvement with zero dependency on community adoption. They work even if nobody ever vouches for anything.

**Ship second (v1.x):**
4. Content-hash-bound vouches — `syllago vouch`, vouch display in install flow
5. Vouch cost function — weight adjustment based on track record
6. Badge infrastructure — embeddable badges for READMEs

These provide discovery value. They depend on community adoption but don't affect security.

**Ship later (v2+):**
7. Cryptographic signing of vouch records
8. Transparency log for audit trail
9. Enterprise policy layer (separate design)
10. Cross-registry denouncement federation protocol

## How This Differs From v1 Strategy

| Aspect | v1 (Killed) | v2 (This Doc) |
|--------|------------|---------------|
| **Framing** | "Social-first trust" | "Technical security + community signals" |
| **What drives adoption** | Vouches (social proof) | Security features (scan, hash, denouncement) |
| **What vouches mean** | Trust signal | Discovery signal (explicitly not trust) |
| **Vouch target** | Person or version | Content hash (specific artifact) |
| **Vouch cost** | None (cheap talk) | Weight decreases when vouched skills get denounced |
| **"0 vouches" display** | Alarming (like HTTP without padlock) | Neutral (skill is new, not suspicious) |
| **Identity** | GitHub only | Multiple providers, no social metric weighting |
| **Enterprise story** | "Layer social trust, add enterprise later" | "Enterprise is a separate system built on technical verification" |
| **Ship order** | Social layer first | Technical security first, community signals second |
| **Denouncement** | Unspecified governance | Tiered model with dispute process and weaponization protection |

## Open Questions (Reduced)

Most open questions from v1 were resolved by the panel's feedback. Remaining:

1. **Denouncement federation protocol details** — how do registries subscribe to each other's feeds? Pull (polling) or push (webhooks)? What's the format for cross-registry denouncement records?
2. **Vouch cost function specifics** — what's the exact decay rate? How quickly does weight recover? Is the function public or opaque? (Panel split: Dmitri says publish it for accountability; Miriam says keep it opaque to prevent gaming.)
3. **Behavioral scanner scope** — what patterns are scanned? How is the pattern set updated? Who maintains it? Is there a community contribution model for scanner rules?
4. **Offline behavior** — when vouch/denouncement data is unavailable, what does the install flow show? Proposed: show "Community data unavailable (offline)" with no security impact — scan and hash verification still work locally.
