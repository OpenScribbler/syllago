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
// ValidateContentResponse. NewURL is the healed URL the orchestrator
// should record in the manifest PR.
type HealResult struct {
	Success    bool
	NewURL     string
	Strategy   string
	Proof      string   // short human-readable explanation for PR body
	TriedURLs  []string // every candidate URL probed (for audit/debug)
	FailReason string   // populated when Success=false — why each strategy declined
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

	var reasons []string
	for _, strat := range strategies {
		var candidates []string
		var proofs map[string]string
		switch strat {
		case "redirect":
			cand, proof, ok := runRedirectStrategy(ctx, src.URL)
			if !ok {
				reasons = append(reasons, "redirect: "+proof)
				continue
			}
			candidates = []string{cand}
			proofs = map[string]string{cand: proof}
		case "github-rename":
			ranked, err := DetectGitHubRename(ctx, src.URL)
			if err != nil {
				reasons = append(reasons, "github-rename: "+err.Error())
				continue
			}
			if len(ranked) == 0 {
				reasons = append(reasons, "github-rename: no candidates above score floor")
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
				reasons = append(reasons, "variant: no candidates generated")
				continue
			}
			proofs = map[string]string{}
			for _, v := range variants {
				candidates = append(candidates, v.URL)
				proofs[v.URL] = v.Reason
			}
		default:
			reasons = append(reasons, fmt.Sprintf("%s: unknown strategy (ignored)", strat))
			continue
		}

		// Validate each candidate by fetching and running the content gate.
		for _, cand := range candidates {
			result.TriedURLs = append(result.TriedURLs, cand)
			if err := validateHealCandidate(ctx, src.URL, cand); err != nil {
				var invalid *ErrContentInvalid
				if errors.As(err, &invalid) {
					reasons = append(reasons, fmt.Sprintf("%s %q: %s", strat, cand, invalid.Reason))
					continue
				}
				reasons = append(reasons, fmt.Sprintf("%s %q: %v", strat, cand, err))
				continue
			}
			result.Success = true
			result.NewURL = cand
			result.Strategy = strat
			result.Proof = proofs[cand]
			return result, nil
		}
	}

	if len(reasons) > 0 {
		result.FailReason = joinReasons(reasons)
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

// validateHealCandidate GETs a candidate URL, passes the response body
// through ValidateContentResponse, and returns nil only if every gate
// passes. The orchestrator calls this for each candidate regardless of
// the strategy that produced it — we never trust a HEAD-only match.
func validateHealCandidate(ctx context.Context, originalURL, candidateURL string) error {
	// Short per-candidate timeout. The outer context bounds the total
	// healing budget; this bounds each individual fetch so one stalled
	// server can't consume the whole budget.
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, candidateURL, nil)
	if err != nil {
		return fmt.Errorf("create heal GET: %w", err)
	}
	req.Header.Set("User-Agent", "syllago-capmon/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("heal GET %q: %w", candidateURL, err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("heal GET %q: status %d", candidateURL, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read heal body: %w", err)
	}
	// resp.Request.URL reflects the final URL after Go's automatic
	// redirect following (default client follows); pass that as the
	// final URL to the eTLD+1 check.
	finalURL := candidateURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	return ValidateContentResponse(body, resp.Header.Get("Content-Type"), originalURL, finalURL)
}

func joinReasons(reasons []string) string {
	// Keep failure reasons on one line so issue bodies don't balloon. The
	// orchestrator emits the full list; this is the summary stored in
	// HealResult.
	if len(reasons) == 0 {
		return ""
	}
	out := reasons[0]
	for _, r := range reasons[1:] {
		out += "; " + r
	}
	return out
}
