package moat

// Tests for the Sync orchestrator. The cryptographic verification step is
// injected via syncVerifyFn — the real VerifyManifest is exercised in
// manifest_verify_test.go against the Phase 0 fixture. Here we focus on:
//
//   - Programmer-error guards (nil registry, wrong type, missing URI)
//   - 304 NotModified short-circuit behavior
//   - TOFU classification (no pinned profile)
//   - ProfileChanged classification (pinned profile differs from wire)
//   - Matching-profile happy path (no classification flags)
//   - Transport-level fetch failures (bundle 404, manifest 5xx)
//   - Verification failures (VerifyError propagates unchanged)
//   - Staleness integration with manifest.Expires
//   - SyncRegistryRevocations side-effect (lockfile archival)
//
// Every test owns its own httptest.Server so the harness can simulate
// distinct manifest/bundle URL responses.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
)

// withStubVerifier swaps syncVerifyFn for the duration of a test and
// restores it on cleanup. The stub returns the caller-supplied result and
// error verbatim — tests use it to simulate both success (zero-value
// result, nil err) and crypto-failure paths.
func withStubVerifier(t *testing.T, res VerificationResult, verr error) {
	t.Helper()
	orig := syncVerifyFn
	syncVerifyFn = func(_ []byte, _ []byte, _ *SigningProfile, _ []byte) (VerificationResult, error) {
		return res, verr
	}
	t.Cleanup(func() { syncVerifyFn = orig })
}

// fixtureManifestJSON is a parseable minimal manifest with a stable
// RegistrySigningProfile used by most Sync tests. Kept distinct from
// manifest_test.go's minimalManifestJSON so tweaks here cannot ripple into
// the ParseManifest validation fixture set.
const fixtureManifestJSON = `{
  "schema_version": 1,
  "manifest_uri": "https://registry.example.com/manifest.json",
  "name": "Example MOAT Registry",
  "operator": "Example Operator",
  "updated_at": "2026-04-09T00:00:00Z",
  "registry_signing_profile": {
    "issuer": "https://token.actions.githubusercontent.com",
    "subject": "repo:example/registry:ref:refs/heads/main",
    "repository_id": "100",
    "repository_owner_id": "200"
  },
  "content": [],
  "revocations": []
}`

// manifestWithRevocations builds a parseable manifest that contains a
// mix of registry-source and publisher-source revocations plus private
// and public content entries. Used to assert SyncRegistryRevocations and
// PrivateContentCount integration in one pass.
func manifestWithRevocations(t *testing.T) string {
	t.Helper()
	tmpl := map[string]any{
		"schema_version": 1,
		"manifest_uri":   "https://registry.example.com/manifest.json",
		"name":           "Example",
		"operator":       "Example",
		"updated_at":     "2026-04-09T00:00:00Z",
		"registry_signing_profile": map[string]any{
			"issuer":  "https://token.actions.githubusercontent.com",
			"subject": "repo:example/registry:ref:refs/heads/main",
		},
		"content": []map[string]any{
			{
				"name": "public-tool", "display_name": "Public", "type": "skill",
				"content_hash": testHashA,
				"source_uri":   "https://github.com/example/public",
				"attested_at":  "2026-04-09T00:00:00Z",
				"private_repo": false,
			},
			{
				"name": "private-tool", "display_name": "Private", "type": "skill",
				"content_hash": testHashB,
				"source_uri":   "https://github.com/example/private",
				"attested_at":  "2026-04-09T00:00:00Z",
				"private_repo": true,
			},
		},
		"revocations": []map[string]any{
			{
				"content_hash": testHashC,
				"reason":       "malicious",
				"source":       "registry",
				"revoked_at":   "2026-04-08T00:00:00Z",
				"details_url":  "https://registry.example.com/advisories/testHashC",
			},
			{
				"content_hash": testHashD,
				"reason":       "deprecated",
				"source":       "publisher",
				"revoked_at":   "2026-04-08T00:00:00Z",
			},
		},
	}
	data, err := json.Marshal(tmpl)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	return string(data)
}

// newSyncHarness starts an httptest.Server that serves a manifest at
// /manifest.json and a bundle at /manifest.json.sigstore. Callers can
// override per-request behavior by passing custom handlers; the harness
// composes defaults for the common happy path.
//
// The harness also exposes server.Client() as h.client so callers can
// route fetches through a per-server *http.Transport. Without this, the
// zero-value Fetcher falls back to http.DefaultTransport — a process-wide
// singleton — and one test's t.Cleanup(server.Close) calls CloseIdleConnections
// on connections other parallel tests are mid-request on. See syllago-jm704.
type syncHarness struct {
	server       *httptest.Server
	client       *http.Client
	fetcher      *Fetcher
	manifestURL  string
	manifestHits int
	bundleHits   int
}

func newSyncHarness(t *testing.T, manifestJSON string, bundleBytes []byte, etag string) *syncHarness {
	t.Helper()
	h := &syncHarness{}
	mux := http.NewServeMux()
	mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		h.manifestHits++
		if etag != "" && r.Header.Get("If-None-Match") == etag {
			w.Header().Set("ETag", etag)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		if etag != "" {
			w.Header().Set("ETag", etag)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(manifestJSON))
	})
	mux.HandleFunc("/manifest.json.sigstore", func(w http.ResponseWriter, r *http.Request) {
		h.bundleHits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(bundleBytes)
	})
	h.server = httptest.NewServer(mux)
	h.client = h.server.Client()
	h.fetcher = &Fetcher{Client: h.client}
	h.manifestURL = h.server.URL + "/manifest.json"
	t.Cleanup(h.server.Close)
	return h
}

// isolatedFetcher returns a Fetcher whose Client uses srv's per-server
// Transport. Every TestSync_* call site that hits an httptest.Server must
// route through one of these — see newSyncHarness for the full rationale.
func isolatedFetcher(srv *httptest.Server) *Fetcher {
	return &Fetcher{Client: srv.Client()}
}

// moatRegistry constructs a minimal MOAT-typed config.Registry pointed at
// the given URL. pinned, if non-nil, populates SigningProfile so the sync
// takes the pinned path.
func moatRegistry(url string, pinned *config.SigningProfile) *config.Registry {
	return &config.Registry{
		Name:           "example",
		URL:            url,
		Type:           config.RegistryTypeMOAT,
		ManifestURI:    url,
		SigningProfile: pinned,
	}
}

func TestSync_NilRegistry(t *testing.T) {
	t.Parallel()
	lf := &Lockfile{}
	_, err := Sync(context.Background(), nil, lf, nil, nil, time.Now())
	if err == nil || !strings.Contains(err.Error(), "registry is nil") {
		t.Fatalf("expected nil-registry error, got %v", err)
	}
}

func TestSync_NilLockfile(t *testing.T) {
	t.Parallel()
	reg := moatRegistry("https://example.com/manifest.json", nil)
	_, err := Sync(context.Background(), reg, nil, nil, nil, time.Now())
	if err == nil || !strings.Contains(err.Error(), "lockfile is nil") {
		t.Fatalf("expected nil-lockfile error, got %v", err)
	}
}

func TestSync_GitRegistryRejected(t *testing.T) {
	t.Parallel()
	reg := &config.Registry{Name: "git-only", Type: config.RegistryTypeGit, URL: "https://example.com/repo.git"}
	lf := &Lockfile{}
	_, err := Sync(context.Background(), reg, lf, nil, nil, time.Now())
	if err == nil || !strings.Contains(err.Error(), "not MOAT") {
		t.Fatalf("expected non-MOAT error, got %v", err)
	}
}

func TestSync_EmptyManifestURI(t *testing.T) {
	t.Parallel()
	reg := &config.Registry{Name: "broken", Type: config.RegistryTypeMOAT}
	lf := &Lockfile{}
	_, err := Sync(context.Background(), reg, lf, nil, nil, time.Now())
	if err == nil || !strings.Contains(err.Error(), "manifest_uri is empty") {
		t.Fatalf("expected empty-manifest-uri error, got %v", err)
	}
}

func TestSync_NotModified_AdvancesFetchedAt(t *testing.T) {
	t.Parallel()
	const etag = `"v42"`
	h := newSyncHarness(t, fixtureManifestJSON, []byte("ignored"), etag)

	reg := moatRegistry(h.manifestURL, &config.SigningProfile{
		Issuer:  "https://token.actions.githubusercontent.com",
		Subject: "repo:example/registry:ref:refs/heads/main",
	})
	reg.ManifestETag = etag
	lf := &Lockfile{}

	// Pre-seed an old fetched_at so we can observe the bump.
	old := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	lf.SetRegistryFetchedAt(reg.ManifestURI, old)

	now := time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC)
	res, err := Sync(context.Background(), reg, lf, nil, h.fetcher, now)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if !res.NotModified {
		t.Error("expected NotModified=true on 304")
	}
	if res.Manifest != nil || res.ManifestBytes != nil {
		t.Error("Manifest and ManifestBytes should be nil on 304")
	}
	if res.ETag != etag {
		t.Errorf("ETag = %q; want %q", res.ETag, etag)
	}
	if h.bundleHits != 0 {
		t.Errorf("304 should not fetch the bundle; bundleHits=%d", h.bundleHits)
	}
	if state := lf.Registries[reg.ManifestURI]; state.FetchedAt.Equal(old) {
		t.Error("fetched_at should have advanced past the pre-seeded timestamp")
	}
}

func TestSync_HappyPath_ProfileMatches(t *testing.T) {
	// No t.Parallel — swaps package-level syncVerifyFn. See withStubVerifier.
	h := newSyncHarness(t, fixtureManifestJSON, []byte("bundle-bytes"), "")
	withStubVerifier(t, VerificationResult{
		SignatureValid:        true,
		CertificateChainValid: true,
		RekorProofValid:       true,
		IdentityMatches:       true,
		NumericIDMatched:      true,
	}, nil)

	pinned := &config.SigningProfile{
		Issuer:            "https://token.actions.githubusercontent.com",
		Subject:           "repo:example/registry:ref:refs/heads/main",
		RepositoryID:      "100",
		RepositoryOwnerID: "200",
	}
	reg := moatRegistry(h.manifestURL, pinned)
	lf := &Lockfile{}
	now := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)

	res, err := Sync(context.Background(), reg, lf, []byte("trusted-root-bytes"), h.fetcher, now)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.NotModified {
		t.Error("expected NotModified=false on 200")
	}
	if res.IsTOFU {
		t.Error("pinned profile matching wire profile → IsTOFU must be false")
	}
	if res.ProfileChanged {
		t.Error("pinned profile matching wire profile → ProfileChanged must be false")
	}
	if !res.Verification.SignatureValid {
		t.Error("Verification fields should be populated from the stub")
	}
	if res.Staleness != StalenessFresh {
		t.Errorf("Staleness = %v; want fresh (just fetched)", res.Staleness)
	}
	if h.manifestHits != 1 || h.bundleHits != 1 {
		t.Errorf("expected exactly one manifest and one bundle fetch; manifest=%d bundle=%d",
			h.manifestHits, h.bundleHits)
	}
	if state := lf.Registries[reg.ManifestURI]; state.FetchedAt.IsZero() {
		t.Error("fetched_at should be set on success")
	}
}

func TestSync_TOFU_NoPinnedProfile(t *testing.T) {
	// No t.Parallel — swaps package-level syncVerifyFn.
	h := newSyncHarness(t, fixtureManifestJSON, []byte("bundle"), "")
	withStubVerifier(t, VerificationResult{
		SignatureValid:        true,
		CertificateChainValid: true,
		RekorProofValid:       true,
		IdentityMatches:       true,
	}, nil)

	reg := moatRegistry(h.manifestURL, nil) // no pinned profile
	lf := &Lockfile{}
	now := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)

	res, err := Sync(context.Background(), reg, lf, []byte("tr"), h.fetcher, now)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if !res.IsTOFU {
		t.Error("no pinned profile → IsTOFU must be true")
	}
	if res.ProfileChanged {
		t.Error("no pinned profile → ProfileChanged must be false")
	}
	wantIssuer := "https://token.actions.githubusercontent.com"
	if res.IncomingProfile.Issuer != wantIssuer {
		t.Errorf("IncomingProfile.Issuer = %q; want %q", res.IncomingProfile.Issuer, wantIssuer)
	}
	if res.IncomingProfile.RepositoryID != "100" || res.IncomingProfile.RepositoryOwnerID != "200" {
		t.Errorf("IncomingProfile numeric IDs not populated: %+v", res.IncomingProfile)
	}
}

func TestSync_ProfileChanged(t *testing.T) {
	// No t.Parallel — swaps package-level syncVerifyFn.
	h := newSyncHarness(t, fixtureManifestJSON, []byte("bundle"), "")
	withStubVerifier(t, VerificationResult{
		SignatureValid:        true,
		CertificateChainValid: true,
		RekorProofValid:       true,
		IdentityMatches:       true,
	}, nil)

	// Pin a DIFFERENT subject than the wire manifest declares.
	pinned := &config.SigningProfile{
		Issuer:  "https://token.actions.githubusercontent.com",
		Subject: "repo:example/registry:ref:refs/heads/OLD-BRANCH",
	}
	reg := moatRegistry(h.manifestURL, pinned)
	lf := &Lockfile{}
	now := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)

	res, err := Sync(context.Background(), reg, lf, []byte("tr"), h.fetcher, now)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.IsTOFU {
		t.Error("pinned profile exists → IsTOFU must be false")
	}
	if !res.ProfileChanged {
		t.Error("wire subject differs from pinned → ProfileChanged must be true")
	}
}

func TestSync_BundleFetchFailure(t *testing.T) {
	t.Parallel()
	// Serve manifest but return 404 on bundle.
	mux := http.NewServeMux()
	mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(fixtureManifestJSON))
	})
	mux.HandleFunc("/manifest.json.sigstore", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	reg := moatRegistry(srv.URL+"/manifest.json", nil)
	lf := &Lockfile{}

	_, err := Sync(context.Background(), reg, lf, nil, isolatedFetcher(srv), time.Now())
	if err == nil || !strings.Contains(err.Error(), "fetch bundle") {
		t.Fatalf("expected fetch-bundle error, got %v", err)
	}
	// fetched_at must NOT advance on bundle failure — sync did not succeed.
	if _, ok := lf.Registries[reg.ManifestURI]; ok {
		t.Error("fetched_at must not be written when bundle fetch fails")
	}
}

func TestSync_ManifestFetch5xx(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	reg := moatRegistry(srv.URL+"/manifest.json", nil)
	lf := &Lockfile{}

	_, err := Sync(context.Background(), reg, lf, nil, isolatedFetcher(srv), time.Now())
	if err == nil || !strings.Contains(err.Error(), "fetch manifest") {
		t.Fatalf("expected fetch-manifest error, got %v", err)
	}
	if _, ok := lf.Registries[reg.ManifestURI]; ok {
		t.Error("fetched_at must not be written when manifest fetch fails")
	}
}

func TestSync_VerifyErrorPropagates(t *testing.T) {
	// No t.Parallel — swaps package-level syncVerifyFn.
	h := newSyncHarness(t, fixtureManifestJSON, []byte("bundle"), "")
	stubErr := verifyError(CodeIdentityMismatch, "cert SAN differs from pinned", nil)
	withStubVerifier(t, VerificationResult{}, stubErr)

	reg := moatRegistry(h.manifestURL, &config.SigningProfile{
		Issuer:  "https://token.actions.githubusercontent.com",
		Subject: "repo:example/registry:ref:refs/heads/main",
	})
	lf := &Lockfile{}

	_, err := Sync(context.Background(), reg, lf, []byte("tr"), h.fetcher, time.Now())
	var ve *VerifyError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *VerifyError, got %T: %v", err, err)
	}
	if ve.Code != CodeIdentityMismatch {
		t.Errorf("VerifyError code = %q; want %q", ve.Code, CodeIdentityMismatch)
	}
	if _, ok := lf.Registries[reg.ManifestURI]; ok {
		t.Error("fetched_at must not be written when verification fails")
	}
}

func TestSync_RevocationSyncAndPrivateCount(t *testing.T) {
	// No t.Parallel — swaps package-level syncVerifyFn.
	manifestJSON := manifestWithRevocations(t)
	h := newSyncHarness(t, manifestJSON, []byte("bundle"), "")
	withStubVerifier(t, VerificationResult{
		SignatureValid:        true,
		CertificateChainValid: true,
		RekorProofValid:       true,
		IdentityMatches:       true,
	}, nil)

	reg := moatRegistry(h.manifestURL, nil)
	lf := &Lockfile{}
	now := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)

	res, err := Sync(context.Background(), reg, lf, []byte("tr"), h.fetcher, now)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.RevocationsAdded != 1 {
		t.Errorf("RevocationsAdded = %d; want 1 (only registry-source)", res.RevocationsAdded)
	}
	if res.PrivateContentCount != 1 {
		t.Errorf("PrivateContentCount = %d; want 1", res.PrivateContentCount)
	}
	if !lf.IsRevoked(testHashC) {
		t.Error("registry-source revocation should be archived")
	}
	if lf.IsRevoked(testHashD) {
		t.Error("publisher-source revocation must NOT be archived")
	}
}

func TestSync_Staleness_RespectsManifestExpires(t *testing.T) {
	// No t.Parallel — swaps package-level syncVerifyFn.
	// Manifest with an `expires` already in the past relative to `now`.
	expired := map[string]any{
		"schema_version": 1,
		"manifest_uri":   "https://registry.example.com/manifest.json",
		"name":           "Example",
		"operator":       "Example",
		"updated_at":     "2026-04-09T00:00:00Z",
		"expires":        "2026-04-10T00:00:00Z",
		"registry_signing_profile": map[string]any{
			"issuer":  "https://token.actions.githubusercontent.com",
			"subject": "repo:example/registry:ref:refs/heads/main",
		},
		"content":     []any{},
		"revocations": []any{},
	}
	mb, err := json.Marshal(expired)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	h := newSyncHarness(t, string(mb), []byte("bundle"), "")
	withStubVerifier(t, VerificationResult{SignatureValid: true, CertificateChainValid: true, RekorProofValid: true, IdentityMatches: true}, nil)

	reg := moatRegistry(h.manifestURL, nil)
	lf := &Lockfile{}
	// now > expires → StalenessExpired
	now := time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC)

	res, err := Sync(context.Background(), reg, lf, []byte("tr"), h.fetcher, now)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Staleness != StalenessExpired {
		t.Errorf("Staleness = %v; want expired", res.Staleness)
	}
}

func TestSync_ContextCanceled(t *testing.T) {
	t.Parallel()
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	t.Cleanup(func() {
		close(block)
		srv.Close()
	})

	reg := moatRegistry(srv.URL+"/manifest.json", nil)
	lf := &Lockfile{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Sync(ctx, reg, lf, nil, isolatedFetcher(srv), time.Now())
	if err == nil {
		t.Fatal("expected context-canceled error")
	}
}

// TestSync_BundleURLIsManifestURIPlusSuffix locks the spec-defined URL
// convention (`{manifest_uri}.sigstore`) against silent drift. If a future
// refactor moved the bundle elsewhere, this test fails loudly — and so
// would every downstream publisher's workflow.
func TestSync_BundleURLIsManifestURIPlusSuffix(t *testing.T) {
	// No t.Parallel — swaps package-level syncVerifyFn.
	h := newSyncHarness(t, fixtureManifestJSON, []byte("bundle"), "")
	withStubVerifier(t, VerificationResult{SignatureValid: true, CertificateChainValid: true, RekorProofValid: true, IdentityMatches: true}, nil)

	reg := moatRegistry(h.manifestURL, nil)
	lf := &Lockfile{}
	res, err := Sync(context.Background(), reg, lf, []byte("tr"), h.fetcher, time.Now())
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	want := reg.ManifestURI + BundleURLSuffix
	if res.BundleURL != want {
		t.Errorf("BundleURL = %q; want %q", res.BundleURL, want)
	}
	// The harness recorded the bundle path; confirm the exact URL was hit.
	if h.bundleHits != 1 {
		t.Errorf("bundle endpoint should have been hit once; got %d", h.bundleHits)
	}
}

// TestSync_MalformedManifest verifies that a 200 response with garbage
// JSON surfaces as a fetch-manifest error (ParseManifest runs inside
// Fetcher.Fetch). Side-effect: fetched_at must not advance.
func TestSync_MalformedManifest(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{not json"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	reg := moatRegistry(srv.URL+"/manifest.json", nil)
	lf := &Lockfile{}
	_, err := Sync(context.Background(), reg, lf, nil, isolatedFetcher(srv), time.Now())
	if err == nil || !strings.Contains(err.Error(), "fetch manifest") {
		t.Fatalf("expected fetch-manifest parse error, got %v", err)
	}
}

// TestSync_OversizedBundle confirms the MaxManifestBytes cap applies to
// bundles too — a gigabyte of bundle is a malicious-or-broken registry
// and must be rejected before verification.
func TestSync_OversizedBundle(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(fixtureManifestJSON))
	})
	mux.HandleFunc("/manifest.json.sigstore", func(w http.ResponseWriter, r *http.Request) {
		// Write a stream larger than the cap — write in chunks so we don't
		// allocate 50 MiB in a single []byte.
		chunk := make([]byte, 1<<16)
		sent := 0
		for sent <= MaxManifestBytes {
			n, err := w.Write(chunk)
			if err != nil {
				return
			}
			sent += n
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	reg := moatRegistry(srv.URL+"/manifest.json", nil)
	lf := &Lockfile{}
	_, err := Sync(context.Background(), reg, lf, nil, isolatedFetcher(srv), time.Now())
	if err == nil || !strings.Contains(err.Error(), "cap") {
		t.Fatalf("expected bundle cap error, got %v", err)
	}
}

// sanity: make sure we didn't accidentally break the registry constructor
// helper and tests actually see a MOAT registry (back-compat guard).
func TestSync_moatRegistry_ProducesMoatType(t *testing.T) {
	t.Parallel()
	r := moatRegistry("https://example.com/manifest.json", nil)
	if !r.IsMOAT() {
		t.Fatalf("moatRegistry should produce a MOAT-typed registry, got type=%q", r.Type)
	}
}
