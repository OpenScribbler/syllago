package capmon

import (
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// InvalidKind tags the specific reason ValidateContentResponse rejected a
// response. Heal callers map this 1:1 onto CandidateOutcomeKind so the
// reason for rejection can be reported structurally (without parsing the
// human-readable Reason string).
type InvalidKind string

const (
	InvalidBinaryContent  InvalidKind = "binary_content"
	InvalidBodyTooSmall   InvalidKind = "body_too_small"
	InvalidDomainMismatch InvalidKind = "domain_mismatch"
)

// ErrContentInvalid is returned by ValidateContentResponse when the fetched
// content fails one of the readability checks. It carries a Reason field for
// human-readable diagnostics and a Kind for machine routing; distinguishable
// via errors.As.
type ErrContentInvalid struct {
	Kind   InvalidKind
	Reason string
}

func (e *ErrContentInvalid) Error() string {
	return fmt.Sprintf("content invalid: %s", e.Reason)
}

// minContentBytesHTML is the minimum body size for an HTML response to be
// considered meaningful. The threshold filters blank stubs, redirect pages,
// and lean 404 HTML — all of which are HTML by construction. It does NOT
// apply to non-HTML responses, because legitimate source files (TypeScript
// enums, Rust modules, lean Markdown stubs) are routinely tiny: a 196-byte
// `HookScope.ts` from raw.githubusercontent.com is real content, not a stub.
//
// The same gate exists in provmon (cli/internal/provmon/checker_source_hash.go)
// where the call site comments that "ValidateContentResponse is only meaningful
// for HTML responses." That observation is correct; this module now enforces it
// at the validator instead of relying on every caller to repeat the check.
const minContentBytesHTML = 512

// binaryMIMEPrefixes are MIME type prefixes that indicate non-text content.
// Content with these types cannot be meaningfully read as documentation.
var binaryMIMEPrefixes = []string{
	"image/",
	"video/",
	"audio/",
}

// binaryMIMEExact are specific MIME types that indicate binary or structured
// non-text content that cannot be read as documentation.
var binaryMIMEExact = map[string]bool{
	"application/octet-stream": true,
	"application/zip":          true,
	"application/pdf":          true,
	"application/gzip":         true,
	"application/x-tar":        true,
}

// ValidateContentResponse checks that a fetched HTTP response body represents
// readable, on-domain content. It performs three checks:
//
//  1. Body is non-empty, and (for HTML responses only) at least
//     minContentBytesHTML — avoids treating HTML stubs/redirect pages as valid
//     while letting legitimate small source files pass.
//  2. Content-Type does not indicate binary content
//  3. The final URL's eTLD+1 matches the original URL's eTLD+1 — detects
//     redirect-to-login domain hijacking
//
// Returns *ErrContentInvalid for any failed check, allowing callers to
// distinguish content validity failures from other errors via errors.As.
// The Kind field on the returned error tags which check failed.
func ValidateContentResponse(body []byte, contentType, originalURL, finalURL string) error {
	// Check 1a: empty bodies are always invalid — that's a definitive failure
	// mode (truncated transfer, vanished resource) and surfaces real upstream
	// emptiness rather than masking it.
	if len(body) == 0 {
		return &ErrContentInvalid{
			Kind:   InvalidBodyTooSmall,
			Reason: "body empty",
		}
	}
	// Check 1b: HTML-only size threshold. Non-HTML responses are not
	// size-checked because real source files and short docs stubs can be
	// legitimately tiny.
	if isHTMLContentType(contentType) && len(body) < minContentBytesHTML {
		return &ErrContentInvalid{
			Kind:   InvalidBodyTooSmall,
			Reason: fmt.Sprintf("HTML body too small (%d bytes, minimum %d)", len(body), minContentBytesHTML),
		}
	}

	// Check 2: reject binary MIME types.
	// Strip parameters (e.g. "text/html; charset=utf-8" → "text/html").
	mimeBase := strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0])
	for _, prefix := range binaryMIMEPrefixes {
		if strings.HasPrefix(mimeBase, prefix) {
			return &ErrContentInvalid{
				Kind:   InvalidBinaryContent,
				Reason: fmt.Sprintf("binary content-type %q", mimeBase),
			}
		}
	}
	if binaryMIMEExact[mimeBase] {
		return &ErrContentInvalid{
			Kind:   InvalidBinaryContent,
			Reason: fmt.Sprintf("binary content-type %q", mimeBase),
		}
	}

	// Check 3: domain match between original and final URLs. Same-host
	// short-circuit: identical hostnames can't be a cross-domain redirect
	// hijack. This also avoids publicsuffix lookups that reject raw IPs
	// (httptest servers listen on 127.0.0.1).
	origURL, err := url.Parse(originalURL)
	if err != nil {
		return fmt.Errorf("parse original URL: %w", err)
	}
	finURL, err := url.Parse(finalURL)
	if err != nil {
		return fmt.Errorf("parse final URL: %w", err)
	}
	if origURL.Hostname() == finURL.Hostname() {
		return nil
	}
	origETLD, err := etldPlusOne(originalURL)
	if err != nil {
		return fmt.Errorf("parse original URL: %w", err)
	}
	finalETLD, err := etldPlusOne(finalURL)
	if err != nil {
		return fmt.Errorf("parse final URL: %w", err)
	}
	if origETLD != finalETLD {
		return &ErrContentInvalid{
			Kind:   InvalidDomainMismatch,
			Reason: fmt.Sprintf("redirect domain mismatch: %q → %q", origETLD, finalETLD),
		}
	}

	return nil
}

// isHTMLContentType reports whether the Content-Type header indicates HTML.
// Parameters (e.g. "; charset=utf-8") are stripped before comparison.
// Mirrors the helper of the same name in cli/internal/provmon — the gate is
// intentionally identical so the two packages share one definition of "this
// looks like an HTML response."
func isHTMLContentType(contentType string) bool {
	mimeBase := strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0])
	return strings.EqualFold(mimeBase, "text/html") || strings.EqualFold(mimeBase, "application/xhtml+xml")
}

// etldPlusOne parses rawURL and returns its eTLD+1 (e.g. "github.com" from
// "raw.githubusercontent.com"). Returns an error if the URL cannot be parsed
// or the host has no valid public suffix.
func etldPlusOne(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse %q: %w", rawURL, err)
	}
	host := u.Hostname() // strips port
	if host == "" {
		return "", fmt.Errorf("no host in URL %q", rawURL)
	}
	etld, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		return "", fmt.Errorf("eTLD+1 for %q: %w", host, err)
	}
	return etld, nil
}
