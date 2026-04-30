package capmon

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	result, err := AttemptHeal(context.Background(), src, nil, errors.New("fetch 404"))
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
	result, err := AttemptHeal(context.Background(), src, nil, errors.New("404"))
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
	result, err := AttemptHeal(context.Background(), src, nil, errors.New("404"))
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
	result, _ := AttemptHeal(context.Background(), src, nil, errors.New("404"))
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
	result, err := AttemptHeal(context.Background(), src, nil, errors.New("404"))
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
	result, _ := AttemptHeal(context.Background(), src, nil, errors.New("404"))
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
	result, err := AttemptHeal(context.Background(), src, nil, errors.New("404"))
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
	result, err := AttemptHeal(context.Background(), src, nil, errors.New("404"))
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
	result, err := AttemptHeal(context.Background(), src, nil, errors.New("404"))
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
	result, err := AttemptHeal(context.Background(), src, nil, errors.New("404"))
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
	result, err := AttemptHeal(context.Background(), src, nil, errors.New("404"))
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
	result, err := AttemptHeal(context.Background(), src, nil, errors.New("404"))
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
	result, err := AttemptHeal(context.Background(), src, nil, errors.New("404"))
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

func TestAttemptHeal_StrategyDeclinesPopulatedAlongsideOutcomes(t *testing.T) {
	// Regression for syllago-dm1zr / ampcode #92: when redirect strategy
	// declines AND a later strategy probes candidates, the decline reasons
	// must surface on result.StrategyDeclines so humans triaging drift can
	// see what each strategy turned up. Previously, declineReasons was
	// silently dropped whenever len(CandidateOutcomes) > 0.
	//
	// FailReason behavior is unchanged: when candidates were probed, it
	// stays the structured candidate-summary one-liner, NOT the decline
	// join. Both signals coexist in the result.
	mux := http.NewServeMux()
	mux.HandleFunc("/old/foo-bar.md", func(w http.ResponseWriter, r *http.Request) {
		// 302 — redirect strategy declines with "temporary" reason.
		w.Header().Set("Location", "/new/foo-bar.md")
		w.WriteHeader(http.StatusFound)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// All other paths 404 — variant candidates fail with http_error.
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := SourceEntry{
		URL: srv.URL + "/old/foo-bar.md",
		Healing: &HealingConfig{
			Strategies: []string{"redirect", "variant"},
		},
	}
	result, err := AttemptHeal(context.Background(), src, nil, errors.New("404"))
	if err != nil {
		t.Fatalf("AttemptHeal: %v", err)
	}
	if result.Success {
		t.Fatalf("expected failure; got Success with NewURL=%q", result.NewURL)
	}

	// Assertion 1: StrategyDeclines contains the redirect decline reason verbatim.
	if len(result.StrategyDeclines) == 0 {
		t.Fatalf("StrategyDeclines should be populated even when CandidateOutcomes are present; got empty")
	}
	foundRedirectDecline := false
	for _, d := range result.StrategyDeclines {
		if strings.HasPrefix(d, "redirect: ") && strings.Contains(d, "temporary") {
			foundRedirectDecline = true
			break
		}
	}
	if !foundRedirectDecline {
		t.Errorf("StrategyDeclines = %v, want one entry starting with 'redirect: ' and mentioning 'temporary'", result.StrategyDeclines)
	}

	// Assertion 2: CandidateOutcomes contains the variant outcomes.
	if len(result.CandidateOutcomes) == 0 {
		t.Fatalf("CandidateOutcomes should record variant probes")
	}
	foundVariantOutcome := false
	for _, o := range result.CandidateOutcomes {
		if o.Strategy == "variant" {
			foundVariantOutcome = true
			break
		}
	}
	if !foundVariantOutcome {
		t.Errorf("CandidateOutcomes = %+v, want at least one variant outcome", result.CandidateOutcomes)
	}

	// Assertion 3: FailReason is the candidate-summary one-liner (NOT the decline join).
	// summarizeCandidateOutcomes mentions the word "candidates"; the decline
	// fallback joinReasons would produce a string starting with "redirect:".
	if !strings.Contains(result.FailReason, "candidates") {
		t.Errorf("FailReason = %q, want candidate-summary mentioning 'candidates'", result.FailReason)
	}
	if strings.HasPrefix(result.FailReason, "redirect:") {
		t.Errorf("FailReason = %q, must not be the decline-join when candidates were probed", result.FailReason)
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
	result, err := AttemptHeal(context.Background(), src, nil, errors.New("404"))
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

// hostRewriteTransport redirects all requests to a fixed loopback address,
// ignoring the URL's hostname. Used to exercise eTLD+1 logic with httptest
// servers where the URL's hostname differs from the actual listener address.
type hostRewriteTransport struct {
	target    string // e.g. "127.0.0.1:55123"
	tlsConfig *tls.Config
}

func (h *hostRewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t := &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, h.target)
		},
		TLSClientConfig: h.tlsConfig,
	}
	return t.RoundTrip(req)
}

// withRedirectClientForTest installs a custom http.Client for the
// FollowRedirectChain HEAD walker for the duration of the test. The client
// MUST set CheckRedirect to http.ErrUseLastResponse so redirects aren't
// auto-followed by stdlib.
func withRedirectClientForTest(t *testing.T, target string, tlsConfig *tls.Config) {
	t.Helper()
	client := &http.Client{
		Transport: &hostRewriteTransport{target: target, tlsConfig: tlsConfig},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	SetRedirectClientForTest(client)
	t.Cleanup(func() { SetRedirectClientForTest(nil) })
}

func TestRunRedirectStrategy_DeclinesCrossRegistrableDomain(t *testing.T) {
	// Origin host is www.example.com; chain 308s to docs.other-domain.com.
	// Two distinct eTLD+1s — must decline at the strategy gate, before any
	// GET probe, with a rich human-readable reason.
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Host {
		case "www.example.com":
			w.Header().Set("Location", "https://docs.other-domain.com/new")
			w.WriteHeader(http.StatusPermanentRedirect) // 308
		case "docs.other-domain.com":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse srv URL: %v", err)
	}
	withRedirectClientForTest(t, u.Host, &tls.Config{InsecureSkipVerify: true})

	cand, reason, ok := runRedirectStrategy(context.Background(), "https://www.example.com/old")
	if ok {
		t.Fatalf("expected decline; got cand=%q ok=true", cand)
	}
	for _, want := range []string{
		"example.com",           // origin eTLD+1
		"other-domain.com",      // final eTLD+1
		"docs.other-domain.com", // final URL hostname (verbatim in URL)
		"1 hop",                 // hop count
		"200",                   // terminating status
	} {
		if !strings.Contains(reason, want) {
			t.Errorf("reason missing %q; got %q", want, reason)
		}
	}
	if !strings.Contains(reason, "crosses") && !strings.Contains(reason, "registrable") {
		t.Errorf("reason should describe registrable-domain crossover; got %q", reason)
	}
}

func TestRunRedirectStrategy_AcceptsSameRegistrableDomain(t *testing.T) {
	// www.example.com 308s to docs.example.com — same eTLD+1 (example.com).
	// Strategy must still accept; eTLD+1 check is a regression guard.
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Host {
		case "www.example.com":
			w.Header().Set("Location", "https://docs.example.com/new")
			w.WriteHeader(http.StatusPermanentRedirect) // 308
		case "docs.example.com":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse srv URL: %v", err)
	}
	withRedirectClientForTest(t, u.Host, &tls.Config{InsecureSkipVerify: true})

	cand, reason, ok := runRedirectStrategy(context.Background(), "https://www.example.com/old")
	if !ok {
		t.Fatalf("expected accept; got reason=%q", reason)
	}
	if cand != "https://docs.example.com/new" {
		t.Errorf("cand = %q, want https://docs.example.com/new", cand)
	}
}

// TestAttemptHeal_RetiredPathFiltersVariant pins the docs_conventions
// retired_paths semantics. Original URL has stem "Hooks" — variant
// generator emits "/manual/hooks.md" via the lowercase-stem rule. That
// path is in retired_paths, so it must be dropped before probing.
// Without conventions, the same variant would be probed and (in this
// fixture) succeed; with conventions, no variant remains and the heal
// fails with a "retired_paths" decline reason.
func TestAttemptHeal_RetiredPathFiltersVariant(t *testing.T) {
	mux := http.NewServeMux()
	// /manual/hooks.md returns valid content. Without conventions, the
	// lowercase-stem variant of /manual/Hooks.md would heal here.
	mux.HandleFunc("/manual/hooks.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		_, _ = w.Write(validBody)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := SourceEntry{
		URL: srv.URL + "/manual/Hooks.md",
		Healing: &HealingConfig{
			Strategies: []string{"variant"},
		},
	}
	conventions := &DocsConventions{
		RetiredPaths: []string{"/manual/hooks.md"},
	}
	result, err := AttemptHeal(context.Background(), src, conventions, errors.New("404"))
	if err != nil {
		t.Fatalf("AttemptHeal: %v", err)
	}
	// The retired path must not appear in CandidateOutcomes — filtering
	// happens before any probe.
	for _, o := range result.CandidateOutcomes {
		if strings.HasSuffix(o.URL, "/manual/hooks.md") {
			t.Errorf("retired path %q was probed; expected to be filtered. CandidateOutcomes=%+v", o.URL, result.CandidateOutcomes)
		}
	}
	// At least one strategy decline should record the drop.
	foundDecline := false
	for _, d := range result.StrategyDeclines {
		if strings.Contains(d, "retired_paths") {
			foundDecline = true
			break
		}
	}
	if !foundDecline {
		t.Errorf("StrategyDeclines = %v, want one mentioning retired_paths", result.StrategyDeclines)
	}
}

// TestAttemptHeal_AuthGateDetailEnriched pins the docs_conventions
// auth_gated_paths semantics. Variant candidate 302-redirects to
// /auth/sign-in (qc7yf already rejects this on correctness grounds).
// With conventions populated, the resulting CandidateOutcome.Detail
// should call out the auth gate so triagers see the cause, not just
// "redirected".
func TestAttemptHeal_AuthGateDetailEnriched(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/sign-in", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(validBody)
	})
	mux.HandleFunc("/manual/hooks/index.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/auth/sign-in?returnTo="+r.URL.Path)
		w.WriteHeader(http.StatusFound) // 302
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := SourceEntry{
		URL: srv.URL + "/manual/hooks.md",
		Healing: &HealingConfig{
			Strategies: []string{"variant"},
		},
	}
	conventions := &DocsConventions{
		AuthGatedPaths: []string{"/auth/sign-in", "/login"},
	}
	result, err := AttemptHeal(context.Background(), src, conventions, errors.New("404"))
	if err != nil {
		t.Fatalf("AttemptHeal: %v", err)
	}
	if result.Success {
		t.Fatalf("expected rejection on auth-gate redirect; got NewURL=%q", result.NewURL)
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
		t.Fatalf("expected a redirected outcome; got %+v", result.CandidateOutcomes)
	}
	if !strings.Contains(redirected.Detail, "auth gate") {
		t.Errorf("redirected.Detail = %q, want mention of auth gate", redirected.Detail)
	}
	if !strings.Contains(redirected.Detail, "/auth/sign-in") {
		t.Errorf("redirected.Detail = %q, want the matched gate path included", redirected.Detail)
	}
}

// TestAttemptHeal_AuthGateDetail_MidChainHop pins the live amp pattern
// observed during yogqh end-to-end testing: the candidate redirects
// through /auth/sign-in but the chain terminates on a different host
// (e.g. auth.ampcode.com/?client_id=...). Enrichment must scan the
// redirect hops, not just FinalURL — otherwise the qc7yf incident's
// most common surface (Hooks/index.md → /auth/sign-in → cross-host
// auth bootstrap) shows up as a generic "redirected" cell.
func TestAttemptHeal_AuthGateDetail_MidChainHop(t *testing.T) {
	// External "auth bootstrap" host the chain terminates on.
	authHost := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(validBody)
	}))
	defer authHost.Close()

	mux := http.NewServeMux()
	// /auth/sign-in is the gate path — bounces to the external host.
	mux.HandleFunc("/auth/sign-in", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", authHost.URL+"/?client_id=fake")
		w.WriteHeader(http.StatusFound)
	})
	// Variant landing page — 302s to /auth/sign-in.
	mux.HandleFunc("/manual/hooks/index.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/auth/sign-in")
		w.WriteHeader(http.StatusFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := SourceEntry{
		URL: srv.URL + "/manual/hooks.md",
		Healing: &HealingConfig{
			Strategies: []string{"variant"},
		},
	}
	conventions := &DocsConventions{
		AuthGatedPaths: []string{"/auth/sign-in", "/login"},
	}
	result, err := AttemptHeal(context.Background(), src, conventions, errors.New("404"))
	if err != nil {
		t.Fatalf("AttemptHeal: %v", err)
	}
	if result.Success {
		t.Fatalf("expected rejection on cross-host auth-gate redirect; got NewURL=%q", result.NewURL)
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
		t.Fatalf("expected a redirected outcome; got %+v", result.CandidateOutcomes)
	}
	// Sanity: chain must have a hop into /auth/sign-in but final URL must
	// be the external auth host (no /auth/sign-in substring).
	if strings.Contains(redirected.FinalURL, "/auth/sign-in") {
		t.Fatalf("test setup wrong: FinalURL still contains gate substring: %s", redirected.FinalURL)
	}
	if !strings.Contains(redirected.Detail, "auth gate") {
		t.Errorf("mid-chain auth gate not surfaced; Detail = %q", redirected.Detail)
	}
	if !strings.Contains(redirected.Detail, "/auth/sign-in") {
		t.Errorf("Detail should name matched gate path; got %q", redirected.Detail)
	}
}

// TestAttemptHeal_AuthGateDetail_NilConventionsNoOp ensures the
// enrichment is genuinely additive — when conventions is nil, the
// Detail field for OutcomeRedirected stays empty (matching pre-bead
// behavior).
func TestAttemptHeal_AuthGateDetail_NilConventionsNoOp(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/sign-in", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(validBody)
	})
	mux.HandleFunc("/manual/hooks/index.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/auth/sign-in")
		w.WriteHeader(http.StatusFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := SourceEntry{
		URL: srv.URL + "/manual/hooks.md",
		Healing: &HealingConfig{
			Strategies: []string{"variant"},
		},
	}
	result, err := AttemptHeal(context.Background(), src, nil, errors.New("404"))
	if err != nil {
		t.Fatalf("AttemptHeal: %v", err)
	}
	for _, o := range result.CandidateOutcomes {
		if o.Outcome == OutcomeRedirected && o.Detail != "" {
			t.Errorf("with nil conventions, redirected outcome should have empty Detail; got %q", o.Detail)
		}
	}
}
