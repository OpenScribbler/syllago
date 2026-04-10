# Install Safety Strategy for Syllago

**Date:** 2026-04-01
**Status:** Final — replaces files 16 (killed) and 17 (superseded)
**Context:** Outcome of multi-session research, 6-approach strategy evaluation, two adversarial panels, vouch/web-of-trust research, security scanning landscape research

## Design Principle

**No human involvement, but human judgment.**

The system presents facts computed from artifacts and git history. No governance layer, no social trust system, no feeds to maintain, no governing body. The human decides based on the facts. Nothing is gameable except through the extraordinary cost of fabricating git history or compromising platform APIs.

## What We're NOT Building

These were explored and explicitly rejected through adversarial review:

| Rejected | Why |
|----------|-----|
| **Vouch/endorsement system** | Cheap talk without a cost function. Users conflate social signals with security guarantees. Requires governance nobody will operate. (Panel Round 2: unanimous) |
| **Denouncement network** | Requires a governing body to adjudicate disputes, maintain feeds, handle appeals. Static files don't update themselves. Propagation requires infrastructure nobody maintains. |
| **New competing spec** | 33+ tools adopted the current spec. Network effects are unbeatable. Fatal verdict from all 8 adversarial personas across two panels. |
| **Extending the existing spec** | Zero community contributions merged in 3.5 months. 48 proposals filed, 1 answered. Governance is non-functional. |
| **Social trust layer** | "Every reputation system gets gamed exactly like Yelp." Users treat any signal labeled "trust" as a security guarantee regardless of disclaimers. (Dr. Miriam, Trust Researcher) |
| **Community-maintained feeds** | "A .td file in a repo doesn't update itself." Any system requiring ongoing human maintenance dies when nobody maintains it. |

## What We ARE Building

Five capabilities, all computed from facts, all automatic, zero governance:

### 1. Quarantine Fetch (Bubble Wrap Isolation)

Skills are fetched to an isolated staging directory before installation. Files cannot execute, cannot make network calls, cannot access the filesystem. The existing `sandbox.StagingDir` infrastructure handles this — random ID, auto-cleanup of stale sessions, restricted permissions.

**Flow:**
```
1. Fetch skill files to staging dir (quarantined)
2. Compute content hash on quarantined files
3. Run behavioral scan on quarantined files
4. Read git metadata from cloned repo
5. Query platform API (if recognized host, cached)
6. Present results — user decides
7a. Approve → move from staging to skill directory (installed)
7b. Decline → staging.Cleanup() (files deleted, never installed)
```

Files are never placed in a location where an AI agent could discover or execute them until the user explicitly approves. The staging directory is the existing `StagingDir` with its random hex ID, XDG_RUNTIME_DIR preference, and `CleanStale()` on startup.

### 2. Content Hash (Integrity Verification)

SHA-256 hash computed at publish time, verified at install time. If the hash doesn't match, the install fails. No override. Mathematical fact, not an opinion.

**Covers:** All files in the skill directory.
**Edge cases:**
- Binary files: hashed as-is, no line-ending normalization
- Symlinks: resolved if target is within skill directory, excluded if external
- The hash field itself: blanked with exact regex before computation

A reference implementation with 20+ test vectors is required before any tool can claim conformance. This ensures that the 33+ tools implementing the spec all compute the same hash for the same content.

### 3. Behavioral Scanner (Pattern Detection)

Native Go pattern matching on file contents. Zero external dependencies. Deterministic — same files always produce the same results. Runs on quarantined files before installation.

#### What It Detects

**Categories (published in docs):**
- Shell commands with dangerous execution patterns
- Encoded or obfuscated content
- References to sensitive file paths or environment variables
- Hardcoded network endpoints and suspicious URLs
- Unicode manipulation techniques (Tag characters, zero-width, bidirectional)
- Known prompt injection patterns

**Specific patterns (in source code, not published as a checklist):**
- Unicode Tag characters (U+E0000-U+E007F) — invisible to editors, interpreted by LLMs
- Zero-width characters (U+200B, U+200C, U+200D, U+FEFF)
- Bidirectional text markers (U+202A-U+202E, U+2066-U+2069)
- Base64-encoded shell commands
- `curl | bash`, `wget | sh`, `eval` execution patterns
- Hardcoded API key formats (AWS, GitHub, Stripe, etc.)
- Environment variable exfiltration patterns ($AWS_*, $GITHUB_TOKEN, etc.)
- File path traversal (../../.ssh/, ~/.aws/credentials)
- Known prompt injection phrases ("ignore previous instructions", "you are now", etc.)

**Transparency model:** Publish categories (what kinds of things we scan for), not exact patterns (the specific regexes). Same model as antivirus vendors — users know the scope of protection without getting a bypass checklist. Categories documented at syllago.dev/scan.

#### What It Does NOT Detect (published honestly)

- Novel prompt injection written as natural-sounding instructions
- Context-dependent instructions (legitimate vs malicious credential access)
- Subtle behavioral manipulation
- Zero-day techniques not yet in the pattern set

#### UX Language

**Critical design choice:** Never say "clean" or "safe." These create false confidence.

When no issues found:
```
Scan:   no known issues detected ⁱ

ⁱ Scan checks for known malicious patterns.
  It does not guarantee safety. See: syllago.dev/scan
```

When issues found (with file:line references):
```
Scan:   ⚠ 3 issues found
        · shell command: curl with piped execution (setup.sh:14)
        · network: hardcoded IP address (SKILL.md:47)
        · encoding: base64 blob (assets/config.dat)
```

#### Optional: YARA Deep Scan

If the `yara` CLI is installed on the system, syllago shells out to it with embedded rule files for deeper pattern matching. Same external-tool pattern as bubblewrap — optional enhancement, graceful degradation without it.

YARA rules are embedded in the syllago binary via `//go:embed`, written to a temp dir at scan time, passed to the `yara` CLI. Rules are ours, engine is theirs.

```
$ syllago install github.com/someone/some-skill

  Scan:   no known issues detected ⁱ (basic)
          yara not installed — install for deeper scanning

  ⁱ See: syllago.dev/scan
```

vs. with YARA:

```
  Scan:   no known issues detected ⁱ (basic + yara)
```

#### Optional: External Scanner Integration

For users who want ML-based detection:
- Snyk agent-scan (`uvx snyk-agent-scan@latest`) — 91% recall, 0% false positive on malicious skills
- SafeDep vet (`vet scan --agent-skill`) — Go-native, Apache-2.0

These are opt-in integrations, not dependencies. Syllago detects if they're available and offers to use them.

### 4. Health Signals (Registry + Content Level)

Facts computed at install/sync time from the repo and (optionally) the hosting platform API. Cached locally so they're always visible in the TUI and CLI without re-fetching.

#### Signal Design Principles

1. **Show individual signals, not a composite score.** Every mature system (OpenSSF Scorecard, Socket.dev, crates.io) converged on this. Composite scores get Goodharted — attackers optimize for the formula. Individual signals let humans spot inconsistencies.

2. **Correlation detects manipulation.** No single metric matters much. What matters is whether metrics agree: 500 stars + 0 issues + 1 contributor (5-day account) = screaming manipulation. 50 stars + 20 closed issues + 5 contributors (2+ year accounts) = genuine small project.

3. **Two-tier display: registry + content.** "Healthy repo ≠ healthy content." A registry with 50 skills committed to yesterday could have 30 skills untouched for 18 months. Always show both levels.

4. **Content type determines scan depth, not display prominence.** Different types have different threat models but results are presented uniformly.

#### Signal Tiers by Gaming Resistance

**Tier 1: Hard to fake (server-side, graph-based — highest value)**

| Signal | Why Strong | Source |
|--------|-----------|--------|
| Repo creation date | Server-side timestamp, unforgeable | Platform API |
| Contributor account ages | Can't backdate account creation | Platform API per contributor |
| Cross-project contributor activity | Would need real PRs merged elsewhere | Platform API |
| Issue/PR with external participants | Server-side timestamps, real engagement | Platform API |
| Release cadence | Server-side publish dates | Platform API |

**Tier 2: Moderate to fake (require sustained effort over months)**

| Signal | Why Moderate | Source |
|--------|-------------|--------|
| Issue close rate + median response time | Requires ongoing human engagement | Platform API |
| PR review depth | Multi-party discussions expensive to fake | Platform API |
| CI/CD passing status | Requires actual test infrastructure | Platform API |
| Test file presence | Writing tests for malicious code is wasted attacker effort | Git tree |

**Tier 3: Easy to fake (supplementary context only — never rely on alone)**

| Signal | Cost to Game |
|--------|-------------|
| Stars | $2-10 per 100 via services |
| Fork count | Free (single API call per bot) |
| Commit count | Trivial script, backdatable |
| README quality | 5 minutes with LLM |
| Raw contributor count | Sock puppets with trivial commits |

#### Registry-Level Signals

These apply to the repo/registry as a whole:

| Signal | Source | Gaming Resistance |
|--------|--------|-------------------|
| Repo creation date | Platform API (server-side) | Hard |
| License | Git tree / API | Easy to add, absence is a flag |
| Total contributor count | `git shortlog -sn` | Moderate (inflate with sock puppets) |
| Contributor account ages (avg top 3) | Platform API | Hard |
| Cross-project contributor activity | Platform API | Hard |
| Last repo commit | `git log -1` | Easy to fake but pointless if content is stale |
| Issue close rate | Platform API | Moderate |
| Median issue response time | Platform API | Moderate |
| PR merge rate | Platform API | Moderate |
| Stars | Platform API | Easy — supplementary only |
| Archived status | Platform API | Hard (owner action) |
| Has SECURITY.md | Git tree | Easy to add, absence is a flag |
| CI/CD presence + status | Platform API | Moderate |

#### Content-Level Signals (Per-Item)

These are computed per installed content item. This is the critical layer — it prevents "healthy repo = healthy content" false confidence.

| Signal | Source | Why Per-Item |
|--------|--------|-------------|
| Last modified date | `git log` for specific path | THE critical per-item signal |
| Author (git blame) | `git blame` / `git log` | May differ from repo owner |
| Content hash | SHA-256 of files | Integrity of THIS artifact |
| Scan results | Behavioral scanner | What THIS content contains |
| Content type risk tier | File structure analysis | Different types = different threats |
| File count + has scripts | Directory listing | Scripts = higher risk tier |

#### Content-Type Risk Tiers

Different content types get different scan intensity:

| Tier | Types | Risk | Scan Depth | Freshness Interpretation |
|------|-------|------|-----------|------------------------|
| **1: Executable** | Hooks, Skills with scripts | Highest — runs on machine | Full behavioral scan, script analysis, URL validation | Staleness is a concern — target tools evolve |
| **2: External-pointing** | MCP configs | Medium-high — redirects traffic | URL validation, domain checks | Is the server still running? |
| **3: Instructional** | Skills (markdown), Commands, Agents | Medium — prompt injection risk | Content scan for injection, Unicode tricks | Depends on what's referenced |
| **4: Passive** | Rules | Lower but highest exposure (always loaded) | Content hash, change frequency | Stability is generally GOOD — frequent changes are suspicious |

#### Computed / Derived Signals

These are the most valuable — gaps between registry and content health:

| Derived Signal | Computation | What It Means |
|---------------|-------------|---------------|
| **Stale content in active repo** | Repo committed to within 30d, content not touched in 12+ months | "Registry is active but this item appears unmaintained" |
| **Author divergence** | Content author ≠ repo owner | Not a warning — just visibility. Who actually wrote this? |
| **Bus factor** | Min contributors accounting for 50% of commits | 1 = single point of failure risk |
| **Health indicator** | Composite of last commit + issue close rate + activity | 🟢 Active / 🟡 Slow / 🔴 Dormant / ⚫ Archived |
| **Star/engagement mismatch** | High stars but zero issues/PRs or very new contributors | Potential manipulation signal |

#### Anti-Pattern Flags

Any single one of these warrants caution in the display:

- Repo < 7 days old
- All contributors have accounts < 30 days old
- Name is Levenshtein distance ≤ 2 from a popular package (typosquatting)
- High stars but zero issues/PRs (manipulation signal)
- Content contains executable patterns but is labeled as markdown-only

#### Cached Signal Store

Signals are computed at install/sync time and cached locally so they're always available in the TUI and CLI without network calls.

**When signals are collected:**
- `syllago install` — computed during quarantine fetch, stored on successful install
- `syllago sync` / `syllago update` — refreshed for all installed content from their source registries
- `syllago scan` — rescan installed content, update scan results
- TUI registry refresh (`R` key) — refreshes registry-level signals

**What gets cached (per installed content item):**

```json
{
  "content_id": "github.com/acme-corp/skill-collection/sql-helpers",
  "scanned_at": "2026-04-01T14:32:00Z",
  "content_hash": "sha256:abc123",
  "scan": {
    "scanner_version": "0.5.0",
    "issues": [],
    "yara_available": false
  },
  "content": {
    "type": "skill",
    "risk_tier": 1,
    "has_scripts": true,
    "last_modified": "2026-03-29T10:00:00Z",
    "last_author": "github:alice",
    "file_count": 3
  },
  "registry": {
    "source": "github.com/acme-corp/skill-collection",
    "repo_created": "2024-02-15T00:00:00Z",
    "contributors": 8,
    "contributor_avg_account_age_days": 1420,
    "last_commit": "2026-03-31T16:00:00Z",
    "stars": 342,
    "open_issues": 12,
    "closed_issues": 89,
    "issue_close_rate": 0.87,
    "license": "MIT",
    "archived": false,
    "has_security_md": true,
    "health": "active"
  },
  "derived": {
    "stale_in_active_repo": false,
    "author_is_repo_owner": true,
    "bus_factor": 3,
    "star_engagement_mismatch": false
  },
  "platform": "github"
}
```

**Where it lives:** `~/.local/share/syllago/signals/` (or XDG_DATA_HOME), one JSON file per installed content item, keyed by content ID.

**Staleness:** Signals show `scanned_at` date in all displays. Signals older than 30 days show a note: "(last checked 45 days ago — run `syllago sync` to refresh)".

**Refresh follows the existing `registryAutoSync` preference:**
- When `registryAutoSync` is `true` (in settings): signals for installed content refresh alongside registry sync at launch (existing 5-second timeout applies). This means users who have auto-sync on always have fresh signals without thinking about it.
- When `registryAutoSync` is `false`: signals update only on explicit `syllago install`, `syllago sync`, or `syllago scan`. The TUI's `R` (rescan catalog) key also triggers a signal refresh.
- No separate signal-sync setting. One toggle controls both registry content and signal freshness. Less config surface, same behavior users already understand.

**TUI integration:** The existing metadata panel in the TUI can display signal data from the cache. Library view shows the health indicator (🟢/🟡/🔴/⚫) inline with each item. Detail view shows the full signal breakdown. No network calls from the TUI — everything comes from the cache.

**CLI integration:**
```
$ syllago inspect community/sql-helpers

  sql-helpers v1.2.0 (skill, with scripts)
  ──────────────────────────────────────────
  Hash:    sha256:abc123 (verified at install)
  Scan:    no known issues detected (scanned 2026-04-01)
  ──────────────────────────────────────────
  Content:
    Type:          skill (risk tier 1: executable)
    Last modified: 3 days ago by @alice
    Files:         3 (includes scripts)
  Registry: acme-corp/skill-collection
    Health:        🟢 active
    Contributors:  8 (avg account age: 3.9 years)
    Issues:        87% close rate · 12 open
    Repo age:      2.1 years
    Stars:         342 · MIT license
  Signals:
    Last checked:  2026-04-01
  ──────────────────────────────────────────
```

#### Graceful Degradation

| Source | If unavailable |
|--------|---------------|
| Platform API (GitHub/GitLab/Codeberg) | Git-native signals only, note "(platform signals unavailable)" |
| Git metadata | Hash + scan still work, note "(git metadata unavailable)" |
| Network entirely | Scan runs locally on fetched files, health signals skipped |
| Cached signals | Show "(no signal data — run `syllago sync` to collect)" |

#### Example Output: Install Flow

Healthy skill from GitHub:
```
$ syllago install github.com/alice/sql-helpers

  sql-helpers v1.2.0 (skill, with scripts)
  ──────────────────────────────────────────
  Hash:   sha256:abc123 (verified)
  Scan:   no known issues detected ⁱ
  Source: github.com/alice/sql-helpers@def456
  ──────────────────────────────────────────
  Content:
    Last modified: 3 days ago by @alice
  Registry:
    🟢 active · 3 contributors (avg 4.2yr) · 87% issues closed
    Repo age: 2.1 years · ★ 342 · MIT license

  Install? [Y/n]
```

Stale content in active repo:
```
$ syllago install github.com/acme-corp/skills/old-formatter

  old-formatter v1.0.0 (skill, markdown only)
  ──────────────────────────────────────────
  Hash:   sha256:def456 (verified)
  Scan:   no known issues detected ⁱ
  Source: github.com/acme-corp/skills@ghi789
  ──────────────────────────────────────────
  Content:
    Last modified: 18 months ago by @departed-dev
    ⚠ Registry is active but this item appears unmaintained
  Registry:
    🟢 active · 8 contributors (avg 3.1yr) · 82% issues closed
    Repo age: 3 years · ★ 512 · Apache-2.0

  Install? [Y/n]
```

Suspicious signals:
```
$ syllago install github.com/t0tally-legit/ai-helper

  ai-helper v1.0.0 (skill, with scripts)
  ──────────────────────────────────────────
  Hash:   sha256:xyz789 (verified)
  Scan:   ⚠ 3 issues found
          · shell command: curl with piped execution (setup.sh:14)
          · network: hardcoded IP address (SKILL.md:47)
          · encoding: base64 blob (assets/config.dat)
  Source: github.com/t0tally-legit/ai-helper@aaa111
  ──────────────────────────────────────────
  Content:
    Last modified: 2 days ago by @new-account-2026
  Registry:
    🔴 new · 1 contributor (account age: 5 days)
    Repo age: 3 days · ★ 487 · no license
    ⚠ High stars but no issues or PRs (unusual)

  ⚠ Review scan issues before installing
  Show details? [y/N]
```

Self-hosted git (no platform API):
```
$ syllago install git.company.com/team/internal-skill

  internal-skill v3.0.0 (skill, markdown only)
  ──────────────────────────────────────────
  Hash:   sha256:bbb222 (verified)
  Scan:   no known issues detected ⁱ
  Source: git.company.com/team/internal-skill@ccc333
  ──────────────────────────────────────────
  Content:
    Last modified: 12 days ago by @jane
  Registry:
    5 contributors · last commit 12d ago
    (platform signals unavailable)

  Install? [Y/n]
```

### 5. Dependency Auto-Install

First-run (or first-use) detection of system dependencies with guided installation. Follows the pattern established by Homebrew (Xcode CLI tools), rustup (linker detection), and Docker Desktop (virtualization check).

#### At First Run (`syllago setup` or first command)

```
$ syllago

  Checking system dependencies...
    bubblewrap:  not found (required for sandbox isolation)
    socat:       not found (required for sandbox networking)
    yara:        not found (optional — enables deeper security scanning)

  Install required dependencies? [Y/n]

  Running: sudo apt install bubblewrap socat
  [sudo] password for user:
  Installing bubblewrap... done
  Installing socat... done

  Optional: install yara for deeper security scanning?
  Run: sudo apt install yara

  ✓ Ready
```

#### Package Manager Detection

Detect the system package manager and construct the right install command:

| Package Manager | Distros | Command |
|----------------|---------|---------|
| apt | Debian, Ubuntu, Pop!_OS | `sudo apt install <pkg>` |
| dnf | Fedora, RHEL, CentOS Stream | `sudo dnf install <pkg>` |
| pacman | Arch, Manjaro | `sudo pacman -S <pkg>` |
| apk | Alpine | `sudo apk add <pkg>` |
| zypper | openSUSE | `sudo zypper install <pkg>` |
| brew | macOS (bwrap not available — note limitation) | N/A for bwrap |

#### Dependency Tiers

| Dependency | Tier | Needed For |
|-----------|------|-----------|
| bubblewrap | Required for sandbox features | `syllago sandbox`, quarantine fetch |
| socat | Required for sandbox networking | `syllago sandbox` egress proxy |
| yara | Optional | Deeper scan patterns beyond native Go checks |
| git | Required (likely already present) | All git operations |

#### At Feature-Use Time (if setup was skipped)

```
$ syllago sandbox claude-code

  bubblewrap is required for sandboxing but isn't installed.
  Install now? [Y/n]
```

#### Implementation

Extends the existing `sandbox.Check()` pre-flight infrastructure in `check.go`. The current code already detects missing dependencies and reports them — we add the "install now?" prompt and package manager detection.

## Architecture Summary

```
syllago install <source>
    │
    ├── 1. Fetch to StagingDir (quarantined, isolated)
    │       └── Existing sandbox.StagingDir infrastructure
    │
    ├── 2. Content hash verification
    │       └── SHA-256, fail-closed on mismatch
    │
    ├── 3. Behavioral scan (on quarantined files)
    │       ├── Native Go pattern matching (always)
    │       ├── YARA rules via CLI (if yara installed)
    │       └── External scanners (if available, opt-in)
    │
    ├── 4. Git metadata extraction
    │       └── git log, shortlog — works on any git repo
    │
    ├── 5. Platform health signals (optional)
    │       └── GitHub/GitLab/Codeberg API, cached 1hr
    │
    ├── 6. Present results to user
    │       └── Hash + scan + health, honest language
    │
    └── 7. User decision
            ├── Approve → move to skill directory
            └── Decline → staging.Cleanup()
```

**Zero governance. Zero feeds. Zero social layer. Just facts.**

## Why This Won't Fail Like PGP

PGP failed because it merged person-trust and artifact-trust into one system (cryptographic keys), creating UX that humans couldn't manage.

This strategy doesn't attempt trust at all. It presents:
- **Artifact facts** — hash matches? scan findings? (computed from the files)
- **Source facts** — who wrote it? how active? how maintained? (computed from git/platform)

The user applies judgment. The system never says "this is trusted" or "this is safe." It says "here's what we know." The word "trust" doesn't appear in the UI.

The adversarial panel's key finding was that every system that CALLS itself a trust system gets treated as a security guarantee by users, regardless of disclaimers. By not being a trust system — by being a fact-presentation system — we sidestep this entirely.

## Open Questions (Reduced to Implementation Details)

1. **Content hash spec details** — exact algorithm, normalization rules, blanking regex. Carry forward from prior panel's recommendations (drop CRLF normalization, mandate LF, specify exact regex).
2. **Scanner rule update cadence** — how often do we add new patterns? On each syllago release? Separate update mechanism?
3. **Platform API rate limiting** — GitHub API has rate limits for unauthenticated requests (60/hr). Support optional `GITHUB_TOKEN` / `GITLAB_TOKEN` for higher limits. Cache aggressively — signals don't need to be real-time.
4. **macOS support** — bubblewrap uses Linux namespaces. What's the quarantine story on macOS? (Likely: temp directory with restrictive permissions, no namespace isolation. Scan still works.)
5. **YARA rule authoring** — what's the initial rule set? How do community members contribute rules?
6. **Signal cache location** — `~/.local/share/syllago/signals/` follows XDG. Need to handle the case where syllago is used across multiple machines (signals are local, not synced).
7. **Signal cache invalidation** — 30-day staleness warning is a starting point. Should different signal types have different TTLs? (Stars change fast, repo age never changes, issue close rate changes slowly.)
8. **Cross-project contributor lookups** — checking whether contributors contribute to other repos requires N API calls per contributor. Rate limit impact. May need to be opt-in or limited to top 3 contributors.

## Relationship to the Broader Strategy

This document covers the **install safety** piece. The broader Agent Skills strategy (from the earlier panel work) identified other concerns:

| Concern | Status | Where It Lives |
|---------|--------|---------------|
| Install safety (this doc) | Strategy complete | This document |
| Activation reliability | Separate concern | SKILL.md frontmatter fields (`mode`, `paths`, `commands`), syllago hub-and-spoke conversion |
| Distribution / versioning | Separate concern | Collection manifests, syllago registry system |
| Path fragmentation | Solved by syllago | Provider adapters, multi-path installation |
| Spec governance | Not our problem | Route around it — build tools, not spec proposals |
