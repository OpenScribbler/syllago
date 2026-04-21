package moat

import (
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"

	rekorv1 "github.com/sigstore/protobuf-specs/gen/pb-go/rekor/v1"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tlog"
	sgsig "github.com/sigstore/sigstore/pkg/signature"
)

// Regression tests for the sigstore-go v1.1.4 NewEntry shard-index bug.
//
// rekor.sigstore.dev runs multiple transparency log shards. The top-level
// Rekor API "logIndex" is a global virtual index that monotonically counts
// entries across every shard; the nested verification.inclusionProof.logIndex
// is the shard-local position, which is what Merkle-tree math inside
// VerifyInclusion requires. Tree sizes are shard-local, so on shard 2+ a
// global index can (and routinely does) exceed the current shard's tree
// size.
//
// tlog.NewEntry (and tlog.GenerateTransparencyLogEntry) in sigstore-go v1.1.4
// overwrite InclusionProof.LogIndex with the top-level global index
// (entry.go:114), producing an inclusion proof that VerifyInclusion rejects
// whenever global_index >= shard_tree_size. Syllago dodges this by hand-
// populating a *rekorv1.TransparencyLogEntry via buildTransparencyLogEntry
// and handing it to tlog.ParseTransparencyLogEntry directly.
//
// These tests pin that workaround. If someone replaces buildTransparencyLogEntry
// with tle.NewEntry / tle.GenerateTransparencyLogEntry (deceptively tempting
// because "newer API, less code"), these tests fail.
//
// Project-memory reference: sigstore-go-NewEntry-shardindex-bug.

// TestBuildTransparencyLogEntry_PreservesShardLocalLogIndex is the direct
// pin: after buildTransparencyLogEntry runs, the inclusion-proof LogIndex
// must equal the shard-local value from the Rekor API response, not the
// global top-level LogIndex.
func TestBuildTransparencyLogEntry_PreservesShardLocalLogIndex(t *testing.T) {
	t.Parallel()

	rekorRaw := loadRekorFixture(t)
	entry, err := parseRekorEntry(rekorRaw)
	if err != nil {
		t.Fatalf("parseRekorEntry on fixture: %v", err)
	}

	// Fixture invariants the bug depends on. If these change, the fixture
	// was regenerated against a different Rekor entry and this test may no
	// longer exercise the shard-index code path — regenerate against a
	// shard-2+ entry (global > tree_size) to keep the regression meaningful.
	const (
		wantGlobal     int64 = 1336116369
		wantShardLocal int64 = 1214212107
		wantTreeSize   int64 = 1215255133
	)
	if entry.LogIndex != wantGlobal {
		t.Fatalf("fixture global logIndex changed: got %d want %d — regenerate fixture against a shard-2+ entry",
			entry.LogIndex, wantGlobal)
	}
	if entry.Verification.InclusionProof.LogIndex != wantShardLocal {
		t.Fatalf("fixture shard-local logIndex changed: got %d want %d",
			entry.Verification.InclusionProof.LogIndex, wantShardLocal)
	}
	if entry.Verification.InclusionProof.TreeSize != wantTreeSize {
		t.Fatalf("fixture tree size changed: got %d want %d",
			entry.Verification.InclusionProof.TreeSize, wantTreeSize)
	}
	if entry.LogIndex <= entry.Verification.InclusionProof.TreeSize {
		t.Fatalf("fixture no longer triggers the bug: global logIndex (%d) must exceed shard tree size (%d)",
			entry.LogIndex, entry.Verification.InclusionProof.TreeSize)
	}

	bodyBytes, err := base64.StdEncoding.DecodeString(entry.Body)
	if err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	logID, err := hex.DecodeString(entry.LogID)
	if err != nil {
		t.Fatalf("decoding logID: %v", err)
	}
	set, err := base64.StdEncoding.DecodeString(entry.Verification.SignedEntryTimestamp)
	if err != nil {
		t.Fatalf("decoding SET: %v", err)
	}

	tle, err := buildTransparencyLogEntry(entry, bodyBytes, logID, set)
	if err != nil {
		t.Fatalf("buildTransparencyLogEntry: %v", err)
	}

	// Top-level LogIndex must be the global — the SET hashes the global index.
	if tle.LogIndex != wantGlobal {
		t.Errorf("tle.LogIndex = %d, want global index %d", tle.LogIndex, wantGlobal)
	}

	// InclusionProof.LogIndex must be shard-local. This is the precise field
	// the sigstore-go NewEntry bug clobbers.
	if tle.InclusionProof == nil {
		t.Fatal("tle.InclusionProof is nil; buildTransparencyLogEntry did not populate the proof")
	}
	if tle.InclusionProof.LogIndex != wantShardLocal {
		t.Errorf("tle.InclusionProof.LogIndex = %d, want shard-local %d — did buildTransparencyLogEntry get replaced with tle.NewEntry?",
			tle.InclusionProof.LogIndex, wantShardLocal)
	}
	if tle.InclusionProof.LogIndex == tle.LogIndex {
		t.Errorf("tle.InclusionProof.LogIndex (%d) must NOT equal top-level LogIndex (%d); that is the sigstore-go NewEntry bug signature",
			tle.InclusionProof.LogIndex, tle.LogIndex)
	}
	if tle.InclusionProof.TreeSize != wantTreeSize {
		t.Errorf("tle.InclusionProof.TreeSize = %d, want %d", tle.InclusionProof.TreeSize, wantTreeSize)
	}
}

// TestBuildTransparencyLogEntry_VerifyInclusionRejectsGlobalIndex demonstrates
// the bug mechanism end-to-end: constructs two TransparencyLogEntry values
// from the same fixture — the correct variant from buildTransparencyLogEntry
// and a "buggy" variant mirroring what sigstore-go NewEntry produces — then
// checks that VerifyInclusion accepts the correct one and rejects the buggy
// one. If someone "fixes" buildTransparencyLogEntry by routing through
// tle.NewEntry, the correct path in this test will start failing.
func TestBuildTransparencyLogEntry_VerifyInclusionRejectsGlobalIndex(t *testing.T) {
	t.Parallel()

	rekorRaw := loadRekorFixture(t)
	entry, err := parseRekorEntry(rekorRaw)
	if err != nil {
		t.Fatalf("parseRekorEntry: %v", err)
	}
	tr, err := root.NewTrustedRootFromJSON(loadTrustedRoot(t))
	if err != nil {
		t.Fatalf("parsing trusted root: %v", err)
	}

	bodyBytes, err := base64.StdEncoding.DecodeString(entry.Body)
	if err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	logID, err := hex.DecodeString(entry.LogID)
	if err != nil {
		t.Fatalf("decoding logID: %v", err)
	}
	set, err := base64.StdEncoding.DecodeString(entry.Verification.SignedEntryTimestamp)
	if err != nil {
		t.Fatalf("decoding SET: %v", err)
	}

	correctTLE, err := buildTransparencyLogEntry(entry, bodyBytes, logID, set)
	if err != nil {
		t.Fatalf("buildTransparencyLogEntry: %v", err)
	}

	// Buggy variant: clone the correct TLE and overwrite InclusionProof.LogIndex
	// with the global top-level LogIndex — this is what tlog.NewEntry does
	// internally at sigstore-go v1.1.4 entry.go:114.
	buggyTLE := &rekorv1.TransparencyLogEntry{
		LogIndex:          correctTLE.LogIndex,
		LogId:             correctTLE.LogId,
		KindVersion:       correctTLE.KindVersion,
		IntegratedTime:    correctTLE.IntegratedTime,
		CanonicalizedBody: correctTLE.CanonicalizedBody,
		InclusionPromise:  correctTLE.InclusionPromise,
		InclusionProof: &rekorv1.InclusionProof{
			LogIndex:   correctTLE.LogIndex, // BUG: clobbered with global cross-shard index
			RootHash:   correctTLE.InclusionProof.RootHash,
			TreeSize:   correctTLE.InclusionProof.TreeSize,
			Hashes:     correctTLE.InclusionProof.Hashes,
			Checkpoint: correctTLE.InclusionProof.Checkpoint,
		},
	}

	hexKeyID := hex.EncodeToString(logID)
	tlogVerifier, ok := tr.RekorLogs()[hexKeyID]
	if !ok {
		t.Fatalf("rekor log %s not in trusted root", hexKeyID)
	}
	sigVerifier, err := sgsig.LoadVerifier(tlogVerifier.PublicKey, tlogVerifier.SignatureHashFunc)
	if err != nil {
		t.Fatalf("loading rekor verifier: %v", err)
	}

	// Correct TLE: inclusion must verify.
	correctEntry, err := tlog.ParseTransparencyLogEntry(correctTLE)
	if err != nil {
		t.Fatalf("ParseTransparencyLogEntry on correct TLE: %v", err)
	}
	if err := tlog.VerifyInclusion(correctEntry, sigVerifier); err != nil {
		t.Fatalf("VerifyInclusion on correct (shard-local) TLE must succeed: %v", err)
	}

	// Buggy TLE: must be rejected somewhere before or during VerifyInclusion.
	// Either ParseTransparencyLogEntry catches the out-of-range index early,
	// or VerifyInclusion fails on the Merkle path — both are acceptable
	// regression-guard outcomes, so long as the buggy path does NOT succeed.
	buggyEntry, parseErr := tlog.ParseTransparencyLogEntry(buggyTLE)
	if parseErr != nil {
		return // sigstore-go rejected the overflow early
	}
	err = tlog.VerifyInclusion(buggyEntry, sigVerifier)
	if err == nil {
		t.Fatal("VerifyInclusion on buggy TLE must fail (InclusionProof.LogIndex was overwritten with the global cross-shard index, exceeding shard tree size), but it succeeded — did someone silently fix the sigstore-go NewEntry bug? Re-evaluate whether the workaround is still needed.")
	}
	// Sanity check: error should reference the inclusion proof in some form.
	// If the wording drifts away from these keywords, the regression signal on
	// "did sigstore-go silently fix NewEntry?" becomes noisy — flag loudly.
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "index") && !strings.Contains(msg, "tree") && !strings.Contains(msg, "inclusion") && !strings.Contains(msg, "proof") {
		t.Errorf("VerifyInclusion on buggy TLE failed as expected, but the error wording no longer mentions index/tree/inclusion/proof — future readers cannot tell whether this is the shard-index regression or a different failure; got: %v", err)
	}
}
