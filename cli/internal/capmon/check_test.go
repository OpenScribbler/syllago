package capmon_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// checkTestEnv sets up a minimal directory structure for RunCapmonCheck tests.
type checkTestEnv struct {
	Dir  string
	opts capmon.CapmonCheckOptions
}

func newCheckTestEnv(t *testing.T) *checkTestEnv {
	t.Helper()
	dir := t.TempDir()
	formatsDir := filepath.Join(dir, "formats")
	sourcesDir := filepath.Join(dir, "sources")
	os.MkdirAll(formatsDir, 0755)
	os.MkdirAll(sourcesDir, 0755)

	canonicalKeysPath := filepath.Join(dir, "canonical-keys.yaml")
	os.WriteFile(canonicalKeysPath, []byte(`content_types:
  skills:
    display_name:
      description: "Display name"
      type: string
`), 0644)

	providersJSON := filepath.Join(dir, "providers.json")

	return &checkTestEnv{
		Dir: dir,
		opts: capmon.CapmonCheckOptions{
			ProvidersJSON:     providersJSON,
			FormatsDir:        formatsDir,
			SourcesDir:        sourcesDir,
			CacheRoot:         filepath.Join(dir, "cache"),
			CanonicalKeysPath: canonicalKeysPath,
		},
	}
}

func (e *checkTestEnv) writeProviders(t *testing.T, slugs []string) {
	t.Helper()
	type entry struct {
		Slug string `json:"slug"`
	}
	type doc struct {
		Providers []entry `json:"providers"`
	}
	d := doc{}
	for _, s := range slugs {
		d.Providers = append(d.Providers, entry{Slug: s})
	}
	b, _ := json.Marshal(d)
	os.WriteFile(e.opts.ProvidersJSON, b, 0644)
}

func (e *checkTestEnv) writeSourceManifest(t *testing.T, provider string) {
	t.Helper()
	content := "schema_version: \"1\"\nslug: " + provider + "\ncontent_types: {}\n"
	os.WriteFile(filepath.Join(e.opts.SourcesDir, provider+".yaml"), []byte(content), 0644)
}

// writeFormatDoc writes a minimal format doc YAML that passes ValidateFormatDoc.
// contentHash is the stored hash for the single source; use "" for first-time.
func (e *checkTestEnv) writeFormatDoc(t *testing.T, provider, sourceURI, contentHash string) {
	t.Helper()
	content := `provider: ` + provider + `
docs_url: "https://example.com/docs"
category: cli
last_fetched_at: "2026-04-11T00:00:00Z"
generation_method: human-edited
content_types:
  skills:
    status: supported
    sources:
      - uri: "` + sourceURI + `"
        type: documentation
        fetch_method: md_url
        content_hash: "` + contentHash + `"
        fetched_at: "2026-04-11T00:00:00Z"
    canonical_mappings:
      display_name:
        supported: true
        mechanism: "yaml key: name"
        confidence: confirmed
    provider_extensions: []
`
	os.WriteFile(filepath.Join(e.opts.FormatsDir, provider+".yaml"), []byte(content), 0644)
}

func (e *checkTestEnv) setHTTPResponse(t *testing.T, body []byte, contentType string) {
	t.Helper()
	capmon.SetHTTPClientForTest(&http.Client{
		Transport: &mockTransport{body: body, contentType: contentType},
	})
	t.Cleanup(func() { capmon.SetHTTPClientForTest(nil) })
}

func (e *checkTestEnv) captureGHCalls(t *testing.T) *[][]string {
	t.Helper()
	calls := &[][]string{}
	capmon.SetGHCommandForTest(func(args ...string) ([]byte, error) {
		cp := make([]string, len(args))
		copy(cp, args)
		*calls = append(*calls, cp)
		// Return appropriate responses per command
		if len(args) >= 2 && args[0] == "issue" && args[1] == "list" {
			return []byte(`[]`), nil
		}
		if len(args) >= 2 && args[0] == "issue" && args[1] == "create" {
			return []byte("https://github.com/test/repo/issues/1\n"), nil
		}
		return []byte(""), nil
	})
	t.Cleanup(func() { capmon.SetGHCommandForTest(nil) })
	return calls
}

// mockTransport is a test RoundTripper that returns a fixed response.
type mockTransport struct {
	body        []byte
	contentType string
	err         error
}

func (m *mockTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	hdr := http.Header{}
	if m.contentType != "" {
		hdr.Set("Content-Type", m.contentType)
	} else {
		hdr.Set("Content-Type", "text/html")
	}
	return &http.Response{
		StatusCode: 200,
		Header:     hdr,
		Body:       io.NopCloser(strings.NewReader(string(m.body))),
		Request:    r,
	}, nil
}

func TestRunCapmonCheck_NoChange(t *testing.T) {
	env := newCheckTestEnv(t)

	testContent := []byte(strings.Repeat("x", 1000))
	expectedHash := capmon.SHA256Hex(testContent)

	env.writeProviders(t, []string{"test-provider"})
	env.writeSourceManifest(t, "test-provider")
	env.writeFormatDoc(t, "test-provider", "https://example.com/skills.md", expectedHash)
	env.setHTTPResponse(t, testContent, "text/html")
	calls := env.captureGHCalls(t)

	err := capmon.RunCapmonCheck(context.Background(), env.opts)
	if err != nil {
		t.Fatalf("RunCapmonCheck: %v", err)
	}
	// No GitHub calls expected when hash matches.
	if len(*calls) != 0 {
		t.Errorf("expected 0 gh calls for no-change, got %d: %v", len(*calls), *calls)
	}
}

func TestRunCapmonCheck_Changed(t *testing.T) {
	env := newCheckTestEnv(t)

	testContent := []byte(strings.Repeat("y", 1000))

	env.writeProviders(t, []string{"test-provider"})
	env.writeSourceManifest(t, "test-provider")
	// Store a different (stale) hash in the format doc.
	env.writeFormatDoc(t, "test-provider", "https://example.com/skills.md", "sha256:stale_hash_not_matching")
	env.setHTTPResponse(t, testContent, "text/html")
	calls := env.captureGHCalls(t)

	err := capmon.RunCapmonCheck(context.Background(), env.opts)
	if err != nil {
		t.Fatalf("RunCapmonCheck: %v", err)
	}
	// Expect at least one gh call (issue list + issue create).
	if len(*calls) == 0 {
		t.Error("expected gh calls for changed content, got none")
	}
}

func TestRunCapmonCheck_FetchError(t *testing.T) {
	env := newCheckTestEnv(t)

	env.writeProviders(t, []string{"test-provider"})
	env.writeSourceManifest(t, "test-provider")
	env.writeFormatDoc(t, "test-provider", "https://example.com/skills.md", "")
	// HTTP returns an error.
	capmon.SetHTTPClientForTest(&http.Client{
		Transport: &mockTransport{err: errors.New("connection refused")},
	})
	t.Cleanup(func() { capmon.SetHTTPClientForTest(nil) })
	calls := env.captureGHCalls(t)

	err := capmon.RunCapmonCheck(context.Background(), env.opts)
	if err != nil {
		t.Fatalf("RunCapmonCheck: %v (fetch errors should be non-blocking)", err)
	}
	// Should create a fetch-error issue.
	hasFetchErr := false
	for _, c := range *calls {
		for _, a := range c {
			if strings.Contains(a, "capmon-fetch-error") {
				hasFetchErr = true
			}
		}
	}
	if !hasFetchErr {
		t.Errorf("expected gh call with capmon-fetch-error label, got: %v", *calls)
	}
}

func TestRunCapmonCheck_ContentValidityFailure(t *testing.T) {
	env := newCheckTestEnv(t)

	// Body too small — should trigger fetch-error.
	tinyContent := []byte("tiny")

	env.writeProviders(t, []string{"test-provider"})
	env.writeSourceManifest(t, "test-provider")
	env.writeFormatDoc(t, "test-provider", "https://example.com/skills.md", "")
	env.setHTTPResponse(t, tinyContent, "text/html")
	calls := env.captureGHCalls(t)

	err := capmon.RunCapmonCheck(context.Background(), env.opts)
	if err != nil {
		t.Fatalf("RunCapmonCheck: %v", err)
	}
	hasFetchErr := false
	for _, c := range *calls {
		for _, a := range c {
			if strings.Contains(a, "capmon-fetch-error") {
				hasFetchErr = true
			}
		}
	}
	if !hasFetchErr {
		t.Errorf("expected gh call with capmon-fetch-error label for tiny body, got: %v", *calls)
	}
}

func TestRunCapmonCheck_OrphanDetection(t *testing.T) {
	env := newCheckTestEnv(t)

	testContent := []byte(strings.Repeat("z", 1000))
	expectedHash := capmon.SHA256Hex(testContent)

	// providers.json does NOT include "orphan-provider".
	env.writeProviders(t, []string{})
	env.writeSourceManifest(t, "orphan-provider")
	env.writeFormatDoc(t, "orphan-provider", "https://example.com/skills.md", expectedHash)
	env.setHTTPResponse(t, testContent, "text/html")
	env.captureGHCalls(t)

	// Capture stderr to verify warning.
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() { os.Stderr = oldStderr }()

	err := capmon.RunCapmonCheck(context.Background(), env.opts)
	w.Close()
	stderrOut, _ := io.ReadAll(r)

	if err != nil {
		t.Fatalf("RunCapmonCheck: %v (orphan should be non-blocking)", err)
	}
	if !strings.Contains(string(stderrOut), "orphan-provider") {
		t.Errorf("expected orphan warning mentioning 'orphan-provider', got: %q", string(stderrOut))
	}
}

// TestRunCapmonCheck_FormatDocWarningLoggedToStderr verifies that non-blocking
// allow-list warnings from ValidateFormatDocWithWarnings surface on stderr with
// the DeduplicationKey and field path, and that the pipeline continues normally.
func TestRunCapmonCheck_FormatDocWarningLoggedToStderr(t *testing.T) {
	env := newCheckTestEnv(t)

	// Benign content so Step 3 finds matching hash and skips issue creation.
	testContent := []byte(strings.Repeat("a", 1000))
	expectedHash := capmon.SHA256Hex(testContent)

	env.writeProviders(t, []string{"test-provider"})
	env.writeSourceManifest(t, "test-provider")

	// Format doc passes blocking validation but carries a non-allow-listed
	// value_type, which ValidateFormatDocWithWarnings returns as a warning.
	docWithWarning := `provider: test-provider
docs_url: "https://example.com/docs"
category: cli
last_fetched_at: "2026-04-15T00:00:00Z"
content_types:
  skills:
    status: supported
    sources:
      - uri: "https://example.com/skills.md"
        type: documentation
        fetch_method: md_url
        content_hash: "` + expectedHash + `"
        fetched_at: "2026-04-15T00:00:00Z"
    canonical_mappings:
      display_name:
        supported: true
        mechanism: "yaml key: name"
        confidence: confirmed
    provider_extensions:
      - id: bad_type_ext
        name: "Bad Type Ext"
        summary: "test extension with non-allow-listed value_type"
        source_ref: "https://example.com"
        conversion: embedded
        value_type: "not-in-allow-list"
`
	if err := os.WriteFile(filepath.Join(env.opts.FormatsDir, "test-provider.yaml"), []byte(docWithWarning), 0644); err != nil {
		t.Fatal(err)
	}
	env.setHTTPResponse(t, testContent, "text/html")
	env.captureGHCalls(t)

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	defer func() { os.Stderr = oldStderr }()

	err := capmon.RunCapmonCheck(context.Background(), env.opts)
	w.Close()
	stderrOut, _ := io.ReadAll(r)

	if err != nil {
		t.Fatalf("RunCapmonCheck: %v (allow-list warnings must be non-blocking)", err)
	}
	s := string(stderrOut)
	if !strings.Contains(s, "value_type") {
		t.Errorf("expected stderr warning to mention field path 'value_type', got: %q", s)
	}
	if !strings.Contains(s, "not-in-allow-list") {
		t.Errorf("expected stderr warning to quote the offending value, got: %q", s)
	}
	if !strings.Contains(s, "test-provider") {
		t.Errorf("expected stderr warning to name the provider, got: %q", s)
	}
}

// TestRunCapmonCheck_CIModeCreatesWarningIssues verifies that when GITHUB_TOKEN
// is set, validation warnings are routed to GitHub issues (not just stderr).
func TestRunCapmonCheck_CIModeCreatesWarningIssues(t *testing.T) {
	env := newCheckTestEnv(t)

	testContent := []byte(strings.Repeat("a", 1000))
	expectedHash := capmon.SHA256Hex(testContent)

	env.writeProviders(t, []string{"test-provider"})
	env.writeSourceManifest(t, "test-provider")

	// Format doc with a non-allow-listed value_type to trigger a warning.
	docWithWarning := `provider: test-provider
docs_url: "https://example.com/docs"
category: cli
last_fetched_at: "2026-04-15T00:00:00Z"
content_types:
  skills:
    status: supported
    sources:
      - uri: "https://example.com/skills.md"
        type: documentation
        fetch_method: md_url
        content_hash: "` + expectedHash + `"
        fetched_at: "2026-04-15T00:00:00Z"
    canonical_mappings:
      display_name:
        supported: true
        mechanism: "yaml key: name"
        confidence: confirmed
    provider_extensions:
      - id: bad_ext
        name: "Bad Ext"
        summary: "extension with invalid value_type"
        source_ref: "https://example.com"
        conversion: embedded
        value_type: "not-in-allow-list"
`
	os.WriteFile(filepath.Join(env.opts.FormatsDir, "test-provider.yaml"), []byte(docWithWarning), 0644)
	env.setHTTPResponse(t, testContent, "text/html")

	// Set GITHUB_TOKEN to activate CI mode.
	t.Setenv("GITHUB_TOKEN", "ghp_test_token")

	var issueCreateCalled bool
	var issueListForWarnCalled bool
	var closeListCalled bool
	capmon.SetGHCommandForTest(func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "issue" && args[1] == "list" {
			for _, a := range args {
				if a == "capmon-warn" {
					issueListForWarnCalled = true
				}
			}
			return []byte(`[]`), nil
		}
		if len(args) >= 2 && args[0] == "issue" && args[1] == "create" {
			for _, a := range args {
				if a == "capmon-warn" {
					issueCreateCalled = true
				}
			}
			return []byte("https://github.com/test/repo/issues/88\n"), nil
		}
		if len(args) >= 2 && args[0] == "issue" && args[1] == "close" {
			closeListCalled = true
			return []byte(""), nil
		}
		return []byte(""), nil
	})
	t.Cleanup(func() { capmon.SetGHCommandForTest(nil) })

	err := capmon.RunCapmonCheck(context.Background(), env.opts)
	if err != nil {
		t.Fatalf("RunCapmonCheck: %v", err)
	}
	if !issueListForWarnCalled {
		t.Error("expected gh issue list call with capmon-warn label")
	}
	if !issueCreateCalled {
		t.Error("expected gh issue create call with capmon-warn label (new warning)")
	}
	// Close list is called even though nothing needs closing — it queries then skips.
	_ = closeListCalled
}

// TestRunCapmonCheck_CIModeDryRunSkipsIssues verifies that dry-run mode does NOT
// create warning issues even when GITHUB_TOKEN is set.
func TestRunCapmonCheck_CIModeDryRunSkipsIssues(t *testing.T) {
	env := newCheckTestEnv(t)

	testContent := []byte(strings.Repeat("b", 1000))
	expectedHash := capmon.SHA256Hex(testContent)

	env.writeProviders(t, []string{"test-provider"})
	env.writeSourceManifest(t, "test-provider")

	docWithWarning := `provider: test-provider
docs_url: "https://example.com/docs"
category: cli
last_fetched_at: "2026-04-15T00:00:00Z"
content_types:
  skills:
    status: supported
    sources:
      - uri: "https://example.com/skills.md"
        type: documentation
        fetch_method: md_url
        content_hash: "` + expectedHash + `"
        fetched_at: "2026-04-15T00:00:00Z"
    canonical_mappings:
      display_name:
        supported: true
        mechanism: "yaml key: name"
        confidence: confirmed
    provider_extensions:
      - id: bad_ext
        name: "Bad Ext"
        summary: "extension with invalid value_type"
        source_ref: "https://example.com"
        conversion: embedded
        value_type: "not-in-allow-list"
`
	os.WriteFile(filepath.Join(env.opts.FormatsDir, "test-provider.yaml"), []byte(docWithWarning), 0644)
	env.setHTTPResponse(t, testContent, "text/html")

	t.Setenv("GITHUB_TOKEN", "ghp_test_token")

	ghCalled := false
	capmon.SetGHCommandForTest(func(args ...string) ([]byte, error) {
		ghCalled = true
		return nil, nil
	})
	t.Cleanup(func() { capmon.SetGHCommandForTest(nil) })

	opts := env.opts
	opts.DryRun = true
	err := capmon.RunCapmonCheck(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunCapmonCheck dry-run: %v", err)
	}
	if ghCalled {
		t.Error("dry-run + CI mode: expected no gh calls for warnings")
	}
}

func TestRunCapmonCheck_DryRun(t *testing.T) {
	env := newCheckTestEnv(t)

	testContent := []byte(strings.Repeat("w", 1000))
	// Different hash → would normally create issue.
	env.writeProviders(t, []string{"test-provider"})
	env.writeSourceManifest(t, "test-provider")
	env.writeFormatDoc(t, "test-provider", "https://example.com/skills.md", "sha256:old_hash")
	env.setHTTPResponse(t, testContent, "text/html")

	ghCalled := false
	capmon.SetGHCommandForTest(func(args ...string) ([]byte, error) {
		ghCalled = true
		return nil, nil
	})
	t.Cleanup(func() { capmon.SetGHCommandForTest(nil) })

	opts := env.opts
	opts.DryRun = true
	err := capmon.RunCapmonCheck(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunCapmonCheck dry-run: %v", err)
	}
	if ghCalled {
		t.Error("dry-run: expected no gh calls")
	}
}
