# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest release | Yes |
| Previous minor release | Best-effort security patches |
| Older versions | No |

We recommend always running the latest release. Use `syllago update` to upgrade.

## Scope

Vulnerabilities we want to hear about:

- **Malicious content injection** -- rules, skills, agents, or hooks that execute attacker-controlled code through syllago's install or apply mechanisms
- **Path traversal** -- add, install, or export operations writing files outside the expected target directory
- **MCP config injection** -- JSON merge inserting unexpected server entries into provider settings
- **Symlink escape** -- content that resolves outside the expected install target via symlink manipulation
- **Registry trust bypass** -- mechanisms to install unconfirmed content without user awareness or confirmation
- **Private content leakage** -- content from private registries being published or shared to public registries through syllago commands

### Out of Scope (By Design)

- **Hooks and MCP servers execute code by design.** They are shell scripts and server processes. The user is responsible for trusting their source before installing.
- **Third-party registry content.** Syllago does not own, curate, or verify registry content. Registries are git repositories maintained by their owners.

## Trust Model

- Syllago operates no central registry or marketplace, but it is the reference implementation of [MOAT](https://github.com/OpenScribbler/moat) and ships with a bundled allowlist that auto-pins the official meta-registry's signing identity.
- Registries are git repositories cloned over HTTPS. When the registry publishes a MOAT manifest, syllago performs cryptographic verification on top of the git transport (Sigstore signature, Rekor inclusion, GitHub OIDC numeric-ID match). Registries without a MOAT manifest fall back to git-only integrity.
- **Built-in content** (the items shipped at `~/.syllago/content/` after `syllago init`) is maintained by the syllago team.
- **Registry content** is verifiable: each item carries a trust tier — `DUAL-ATTESTED`, `SIGNED`, or `UNSIGNED` — surfaced by `syllago list`, `syllago info`, and the TUI.
- **App install scripts** from registries require explicit user confirmation before execution.
- Users should still review hooks and MCP configs before installing — by design, they execute as your user. Signature verification proves origin and integrity, not safety of the code itself.

## Content Verification (MOAT)

Syllago is the reference implementation of MOAT (Model for Origin, Attestation, and Trust), a community-owned spec for verifying AI coding tool content. The MOAT package is at [`cli/internal/moat/`](cli/internal/moat/). The full spec lives at https://github.com/OpenScribbler/moat.

**Trust tiers (`cli/internal/moat/lockfile.go`):**

| Tier | Meaning |
|---|---|
| `DUAL-ATTESTED` | Both the registry signing profile *and* the per-item attestation verify against bundled trust roots, with matching Rekor inclusion proofs. Strongest tier. |
| `SIGNED` | Manifest signature verifies against a pinned signing profile, but no per-item dual-attestation is present. |
| `UNSIGNED` | No MOAT manifest. Falls back to git-only integrity. Accepted only if the registry was added without `--moat` and is not in the allowlist. |

**Verification pipeline:**

1. **Fetch** — clone the registry over HTTPS, locate `moat-registry.json` and per-item attestations.
2. **Sigstore verify** — verify the cosign bundle against the bundled Sigstore trusted root.
3. **Rekor inclusion** — check the Rekor log entry referenced by `rekor_log_index` is present and matches the canonical payload hash.
4. **Identity match** — confirm the OIDC certificate's subject (workflow SAN), issuer, and **GitHub repository numeric IDs** (`repository_id`, `repository_owner_id`) match the pinned signing profile. Numeric ID pinning defeats repo-rename and ownership-transfer attacks where an attacker reuses a human-readable name.
5. **Tier resolution** — assign one of the three tiers above; `--min-trust` policy can refuse anything below the configured floor (`MOAT_009`).

**Pinning paths (at `syllago registry add` time):**

- **Bundled allowlist** — well-known registries auto-pin from [`cli/internal/moat/signing_identities.json`](cli/internal/moat/signing_identities.json). Currently the official meta-registry only.
- **Explicit flags** — `--signing-identity <workflow-san> --signing-repository-id <id> --signing-repository-owner-id <id>`. Operator records the pin in the commit that runs `registry add`.
- **Hard fail** — if MOAT was requested or implied by partial flags but neither path provides a complete identity, `MOAT_001 IDENTITY_UNPINNED` blocks the add. There is no Trust-On-First-Use path by design.

**Trusted-root staleness cliff:** the bundled Sigstore trusted root has a 365-day cliff (per ADR 0007). Once expired, `MOAT_005` blocks verification until the binary is updated or `--trusted-root` is supplied.

**Revocation:** registries and publishers can be revoked archivally (in the bundled list) or live (via the registry source). Reasons: `malicious`, `compromised`, `deprecated`, `policy_violation`. Blocked installs surface `MOAT_008`.

**Error codes:** `MOAT_001` through `MOAT_009` are documented in [`cli/internal/output/errors.go`](cli/internal/output/errors.go) and explainable at runtime via `syllago explain MOAT_NNN`.

## Sandbox (Linux)

Syllago ships a bubblewrap-based sandbox for executing third-party AI CLI tools with credential and filesystem isolation. Implementation is at [`cli/internal/sandbox/`](cli/internal/sandbox/) (24 source files: `bwrap.go`, `bridge.go`, `configdiff.go`, `envfilter.go`, `proxy.go`, `runner.go`, `staging.go`, plus tests).

**Capabilities:**

- **Process isolation** — bubblewrap (`bwrap`) wraps the wrapped tool's process tree.
- **Credential protection** — env-var allowlist (`envfilter.go`) drops secrets that the wrapped tool does not need.
- **Network egress allowlist** — domain + port list, enforced via a proxy (`proxy.go`).
- **Filesystem staging** — `staging.go` mounts a curated subset of the user's home into the sandbox.
- **Config diff-and-approve** — on session exit, `configdiff.go` shows what the wrapped tool changed in its config and prompts for approval. High-risk keys (`mcpServers`, `hooks`, `commands`) trigger a stricter confirmation.

**Commands:**

- `syllago sandbox run <tool> -- <args>` — execute inside the sandbox.
- `syllago sandbox check` — preflight diagnostic (verifies `bwrap` present, profile valid).
- `syllago sandbox info` — show effective profile.

**Platform support:** Linux only today. macOS sandbox support is on the roadmap.

## Registry Privacy

Syllago includes a privacy gate system to prevent accidental leakage of content from private registries to public destinations.

- **Detection:** Private registries are identified via hosting platform APIs (GitHub, GitLab, Bitbucket) and an optional `visibility` field in `registry.yaml`. Unknown visibility defaults to private.
- **Tainting:** Content imported from private registries is permanently tagged with its source registry and visibility in metadata. This taint persists through the content's lifecycle in the library.
- **Enforcement:** Four gates block private-tainted content from reaching public registries -- at `publish`, `share`, `loadout create` (warning), and `loadout publish` (block).
- **Scope of protection:** This is a soft gate designed to prevent accidental leakage through syllago commands. It does not prevent intentional circumvention via direct filesystem operations, direct git commands, or modification of content after export. There is no override flag by design -- removing the taint requires re-adding the content from a public source.

## Release Integrity

All release binaries are cryptographically signed and checksummed:

- **Sigstore cosign** -- keyless signing via [Sigstore](https://www.sigstore.dev/). Every release includes a `checksums.txt.bundle` attestation.
- **SHA-256 checksums** -- `checksums.txt` lists the hash for every release artifact.
- **Verification:**
  ```bash
  # Download the release artifacts, checksums, and bundle
  # Verify the cosign signature
  cosign verify-blob --bundle checksums.txt.bundle checksums.txt

  # Verify individual binary checksums
  sha256sum --check checksums.txt
  ```

## CI Security Practices

- **Pinned dependencies** -- all GitHub Actions are pinned to full-length commit SHAs, not mutable version tags
- **Least-privilege tokens** -- CI workflows use explicit `permissions:` blocks with minimal scope
- **Automated vulnerability scanning** -- `govulncheck` runs in CI to catch known-vulnerable dependencies
- **Static analysis** -- golangci-lint with `gosec` (security linter) runs on every PR and push
- **Race detector** -- `go test -race` runs in CI to catch data race conditions
- **Dependency updates** -- [Dependabot](https://github.com/dependabot) is configured for automated dependency security patches
- **Module integrity** -- `go mod tidy` check ensures no unexpected dependency changes

## Reporting a Vulnerability

**Email:** openscribbler.dev@pm.me

Subject line: `[SECURITY] syllago -- <brief description>`

**Response targets:**

- Acknowledgment: 48 hours
- Fix or mitigation: 7 days

**Please include:**

- Description of the vulnerability and impact
- Reproduction steps
- Affected versions (check `syllago version`)

This is a pre-revenue open source project. There is no bug bounty program.

## Safe Harbor

We support good-faith security research. If you act in good faith to identify and report vulnerabilities following this policy, we will not pursue legal action against you. We ask that you:

- Make a reasonable effort to avoid privacy violations, data destruction, and service disruption
- Only interact with accounts you own or with explicit permission from the account holder
- Give us reasonable time to address the issue before public disclosure

## Disclosure Policy

We prefer coordinated disclosure. Please do not open public GitHub issues for security vulnerabilities. We will credit reporters in the security advisory unless you prefer to remain anonymous.
