package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// backfillMockTransport is a minimal http.RoundTripper that returns a fixed
// response for every request. Mirrors capmon_test.mockTransport but local to
// the main package since it can't be imported across test packages.
type backfillMockTransport struct {
	body []byte
}

func (m *backfillMockTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	hdr := http.Header{}
	hdr.Set("Content-Type", "text/markdown")
	return &http.Response{
		StatusCode: 200,
		Header:     hdr,
		Body:       io.NopCloser(strings.NewReader(string(m.body))),
		Request:    r,
	}, nil
}

func TestCapmonBackfillCmd_Registered(t *testing.T) {
	t.Parallel()
	found := false
	for _, sub := range capmonCmd.Commands() {
		if sub.Use == "backfill" {
			found = true
			break
		}
	}
	if !found {
		t.Error("backfill subcommand not registered under capmonCmd")
	}
}

func TestCapmonBackfillCmd_RequiresProvider(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	_ = stdout

	capmonBackfillCmd.Flags().Set("provider", "")
	defer capmonBackfillCmd.Flags().Set("provider", "")

	err := capmonBackfillCmd.RunE(capmonBackfillCmd, []string{})
	if err == nil {
		t.Fatal("expected error when --provider is not set")
	}
}

func TestCapmonBackfillCmd_HappyPath(t *testing.T) {
	stdout, _ := output.SetForTest(t)

	dir := t.TempDir()
	formatsDir := filepath.Join(dir, "formats")
	if err := os.MkdirAll(formatsDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	fixture := `provider: test-provider
docs_url: "https://example.com/docs"
category: cli
last_fetched_at: "2026-04-11T00:00:00Z"
generation_method: human-edited
content_types:
  skills:
    status: supported
    sources:
      - uri: "https://example.com/src.md"
        type: documentation
        fetch_method: md_url
        content_hash: ""
        fetched_at: ""
    canonical_mappings:
      display_name:
        supported: true
        mechanism: "yaml key: name"
        confidence: confirmed
    provider_extensions: []
`
	fixturePath := filepath.Join(formatsDir, "test-provider.yaml")
	if err := os.WriteFile(fixturePath, []byte(fixture), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	body := []byte("fresh markdown body")
	capmon.SetHTTPClientForTest(&http.Client{Transport: &backfillMockTransport{body: body}})
	t.Cleanup(func() { capmon.SetHTTPClientForTest(nil) })

	capmonBackfillCmd.Flags().Set("provider", "test-provider")
	capmonBackfillCmd.Flags().Set("formats-dir", formatsDir)
	capmonBackfillCmd.SetContext(context.Background())
	defer func() {
		capmonBackfillCmd.Flags().Set("provider", "")
		capmonBackfillCmd.Flags().Set("formats-dir", "docs/provider-formats")
	}()

	if err := capmonBackfillCmd.RunE(capmonBackfillCmd, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	// Assert FormatDoc now has the expected hash.
	doc, err := capmon.LoadFormatDoc(fixturePath)
	if err != nil {
		t.Fatalf("LoadFormatDoc: %v", err)
	}
	srcs := doc.ContentTypes["skills"].Sources
	expected := capmon.SHA256Hex(body)
	if srcs[0].ContentHash != expected {
		t.Errorf("ContentHash = %q, want %q", srcs[0].ContentHash, expected)
	}

	// Assert command printed a summary mentioning the count.
	out := stdout.String()
	if !strings.Contains(out, "1") {
		t.Errorf("expected output to mention '1 source backfilled'; got %q", out)
	}
}
