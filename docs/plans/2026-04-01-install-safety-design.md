# Install Safety System - Design Document

**Goal:** Present computed facts about content at install time so users can make informed decisions — no governance, no social trust, no composite scores.

**Decision Date:** 2026-04-01

**Strategy Reference:** `docs/research/agent-skills-spec/18-install-safety-strategy.md`

---

## Problem Statement

Users install AI coding tool content (skills, hooks, agents, rules) from community registries with no visibility into what they're getting. There's no scan, no metadata, no health context. A malicious skill could exfiltrate credentials, inject prompts, or execute arbitrary commands — and the user would never know until it's too late.

The existing spec ecosystem has no safety mechanisms, and governance is non-functional (48 proposals, 1 answered). We need a tool-level solution that works without spec changes or community coordination.

## Design Principles

1. **Facts, not opinions.** Show what we computed. Never say "safe" or "trusted."
2. **User autonomy.** Never block installation. Warn, inform, let the user decide. Bypass is always available.
3. **Zero governance.** No feeds to maintain, no governing body, no social trust layer.
4. **Graceful degradation.** Every capability works independently. Missing bwrap? Standard scan. No API token? Git-only signals. No network? Local scan still runs.
5. **No magic trust.** Private repos get the same standard scan as public repos. Trust is always explicit — add a source to `trustedSources` to skip scanning.

## Proposed Solution

Four capabilities shipping in v1, one deferred:

| # | Capability | Status |
|---|-----------|--------|
| 1 | Quarantine Fetch | v1 |
| 2 | Content Hash | **Deferred** — needs research spike on normalization, cross-tool interop |
| 3 | Behavioral Scanner | v1 |
| 4 | Health Signals | v1 |
| 5 | Dependency Auto-Install | v1 |

---

## Architecture

```
syllago install <source>
    |
    +-- 1. Fetch to StagingDir (quarantined, outside agent-visible paths)
    |       +-- Existing sandbox.StagingDir infrastructure
    |
    +-- 2. Behavioral scan (on quarantined files)
    |       +-- Standard: native Go pattern matching (always)
    |       +-- Sandboxed: bwrap (Linux) or sandbox-exec (macOS) if available
    |       +-- Optional: YARA rules via CLI (if yara installed)
    |       +-- Optional: External scanners (Snyk, SafeDep — opt-in)
    |
    +-- 3. Git metadata extraction
    |       +-- git log, shortlog -- works on any git repo
    |
    +-- 4. Platform health signals (optional)
    |       +-- GitHub/GitLab/Codeberg API, cached locally
    |       +-- Token from env (GITHUB_TOKEN etc.), never persisted
    |
    +-- 5. Present results to user
    |       +-- Scan findings + health signals, honest language
    |
    +-- 6. User decision
            +-- Approve -> move to skill directory (installed)
            +-- Decline -> staging.Cleanup()
```

---

## Capability 1: Quarantine Fetch

Files are fetched to an isolated staging directory before installation. Files never land in a location where an AI agent could discover or execute them until the user explicitly approves.

**Infrastructure:** Reuses existing `sandbox.StagingDir` — random hex ID, XDG_RUNTIME_DIR preference, `CleanStale()` on startup, auto-cleanup.

**Isolation model:** The staging directory is outside any configured skill/rule/hook path. This is *location-based isolation*, not process-level sandboxing. The behavioral scanner reads files from this directory as normal Go code.

**Optional sandboxed scan:** If bwrap (Linux) or sandbox-exec (macOS) is available and the user opts in, the scanner runs inside a sandbox with read-only access to the staging dir, no network, and no host filesystem access. This is defense-in-depth — the standard scan works without it.

| Platform | Sandbox Tool | Mechanism |
|----------|-------------|-----------|
| Linux | bubblewrap (bwrap) | Linux namespaces, seccomp |
| macOS | sandbox-exec | Seatbelt profiles (deprecated but widely used by OpenAI, Google, Cursor) |
| Either missing | None | Standard scan, no isolation |

**Flow:**
1. Create StagingDir (random hex, restricted permissions)
2. Git clone into staging dir
3. Run behavioral scan (standard or sandboxed)
4. Collect health signals
5. Present results
6. User approves -> move files to install path
7. User declines -> `staging.Cleanup()`

**Ctrl+C handling:** Existing signal handler calls `staging.Cleanup()`.

---

## Capability 2: Content Hash (DEFERRED)

SHA-256 content hashing is deferred to a separate research spike. The normalization rules (line endings, file ordering, hash field blanking regex) and cross-tool interoperability concerns need dedicated research before implementation.

**What's deferred:** Hash computation, hash verification at install, hash storage in signal cache.

**What's NOT deferred:** Per-file integrity hashing is included in v1 (see TOCTOU Integrity below). The deferred work is about the *interoperable* cross-tool content hash spec, not basic integrity verification.

### TOCTOU Integrity (Per-File Hash Verification)

To close the time-of-check-to-time-of-use gap between scanning and installing, syllago computes per-file SHA-256 hashes at scan time and re-verifies them before moving files to the install path.

**At scan time:** Walk the staging directory. Hash every file individually. Store a map of `relative_path -> sha256_hash`. Also record the complete file list.

**Before move:** Re-walk the staging directory. Verify:
1. The file list matches exactly (no files added or removed)
2. Every file's hash matches what was recorded at scan time

**On mismatch:** Abort the install with `INSTALL_006` error. Files remain in staging for inspection.

**Performance:** SHA-256 is hardware-accelerated on modern CPUs. 100 files at 10 KB each: ~0.5ms total. Negligible compared to the git clone that precedes it.

**Note:** These hashes are internal to syllago — they're not the publishable cross-tool content hash (which is deferred). They serve only to ensure the files that get installed are exactly the files that were scanned.

---

## Capability 3: Behavioral Scanner

Native Go pattern matcher. Zero external dependencies for the base scanner. Deterministic — same files always produce the same results.

### Scan Levels

| Level | Name | Behavior | Default For |
|-------|------|----------|-------------|
| 0 | Skip | No scan, direct install | Trusted sources, `--no-scan` flag |
| 1 | Standard | Content-type-aware scanning. Scripts get full rules, markdown gets relaxed rules. High-confidence patterns only. | Public repos (default) |
| 2 | Strict | All patterns applied to all file types regardless of content type. Includes lower-confidence matches. | Opt-in via `--strict` or config |

### Trust Model

Per-repo trust persisted in syllago config:

```json
{
  "trustedSources": [
    "github.com/acme-corp/skill-collection",
    "git.company.com/team/*"
  ]
}
```

| Source Type | Default Scan Level | Override |
|------------|-------------------|---------|
| Trusted source (config) | Skip (level 0) | `--scan` to force |
| Any repo (public or private) | Standard (level 1) | `--strict` for level 2, `--no-scan` for level 0 |

Trust is per-repo (not per-item). "I trust this source" applies to all content from that registry.

Users can force scan on trusted sources with `--scan`. Users can bypass scan on any source with `--no-scan`.

### Detection Categories (Published in Docs)

- Shell commands with dangerous execution patterns
- Encoded or obfuscated content
- References to sensitive file paths or environment variables
- Hardcoded network endpoints and suspicious URLs
- Unicode manipulation techniques
- Known prompt injection patterns

### Specific Patterns (In Source Code, NOT Published)

- Unicode Tag characters (U+E0000-U+E007F)
- Zero-width characters (U+200B, U+200C, U+200D, U+FEFF)
- Bidirectional text markers (U+202A-U+202E, U+2066-U+2069)
- Base64-encoded shell commands
- `curl | bash`, `wget | sh`, `eval` execution patterns
- Hardcoded API key formats (AWS, GitHub, Stripe, etc.)
- Environment variable exfiltration patterns
- File path traversal (../../.ssh/, ~/.aws/credentials)
- Known prompt injection phrases

**Transparency model:** Publish categories, not exact patterns. Same as antivirus vendors.

### Scanner Precision Philosophy

**Precision over coverage.** npm audit's biggest failure is alert fatigue — too many low-confidence findings train users to ignore everything. Our scanner matches **dangerous patterns precisely**, not tool names broadly:

- `curl | bash` in a `.sh` file -> flag (piped execution pattern)
- `curl https://api.example.com` in a `.sh` -> no flag (normal HTTP call)
- `eval $(base64 -d ...)` -> flag (obfuscated execution)
- `eval "$variable"` -> no flag in standard mode (common shell pattern)

The initial pattern set should be **small and high-confidence**. Better to miss an edge case than to cry wolf. Expand patterns based on real-world feedback and verified attack data.

### What It Does NOT Detect (Published Honestly)

- Novel prompt injection written as natural-sounding instructions
- Context-dependent instructions (legitimate vs malicious credential access)
- Subtle behavioral manipulation
- Zero-day techniques not yet in the pattern set

### UX Language

Never say "clean" or "safe." These create false confidence.

No issues:
```
Scan:   no known issues detected
        (checks known patterns, not a safety guarantee)
```

Issues found:
```
Scan:   ! 3 issues found
        . shell command: curl with piped execution (setup.sh:14)
        . network: hardcoded IP address (SKILL.md:47)
        . encoding: base64 blob (assets/config.dat)
```

### Optional Enhancements

**YARA deep scan:** If `yara` CLI is installed, syllago shells out with embedded rule files (`//go:embed`). Same external-tool pattern as bubblewrap.

**External scanners:** Snyk agent-scan, SafeDep vet. Opt-in integrations, not dependencies. Syllago detects availability and offers to use them.

### Known-Bad Feed Integration (v1)

**GHSA (GitHub Security Advisory):** Query GHSA via the GitHub API for known CVEs matching the content being installed. As of early 2026, 30+ CVEs exist for MCP servers (mostly command injection vulnerabilities). This uses the same GitHub API we already call for health signals -- one additional GraphQL query per install. Only matches content that's also a published npm/PyPI package.

**Snyk Agent Scan:** If `snyk-agent-scan` CLI is installed (`uvx snyk-agent-scan@latest`), syllago invokes it for 15+ risk categories including prompt injection, tool poisoning, toxic flows, malware payloads, and credential exposure. Same optional-external-tool pattern as YARA. No API key required (local scanner).

**Feed architecture:** Both integrations implement a `FeedChecker` interface that returns findings in the same format as the behavioral scanner. The scanner pipeline is: native Go patterns -> YARA (if available) -> GHSA check (if GitHub API accessible) -> Snyk (if installed). Results are merged into a single `ScanResult`.

### Scanner on Update/Sync

- `syllago update` / `syllago sync`: Always re-scan all content, even from trusted sources.
- Trust means "skip the interactive prompt on first install", NOT "skip the scan."
- **When scan findings change on update:** The update is **held for user approval** — content stays in staging until the user reviews the new findings. This matches the install flow behavior.
- **When no new findings:** Update applies silently.
- This ensures malicious content injected into a previously-clean repo is always caught before installation, even from trusted sources.

---

## Capability 4: Health Signals

Facts computed from git history and (optionally) platform APIs. Cached locally. Always available in TUI and CLI without network calls after initial collection.

### Signal Design Principles

1. **Individual signals, not composite scores.** Composite scores get Goodharted.
2. **Correlation detects manipulation.** 500 stars + 0 issues + 1 contributor (5-day account) = manipulation.
3. **Two-tier display.** Registry-level + content-level. Healthy repo != healthy content.
4. **Content type determines scan depth**, not display prominence.

### Full Signal Set (v1 — All Tiers)

#### Tier 1: Hard to Fake (Highest Value — Opportunistic Collection)

Tier 1 signals require authenticated, multi-round-trip API calls. They are collected **opportunistically** — when a token is available and rate budget allows. If Tier 1 collection fails or is rate-limited, Tier 2+3 signals are shown with a note about what's missing.

| Signal | Source |
|--------|--------|
| Repo creation date | Platform API (server-side timestamp) |
| Contributor account ages | Platform API per contributor (top 3 only) |
| Cross-project contributor activity | Platform API (top 3 contributors only) |
| Issue/PR with external participants | Platform API |
| Release cadence | Platform API |

#### Tier 2: Moderate to Fake

| Signal | Source |
|--------|--------|
| Issue close rate + median response time | Platform API |
| PR review depth | Platform API |
| CI/CD passing status | Platform API |
| Test file presence | Git tree |

#### Tier 3: Easy to Fake (Supplementary Only)

| Signal | Source |
|--------|--------|
| Stars | Platform API |
| Fork count | Platform API |
| Commit count | Git log |
| README quality | Git tree |
| Raw contributor count | Git shortlog |

#### Registry-Level Signals

Repo creation date, license, total contributors, contributor account ages (avg top 3), cross-project activity, last commit, issue close rate, median response time, PR merge rate, stars, archived status, has SECURITY.md, CI/CD presence + status.

#### Content-Level Signals (Per-Item)

Last modified date, author (git blame), scan results, content type risk tier, file count + has scripts.

#### Content-Type Risk Tiers

| Tier | Types | Risk |
|------|-------|------|
| 1: Executable | Hooks, skills with scripts | Highest |
| 2: External-pointing | MCP configs | Medium-high |
| 3: Instructional | Skills (markdown), commands, agents | Medium |
| 4: Passive | Rules | Lower (but highest exposure) |

#### Derived Signals

| Signal | Computation |
|--------|-------------|
| Stale content in active repo | Repo active, content untouched 12+ months (threshold: AI tools evolve rapidly; 12mo content likely targets outdated models/APIs) |
| Author divergence | Content author != repo owner |
| Bus factor | Min contributors for 50% of commits |
| Health indicator | Composite: active / slow / dormant / archived |
| Star/engagement mismatch | High stars + zero issues/PRs |

#### Anti-Pattern Flags

- Repo < 7 days old
- All contributors have accounts < 30 days old
- Name is Levenshtein distance <= 2 from known package (typosquatting)
- High stars but zero issues/PRs
- Content contains executable patterns but labeled markdown-only

**Typosquatting corpus:** Seeded from known attack data (SANDWORM_MODE campaign's 19 typosquatted packages, Snyk's scanned skill marketplace data) and embedded in the binary via `go:embed`. Updated each syllago release. Supplemented at runtime with content names from the user's configured registries, providing dynamic expansion without remote dependencies.

### Platform API & Token Handling

**Strategy:** Cache aggressively + support optional tokens. Without token: cache + top-3 contributors + backoff. With token: full signal set, faster refresh.

**Token security (CRITICAL):**
- Read `GITHUB_TOKEN` / `GITLAB_TOKEN` from environment ONLY
- Never log, cache, persist, or include tokens in error messages
- Never pass tokens to subprocesses — Go HTTP client uses them directly
- Invalid token (401) -> warn + fall back to unauthenticated
- Tokens are never in signal cache files or debug output

**Rate limiting:**
- Cache all API responses for 24 hours
- Batch contributor lookups (top 3 only)
- Rate-limit own calls with exponential backoff
- Without token: 60 req/hr (GitHub). With token: 5000 req/hr.

### Cached Signal Store

**Location:** `~/.local/share/syllago/signals/` (XDG_DATA_HOME)

**When collected:**
- `syllago install` — during quarantine fetch, stored on successful install
- `syllago sync` / `syllago update` — refreshed for all installed content
- `syllago scan` — rescan installed content
- TUI `R` key — registry-level signal refresh

**Staleness:** Signals show `scanned_at` date. Older than 30 days: "(last checked 45 days ago -- run `syllago sync` to refresh)"

**Sync follows existing `registryAutoSync` preference.** No separate signal-sync setting.

**Cache integrity:** Cache files use 0600 permissions. The signal cache is a convenience layer, not a security boundary -- fresh signals can always be re-fetched via `syllago sync`. A local attacker with write access to the home directory is outside our threat model (they already own the machine). This is a documented limitation.

### Graceful Degradation

| Source | If Unavailable |
|--------|---------------|
| Platform API | Git-native signals only, note "(platform signals unavailable)" |
| Git metadata | Scan still works, note "(git metadata unavailable)" |
| Network entirely | Scan runs locally, health signals skipped |
| Cached signals | "(no signal data -- run `syllago sync` to collect)" |

---

## Capability 5: Dependency Auto-Install

First-run detection with guided installation. Follows Homebrew/rustup patterns.

### Dependencies

| Dependency | Tier | Needed For |
|-----------|------|-----------|
| git | Required (likely present) | All git operations |
| bubblewrap | Optional | Sandboxed scan (Linux), `syllago sandbox` |
| socat | Optional | `syllago sandbox` egress proxy |
| yara | Optional | Deep scan patterns |

**Note:** bwrap and socat are NOT required for quarantine fetch or standard scanning. They're only needed for the sandboxed scan enhancement and the separate `syllago sandbox` command.

### Package Manager Detection

| Package Manager | Distros | Command |
|----------------|---------|---------|
| apt | Debian, Ubuntu, Pop!_OS | `sudo apt install <pkg>` |
| dnf | Fedora, RHEL, CentOS Stream | `sudo dnf install <pkg>` |
| pacman | Arch, Manjaro | `sudo pacman -S <pkg>` |
| apk | Alpine | `sudo apk add <pkg>` |
| zypper | openSUSE | `sudo zypper install <pkg>` |
| brew | macOS | `brew install <pkg>` (bwrap N/A) |

### Flow

At first run or first use of a feature requiring a missing dependency:
1. Detect missing dependencies
2. Detect package manager
3. Prompt: "Install required dependencies? [Y/n]"
4. Run install command (requires sudo for system packages)
5. For optional deps: separate prompt with explanation

Extends existing `sandbox.Check()` pre-flight infrastructure.

---

## Error Codes

### New: SCAN Category

| Code | Constant | Meaning |
|------|----------|---------|
| SCAN_001 | ErrScanTimeout | Scanner exceeded time limit -- partial results |
| SCAN_002 | ErrScanCrash | Scanner panicked on a file -- partial results |
| SCAN_003 | ErrScanMalformed | File could not be parsed for scanning |
| SCAN_004 | ErrScanYaraFailed | YARA execution failed -- basic scan used |
| SCAN_005 | ErrScanSandboxFailed | bwrap/sandbox-exec failed -- standard scan used |

**Note:** "Scan completed with findings" and "Scan skipped" are not errors — they're status fields on the `ScanResult` struct (`ScanResult.IssuesFound`, `ScanResult.Skipped`). Only actual failures get error codes.

### New: SIGNAL Category

| Code | Constant | Meaning |
|------|----------|---------|
| SIGNAL_001 | ErrSignalAPIAuth | Token invalid (401) -- unauthenticated fallback |
| SIGNAL_002 | ErrSignalAPIRate | Rate limited (403/429) -- using cache |
| SIGNAL_003 | ErrSignalAPITimeout | API timeout -- using cache |
| SIGNAL_004 | ErrSignalAPINoPlatform | Unrecognized git host -- no platform signals |
| SIGNAL_005 | ErrSignalCacheStale | Cached signals older than 30 days |
| SIGNAL_006 | ErrSignalCacheMissing | No cached signals available |
| SIGNAL_007 | ErrSignalGitShallow | Shallow clone -- limited git history |
| SIGNAL_008 | ErrSignalAntiPattern | Anti-pattern flag triggered |

### Additions to INSTALL Category

| Code | Constant | Meaning |
|------|----------|---------|
| INSTALL_006 | ErrInstallQuarantineFailed | Could not create staging directory |
| INSTALL_007 | ErrInstallScanWarn | Proceeding despite incomplete scan |
| INSTALL_008 | ErrInstallUntrustedDeclined | User declined after reviewing results |

### Additions to SYSTEM Category

| Code | Constant | Meaning |
|------|----------|---------|
| SYSTEM_003 | ErrSystemDepMissing | System dependency not found |
| SYSTEM_004 | ErrSystemDepInstallFailed | Dependency auto-install failed |
| SYSTEM_005 | ErrSystemPkgMgrNotFound | No supported package manager detected |

---

## TUI Integration (High-Level)

| View | What Shows | Data Source |
|------|-----------|-------------|
| Library list | Health indicator inline | Signal cache -> `registry.health` |
| Library metadata panel | Scan status, risk tier, last scan date | Signal cache -> `scan.*`, `content.*` |
| Detail drill-in | Full signal breakdown | Full signal cache entry |
| Install wizard | Scan results + health summary before confirm | Live scan + signal fetch |
| Settings | Scan level preference, trusted sources | Config file |

**Note:** The detail drill-in view currently uses a two-panel split (file tree + file content). Fitting the full signal breakdown into this layout needs a separate brainstorm + prototype cycle to get the UX right. Do not assume the current panel structure accommodates signal data without design work.

**Backend exposes:**
- `SignalStore` — reads/writes signal cache directory
- `ScanResult` struct — issues list with file:line, scanner version, scan level
- `TrustConfig` — trusted sources list, scan level preference

---

## Key Decisions

| Decision | Choice | Reasoning |
|----------|--------|-----------|
| Quarantine isolation | Location-based (StagingDir), not process-based | bwrap is optional, not everyone has it, macOS has no equivalent for hard isolation |
| Sandboxed scan | Optional enhancement (bwrap/sandbox-exec) | Defense-in-depth for users who want it, not a requirement |
| Content hash | Deferred to research spike | Normalization rules and cross-tool interop need dedicated research |
| Scan levels | 3 levels (skip/standard/strict) | User chooses their risk tolerance. Never block installation. |
| Trust scope | Per-repo, not per-item | Natural mental model: "I trust this source" |
| Private repos | Same as public (standard scan) | Private != reviewed (enterprise forks, shared repos, CI/CD writes). Trust is always explicit via trustedSources config. |
| Token handling | Env vars only, never persisted | Don't become a credential store |
| Signal set | Full set in v1 | Correlation between signals is the key value -- partial set undermines this |
| Scan on update | Always re-scan, even trusted | Trust skips the prompt, not the scan |
| macOS sandbox | sandbox-exec (deprecated but universally used) | No Apple alternative exists; OpenAI, Google, Cursor all use it |
| Scanner failures | Warn, don't block | Respect user autonomy; incomplete scan is a warning, not a blocker |
| Error codes | 18 new codes across SCAN, SIGNAL, INSTALL, SYSTEM | Structured codes for docs, debugging, CI/CD programmatic handling |
| TOCTOU integrity | Per-file SHA-256 at scan time, verify before move | Closes the scan-then-move integrity gap without the deferred cross-tool hash spec |
| Update review gate | Hold update when new scan findings appear | Prevents malicious updates from silently installing even from trusted sources |
| Scanner precision | Match dangerous patterns, not tool names | Avoids npm-audit-style alert fatigue that trains users to bypass |
| Known-bad feeds | GHSA (built-in) + Snyk agent-scan (optional) | 30+ MCP CVEs exist in GHSA; Snyk covers 15+ risk categories for skills |
| Typosquatting corpus | Embedded attack data + registry-derived names | Seeded from SANDWORM_MODE campaign, supplemented with local registry names |
| Install output | Compact by default (5-7 lines), --verbose for full signals | Prevents information overload at the install prompt |

## Data Flow

```
Install Request
    |
    v
[Check Trust Config] --> Trusted? --> [Skip Scan] --> [Fetch + Install]
    |                                                       |
    | Not trusted                                           |
    v                                                       |
[Create StagingDir] ----+                                   |
    |                   |                                   |
    v                   v                                   |
[Git Clone to         [Collect Health                       |
 Staging Dir]          Signals (parallel)]                  |
    |                   |                                   |
    v                   |                                   |
[Behavioral Scan]      |                                   |
    |                   |                                   |
    +-------------------+                                   |
    |                                                       |
    v                                                       |
[Present Results] --> User Approves --> [Move to Install] --+
    |                                                       |
    +---> User Declines --> [staging.Cleanup()]              |
                                                            v
                                                    [Cache Signals]
```

## Success Criteria

1. `syllago install` from a public GitHub repo shows scan results and health signals before prompting
2. Scan detects all pattern categories listed (shell exec, unicode, injection, etc.)
3. Health signals degrade gracefully when API is unavailable
4. Private and public repos both get standard scan by default
5. Trusted sources skip scan prompt on first install, but updates with new findings are held for approval
6. Signal cache persists across sessions, refreshes on sync
7. All 18 error codes have corresponding docs pages
8. GHSA feed check catches known MCP CVEs during install
9. Per-file integrity hashes verified before moving from staging to install path
10. macOS and Linux both support sandboxed scan (when tools available)
11. First-run dependency detection works across 6 package managers

## Open Questions (Reduced to Implementation Details)

1. Scanner rule update cadence — on each syllago release? Separate mechanism?
2. YARA initial rule set scope — what rules ship embedded?
3. Signal cache TTL per signal type — should stars (fast-changing) have different TTL than repo age (never changes)?
4. TUI detail drill-in layout for signal data — needs separate brainstorm + prototype
5. Compact install output format — exact layout of the 5-7 line default view

---

## Panel Review History

### Round 1 (2026-04-01)

**Reviewers:** Skeptic, Security, Pragmatist, Package Manager Expert

**Critical Issues Addressed:**
- TOCTOU gap: Added per-file SHA-256 integrity verification between scan and move
- Private repo trust too broad: Changed to same-as-public (standard scan). Trust is always explicit.
- Update review gate: New scan findings on update now hold for user approval

**Moderate Concerns Addressed:**
- Signal cache integrity: 0600 permissions, documented as convenience not security boundary
- Full signal set scoping: Tier 1 signals collected opportunistically (token + rate budget permitting)
- Scanner false positives: Precision-first philosophy — match dangerous patterns, not tool names
- Known-bad feeds: Added GHSA (built-in) + Snyk agent-scan (optional) in v1
- Typosquatting corpus: Seeded from SANDWORM_MODE attack data + registry-derived names

**Minor Issues Fixed:**
- SCAN_006/007 moved from error codes to ScanResult status fields
- 12-month staleness threshold documented with rationale
- Install output compacted to 5-7 lines default, --verbose for full signals
- Footnote pattern replaced with dim contextual hint

---

## Next Steps

1. Create implementation plan (break into phases/tasks)
2. Implementation order: Scanner -> Quarantine Fetch -> Health Signals -> Feed Integration -> Dependency Auto-Install
