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
	if len(result.CandidateOutcomes) == 0 {
		t.Error("CandidateOutcomes should be populated")
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

func TestAttemptHeal_VariantRejectsRedirectedCandidate(t *testing.T) {
	// Regression for syllago-qc7yf / ampcode #92: variant generates
	// /manual/hooks/index.md, which 302-redirects to /auth/sign-in on the
	// same host. After Impl-3, ANY redirect on a heal candidate is drift,
	// regardless of where it lands or what the final body looks like.
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/sign-in", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(validBody)
	})
	mux.HandleFunc("/docs/foo_bar.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/auth/sign-in?returnTo="+r.URL.Path)
		w.WriteHeader(http.StatusFound) // 302
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
		t.Fatalf("expected rejection; got Success with NewURL=%q", result.NewURL)
	}
	// Find the candidate that hit the 302.
	var redirected *CandidateOutcome
	for i := range result.CandidateOutcomes {
		o := &result.CandidateOutcomes[i]
		if o.URL == srv.URL+"/docs/foo_bar.md" {
			redirected = o
			break
		}
	}
	if redirected == nil {
		t.Fatalf("expected /docs/foo_bar.md outcome in CandidateOutcomes; got %+v", result.CandidateOutcomes)
	}
	if redirected.Outcome != OutcomeRedirected {
		t.Errorf("Outcome = %q, want %q", redirected.Outcome, OutcomeRedirected)
	}
	if len(redirected.Redirects) != 1 {
		t.Fatalf("Redirects = %v, want 1 hop", redirected.Redirects)
	}
	if redirected.Redirects[0].Status != 302 {
		t.Errorf("Redirects[0].Status = %d, want 302", redirected.Redirects[0].Status)
	}
	if !strings.HasSuffix(redirected.FinalURL, "/auth/sign-in") && !strings.Contains(redirected.FinalURL, "/auth/sign-in?") {
		t.Errorf("FinalURL = %q, want end with /auth/sign-in", redirected.FinalURL)
	}
}

func TestAttemptHeal_VariantMultiHopRedirectChainCaptured(t *testing.T) {
	// Multi-hop chain: variant -> 301 -> 302 -> 200. We capture every hop
	// and reject regardless of whether the final destination is a 200.
	mux := http.NewServeMux()
	mux.HandleFunc("/docs/foo_bar.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/middle.md")
		w.WriteHeader(http.StatusMovedPermanently) // 301
	})
	mux.HandleFunc("/middle.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/final.md")
		w.WriteHeader(http.StatusFound) // 302
	})
	mux.HandleFunc("/final.md", func(w http.ResponseWriter, r *http.Request) {
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
	if result.Success {
		t.Fatalf("expected rejection of multi-hop chain")
	}
	var redirected *CandidateOutcome
	for i := range result.CandidateOutcomes {
		o := &result.CandidateOutcomes[i]
		if o.Outcome == OutcomeRedirected {
			redirected = o
			break
		}
	}
	if redirected == nil {
		t.Fatalf("expected at least one redirected outcome; got %+v", result.CandidateOutcomes)
	}
	if len(redirected.Redirects) != 2 {
		t.Fatalf("Redirects = %v, want 2 hops", redirected.Redirects)
	}
	if redirected.Redirects[0].Status != 301 {
		t.Errorf("Hop 0 status = %d, want 301", redirected.Redirects[0].Status)
	}
	if redirected.Redirects[1].Status != 302 {
		t.Errorf("Hop 1 status = %d, want 302", redirected.Redirects[1].Status)
	}
	if !strings.HasSuffix(redirected.FinalURL, "/final.md") {
		t.Errorf("FinalURL = %q, want end with /final.md", redirected.FinalURL)
	}
}

func TestAttemptHeal_VariantHTTPError(t *testing.T) {
	// 404 produces a clean http_error outcome with no Redirects, no body.
	mux := http.NewServeMux()
	mux.HandleFunc("/docs/foo_bar.md", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
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
		t.Fatalf("expected failure on 404")
	}
	var notFound *CandidateOutcome
	for i := range result.CandidateOutcomes {
		o := &result.CandidateOutcomes[i]
		if o.URL == srv.URL+"/docs/foo_bar.md" {
			notFound = o
			break
		}
	}
	if notFound == nil {
		t.Fatalf("expected /docs/foo_bar.md outcome; got %+v", result.CandidateOutcomes)
	}
	if notFound.Outcome != OutcomeHTTPError {
		t.Errorf("Outcome = %q, want %q", notFound.Outcome, OutcomeHTTPError)
	}
	if notFound.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want 404", notFound.StatusCode)
	}
	if len(notFound.Redirects) != 0 {
		t.Errorf("Redirects = %v, want empty", notFound.Redirects)
	}
}

func TestAttemptHeal_VariantConnectError(t *testing.T) {
	// Closed server -> connection refused -> connect_error outcome.
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	closedURL := srv.URL
	srv.Close() // immediate close so the URL refuses connections

	src := SourceEntry{
		URL: closedURL + "/docs/foo-bar.md",
		Healing: &HealingConfig{
			Strategies: []string{"variant"},
		},
	}
	result, err := AttemptHeal(context.Background(), src, errors.New("404"))
	if err != nil {
		t.Fatalf("AttemptHeal: %v", err)
	}
	if result.Success {
		t.Fatalf("expected failure on closed server")
	}
	if len(result.CandidateOutcomes) == 0 {
		t.Fatalf("expected at least one CandidateOutcome with connect_error")
	}
	var connectErr *CandidateOutcome
	for i := range result.CandidateOutcomes {
		if result.CandidateOutcomes[i].Outcome == OutcomeConnectError {
			connectErr = &result.CandidateOutcomes[i]
			break
		}
	}
	if connectErr == nil {
		t.Fatalf("expected at least one connect_error outcome; got %+v", result.CandidateOutcomes)
	}
	if connectErr.StatusCode != 0 {
		t.Errorf("StatusCode = %d, want 0", connectErr.StatusCode)
	}
	if connectErr.Detail == "" {
		t.Errorf("Detail = %q, want non-empty error string", connectErr.Detail)
	}
}

func TestAttemptHeal_SuccessCapturesOutcomes(t *testing.T) {
	// First variant 404s, second variant succeeds. Outcomes should record
	// both probes; result.NewURL must equal the success outcome's URL.
	mux := http.NewServeMux()
	hits := map[string]int{}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		hits[r.URL.Path]++
		switch r.URL.Path {
		case "/docs/foo_bar.md":
			w.Header().Set("Content-Type", "text/markdown")
			_, _ = w.Write(validBody)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
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
		t.Fatalf("expected success; FailReason=%q outcomes=%+v", result.FailReason, result.CandidateOutcomes)
	}
	if len(result.CandidateOutcomes) == 0 {
		t.Fatalf("expected outcomes recorded on success")
	}
	last := result.CandidateOutcomes[len(result.CandidateOutcomes)-1]
	if last.Outcome != OutcomeSuccess {
		t.Errorf("last outcome = %q, want %q", last.Outcome, OutcomeSuccess)
	}
	if result.NewURL != last.URL {
		t.Errorf("NewURL = %q, want %q (last outcome URL)", result.NewURL, last.URL)
	}
}

func TestAttemptHeal_FailReasonDerivedSummary(t *testing.T) {
	// All variant candidates 404. FailReason should be the derived
	// summary string from summarizeCandidateOutcomes.
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
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
		t.Fatalf("expected failure")
	}
	if len(result.CandidateOutcomes) == 0 {
		t.Fatalf("expected at least one CandidateOutcome")
	}
	if !strings.Contains(result.FailReason, "candidates") {
		t.Errorf("FailReason = %q, want derived summary mentioning 'candidates'", result.FailReason)
	}
	if !strings.Contains(result.FailReason, "http_error") {
		t.Errorf("FailReason = %q, want mention of http_error", result.FailReason)
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
