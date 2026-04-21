package moat

// Tests for enrich.go (ADR 0007 Phase 2, bead syllago-kvf66).
//
// Coverage bar is ≥90% on the three exported surfaces plus the private
// tier mapper. moatTierToCatalogTier is exercised through EnrichCatalog —
// the function is a switch with four arms and EnrichCatalog hits every
// one via the TrustTier fixtures below.

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// --- fixtures ----------------------------------------------------------

// pad64 expands a short hex prefix to a valid 64-char sha256 body.
// ParseContentHash rejects short digests, so the manifest fixtures below
// would fail validate() if we used raw "aa..." strings.
func pad64(prefix string) string {
	out := make([]byte, 64)
	for i := range out {
		out[i] = '0'
	}
	copy(out, prefix)
	return string(out)
}

func hashA() string { return "sha256:" + pad64("aa") }
func hashB() string { return "sha256:" + pad64("bb") }
func hashC() string { return "sha256:" + pad64("cc") }

// unsignedEntry has no rekor index — TrustTier() returns TrustTierUnsigned.
func unsignedEntry(name, hash string) ContentEntry {
	return ContentEntry{Name: name, ContentHash: hash}
}

// signedEntry has a rekor index but no per-item signing_profile — TrustTier() returns TrustTierSigned.
func signedEntry(name, hash string) ContentEntry {
	idx := int64(12345)
	return ContentEntry{Name: name, ContentHash: hash, RekorLogIndex: &idx}
}

// dualAttestedEntry has both rekor_log_index and signing_profile — TrustTier() returns TrustTierDualAttested.
func dualAttestedEntry(name, hash string) ContentEntry {
	idx := int64(67890)
	return ContentEntry{
		Name:           name,
		ContentHash:    hash,
		RekorLogIndex:  &idx,
		SigningProfile: &SigningProfile{Issuer: "https://token.example.com", Subject: "pub@example.com"},
	}
}

// g13MismatchEntry carries the dual-attested shape *plus* the mismatch
// flag — TrustTier() returns Signed per G-13 defensive downgrade.
func g13MismatchEntry(name, hash string) ContentEntry {
	e := dualAttestedEntry(name, hash)
	e.AttestationHashMismatch = true
	return e
}

// --- FindContentEntry --------------------------------------------------

func TestFindContentEntry_NilManifest(t *testing.T) {
	t.Parallel()
	got, ok := FindContentEntry(nil, "anything")
	if ok || got != nil {
		t.Fatalf("FindContentEntry(nil, _) = (%v, %v), want (nil, false)", got, ok)
	}
}

func TestFindContentEntry_EmptyContent(t *testing.T) {
	t.Parallel()
	m := &Manifest{Content: []ContentEntry{}}
	got, ok := FindContentEntry(m, "missing")
	if ok || got != nil {
		t.Fatalf("FindContentEntry empty = (%v, %v), want (nil, false)", got, ok)
	}
}

func TestFindContentEntry_Hit(t *testing.T) {
	t.Parallel()
	m := &Manifest{Content: []ContentEntry{
		unsignedEntry("a", hashA()),
		signedEntry("b", hashB()),
		dualAttestedEntry("c", hashC()),
	}}
	got, ok := FindContentEntry(m, "b")
	if !ok {
		t.Fatalf("FindContentEntry(b) missed")
	}
	if got.ContentHash != hashB() {
		t.Errorf("content hash = %q, want %q", got.ContentHash, hashB())
	}
	// Pointer must alias the manifest slot so callers can read up-to-date
	// fields without a copy.
	if got != &m.Content[1] {
		t.Errorf("returned pointer does not alias m.Content[1]")
	}
}

func TestFindContentEntry_Miss(t *testing.T) {
	t.Parallel()
	m := &Manifest{Content: []ContentEntry{unsignedEntry("a", hashA())}}
	got, ok := FindContentEntry(m, "nope")
	if ok || got != nil {
		t.Fatalf("FindContentEntry(nope) = (%v, %v), want (nil, false)", got, ok)
	}
}

// --- EnrichCatalog: nil guards ----------------------------------------

func TestEnrichCatalog_NilCatalog(t *testing.T) {
	t.Parallel()
	m := &Manifest{Content: []ContentEntry{unsignedEntry("a", hashA())}}
	// Must not panic.
	EnrichCatalog(nil, "reg", m)
}

func TestEnrichCatalog_NilManifest(t *testing.T) {
	t.Parallel()
	cat := &catalog.Catalog{Items: []catalog.ContentItem{{Name: "a", Registry: "reg"}}}
	EnrichCatalog(cat, "reg", nil)
	// Items must be unchanged.
	if cat.Items[0].TrustTier != catalog.TrustTierUnknown {
		t.Errorf("TrustTier mutated despite nil manifest: got %v", cat.Items[0].TrustTier)
	}
	if cat.Items[0].Recalled {
		t.Errorf("Recalled set despite nil manifest")
	}
}

// --- EnrichCatalog: tier mapping (covers moatTierToCatalogTier) -------

func TestEnrichCatalog_TierMapping(t *testing.T) {
	t.Parallel()
	m := &Manifest{
		Content: []ContentEntry{
			unsignedEntry("u", hashA()),
			signedEntry("s", hashB()),
			dualAttestedEntry("d", hashC()),
		},
	}
	cat := &catalog.Catalog{Items: []catalog.ContentItem{
		{Name: "u", Registry: "reg"},
		{Name: "s", Registry: "reg"},
		{Name: "d", Registry: "reg"},
	}}

	EnrichCatalog(cat, "reg", m)

	cases := []struct {
		name string
		want catalog.TrustTier
	}{
		{"u", catalog.TrustTierUnsigned},
		{"s", catalog.TrustTierSigned},
		{"d", catalog.TrustTierDualAttested},
	}
	for i, c := range cases {
		if got := cat.Items[i].TrustTier; got != c.want {
			t.Errorf("items[%d] (%s) tier = %v, want %v", i, c.name, got, c.want)
		}
		if cat.Items[i].Recalled {
			t.Errorf("items[%d] (%s) unexpectedly Recalled", i, c.name)
		}
	}
}

// --- EnrichCatalog: G-13 attestation_hash_mismatch downgrade ----------

func TestEnrichCatalog_G13MismatchDowngradesToSigned(t *testing.T) {
	t.Parallel()
	m := &Manifest{Content: []ContentEntry{g13MismatchEntry("d", hashA())}}
	cat := &catalog.Catalog{Items: []catalog.ContentItem{{Name: "d", Registry: "reg"}}}

	EnrichCatalog(cat, "reg", m)

	// Even though signing_profile is populated, the mismatch flag forces
	// the moat-side TrustTier() to return Signed — and the catalog mirror
	// must match it rather than elevate to DualAttested.
	if got := cat.Items[0].TrustTier; got != catalog.TrustTierSigned {
		t.Errorf("G-13 downgrade: catalog tier = %v, want Signed", got)
	}
}

// --- EnrichCatalog: revocation population -----------------------------

func TestEnrichCatalog_RevocationSetsRecalled(t *testing.T) {
	t.Parallel()
	m := &Manifest{
		Content: []ContentEntry{signedEntry("bad", hashA())},
		Revocations: []Revocation{{
			ContentHash: hashA(),
			Reason:      RevocationReasonMalicious,
			DetailsURL:  "https://example.com/revs/1",
		}},
	}
	cat := &catalog.Catalog{Items: []catalog.ContentItem{{Name: "bad", Registry: "reg"}}}

	EnrichCatalog(cat, "reg", m)

	if !cat.Items[0].Recalled {
		t.Fatalf("Recalled not set for revoked item")
	}
	if cat.Items[0].RecallReason != RevocationReasonMalicious {
		t.Errorf("RecallReason = %q, want %q", cat.Items[0].RecallReason, RevocationReasonMalicious)
	}
	// Tier should still be populated alongside Recalled — the display
	// collapses via UserFacingBadge, not by zeroing the tier.
	if cat.Items[0].TrustTier != catalog.TrustTierSigned {
		t.Errorf("TrustTier lost when Recalled set: got %v", cat.Items[0].TrustTier)
	}
}

func TestEnrichCatalog_NoRevocationLeavesFieldsZero(t *testing.T) {
	t.Parallel()
	m := &Manifest{
		Content: []ContentEntry{signedEntry("ok", hashA())},
		Revocations: []Revocation{{
			ContentHash: hashB(), // different hash — should not affect item
			Reason:      RevocationReasonDeprecated,
			DetailsURL:  "https://example.com/revs/2",
		}},
	}
	cat := &catalog.Catalog{Items: []catalog.ContentItem{{Name: "ok", Registry: "reg"}}}

	EnrichCatalog(cat, "reg", m)

	if cat.Items[0].Recalled {
		t.Errorf("Recalled set when hash does not match any revocation")
	}
	if cat.Items[0].RecallReason != "" {
		t.Errorf("RecallReason = %q, want empty", cat.Items[0].RecallReason)
	}
}

// --- EnrichCatalog: registry filtering --------------------------------

func TestEnrichCatalog_OtherRegistryUntouched(t *testing.T) {
	t.Parallel()
	m := &Manifest{Content: []ContentEntry{signedEntry("a", hashA())}}
	cat := &catalog.Catalog{Items: []catalog.ContentItem{
		{Name: "a", Registry: "reg"},
		{Name: "a", Registry: "other"}, // same name, different registry
		{Name: "a", Registry: ""},      // library/shared item — empty Registry
	}}

	EnrichCatalog(cat, "reg", m)

	if cat.Items[0].TrustTier != catalog.TrustTierSigned {
		t.Errorf("items[0] tier = %v, want Signed", cat.Items[0].TrustTier)
	}
	if cat.Items[1].TrustTier != catalog.TrustTierUnknown {
		t.Errorf("items[1] (other registry) tier mutated: got %v", cat.Items[1].TrustTier)
	}
	if cat.Items[2].TrustTier != catalog.TrustTierUnknown {
		t.Errorf("items[2] (empty registry) tier mutated: got %v", cat.Items[2].TrustTier)
	}
}

func TestEnrichCatalog_MissingEntryLeavesItemUntouched(t *testing.T) {
	t.Parallel()
	m := &Manifest{Content: []ContentEntry{signedEntry("known", hashA())}}
	cat := &catalog.Catalog{Items: []catalog.ContentItem{
		{Name: "known", Registry: "reg"},
		{Name: "ghost", Registry: "reg"}, // not in manifest
	}}

	EnrichCatalog(cat, "reg", m)

	if cat.Items[0].TrustTier != catalog.TrustTierSigned {
		t.Errorf("known item tier = %v, want Signed", cat.Items[0].TrustTier)
	}
	if cat.Items[1].TrustTier != catalog.TrustTierUnknown {
		t.Errorf("ghost item tier mutated: got %v", cat.Items[1].TrustTier)
	}
}

// --- EnrichCatalog: duplicate revocations use first match -------------

func TestEnrichCatalog_DuplicateRevocationKeepsFirst(t *testing.T) {
	t.Parallel()
	m := &Manifest{
		Content: []ContentEntry{signedEntry("a", hashA())},
		Revocations: []Revocation{
			{ContentHash: hashA(), Reason: RevocationReasonMalicious, DetailsURL: "u1"},
			{ContentHash: hashA(), Reason: RevocationReasonDeprecated, DetailsURL: "u2"},
		},
	}
	cat := &catalog.Catalog{Items: []catalog.ContentItem{{Name: "a", Registry: "reg"}}}

	EnrichCatalog(cat, "reg", m)

	if cat.Items[0].RecallReason != RevocationReasonMalicious {
		t.Errorf("RecallReason = %q, want first-match %q",
			cat.Items[0].RecallReason, RevocationReasonMalicious)
	}
}

// --- moatTierToCatalogTier: direct call covers the defensive default --

func TestMoatTierToCatalogTier_UnknownFallsThroughToUnknown(t *testing.T) {
	t.Parallel()
	// Cast an out-of-range int into the TrustTier enum to simulate a
	// future tier value we do not yet map. The contract documented in
	// enrich.go is to return catalog.TrustTierUnknown rather than a
	// best-guess — exercise that branch explicitly.
	got := moatTierToCatalogTier(TrustTier(99))
	if got != catalog.TrustTierUnknown {
		t.Errorf("unknown TrustTier mapped to %v, want TrustTierUnknown", got)
	}
}

// --- EnrichCatalog: multiple items, mixed state ------------------------

func TestEnrichCatalog_MixedCatalog(t *testing.T) {
	t.Parallel()
	m := &Manifest{
		Content: []ContentEntry{
			dualAttestedEntry("alpha", hashA()),
			signedEntry("beta", hashB()),
			unsignedEntry("gamma", hashC()),
		},
		Revocations: []Revocation{{
			ContentHash: hashB(),
			Reason:      RevocationReasonCompromised,
			DetailsURL:  "https://example.com/revs/x",
		}},
	}
	cat := &catalog.Catalog{Items: []catalog.ContentItem{
		{Name: "alpha", Registry: "reg"},
		{Name: "beta", Registry: "reg"},
		{Name: "gamma", Registry: "reg"},
		{Name: "beta", Registry: "other"}, // different registry; same hash would be revoked there too IF we ran EnrichCatalog("other"), but we don't
	}}

	EnrichCatalog(cat, "reg", m)

	if cat.Items[0].TrustTier != catalog.TrustTierDualAttested || cat.Items[0].Recalled {
		t.Errorf("alpha: tier=%v recalled=%v, want DualAttested/false",
			cat.Items[0].TrustTier, cat.Items[0].Recalled)
	}
	if cat.Items[1].TrustTier != catalog.TrustTierSigned || !cat.Items[1].Recalled {
		t.Errorf("beta: tier=%v recalled=%v, want Signed/true",
			cat.Items[1].TrustTier, cat.Items[1].Recalled)
	}
	if cat.Items[1].RecallReason != RevocationReasonCompromised {
		t.Errorf("beta RecallReason = %q, want %q",
			cat.Items[1].RecallReason, RevocationReasonCompromised)
	}
	if cat.Items[2].TrustTier != catalog.TrustTierUnsigned {
		t.Errorf("gamma tier = %v, want Unsigned", cat.Items[2].TrustTier)
	}
	if cat.Items[3].TrustTier != catalog.TrustTierUnknown {
		t.Errorf("other-registry beta mutated: tier=%v", cat.Items[3].TrustTier)
	}
}

// --- Phase 2c: new drill-down fields ----------------------------------
//
// PrivateRepo, RecallSource, RecallDetailsURL, RecallIssuer were added in
// MOAT Phase 2c (bead syllago-lqas0). Each test below pins down one field
// end-to-end: how it is populated, how sanitization runs, and what the
// zero-value semantics look like on a non-Recalled or non-MOAT item.

// TestEnrichCatalog_PrivateRepoPopulated confirms the G-10 per-item private
// declaration propagates even when no revocation is present. The registry-
// level visibility probe is irrelevant here — ContentEntry.PrivateRepo is
// a publisher declaration and wins.
func TestEnrichCatalog_PrivateRepoPopulated(t *testing.T) {
	t.Parallel()
	privEntry := signedEntry("private-skill", hashA())
	privEntry.PrivateRepo = true
	pubEntry := signedEntry("public-skill", hashB())
	m := &Manifest{Content: []ContentEntry{privEntry, pubEntry}}
	cat := &catalog.Catalog{Items: []catalog.ContentItem{
		{Name: "private-skill", Registry: "reg"},
		{Name: "public-skill", Registry: "reg"},
	}}

	EnrichCatalog(cat, "reg", m)

	if !cat.Items[0].PrivateRepo {
		t.Error("PrivateRepo=true not propagated for private entry")
	}
	if cat.Items[1].PrivateRepo {
		t.Error("PrivateRepo set on public entry")
	}
}

// TestEnrichCatalog_RecallSourceDefaultsRegistry exercises the MOAT spec
// default: a Revocation with empty source is treated as registry-source.
// EffectiveSource() owns the default; enrich must preserve it.
func TestEnrichCatalog_RecallSourceDefaultsRegistry(t *testing.T) {
	t.Parallel()
	m := &Manifest{
		Name:                   "example-registry",
		Operator:               "Example Inc",
		RegistrySigningProfile: SigningProfile{Issuer: "https://iss", Subject: "ops@example.com"},
		Content:                []ContentEntry{signedEntry("x", hashA())},
		Revocations: []Revocation{{
			ContentHash: hashA(),
			Reason:      RevocationReasonCompromised,
			// Source intentionally empty — MOAT default is "registry".
		}},
	}
	cat := &catalog.Catalog{Items: []catalog.ContentItem{{Name: "x", Registry: "reg"}}}

	EnrichCatalog(cat, "reg", m)

	if got := cat.Items[0].RecallSource; got != RevocationSourceRegistry {
		t.Errorf("RecallSource = %q, want %q", got, RevocationSourceRegistry)
	}
	// Registry-source issuer uses Operator when present.
	if got := cat.Items[0].RecallIssuer; got != "Example Inc" {
		t.Errorf("RecallIssuer = %q, want %q", got, "Example Inc")
	}
}

// TestEnrichCatalog_RecallSourcePublisher proves the publisher branch
// reports the right source + uses the per-entry signing profile subject.
func TestEnrichCatalog_RecallSourcePublisher(t *testing.T) {
	t.Parallel()
	entry := dualAttestedEntry("x", hashA())
	// dualAttestedEntry sets SigningProfile with subject "pub@example.com".
	m := &Manifest{
		Content: []ContentEntry{entry},
		Revocations: []Revocation{{
			ContentHash: hashA(),
			Source:      RevocationSourcePublisher,
			Reason:      RevocationReasonDeprecated,
		}},
	}
	cat := &catalog.Catalog{Items: []catalog.ContentItem{{Name: "x", Registry: "reg"}}}

	EnrichCatalog(cat, "reg", m)

	if got := cat.Items[0].RecallSource; got != RevocationSourcePublisher {
		t.Errorf("RecallSource = %q, want %q", got, RevocationSourcePublisher)
	}
	if got := cat.Items[0].RecallIssuer; got != "pub@example.com" {
		t.Errorf("RecallIssuer = %q, want %q", got, "pub@example.com")
	}
}

// TestEnrichCatalog_RecallIssuerRegistryFallback: if Manifest.Operator is
// empty, the resolver falls back to RegistrySigningProfile.Subject. The
// manifest validator guarantees Subject is non-empty, so this path is
// always safe at runtime; we exercise it explicitly.
func TestEnrichCatalog_RecallIssuerRegistryFallback(t *testing.T) {
	t.Parallel()
	m := &Manifest{
		// No Operator.
		RegistrySigningProfile: SigningProfile{Issuer: "https://iss", Subject: "ops@example.com"},
		Content:                []ContentEntry{signedEntry("x", hashA())},
		Revocations: []Revocation{{
			ContentHash: hashA(),
			Source:      RevocationSourceRegistry,
			Reason:      RevocationReasonMalicious,
		}},
	}
	cat := &catalog.Catalog{Items: []catalog.ContentItem{{Name: "x", Registry: "reg"}}}

	EnrichCatalog(cat, "reg", m)

	if got := cat.Items[0].RecallIssuer; got != "ops@example.com" {
		t.Errorf("RecallIssuer fallback = %q, want RegistrySigningProfile.Subject", got)
	}
}

// TestEnrichCatalog_RecallIssuerPublisherFallback: when a publisher-source
// revocation lands on an entry with no SigningProfile, the resolver must
// still produce non-empty text so the drill-down banner has something to
// render. The sentinel is a committed contract — tests in the TUI rely on
// exactly this string.
func TestEnrichCatalog_RecallIssuerPublisherFallback(t *testing.T) {
	t.Parallel()
	m := &Manifest{
		Content: []ContentEntry{signedEntry("x", hashA())}, // no SigningProfile
		Revocations: []Revocation{{
			ContentHash: hashA(),
			Source:      RevocationSourcePublisher,
			Reason:      RevocationReasonDeprecated,
		}},
	}
	cat := &catalog.Catalog{Items: []catalog.ContentItem{{Name: "x", Registry: "reg"}}}

	EnrichCatalog(cat, "reg", m)

	want := "(publisher — identity not provided)"
	if got := cat.Items[0].RecallIssuer; got != want {
		t.Errorf("RecallIssuer sentinel = %q, want %q", got, want)
	}
}

// TestEnrichCatalog_SanitizesPublisherStrings pins down the enrich-boundary
// security contract: every publisher-controlled display string runs through
// SanitizeForDisplay before it lands on ContentItem. A malicious manifest
// cannot place terminal control bytes or bidi overrides on a user's TUI.
func TestEnrichCatalog_SanitizesPublisherStrings(t *testing.T) {
	t.Parallel()
	// Reason: ANSI SGR wrapping plus a bidi override.
	hostileReason := "\x1b[31mMalicious\x1b[0m\u202E content"
	// DetailsURL: null-byte injection attempt.
	hostileURL := "https://example.com/revs/\x00../../etc/passwd"
	// Per-entry signing profile subject: colored text + trailing newline.
	hostileSubject := "\x1b[32mpub@evil.com\x1b[0m\n"

	entry := dualAttestedEntry("x", hashA())
	entry.SigningProfile.Subject = hostileSubject

	m := &Manifest{
		Content: []ContentEntry{entry},
		Revocations: []Revocation{{
			ContentHash: hashA(),
			Source:      RevocationSourcePublisher,
			Reason:      hostileReason,
			DetailsURL:  hostileURL,
		}},
	}
	cat := &catalog.Catalog{Items: []catalog.ContentItem{{Name: "x", Registry: "reg"}}}

	EnrichCatalog(cat, "reg", m)

	if got := cat.Items[0].RecallReason; got != "Malicious content" {
		t.Errorf("RecallReason = %q; want sanitized %q", got, "Malicious content")
	}
	if got := cat.Items[0].RecallDetailsURL; got != "https://example.com/revs/../../etc/passwd" {
		t.Errorf("RecallDetailsURL = %q; null byte not stripped", got)
	}
	if got := cat.Items[0].RecallIssuer; got != "pub@evil.com" {
		t.Errorf("RecallIssuer = %q; want sanitized %q", got, "pub@evil.com")
	}
}

// TestEnrichCatalog_NonRecalledItemHasZeroDrillDownFields documents the
// contract that drill-down fields stay zero on a verified-but-not-recalled
// item. Consumers rely on this to branch purely on `Recalled` without
// worrying about stale RecallSource / RecallIssuer data from a prior run.
func TestEnrichCatalog_NonRecalledItemHasZeroDrillDownFields(t *testing.T) {
	t.Parallel()
	m := &Manifest{
		Name:                   "reg",
		Operator:               "Example",
		RegistrySigningProfile: SigningProfile{Issuer: "https://iss", Subject: "s"},
		Content:                []ContentEntry{signedEntry("ok", hashA())},
		Revocations: []Revocation{{
			ContentHash: hashB(), // different hash — no match
			Reason:      RevocationReasonDeprecated,
			DetailsURL:  "https://example.com/revs/other",
		}},
	}
	cat := &catalog.Catalog{Items: []catalog.ContentItem{{Name: "ok", Registry: "reg"}}}

	EnrichCatalog(cat, "reg", m)

	item := cat.Items[0]
	if item.Recalled {
		t.Fatalf("Recalled set on non-matching item")
	}
	if item.RecallSource != "" || item.RecallDetailsURL != "" || item.RecallIssuer != "" {
		t.Errorf("drill-down fields leaked onto non-recalled item: source=%q url=%q issuer=%q",
			item.RecallSource, item.RecallDetailsURL, item.RecallIssuer)
	}
}
