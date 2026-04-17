package capmon

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// validBody is a 600-byte stub that passes minContentBytes (512) and looks
// like plausible text content. Used as the success body in heal fixtures.
var validBody = []byte(strings.Repeat("content content content content content\n", 20))

func TestAttemptHeal_DisabledShortCircuits(t *testing.T) {
	enabled := false
	src := SourceEntry{
		URL:     "https://example.com/docs/foo.md",
		Healing: &HealingConfig{Enabled: &enabled},
	}
	result, err := AttemptHeal(context.Background(), src, errors.New("fetch 404"))
	if err != nil {
		t.Fatalf("AttemptHeal: %v", err)
	}
	if result.Success {
		t.Error("expected Success=false when healing disabled")
	}
	if !strings.Contains(result.FailReason, "disabled") {
		t.Errorf("FailReason = %q, want mention of disabled", result.FailReason)
	}
}

func TestAttemptHeal_VariantStrategySucceeds(t *testing.T) {
	// Tests the end-to-end path: variant produces candidates, candidate
	// passes fetch + ValidateContentResponse.
	mux := http.NewServeMux()
	mux.HandleFunc("/docs/foo_bar.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		_, _ = w.Write(validBody)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := SourceEntry{
		URL: srv.URL + "/docs/foo-bar.md",
		Healing: &HealingConfig{
			Strategies: []string{"variant"},
		},
	}
	result, err := AttemptHeal(context.Background(), src, errors.New("404"))
	if err != nil {
		t.Fatalf("AttemptHeal: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected Success, got fail: %s", result.FailReason)
	}
	if result.Strategy != "variant" {
		t.Errorf("Strategy = %q, want variant", result.Strategy)
	}
	if result.NewURL != srv.URL+"/docs/foo_bar.md" {
		t.Errorf("NewURL = %q, want %q", result.NewURL, srv.URL+"/docs/foo_bar.md")
	}
	if len(result.TriedURLs) == 0 {
		t.Error("TriedURLs should be populated")
	}
}

func TestAttemptHeal_VariantFailsContentValidation(t *testing.T) {
	// Variant resolves to a 200 with BINARY content-type — must be rejected.
	mux := http.NewServeMux()
	mux.HandleFunc("/docs/foo_bar.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(validBody)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := SourceEntry{
		URL: srv.URL + "/docs/foo-bar.md",
		Healing: &HealingConfig{
			Strategies: []string{"variant"},
		},
	}
	result, err := AttemptHeal(context.Background(), src, errors.New("404"))
	if err != nil {
		t.Fatalf("AttemptHeal: %v", err)
	}
	if result.Success {
		t.Error("expected failure when candidate content-type is binary")
	}
	if !strings.Contains(result.FailReason, "binary") && !strings.Contains(result.FailReason, "content invalid") {
		t.Errorf("FailReason = %q, want mention of content validation failure", result.FailReason)
	}
}

func TestAttemptHeal_VariantTooSmall(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/docs/foo_bar.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		_, _ = w.Write([]byte("short"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := SourceEntry{
		URL: srv.URL + "/docs/foo-bar.md",
		Healing: &HealingConfig{
			Strategies: []string{"variant"},
		},
	}
	result, _ := AttemptHeal(context.Background(), src, errors.New("404"))
	if result.Success {
		t.Error("expected failure when candidate body too small")
	}
}

func TestAttemptHeal_UnknownStrategyIgnored(t *testing.T) {
	src := SourceEntry{
		URL: "https://example.com/docs/foo.md",
		Healing: &HealingConfig{
			Strategies: []string{"telepathy"},
		},
	}
	result, err := AttemptHeal(context.Background(), src, errors.New("404"))
	if err != nil {
		t.Fatalf("AttemptHeal: %v", err)
	}
	if result.Success {
		t.Error("unknown strategy should not succeed")
	}
	if !strings.Contains(result.FailReason, "unknown strategy") {
		t.Errorf("FailReason = %q, want mention of unknown strategy", result.FailReason)
	}
}

func TestAttemptHeal_RedirectStrategyRejectsTemporary(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/final", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(validBody)
	})
	mux.HandleFunc("/old", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/final")
		w.WriteHeader(http.StatusFound) // 302
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := SourceEntry{
		URL: srv.URL + "/old",
		Healing: &HealingConfig{
			Strategies: []string{"redirect"},
		},
	}
	result, _ := AttemptHeal(context.Background(), src, errors.New("404"))
	if result.Success {
		t.Error("302 chain must not produce a successful heal")
	}
	if !strings.Contains(result.FailReason, "temporary") {
		t.Errorf("FailReason = %q, want mention of temporary", result.FailReason)
	}
}

func TestAttemptHeal_RedirectStrategySucceedsOnPermanent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/final", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(validBody)
	})
	mux.HandleFunc("/old", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/final")
		w.WriteHeader(http.StatusMovedPermanently)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := SourceEntry{
		URL: srv.URL + "/old",
		Healing: &HealingConfig{
			Strategies: []string{"redirect"},
		},
	}
	result, err := AttemptHeal(context.Background(), src, errors.New("404"))
	if err != nil {
		t.Fatalf("AttemptHeal: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success; FailReason=%q", result.FailReason)
	}
	if result.Strategy != "redirect" {
		t.Errorf("Strategy = %q, want redirect", result.Strategy)
	}
	if result.NewURL != srv.URL+"/final" {
		t.Errorf("NewURL = %q, want %q", result.NewURL, srv.URL+"/final")
	}
}

func TestAttemptHeal_StrategiesTriedInOrder(t *testing.T) {
	// Set up variant to succeed; redirect should be tried first and
	// (harmlessly) fail because there are no redirects — then variant
	// runs. We detect order by which strategy ends up in Result.Strategy.
	mux := http.NewServeMux()
	mux.HandleFunc("/docs/foo_bar.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		_, _ = w.Write(validBody)
	})
	mux.HandleFunc("/docs/foo-bar.md", func(w http.ResponseWriter, r *http.Request) {
		// Origin 404 — not a real fetch path for AttemptHeal; it only
		// matters that the redirect strategy finds no chain.
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := SourceEntry{
		URL: srv.URL + "/docs/foo-bar.md",
		Healing: &HealingConfig{
			Strategies: []string{"redirect", "variant"},
		},
	}
	result, err := AttemptHeal(context.Background(), src, errors.New("404"))
	if err != nil {
		t.Fatalf("AttemptHeal: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success via variant; FailReason=%q", result.FailReason)
	}
	if result.Strategy != "variant" {
		t.Errorf("Strategy = %q, want variant after redirect fails", result.Strategy)
	}
}
