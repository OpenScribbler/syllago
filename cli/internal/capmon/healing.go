package capmon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HealResult captures the outcome of an AttemptHeal call. Success is true
// only when a candidate URL was found AND its content passed
// ValidateContentResponse with zero redirects observed during the fetch.
// CandidateOutcomes records every probe in order so humans triaging drift
// can see what each strategy turned up.
type HealResult struct {
	Success           bool
	NewURL            string
	Strategy          string
	Proof             string             // short human-readable explanation for PR body
	CandidateOutcomes []CandidateOutcome // every probed candidate, in attempt order
	FailReason        string             // populated when Success=false — derived summary or strategy-level decline
}

// AttemptHeal runs the configured healing strategies in order for a
// source whose fetch just failed. Returns a HealResult whose Success
// flag indicates whether a verified replacement was found.
//
// Ordering is taken from src.EffectiveStrategies() — callers cannot
// bypass opt-out (healing.enabled=false causes AttemptHeal to return
// Success=false with a FailReason explaining the skip, never an error).
//
// The orchestrator never mutates src. All new URLs are proposed via
// the PR flow downstream of this call.
func AttemptHeal(ctx context.Context, src SourceEntry, fetchErr error) (*HealResult, error) {
	result := &HealResult{}
	if !src.IsHealingEnabled() {
		result.FailReason = "healing disabled for this source"
		return result, nil
	}
	strategies := src.EffectiveStrategies()
	if len(strategies) == 0 {
		result.FailReason = "no healing strategies configured"
		return result, nil
	}

	// Strategy-level decline reasons (e.g., redirect chain non-permanent).
	// Used only for FailReason when no candidates were probed.
	var declineReasons []string
	for _, strategy := range strategies {
		var candidates []string
		var proofs map[string]string
		switch strategy {
		case "redirect":
			cand, proof, ok := runRedirectStrategy(ctx, src.URL)
			if !ok {
				declineReasons = append(declineReasons, "redirect: "+proof)
				continue
			}
			candidates = []string{cand}
			proofs = map[string]string{cand: proof}
		case "github-rename":
			ranked, err := DetectGitHubRename(ctx, src.URL)
			if err != nil {
				declineReasons = append(declineReasons, "github-rename: "+err.Error())
				continue
			}
			if len(ranked) == 0 {
				declineReasons = append(declineReasons, "github-rename: no candidates above score floor")
				continue
			}
			proofs = map[string]string{}
			for _, r := range ranked {
				candidates = append(candidates, r.URL)
				proofs[r.URL] = r.Reason
			}
		case "variant":
			variants := GenerateVariants(src.URL)
			if len(variants) == 0 {
				declineReasons = append(declineReasons, "variant: no candidates generated")
				continue
			}
			proofs = map[string]string{}
			for _, v := range variants {
				candidates = append(candidates, v.URL)
				proofs[v.URL] = v.Reason
			}
		default:
			declineReasons = append(declineReasons, fmt.Sprintf("%s: unknown strategy (ignored)", strategy))
			continue
		}

		for _, cand := range candidates {
			outcome := validateHealCandidate(ctx, src.URL, cand, strategy)
			result.CandidateOutcomes = append(result.CandidateOutcomes, outcome)
			if outcome.Outcome == OutcomeSuccess {
				result.Success = true
				result.NewURL = cand
				result.Strategy = strategy
				result.Proof = proofs[cand]
				return result, nil
			}
		}
	}

	// Failure path. Prefer the structured summary when any candidate was
	// probed; otherwise fall back to strategy-level decline reasons so
	// e.g. "redirect chain contains a temporary redirect" still surfaces
	// when no other strategies were configured.
	if len(result.CandidateOutcomes) > 0 {
		result.FailReason = summarizeCandidateOutcomes(result.CandidateOutcomes)
	} else if len(declineReasons) > 0 {
		result.FailReason = joinReasons(declineReasons)
	} else if fetchErr != nil {
		result.FailReason = "no healing strategy produced a candidate"
	}
	return result, nil
}

// runRedirectStrategy follows the redirect chain and returns the final
// URL only if the chain is entirely permanent and terminated in a 2xx.
// A single 302 anywhere poisons the chain — temporary redirects aren't
// safe to bake into a manifest.
func runRedirectStrategy(ctx context.Context, rawURL string) (string, string, bool) {
	chain, err := FollowRedirectChain(ctx, rawURL)
	if err != nil {
		return "", err.Error(), false
	}
	if len(chain.Hops) == 0 {
		return "", "no redirects from origin", false
	}
	if !chain.Permanent {
		return "", "chain contains a temporary (302/307) redirect", false
	}
	if chain.FinalURL == rawURL {
		return "", "chain returned to origin URL", false
	}
	return chain.FinalURL, fmt.Sprintf("followed %d permanent redirects", len(chain.Hops)), true
}

// validateHealCandidate GETs a candidate URL and reports a structured
// outcome describing what happened. Any redirect during the fetch is
// drift signal — the chain is captured but the candidate is rejected
// regardless of where it lands. Body content checks run only when the
// fetch resolved with zero redirects to a 2xx response.
func validateHealCandidate(ctx context.Context, originalURL, candidateURL, strategy string) CandidateOutcome {
	out := CandidateOutcome{
		URL:      candidateURL,
		Strategy: strategy,
	}
	var hops []RedirectHop
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.Response != nil && req.Response.Request != nil && req.Response.Request.URL != nil {
				hops = append(hops, RedirectHop{
					From:   req.Response.Request.URL.String(),
					To:     req.URL.String(),
					Status: req.Response.StatusCode,
				})
			}
			if len(via) >= maxRedirectHops {
				return fmt.Errorf("stopped after %d redirects", maxRedirectHops)
			}
			return nil
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, candidateURL, nil)
	if err != nil {
		out.Outcome = OutcomeConnectError
		out.Detail = fmt.Sprintf("create heal GET: %v", err)
		return out
	}
	req.Header.Set("User-Agent", "syllago-capmon/1.0")
	resp, err := client.Do(req)
	if err != nil {
		out.Outcome = OutcomeConnectError
		out.Detail = err.Error()
		// Redirect-cap or loop errors still produced hops worth recording.
		if len(hops) > 0 {
			out.Redirects = hops
			if last := hops[len(hops)-1]; last.To != "" {
				out.FinalURL = last.To
			}
		}
		return out
	}
	defer resp.Body.Close() //nolint:errcheck // nothing actionable on close failure of a drained body

	// resp.Request.URL reflects where the chain landed (after default
	// CheckRedirect followed each hop). When hops were captured, that's
	// the FinalURL; otherwise it's the candidate URL itself.
	finalURL := candidateURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}

	if len(hops) > 0 {
		out.Outcome = OutcomeRedirected
		out.Redirects = hops
		out.FinalURL = finalURL
		out.StatusCode = resp.StatusCode
		// Drain the body to free the connection but do not validate it —
		// any redirect already disqualifies the candidate.
		_, _ = io.Copy(io.Discard, resp.Body)
		return out
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		out.Outcome = OutcomeHTTPError
		out.StatusCode = resp.StatusCode
		out.Detail = fmt.Sprintf("status %d", resp.StatusCode)
		return out
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		out.Outcome = OutcomeConnectError
		out.Detail = fmt.Sprintf("read heal body: %v", err)
		return out
	}
	out.StatusCode = resp.StatusCode
	out.ContentType = resp.Header.Get("Content-Type")
	out.BodySize = len(body)
	out.FinalURL = finalURL

	if vErr := ValidateContentResponse(body, out.ContentType, originalURL, finalURL); vErr != nil {
		var invalid *ErrContentInvalid
		if errors.As(vErr, &invalid) {
			out.Outcome = invalidKindToOutcome(invalid.Kind)
			out.Detail = invalid.Reason
			return out
		}
		// Non-content-validity error (URL parse, etc.).
		out.Outcome = OutcomeConnectError
		out.Detail = vErr.Error()
		return out
	}

	out.Outcome = OutcomeSuccess
	return out
}

// invalidKindToOutcome maps content-invalidity kinds onto the heal
// outcome enum 1:1.
func invalidKindToOutcome(k InvalidKind) CandidateOutcomeKind {
	switch k {
	case InvalidBinaryContent:
		return OutcomeBinaryContent
	case InvalidBodyTooSmall:
		return OutcomeBodyTooSmall
	case InvalidDomainMismatch:
		return OutcomeDomainMismatch
	default:
		// Unknown kinds shouldn't reach here, but treat as a generic
		// connect_error rather than crashing.
		return OutcomeConnectError
	}
}

func joinReasons(reasons []string) string {
	// Keep failure reasons on one line so issue bodies don't balloon. Used
	// only as a fallback FailReason when no candidates were probed.
	if len(reasons) == 0 {
		return ""
	}
	out := reasons[0]
	for _, r := range reasons[1:] {
		out += "; " + r
	}
	return out
}
