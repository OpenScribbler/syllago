package capmon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDetectGitHubRename_NotAGitHubURL(t *testing.T) {
	got, err := DetectGitHubRename(context.Background(), "https://example.com/docs/foo.md")
	if err != nil {
		t.Fatalf("DetectGitHubRename: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for non-github URL, got %v", got)
	}
}

func TestDetectGitHubRename_MalformedPath(t *testing.T) {
	// raw.githubusercontent.com with too-short path.
	_, err := DetectGitHubRename(context.Background(), "https://raw.githubusercontent.com/owner/repo")
	if err == nil {
		t.Fatal("expected error for malformed github raw path")
	}
}

func TestDetectGitHubRename_RenamedFile(t *testing.T) {
	// Simulate a realistic rename: docs/settings.md → docs/settings-strict.md.
	// The new name shares the old stem as a prefix — the most common pattern
	// for real-world doc renames (adding qualifiers or version suffixes).
	tree := gitTreeResponse{
		Tree: []gitTreeEntry{
			{Path: "README.md", Type: "blob"},
			{Path: "docs/settings-strict.md", Type: "blob"},
			{Path: "docs/unrelated.md", Type: "blob"},
			{Path: "src/code.go", Type: "blob"},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/git/trees/main" {
			t.Errorf("unexpected tree path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tree)
	}))
	defer srv.Close()

	SetGitHubBaseURLForTest(srv.URL)
	defer SetGitHubBaseURLForTest("")

	got, err := DetectGitHubRename(context.Background(), "https://raw.githubusercontent.com/owner/repo/main/docs/settings.md")
	if err != nil {
		t.Fatalf("DetectGitHubRename: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one candidate")
	}
	// Top candidate should be docs/settings-strict.md — shares stem "settings"
	// token + prefix match + same directory.
	if got[0].Path != "docs/settings-strict.md" {
		t.Errorf("top candidate = %q, want docs/settings-strict.md", got[0].Path)
	}
	wantURL := "https://raw.githubusercontent.com/owner/repo/main/docs/settings-strict.md"
	if got[0].URL != wantURL {
		t.Errorf("top candidate URL = %q, want %q", got[0].URL, wantURL)
	}
}

func TestDetectGitHubRename_ExtensionMismatch(t *testing.T) {
	// docs/foo.md is the original; repo has docs/foo.json with an identical
	// stem. We should NOT return it — ext mismatch.
	tree := gitTreeResponse{
		Tree: []gitTreeEntry{
			{Path: "docs/foo.json", Type: "blob"},
			{Path: "docs/unrelated.md", Type: "blob"},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tree)
	}))
	defer srv.Close()
	SetGitHubBaseURLForTest(srv.URL)
	defer SetGitHubBaseURLForTest("")

	got, _ := DetectGitHubRename(context.Background(), "https://raw.githubusercontent.com/owner/repo/main/docs/foo.md")
	for _, c := range got {
		if c.Path == "docs/foo.json" {
			t.Errorf("extension-mismatched candidate should be filtered: %v", c)
		}
	}
}

func TestDetectGitHubRename_NoCandidates(t *testing.T) {
	tree := gitTreeResponse{
		Tree: []gitTreeEntry{
			{Path: "completely/different.md", Type: "blob"},
			{Path: "zzz.md", Type: "blob"},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tree)
	}))
	defer srv.Close()
	SetGitHubBaseURLForTest(srv.URL)
	defer SetGitHubBaseURLForTest("")

	got, err := DetectGitHubRename(context.Background(), "https://raw.githubusercontent.com/owner/repo/main/docs/settings.md")
	if err != nil {
		t.Fatalf("DetectGitHubRename: %v", err)
	}
	// No candidate meets the score floor — got may be nil or empty. Neither
	// is an error.
	for _, c := range got {
		if c.Score >= 1.0 {
			t.Errorf("unexpected exact match: %v", c)
		}
	}
}

func TestDetectGitHubRename_TreeAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()
	SetGitHubBaseURLForTest(srv.URL)
	defer SetGitHubBaseURLForTest("")

	_, err := DetectGitHubRename(context.Background(), "https://raw.githubusercontent.com/owner/repo/main/docs/x.md")
	if err == nil {
		t.Fatal("expected error when tree API returns 404")
	}
}

func TestDetectGitHubRename_FiltersTreesAndCommits(t *testing.T) {
	// Only blob entries should be considered — "tree" and "commit" types
	// represent directories and submodules, not files.
	tree := gitTreeResponse{
		Tree: []gitTreeEntry{
			{Path: "docs", Type: "tree"},
			{Path: "docs/new-name.md", Type: "blob"},
			{Path: "external-submodule", Type: "commit"},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tree)
	}))
	defer srv.Close()
	SetGitHubBaseURLForTest(srv.URL)
	defer SetGitHubBaseURLForTest("")

	got, err := DetectGitHubRename(context.Background(), "https://raw.githubusercontent.com/owner/repo/main/docs/old-name.md")
	if err != nil {
		t.Fatalf("DetectGitHubRename: %v", err)
	}
	for _, c := range got {
		if c.Path == "docs" || c.Path == "external-submodule" {
			t.Errorf("non-blob entry leaked into candidates: %q", c.Path)
		}
	}
}

func TestStemSimilarity(t *testing.T) {
	tests := []struct {
		name   string
		a, b   string
		wantGt float64 // want score > this
		wantLt float64 // want score < this (0 means no upper bound)
	}{
		{"identical", "foo-bar", "foo-bar", 0.99, 0},
		{"token reorder", "create-workflow", "workflow-create", 0.6, 0},
		{"substring extension", "settings", "settings-v2", 0.4, 0},
		{"completely different", "alpha", "zebra", -0.01, 0.5},
		{"case insensitive", "FooBar", "foobar", 0.9, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stemSimilarity(tt.a, tt.b)
			if got <= tt.wantGt {
				t.Errorf("stemSimilarity(%q, %q) = %.3f, want > %.3f", tt.a, tt.b, got, tt.wantGt)
			}
			if tt.wantLt > 0 && got >= tt.wantLt {
				t.Errorf("stemSimilarity(%q, %q) = %.3f, want < %.3f", tt.a, tt.b, got, tt.wantLt)
			}
		})
	}
}

func TestTokenizeStem(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"foo-bar", []string{"foo", "bar"}},
		{"foo_bar", []string{"foo", "bar"}},
		{"foo.bar", []string{"foo", "bar"}},
		{"foo-bar_baz.qux", []string{"foo", "bar", "baz", "qux"}},
		{"plain", []string{"plain"}},
		{"", nil},
	}
	for _, tt := range tests {
		got := tokenizeStem(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("tokenizeStem(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("tokenizeStem(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}
