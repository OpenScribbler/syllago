# Securing a Package Manager for AI Tools: From Adversarial Audit to Secretless CI

*How we used AI red-teaming, Ed25519 signing, and Aembit workload identity to harden syllago's security posture before its public release.*

---

## The Challenge

syllago is a package manager for AI coding tool content. It imports, exports, and converts rules, skills, agents, hooks, MCP configs, and prompts between providers like Claude Code, Gemini CLI, Cursor, Copilot, and others.

That description sounds innocuous until you realize what "install" means in this context. syllago doesn't install inert files. It installs content that **executes automatically** -- hooks fire on every tool use, MCP servers run persistently, rules shape AI behavior silently. The install path *is* the execution path, which makes syllago's security posture critical for anyone who uses it.

Before making the repository public, we needed to answer a direct question: **does syllago do anything dangerous by design?**

## Phase 1: AI Adversarial Audit

Instead of a manual code review, we ran five parallel AI agents as adversarial analysts, each examining a different attack surface in syllago's CLI:

| Agent | Focus Area |
|-------|-----------|
| 1 | Filesystem operations -- symlinks, path traversal, destructive ops |
| 2 | JSON merge & settings modification -- corruption, key collisions, atomicity |
| 3 | Network operations -- all HTTP/git calls, telemetry, MITM risks |
| 4 | Code execution & hooks -- injection, sandbox escape, execution model |
| 5 | Data exposure & privacy -- audit logs, path leakage, credential handling |

Each agent performed read-only analysis of the relevant source code and reported findings in a structured format with severity ratings. We then had the findings cross-verified by a second AI (Gemini) to validate accuracy and catch any missed issues.

### What We Found

**18 findings total** -- 2 High, 9 Medium, 7 Low. No critical vulnerabilities.

The good news was substantial. syllago already had strong security foundations: atomic file writes via temp-then-rename, item name validation that blocks all path traversal, a four-gate privacy system with fail-closed defaults, sandbox isolation via bubblewrap with capability drops, and git clone hardening that disables hooks and submodules.

The bad news was targeted:

**The two HIGH findings:**

1. **Self-update without signature verification.** The updater downloaded binaries from GitHub Releases and verified SHA-256 checksums -- but checksums.txt came from the same source as the binary. Same-origin checksums only catch accidental corruption, not deliberate tampering. If GitHub Releases were compromised, an attacker controls both the binary and the checksums.

2. **MCP server key collision.** Installing an MCP config with the same server name as a user's existing manual configuration silently overwrote it. No warning, no backup the user knew about, and uninstall would delete the key entirely.

**The most actionable MEDIUM finding:**

Registry `Sync()` (git pull) didn't apply the same security hardening as `Clone()`. Clone disabled git hooks (`core.hooksPath=/dev/null`), blocked submodules, and set `GIT_CONFIG_NOSYSTEM=1`. Sync did none of this. A malicious registry could add a `post-merge` hook in an update, and it would execute on the next sync. The fix was three lines of code.

### Notable Positive Findings

- **No telemetry.** Zero phone-home behavior. All HTTP requests are GET-only. No data uploads except user-initiated git push.
- **Privacy gates work.** The taint tracking system prevents private registry content from leaking to public destinations, with anti-laundering detection that catches copy-and-readd attempts.
- **Hook security scanner exists** (but wasn't wired into the install path -- we fixed that).

## Phase 2: Implementing All 18 Fixes

We fixed everything in a single session, working through the findings by severity:

**HIGH fixes:**
- Added Ed25519 signature verification to the self-update flow. The public key is embedded in the binary at compile time via ldflags. Release builds sign checksums.txt with the corresponding private key. Dev builds gracefully skip verification. Invalid signatures abort the update.
- Added collision detection to MCP install. If a server key exists and wasn't installed by syllago, the operation blocks with a clear error message.

**MEDIUM fixes included:**
- Registry sync hardening (matching clone's protections)
- Hook security scanner wired into the install path
- Hook event name validation against known events (preventing sjson key injection via dots)
- Hook matcher group whitelist filtering through a typed struct
- Universal JSONC comment stripping (fixing Zed settings corruption)
- Absolute path sanitization in the promote pipeline
- Sandbox config diff inverted from denylist to allowlist
- Proxy port enforcement (the `--allow-port` flag was stored but never checked)

**LOW fixes included:**
- Atomic symlink replacement (temp + rename)
- Uninstall scope validation before `RemoveAll`
- Sensitive system path blocklist for provider paths
- Atomic backup writes
- Directory ownership checks before cleanup
- Symlink resolution before containment checks
- Doctor command orphan detection for crash-window entries

All 21 Go packages passed. The binary was rebuilt. Every finding got a tracked issue with success criteria and test requirements.

## Phase 3: Securing the GitHub Repository

With the code hardened, we turned to the repository itself. A security audit of the GitHub configuration revealed the repo was already well-configured in several areas:

**Already solid:**
- All GitHub Actions pinned to full-length commit SHAs (not mutable version tags)
- SHA pinning required at the repo level
- Default workflow token permissions set to read-only
- Secret scanning with push protection enabled
- Dependabot configured for both Go modules and GitHub Actions
- Per-workflow explicit permissions blocks

**Gaps identified:**
- No required status checks on the main branch
- No tag protection for release tags (`v*`)
- No CODEOWNERS file for security-sensitive paths
- Stale PR reviews not dismissed on new pushes
- No release environment with manual approval gate

These were tracked as blockers for the public release.

## Phase 4: Secretless CI with Aembit

The repo had two static long-lived secrets stored in GitHub:
- `CLAUDE_CODE_OAUTH_TOKEN` -- authentication for the Claude Code GitHub Action
- `HOMEBREW_TAP_TOKEN` -- pushing formula updates during releases

Static secrets in CI/CD are a known attack vector. The 2025 tj-actions/changed-files supply chain breach demonstrated this: attackers compromised a bot account and modified a widely-used GitHub Action to scrape secrets from CI runner memory, exposing AWS keys, GitHub tokens, and npm credentials across 23,000+ repositories.

We replaced the Claude credential with [Aembit](https://aembit.io)'s secretless credential delivery using the [`Aembit/get-credentials`](https://github.com/Aembit/get-credentials) GitHub Action. The flow:

1. The GitHub Actions runner requests an OIDC token from GitHub's identity provider
2. The Aembit action presents this token to Aembit's trust provider
3. Aembit verifies the token cryptographically -- confirming the request comes from the `OpenScribbler/syllago` repository
4. Aembit delivers the credential just-in-time as a masked step output
5. The credential is passed to the Claude Code Action as an input parameter

The workflow change was minimal:

```yaml
# Before: static secret
- name: Run Claude Code
  uses: anthropics/claude-code-action@...
  with:
    claude_code_oauth_token: ${{ secrets.CLAUDE_CODE_OAUTH_TOKEN }}

# After: Aembit secretless delivery
- name: Get Claude credentials from Aembit
  id: aembit
  uses: Aembit/get-credentials@3de1076c... # v1.1.1, SHA-pinned
  with:
    client-id: aembit:useast2:...:github_idtoken:...
    credential-type: ApiKey
    server-host: api.anthropic.com

- name: Run Claude Code
  uses: anthropics/claude-code-action@...
  with:
    claude_code_oauth_token: ${{ steps.aembit.outputs.api-key }}
```

The benefits over a raw GitHub secret:

- **Identity-based access control.** The runner must prove it's the syllago repo before getting the credential. A compromised action in a different repo can't exfiltrate it.
- **Centralized audit trail.** Every credential access is logged in Aembit's dashboard -- which workload, when, what policy matched.
- **Single rotation point.** When the credential expires, rotate it once in Aembit instead of updating GitHub secrets.
- **No secret in GitHub at all.** The credential never touches GitHub's secret storage. There's nothing to leak from a repo settings compromise.

We applied the same pattern to the release pipeline's `HOMEBREW_TAP_TOKEN` -- a GitHub PAT used to push formula updates to the Homebrew tap repository. Same trust provider (the runner attests as `OpenScribbler/syllago`), different credential provider and server workload (`github.com` instead of `api.anthropic.com`). One Aembit policy per credential, both secretless.

The result: **zero static secrets in GitHub**. Both credentials are delivered just-in-time via OIDC attestation.

We tested the Claude integration end-to-end by creating a GitHub issue tagged with `@claude`. The workflow triggered, Aembit delivered the credential, and Claude responded successfully -- confirming the full OIDC attestation pipeline works in production.

## The Result

syllago's security posture before public release:

| Layer | Measure |
|-------|---------|
| **Code** | 18 findings fixed, Ed25519 update signing, privacy gates, sandbox isolation |
| **CI/CD** | All actions SHA-pinned, secretless credentials via Aembit, explicit permissions |
| **Repository** | Branch protection, tag protection, CODEOWNERS, required reviews |
| **Supply chain** | Sigstore cosign on releases, SBOM generation, Dependabot, govulncheck |
| **Transparency** | SECURITY.md with clear scope, trust model, and reporting process |

For a package manager that installs executable content into AI coding tools, this is the minimum bar. The attack surface is unique -- syllago sits between untrusted registries and provider configurations that auto-execute content. Every layer of defense matters.

The AI adversarial audit approach proved effective: five parallel agents found real issues that a manual review might have missed or deprioritized, and the structured severity ratings made it easy to triage and prioritize fixes. The entire process -- from audit to implementation to secretless CI -- took a single working session.

---

*syllago is an open-source package manager for AI coding tool content. Learn more at [github.com/OpenScribbler/syllago](https://github.com/OpenScribbler/syllago).*

*[Aembit](https://aembit.io) provides workload identity and access management for non-human identities, replacing static secrets with policy-based, just-in-time credential delivery.*
