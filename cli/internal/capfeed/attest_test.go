package capfeed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	in_toto "github.com/in-toto/attestation/go/v1"
	"github.com/sigstore/sigstore-go/pkg/verify"

	"github.com/OpenScribbler/syllago/cli/internal/moat"
)

// loadSnapshot returns the captured live feed snapshot: the exact
// v1/index.json bytes and their real Sigstore attestation bundle, recorded
// from the live feed + GitHub attestations API (see testdata/feedsnapshot).
func loadSnapshot(t *testing.T) (indexBytes, bundleBytes []byte) {
	t.Helper()
	var err error
	indexBytes, err = os.ReadFile(filepath.Join("testdata", "feedsnapshot", "index.json"))
	if err != nil {
		t.Fatalf("reading snapshot index: %v", err)
	}
	bundleBytes, err = os.ReadFile(filepath.Join("testdata", "feedsnapshot", "attestation-bundle.json"))
	if err != nil {
		t.Fatalf("reading snapshot bundle: %v", err)
	}
	return indexBytes, bundleBytes
}

func trustedRootBytes(t *testing.T) []byte {
	t.Helper()
	info := moat.BundledTrustedRoot(time.Now())
	if len(info.Bytes) == 0 {
		t.Fatal("bundled trusted root is empty")
	}
	return info.Bytes
}

func TestVerifyFeedProvenance_ValidSnapshot(t *testing.T) {
	indexBytes, bundleBytes := loadSnapshot(t)
	err := VerifyFeedProvenance(indexBytes, [][]byte{bundleBytes}, trustedRootBytes(t))
	if err != nil {
		t.Fatalf("VerifyFeedProvenance on the recorded live snapshot: %v", err)
	}
}

// TestCheckProvenancePredicate is the regression test for the predicate
// gate: a signature from the pinned workflow over the right digest is not
// enough — the verified statement must actually be SLSA provenance.
func TestCheckProvenancePredicate(t *testing.T) {
	tests := []struct {
		name    string
		res     *verify.VerificationResult
		wantErr bool
	}{
		{
			name: "slsa provenance v1 accepted",
			res: &verify.VerificationResult{Statement: &in_toto.Statement{
				PredicateType: "https://slsa.dev/provenance/v1",
			}},
		},
		{
			name: "other predicate type rejected",
			res: &verify.VerificationResult{Statement: &in_toto.Statement{
				PredicateType: "https://spdx.dev/Document",
			}},
			wantErr: true,
		},
		{
			name:    "missing statement rejected",
			res:     &verify.VerificationResult{},
			wantErr: true,
		},
		{
			name:    "nil result rejected",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkProvenancePredicate(tt.res)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkProvenancePredicate = %v; wantErr %t", err, tt.wantErr)
			}
		})
	}
}

func TestVerifyFeedProvenance_TamperedIndex(t *testing.T) {
	indexBytes, bundleBytes := loadSnapshot(t)
	root := trustedRootBytes(t)

	positions := []struct {
		name string
		pos  int
	}{
		{name: "first byte", pos: 0},
		{name: "middle byte", pos: len(indexBytes) / 2},
		{name: "last byte", pos: len(indexBytes) - 1},
	}
	for _, tt := range positions {
		t.Run(tt.name, func(t *testing.T) {
			tampered := append([]byte(nil), indexBytes...)
			tampered[tt.pos] ^= 0x01
			if err := VerifyFeedProvenance(tampered, [][]byte{bundleBytes}, root); err == nil {
				t.Fatal("tampered index verified successfully; want digest-mismatch error")
			}
		})
	}
}

func TestVerifyFeedProvenance_WrongSignerIdentity(t *testing.T) {
	indexBytes, bundleBytes := loadSnapshot(t)
	root := trustedRootBytes(t)

	tests := []struct {
		name    string
		subject string
		issuer  string
	}{
		{
			name:    "wrong repository in subject",
			subject: "https://github.com/evil-org/capmon/.github/workflows/publish.yml@refs/heads/main",
			issuer:  feedSignerIssuer,
		},
		{
			name:    "wrong workflow path in subject",
			subject: "https://github.com/OpenScribbler/capmon/.github/workflows/backdoor.yml@refs/heads/main",
			issuer:  feedSignerIssuer,
		},
		{
			name:    "wrong issuer",
			subject: feedSignerSubject,
			issuer:  "https://accounts.google.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := verifyWithIdentity(indexBytes, bundleBytes, root, tt.subject, tt.issuer)
			if err == nil {
				t.Fatal("verification with a mismatched pinned identity succeeded; want error")
			}
		})
	}

	// Sanity: the same seam with the real pinned identity verifies, proving
	// the failures above are identity failures and not fixture rot.
	if err := verifyWithIdentity(indexBytes, bundleBytes, root, feedSignerSubject, feedSignerIssuer); err != nil {
		t.Fatalf("verifyWithIdentity with the pinned identity: %v", err)
	}
}

func TestVerifyFeedProvenance_NoUsableBundle(t *testing.T) {
	indexBytes, _ := loadSnapshot(t)
	root := trustedRootBytes(t)

	tests := []struct {
		name    string
		bundles [][]byte
	}{
		{name: "no bundles at all", bundles: nil},
		{name: "empty slice", bundles: [][]byte{}},
		{name: "garbage bundle JSON", bundles: [][]byte{[]byte(`{"mediaType":"nope"}`)}},
		{name: "non-JSON bundle", bundles: [][]byte{[]byte("not json at all")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := VerifyFeedProvenance(indexBytes, tt.bundles, root); err == nil {
				t.Fatal("VerifyFeedProvenance succeeded without a usable bundle; want error")
			}
		})
	}
}

func TestFetchAttestationBundle_RequestShape(t *testing.T) {
	const digest = "06b783603fe451ac4485fc2cef9df8c38fa7e1a214d8689a3d22b38ebefdaac7"

	var gotPath, gotUA, gotAccept, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotUA = r.Header.Get("User-Agent")
		gotAccept = r.Header.Get("Accept")
		gotAuth = r.Header.Get("Authorization")
		resp := map[string]any{
			"attestations": []map[string]any{
				{"bundle": map[string]any{"mediaType": "application/vnd.dev.sigstore.bundle.v0.3+json"}},
				{"bundle": map[string]any{"mediaType": "application/vnd.dev.sigstore.bundle.v0.3+json"}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	restore := SetAttestationsAPIBaseURLForTest(srv.URL + "/attestations/")
	defer restore()

	bundles, err := FetchAttestationBundle(context.Background(), nil, digest)
	if err != nil {
		t.Fatalf("FetchAttestationBundle: %v", err)
	}
	if len(bundles) != 2 {
		t.Errorf("got %d bundle blobs; want 2", len(bundles))
	}
	for i, b := range bundles {
		if !json.Valid(b) {
			t.Errorf("bundle blob %d is not valid JSON: %q", i, b)
		}
	}
	if !strings.HasSuffix(gotPath, "sha256:"+digest) {
		t.Errorf("request path %q does not end with sha256:%s", gotPath, digest)
	}
	if gotUA == "" {
		t.Error("request sent no User-Agent; GitHub rejects those with 403")
	}
	if gotAccept != "application/vnd.github+json" {
		t.Errorf("Accept = %q; want application/vnd.github+json", gotAccept)
	}
	if gotAuth != "" {
		t.Errorf("request sent Authorization %q; the attestations fetch must be unauthenticated", gotAuth)
	}
}

func TestFetchAttestationBundle_Errors(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{
			name: "404 no attestation recorded",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
		},
		{
			name: "empty attestations list",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"attestations": []}`))
			},
		},
		{
			name: "malformed response body",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"attestations": [`))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			restore := SetAttestationsAPIBaseURLForTest(srv.URL + "/attestations/")
			defer restore()

			bundles, err := FetchAttestationBundle(context.Background(), nil, "deadbeef")
			if err == nil {
				t.Fatalf("FetchAttestationBundle returned %d bundles; want error", len(bundles))
			}
		})
	}
}
