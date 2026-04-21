package installer

// Gate primitive tests for MOAT install-flow wiring (ADR 0007 Phase 2b,
// bead syllago-8iej2).
//
// These tests exercise the pure-function gate primitives in isolation. No
// filesystem, no network, no sigstore verification — the primitives compose
// moat types that are themselves covered in the moat package's own tests,
// so here we only verify the gate's branching logic and the LockEntry
// contract.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/moat"
)

// Stable fixture hashes. Content is opaque; only equality across records
// matters for the gate's logic.
const (
	gateHashA = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	gateHashB = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	gateHashC = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	gateReg   = "https://registry.example.com/manifest.json"
	gateReg2  = "https://other.example.com/manifest.json"
)

// rekorIndex returns a *int64 of v so test fixtures can populate
// ContentEntry.RekorLogIndex inline without a scratch variable.
func rekorIndex(v int64) *int64 { return &v }

// signedEntry is a SIGNED-tier ContentEntry (rekor_log_index present, no
// signing_profile).
func signedEntry(hash string) *moat.ContentEntry {
	return &moat.ContentEntry{
		Name:          "demo",
		Type:          "skill",
		ContentHash:   hash,
		AttestedAt:    time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		RekorLogIndex: rekorIndex(1336116369),
	}
}

// dualAttestedEntry adds a per-item signing_profile so TrustTier() returns
// DualAttested.
func dualAttestedEntry(hash string) *moat.ContentEntry {
	e := signedEntry(hash)
	e.SigningProfile = &moat.SigningProfile{
		Issuer:  moat.GitHubActionsIssuer,
		Subject: "https://github.com/ex/repo/.github/workflows/publish.yml@refs/heads/main",
	}
	return e
}

// unsignedEntry omits rekor_log_index; TrustTier() returns Unsigned.
func unsignedEntry(hash string) *moat.ContentEntry {
	return &moat.ContentEntry{
		Name:        "demo-unsigned",
		Type:        "skill",
		ContentHash: hash,
		AttestedAt:  time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	}
}

// fakeBundle is a placeholder attestation bundle. For BuildLockEntry the
// bundle bytes are preserved verbatim — the gate does not parse them.
var fakeBundle = json.RawMessage(`{"kind":"fake-bundle","version":"0"}`)

// manifestWith wraps revocations into a Manifest so RevocationSet.AddFromManifest
// can ingest them.
func manifestWith(revs ...moat.Revocation) *moat.Manifest {
	return &moat.Manifest{Revocations: revs}
}

func TestMOATGateDecision_String(t *testing.T) {
	t.Parallel()
	cases := map[MOATGateDecision]string{
		MOATGateProceed:         "proceed",
		MOATGateHardBlock:       "hard-block",
		MOATGatePublisherWarn:   "publisher-warn",
		MOATGatePrivatePrompt:   "private-prompt",
		MOATGateTierBelowPolicy: "tier-below-policy",
		MOATGateDecision(99):    "unknown",
	}
	for d, want := range cases {
		if got := d.String(); got != want {
			t.Errorf("MOATGateDecision(%d).String() = %q, want %q", d, got, want)
		}
	}
}

func TestPreInstallCheck_NilEntryPanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil entry, got none")
		}
	}()
	_ = PreInstallCheck(nil, gateReg, nil, nil, nil, moat.TrustTierUnsigned)
}

func TestPreInstallCheck_HappyPath(t *testing.T) {
	t.Parallel()
	gate := PreInstallCheck(signedEntry(gateHashA), gateReg, nil, nil, nil, moat.TrustTierUnsigned)
	if gate.Decision != MOATGateProceed {
		t.Fatalf("expected Proceed, got %s", gate.Decision)
	}
	if gate.Revocation != nil {
		t.Errorf("Proceed should not populate Revocation, got %+v", gate.Revocation)
	}
}

func TestPreInstallCheck_ArchivalRevocationHardBlocks(t *testing.T) {
	t.Parallel()
	lf := moat.NewLockfile()
	lf.AddRevokedHash(gateHashA)

	// Even with empty revSet (manifest no longer lists it), the lockfile
	// archival record is authoritative — install MUST refuse.
	gate := PreInstallCheck(signedEntry(gateHashA), gateReg, lf, nil, nil, moat.TrustTierUnsigned)
	if gate.Decision != MOATGateHardBlock {
		t.Fatalf("expected HardBlock, got %s", gate.Decision)
	}
	if gate.Revocation == nil || gate.Revocation.Source != moat.RevocationSourceRegistry {
		t.Errorf("HardBlock should carry registry-source Revocation, got %+v", gate.Revocation)
	}
	if gate.Revocation.IssuingRegistryURL != gateReg {
		t.Errorf("Revocation URL = %q, want %q", gate.Revocation.IssuingRegistryURL, gateReg)
	}
}

func TestPreInstallCheck_LiveRegistryRevocationHardBlocks(t *testing.T) {
	t.Parallel()
	set := moat.NewRevocationSet()
	set.AddFromManifest(manifestWith(moat.Revocation{
		ContentHash: gateHashA,
		Reason:      moat.RevocationReasonMalicious,
		DetailsURL:  "https://example.com/cve",
		Source:      moat.RevocationSourceRegistry,
	}), gateReg)

	gate := PreInstallCheck(signedEntry(gateHashA), gateReg, moat.NewLockfile(), set, moat.NewSession(), moat.TrustTierUnsigned)
	if gate.Decision != MOATGateHardBlock {
		t.Fatalf("expected HardBlock, got %s", gate.Decision)
	}
	if gate.Revocation == nil || gate.Revocation.Status != moat.RevStatusRegistryBlock {
		t.Errorf("HardBlock should carry RegistryBlock status, got %+v", gate.Revocation)
	}
}

func TestPreInstallCheck_LivePublisherRevocationWarnsFirstTime(t *testing.T) {
	t.Parallel()
	set := moat.NewRevocationSet()
	set.AddFromManifest(manifestWith(moat.Revocation{
		ContentHash: gateHashA,
		Reason:      moat.RevocationReasonDeprecated,
		Source:      moat.RevocationSourcePublisher,
	}), gateReg)

	sess := moat.NewSession()
	gate := PreInstallCheck(signedEntry(gateHashA), gateReg, moat.NewLockfile(), set, sess, moat.TrustTierUnsigned)
	if gate.Decision != MOATGatePublisherWarn {
		t.Fatalf("expected PublisherWarn on first encounter, got %s", gate.Decision)
	}
	if gate.Revocation == nil || gate.Revocation.Status != moat.RevStatusPublisherWarn {
		t.Errorf("PublisherWarn should carry PublisherWarn status, got %+v", gate.Revocation)
	}
}

func TestPreInstallCheck_LivePublisherRevocationSuppressedAfterConfirm(t *testing.T) {
	t.Parallel()
	set := moat.NewRevocationSet()
	set.AddFromManifest(manifestWith(moat.Revocation{
		ContentHash: gateHashA,
		Reason:      moat.RevocationReasonDeprecated,
		Source:      moat.RevocationSourcePublisher,
	}), gateReg)

	sess := moat.NewSession()
	MarkPublisherConfirmed(sess, gateReg, gateHashA)

	gate := PreInstallCheck(signedEntry(gateHashA), gateReg, moat.NewLockfile(), set, sess, moat.TrustTierUnsigned)
	if gate.Decision != MOATGateProceed {
		t.Fatalf("confirmed publisher-warn should Proceed, got %s", gate.Decision)
	}
}

// Publisher confirmations are scoped to (registry, hash). A confirmation
// on registry A must not silence the same hash observed on registry B.
func TestPreInstallCheck_PublisherConfirmationIsPerRegistry(t *testing.T) {
	t.Parallel()
	set := moat.NewRevocationSet()
	set.AddFromManifest(manifestWith(moat.Revocation{
		ContentHash: gateHashA,
		Source:      moat.RevocationSourcePublisher,
	}), gateReg2)

	sess := moat.NewSession()
	MarkPublisherConfirmed(sess, gateReg, gateHashA) // confirmed on OTHER registry

	gate := PreInstallCheck(signedEntry(gateHashA), gateReg2, moat.NewLockfile(), set, sess, moat.TrustTierUnsigned)
	if gate.Decision != MOATGatePublisherWarn {
		t.Fatalf("confirmation on registry A must not suppress warn on registry B, got %s", gate.Decision)
	}
}

func TestPreInstallCheck_TierBelowPolicy(t *testing.T) {
	t.Parallel()
	gate := PreInstallCheck(unsignedEntry(gateHashA), gateReg, nil, nil, nil, moat.TrustTierSigned)
	if gate.Decision != MOATGateTierBelowPolicy {
		t.Fatalf("expected TierBelowPolicy, got %s", gate.Decision)
	}
	if gate.ObservedTier != moat.TrustTierUnsigned {
		t.Errorf("ObservedTier = %v, want Unsigned", gate.ObservedTier)
	}
	if gate.MinTier != moat.TrustTierSigned {
		t.Errorf("MinTier = %v, want Signed", gate.MinTier)
	}
}

// G-13 defensive downgrade: an entry that declares signing_profile but
// also carries attestation_hash_mismatch=true MUST compute as SIGNED, not
// DUAL-ATTESTED. When the policy floor is DUAL-ATTESTED, the gate must
// block even though the raw manifest field suggests otherwise.
func TestPreInstallCheck_G13DowngradeBlocksDualAttestedFloor(t *testing.T) {
	t.Parallel()
	entry := dualAttestedEntry(gateHashA)
	entry.AttestationHashMismatch = true

	gate := PreInstallCheck(entry, gateReg, nil, nil, nil, moat.TrustTierDualAttested)
	if gate.Decision != MOATGateTierBelowPolicy {
		t.Fatalf("G-13 mismatch at DUAL-ATTESTED floor must be TierBelowPolicy, got %s", gate.Decision)
	}
	if gate.ObservedTier != moat.TrustTierSigned {
		t.Errorf("downgrade should yield Signed, got %v", gate.ObservedTier)
	}
}

func TestPreInstallCheck_PrivatePromptFirstTime(t *testing.T) {
	t.Parallel()
	entry := signedEntry(gateHashA)
	entry.PrivateRepo = true

	gate := PreInstallCheck(entry, gateReg, nil, nil, moat.NewSession(), moat.TrustTierUnsigned)
	if gate.Decision != MOATGatePrivatePrompt {
		t.Fatalf("expected PrivatePrompt, got %s", gate.Decision)
	}
}

func TestPreInstallCheck_PrivateSuppressedAfterConfirm(t *testing.T) {
	t.Parallel()
	entry := signedEntry(gateHashA)
	entry.PrivateRepo = true

	sess := moat.NewSession()
	MarkPrivateConfirmed(sess, gateReg, gateHashA)

	gate := PreInstallCheck(entry, gateReg, nil, nil, sess, moat.TrustTierUnsigned)
	if gate.Decision != MOATGateProceed {
		t.Fatalf("confirmed private-content should Proceed, got %s", gate.Decision)
	}
}

// Private-confirmation state lives in a disjoint key namespace from
// publisher-warn confirmations — confirming one MUST NOT suppress the
// other for the same (registry, hash) pair.
func TestPreInstallCheck_PublisherAndPrivateKeyspacesDisjoint(t *testing.T) {
	t.Parallel()
	// Confirming the publisher-warn side should leave private-prompt intact.
	entry := signedEntry(gateHashA)
	entry.PrivateRepo = true
	sess := moat.NewSession()
	MarkPublisherConfirmed(sess, gateReg, gateHashA)

	gate := PreInstallCheck(entry, gateReg, nil, nil, sess, moat.TrustTierUnsigned)
	if gate.Decision != MOATGatePrivatePrompt {
		t.Fatalf("publisher-confirm must not silence private-prompt, got %s", gate.Decision)
	}

	// And the reverse: confirming private must not silence publisher-warn.
	set := moat.NewRevocationSet()
	set.AddFromManifest(manifestWith(moat.Revocation{
		ContentHash: gateHashB,
		Source:      moat.RevocationSourcePublisher,
	}), gateReg)

	sess2 := moat.NewSession()
	MarkPrivateConfirmed(sess2, gateReg, gateHashB)

	gate2 := PreInstallCheck(signedEntry(gateHashB), gateReg, nil, set, sess2, moat.TrustTierUnsigned)
	if gate2.Decision != MOATGatePublisherWarn {
		t.Fatalf("private-confirm must not silence publisher-warn, got %s", gate2.Decision)
	}
}

// Gate precedence: archival revocation fires BEFORE the tier-policy
// check, so even an UNSIGNED entry that would fail the policy floor
// first blocks as HardBlock rather than TierBelowPolicy.
func TestPreInstallCheck_ArchivalRevocationBeatsTierPolicy(t *testing.T) {
	t.Parallel()
	lf := moat.NewLockfile()
	lf.AddRevokedHash(gateHashA)

	gate := PreInstallCheck(unsignedEntry(gateHashA), gateReg, lf, nil, nil, moat.TrustTierSigned)
	if gate.Decision != MOATGateHardBlock {
		t.Fatalf("archival revocation must win over tier policy, got %s", gate.Decision)
	}
}

func TestMarkPublisherConfirmed_NilSessionIsSafe(t *testing.T) {
	t.Parallel()
	// Must not panic.
	MarkPublisherConfirmed(nil, gateReg, gateHashA)
	MarkPrivateConfirmed(nil, gateReg, gateHashA)
}

func TestPrivateSessionKey_Namespacing(t *testing.T) {
	t.Parallel()
	got := privateSessionKey(gateReg)
	want := "private:" + gateReg
	if got != want {
		t.Errorf("privateSessionKey(%q) = %q, want %q", gateReg, got, want)
	}
}

func TestBuildLockEntry_NilEntry(t *testing.T) {
	t.Parallel()
	_, _, err := BuildLockEntry(nil, gateReg, fakeBundle, time.Now())
	if !errors.Is(err, ErrEntryNil) {
		t.Fatalf("expected ErrEntryNil, got %v", err)
	}
}

func TestBuildLockEntry_UnsignedHasNullPayload(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	le, expected, err := BuildLockEntry(unsignedEntry(gateHashA), gateReg, nil, now)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if expected != "" {
		t.Errorf("UNSIGNED expected=%q, want empty", expected)
	}
	if le.TrustTier != moat.LockTrustTierUnsigned {
		t.Errorf("TrustTier = %q, want %s", le.TrustTier, moat.LockTrustTierUnsigned)
	}
	if le.SignedPayload != nil {
		t.Errorf("UNSIGNED must have nil SignedPayload, got %v", *le.SignedPayload)
	}
	// Bundle must be the JSON null literal (4 bytes).
	if string(le.AttestationBundle) != "null" {
		t.Errorf("UNSIGNED bundle = %s, want null", string(le.AttestationBundle))
	}
	if !le.PinnedAt.Equal(now) {
		t.Errorf("PinnedAt = %v, want %v", le.PinnedAt, now)
	}
}

func TestBuildLockEntry_SignedProducesCanonicalPayload(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	le, expected, err := BuildLockEntry(signedEntry(gateHashA), gateReg, fakeBundle, now)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if le.TrustTier != moat.LockTrustTierSigned {
		t.Errorf("TrustTier = %q, want %s", le.TrustTier, moat.LockTrustTierSigned)
	}
	if le.SignedPayload == nil {
		t.Fatal("SIGNED must have non-nil SignedPayload")
	}
	wantPayload := string(moat.CanonicalPayloadFor(gateHashA))
	if *le.SignedPayload != wantPayload {
		t.Errorf("SignedPayload = %q, want %q", *le.SignedPayload, wantPayload)
	}
	// expectedDataHashValue must equal sha256 of the payload hex-encoded.
	digest := sha256.Sum256([]byte(wantPayload))
	wantHex := hex.EncodeToString(digest[:])
	if expected != wantHex {
		t.Errorf("expectedDataHashValue = %q, want %q", expected, wantHex)
	}
	if string(le.AttestationBundle) != string(fakeBundle) {
		t.Errorf("AttestationBundle round-trip corrupted: got %s, want %s",
			string(le.AttestationBundle), string(fakeBundle))
	}
}

func TestBuildLockEntry_DualAttestedLabel(t *testing.T) {
	t.Parallel()
	le, _, err := BuildLockEntry(dualAttestedEntry(gateHashA), gateReg, fakeBundle, time.Now())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if le.TrustTier != moat.LockTrustTierDualAttested {
		t.Errorf("TrustTier = %q, want %s", le.TrustTier, moat.LockTrustTierDualAttested)
	}
}

// G-13 downgrade must land in the LockEntry.TrustTier wire label: a
// DUAL-ATTESTED manifest with attestation_hash_mismatch=true writes as
// SIGNED, not DUAL-ATTESTED.
func TestBuildLockEntry_G13DowngradeWritesSignedLabel(t *testing.T) {
	t.Parallel()
	entry := dualAttestedEntry(gateHashA)
	entry.AttestationHashMismatch = true

	le, _, err := BuildLockEntry(entry, gateReg, fakeBundle, time.Now())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if le.TrustTier != moat.LockTrustTierSigned {
		t.Errorf("G-13 mismatch must label as SIGNED, got %s", le.TrustTier)
	}
}

func TestBuildLockEntry_SignedRequiresBundle(t *testing.T) {
	t.Parallel()
	_, _, err := BuildLockEntry(signedEntry(gateHashA), gateReg, nil, time.Now())
	if err == nil {
		t.Fatal("expected error on SIGNED without bundle, got nil")
	}
}

func TestRecordInstall_NilLockfile(t *testing.T) {
	t.Parallel()
	_, err := RecordInstall(nil, signedEntry(gateHashA), gateReg, fakeBundle, time.Now())
	if err == nil {
		t.Fatal("expected error on nil lockfile, got nil")
	}
}

func TestRecordInstall_HappyPathAppends(t *testing.T) {
	t.Parallel()
	lf := moat.NewLockfile()
	now := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)

	le, err := RecordInstall(lf, signedEntry(gateHashA), gateReg, fakeBundle, now)
	if err != nil {
		t.Fatalf("RecordInstall err: %v", err)
	}
	if len(lf.Entries) != 1 {
		t.Fatalf("lf.Entries len = %d, want 1", len(lf.Entries))
	}
	if lf.Entries[0].ContentHash != gateHashA {
		t.Errorf("ContentHash = %q, want %q", lf.Entries[0].ContentHash, gateHashA)
	}
	if le.ContentHash != gateHashA {
		t.Errorf("returned entry has wrong hash: %q", le.ContentHash)
	}
}

func TestRecordInstall_PropagatesBuildError(t *testing.T) {
	t.Parallel()
	lf := moat.NewLockfile()
	_, err := RecordInstall(lf, nil, gateReg, fakeBundle, time.Now())
	if !errors.Is(err, ErrEntryNil) {
		t.Fatalf("expected ErrEntryNil propagated from BuildLockEntry, got %v", err)
	}
	if len(lf.Entries) != 0 {
		t.Errorf("lockfile must not grow on BuildLockEntry error, got %d entries", len(lf.Entries))
	}
}

// RecordInstall must propagate errors from lf.AddEntry, not just from
// BuildLockEntry. Trigger this by supplying an entry with an empty Name —
// BuildLockEntry doesn't validate required fields, but AddEntry does.
func TestRecordInstall_PropagatesAddEntryError(t *testing.T) {
	t.Parallel()
	lf := moat.NewLockfile()
	entry := signedEntry(gateHashA)
	entry.Name = "" // AddEntry rejects empty name

	_, err := RecordInstall(lf, entry, gateReg, fakeBundle, time.Now())
	if err == nil {
		t.Fatal("expected error from AddEntry validation, got nil")
	}
	if len(lf.Entries) != 0 {
		t.Errorf("lockfile must not grow on AddEntry error, got %d entries", len(lf.Entries))
	}
}

// A deliberately-broken caller that constructs a mismatching LockEntry by
// hand (signed_payload doesn't match the expectedDataHashValue derived
// from a DIFFERENT content hash) must fail lf.AddEntry's sha256 check.
// RecordInstall, which uses BuildLockEntry's self-consistent expected,
// cannot produce this naturally — so we call lf.AddEntry directly with
// manufactured arguments to prove the invariant wiring is real.
func TestLockfile_AddEntry_RejectsHashMismatch(t *testing.T) {
	t.Parallel()
	lf := moat.NewLockfile()

	// Build a legitimate SIGNED entry whose signed_payload covers hashA.
	le, _, err := BuildLockEntry(signedEntry(gateHashA), gateReg, fakeBundle, time.Now())
	if err != nil {
		t.Fatalf("setup BuildLockEntry: %v", err)
	}

	// Lie to AddEntry about the expected Rekor data.hash.value — hand it
	// the hash of hashB's canonical payload instead. AddEntry must refuse.
	wrongDigest := sha256.Sum256(moat.CanonicalPayloadFor(gateHashB))
	wrongHex := hex.EncodeToString(wrongDigest[:])

	err = lf.AddEntry(le, wrongHex)
	if !errors.Is(err, moat.ErrSignedPayloadHashMismatch) {
		t.Fatalf("expected ErrSignedPayloadHashMismatch, got %v", err)
	}
	if len(lf.Entries) != 0 {
		t.Errorf("lockfile must not grow on mismatch, got %d entries", len(lf.Entries))
	}
}
