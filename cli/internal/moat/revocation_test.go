package moat

import (
	"reflect"
	"testing"
)

// Stable hashes for tests. Content of the hash doesn't matter — these are
// opaque identifiers in the revocation index.
const (
	revHashA = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	revHashB = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	revHashC = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
)

// makeRev constructs a Revocation without going through ParseManifest so tests
// can exercise the enforcement layer independently of manifest validation.
func makeRev(hash, reason, source, detailsURL string) Revocation {
	return Revocation{
		ContentHash: hash,
		Reason:      reason,
		DetailsURL:  detailsURL,
		Source:      source,
	}
}

func TestRevocationStatus_String(t *testing.T) {
	cases := map[RevocationStatus]string{
		RevStatusNone:          "none",
		RevStatusRegistryBlock: "registry-block",
		RevStatusPublisherWarn: "publisher-warn",
		RevocationStatus(99):   "unknown",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("RevocationStatus(%d).String() = %q, want %q", s, got, want)
		}
	}
}

func TestRevocationSet_AddFromManifest_RegistrySourceBlocks(t *testing.T) {
	set := NewRevocationSet()
	m := &Manifest{
		Revocations: []Revocation{
			makeRev(revHashA, RevocationReasonMalicious, RevocationSourceRegistry, "https://example.com/a"),
		},
	}
	set.AddFromManifest(m, "https://registry.example.com/manifest.json")

	recs := set.Lookup(revHashA)
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	if recs[0].Status != RevStatusRegistryBlock {
		t.Errorf("registry source must map to RevStatusRegistryBlock, got %s", recs[0].Status)
	}
	if recs[0].IssuingRegistryURL != "https://registry.example.com/manifest.json" {
		t.Errorf("issuing registry not propagated, got %q", recs[0].IssuingRegistryURL)
	}
	if recs[0].Source != RevocationSourceRegistry {
		t.Errorf("source not preserved, got %q", recs[0].Source)
	}
}

func TestRevocationSet_AddFromManifest_PublisherSourceWarns(t *testing.T) {
	set := NewRevocationSet()
	m := &Manifest{
		Revocations: []Revocation{
			makeRev(revHashB, RevocationReasonDeprecated, RevocationSourcePublisher, "https://example.com/b"),
		},
	}
	set.AddFromManifest(m, "https://r.example/mf")

	recs := set.Lookup(revHashB)
	if len(recs) != 1 || recs[0].Status != RevStatusPublisherWarn {
		t.Fatalf("publisher source must map to RevStatusPublisherWarn, got %+v", recs)
	}
}

func TestRevocationSet_AbsentSourceDefaultsToRegistry(t *testing.T) {
	// Spec §Revocation Mechanism: absent source defaults to "registry" — that
	// means HARD-BLOCK, not WARN. If this regresses, unlisted revocations
	// would silently downgrade to warn, which is a safety bug.
	set := NewRevocationSet()
	m := &Manifest{
		Revocations: []Revocation{
			{ContentHash: revHashA, Reason: RevocationReasonMalicious, DetailsURL: "https://x"}, // no source
		},
	}
	set.AddFromManifest(m, "https://r/mf")

	recs := set.Lookup(revHashA)
	if len(recs) != 1 {
		t.Fatalf("expected 1 record")
	}
	if recs[0].Status != RevStatusRegistryBlock {
		t.Errorf("absent source must default to registry-block, got %s", recs[0].Status)
	}
	if recs[0].Source != RevocationSourceRegistry {
		t.Errorf("EffectiveSource should backfill 'registry', got %q", recs[0].Source)
	}
}

func TestRevocationSet_UnknownReasonCarriedVerbatim(t *testing.T) {
	// Forward-compat: a future registry may ship a reason outside the v0.6.0
	// closed set. The enforcement layer MUST NOT reject or rewrite the
	// string — it should pass through so the UI can display it.
	set := NewRevocationSet()
	m := &Manifest{
		Revocations: []Revocation{
			makeRev(revHashA, "supply_chain_breach", RevocationSourceRegistry, "https://x"),
		},
	}
	set.AddFromManifest(m, "https://r/mf")

	recs := set.Lookup(revHashA)
	if len(recs) != 1 {
		t.Fatalf("expected 1 record")
	}
	if recs[0].Reason != "supply_chain_breach" {
		t.Errorf("unknown reason must be preserved verbatim, got %q", recs[0].Reason)
	}
	// Must still produce an enforcement decision — unknown reason + known
	// source should still block.
	if recs[0].Status != RevStatusRegistryBlock {
		t.Errorf("unknown reason with registry source should still block, got %s", recs[0].Status)
	}
}

func TestRevocationSet_UnknownSourceFailsClosed(t *testing.T) {
	// Spec §Revocation Mechanism (G-17): clients MUST branch on the explicit
	// source field. ParseManifest rejects unknown source values, but this
	// enforcement layer is reached by programmatically-built Revocations too
	// (in-process manifest builders, tests, future callers). If the source
	// value is anything other than "publisher", enforcement MUST fall through
	// to a registry-block — never downgrade to warn on an unrecognized value.
	set := NewRevocationSet()
	m := &Manifest{
		Revocations: []Revocation{
			makeRev(revHashA, RevocationReasonMalicious, "future_source", "https://x"),
		},
	}
	set.AddFromManifest(m, "https://r/mf")

	recs := set.Lookup(revHashA)
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	if recs[0].Status != RevStatusRegistryBlock {
		t.Errorf("unknown source must fail closed to registry-block, got %s", recs[0].Status)
	}
	// Source is carried verbatim — enforcement downgrades to block but the
	// original string is preserved for diagnostics.
	if recs[0].Source != "future_source" {
		t.Errorf("unknown source value must be preserved verbatim, got %q", recs[0].Source)
	}
}

func TestRevocationSet_StatusBranchesOnSourceNotContext(t *testing.T) {
	// Spec §Revocation Mechanism (G-17): enforcement decision is a pure
	// function of source. Same hash, same reason, same details_url — flipping
	// source MUST flip Status. This guards against a future refactor that
	// tries to "infer" source from reason or other fields.
	set := NewRevocationSet()
	m := &Manifest{
		Revocations: []Revocation{
			makeRev(revHashA, RevocationReasonMalicious, RevocationSourceRegistry, "https://same"),
			makeRev(revHashB, RevocationReasonMalicious, RevocationSourcePublisher, "https://same"),
		},
	}
	set.AddFromManifest(m, "https://r/mf")

	a := set.Lookup(revHashA)
	b := set.Lookup(revHashB)
	if len(a) != 1 || a[0].Status != RevStatusRegistryBlock {
		t.Fatalf("hash A (source=registry) must block, got %+v", a)
	}
	if len(b) != 1 || b[0].Status != RevStatusPublisherWarn {
		t.Fatalf("hash B (source=publisher) must warn, got %+v", b)
	}
}

func TestRevocationSet_MultipleRegistriesIndexIndependently(t *testing.T) {
	// The same hash can be revoked by multiple registries; Lookup should
	// return every record so the caller can attribute each to its source.
	set := NewRevocationSet()
	m1 := &Manifest{Revocations: []Revocation{makeRev(revHashA, RevocationReasonMalicious, RevocationSourceRegistry, "https://d1")}}
	m2 := &Manifest{Revocations: []Revocation{makeRev(revHashA, RevocationReasonDeprecated, RevocationSourcePublisher, "https://d2")}}
	set.AddFromManifest(m1, "https://reg1/mf")
	set.AddFromManifest(m2, "https://reg2/mf")

	recs := set.Lookup(revHashA)
	if len(recs) != 2 {
		t.Fatalf("expected 2 records (one per registry), got %d", len(recs))
	}
	// Lookup returns insertion order; both registries must be represented.
	found := map[string]bool{}
	for _, r := range recs {
		found[r.IssuingRegistryURL] = true
	}
	if !found["https://reg1/mf"] || !found["https://reg2/mf"] {
		t.Errorf("expected both registries in records, got %+v", recs)
	}
}

func TestRevocationSet_NilSafeOps(t *testing.T) {
	var s *RevocationSet
	if got := s.Lookup(revHashA); got != nil {
		t.Errorf("nil set Lookup = %v, want nil", got)
	}
	if got := s.Len(); got != 0 {
		t.Errorf("nil set Len = %d, want 0", got)
	}
	s.AddFromManifest(&Manifest{}, "https://r") // must not panic

	set := NewRevocationSet()
	set.AddFromManifest(nil, "https://r") // nil manifest must be a no-op
	if set.Len() != 0 {
		t.Errorf("nil manifest should leave set empty, got Len=%d", set.Len())
	}
}

func TestSession_WarnOncePerSession(t *testing.T) {
	sess := NewSession()
	if !sess.ShouldWarn("https://r/mf", revHashA) {
		t.Error("first ShouldWarn must return true")
	}
	sess.MarkConfirmed("https://r/mf", revHashA)
	if sess.ShouldWarn("https://r/mf", revHashA) {
		t.Error("second ShouldWarn after MarkConfirmed must return false")
	}
}

func TestSession_KeyDistinguishesRegistryAndHash(t *testing.T) {
	// Confirming (reg1, hashA) must not silence (reg2, hashA) nor (reg1, hashB).
	sess := NewSession()
	sess.MarkConfirmed("https://reg1/mf", revHashA)

	if !sess.ShouldWarn("https://reg2/mf", revHashA) {
		t.Error("different registry should still warn")
	}
	if !sess.ShouldWarn("https://reg1/mf", revHashB) {
		t.Error("different hash should still warn")
	}
	if sess.ShouldWarn("https://reg1/mf", revHashA) {
		t.Error("same key should be suppressed")
	}
}

func TestSession_NilSafe(t *testing.T) {
	var sess *Session
	if !sess.ShouldWarn("r", "h") {
		t.Error("nil session must always warn (fail open)")
	}
	sess.MarkConfirmed("r", "h") // must not panic
}

func TestCheckLockfile_FiltersRevokedEntries(t *testing.T) {
	lf := NewLockfile()
	lf.Entries = []LockEntry{
		{Name: "good", ContentHash: revHashC},
		{Name: "blocked", ContentHash: revHashA},
		{Name: "warned", ContentHash: revHashB},
	}

	set := NewRevocationSet()
	set.AddFromManifest(&Manifest{
		Revocations: []Revocation{
			makeRev(revHashA, RevocationReasonMalicious, RevocationSourceRegistry, "https://x/a"),
			makeRev(revHashB, RevocationReasonDeprecated, RevocationSourcePublisher, "https://x/b"),
		},
	}, "https://r/mf")

	got := CheckLockfile(lf, set)
	if len(got) != 2 {
		t.Fatalf("expected 2 revoked entries, got %d", len(got))
	}
	// Deterministic order: registry URL, then hash. hashA sorts before hashB.
	if got[0].ContentHash != revHashA || got[1].ContentHash != revHashB {
		t.Errorf("expected sorted by hash (A, B), got (%s, %s)", got[0].ContentHash, got[1].ContentHash)
	}
	if got[0].Status != RevStatusRegistryBlock {
		t.Errorf("hashA should be RevStatusRegistryBlock, got %s", got[0].Status)
	}
	if got[1].Status != RevStatusPublisherWarn {
		t.Errorf("hashB should be RevStatusPublisherWarn, got %s", got[1].Status)
	}
}

func TestCheckLockfile_DedupesWithinRegistryHashSource(t *testing.T) {
	// If two lockfile entries have the same ContentHash and the revocation
	// set has one record for that hash, CheckLockfile must not return the
	// record twice.
	lf := NewLockfile()
	lf.Entries = []LockEntry{
		{Name: "first", ContentHash: revHashA},
		{Name: "second", ContentHash: revHashA}, // same hash, different name
	}

	set := NewRevocationSet()
	set.AddFromManifest(&Manifest{
		Revocations: []Revocation{makeRev(revHashA, RevocationReasonMalicious, RevocationSourceRegistry, "https://x")},
	}, "https://r/mf")

	got := CheckLockfile(lf, set)
	if len(got) != 1 {
		t.Fatalf("expected 1 deduped record, got %d", len(got))
	}
}

func TestCheckLockfile_NilInputs(t *testing.T) {
	if got := CheckLockfile(nil, NewRevocationSet()); got != nil {
		t.Errorf("nil lockfile should yield nil, got %v", got)
	}
	if got := CheckLockfile(NewLockfile(), nil); got != nil {
		t.Errorf("nil set should yield nil, got %v", got)
	}
}

func TestCheckLockfile_NoRevocationsReturnsNil(t *testing.T) {
	lf := NewLockfile()
	lf.Entries = []LockEntry{{Name: "ok", ContentHash: revHashA}}
	set := NewRevocationSet() // empty
	if got := CheckLockfile(lf, set); got != nil {
		t.Errorf("clean lockfile should yield nil, got %v", got)
	}
}

func TestCheckLockfile_DeterministicOrder(t *testing.T) {
	// Three registries, each revoking one hash. The result must be sorted
	// stably by (registry, hash, source) regardless of insertion order.
	lf := NewLockfile()
	lf.Entries = []LockEntry{
		{Name: "x", ContentHash: revHashA},
		{Name: "y", ContentHash: revHashB},
		{Name: "z", ContentHash: revHashC},
	}

	set := NewRevocationSet()
	// Insertion order deliberately inverse of expected sorted output.
	set.AddFromManifest(&Manifest{Revocations: []Revocation{makeRev(revHashC, "malicious", RevocationSourceRegistry, "https://d3")}}, "https://reg-z/mf")
	set.AddFromManifest(&Manifest{Revocations: []Revocation{makeRev(revHashA, "malicious", RevocationSourceRegistry, "https://d1")}}, "https://reg-a/mf")
	set.AddFromManifest(&Manifest{Revocations: []Revocation{makeRev(revHashB, "deprecated", RevocationSourcePublisher, "https://d2")}}, "https://reg-m/mf")

	got := CheckLockfile(lf, set)
	wantRegistries := []string{"https://reg-a/mf", "https://reg-m/mf", "https://reg-z/mf"}
	gotRegistries := make([]string, len(got))
	for i, r := range got {
		gotRegistries[i] = r.IssuingRegistryURL
	}
	if !reflect.DeepEqual(gotRegistries, wantRegistries) {
		t.Errorf("output not sorted by registry URL: got %v, want %v", gotRegistries, wantRegistries)
	}
}

func TestRevocationRecord_PreservesAllFields(t *testing.T) {
	// Regression: a past refactor dropped DetailsURL — verify every field
	// survives the AddFromManifest → Lookup round trip.
	set := NewRevocationSet()
	m := &Manifest{
		Revocations: []Revocation{{
			ContentHash: revHashA,
			Reason:      RevocationReasonCompromised,
			DetailsURL:  "https://advisory.example.com/CVE-2026-0001",
			Source:      RevocationSourcePublisher,
		}},
	}
	set.AddFromManifest(m, "https://r/mf")

	recs := set.Lookup(revHashA)
	if len(recs) != 1 {
		t.Fatalf("expected 1 record")
	}
	want := RevocationRecord{
		ContentHash:        revHashA,
		Reason:             RevocationReasonCompromised,
		DetailsURL:         "https://advisory.example.com/CVE-2026-0001",
		Source:             RevocationSourcePublisher,
		IssuingRegistryURL: "https://r/mf",
		Status:             RevStatusPublisherWarn,
	}
	if !reflect.DeepEqual(recs[0], want) {
		t.Errorf("record fields lost:\n got %+v\nwant %+v", recs[0], want)
	}
}
