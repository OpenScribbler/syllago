package moat

// FetchPublisherAttestation — Dual-Attested second leg.
//
// MOAT's Dual-Attested tier carries TWO independent Rekor entries per item:
//
//  1. The registry's per-item Rekor entry, indexed by manifest
//     content[].rekor_log_index. Its cert subject pins to the manifest's
//     registry_signing_profile.
//  2. The publisher's per-item Rekor entry, declared in
//     moat-attestation.json on the source repo's `moat-attestation`
//     branch. Its cert subject pins to per-item content[].signing_profile.
//
// Self-publishing (one repo running both moat-publisher.yml and
// moat-registry.yml) is a first-class spec model — see moat-spec.md
// §"Self-Publishing Pattern" lines 124–128. The two Rekor entries are
// distinct because the workflows run with distinct OIDC subjects.
//
// This file fetches and parses the publisher attestation document. The
// install path passes the resulting logIndex to FetchRekorEntry and reuses
// VerifyAttestationItem with content[].signing_profile as the pin —
// mirroring the reference implementation moat_verify.py:_online_step5.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// publisherAttestationBaseURL is the GitHub raw.githubusercontent.com host
// used to fetch moat-attestation.json from a publisher's source repo. The
// reference impl pulls from the moat-attestation BRANCH, not main —
// moat-spec.md still says /raw/main/ but moat_verify.py:_online_step5 uses
// the moat-attestation branch. Package-level so tests can swap to a
// httptest.NewServer URL.
var publisherAttestationBaseURL = "https://raw.githubusercontent.com"

// PublisherAttestationBaseURLForTest returns the current value. Test-only
// surface paired with Set/Cleanup.
func PublisherAttestationBaseURLForTest() string { return publisherAttestationBaseURL }

// SetPublisherAttestationBaseURLForTest swaps the host for offline tests.
// Production callers MUST NOT touch this.
func SetPublisherAttestationBaseURLForTest(u string) { publisherAttestationBaseURL = u }

// FetchPublisherAttestation GETs moat-attestation.json from the publisher
// source repo identified by sourceURI. sourceURI is the per-item
// content[].source_uri (an https:// tarball) — this function extracts the
// owner/repo prefix and constructs the raw URL on the moat-attestation
// branch.
//
// Returns the response body verbatim. VerifyAttestationItem hashes Rekor
// bytes downstream, but the publisher attestation document itself is JSON
// we parse — re-marshaling is fine in callers, but this layer stays a byte
// pipe to keep test fixtures honest.
func FetchPublisherAttestation(ctx context.Context, sourceURI string) ([]byte, error) {
	ownerRepo, err := publisherOwnerRepoFromSourceURI(sourceURI)
	if err != nil {
		return nil, err
	}

	rawURL := fmt.Sprintf("%s/%s/moat-attestation/moat-attestation.json", publisherAttestationBaseURL, ownerRepo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("publisher-attestation: build request: %w", err)
	}
	req.Header.Set("User-Agent", DefaultUserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := rekorFetchClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("publisher-attestation: fetch %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("publisher-attestation: %s not found at %s — publisher repo missing the moat-attestation branch or moat-attestation.json", ownerRepo, rawURL)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("publisher-attestation: unexpected status %d for %s", resp.StatusCode, rawURL)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxRekorBytes+1))
	if err != nil {
		return nil, fmt.Errorf("publisher-attestation: read body: %w", err)
	}
	if len(body) > maxRekorBytes {
		return nil, fmt.Errorf("publisher-attestation: response exceeds %d byte cap", maxRekorBytes)
	}
	return body, nil
}

// FindPublisherEntry parses moat-attestation.json and returns the
// rekor_log_index whose items[] entry matches the given content_hash. The
// publisher attestation lists every item the publisher signed; the registry
// emits one manifest entry per item, so matching by content_hash is the
// stable cross-reference.
//
// Per moat-spec.md and moat-attestation schema, items[].content_hash is the
// authoritative key. Name is not — derivation, rename, or fork can change
// the name while content_hash stays constant.
func FindPublisherEntry(rawJSON []byte, contentHash string) (int64, error) {
	var att Attestation
	if err := json.Unmarshal(rawJSON, &att); err != nil {
		return 0, fmt.Errorf("publisher-attestation: parse json: %w", err)
	}
	for _, it := range att.Items {
		if strings.EqualFold(it.ContentHash, contentHash) {
			return it.RekorLogIndex, nil
		}
	}
	return 0, fmt.Errorf("publisher-attestation: no items[] entry with content_hash %s", contentHash)
}

// publisherOwnerRepoFromSourceURI extracts "owner/repo" from a GitHub
// source_uri. The MOAT manifest's source_uri is the content tarball — for
// GitHub-hosted publishers this is a /archive/ or release URL on github.com
// or api.github.com. The owner/repo prefix is the same for both.
//
// Examples:
//   - https://github.com/OWNER/REPO/archive/refs/tags/v1.0.tar.gz → OWNER/REPO
//   - https://github.com/OWNER/REPO/releases/download/v1/asset.tar.gz → OWNER/REPO
//   - https://api.github.com/repos/OWNER/REPO/tarball/SHA → OWNER/REPO
func publisherOwnerRepoFromSourceURI(sourceURI string) (string, error) {
	u, err := url.Parse(sourceURI)
	if err != nil {
		return "", fmt.Errorf("publisher-attestation: parse source_uri: %w", err)
	}
	if u.Scheme != "https" {
		return "", fmt.Errorf("publisher-attestation: source_uri must be https, got %q", u.Scheme)
	}
	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")

	switch u.Host {
	case "github.com":
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			return "", fmt.Errorf("publisher-attestation: cannot extract owner/repo from %s", sourceURI)
		}
		return parts[0] + "/" + parts[1], nil
	case "api.github.com":
		if len(parts) < 3 || parts[0] != "repos" || parts[1] == "" || parts[2] == "" {
			return "", fmt.Errorf("publisher-attestation: cannot extract owner/repo from %s", sourceURI)
		}
		return parts[1] + "/" + parts[2], nil
	default:
		return "", fmt.Errorf("publisher-attestation: source_uri host %q not supported (GitHub only for now)", u.Host)
	}
}
