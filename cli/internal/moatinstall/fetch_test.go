package moatinstall

// Tests for FetchAndRecord and helpers (moved from cmd/syllago when the
// orchestration was extracted into this package — bead syllago-kdxus
// Phase 2; rewritten in syllago-cvwj5 to follow the spec-correct
// clone+tree-hash flow).
//
// Scope:
//   - FetchAndRecord — end-to-end Proceed path: clone + content_hash
//     verify + copyTree + RecordInstall + lf.Save. Also exercises the
//     two early refusals (non-UNSIGNED tier with no profile, non-https
//     scheme), the hash-mismatch failure, and the Dual-Attested second
//     leg failure modes.
//
// Test seam: CloneRepoFn is stubbed with a copyTree-from-fixture function
// so we exercise the full hash + extract path without spawning git.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// makeRepoFixture creates a fake source-repo tree at
// <root>/<categoryDir>/<name>/SKILL.md (or whatever filename the caller
// supplies) and returns the spec-correct content_hash for the item
// subdirectory. Mirrors moat-spec.md §"Repository Layout": the item lives
// at <category>/<name>/, and content_hash covers that subdirectory.
func makeRepoFixture(t *testing.T, root, categoryDir, name string, files map[string]string) string {
	t.Helper()
	itemDir := filepath.Join(root, categoryDir, name)
	if err := os.MkdirAll(itemDir, 0o755); err != nil {
		t.Fatalf("mkdir item: %v", err)
	}
	for rel, content := range files {
		full := filepath.Join(itemDir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir parent: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}
	h, err := moat.ContentHash(itemDir)
	if err != nil {
		t.Fatalf("ContentHash: %v", err)
	}
	return h
}

// stubCloneFromFixture replaces CloneRepoFn with one that copies
// fixtureRoot into the production code's destDir, mimicking a successful
// `git clone` of fixtureRoot.
func stubCloneFromFixture(t *testing.T, fixtureRoot string) {
	t.Helper()
	orig := CloneRepoFn
	CloneRepoFn = func(_ context.Context, _, destDir string) error {
		return copyTree(fixtureRoot, destDir)
	}
	t.Cleanup(func() { CloneRepoFn = orig })
}

// stubCloneRepoErr replaces CloneRepoFn with one that always returns retErr.
func stubCloneRepoErr(t *testing.T, retErr error) {
	t.Helper()
	orig := CloneRepoFn
	CloneRepoFn = func(_ context.Context, _, _ string) error { return retErr }
	t.Cleanup(func() { CloneRepoFn = orig })
}

// stubCloneScratchDir gives tests a hermetic temp dir for clone scratch.
func stubCloneScratchDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	orig := CloneScratchDir
	CloneScratchDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { CloneScratchDir = orig })
}

// stubSourceCacheDir points the install cache at a temp dir.
func stubSourceCacheDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig := SourceCacheDir
	SourceCacheDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { SourceCacheDir = orig })
	return dir
}

const fakeRepoURL = "https://github.com/example/repo"

func TestFetchAndRecord_Happy_Unsigned(t *testing.T) {
	fixtureRoot := t.TempDir()
	entryHash := makeRepoFixture(t, fixtureRoot, "skills", "my-skill", map[string]string{
		"SKILL.md": "# hi\n",
	})

	stubCloneFromFixture(t, fixtureRoot)
	stubCloneScratchDir(t)
	stubSourceCacheDir(t)

	projectRoot := t.TempDir()
	lf := &moat.Lockfile{}
	entry := &moat.ContentEntry{
		Name:        "my-skill",
		Type:        "skill",
		ContentHash: entryHash,
		SourceURI:   fakeRepoURL,
		AttestedAt:  time.Now().UTC(),
	}

	origNow := Now
	Now = func() time.Time { return time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { Now = origNow })

	dir, err := FetchAndRecord(
		context.Background(),
		entry,
		"example",
		"https://example.com/manifest.json",
		moat.LockfilePath(projectRoot),
		lf,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("FetchAndRecord: %v", err)
	}

	if !strings.Contains(dir, "example/my-skill") {
		t.Errorf("cache dir path missing registry/item components: %s", dir)
	}
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
		t.Errorf("SKILL.md missing from cache: %v", err)
	}
	if len(lf.Entries) != 1 {
		t.Fatalf("lockfile should have 1 entry, got %d", len(lf.Entries))
	}
	if lf.Entries[0].ContentHash != entryHash {
		t.Errorf("lockfile hash = %q, want %q", lf.Entries[0].ContentHash, entryHash)
	}
	if lf.Entries[0].TrustTier != "UNSIGNED" {
		t.Errorf("lockfile trust_tier = %q, want UNSIGNED", lf.Entries[0].TrustTier)
	}

	onDisk, err := moat.LoadLockfile(moat.LockfilePath(projectRoot))
	if err != nil {
		t.Fatalf("LoadLockfile: %v", err)
	}
	if len(onDisk.Entries) != 1 {
		t.Errorf("on-disk lockfile should have 1 entry, got %d", len(onDisk.Entries))
	}
}

func TestFetchAndRecord_HashMismatch(t *testing.T) {
	fixtureRoot := t.TempDir()
	// Real hash from the fixture, but we set entry.ContentHash to a wrong
	// value to simulate publisher-source drift.
	_ = makeRepoFixture(t, fixtureRoot, "skills", "my-skill", map[string]string{
		"SKILL.md": "# hi\n",
	})
	wrongHash := "sha256:" + strings.Repeat("aa", 32)

	stubCloneFromFixture(t, fixtureRoot)
	stubCloneScratchDir(t)
	stubSourceCacheDir(t)

	lf := &moat.Lockfile{}
	entry := &moat.ContentEntry{
		Name:        "my-skill",
		Type:        "skill",
		ContentHash: wrongHash,
		SourceURI:   fakeRepoURL,
	}
	_, err := FetchAndRecord(context.Background(), entry, "example", "https://example.com/m", "/tmp/lockfile.json", lf, nil, nil)
	assertStructuredCode(t, err, output.ErrMoatInvalid)
	var se output.StructuredError
	if !errors.As(err, &se) || !strings.Contains(se.Message, "content_hash mismatch") {
		t.Errorf("expected hash-mismatch message; got %+v", err)
	}
	if len(lf.Entries) != 0 {
		t.Errorf("lockfile must not be mutated on hash-mismatch; got %d entries", len(lf.Entries))
	}
}

// verifyCall is one captured invocation of the VerifyItem seam — used so
// Dual-Attested tests can inspect both the registry leg and the publisher
// leg independently.
type verifyCall struct {
	Item    moat.AttestationItem
	Profile *moat.SigningProfile
	Rekor   []byte
}

// verifyCapture aggregates multi-call captures plus a single-call view for
// existing tests that only assert on the last invocation.
type verifyCapture struct {
	Item    moat.AttestationItem
	Profile *moat.SigningProfile
	Rekor   []byte
	Called  int
	Calls   []verifyCall
}

func withVerifyItemStub(t *testing.T, result moat.VerificationResult, retErr error) *verifyCapture {
	t.Helper()
	captured := &verifyCapture{}
	orig := VerifyItem
	VerifyItem = func(item moat.AttestationItem, profile *moat.SigningProfile, rekorRaw []byte, trustedRootJSON []byte) (moat.VerificationResult, error) {
		rekorCopy := append([]byte(nil), rekorRaw...)
		captured.Item = item
		captured.Profile = profile
		captured.Rekor = rekorCopy
		captured.Called++
		captured.Calls = append(captured.Calls, verifyCall{Item: item, Profile: profile, Rekor: rekorCopy})
		return result, retErr
	}
	t.Cleanup(func() { VerifyItem = orig })
	return captured
}

func withRekorStub(t *testing.T, body []byte) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	orig := moat.RekorBaseURLForTest()
	moat.SetRekorBaseURLForTest(srv.URL)
	t.Cleanup(func() { moat.SetRekorBaseURLForTest(orig) })
}

// withRekorPerIndexStub serves a different body per Rekor logIndex.
// Dual-Attested tests need this because the registry leg and the publisher
// leg fetch different Rekor entries against the same package-level base URL.
func withRekorPerIndexStub(t *testing.T, bodies map[int64][]byte) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := r.URL.Query().Get("logIndex")
		var match []byte
		for k, v := range bodies {
			if fmt.Sprintf("%d", k) == idx {
				match = v
				break
			}
		}
		if match == nil {
			http.Error(w, "no fixture", http.StatusNotFound)
			return
		}
		_, _ = w.Write(match)
	}))
	t.Cleanup(srv.Close)
	orig := moat.RekorBaseURLForTest()
	moat.SetRekorBaseURLForTest(srv.URL)
	t.Cleanup(func() { moat.SetRekorBaseURLForTest(orig) })
}

// withPublisherAttestationStub overrides FetchPublisherAttestationFn to
// return canned bytes (or an error) without touching the network. Pair
// with withRekorPerIndexStub for end-to-end Dual-Attested coverage.
func withPublisherAttestationStub(t *testing.T, body []byte, retErr error) *struct{ Called int } {
	t.Helper()
	called := &struct{ Called int }{}
	orig := FetchPublisherAttestationFn
	FetchPublisherAttestationFn = func(_ context.Context, _ string) ([]byte, error) {
		called.Called++
		return body, retErr
	}
	t.Cleanup(func() { FetchPublisherAttestationFn = orig })
	return called
}

func signedFixture(srvURL, contentHash string, withProfile bool) *moat.ContentEntry {
	logIndex := int64(1336116369)
	entry := &moat.ContentEntry{
		Name:          "my-skill",
		Type:          "skill",
		ContentHash:   contentHash,
		SourceURI:     srvURL,
		AttestedAt:    time.Now().UTC(),
		RekorLogIndex: &logIndex,
	}
	if withProfile {
		entry.SigningProfile = &moat.SigningProfile{
			Issuer:  "https://token.actions.githubusercontent.com",
			Subject: "https://github.com/example/repo/.github/workflows/sign.yml@refs/heads/main",
		}
	}
	return entry
}

func TestFetchAndRecord_Happy_Signed(t *testing.T) {
	fixtureRoot := t.TempDir()
	entryHash := makeRepoFixture(t, fixtureRoot, "skills", "my-skill", map[string]string{
		"SKILL.md": "# hi\n",
	})

	rekorBytes := []byte(`{"abc123":{"body":"...","integratedTime":1700000000,"logID":"deadbeef","logIndex":1336116369,"verification":{"inclusionProof":{"checkpoint":"","hashes":[],"logIndex":1336116369,"rootHash":"","treeSize":100},"signedEntryTimestamp":""}}}`)
	withRekorStub(t, rekorBytes)
	captured := withVerifyItemStub(t, moat.VerificationResult{}, nil)

	stubCloneFromFixture(t, fixtureRoot)
	stubCloneScratchDir(t)
	stubSourceCacheDir(t)

	projectRoot := t.TempDir()
	lf := &moat.Lockfile{}
	entry := signedFixture(fakeRepoURL, entryHash, false)
	registryProfile := &moat.SigningProfile{
		Issuer:  "https://token.actions.githubusercontent.com",
		Subject: "https://github.com/example/repo/.github/workflows/registry.yml@refs/heads/main",
	}

	origNow := Now
	Now = func() time.Time { return time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { Now = origNow })

	dir, err := FetchAndRecord(
		context.Background(),
		entry,
		"example",
		"https://example.com/manifest.json",
		moat.LockfilePath(projectRoot),
		lf,
		registryProfile,
		[]byte(`{"trusted":"root"}`),
	)
	if err != nil {
		t.Fatalf("FetchAndRecord: %v", err)
	}
	if !strings.Contains(dir, "example/my-skill") {
		t.Errorf("cache dir path missing registry/item components: %s", dir)
	}

	if captured.Called != 1 {
		t.Fatalf("verify called %d times, want 1", captured.Called)
	}
	if captured.Profile != registryProfile {
		t.Errorf("SIGNED tier should pass registry-level profile to verify, got %+v", captured.Profile)
	}
	if captured.Item.ContentHash != entryHash {
		t.Errorf("AttestationItem.ContentHash = %q, want %q", captured.Item.ContentHash, entryHash)
	}
	if captured.Item.RekorLogIndex != 1336116369 {
		t.Errorf("AttestationItem.RekorLogIndex = %d, want 1336116369", captured.Item.RekorLogIndex)
	}
	if string(captured.Rekor) != string(rekorBytes) {
		t.Errorf("rekor bytes did not round-trip verbatim")
	}

	if len(lf.Entries) != 1 {
		t.Fatalf("lockfile should have 1 entry, got %d", len(lf.Entries))
	}
	if lf.Entries[0].TrustTier != "SIGNED" {
		t.Errorf("lockfile trust_tier = %q, want SIGNED", lf.Entries[0].TrustTier)
	}
	if string(lf.Entries[0].AttestationBundle) != string(rekorBytes) {
		t.Errorf("lockfile AttestationBundle did not preserve Rekor bytes")
	}
}

func TestFetchAndRecord_Happy_DualAttested(t *testing.T) {
	fixtureRoot := t.TempDir()
	entryHash := makeRepoFixture(t, fixtureRoot, "skills", "my-skill", map[string]string{
		"SKILL.md": "# hi\n",
	})

	registryRekorIdx := int64(1336116369)
	publisherRekorIdx := int64(1336116370)
	registryRekorBytes := []byte(`{"reg":{"body":"...","logIndex":1336116369}}`)
	publisherRekorBytes := []byte(`{"pub":{"body":"...","logIndex":1336116370}}`)
	withRekorPerIndexStub(t, map[int64][]byte{
		registryRekorIdx:  registryRekorBytes,
		publisherRekorIdx: publisherRekorBytes,
	})

	publisherJSON := []byte(fmt.Sprintf(
		`{"schema_version":1,"items":[{"name":"my-skill","content_hash":%q,"source_ref":"refs/heads/main","rekor_log_id":"x","rekor_log_index":%d}]}`,
		entryHash, publisherRekorIdx,
	))
	pubStub := withPublisherAttestationStub(t, publisherJSON, nil)
	captured := withVerifyItemStub(t, moat.VerificationResult{}, nil)

	stubCloneFromFixture(t, fixtureRoot)
	stubCloneScratchDir(t)
	stubSourceCacheDir(t)

	projectRoot := t.TempDir()
	lf := &moat.Lockfile{}
	entry := signedFixture(fakeRepoURL, entryHash, true)
	registryProfile := &moat.SigningProfile{
		Issuer:  "https://token.actions.githubusercontent.com",
		Subject: "https://github.com/example/repo/.github/workflows/registry.yml@refs/heads/main",
	}

	if _, err := FetchAndRecord(
		context.Background(), entry, "example", "https://example.com/m",
		moat.LockfilePath(projectRoot), lf, registryProfile, []byte(`{"trusted":"root"}`),
	); err != nil {
		t.Fatalf("FetchAndRecord: %v", err)
	}

	if pubStub.Called != 1 {
		t.Errorf("publisher attestation must be fetched exactly once for DUAL-ATTESTED; got %d", pubStub.Called)
	}
	if captured.Called != 2 {
		t.Fatalf("DUAL-ATTESTED requires 2 verify calls (registry + publisher); got %d", captured.Called)
	}
	if captured.Calls[0].Profile != registryProfile {
		t.Errorf("first verify call must pin to registry profile; got %+v", captured.Calls[0].Profile)
	}
	if string(captured.Calls[0].Rekor) != string(registryRekorBytes) {
		t.Errorf("first verify must receive registry Rekor bytes verbatim")
	}
	if captured.Calls[1].Profile != entry.SigningProfile {
		t.Errorf("second verify call must pin to per-item publisher profile; got %+v", captured.Calls[1].Profile)
	}
	if string(captured.Calls[1].Rekor) != string(publisherRekorBytes) {
		t.Errorf("second verify must receive publisher Rekor bytes verbatim")
	}
	if captured.Calls[1].Item.RekorLogIndex != publisherRekorIdx {
		t.Errorf("second verify must use publisher logIndex %d; got %d", publisherRekorIdx, captured.Calls[1].Item.RekorLogIndex)
	}
	if lf.Entries[0].TrustTier != "DUAL-ATTESTED" {
		t.Errorf("lockfile trust_tier = %q, want DUAL-ATTESTED", lf.Entries[0].TrustTier)
	}
	// Lockfile keeps the registry's per-item Rekor bundle (the one whose
	// content_hash and source_uri the manifest published) — not the
	// publisher's separate entry.
	if string(lf.Entries[0].AttestationBundle) != string(registryRekorBytes) {
		t.Errorf("lockfile AttestationBundle must hold registry Rekor bytes, not publisher's")
	}
}

func TestFetchAndRecord_Signed_RekorFetchFails(t *testing.T) {
	fixtureRoot := t.TempDir()
	entryHash := makeRepoFixture(t, fixtureRoot, "skills", "my-skill", map[string]string{
		"SKILL.md": "# hi\n",
	})

	rekorSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	t.Cleanup(rekorSrv.Close)
	orig := moat.RekorBaseURLForTest()
	moat.SetRekorBaseURLForTest(rekorSrv.URL)
	t.Cleanup(func() { moat.SetRekorBaseURLForTest(orig) })

	verifyCalled := 0
	origVerify := VerifyItem
	VerifyItem = func(_ moat.AttestationItem, _ *moat.SigningProfile, _ []byte, _ []byte) (moat.VerificationResult, error) {
		verifyCalled++
		return moat.VerificationResult{}, nil
	}
	t.Cleanup(func() { VerifyItem = origVerify })

	stubCloneFromFixture(t, fixtureRoot)
	stubCloneScratchDir(t)
	stubSourceCacheDir(t)

	lf := &moat.Lockfile{}
	entry := signedFixture(fakeRepoURL, entryHash, false)
	registryProfile := &moat.SigningProfile{
		Issuer: "https://token.actions.githubusercontent.com", Subject: "https://github.com/example/repo/.github/workflows/registry.yml@refs/heads/main",
	}

	_, err := FetchAndRecord(
		context.Background(), entry, "example", "https://example.com/m",
		"/tmp/lf.json", lf, registryProfile, []byte(`{"trusted":"root"}`),
	)
	assertStructuredCode(t, err, output.ErrMoatInvalid)
	if verifyCalled != 0 {
		t.Errorf("verify must not run when Rekor fetch fails; called %d times", verifyCalled)
	}
	if len(lf.Entries) != 0 {
		t.Errorf("lockfile must not be mutated when Rekor fetch fails; got %d entries", len(lf.Entries))
	}
}

func TestFetchAndRecord_Signed_VerifyFails(t *testing.T) {
	fixtureRoot := t.TempDir()
	entryHash := makeRepoFixture(t, fixtureRoot, "skills", "my-skill", map[string]string{
		"SKILL.md": "# hi\n",
	})

	rekorBytes := []byte(`{"abc123":{"body":"...","logIndex":1336116369}}`)
	withRekorStub(t, rekorBytes)
	verifyErr := errors.New("identity mismatch")
	withVerifyItemStub(t, moat.VerificationResult{}, verifyErr)

	stubCloneFromFixture(t, fixtureRoot)
	stubCloneScratchDir(t)
	stubSourceCacheDir(t)

	lf := &moat.Lockfile{}
	entry := signedFixture(fakeRepoURL, entryHash, false)
	registryProfile := &moat.SigningProfile{
		Issuer: "https://token.actions.githubusercontent.com", Subject: "https://github.com/example/repo/.github/workflows/registry.yml@refs/heads/main",
	}

	_, err := FetchAndRecord(
		context.Background(), entry, "example", "https://example.com/m",
		"/tmp/lf.json", lf, registryProfile, []byte(`{"trusted":"root"}`),
	)
	assertStructuredCode(t, err, output.ErrMoatInvalid)
	if len(lf.Entries) != 0 {
		t.Errorf("lockfile must not be mutated when verify fails; got %d entries", len(lf.Entries))
	}
}

func TestFetchAndRecord_Signed_RequiresProfile(t *testing.T) {
	idx := int64(42)
	entry := &moat.ContentEntry{
		Name:          "my-skill",
		Type:          "skill",
		ContentHash:   "sha256:" + strings.Repeat("cc", 32),
		SourceURI:     fakeRepoURL,
		RekorLogIndex: &idx,
	}
	_, err := FetchAndRecord(
		context.Background(), entry, "example", "https://example.com/m",
		"/tmp/lf.json", &moat.Lockfile{}, nil, []byte(`{"trusted":"root"}`),
	)
	assertStructuredCode(t, err, output.ErrMoatInvalid)
}

// TestFetchAndRecord_DualAttested_PublisherFetchFails covers the case
// where the registry leg succeeds but moat-attestation.json cannot be
// retrieved. The whole install must fail-closed — partial trust is not a
// valid Dual-Attested outcome.
func TestFetchAndRecord_DualAttested_PublisherFetchFails(t *testing.T) {
	fixtureRoot := t.TempDir()
	entryHash := makeRepoFixture(t, fixtureRoot, "skills", "my-skill", map[string]string{
		"SKILL.md": "# hi\n",
	})

	withRekorStub(t, []byte(`{"reg":{"logIndex":1336116369}}`))
	withPublisherAttestationStub(t, nil, errors.New("404 not found"))
	withVerifyItemStub(t, moat.VerificationResult{}, nil)

	stubCloneFromFixture(t, fixtureRoot)
	stubCloneScratchDir(t)
	stubSourceCacheDir(t)

	lf := &moat.Lockfile{}
	entry := signedFixture(fakeRepoURL, entryHash, true)
	registryProfile := &moat.SigningProfile{
		Issuer:  "https://token.actions.githubusercontent.com",
		Subject: "https://github.com/example/repo/.github/workflows/registry.yml@refs/heads/main",
	}

	_, err := FetchAndRecord(
		context.Background(), entry, "example", "https://example.com/m",
		"/tmp/lf.json", lf, registryProfile, []byte(`{"trusted":"root"}`),
	)
	assertStructuredCode(t, err, output.ErrMoatInvalid)
	var se output.StructuredError
	if !errors.As(err, &se) || !strings.Contains(se.Message, "publisher attestation") {
		t.Errorf("expected publisher-attestation message; got %+v", err)
	}
	if len(lf.Entries) != 0 {
		t.Errorf("lockfile must not be mutated when publisher leg fails; got %d entries", len(lf.Entries))
	}
}

// TestFetchAndRecord_DualAttested_PublisherEntryMissing covers the case
// where moat-attestation.json is fetched but contains no items[] entry
// matching the manifest's content_hash. Indicates the registry indexed
// content the publisher never attested.
func TestFetchAndRecord_DualAttested_PublisherEntryMissing(t *testing.T) {
	fixtureRoot := t.TempDir()
	entryHash := makeRepoFixture(t, fixtureRoot, "skills", "my-skill", map[string]string{
		"SKILL.md": "# hi\n",
	})

	withRekorStub(t, []byte(`{"reg":{"logIndex":1336116369}}`))
	// Publisher attestation lists a different content_hash → no match.
	withPublisherAttestationStub(t, []byte(`{"items":[{"name":"other","content_hash":"sha256:`+strings.Repeat("ff", 32)+`","rekor_log_index":99}]}`), nil)
	withVerifyItemStub(t, moat.VerificationResult{}, nil)

	stubCloneFromFixture(t, fixtureRoot)
	stubCloneScratchDir(t)
	stubSourceCacheDir(t)

	lf := &moat.Lockfile{}
	entry := signedFixture(fakeRepoURL, entryHash, true)
	registryProfile := &moat.SigningProfile{
		Issuer: "https://token.actions.githubusercontent.com", Subject: "https://github.com/example/repo/.github/workflows/registry.yml@refs/heads/main",
	}

	_, err := FetchAndRecord(
		context.Background(), entry, "example", "https://example.com/m",
		"/tmp/lf.json", lf, registryProfile, []byte(`{"trusted":"root"}`),
	)
	assertStructuredCode(t, err, output.ErrMoatInvalid)
	var se output.StructuredError
	if !errors.As(err, &se) || !strings.Contains(se.Message, "does not cover") {
		t.Errorf("expected 'does not cover' message; got %+v", err)
	}
	if len(lf.Entries) != 0 {
		t.Errorf("lockfile must not be mutated when publisher entry is missing; got %d entries", len(lf.Entries))
	}
}

// TestFetchAndRecord_DualAttested_PublisherVerifyFails covers the case
// where both Rekor entries fetch fine, but the publisher's separate Rekor
// entry's cert subject does not match the per-item signing_profile.
func TestFetchAndRecord_DualAttested_PublisherVerifyFails(t *testing.T) {
	fixtureRoot := t.TempDir()
	entryHash := makeRepoFixture(t, fixtureRoot, "skills", "my-skill", map[string]string{
		"SKILL.md": "# hi\n",
	})

	registryRekorIdx := int64(1336116369)
	publisherRekorIdx := int64(1336116370)
	withRekorPerIndexStub(t, map[int64][]byte{
		registryRekorIdx:  []byte(`{"reg":{"logIndex":1336116369}}`),
		publisherRekorIdx: []byte(`{"pub":{"logIndex":1336116370}}`),
	})
	withPublisherAttestationStub(t, []byte(fmt.Sprintf(
		`{"items":[{"name":"my-skill","content_hash":%q,"rekor_log_index":%d}]}`,
		entryHash, publisherRekorIdx,
	)), nil)

	// First call (registry leg) succeeds; second (publisher leg) fails.
	callCount := 0
	origVerify := VerifyItem
	VerifyItem = func(_ moat.AttestationItem, _ *moat.SigningProfile, _, _ []byte) (moat.VerificationResult, error) {
		callCount++
		if callCount == 2 {
			return moat.VerificationResult{}, errors.New("publisher cert subject mismatch")
		}
		return moat.VerificationResult{}, nil
	}
	t.Cleanup(func() { VerifyItem = origVerify })

	stubCloneFromFixture(t, fixtureRoot)
	stubCloneScratchDir(t)
	stubSourceCacheDir(t)

	lf := &moat.Lockfile{}
	entry := signedFixture(fakeRepoURL, entryHash, true)
	registryProfile := &moat.SigningProfile{
		Issuer: "https://token.actions.githubusercontent.com", Subject: "https://github.com/example/repo/.github/workflows/registry.yml@refs/heads/main",
	}

	_, err := FetchAndRecord(
		context.Background(), entry, "example", "https://example.com/m",
		"/tmp/lf.json", lf, registryProfile, []byte(`{"trusted":"root"}`),
	)
	assertStructuredCode(t, err, output.ErrMoatInvalid)
	var se output.StructuredError
	if !errors.As(err, &se) || !strings.Contains(se.Message, "publisher attestation verification failed") {
		t.Errorf("expected publisher-verify message; got %+v", err)
	}
	if callCount != 2 {
		t.Errorf("expected 2 verify calls (registry + publisher); got %d", callCount)
	}
	if len(lf.Entries) != 0 {
		t.Errorf("lockfile must not be mutated when publisher verify fails; got %d entries", len(lf.Entries))
	}
}

func TestFetchAndRecord_RefusesNonHTTPSScheme(t *testing.T) {
	entry := &moat.ContentEntry{
		Name:        "my-skill",
		Type:        "skill",
		ContentHash: "sha256:" + strings.Repeat("cc", 32),
		SourceURI:   "git+https://example.com/repo.git",
	}
	_, err := FetchAndRecord(context.Background(), entry, "example", "https://example.com/m", "/tmp/lockfile.json", &moat.Lockfile{}, nil, nil)
	assertStructuredCode(t, err, output.ErrMoatInvalid)
	var se output.StructuredError
	if !errors.As(err, &se) || !strings.Contains(se.Message, "source_uri scheme") {
		t.Errorf("expected source_uri scheme refusal; got %+v", err)
	}
}

func TestFetchAndRecord_RejectsUnsupportedType(t *testing.T) {
	entry := &moat.ContentEntry{
		Name:        "my-hook",
		Type:        "hook",
		ContentHash: "sha256:" + strings.Repeat("cc", 32),
		SourceURI:   fakeRepoURL,
	}
	_, err := FetchAndRecord(context.Background(), entry, "example", "https://example.com/m", "/tmp/lockfile.json", &moat.Lockfile{}, nil, nil)
	assertStructuredCode(t, err, output.ErrMoatInvalid)
	var se output.StructuredError
	if !errors.As(err, &se) || !strings.Contains(se.Message, "unsupported MOAT content type") {
		t.Errorf("expected unsupported-type refusal; got %+v", err)
	}
}

func TestFetchAndRecord_NilGuards(t *testing.T) {
	if _, err := FetchAndRecord(context.Background(), nil, "r", "u", "p", &moat.Lockfile{}, nil, nil); err == nil {
		t.Error("expected error on nil entry")
	}
	entry := &moat.ContentEntry{Name: "x", Type: "skill", ContentHash: "sha256:aa", SourceURI: fakeRepoURL}
	if _, err := FetchAndRecord(context.Background(), entry, "r", "u", "p", nil, nil, nil); err == nil {
		t.Error("expected error on nil lockfile")
	}
}

func TestFetchAndRecord_CloneFailure(t *testing.T) {
	stubCloneRepoErr(t, errors.New("git clone failed: repo not found"))
	stubCloneScratchDir(t)
	stubSourceCacheDir(t)

	entry := &moat.ContentEntry{
		Name:        "my-skill",
		Type:        "skill",
		ContentHash: "sha256:" + strings.Repeat("aa", 32),
		SourceURI:   fakeRepoURL,
	}
	_, err := FetchAndRecord(context.Background(), entry, "example", "https://example.com/m", "/tmp/lf.json", &moat.Lockfile{}, nil, nil)
	assertStructuredCode(t, err, output.ErrMoatInvalid)
	var se output.StructuredError
	if !errors.As(err, &se) || !strings.Contains(se.Message, "could not clone source repo") {
		t.Errorf("expected clone-failure message; got %+v", err)
	}
}

func TestFetchAndRecord_ItemNotFoundInRepo(t *testing.T) {
	// Repo exists but has no skills/missing-skill/ subdirectory.
	fixtureRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(fixtureRoot, "skills", "other-skill"), 0o755); err != nil {
		t.Fatal(err)
	}

	stubCloneFromFixture(t, fixtureRoot)
	stubCloneScratchDir(t)
	stubSourceCacheDir(t)

	entry := &moat.ContentEntry{
		Name:        "missing-skill",
		Type:        "skill",
		ContentHash: "sha256:" + strings.Repeat("aa", 32),
		SourceURI:   fakeRepoURL,
	}
	_, err := FetchAndRecord(context.Background(), entry, "example", "https://example.com/m", "/tmp/lf.json", &moat.Lockfile{}, nil, nil)
	assertStructuredCode(t, err, output.ErrMoatInvalid)
	var se output.StructuredError
	if !errors.As(err, &se) || !strings.Contains(se.Message, "item not found in source repo") {
		t.Errorf("expected item-not-found message; got %+v", err)
	}
}

// assertStructuredCode fails the test if err is nil or is not a structured
// error carrying the expected code.
func assertStructuredCode(t *testing.T, err error, wantCode string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %s, got nil", wantCode)
	}
	var se output.StructuredError
	if !errors.As(err, &se) {
		t.Fatalf("expected output.StructuredError, got %T: %v", err, err)
	}
	if se.Code != wantCode {
		t.Errorf("error code = %s; want %s (msg: %s)", se.Code, wantCode, se.Error())
	}
}
