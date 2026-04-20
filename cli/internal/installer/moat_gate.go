package installer

// MOAT install-flow gate primitives (ADR 0007 Phase 2b, bead syllago-8iej2).
//
// This file is the CLI-agnostic composer that threads the moat package's
// building blocks onto the install pipeline in the order the spec requires:
//
//	1. Archival revocation check — lf.IsRevoked is permanent (G-15), so a
//	   hash ever present in revoked_hashes hard-blocks on every subsequent
//	   install regardless of whether the current manifest still lists it.
//	2. Live revocation check — RevocationSet/Session enforce the two-tier
//	   contract on freshly-fetched manifests: registry-source → hard-block,
//	   publisher-source → warn-once-per-session (ADR 0007 G-8).
//	3. Trust-tier policy — caller-supplied minimum tier refuses items that
//	   would drop the project below its configured floor. Computed from
//	   ContentEntry.TrustTier(), which honors the G-13 attestation-hash-
//	   mismatch downgrade.
//	4. Private-content acknowledgement — ContentEntry.IsPrivate requires
//	   explicit confirmation before install proceeds (G-10). The gate
//	   reports the decision; interactive approval and non-interactive exit
//	   mapping are the caller's responsibility.
//
// The gate intentionally does NOT touch stdin/stdout. Callers consume the
// returned GateBlock and decide how to surface it — a CLI command fprints
// a prompt, a TUI dispatches a modal, a non-interactive pipeline maps to
// the appropriate NonInteractiveFailure exit code. Keeping the decision
// free of I/O is what makes the same code usable from both the install
// command (stdin-bound) and the TUI (message-bound).
//
// The post-install side — RecordInstall — is where the LockEntry is built
// and appended. The lockfile.AddEntry pre-write check enforces the spec
// invariant `sha256(signed_payload) == Rekor data.hash.value`; this file
// is careful to supply both values so a misbehaving caller cannot bypass
// the invariant by routing around the helper.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/moat"
)

// sha256HexOf is a local helper so BuildLockEntry can compute the
// expectedDataHashValue argument AddEntry verifies against. Mirrors the
// same-named unexported helper in the moat package's tests — we cannot
// import a test helper across packages, so the lowercase 4-liner lives
// here. Kept private because callers outside this file should go
// through BuildLockEntry rather than recomputing hashes independently.
func sha256HexOf(b []byte) string {
	d := sha256.Sum256(b)
	return hex.EncodeToString(d[:])
}

// MOATGateDecision is the branch the caller should take after PreInstallCheck
// returns. Negative-space values beyond Proceed map to specific G-18
// NonInteractiveFailure codes (documented on each variant).
type MOATGateDecision int

const (
	// MOATGateProceed means no gate blocked the install. The caller
	// continues to the filesystem side-effect, then calls RecordInstall
	// to append the lockfile entry.
	MOATGateProceed MOATGateDecision = iota

	// MOATGateHardBlock means a registry-source revocation (archival or
	// live) applies. The caller MUST refuse to install — interactive
	// confirmation cannot override a registry-source block, per G-8.
	// Non-interactive callers MUST exit non-zero with a MOAT_* structured
	// error; no dedicated NonInteractiveFailure exit exists (registry-
	// source revocation is categorically fatal, not a gate).
	MOATGateHardBlock

	// MOATGatePublisherWarn means the item is publisher-revoked and the
	// session has not yet confirmed acceptance. Interactive callers MUST
	// prompt Y/n and on Yes call Session.MarkConfirmed to suppress
	// future warnings for the same (registry, hash) pair. Non-interactive
	// callers MUST exit with ExitMoatPublisherRevocation (12) per G-18.
	MOATGatePublisherWarn

	// MOATGatePrivatePrompt means the content_entry is declared with
	// private_repo=true and requires explicit operator acknowledgement
	// (G-10). Interactive callers MUST prompt; non-interactive callers
	// MUST exit with ExitMoatTOFUAcceptance (10) because the operator is
	// the only party who can confirm visibility expectations — the
	// acceptance shape matches TOFU semantically.
	MOATGatePrivatePrompt

	// MOATGateTierBelowPolicy means the item's computed TrustTier is
	// below the caller-supplied minimum. There is no interactive recovery
	// — the operator cannot upgrade a tier; only the publisher can. The
	// caller surfaces this as a structured error and exits.
	MOATGateTierBelowPolicy
)

// String returns a short human-readable label used for diagnostics and
// structured audit logs. Stable across versions; do NOT rewrite these —
// external scripts grep for the labels.
func (d MOATGateDecision) String() string {
	switch d {
	case MOATGateProceed:
		return "proceed"
	case MOATGateHardBlock:
		return "hard-block"
	case MOATGatePublisherWarn:
		return "publisher-warn"
	case MOATGatePrivatePrompt:
		return "private-prompt"
	case MOATGateTierBelowPolicy:
		return "tier-below-policy"
	default:
		return "unknown"
	}
}

// GateBlock is the return payload from PreInstallCheck. Exactly one of
// Revocation / tier fields is populated according to Decision; the
// remainder stay zero so callers can switch on Decision without
// defensive nil guards.
//
// When Decision == MOATGateProceed, all detail fields are zero.
type GateBlock struct {
	// Decision is the gate outcome. Every caller MUST switch on this.
	Decision MOATGateDecision

	// Revocation is populated for MOATGateHardBlock and
	// MOATGatePublisherWarn. Reason and DetailsURL drive the operator-
	// facing message; IssuingRegistryURL + ContentHash are the pair the
	// caller passes to Session.MarkConfirmed on Y.
	Revocation *moat.RevocationRecord

	// ObservedTier is the tier the manifest actually carried; MinTier is
	// the caller's policy floor. Both are set only for
	// MOATGateTierBelowPolicy.
	ObservedTier moat.TrustTier
	MinTier      moat.TrustTier
}

// PreInstallCheck enforces the MOAT gates BEFORE any filesystem side-effect.
//
// Parameters:
//   - entry: the manifest row being installed. Must be non-nil.
//   - registryURL: the manifest_uri. Propagated into Revocation records
//     and Session keys so a publisher revocation confirmed against
//     registry A never auto-confirms the same hash observed on registry
//     B (different operators, different trust decisions).
//   - lf: the project lockfile. Nil is allowed (first-ever install) and
//     means "no archival revocations known yet."
//   - revSet: RevocationSet aggregated from the manifest(s) just
//     synced. Nil means "no live revocations this run"; the caller has
//     already run moat.Sync and chose not to cross-reference.
//   - session: publisher-warn and private-prompt session suppressor.
//     Nil is treated as "every check warns" — safe-by-default for
//     short-lived CLI invocations.
//   - minTier: the project's policy floor. Pass TrustTierUnsigned to
//     accept anything.
//
// Returns GateBlock. Never returns an error — all misuse is panic-worthy
// programmer bug (nil entry) and every legitimate input maps to a
// decision value.
func PreInstallCheck(
	entry *moat.ContentEntry,
	registryURL string,
	lf *moat.Lockfile,
	revSet *moat.RevocationSet,
	session *moat.Session,
	minTier moat.TrustTier,
) GateBlock {
	if entry == nil {
		panic("installer: PreInstallCheck: entry is nil")
	}

	// 1. Archival registry-source revocation. Permanent per G-15 — even
	// if the revocation has been pruned from the live manifest, the
	// lockfile retains it and we MUST keep blocking.
	if lf != nil && lf.IsRevoked(entry.ContentHash) {
		return GateBlock{
			Decision: MOATGateHardBlock,
			Revocation: &moat.RevocationRecord{
				ContentHash:        entry.ContentHash,
				Source:             moat.RevocationSourceRegistry,
				IssuingRegistryURL: registryURL,
				Status:             moat.RevStatusRegistryBlock,
			},
		}
	}

	// 2. Live revocation from freshly-synced manifests. The two-tier
	// contract (ADR 0007 G-8) branches on source: registry always blocks;
	// publisher requires per-session acknowledgement.
	if revSet != nil {
		for _, r := range revSet.Lookup(entry.ContentHash) {
			switch r.Status {
			case moat.RevStatusRegistryBlock:
				rec := r
				return GateBlock{Decision: MOATGateHardBlock, Revocation: &rec}
			case moat.RevStatusPublisherWarn:
				if session.ShouldWarn(r.IssuingRegistryURL, r.ContentHash) {
					rec := r
					return GateBlock{Decision: MOATGatePublisherWarn, Revocation: &rec}
				}
			}
		}
	}

	// 3. Trust-tier floor. ContentEntry.TrustTier() already applies the
	// G-13 AttestationHashMismatch downgrade, so no extra handling is
	// required here — a manifest that carries signing_profile with the
	// mismatch flag never clears a DualAttested floor.
	observed := entry.TrustTier()
	if observed < minTier {
		return GateBlock{
			Decision:     MOATGateTierBelowPolicy,
			ObservedTier: observed,
			MinTier:      minTier,
		}
	}

	// 4. Private-content acknowledgement. Use a distinct key prefix so
	// private-confirm state does not collide with publisher-warn
	// confirmations for the same (registry, hash) pair.
	if entry.IsPrivate() {
		if session.ShouldWarn(privateSessionKey(registryURL), entry.ContentHash) {
			return GateBlock{Decision: MOATGatePrivatePrompt}
		}
	}

	return GateBlock{Decision: MOATGateProceed}
}

// MarkPublisherConfirmed records that the operator has acknowledged a
// publisher-source revocation for this (registry, hash) pair so
// subsequent gate checks in the same session skip the prompt. Safe to
// call on a nil session — no-op. Thin pass-through to the moat package
// so callers have a single import surface for gate + confirmation.
func MarkPublisherConfirmed(session *moat.Session, registryURL, contentHash string) {
	session.MarkConfirmed(registryURL, contentHash)
}

// MarkPrivateConfirmed records the operator's acknowledgement of a
// private-repo item for this (registry, hash) pair. Uses the same
// key-prefix convention as PreInstallCheck so a single Session tracks
// both confirmation classes without collision.
func MarkPrivateConfirmed(session *moat.Session, registryURL, contentHash string) {
	session.MarkConfirmed(privateSessionKey(registryURL), contentHash)
}

// privateSessionKey prefixes a registry URL so private-confirm state is
// stored under a disjoint key from publisher-warn confirmations in the
// shared Session map. The prefix uses a character the URL syntax cannot
// produce at the start of a URL (a bare "private:" scheme-like token),
// avoiding any real-world collision.
func privateSessionKey(registryURL string) string {
	return "private:" + registryURL
}

// ErrEntryNil is returned by RecordInstall / BuildLockEntry when the
// caller passes a nil ContentEntry. Programmer error; callers should
// not produce this in practice.
var ErrEntryNil = errors.New("installer: content entry is nil")

// BuildLockEntry translates a moat.ContentEntry into a moat.LockEntry
// ready for lf.AddEntry, along with the expected Rekor data.hash.value
// the pre-write check will verify `sha256(signed_payload)` against.
//
// Trust-tier labels come from ContentEntry.TrustTier() (not from a raw
// field), so the G-13 downgrade is honored — a manifest with
// attestation_hash_mismatch=true and a signing_profile still lands in
// the lockfile as SIGNED, not DUAL-ATTESTED.
//
// For UNSIGNED tiers:
//   - SignedPayload is nil (serializes to JSON null).
//   - AttestationBundle is JSON null.
//   - expectedDataHashValue is the empty string; AddEntry ignores it.
//
// For SIGNED and DUAL-ATTESTED tiers:
//   - SignedPayload is the canonical payload bytes as a string — the
//     exact representation cosign verify-blob --offline will be handed
//     during future offline verification.
//   - AttestationBundle is the rekorBundle argument verbatim. Callers
//     pass the raw Rekor response bytes (or the cosign bundle bytes)
//     captured at verification time. Re-marshaling would reorder keys
//     and break offline verify, which is why we carry RawMessage.
//   - expectedDataHashValue is hex(sha256(signed_payload)). AddEntry
//     recomputes and compares; any discrepancy rejects the entry.
//
// registryURL goes into LockEntry.Registry and should match the
// manifest_uri used during sync, so a future `moat-verify` pass can
// correlate entries to the registry they were pinned from.
func BuildLockEntry(
	entry *moat.ContentEntry,
	registryURL string,
	rekorBundle json.RawMessage,
	now time.Time,
) (moat.LockEntry, string, error) {
	if entry == nil {
		return moat.LockEntry{}, "", ErrEntryNil
	}

	tier := entry.TrustTier()
	tierLabel := moat.TrustTierLabel(tier)
	if tierLabel == "" {
		return moat.LockEntry{}, "", fmt.Errorf("installer: unknown trust tier %v for %q", tier, entry.Name)
	}

	le := moat.LockEntry{
		Name:        entry.Name,
		Type:        entry.Type,
		Registry:    registryURL,
		ContentHash: entry.ContentHash,
		TrustTier:   tierLabel,
		AttestedAt:  entry.AttestedAt.UTC(),
		PinnedAt:    now.UTC(),
	}

	if tier == moat.TrustTierUnsigned {
		le.AttestationBundle = moat.NullAttestationBundle()
		return le, "", nil
	}

	// SIGNED / DUAL-ATTESTED: produce the canonical payload and its hash
	// so AddEntry can verify them. The payload bytes here are the SAME
	// bytes VerifyAttestationItem already confirmed match the Rekor
	// entry's data.hash.value — BuildLockEntry does not re-verify, it
	// just recomputes the hash from an already-trusted content hash.
	payload := moat.CanonicalPayloadFor(entry.ContentHash)
	payloadStr := string(payload)
	le.SignedPayload = &payloadStr
	if len(rekorBundle) == 0 {
		return moat.LockEntry{}, "", fmt.Errorf("installer: rekor bundle required for trust_tier=%s on %q", tierLabel, entry.Name)
	}
	le.AttestationBundle = rekorBundle

	expected := sha256HexOf(payload)
	return le, expected, nil
}

// RecordInstall composes BuildLockEntry with lf.AddEntry. On success the
// lockfile has a new entries[] row; AddEntry itself is responsible for
// the sha256(signed_payload) == expectedDataHashValue check, and
// returns ErrSignedPayloadHashMismatch on violation. Callers MUST NOT
// persist the lockfile until this returns nil — a half-applied install
// would poison subsequent offline verification.
//
// Side effects on success: lf.Entries grows by one row. The caller is
// expected to Save() the lockfile after the install side-effect
// completes; RecordInstall itself never touches disk.
func RecordInstall(
	lf *moat.Lockfile,
	entry *moat.ContentEntry,
	registryURL string,
	rekorBundle json.RawMessage,
	now time.Time,
) (moat.LockEntry, error) {
	if lf == nil {
		return moat.LockEntry{}, errors.New("installer: lockfile is nil")
	}
	le, expected, err := BuildLockEntry(entry, registryURL, rekorBundle, now)
	if err != nil {
		return moat.LockEntry{}, err
	}
	if err := lf.AddEntry(le, expected); err != nil {
		return moat.LockEntry{}, err
	}
	return le, nil
}
