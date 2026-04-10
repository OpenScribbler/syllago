# Implementation Plan: Registry Trust Tiers

**Bead:** syllago-plg8
**Status:** Draft
**Date:** 2026-03-22

---

## Background

Syllago currently treats all registries equally at install time. The security considerations spec (Section 1 -- Threat Model) identifies malicious hook authors and compromised registries as primary threat actors. The existing infrastructure includes:

- **Registry allowlist** (`allowed_registries` in config) -- binary allow/deny at the URL level
- **Visibility detection** (`visibility.go`) -- public/private/unknown classification
- **Security warning** -- displayed on first `registry add`, brief reminder on subsequent adds

These mechanisms control *which* registries can be added but provide no graduated trust model for *what happens* when content is installed from different registries. A team admin who adds their company's internal registry and a random community registry gets the same install experience for both.

Trust tiers fill this gap: they let users (and team admins) declare how much they trust each registry, and syllago adjusts its install behavior accordingly.

---

## 1. Trust Level Definitions

Three trust tiers, ordered from most to least trusted:

| Tier | Meaning | Install Behavior |
|------|---------|-----------------|
| `trusted` | Organizational/internal registry. Content has been vetted by a team the user trusts. | Auto-approve installs. No confirmation prompt. |
| `verified` | Known community registry. Content is publicly reviewable but not internally vetted. | Install with brief summary. Single confirmation prompt. |
| `community` | Unknown or unvetted registry. Content has no provenance guarantees. | Install requires explicit review. Show content details + risk indicators. Require `y` confirmation per item (or `--yes` flag for batch). |

**Default tier:** `community`. Every registry starts here unless explicitly promoted. This is fail-safe -- the most restrictive behavior applies when no trust level is configured.

**Why three tiers (not two or five):**
- Two tiers (trusted/untrusted) doesn't capture the middle ground of "I know this registry exists and it's reputable, but I haven't audited every item."
- Five+ tiers add configuration burden without meaningfully different install behaviors. Three tiers map to three distinct UX flows: auto-approve, confirm-once, review-each.

---

## 2. Configuration Format

Trust tiers are configured per-registry in `config.json`. The `trust` field is added to the existing `Registry` struct.

### In `config.Registry`

```go
type Registry struct {
    Name                string     `json:"name"`
    URL                 string     `json:"url"`
    Ref                 string     `json:"ref,omitempty"`
    Trust               string     `json:"trust,omitempty"`               // "trusted", "verified", "community" (default: "community")
    Visibility          string     `json:"visibility,omitempty"`
    VisibilityCheckedAt *time.Time `json:"visibility_checked_at,omitempty"`
}
```

### Example config.json

```json
{
  "registries": [
    {
      "name": "acme/internal-rules",
      "url": "https://github.com/acme/internal-rules.git",
      "trust": "trusted"
    },
    {
      "name": "claude-community/skills",
      "url": "https://github.com/claude-community/skills.git",
      "trust": "verified"
    },
    {
      "name": "random-user/cool-hooks",
      "url": "https://github.com/random-user/cool-hooks.git"
    }
  ]
}
```

The third registry has no `trust` field and defaults to `community`.

### Trust level constants

```go
const (
    TrustTrusted   = "trusted"
    TrustVerified  = "verified"
    TrustCommunity = "community"
)
```

### Merge behavior

Trust follows the existing registry merge pattern: project config overrides global for the same registry name. A team admin can set trust in the project config, and individual users cannot escalate it (because project config is git-tracked and reviewed).

**Note:** There is no enforcement preventing a user from editing project config locally. Trust tiers are a UX guardrail, not a security boundary. The security boundary is the `allowed_registries` allowlist, which prevents unauthorized registries entirely.

---

## 3. Per-Registry Policy Overrides

Beyond the three trust tiers, registries can have per-item-type policy overrides. This handles the case where a trusted registry publishes hooks (high-risk) alongside rules (low-risk), and the admin wants different behavior for each.

### Configuration

```json
{
  "registries": [
    {
      "name": "acme/internal-rules",
      "url": "https://github.com/acme/internal-rules.git",
      "trust": "trusted",
      "policy": {
        "hooks": "verified",
        "mcp": "verified"
      }
    }
  ]
}
```

This says: "Trust this registry for everything *except* hooks and MCP configs, which require the `verified` install flow (single confirmation)."

### In `config.Registry`

```go
type Registry struct {
    // ... existing fields ...
    Trust  string            `json:"trust,omitempty"`
    Policy map[string]string `json:"policy,omitempty"` // content-type → trust override
}
```

### Resolution logic

```
EffectiveTrust(registry, contentType) =
    if registry.Policy[contentType] exists:
        return registry.Policy[contentType]
    return registry.Trust (or "community" if empty)
```

Policy overrides can only *restrict* (not escalate) relative to the base trust tier. If a registry is `verified` and the policy sets `hooks: "trusted"`, the effective trust for hooks is still `verified`. Rationale: escalation through per-type overrides undermines the purpose of setting a base tier.

**Implementation detail:** `EffectiveTrust()` computes `min(baseTrust, policyOverride)` using a rank function similar to the existing `stricterOf()` in visibility.go.

---

## 4. Registry Pinning

Registry pinning ensures that content installed from a registry can be traced back to an exact version, preventing supply-chain drift.

### Pin format

Pinning uses the existing `ref` field on `config.Registry`, extended with commit SHA support:

```json
{
  "name": "acme/rules",
  "url": "https://github.com/acme/rules.git",
  "ref": "v2.1.0",
  "pin": "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
}
```

| Field | Purpose |
|-------|---------|
| `ref` | Human-readable reference (branch, tag). Already exists. Used for `git clone --branch`. |
| `pin` | Full commit SHA. If set, `registry sync` verifies that the ref resolves to this exact commit after pulling. |

### Behavior

- **On `registry add --pin`:** After cloning, record the current HEAD commit SHA as `pin`.
- **On `registry sync`:** If `pin` is set, after `git pull`, verify HEAD matches `pin`. If it doesn't match, warn and refuse to update (the ref moved). User must explicitly `registry pin --update` to accept the new commit.
- **On `registry sync --force`:** Skip pin verification (for when you intentionally want to advance).

### Pin enforcement by trust tier

| Tier | Pin behavior |
|------|-------------|
| `trusted` | Pin optional. Sync proceeds normally. |
| `verified` | Pin recommended. Warn on sync if unpinned. |
| `community` | Pin required for hooks/MCP content types. Warn for other types. |

This graduated enforcement reflects that community registries have the highest supply-chain risk, and hooks/MCP are the highest-risk content types (they execute code).

### In `config.Registry`

```go
type Registry struct {
    // ... existing fields ...
    Pin string `json:"pin,omitempty"` // commit SHA for version pinning
}
```

---

## 5. Integration with Install Flow

The install flow (CLI `install` command and TUI install modal) must consult the trust tier before proceeding.

### Decision function

New file: `cli/internal/registry/trust.go`

```go
// TrustLevel returns the effective trust tier for installing content of the
// given type from the named registry. Returns TrustCommunity if the registry
// is not found or has no trust configured.
func TrustLevel(cfg *config.Config, registryName string, contentType catalog.ContentType) string

// RequiresReview returns true if the trust level requires user confirmation
// before install.
func RequiresReview(trustLevel string) bool

// RequiresItemReview returns true if the trust level requires per-item
// confirmation (community tier).
func RequiresItemReview(trustLevel string) bool
```

### Install flow changes

Current flow:
```
resolve item → resolve target → install (symlink/copy/merge)
```

New flow:
```
resolve item → determine trust tier → gate on trust → install
```

**Gate behavior by tier:**

| Tier | CLI behavior | TUI behavior |
|------|-------------|-------------|
| `trusted` | Install silently. Print result. | Install immediately. Show success toast. |
| `verified` | Print item summary (name, type, source). Prompt `Install? [Y/n]`. | Show install modal with item details. Single "Install" button. |
| `community` | Print item details + risk indicators (has scripts? has hooks? modifies settings?). Prompt `Install? [y/N]` (default No). | Show install modal with expanded detail view. Risk badges visible. Require explicit "Install" click. |

**Note the default flip:** `verified` defaults to Yes (`[Y/n]`), `community` defaults to No (`[y/N]`). This subtle UX difference makes community installs require deliberate opt-in.

### `--yes` flag

The existing `--yes` / `-y` flag on install bypasses confirmation for all tiers. This is intentional for scripted/CI use. The flag is already an explicit user choice to skip prompts.

### Batch install

When installing multiple items (e.g., `--type skills`), group by trust tier:
1. Install all `trusted` items silently.
2. Show summary of `verified` items, single confirmation.
3. Show each `community` item individually (or summarize with `--yes`).

### Content type risk indicators

Some content types carry higher inherent risk. These are shown in the review prompt regardless of trust tier (but only block on `community`):

| Content type | Risk level | Why |
|-------------|-----------|-----|
| hooks | High | Execute arbitrary code on events |
| mcp | High | Run persistent servers with network access |
| commands | Medium | Execute on user invocation |
| skills, rules, agents, prompts | Low | Text/configuration only |

---

## 6. CLI Interface for Managing Trust Levels

### Setting trust on add

```bash
# Add with explicit trust
syllago registry add https://github.com/acme/rules.git --trust trusted

# Add with pin
syllago registry add https://github.com/acme/rules.git --trust verified --pin
```

If `--trust` is not specified, defaults to `community`.

### Changing trust for existing registries

```bash
# Set trust level
syllago registry trust acme/rules trusted

# Set trust with per-type policy override
syllago registry trust acme/rules trusted --restrict hooks=verified,mcp=verified

# View current trust configuration
syllago registry trust acme/rules

# Pin a registry to its current commit
syllago registry pin acme/rules

# Update pin to current HEAD
syllago registry pin acme/rules --update

# Remove pin
syllago registry pin acme/rules --clear
```

### New subcommands

| Command | Description |
|---------|-------------|
| `registry trust <name> [level]` | Get or set trust level. With no level arg, prints current trust config. |
| `registry pin <name>` | Pin registry to current HEAD commit SHA. |
| `registry pin <name> --update` | Update pin to current HEAD. |
| `registry pin <name> --clear` | Remove pin. |

### registry list output

Add trust tier to the `registry list` output:

```
NAME                  STATUS    TRUST       VERSION   URL / DESCRIPTION
────────────────────  ────────  ──────────  ────────  ────────────────────────────
acme/internal-rules   cloned    trusted     2.1.0     https://github.com/acme/...
claude-community/...  cloned    verified    1.0.0     https://github.com/claude-...
random-user/cool-...  cloned    community   -         https://github.com/random-...
```

JSON output includes `trust`, `policy`, and `pin` fields.

---

## 7. Test Cases

### Unit tests: `cli/internal/registry/trust_test.go`

| Test | Description |
|------|-------------|
| `TestTrustLevel_DefaultsCommunity` | Registry with no trust field returns `community`. |
| `TestTrustLevel_ExplicitTiers` | Each of the three tiers returns correctly. |
| `TestTrustLevel_PolicyOverride` | Per-type policy returns override value. |
| `TestTrustLevel_PolicyCannotEscalate` | Policy override stricter than base tier: returns base. E.g., base=`verified`, policy hooks=`trusted` -> effective=`verified`. |
| `TestTrustLevel_RegistryNotFound` | Unknown registry name returns `community`. |
| `TestRequiresReview` | `trusted` -> false, `verified` -> true, `community` -> true. |
| `TestRequiresItemReview` | `trusted` -> false, `verified` -> false, `community` -> true. |
| `TestEffectiveTrust_StricterWins` | Verify the rank function: community > verified > trusted (community is most restrictive). |

### Unit tests: `cli/internal/registry/registry_test.go` (extend existing)

| Test | Description |
|------|-------------|
| `TestRegistryConfig_TrustSerialization` | Trust field round-trips through JSON marshal/unmarshal. |
| `TestRegistryConfig_PolicySerialization` | Policy map round-trips correctly. |
| `TestRegistryConfig_PinSerialization` | Pin field round-trips correctly. |

### Config merge tests: `cli/internal/config/config_test.go` (extend existing)

| Test | Description |
|------|-------------|
| `TestMerge_TrustPreserved` | Trust field survives global+project merge. |
| `TestMerge_ProjectOverridesTrust` | Project trust overrides global trust for same registry. |

### CLI command tests: `cli/cmd/syllago/registry_cmd_test.go` (extend existing)

| Test | Description |
|------|-------------|
| `TestRegistryAdd_TrustFlag` | `--trust verified` sets trust in config. |
| `TestRegistryAdd_DefaultTrust` | No `--trust` flag sets empty (resolves to community). |
| `TestRegistryAdd_InvalidTrust` | `--trust supreme` returns error. |
| `TestRegistryTrust_SetLevel` | `registry trust <name> verified` updates config. |
| `TestRegistryTrust_ShowCurrent` | `registry trust <name>` prints current trust. |
| `TestRegistryPin_SetsCommitSHA` | `registry pin <name>` records HEAD SHA. |
| `TestRegistryPin_Update` | `--update` records new HEAD SHA. |
| `TestRegistryPin_Clear` | `--clear` removes pin from config. |

### Install flow tests: `cli/cmd/syllago/install_cmd_test.go` (extend existing)

| Test | Description |
|------|-------------|
| `TestInstall_TrustedSkipsPrompt` | Item from trusted registry installs without confirmation. |
| `TestInstall_CommunityRequiresConfirm` | Item from community registry requires confirmation (simulated stdin). |
| `TestInstall_YesFlagBypassesReview` | `--yes` flag skips all trust-based prompts. |
| `TestInstall_HighRiskContentShowsWarning` | Hooks from verified registry show risk indicator. |

### Integration tests

| Test | Description |
|------|-------------|
| `TestTrustTier_FullLifecycle` | Add registry -> set trust -> install item -> verify behavior matches tier. |
| `TestPin_SyncVerification` | Pin registry -> advance remote HEAD -> sync -> verify pin mismatch warning. |

---

## Implementation Order

1. **Config changes** -- Add `Trust`, `Policy`, `Pin` fields to `config.Registry`. No behavior change yet.
2. **Trust resolution** -- New `trust.go` with `TrustLevel()`, `RequiresReview()`, etc. Pure functions, easy to test.
3. **CLI commands** -- `registry trust` and `registry pin` subcommands.
4. **`registry add` integration** -- `--trust` and `--pin` flags.
5. **Install flow gating** -- Wire trust checks into `install` command.
6. **TUI integration** -- Wire trust checks into install modal.
7. **`registry list` output** -- Show trust tier in list/JSON output.

Steps 1-4 are low-risk and can land together. Step 5 is the behavioral change. Step 6 follows the same logic but in the TUI context.

---

## Files Changed

| File | Change |
|------|--------|
| `cli/internal/config/config.go` | Add `Trust`, `Policy`, `Pin` fields to `Registry` |
| `cli/internal/registry/trust.go` | New: trust level resolution, review requirements |
| `cli/internal/registry/trust_test.go` | New: unit tests |
| `cli/cmd/syllago/registry_cmd.go` | Add `--trust`/`--pin` flags, `trust`/`pin` subcommands |
| `cli/cmd/syllago/install_cmd.go` | Wire trust gate into install flow |
| `cli/internal/tui/modal.go` | Wire trust gate into TUI install modal |
| `cli/internal/config/config_test.go` | Extend merge tests |
| `cli/cmd/syllago/registry_cmd_test.go` | Extend CLI tests |
| `cli/cmd/syllago/install_cmd_test.go` | Extend install tests |

---

## Non-Goals (Explicit)

- **Cryptographic verification** -- Trust tiers are a UX layer. They don't verify content integrity (that's a separate feature: content hashing/signing per the security spec Section 3).
- **Remote trust registry** -- No central database of trusted registries. Trust is configured locally per user/project.
- **Automatic trust promotion** -- No mechanism to auto-promote registries based on age or usage. Trust changes are always explicit user actions.
- **Trust inheritance** -- Loadouts don't inherit trust from their source registry. Each content item is evaluated against its registry's trust tier at install time.
