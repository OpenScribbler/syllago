package capmon

import (
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// ErrContentInvalid is returned by ValidateContentResponse when the fetched
// content fails one of the readability checks. It carries a Reason field for
// human-readable diagnostics and is distinguishable via errors.As.
type ErrContentInvalid struct {
	Reason string
}

func (e *ErrContentInvalid) Error() string {
	return fmt.Sprintf("content invalid: %s", e.Reason)
}

// minContentBytes is the minimum body size for a response to be considered
// meaningful content (avoids treating redirect pages or empty stubs as valid).
const minContentBytes = 512

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
//  1. Body is at least minContentBytes — avoids treating stubs/redirects as valid
//  2. Content-Type does not indicate binary content
//  3. The final URL's eTLD+1 matches the original URL's eTLD+1 — detects
//     redirect-to-login domain hijacking
//
// Returns *ErrContentInvalid for any failed check, allowing callers to
// distinguish content validity failures from other errors via errors.As.
func ValidateContentResponse(body []byte, contentType, originalURL, finalURL string) error {
	// Check 1: minimum body size.
	if len(body) < minContentBytes {
		return &ErrContentInvalid{
			Reason: fmt.Sprintf("body too small (%d bytes, minimum %d)", len(body), minContentBytes),
		}
	}

	// Check 2: reject binary MIME types.
	// Strip parameters (e.g. "text/html; charset=utf-8" → "text/html").
	mimeBase := strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0])
	for _, prefix := range binaryMIMEPrefixes {
		if strings.HasPrefix(mimeBase, prefix) {
			return &ErrContentInvalid{
				Reason: fmt.Sprintf("binary content-type %q", mimeBase),
			}
		}
	}
	if binaryMIMEExact[mimeBase] {
		return &ErrContentInvalid{
			Reason: fmt.Sprintf("binary content-type %q", mimeBase),
		}
	}

	// Check 3: domain match between original and final URLs.
	origURL, err := url.Parse(originalURL)
	if err != nil {
		return fmt.Errorf("parse original URL: %w", err)
	}
	finURL, err := url.Parse(finalURL)
	if err != nil {
		return fmt.Errorf("parse final URL: %w", err)
	}
	// Same-host short-circuit: identical hostnames can't be a redirect
	// hijack. This also avoids publicsuffix lookups that reject raw IPs
	// (httptest servers listen on 127.0.0.1).
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
			Reason: fmt.Sprintf("redirect domain mismatch: %q → %q", origETLD, finalETLD),
		}
	}

	return nil
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
