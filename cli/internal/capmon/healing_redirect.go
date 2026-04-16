package capmon

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// maxRedirectHops caps the redirect chain length to prevent runaway loops.
const maxRedirectHops = 10

// RedirectChain is the result of walking an HTTP redirect chain manually.
// Each Hop is one response whose Location header produced the next URL.
// FinalURL is the last URL that returned a non-redirect status.
// Permanent is true only when every redirect in the chain was 301 or 308 —
// a single 302/307 anywhere in the chain makes the final URL a poor heal
// candidate because the origin may return a different target next time.
type RedirectChain struct {
	StartURL  string
	FinalURL  string
	Hops      []RedirectHop
	Permanent bool
}

// RedirectHop is one step in a redirect chain.
type RedirectHop struct {
	From   string
	To     string
	Status int
}

// FollowRedirectChain issues a HEAD request to rawURL and walks the redirect
// chain manually — up to maxRedirectHops hops. It returns the final URL and
// whether the entire chain was composed of permanent redirects (301/308).
//
// Same-URL loops (a hop whose To equals its From, or a repeat of a URL seen
// earlier in the chain) terminate with an error.
//
// Callers use this as the first healing strategy: if the chain ends at a
// 2xx URL and every hop is permanent, the final URL is a safe heal candidate.
func FollowRedirectChain(ctx context.Context, rawURL string) (*RedirectChain, error) {
	if _, err := url.Parse(rawURL); err != nil {
		return nil, fmt.Errorf("parse start URL: %w", err)
	}

	// Use a client that does NOT follow redirects automatically — we walk the
	// chain ourselves so we can inspect each hop's status code.
	noFollow := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	chain := &RedirectChain{
		StartURL:  rawURL,
		Permanent: true, // downgrade to false on any 302/307
	}
	current := rawURL
	seen := map[string]bool{current: true}

	for i := 0; i < maxRedirectHops; i++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, current, nil)
		if err != nil {
			return nil, fmt.Errorf("create HEAD request for %q: %w", current, err)
		}
		req.Header.Set("User-Agent", "syllago-capmon/1.0")

		resp, err := noFollow.Do(req)
		if err != nil {
			return nil, fmt.Errorf("HEAD %q: %w", current, err)
		}
		// Drain and close; HEAD bodies are typically empty but stay safe.
		_ = resp.Body.Close()

		if resp.StatusCode < 300 || resp.StatusCode >= 400 {
			// Non-redirect response — chain terminates here.
			chain.FinalURL = current
			return chain, nil
		}

		loc := resp.Header.Get("Location")
		if loc == "" {
			return nil, fmt.Errorf("redirect status %d from %q but no Location header", resp.StatusCode, current)
		}
		next, err := resolveRedirectTarget(current, loc)
		if err != nil {
			return nil, fmt.Errorf("resolve Location %q from %q: %w", loc, current, err)
		}
		if next == current {
			return nil, fmt.Errorf("redirect loop: %q → %q", current, next)
		}
		if seen[next] {
			return nil, fmt.Errorf("redirect cycle: %q already visited", next)
		}
		seen[next] = true

		chain.Hops = append(chain.Hops, RedirectHop{
			From:   current,
			To:     next,
			Status: resp.StatusCode,
		})
		if resp.StatusCode != http.StatusMovedPermanently && resp.StatusCode != http.StatusPermanentRedirect {
			chain.Permanent = false
		}
		current = next
	}

	return nil, fmt.Errorf("redirect chain exceeded %d hops starting from %q", maxRedirectHops, rawURL)
}

// resolveRedirectTarget resolves a Location header (which may be relative)
// against the current URL per RFC 7231 §7.1.2.
func resolveRedirectTarget(currentURL, location string) (string, error) {
	base, err := url.Parse(currentURL)
	if err != nil {
		return "", err
	}
	ref, err := url.Parse(location)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(ref).String(), nil
}
