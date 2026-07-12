package capfeed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
)

// feedFixture builds an in-memory feed: content per path, the matching
// attested IndexFile list, and an httptest server that serves it.
func feedFixture(t *testing.T, contents map[string][]byte) (srv *httptest.Server, files []IndexFile) {
	t.Helper()
	for path, data := range contents {
		sum := sha256.Sum256(data)
		files = append(files, IndexFile{Path: path, SHA256: hex.EncodeToString(sum[:])})
	}
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, ok := contents[r.URL.Path[1:]]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv, files
}

func TestFetchFeedFiles_AllVerified(t *testing.T) {
	contents := map[string][]byte{
		"capabilities/amp.json":      []byte(`{"slug":"amp"}`),
		"by-content-type/hooks.json": []byte(`{"hooks":true}`),
		"spec/field-semantics.md":    []byte("# semantics\n"),
	}
	srv, files := feedFixture(t, contents)

	got, err := FetchFeedFiles(context.Background(), &Fetcher{}, srv.URL+"/", files)
	if err != nil {
		t.Fatalf("FetchFeedFiles: %v", err)
	}
	if len(got) != len(contents) {
		t.Fatalf("got %d files; want %d", len(got), len(contents))
	}
	for path, want := range contents {
		if string(got[path]) != string(want) {
			t.Errorf("file %q = %q; want byte-identical %q", path, got[path], want)
		}
	}
}

func TestFetchFeedFiles_HashMismatchAborts(t *testing.T) {
	paths := []string{"capabilities/amp.json", "by-content-type/hooks.json", "spec/field-semantics.md"}
	for _, corrupt := range paths {
		t.Run("corrupt "+corrupt, func(t *testing.T) {
			contents := map[string][]byte{
				"capabilities/amp.json":      []byte(`{"slug":"amp"}`),
				"by-content-type/hooks.json": []byte(`{"hooks":true}`),
				"spec/field-semantics.md":    []byte("# semantics\n"),
			}
			// Hashes computed over the pristine bytes...
			_, files := feedFixture(t, contents)
			// ...but the server delivers corrupted bytes for one path.
			served := map[string][]byte{}
			for k, v := range contents {
				served[k] = v
			}
			served[corrupt] = append([]byte(nil), contents[corrupt]...)
			served[corrupt][0] ^= 0x01
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				data, ok := served[r.URL.Path[1:]]
				if !ok {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				_, _ = w.Write(data)
			}))
			defer srv.Close()

			got, err := FetchFeedFiles(context.Background(), &Fetcher{}, srv.URL+"/", files)
			if err == nil {
				t.Fatalf("FetchFeedFiles returned %d files; want error on hash mismatch", len(got))
			}
			if got != nil {
				t.Errorf("FetchFeedFiles returned a partial map of %d files alongside the error; want nil (no partial results)", len(got))
			}
		})
	}
}

func TestFetchFeedFiles_MissingFileAborts(t *testing.T) {
	contents := map[string][]byte{
		"capabilities/amp.json": []byte(`{"slug":"amp"}`),
	}
	_, files := feedFixture(t, contents)
	files = append(files, IndexFile{Path: "capabilities/ghost.json", SHA256: "aa"})

	srv, _ := feedFixture(t, contents)
	got, err := FetchFeedFiles(context.Background(), &Fetcher{}, srv.URL+"/", files)
	if err == nil {
		t.Fatalf("FetchFeedFiles returned %d files; want error when a listed file 404s", len(got))
	}
}
