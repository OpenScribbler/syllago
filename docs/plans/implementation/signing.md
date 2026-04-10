# Implementation Plan: Cryptographic Signing for Hooks

**Bead:** syllago-p7uo
**Date:** 2026-03-22
**Status:** Plan (no code)

---

## 1. Recommended Approach: Sigstore/cosign over GPG

### Both Approaches

**Sigstore/cosign (keyless OIDC-based):**
- Author signs with `cosign sign-blob` using their GitHub/Google/Microsoft identity
- Verification checks the signature against the Sigstore transparency log (Rekor)
- No key files to manage, distribute, or rotate
- Identity is tied to an email/OIDC issuer, not a key fingerprint
- Requires network access for signing (OIDC flow) and verification (Rekor lookup)
- Go library: `github.com/sigstore/cosign/v2` and `github.com/sigstore/sigstore-go`

**GPG/PGP (traditional key-based):**
- Author signs with a GPG private key
- Verification requires the author's public key to be distributed and trusted
- Works fully offline
- Key management is the user's responsibility (generation, distribution, revocation)
- Go library: `golang.org/x/crypto/openpgp` (deprecated) or `github.com/ProtonMail/go-crypto`

### Recommendation: Sigstore as primary, GPG as secondary

**Why Sigstore first:**
- Key management is the hardest part of signing for adoption. Sigstore eliminates it entirely for the signer -- you authenticate with an identity you already have (GitHub account).
- For syllago's registry model (git-hosted content from GitHub users), Sigstore identity maps naturally to GitHub identity, which is already visible in commit history.
- Sigstore's transparency log provides a public, auditable record of who signed what and when -- valuable for the enterprise/workspace use case.
- The Go ecosystem has mature Sigstore libraries (`sigstore-go` is the official verification library).

**Why GPG as secondary:**
- Air-gapped environments (common in enterprise/government) cannot reach Sigstore's OIDC or Rekor endpoints.
- Some organizations already have GPG key infrastructure and policy.
- Offline verification is a hard requirement for some workflows.

**Why not GPG-only:**
- GPG key distribution is the adoption killer. Solo developers and small teams will not set up a keyserver or manually exchange key fingerprints. Sigstore's "sign with your GitHub identity" is frictionless.

---

## 2. New Files

### `cli/internal/signing/` (new package)

| File | Purpose |
|------|---------|
| `signing.go` | Core types, interfaces, and format constants |
| `sigstore.go` | Sigstore/cosign sign + verify implementation |
| `gpg.go` | GPG sign + verify implementation |
| `policy.go` | Trust policy loading and evaluation |
| `verify.go` | Unified verification entry point (dispatches to sigstore or gpg) |
| `signing_test.go` | Unit tests for core types and format |
| `sigstore_test.go` | Tests for Sigstore paths (with test fixtures, mocked Rekor) |
| `gpg_test.go` | Tests for GPG paths (with test key fixtures) |
| `policy_test.go` | Tests for trust policy evaluation |
| `verify_test.go` | Integration tests for the unified verify flow |
| `testdata/` | Test fixtures: signed manifests, test keys, tampered files |

### `cli/cmd/syllago/sign_cmd.go` (new file)

Sign and verify CLI commands.

### `cli/cmd/syllago/sign_cmd_test.go` (new file)

CLI command tests.

---

## 3. Function Signatures

### Core Types (`signing.go`)

```go
package signing

// SignatureMethod identifies the signing mechanism.
type SignatureMethod string

const (
    MethodSigstore SignatureMethod = "sigstore"
    MethodGPG      SignatureMethod = "gpg"
)

// Signature represents a single cryptographic signature on a content item.
type Signature struct {
    Method    SignatureMethod `yaml:"method"`              // "sigstore" or "gpg"
    Identity  string         `yaml:"identity,omitempty"`   // OIDC identity (sigstore) or key fingerprint (gpg)
    Issuer    string         `yaml:"issuer,omitempty"`     // OIDC issuer URL (sigstore only)
    Blob      string         `yaml:"blob"`                 // base64-encoded signature bytes
    Bundle    string         `yaml:"bundle,omitempty"`     // base64-encoded Sigstore bundle (sigstore only)
    Timestamp string         `yaml:"timestamp,omitempty"`  // RFC 3339 signing time
}

// ContentDigest computes the SHA-256 digest of all signable files in a hook directory.
// Files are sorted lexicographically, concatenated, and hashed.
// The .syllago.yaml file itself is excluded from the digest (it contains the signature).
func ContentDigest(hookDir string) ([]byte, error)

// Signer is the interface for signing content.
type Signer interface {
    Sign(digest []byte) (*Signature, error)
    Method() SignatureMethod
}

// Verifier is the interface for verifying signatures.
type Verifier interface {
    Verify(digest []byte, sig *Signature) (*VerifyResult, error)
    Method() SignatureMethod
}

// VerifyResult holds the outcome of a verification attempt.
type VerifyResult struct {
    Valid      bool
    Identity   string   // verified signer identity
    Issuer     string   // verified OIDC issuer (sigstore)
    Timestamp  string   // verified signing time
    Expired    bool     // true if signature predates trust policy window
    Warnings   []string // non-fatal issues
}
```

### Sigstore Implementation (`sigstore.go`)

```go
// NewSigstoreSigner creates a signer that uses Sigstore keyless signing.
// Initiates an OIDC flow (opens browser or uses ambient credentials).
func NewSigstoreSigner() (Signer, error)

// NewSigstoreVerifier creates a verifier that checks against the Sigstore
// transparency log (Rekor) and certificate chain.
func NewSigstoreVerifier() Verifier
```

### GPG Implementation (`gpg.go`)

```go
// NewGPGSigner creates a signer using a GPG private key.
// keyID is the GPG key fingerprint or email. If empty, uses the default key.
func NewGPGSigner(keyID string) (Signer, error)

// NewGPGVerifier creates a verifier using a GPG keyring.
// keyringPath is the path to a keyring file. If empty, uses the default keyring.
func NewGPGVerifier(keyringPath string) Verifier
```

### Unified Verify (`verify.go`)

```go
// VerifyHook verifies the signature(s) on a hook's .syllago.yaml metadata.
// Returns the verification result for each signature found.
// If no signatures are present, returns (nil, nil) -- unsigned is not an error.
func VerifyHook(hookDir string) ([]VerifyResult, error)

// VerifyAgainstPolicy checks verification results against a trust policy.
// Returns an error if the policy is not satisfied.
func VerifyAgainstPolicy(results []VerifyResult, policy *TrustPolicy) error
```

### Trust Policy (`policy.go`)

```go
// TrustPolicy controls which signatures are accepted.
type TrustPolicy struct {
    RequireSigned    bool              `yaml:"require_signed"`              // reject unsigned hooks
    TrustedIdentities []TrustedIdentity `yaml:"trusted_identities,omitempty"` // allowlist
    MaxAgeHours      int               `yaml:"max_age_hours,omitempty"`      // reject signatures older than this
}

// TrustedIdentity is an entry in the trust allowlist.
type TrustedIdentity struct {
    Identity string `yaml:"identity"`           // email or fingerprint
    Issuer   string `yaml:"issuer,omitempty"`   // OIDC issuer (sigstore only)
    Method   string `yaml:"method,omitempty"`   // "sigstore", "gpg", or "" (any)
}

// LoadTrustPolicy reads the trust policy from ~/.syllago/trust-policy.yaml.
// Returns a permissive default policy if the file does not exist.
func LoadTrustPolicy() (*TrustPolicy, error)

// Evaluate checks whether a set of verification results satisfies the policy.
// Returns nil if satisfied, or a descriptive error if not.
func (p *TrustPolicy) Evaluate(results []VerifyResult) error
```

---

## 4. CLI Command Interfaces

### `syllago sign`

```
syllago sign <path> [flags]

Sign a content item (hook, rule, skill, etc.) by adding a cryptographic
signature to its .syllago.yaml metadata.

Arguments:
  <path>    Path to the content item directory

Flags:
  --method string    Signing method: "sigstore" (default) or "gpg"
  --key string       GPG key ID or fingerprint (GPG only; default: default key)
  --yes              Skip confirmation prompt

Examples:
  syllago sign hooks/pre-commit-lint
  syllago sign hooks/pre-commit-lint --method gpg --key 0xABCD1234
```

**Behavior:**
1. Compute `ContentDigest(path)` over all files in the directory (excluding `.syllago.yaml`)
2. Create a `Signer` based on `--method`
3. Call `signer.Sign(digest)` -- for Sigstore, this opens a browser for OIDC auth
4. Load existing `.syllago.yaml` (or error if not found)
5. Append the `Signature` to the `signatures` field
6. Write the updated `.syllago.yaml`
7. Print: "Signed hooks/pre-commit-lint as jane@example.com (sigstore)"

### `syllago verify`

```
syllago verify <path> [flags]

Verify the cryptographic signature(s) on a content item.

Arguments:
  <path>    Path to the content item directory

Flags:
  --policy string    Path to trust policy file (default: ~/.syllago/trust-policy.yaml)
  --json             Output results as JSON

Examples:
  syllago verify hooks/pre-commit-lint
  syllago verify hooks/pre-commit-lint --policy ./strict-policy.yaml
```

**Behavior:**
1. Call `VerifyHook(path)` to get verification results
2. If `--policy` is set or `~/.syllago/trust-policy.yaml` exists, evaluate against policy
3. Print results:
   - Valid: "Verified: signed by jane@example.com (sigstore) at 2026-03-22T10:00:00Z"
   - Invalid: "FAILED: signature does not match content (files may have been tampered with)"
   - Unsigned: "Unsigned: no signatures found"
   - Policy violation: "BLOCKED: signer jane@example.com is not in trusted identities"

---

## 5. `.syllago.yaml` Signatures Field Format

The `signatures` field is added to the existing `Meta` struct in `cli/internal/metadata/metadata.go`:

```yaml
# .syllago.yaml
format_version: 1
id: "abc123"
name: "pre-commit-lint"
description: "Lint check before commit"
type: "hooks"
author: "jane@example.com"
version: "1.0.0"

# New field -- array of signatures
signatures:
  - method: sigstore
    identity: "jane@example.com"
    issuer: "https://accounts.google.com"
    blob: "MEUCIQDx...base64..."
    bundle: "eyJtZWRp...base64..."
    timestamp: "2026-03-22T10:00:00Z"

  - method: gpg
    identity: "0xABCD1234EFGH5678"
    blob: "iQIzBAAB...base64..."
    timestamp: "2026-03-22T10:05:00Z"
```

**Design decisions:**
- **Array, not single value:** Allows multiple signatures (e.g., author + org co-sign).
- **Stored in `.syllago.yaml`, not a separate file:** Keeps the metadata together. The digest excludes `.syllago.yaml` itself, so adding a signature does not invalidate other signatures.
- **`blob` is the raw signature, `bundle` is Sigstore-specific:** GPG signatures are self-contained in `blob`. Sigstore needs the bundle (certificate chain + Rekor entry) for offline verification.

**Changes to `metadata.Meta` struct:**

```go
type Meta struct {
    // ... existing fields ...
    Signatures []Signature `yaml:"signatures,omitempty"` // from signing package
}
```

The `Signature` type is defined in the `signing` package. The `metadata` package imports it. This is a one-way dependency: `metadata` -> `signing` (types only), `signing` -> `metadata` (never).

---

## 6. Key Management

### Sigstore (keyless)

- **No keys to manage.** The signer authenticates via OIDC (GitHub, Google, Microsoft).
- The OIDC identity (email) is embedded in a short-lived certificate issued by Fulcio.
- The certificate and signature are recorded in Rekor (transparency log).
- Verification checks the Rekor entry -- no need to distribute keys.

**Where identity lives:** In the Sigstore infrastructure. The user's GitHub/Google/Microsoft account IS the key. No files on disk.

**CI/CD signing:** For automated signing in CI, Sigstore supports ambient OIDC credentials (GitHub Actions OIDC token, GCP workload identity). No secrets to store.

### GPG

- **Private key:** User's existing GPG keyring (`~/.gnupg/`). Syllago does not manage GPG keys -- it delegates to the `gpg` binary or `go-crypto` library.
- **Public key distribution:** Two options:
  1. **Keyserver:** User uploads their public key to a keyserver. Verifier fetches by fingerprint.
  2. **In-repo:** Public key file committed to the registry repo (e.g., `.syllago/keys/`). Verifier reads from the local clone.
- **Syllago's role:** Syllago calls GPG for sign/verify but does NOT manage key generation, distribution, or revocation. That is the user's responsibility.

### Key files on disk

| File | Location | Purpose |
|------|----------|---------|
| Trust policy | `~/.syllago/trust-policy.yaml` | Controls which identities/keys are trusted |
| GPG keyring | `~/.gnupg/` (system default) | User's GPG keys (not managed by syllago) |

Syllago creates NO key files. It reads trust policy and delegates to existing key infrastructure.

---

## 7. Trust Policy Configuration Format

**File:** `~/.syllago/trust-policy.yaml`

```yaml
# Trust policy for hook signature verification.
# When this file does not exist, all signatures are accepted
# and unsigned content is allowed.

# Reject hooks with no signatures. Default: false.
require_signed: false

# Maximum signature age in hours. Signatures older than this
# are treated as expired. 0 = no expiry check. Default: 0.
max_age_hours: 8760  # 1 year

# Trusted identities. When non-empty, at least one signature
# must match an entry in this list.
# When empty, any valid signature is accepted.
trusted_identities:
  - identity: "jane@example.com"
    issuer: "https://github.com/login/oauth"
    method: sigstore

  - identity: "security-team@acme.corp"
    issuer: "https://accounts.google.com"
    method: sigstore

  - identity: "0xABCD1234EFGH5678"
    method: gpg
```

**Evaluation logic (in order):**
1. If `require_signed` is true and no signatures exist, reject.
2. For each signature on the content:
   a. Verify cryptographic validity (signature matches digest).
   b. If `max_age_hours > 0`, check that the signature is not older than the limit.
   c. If `trusted_identities` is non-empty, check that the signer matches at least one entry.
3. If at least one signature passes all checks, accept.
4. If no signature passes, reject with a descriptive error.

**Default policy (no file):** Accept everything. Unsigned content is allowed. Any valid signature is trusted. This is the solo developer default -- signing is opt-in.

---

## 8. Integration with Install Flow

### Where verification happens

Verification gates hook installation in `installHook()` in `cli/internal/installer/hooks.go`. The check happens early, before any filesystem mutations.

**Modified function: `installHook`** (lines 67-143 of `hooks.go`)

Insert verification between `parseHookFile` (line 69) and the duplicate check (line 83):

```
existing flow:
  parseHookFile -> resolveHookScripts -> check duplicate -> snapshot -> write

new flow:
  parseHookFile -> VERIFY SIGNATURE -> resolveHookScripts -> check duplicate -> snapshot -> write
```

**Pseudocode for the verification gate:**

```go
// After parseHookFile, before resolveHookScripts:

// Load trust policy (returns permissive default if no file)
policy, err := signing.LoadTrustPolicy()
if err != nil {
    return "", fmt.Errorf("loading trust policy: %w", err)
}

// Verify signatures on the hook
results, err := signing.VerifyHook(item.Path)
if err != nil {
    return "", fmt.Errorf("verifying hook signature: %w", err)
}

// Check against policy
if err := signing.VerifyAgainstPolicy(results, policy); err != nil {
    return "", fmt.Errorf("hook blocked by trust policy: %w", err)
}
```

### Other integration points

| Location | Change |
|----------|--------|
| `cli/internal/installer/hooks.go` `installHook()` | Add verification gate (described above) |
| `cli/internal/metadata/metadata.go` `Meta` struct | Add `Signatures` field |
| `cli/cmd/syllago/main.go` | Register `signCmd` and `verifyCmd` |
| `cli/cmd/syllago/sign_cmd.go` | New file: `syllago sign` and `syllago verify` commands |
| `cli/cmd/syllago/install_cmd.go` | Add `--skip-verify` flag to bypass verification |
| `cli/internal/installer/installer.go` | Thread `SkipVerify` option through `Install()` |
| `cli/internal/tui/detail.go` | Display signature status in item detail view |

### TUI integration

The TUI item detail view should show signature status when viewing a hook:
- "Signed by jane@example.com (sigstore)" with a green checkmark
- "Unsigned" with a yellow warning
- "Signature invalid" with a red X

This is a display-only change in the detail panel -- the verification gate is in the installer, not the TUI.

---

## 9. Test Cases

### Unit Tests (`signing_test.go`)

| Test | Description |
|------|-------------|
| `TestContentDigest_StableOrder` | Same files in different directory orders produce the same digest |
| `TestContentDigest_ExcludesSyllagoYaml` | `.syllago.yaml` is not included in the digest |
| `TestContentDigest_IncludesAllFiles` | All files in the directory (scripts, JSON, etc.) are included |
| `TestContentDigest_EmptyDir` | Empty directory produces a deterministic digest |

### Sigstore Tests (`sigstore_test.go`)

| Test | Description |
|------|-------------|
| `TestSigstoreSign_MockOIDC` | Sign with mocked OIDC flow produces a valid Signature struct |
| `TestSigstoreVerify_ValidSignature` | Valid signature + valid bundle passes verification |
| `TestSigstoreVerify_TamperedContent` | Valid signature but digest mismatch returns `Valid: false` |
| `TestSigstoreVerify_InvalidBundle` | Corrupted bundle returns an error |
| `TestSigstoreVerify_ExpiredCert` | Signature with expired certificate returns `Expired: true` |

Note: Sigstore tests use test fixtures with pre-computed signatures. Network-dependent tests (real Rekor) are gated behind `SYLLAGO_TEST_NETWORK=1`.

### GPG Tests (`gpg_test.go`)

| Test | Description |
|------|-------------|
| `TestGPGSign_TestKey` | Sign with a test GPG key produces a valid Signature struct |
| `TestGPGVerify_ValidSignature` | Valid signature verified against the test public key |
| `TestGPGVerify_TamperedContent` | Digest mismatch returns `Valid: false` |
| `TestGPGVerify_UnknownKey` | Signature from a key not in the keyring returns an error |
| `TestGPGVerify_RevokedKey` | Signature from a revoked key returns an error |

GPG tests use a test keyring in `testdata/` -- no dependency on the user's actual keyring.

### Policy Tests (`policy_test.go`)

| Test | Description |
|------|-------------|
| `TestPolicy_DefaultPermissive` | No policy file -> unsigned content is allowed |
| `TestPolicy_RequireSigned_Unsigned` | `require_signed: true` + unsigned content -> rejected |
| `TestPolicy_RequireSigned_Signed` | `require_signed: true` + valid signature -> accepted |
| `TestPolicy_TrustedIdentities_Match` | Signer matches an allowlist entry -> accepted |
| `TestPolicy_TrustedIdentities_NoMatch` | Signer not in allowlist -> rejected |
| `TestPolicy_MaxAge_Fresh` | Signature within age limit -> accepted |
| `TestPolicy_MaxAge_Expired` | Signature older than limit -> rejected |
| `TestPolicy_MultipleSignatures_OneValid` | Multiple sigs, one passes policy -> accepted |

### Integration Tests (`verify_test.go`)

| Test | Description |
|------|-------------|
| `TestVerifyHook_SignedAndValid` | Hook with valid signature passes `VerifyHook` |
| `TestVerifyHook_SignedAndTampered` | Hook with valid sig but modified script fails |
| `TestVerifyHook_Unsigned` | Hook with no signatures returns `(nil, nil)` |
| `TestVerifyHook_MultipleSignatures` | Hook with both Sigstore and GPG sigs verifies both |

### Install Flow Tests (`hooks_test.go` additions)

| Test | Description |
|------|-------------|
| `TestInstallHook_VerifyPass` | Signed hook + permissive policy installs normally |
| `TestInstallHook_VerifyFail_Tampered` | Tampered signed hook is rejected before install |
| `TestInstallHook_VerifyFail_PolicyBlock` | Valid sig but untrusted identity is rejected |
| `TestInstallHook_Unsigned_PermissivePolicy` | Unsigned hook + default policy installs normally |
| `TestInstallHook_Unsigned_StrictPolicy` | Unsigned hook + `require_signed: true` is rejected |
| `TestInstallHook_SkipVerify` | `--skip-verify` flag bypasses all verification |

### CLI Command Tests (`sign_cmd_test.go`)

| Test | Description |
|------|-------------|
| `TestSignCmd_CreatesSignature` | `syllago sign` adds a signature to `.syllago.yaml` |
| `TestSignCmd_NoMeta` | `syllago sign` on a directory without `.syllago.yaml` errors |
| `TestVerifyCmd_ValidOutput` | `syllago verify` prints "Verified" for valid signature |
| `TestVerifyCmd_TamperedOutput` | `syllago verify` prints "FAILED" for tampered content |
| `TestVerifyCmd_UnsignedOutput` | `syllago verify` prints "Unsigned" for no signatures |
| `TestVerifyCmd_JSONOutput` | `--json` flag outputs structured JSON |

---

## 10. Solo Developer Experience

**Signing is entirely optional.** The system is designed so that a solo developer who never runs `syllago sign` experiences zero friction:

1. **No policy file by default.** When `~/.syllago/trust-policy.yaml` does not exist, the default policy allows everything: unsigned content installs normally, any valid signature is accepted.

2. **No new flags required.** `syllago install` works identically to today when there are no signatures and no trust policy.

3. **Verification is a no-op for unsigned content.** `VerifyHook()` on a directory with no `signatures` field in `.syllago.yaml` returns `(nil, nil)`. The default policy evaluation of nil results returns nil (success).

4. **`syllago sign` is opt-in.** A solo developer can start signing their hooks whenever they want. The first time they run `syllago sign`, Sigstore opens a browser for OIDC auth -- that is the entire setup.

5. **`syllago verify` is informational.** Running it on unsigned content prints "Unsigned: no signatures found" and exits 0. It does not block anything unless a trust policy says otherwise.

6. **Trust policy is opt-in.** Only organizations or security-conscious users create `trust-policy.yaml`. The moment they do, they opt into enforcement. The CLI prints a message when a policy is active: "Trust policy loaded from ~/.syllago/trust-policy.yaml".

**Progression path for a solo developer:**
1. Start: No signing, no policy. Everything works.
2. Curious: Run `syllago sign hooks/my-hook`. Now your hooks are signed.
3. Sharing: Others can run `syllago verify hooks/my-hook` to confirm your identity.
4. Strict: Create a trust policy to only install hooks from known identities.

Each step is independent and reversible. No step is required by a previous step.

---

## Dependencies

### New Go dependencies

| Package | Purpose | Size impact |
|---------|---------|-------------|
| `github.com/sigstore/sigstore-go` | Sigstore verification (bundles, Rekor, Fulcio) | ~5 MB (transitive) |
| `github.com/sigstore/cosign/v2/cmd/cosign/cli/sign` | Sigstore signing (OIDC flow) | Large; consider shelling out to `cosign` binary instead |
| `github.com/ProtonMail/go-crypto` | GPG sign/verify without external `gpg` binary | ~2 MB |

### Build consideration: cosign binary vs library

Embedding Sigstore signing as a library adds significant binary size (~15+ MB of transitive dependencies). Two options:

**Option A: Shell out to `cosign` binary for signing, use `sigstore-go` library for verification only.**
- Signing: `cosign sign-blob --bundle sig.bundle <digest-file>` -- requires cosign installed
- Verification: Use `sigstore-go` library (lighter than full cosign)
- Pro: Smaller binary, verification works without cosign installed
- Con: Signing requires cosign to be installed separately

**Option B: Embed both signing and verification as libraries.**
- Pro: Self-contained, no external tool needed
- Con: Large binary size increase

**Recommendation: Option A.** Verification (the common path) is embedded. Signing (the rare path) delegates to cosign. This matches how most Sigstore consumers work -- `cosign` is a developer tool, `sigstore-go` is a verification library. Users who want to sign hooks install cosign once; users who just consume signed hooks need nothing extra.

---

## Open Questions

1. **Should `syllago sign` also hash-lock `content_hashes` (per Section 3.1 of security-considerations.md) at the same time?** The per-file hashes and the signature digest serve different purposes: hashes enable quick tamper detection without crypto, signatures prove identity. Doing both in `syllago sign` would be convenient but conflates two features.

2. **Should the trust policy support registry-level trust?** E.g., "trust all content from registry X" rather than per-identity. This maps to the `allow_managed_only` field in the policy interface spec but needs a concrete implementation.

3. **Should `--skip-verify` require confirmation?** Skipping verification on a hook install is a security-relevant action. A `--yes` flag or interactive confirmation could prevent accidental bypasses.
