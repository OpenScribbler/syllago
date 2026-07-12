package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const checkIndexJSON = `{
  "cadence": "daily",
  "data_revision": "rev-check-1",
  "files": {
    "by-content-type/skills.json": {"sha256": "aaa1"}
  },
  "generated_at": "2026-07-12T20:41:41Z",
  "max_staleness_hours": 48,
  "providers": [
    {"path": "capabilities/amp.json", "sha256": "bbb2", "slug": "amp", "status": "tracked"}
  ]
}`

func TestMain_CheckPrintsRevision(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(checkIndexJSON))
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{"-check", "-feed-url", srv.URL}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run -check exited %d; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "rev-check-1") {
		t.Errorf("stdout %q does not contain the data_revision rev-check-1", out)
	}
	if !strings.Contains(out, "2026-07-12T20:41:41Z") {
		t.Errorf("stdout %q does not contain the generated_at timestamp", out)
	}
}

func TestMain_CheckFailsOnMalformedIndex(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{
			name: "malformed JSON body",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`<html>not json</html>`))
			},
		},
		{
			name: "missing required fields",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"cadence": "daily"}`))
			},
		},
		{
			name: "http error status",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			var stdout, stderr bytes.Buffer
			code := run([]string{"-check", "-feed-url", srv.URL}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("run -check exited 0 with stdout %q; want non-zero", stdout.String())
			}
			if stderr.Len() == 0 {
				t.Error("stderr is empty; want an error message")
			}
		})
	}
}
