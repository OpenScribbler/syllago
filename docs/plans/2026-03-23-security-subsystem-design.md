# Security Subsystem v1 Design

**Date:** 2026-03-23
**Status:** Approved (brainstorm complete)
**Epic:** syllago-q8c5

## Scope

Full security subsystem for syllago v1. Four phases, seven features, ~5500-7400 lines across 30+ files.

## Decisions Made

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Signing methods | Sigstore + GPG | Modern keyless (OIDC) + traditional key-based. Covers open source + enterprise. |
| Sigstore implementation | sigstore-go for verify, shell out to cosign for sign | Keeps binary small; verification (common path) is embedded, signing (rare) needs cosign installed. |
| GPG implementation | ProtonMail/go-crypto | No external gpg binary required. |
| Keybase | Excluded | Maintenance mode since Zoom acquisition (2020). Sigstore solves the same identity-linked signing problem. |
| Scanner scope | Full pluggable pipeline | Built-in regex + external adapter (ShellCheck/Semgrep) + configurable chain. |
| Audit scope | Wire + query CLI + export | JSON Lines logger into all flows, `syllago audit` CLI with filters, CSV + SIEM export. |
| Trust policy | Full policy engine | Per-tier rules, allowed identities/issuers, signature requirements, revocation checking. |
| Execution model | Scope now, execute in phases | Too large for a single session. Beads track each phase. |

## Dependency Graph

```
Phase 1 (Foundation -- parallel, no dependencies)
  syllago-ibmi  Audit logging
  syllago-20v7  Script scanning
  syllago-2qml  Trust tiers

Phase 2 (depends on script scanning)
  syllago-uquc  Pluggable scanner  <-- syllago-20v7

Phase 3 (depends on audit + trust)
  syllago-96ao  Signing  <-- syllago-ibmi, syllago-2qml

Phase 4 (depends on signing + pluggable scanner + trust)
  syllago-9e83  Revocation      <-- syllago-96ao
  syllago-t45m  Policy engine   <-- syllago-96ao, syllago-uquc, syllago-2qml
```

## Phase 1: Foundation

### Audit Logging (syllago-ibmi)
- Wire existing `audit.Logger` into install/uninstall/import/scan flows
- New `audit.WriteInstall()`, `WriteUninstall()`, `WriteScan()` convenience functions
- `syllago audit` CLI: `--since`, `--type`, `--event`, `--hook`, `--json` filters
- CSV + SIEM-compatible export formats
- Size-based rotation: 10MB active + 1 backup
- Log path: `~/.syllago/audit.jsonl`
- ~1000 lines, low risk

### Script Scanning (syllago-20v7)
- Extend `hook_security.go` to scan script files on disk
- File types: `.sh`, `.ps1`, `.py`, `.js`, `.ts`, `.rb`, executable bit (no ext)
- Language-specific patterns:
  - Python: urllib, requests, subprocess, shutil.rmtree
  - JS/TS: fetch, http/https, axios, child_process, fs.rmSync
  - PowerShell: Invoke-WebRequest, Start-Process, Remove-Item -Recurse
  - Ruby: Net::HTTP, open-uri, FileUtils.rm_rf
- Recursive `provider_data` JSON string scanning
- New `ScanHookFull(hookDir, manifestContent)` combining manifest + script scanning
- ~800 lines, low risk

### Trust Tiers (syllago-2qml)
- Three tiers with distinct install behavior:
  - `trusted`: auto-approve, no prompt
  - `verified`: confirm once + summary
  - `community`: review each item, default No
- Config additions: `Registry.Policy` (per-content-type overrides), `Registry.Pin` (commit SHA)
- Policy override rule: can only restrict, not escalate (`min(baseTrust, override)`)
- Functions: `TrustLevel()`, `RequiresReview()`, `RequiresItemReview()`, `EffectiveTrust()`
- CLI: `registry trust <name> [level]`, `registry pin <name>`
- Install flow gate between `resolveItem()` and `resolveTarget()`
- ~600 lines, low risk

## Phase 2: Pluggable Scanner (syllago-uquc)

- `HookScanner` interface: `Name()` + `Scan(hookDir) (ScanResult, error)`
- `BuiltinScanner`: wraps regex patterns + script scanning from Phase 1
- `ExternalScanner`: subprocess execution with JSON protocol
  - Exit codes: 0=clean, 1=findings, 2+=error
  - 30s timeout, SIGKILL after 5s grace
- `ChainScanners()`: runs all scanners, merges results, fail-open per scanner
- CLI: `--hook-scanner /path/to/wrapper` flag on install
- Severity-based behavior:
  - LOW/INFO: print, proceed
  - MEDIUM: print warning, proceed
  - HIGH: block unless `--force`
- ~1000 lines, low risk

## Phase 3: Signing (syllago-96ao)

- `ContentDigest(hookDir) []byte`: SHA-256 of all files (excludes .syllago.yaml)
- Sigstore implementation:
  - Verify: `sigstore-go` library (embedded, no external dep at runtime)
  - Sign: shell out to `cosign` binary (optional, publishers only)
  - Rekor transparency log integration for audit trail
- GPG implementation:
  - `ProtonMail/go-crypto` for sign + verify
  - No external `gpg` binary required
- `TrustPolicy` evaluation: `VerifyAgainstPolicy(results, policy) error`
- CLI: `syllago sign <path> [--method sigstore|gpg] [--key ID]`
- CLI: `syllago verify <path> [--policy FILE]`
- Integration: verification gate in `installHook()` before duplicate check
- ~2000 lines, medium risk (crypto handling, policy evaluation)

## Phase 4: Enforcement

### Revocation (syllago-9e83)
- Per-registry `revocations.json` with entries (hook_id, content_hash, severity, reason)
- Check points: install-time (block), sync-time (fetch + merge)
- Local cache: `~/.syllago/cache/revocations.json`
- Staleness warning if local index >7 days old
- Local override: `syllago allow <hook-id>`
- CLI: `syllago revoke <hook-id> [--hash SHA] [--severity critical|high|medium]`
- Audit events for all revocation checks
- ~1200 lines, medium risk

### Policy Engine (syllago-t45m)
- Orchestration layer: trust + signing + scanning combined
- Per-tier configurable rules (e.g., community requires signature)
- Allowed identities (glob patterns for emails)
- Allowed issuers (OIDC providers for Sigstore)
- Signature requirements per trust level
- Install-time enforcement combining all security signals
- ~800 lines, medium risk

## External Dependencies

| Package | Used by | Purpose | Binary impact |
|---------|---------|---------|---------------|
| `sigstore/sigstore-go` | Signing | Verify Sigstore bundles | ~5 MB transitive |
| `ProtonMail/go-crypto` | Signing | GPG sign + verify | ~2 MB |
| None | All others | Pure stdlib | None |

## Architecture Notes

- **Audit** logs what happened (observability)
- **Signing** verifies where it came from (provenance)
- **Scanning** checks what it does (safety)
- **Trust** defines how much to trust it (policy)
- **Revocation** handles post-publish compromise (incident response)
- **Policy engine** ties them all together (enforcement)

Each component is independently useful. The policy engine is the integration point.
