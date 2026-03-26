# Syllago CLI Threat Inventory

**Date:** 2026-03-26
**Scope:** CLI mechanisms only (not TUI, not registry content quality)
**Focus:** Risks inherent to how syllago operates on the user's machine

---

## Executive Summary

Syllago's codebase shows strong security awareness in several areas: atomic file writes, symlink-following defenses, item name validation, privacy gates with fail-closed defaults, sandbox with namespace isolation, and git clone hardening. No telemetry or phone-home behavior exists.

**Two findings warrant action before public release.** The remaining findings range from defense-in-depth improvements to documentation items.

---

## Findings by Severity

### CRITICAL: None

### HIGH (2) — Fix before release

| # | Bead | Finding | Risk |
|---|------|---------|------|
| H1 | `syllago-cfiug` | [Self-update downloads binary without signature verification](#h1) | Compromised GitHub release or MITM replaces binary |
| H2 | `syllago-6c0oi` | [MCP install silently overwrites user-defined server keys](#h2) | User loses custom MCP config without warning |

### MEDIUM (9) — Fix or accept with documentation

| # | Bead | Finding | Risk |
|---|------|---------|------|
| M1 | `syllago-iqk2r` | [Registry sync missing git hardening](#m1) | Post-merge hooks in registries execute on sync |
| M2 | `syllago-alufx` | [Hook security scanner not called during install](#m2) | Malicious hooks installed without any automated warning |
| M3 | `syllago-ytf4g` | [Hook event name used as sjson key without validation](#m3) | Crafted event names create nested JSON structures |
| M4 | `syllago-tup33` | [Hook matcher groups not whitelist-filtered before merge](#m4) | Arbitrary JSON keys injected into provider settings |
| M5 | `syllago-q6wat` | [JSONC comments destroyed during merge](#m5) | Zed/OpenCode users lose comments permanently |
| M6 | `syllago-xeel5` | [Absolute paths leak through promote pipeline](#m6) | BundledScriptMeta exposes filesystem paths in registries |
| M7 | `syllago-12b0h` | [.syllago/ state files not gitignored](#m7) | Audit logs, installed.json committed with absolute paths |
| M8 | `syllago-o5p0n` | [Sandbox auto-approves "low-risk" config changes](#m8) | Denylist approach misses new executable content types |
| M9 | `syllago-4wccv` | [Proxy allowedPorts stored but never enforced](#m9) | Port allowlist feature is broken/incomplete |

### LOW (7) — Defense-in-depth, no urgency

| # | Bead | Finding | Risk |
|---|------|---------|------|
| L1 | `syllago-jasq5` | Symlink creation TOCTOU race | Requires attacker write access to provider config dir |
| L2 | `syllago-ou39d` | Uninstall RemoveAll without scope validation | Misconfigured PathResolver could delete outside scope |
| L3 | `syllago-3v2f5` | User-configured provider paths not bounds-checked | Shared config could direct writes to sensitive paths |
| L4 | `syllago-pdsi7` | MCP backup is non-atomic and single-depth | .bak can be corrupted on crash or lost on next operation |
| L5 | `syllago-awjpt` | CleanStale removes dirs by name prefix without uid check | Shared systems only |
| L6 | `syllago-i1lj4` | Hook script symlink not resolved before containment check | Symlink in item dir could escape during copy |
| L7 | `syllago-8iomf` | installed.json/settings.json crash window | Orphan hooks if crash between two writes |

### INFO (Positive findings)

- **No telemetry or phone-home** — all HTTP is GET-only, no data uploads
- **Privacy gate system is well-designed** — fail-closed, anti-laundering, 4-gate model
- **Atomic writes for settings** — temp-then-rename pattern correctly implemented
- **Item name validation** — regex blocks all path traversal
- **MCP values whitelist-filtered** — typed struct strips unknown JSON fields
- **Sandbox hardening** — bubblewrap, capability drop, env filtering, git blocking
- **Git clone hardening** — hooks disabled, submodules blocked, system config disabled

---

## Detailed Findings

### H1: Self-update downloads binary without signature verification {#h1}

**Bead:** `syllago-cfiug` (P1)
**File:** `cli/internal/updater/updater.go:109-192`

**What happens:** `syllago update` downloads a binary from GitHub Releases and replaces the running executable. SHA-256 checksum verification exists, but checksums.txt is downloaded from the same source as the binary.

**Why it matters:** If GitHub Releases is compromised (account takeover, CI compromise) or MITM'd, an attacker controls both the binary and the checksums. The `signing` package has interfaces defined but is unimplemented.

**Mitigation:** Implement the signing package. At minimum, use cosign or GPG to sign checksums.txt with a key that isn't stored alongside the release. Alternatively, hardcode the signing public key in the binary.

---

### H2: MCP install silently overwrites user-defined server keys {#h2}

**Bead:** `syllago-6c0oi` (P1)
**File:** `cli/internal/installer/mcp.go:334-336`

**What happens:** `installMCP` uses `sjson.SetRawBytes` which unconditionally overwrites any existing MCP server entry with the same name. No collision check. A `.bak` file is created but the user isn't warned.

**Why it matters:** A user who has manually configured an MCP server named `my-server` loses their config silently if they install content with a same-named server. On uninstall, syllago deletes the key entirely — the original config is gone.

**Mitigation:** Before setting, check if the key exists and wasn't installed by syllago (via installed.json). If collision detected, warn and require `--force`.

---

### M1: Registry sync missing git hardening {#m1}

**Bead:** `syllago-iqk2r` (P1)
**File:** `cli/internal/registry/registry.go:156-170`

**What happens:** `Sync()` runs `git pull --ff-only` without the security hardening that `Clone()` applies. Missing: `GIT_CONFIG_NOSYSTEM=1`, `core.hooksPath=/dev/null`, `--no-recurse-submodules`.

**Why it matters:** A registry could add a `post-merge` git hook in an update. On the next sync, that hook executes with the user's permissions. This is the most actionable fix — a few lines to match Clone()'s hardening.

**Mitigation:** Apply identical env vars and config flags to Sync() as Clone().

---

### M2: Hook security scanner not called during install {#m2}

**Bead:** `syllago-alufx` (P2)
**File:** `cli/internal/installer/hooks.go:68` (installHook never calls ScanHookSecurity)

**What happens:** `hook_security.go` has a regex-based scanner for dangerous commands (curl, wget, nc, rm -rf). But `installHook()` never calls it. The scanner only runs during import/convert — not at install time.

**Why it matters:** Registry content can change between import and install (registry updates). The last chance to warn the user is at install time, and it's skipped.

**Mitigation:** Call `ScanHookSecurity` during install. Display warnings. Consider requiring `--allow-unsafe` for high-severity findings.

---

### M3: Hook event name used as sjson key without validation {#m3}

**Bead:** `syllago-ytf4g` (P2)
**File:** `cli/internal/installer/hooks.go:112`

**What happens:** The event name from hook JSON is used directly in `"hooks." + event + ".-1"`. Unlike MCP server names (validated by `IsValidItemName`), event names aren't validated.

**Why it matters:** An event name containing dots (e.g., `PreToolUse.evil.path`) causes sjson to create nested JSON structures instead of flat keys, corrupting settings.json.

**Mitigation:** Validate event names against a whitelist of known events, or at minimum ensure no dots/special characters.

---

### M4: Hook matcher groups not whitelist-filtered {#m4}

**Bead:** `syllago-tup33` (P2)
**File:** `cli/internal/installer/hooks.go:60-63`

**What happens:** The hook matcher group (everything except the "event" field) is inserted as raw JSON into settings.json. Unlike MCP (which roundtrips through a typed struct), hooks preserve all JSON fields including unknown ones.

**Why it matters:** A crafted hook.json could inject arbitrary keys into the provider's settings.json beyond the expected "matcher" and "hooks" fields.

**Mitigation:** Parse through a typed struct to strip unknown fields, matching the MCP approach.

---

### M5: JSONC comments destroyed during merge {#m5}

**Bead:** `syllago-q6wat` (P2)
**Files:** `cli/internal/installer/mcp.go:234-236`, `cli/internal/installer/jsonmerge.go:15`

**What happens:** For OpenCode, JSONC comments are stripped before merge and the clean JSON is written back — comments are permanently lost. For Zed (which uses JSONC natively), no stripping is applied at all, so sjson may produce corrupt output.

**Why it matters:** Zed's `settings.json` is officially JSONC. Either syllago corrupts it (no stripping) or destroys user comments (with stripping). Neither outcome is acceptable.

**Mitigation:** Use a JSONC-aware library, or refuse to merge into JSONC files and document the limitation.

---

### M6: Absolute paths leak through promote pipeline {#m6}

**Bead:** `syllago-xeel5` (P2)
**File:** `cli/internal/metadata/metadata.go:33`

**What happens:** `BundledScriptMeta.OriginalPath` stores the absolute filesystem path at add time. This survives `copyForPromote` and ends up in public registries.

**Why it matters:** Reveals the user's home directory, username, and filesystem layout to anyone browsing the registry.

**Mitigation:** Strip or relativize `OriginalPath` during promote.

---

### M7: .syllago/ state files not gitignored {#m7}

**Bead:** `syllago-12b0h` (P2)
**Files:** `.syllago/audit.jsonl`, `.syllago/installed.json`, `.syllago/snapshots/`

**What happens:** These files contain machine-local state (audit events with timestamps, absolute paths, install history) but aren't gitignored.

**Why it matters:** If committed, they expose development activity patterns and filesystem structure.

**Mitigation:** Add `.syllago/audit.jsonl`, `.syllago/installed.json`, `.syllago/snapshots/` to `.gitignore`.

---

### M8: Sandbox auto-approves "low-risk" config changes {#m8}

**Bead:** `syllago-o5p0n` (P3)
**File:** `cli/internal/sandbox/runner.go:246-254`

**What happens:** After sandbox exit, config changes are classified as high-risk (`mcpServers`, `hooks`, `commands`) or low-risk. Low-risk changes are auto-applied without prompting.

**Why it matters:** The denylist only checks three hardcoded strings. A provider could add new executable content types that syllago doesn't know about, and they'd be auto-approved.

**Mitigation:** Invert the logic — auto-approve only explicitly-known-safe keys, prompt for everything else.

---

### M9: Proxy allowedPorts stored but never enforced {#m9}

**Bead:** `syllago-4wccv` (P3)
**File:** `cli/internal/sandbox/proxy.go:19`

**What happens:** `--allow-port` flags populate `allowedPorts`, but `handleConn()` only checks `allowedDomains`. Port filtering is never applied.

**Why it matters:** Users configuring port restrictions get a false sense of security.

**Mitigation:** Add port-based filtering in `handleConn()` after the domain check.

---

### LOW Findings — Bead Reference

| # | Bead | Finding | File |
|---|------|---------|------|
| L1 | `syllago-jasq5` | Symlink creation TOCTOU race | `cli/internal/installer/symlink.go:22-28` |
| L2 | `syllago-ou39d` | Uninstall RemoveAll without scope validation | `cli/internal/installer/installer.go:331-332` |
| L3 | `syllago-3v2f5` | User-configured provider paths not bounds-checked | `cli/internal/config/resolver.go:28-43` |
| L4 | `syllago-pdsi7` | MCP backup is non-atomic and single-depth | `cli/internal/installer/jsonmerge.go:64-81` |
| L5 | `syllago-awjpt` | CleanStale removes dirs by prefix without uid check | `cli/internal/sandbox/staging.go:63-84` |
| L6 | `syllago-i1lj4` | Hook script symlink not resolved before containment | `cli/internal/installer/hooks.go:321-326` |
| L7 | `syllago-8iomf` | installed.json/settings.json crash window | `cli/internal/installer/hooks.go:118-141` |

See individual beads for full details, success criteria, and testing requirements.

---

## Network Operations Map

| Operation | Trigger | Auto? | Data Sent | TLS |
|-----------|---------|-------|-----------|-----|
| Update check | TUI launch | Yes (disableable) | User-Agent + IP to GitHub API | Yes |
| Binary download | `syllago update` | No | User-Agent to GitHub Releases | Yes |
| Registry clone | `registry add` | No | Git handshake to user URL | Depends |
| Registry sync | Auto-sync or manual | Configurable | Git handshake to registry remotes | Depends |
| Visibility probe | Privacy gate checks | Implicit | Repo owner/name to GitHub/GitLab/BB | Yes |
| Git push + PR | `syllago share` | No | Branch content + PR metadata | Depends |

**No telemetry.** No POST requests. No data uploads except user-initiated git push/PR.

---

## Release Readiness Assessment

**Can you release?** Yes, with caveats.

**Must fix (H1, H2, M1):**
- H1: Self-update without signatures is a real supply-chain risk. Either implement signing or disable auto-update for initial release and document manual verification.
- H2: Silent MCP key overwrite will cause user frustration and data loss. Add collision detection.
- M1: Sync git hardening gap is a few lines of code and closes a real code execution path.

**Should fix (M2-M4):**
- Hook install-time scanning, event name validation, and matcher whitelist filtering are all defense-in-depth improvements that reduce the blast radius of malicious registry content.

**Can defer (M5-M9, all Lows):**
- JSONC handling is Zed-specific and can be documented as a known limitation.
- Path leakage and gitignore are cleanup items.
- Sandbox improvements are for a feature that's already opt-in.

---

## Methodology

Five parallel adversarial agents examined:
1. Filesystem operations (symlinks, path handling, destructive ops)
2. JSON merge & settings modification (corruption, collisions, atomicity)
3. Network operations (all HTTP/git calls, telemetry check)
4. Code execution & hooks (injection, sandbox escape, execution model)
5. Data exposure & privacy (logging, path leakage, credentials, exports)

Each agent performed read-only analysis of the relevant source files.
