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

// mockOnboardTransport is a minimal RoundTripper for onboard cmd tests.
type mockOnboardTransport struct{}

func (m *mockOnboardTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	body := strings.Repeat("x", 1024)
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"text/plain"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    r,
	}, nil
}

func TestCapmonOnboardCmd_Registered(t *testing.T) {
	t.Parallel()
	found := false
	for _, sub := range capmonCmd.Commands() {
		if sub.Use == "onboard" {
			found = true
			break
		}
	}
	if !found {
		t.Error("onboard subcommand not registered under capmonCmd")
	}
}

func TestCapmonOnboardCmd_MissingProvider(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	_ = stdout

	capmonOnboardCmd.Flags().Set("provider", "")
	defer capmonOnboardCmd.Flags().Set("provider", "")

	err := capmonOnboardCmd.RunE(capmonOnboardCmd, []string{})
	if err == nil {
		t.Fatal("expected error when --provider is not set")
	}
}

func TestCapmonOnboardCmd_InvalidProvider(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	_ = stdout

	capmonOnboardCmd.Flags().Set("provider", "INVALID SLUG")
	defer capmonOnboardCmd.Flags().Set("provider", "")

	err := capmonOnboardCmd.RunE(capmonOnboardCmd, []string{})
	if err == nil {
		t.Fatal("expected error for invalid provider slug")
	}
}

func TestCapmonOnboardCmd_DryRunFlag(t *testing.T) {
	t.Parallel()
	flag := capmonOnboardCmd.Flags().Lookup("dry-run")
	if flag == nil {
		t.Error("--dry-run flag not registered on capmon onboard command")
	}
}

func TestCapmonOnboardCmd_ValidatesSourcesDir(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	_ = stdout

	// Point sources dir override to a temp dir with no manifest.
	dir := t.TempDir()
	orig := capmonOnboardSourcesDirOverride
	capmonOnboardSourcesDirOverride = dir
	defer func() { capmonOnboardSourcesDirOverride = orig }()

	capmonOnboardCmd.Flags().Set("provider", "test-provider")
	defer capmonOnboardCmd.Flags().Set("provider", "")

	err := capmonOnboardCmd.RunE(capmonOnboardCmd, []string{})
	if err == nil {
		t.Fatal("expected error when source manifest is missing")
	}
}

func TestCapmonOnboardCmd_DryRunNoGHCalls(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	_ = stdout

	// Set up a minimal sources dir with a valid manifest.
	dir := t.TempDir()
	manifest := "schema_version: \"1\"\nslug: dry-provider\ncontent_types:\n  skills:\n    sources:\n      - url: https://example.com/skills.md\n        type: documentation\n        format: md\n        selector: {}\n"
	os.WriteFile(filepath.Join(dir, "dry-provider.yaml"), []byte(manifest), 0644)

	orig := capmonOnboardSourcesDirOverride
	capmonOnboardSourcesDirOverride = dir
	defer func() { capmonOnboardSourcesDirOverride = orig }()

	// Mock HTTP so FetchSource doesn't do real network calls.
	capmon.SetHTTPClientForTest(nil) // reset to default first; will be overridden below
	// Use a mock that returns a valid response so dry-run prints one line.
	// (fetch errors are non-blocking in OnboardProvider, so either way GH must not be called)
	capmon.SetHTTPClientForTest(&http.Client{Transport: &mockOnboardTransport{}})
	defer capmon.SetHTTPClientForTest(nil)

	// Mock GH to detect any calls.
	ghCalled := false
	capmon.SetGHCommandForTest(func(args ...string) ([]byte, error) {
		ghCalled = true
		return nil, nil
	})
	defer capmon.SetGHCommandForTest(nil)

	capmonOnboardCmd.Flags().Set("provider", "dry-provider")
	capmonOnboardCmd.Flags().Set("dry-run", "true")
	defer func() {
		capmonOnboardCmd.Flags().Set("provider", "")
		capmonOnboardCmd.Flags().Set("dry-run", "false")
	}()

	// Set context so cmd.Context() doesn't return nil.
	capmonOnboardCmd.SetContext(context.Background())

	// Run — fetch will fail (no real HTTP), but dry-run skips GH.
	capmonOnboardCmd.RunE(capmonOnboardCmd, []string{})

	if ghCalled {
		t.Error("dry-run: expected no gh calls")
	}
}
