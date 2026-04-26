package main

// Test helpers shared by install_moat_*_test.go after the fetch pipeline
// moved into cli/internal/moatinstall. The helpers used to live next to
// fetchAndRecord in install_moat_fetch_test.go; they migrated with the
// production code but the integration test still needs them inside this
// package to drive runInstallFromRegistry end-to-end.

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"net/http"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/moatinstall"
)

// withTLSClient swaps moatinstall.Client for one that trusts httptest's
// self-signed cert. Tests that exercise the Proceed path must serve over
// TLS because FetchAndRecord enforces https:// on SourceURI.
func withTLSClient(t *testing.T) func() {
	t.Helper()
	orig := moatinstall.Client
	moatinstall.Client = &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	return func() { moatinstall.Client = orig }
}

// buildTarGz produces an in-memory gzipped tar containing the given files.
// Used by integration tests that exercise fetch → extract end-to-end.
func buildTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name:     name,
			Mode:     0o644,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatalf("tar header: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("tar write: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}
