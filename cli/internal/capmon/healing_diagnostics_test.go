package capmon

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderCandidatesTable_Empty(t *testing.T) {
	t.Parallel()
	got := RenderCandidatesTable(nil)
	if got != "" {
		t.Errorf("RenderCandidatesTable(nil) = %q, want empty string", got)
	}
	got = RenderCandidatesTable([]CandidateOutcome{})
	if got != "" {
		t.Errorf("RenderCandidatesTable([]) = %q, want empty string", got)
	}
}

func TestRenderCandidatesTable_Success(t *testing.T) {
	t.Parallel()
	outcomes := []CandidateOutcome{
		{
			URL:         "https://docs.example.com/page.md",
			Strategy:    "variant",
			Outcome:     OutcomeSuccess,
			StatusCode:  200,
			ContentType: "text/markdown",
			BodySize:    2048,
		},
	}
	got := RenderCandidatesTable(outcomes)
	// Header row contains the column names.
	for _, want := range []string{"Strategy", "Candidate URL", "Outcome", "Status"} {
		if !strings.Contains(got, want) {
			t.Errorf("table missing header column %q\nGot:\n%s", want, got)
		}
	}
	// Separator row of pipes and dashes.
	if !strings.Contains(got, "|---") {
		t.Errorf("table missing markdown separator row\nGot:\n%s", got)
	}
	// Data row contains the candidate.
	for _, want := range []string{"variant", "https://docs.example.com/page.md", "success", "200"} {
		if !strings.Contains(got, want) {
			t.Errorf("table missing data %q\nGot:\n%s", want, got)
		}
	}
}

func TestRenderCandidatesTable_MixedOutcomes(t *testing.T) {
	t.Parallel()
	outcomes := []CandidateOutcome{
		{
			URL:        "https://a.example.com/x.md",
			Strategy:   "variant",
			Outcome:    OutcomeSuccess,
			StatusCode: 200,
		},
		{
			URL:        "https://a.example.com/missing.md",
			Strategy:   "variant",
			Outcome:    OutcomeHTTPError,
			StatusCode: 404,
		},
		{
			URL:      "https://a.example.com/old.md",
			Strategy: "redirect",
			Outcome:  OutcomeRedirected,
			FinalURL: "https://a.example.com/new.md",
			Redirects: []RedirectHop{
				{From: "https://a.example.com/old.md", To: "https://a.example.com/new.md", Status: 302},
			},
		},
	}
	got := RenderCandidatesTable(outcomes)
	// Each outcome appears in its own row with the right status code.
	if !strings.Contains(got, "success") || !strings.Contains(got, "200") {
		t.Errorf("missing success row\nGot:\n%s", got)
	}
	if !strings.Contains(got, "http_error") || !strings.Contains(got, "404") {
		t.Errorf("missing http_error 404 row\nGot:\n%s", got)
	}
	if !strings.Contains(got, "redirected") {
		t.Errorf("missing redirected row\nGot:\n%s", got)
	}
	// Three data rows + 1 header + 1 separator = 5 lines minimum.
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) < 5 {
		t.Errorf("expected at least 5 lines (header + sep + 3 data), got %d:\n%s", len(lines), got)
	}
}

func TestRenderCandidatesTable_RedirectChainRendering(t *testing.T) {
	t.Parallel()
	outcomes := []CandidateOutcome{
		{
			URL:      "https://a.example.com/start.md",
			Strategy: "variant",
			Outcome:  OutcomeRedirected,
			FinalURL: "https://a.example.com/final.md",
			Redirects: []RedirectHop{
				{From: "https://a.example.com/start.md", To: "https://a.example.com/middle.md", Status: 301},
				{From: "https://a.example.com/middle.md", To: "https://a.example.com/final.md", Status: 302},
			},
		},
	}
	got := RenderCandidatesTable(outcomes)
	// Chain rendered as `A -> B (301) -> C (302)` style.
	for _, want := range []string{
		"https://a.example.com/start.md",
		"https://a.example.com/middle.md",
		"https://a.example.com/final.md",
		"301",
		"302",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("chain rendering missing %q\nGot:\n%s", want, got)
		}
	}
}

func TestSummarizeCandidateOutcomes_Empty(t *testing.T) {
	t.Parallel()
	got := summarizeCandidateOutcomes(nil)
	if !strings.Contains(strings.ToLower(got), "no candidates") {
		t.Errorf("summarizeCandidateOutcomes(nil) = %q, want mention of no candidates", got)
	}
	got = summarizeCandidateOutcomes([]CandidateOutcome{})
	if !strings.Contains(strings.ToLower(got), "no candidates") {
		t.Errorf("summarizeCandidateOutcomes([]) = %q, want mention of no candidates", got)
	}
}

func TestSummarizeCandidateOutcomes_Counts(t *testing.T) {
	t.Parallel()
	outcomes := []CandidateOutcome{
		{URL: "u1", Strategy: "variant", Outcome: OutcomeHTTPError},
		{URL: "u2", Strategy: "variant", Outcome: OutcomeHTTPError},
		{URL: "u3", Strategy: "variant", Outcome: OutcomeHTTPError},
		{URL: "u4", Strategy: "variant", Outcome: OutcomeRedirected},
		{URL: "u5", Strategy: "variant", Outcome: OutcomeBinaryContent},
	}
	got := summarizeCandidateOutcomes(outcomes)
	for _, want := range []string{
		"5 candidates",
		"3 http_error",
		"1 redirected",
		"1 binary_content",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("summary missing %q\nGot: %q", want, got)
		}
	}
	// Frequency desc: http_error (3) appears before redirected (1) and
	// binary_content (1). Alphabetical tie-break: binary_content (b) before
	// redirected (r).
	httpIdx := strings.Index(got, "http_error")
	binIdx := strings.Index(got, "binary_content")
	redIdx := strings.Index(got, "redirected")
	if httpIdx < 0 || binIdx < 0 || redIdx < 0 {
		t.Fatalf("missing one of the kinds in %q", got)
	}
	if httpIdx > binIdx {
		t.Errorf("expected http_error before binary_content (freq desc): %q", got)
	}
	if binIdx > redIdx {
		t.Errorf("expected binary_content before redirected (alphabetical tie-break at count=1): %q", got)
	}
}

func TestRenderStrategyDeclines_Empty(t *testing.T) {
	t.Parallel()
	if got := renderStrategyDeclines(nil); got != "" {
		t.Errorf("renderStrategyDeclines(nil) = %q, want empty string", got)
	}
	if got := renderStrategyDeclines([]string{}); got != "" {
		t.Errorf("renderStrategyDeclines([]) = %q, want empty string", got)
	}
}

func TestRenderStrategyDeclines_SingleDecline(t *testing.T) {
	t.Parallel()
	reason := "redirect: chain crosses registrable domain: example.com -> netlify.app; final URL https://example-docs.netlify.app/skill.md, 3 hops, terminated 200"
	got := renderStrategyDeclines([]string{reason})
	want := "## Other strategies declined\n\n- " + reason + "\n"
	if got != want {
		t.Errorf("renderStrategyDeclines single-decline mismatch\nGot:  %q\nWant: %q", got, want)
	}
}

func TestRenderStrategyDeclines_MultipleDeclines(t *testing.T) {
	t.Parallel()
	declines := []string{
		"redirect: chain crosses registrable domain: example.com -> netlify.app; final URL https://example-docs.netlify.app/skill.md, 3 hops, terminated 200",
		"github-rename: no candidates above score floor",
	}
	got := renderStrategyDeclines(declines)
	want := "## Other strategies declined\n\n- " + declines[0] + "\n- " + declines[1] + "\n"
	if got != want {
		t.Errorf("renderStrategyDeclines multi-decline mismatch\nGot:  %q\nWant: %q", got, want)
	}
}

func TestRedirectHop_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	hop := RedirectHop{
		From:   "https://a.example.com/old",
		To:     "https://a.example.com/new",
		Status: 301,
	}
	raw, err := json.Marshal(hop)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// Lowercase JSON tags.
	for _, want := range []string{`"from"`, `"to"`, `"status"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("JSON missing tag %s in %s", want, raw)
		}
	}
	var rt RedirectHop
	if err := json.Unmarshal(raw, &rt); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if rt != hop {
		t.Errorf("round-trip mismatch: got %+v, want %+v", rt, hop)
	}
}
