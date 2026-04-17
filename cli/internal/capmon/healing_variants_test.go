package capmon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGenerateVariants_CaseSwap(t *testing.T) {
	got := GenerateVariants("https://example.com/docs/Foo.md")
	if !containsVariantURL(got, "https://example.com/docs/foo.md") {
		t.Errorf("expected lowercase variant; got %v", got)
	}
}

func TestGenerateVariants_SeparatorSwap(t *testing.T) {
	got := GenerateVariants("https://example.com/docs/foo-bar.md")
	if !containsVariantURL(got, "https://example.com/docs/foo_bar.md") {
		t.Errorf("expected hyphen→underscore variant; got %v", got)
	}

	got = GenerateVariants("https://example.com/docs/foo_bar.md")
	if !containsVariantURL(got, "https://example.com/docs/foo-bar.md") {
		t.Errorf("expected underscore→hyphen variant; got %v", got)
	}
}

func TestGenerateVariants_PrefixSwap(t *testing.T) {
	got := GenerateVariants("https://example.com/docs/create-workflow.md")
	if !containsVariantURL(got, "https://example.com/docs/add-workflow.md") {
		t.Errorf("expected create→add variant; got %v", got)
	}
	if !containsVariantURL(got, "https://example.com/docs/new-workflow.md") {
		t.Errorf("expected create→new variant; got %v", got)
	}
	if !containsVariantURL(got, "https://example.com/docs/workflow.md") {
		t.Errorf("expected create- dropped variant; got %v", got)
	}
}

func TestGenerateVariants_IndexFallback(t *testing.T) {
	got := GenerateVariants("https://example.com/docs/settings.md")
	if !containsVariantURL(got, "https://example.com/docs/settings/index.md") {
		t.Errorf("expected index-file variant; got %v", got)
	}
}

func TestGenerateVariants_NoDuplicates(t *testing.T) {
	// A URL that would produce the same variant via multiple transforms —
	// e.g. a stem that is already all-lowercase shouldn't add a "lowercase"
	// duplicate.
	got := GenerateVariants("https://example.com/docs/foo.md")
	seen := map[string]bool{}
	for _, v := range got {
		if seen[v.URL] {
			t.Errorf("duplicate variant URL: %q", v.URL)
		}
		seen[v.URL] = true
	}
}

func TestGenerateVariants_ExcludesSelf(t *testing.T) {
	rawURL := "https://example.com/docs/foo.md"
	got := GenerateVariants(rawURL)
	for _, v := range got {
		if v.URL == rawURL {
			t.Errorf("original URL leaked into variants: %q", v.URL)
		}
	}
}

func TestGenerateVariants_CapAtMax(t *testing.T) {
	// A stem that triggers several transforms — verify total stays at cap.
	got := GenerateVariants("https://example.com/docs/Create_Foo-Bar.md")
	if len(got) > maxVariants {
		t.Errorf("got %d variants, want ≤ %d", len(got), maxVariants)
	}
}

func TestGenerateVariants_InvalidURL(t *testing.T) {
	got := GenerateVariants("://not a url")
	if got != nil {
		t.Errorf("expected nil for invalid URL; got %v", got)
	}
}

func TestProbeVariants_FirstSuccess(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/first.md", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/second.md", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cands := []VariantCandidate{
		{URL: srv.URL + "/first.md", Reason: "case swap"},
		{URL: srv.URL + "/second.md", Reason: "separator swap"},
	}
	got, ok, err := ProbeVariants(context.Background(), cands)
	if err != nil {
		t.Fatalf("ProbeVariants: %v", err)
	}
	if !ok {
		t.Fatal("expected a match")
	}
	if got.URL != srv.URL+"/second.md" {
		t.Errorf("matched %q, want %q", got.URL, srv.URL+"/second.md")
	}
}

func TestProbeVariants_AllFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	cands := []VariantCandidate{
		{URL: srv.URL + "/a.md"},
		{URL: srv.URL + "/b.md"},
	}
	_, ok, err := ProbeVariants(context.Background(), cands)
	if err != nil {
		t.Fatalf("ProbeVariants: %v", err)
	}
	if ok {
		t.Error("expected no match when all candidates 404")
	}
}

func TestProbeVariants_SkipsNetworkErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/ok.md") {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cands := []VariantCandidate{
		// This URL can't be reached (unresolvable hostname).
		{URL: "https://definitely-not-a-real-host.invalid/x.md"},
		{URL: srv.URL + "/ok.md"},
	}
	got, ok, err := ProbeVariants(context.Background(), cands)
	if err != nil {
		t.Fatalf("ProbeVariants: %v", err)
	}
	if !ok {
		t.Fatal("expected ok.md to match after skipping unreachable host")
	}
	if !strings.HasSuffix(got.URL, "/ok.md") {
		t.Errorf("matched URL = %q, want suffix /ok.md", got.URL)
	}
}

func containsVariantURL(vs []VariantCandidate, want string) bool {
	for _, v := range vs {
		if v.URL == want {
			return true
		}
	}
	return false
}
