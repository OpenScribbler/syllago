package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// writeTestSourceManifest creates a minimal provider-sources YAML with nSources entries
// under content_types.rules.sources in dir/<slug>.yaml.
func writeTestSourceManifest(t *testing.T, dir, slug string, nSources int) {
	t.Helper()
	var sb strings.Builder
	fmt.Fprintf(&sb, "schema_version: \"1\"\nslug: %s\ndisplay_name: %s\ncontent_types:\n  rules:\n    sources:\n", slug, slug)
	for i := 0; i < nSources; i++ {
		fmt.Fprintf(&sb, "      - url: https://example.com/%s/doc-%d\n        type: docs\n        format: markdown\n", slug, i)
	}
	if err := os.WriteFile(filepath.Join(dir, slug+".yaml"), []byte(sb.String()), 0644); err != nil {
		t.Fatalf("write test source manifest %s: %v", slug, err)
	}
}

// TestCapmonFetchCmd_DryRun_PrintsSourceCounts verifies that --dry-run reports
// the total source count per provider without writing any cache files.
func TestCapmonFetchCmd_DryRun_PrintsSourceCounts(t *testing.T) {
	srcDir := t.TempDir()
	cacheDir := t.TempDir()
	writeTestSourceManifest(t, srcDir, "alpha-provider", 3)

	stdout, _ := output.SetForTest(t)
	capmonFetchCmd.Flags().Set("dry-run", "true")
	capmonFetchCmd.Flags().Set("sources-dir", srcDir)
	capmonFetchCmd.Flags().Set("cache-root", cacheDir)
	defer func() {
		capmonFetchCmd.Flags().Set("dry-run", "false")
		capmonFetchCmd.Flags().Set("sources-dir", "")
		capmonFetchCmd.Flags().Set("cache-root", "")
	}()

	if err := capmonFetchCmd.RunE(capmonFetchCmd, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "alpha-provider") {
		t.Errorf("output missing provider slug %q; got:\n%s", "alpha-provider", out)
	}
	if !strings.Contains(out, "3") {
		t.Errorf("output missing source count 3; got:\n%s", out)
	}

	// Dry-run must not write any cache entries.
	entries, _ := os.ReadDir(cacheDir)
	if len(entries) != 0 {
		t.Errorf("dry-run wrote %d cache entries, want 0", len(entries))
	}
}

// TestCapmonFetchCmd_DryRun_ProviderFilter verifies that --provider restricts
// the dry-run report to the matched slug only.
func TestCapmonFetchCmd_DryRun_ProviderFilter(t *testing.T) {
	srcDir := t.TempDir()
	writeTestSourceManifest(t, srcDir, "alpha-provider", 2)
	writeTestSourceManifest(t, srcDir, "beta-provider", 5)

	stdout, _ := output.SetForTest(t)
	capmonFetchCmd.Flags().Set("dry-run", "true")
	capmonFetchCmd.Flags().Set("sources-dir", srcDir)
	capmonFetchCmd.Flags().Set("provider", "alpha-provider")
	defer func() {
		capmonFetchCmd.Flags().Set("dry-run", "false")
		capmonFetchCmd.Flags().Set("sources-dir", "")
		capmonFetchCmd.Flags().Set("provider", "")
	}()

	if err := capmonFetchCmd.RunE(capmonFetchCmd, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "alpha-provider") {
		t.Errorf("output missing alpha-provider; got:\n%s", out)
	}
	if strings.Contains(out, "beta-provider") {
		t.Errorf("filtered output must not mention beta-provider; got:\n%s", out)
	}
}

// TestCapmonFetchCmd_DryRun_UnknownProvider verifies that --provider set to a
// valid-format slug that has no manifest returns an error listing valid slugs.
func TestCapmonFetchCmd_DryRun_UnknownProvider(t *testing.T) {
	srcDir := t.TempDir()
	writeTestSourceManifest(t, srcDir, "alpha-provider", 1)

	_, _ = output.SetForTest(t)
	capmonFetchCmd.Flags().Set("dry-run", "true")
	capmonFetchCmd.Flags().Set("sources-dir", srcDir)
	capmonFetchCmd.Flags().Set("provider", "unknown-provider")
	defer func() {
		capmonFetchCmd.Flags().Set("dry-run", "false")
		capmonFetchCmd.Flags().Set("sources-dir", "")
		capmonFetchCmd.Flags().Set("provider", "")
	}()

	err := capmonFetchCmd.RunE(capmonFetchCmd, []string{})
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
	if !strings.Contains(err.Error(), "alpha-provider") {
		t.Errorf("error must list valid provider slugs so the user can self-correct; got: %v", err)
	}
}

// TestCapmonFetchCmd_DryRun_InvalidSlugFormat verifies that a --provider value
// with illegal characters surfaces "invalid --provider" (not a generic "not implemented").
func TestCapmonFetchCmd_DryRun_InvalidSlugFormat(t *testing.T) {
	_, _ = output.SetForTest(t)
	capmonFetchCmd.Flags().Set("provider", "INVALID SLUG")
	defer capmonFetchCmd.Flags().Set("provider", "")

	err := capmonFetchCmd.RunE(capmonFetchCmd, []string{})
	if err == nil {
		t.Fatal("expected error for invalid slug format, got nil")
	}
	if !strings.Contains(err.Error(), "invalid --provider") {
		t.Errorf("expected error to mention %q; got: %v", "invalid --provider", err)
	}
}

// TestCapmonFetchCmd_DryRun_JSON verifies that --dry-run with output.JSON=true
// emits valid JSON containing the provider slug and correct source count.
func TestCapmonFetchCmd_DryRun_JSON(t *testing.T) {
	srcDir := t.TempDir()
	writeTestSourceManifest(t, srcDir, "alpha-provider", 4)

	stdout, _ := output.SetForTest(t)
	output.JSON = true
	capmonFetchCmd.Flags().Set("dry-run", "true")
	capmonFetchCmd.Flags().Set("sources-dir", srcDir)
	defer func() {
		capmonFetchCmd.Flags().Set("dry-run", "false")
		capmonFetchCmd.Flags().Set("sources-dir", "")
	}()

	if err := capmonFetchCmd.RunE(capmonFetchCmd, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	out := strings.TrimSpace(stdout.String())
	var payload interface{}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "alpha-provider") {
		t.Errorf("JSON output missing provider slug; got:\n%s", out)
	}
	if !strings.Contains(out, "4") {
		t.Errorf("JSON output missing source count 4; got:\n%s", out)
	}
}
