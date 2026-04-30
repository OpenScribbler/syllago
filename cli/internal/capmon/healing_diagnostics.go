package capmon

import (
	"fmt"
	"sort"
	"strings"
)

// CandidateOutcomeKind is a closed enum tagging the outcome of probing a
// single heal candidate URL. Every probed candidate produces exactly one
// outcome; the value is what humans see in the diagnostic table when
// triaging drift.
type CandidateOutcomeKind string

const (
	OutcomeSuccess        CandidateOutcomeKind = "success"
	OutcomeRedirected     CandidateOutcomeKind = "redirected"
	OutcomeHTTPError      CandidateOutcomeKind = "http_error"
	OutcomeConnectError   CandidateOutcomeKind = "connect_error"
	OutcomeBinaryContent  CandidateOutcomeKind = "binary_content"
	OutcomeBodyTooSmall   CandidateOutcomeKind = "body_too_small"
	OutcomeDomainMismatch CandidateOutcomeKind = "domain_mismatch"
)

// CandidateOutcome captures everything observed during a single candidate
// probe. The JSON form is persisted in the run manifest (HealEvent) and
// rendered into PR bodies and escalation issue bodies.
type CandidateOutcome struct {
	URL         string               `json:"url"`
	Strategy    string               `json:"strategy"`
	Outcome     CandidateOutcomeKind `json:"outcome"`
	StatusCode  int                  `json:"status_code,omitempty"`
	FinalURL    string               `json:"final_url,omitempty"`
	Redirects   []RedirectHop        `json:"redirects,omitempty"`
	ContentType string               `json:"content_type,omitempty"`
	BodySize    int                  `json:"body_size,omitempty"`
	Detail      string               `json:"detail,omitempty"`
}

// RenderCandidatesTable renders outcomes as a markdown table for inclusion
// in PR bodies and GitHub issue bodies. Columns: Strategy, Candidate URL,
// Outcome, Status, Final URL, Detail. Redirect chains are inlined into the
// Final URL cell as `from -> to (302) -> to (302)`. An empty input yields
// an empty string so callers can omit the section entirely.
func RenderCandidatesTable(outcomes []CandidateOutcome) string {
	if len(outcomes) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("| Strategy | Candidate URL | Outcome | Status | Final URL | Detail |\n")
	b.WriteString("|---|---|---|---|---|---|\n")
	for _, o := range outcomes {
		status := ""
		if o.StatusCode != 0 {
			status = fmt.Sprintf("%d", o.StatusCode)
		}
		final := renderFinalCell(o)
		detail := o.Detail
		// Pipes inside cell content break markdown tables.
		fmt.Fprintf(&b,
			"| %s | %s | %s | %s | %s | %s |\n",
			escapeCell(o.Strategy),
			escapeCell(o.URL),
			escapeCell(string(o.Outcome)),
			status,
			escapeCell(final),
			escapeCell(detail),
		)
	}
	return b.String()
}

// renderFinalCell produces the "Final URL" cell content. When redirects are
// present, it inlines the chain as `from -> to (status) -> to (status)`.
// Otherwise it returns the FinalURL field (which may be empty).
func renderFinalCell(o CandidateOutcome) string {
	if len(o.Redirects) == 0 {
		return o.FinalURL
	}
	var parts []string
	parts = append(parts, o.Redirects[0].From)
	for _, hop := range o.Redirects {
		parts = append(parts, fmt.Sprintf("%s (%d)", hop.To, hop.Status))
	}
	return strings.Join(parts, " -> ")
}

// escapeCell replaces characters that would break a markdown table cell.
// Pipes are escaped; newlines are replaced with spaces.
func escapeCell(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

// renderStrategyDeclines returns a markdown section listing strategy-level
// decline reasons. Returns the empty string when declines is empty so callers
// can omit the section entirely without an orphaned header.
//
// Used by both BuildHealPRBody (success path) and the failure-issue body
// builder so the human always sees what strategies declined and why, even
// when other strategies succeeded or produced their own outcomes. Each
// reason is rendered verbatim — no truncation, no escaping. Markdown
// auto-links bare URLs inside list bullets, so URL escaping is unnecessary.
//
// Trailing newline mirrors RenderCandidatesTable so both renderers compose
// uniformly into surrounding body text.
func renderStrategyDeclines(declines []string) string {
	if len(declines) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Other strategies declined\n\n")
	for _, d := range declines {
		b.WriteString("- ")
		b.WriteString(d)
		b.WriteString("\n")
	}
	return b.String()
}

// summarizeCandidateOutcomes returns a one-liner aggregate count, e.g.
// `"5 candidates: 3 http_error, 1 binary_content, 1 redirected"`. Counts
// are ordered by frequency desc, with alphabetical tie-break. Used as
// HealResult.FailReason and for issue-comment append on subsequent
// failures, where a full table would balloon noise.
func summarizeCandidateOutcomes(outcomes []CandidateOutcome) string {
	if len(outcomes) == 0 {
		return "no candidates probed"
	}
	counts := make(map[CandidateOutcomeKind]int)
	for _, o := range outcomes {
		counts[o.Outcome]++
	}
	type kc struct {
		Kind  CandidateOutcomeKind
		Count int
	}
	pairs := make([]kc, 0, len(counts))
	for k, c := range counts {
		pairs = append(pairs, kc{k, c})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].Count != pairs[j].Count {
			return pairs[i].Count > pairs[j].Count
		}
		return pairs[i].Kind < pairs[j].Kind
	})
	parts := make([]string, 0, len(pairs))
	for _, p := range pairs {
		parts = append(parts, fmt.Sprintf("%d %s", p.Count, p.Kind))
	}
	return fmt.Sprintf("%d candidates: %s", len(outcomes), strings.Join(parts, ", "))
}
