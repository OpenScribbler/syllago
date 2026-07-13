package capfeed

// Fail-closed SLSA provenance verification for the Capability Feed
// (ADR 0015). GitHub's attestations API serves Sigstore v0.3-family bundles
// containing DSSE envelopes with in-toto SLSA provenance statements;
// sigstore-go verifies these natively through the same verifier-construction
// pattern as moat.VerifyManifest (cli/internal/moat/manifest_verify.go).
// MOAT itself has no DSSE handling and gains none — this file is the
// in-repo reference if it ever needs one.

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	sgbundle "github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

// The pinned publisher identity. The Capability Feed's publisher is known at
// compile time, so there is no first-contact (TOFU) problem: only bundles
// whose Fulcio certificate carries exactly this workflow identity verify.
// The subject is the certificate SAN form, confirmed against a live bundle.
const (
	feedSignerSubject = "https://github.com/OpenScribbler/capmon/.github/workflows/publish.yml@refs/heads/main"
	feedSignerIssuer  = "https://token.actions.githubusercontent.com"
)

// slsaProvenanceV1 is the only DSSE predicate type accepted for the feed.
// The signature proves who signed which bytes; the predicate type is what
// makes the statement a SLSA provenance claim rather than, say, an SBOM the
// same workflow might someday also attest for the same artifact.
const slsaProvenanceV1 = "https://slsa.dev/provenance/v1"

// attestationsAPIBaseURL is a package var so tests can point it at an
// httptest server (same seam pattern as updater.githubAPIURL).
var attestationsAPIBaseURL = "https://api.github.com/repos/OpenScribbler/capmon/attestations/"

// SetAttestationsAPIBaseURLForTest overrides the attestations API base URL
// and returns a restore func. Test-only.
func SetAttestationsAPIBaseURLForTest(url string) (restore func()) {
	prev := attestationsAPIBaseURL
	attestationsAPIBaseURL = url
	return func() { attestationsAPIBaseURL = prev }
}

// attestationsResponse mirrors the GitHub REST shape:
// {"attestations": [{"bundle": {...}}, ...]}
type attestationsResponse struct {
	Attestations []struct {
		Bundle json.RawMessage `json:"bundle"`
	} `json:"attestations"`
}

// FetchAttestationBundle fetches every Sigstore bundle recorded for the
// given artifact digest from GitHub's public attestations API. The call is
// deliberately unauthenticated (public repo, no token, no gh binary) so it
// runs identically in CI and on WSL. An empty result is an error — a feed
// without provenance must never verify.
func FetchAttestationBundle(ctx context.Context, client *http.Client, sha256Hex string) ([][]byte, error) {
	if sha256Hex == "" {
		return nil, errors.New("capfeed attest: digest is required")
	}
	if client == nil {
		client = &http.Client{Timeout: DefaultFetchTimeout}
	}

	url := attestationsAPIBaseURL + "sha256:" + sha256Hex
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("capfeed attest %s: build request: %w", url, err)
	}
	// GitHub requires a User-Agent header; requests without one get a 403.
	req.Header.Set("User-Agent", DefaultUserAgent)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("capfeed attest %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		return nil, fmt.Errorf("capfeed attest %s: unexpected status %d %s",
			url, resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxFeedBytes+1))
	if err != nil {
		return nil, fmt.Errorf("capfeed attest %s: reading body: %w", url, err)
	}
	if len(body) > MaxFeedBytes {
		return nil, fmt.Errorf("capfeed attest %s: response exceeds %d-byte cap", url, MaxFeedBytes)
	}

	var parsed attestationsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("capfeed attest %s: decode: %w", url, err)
	}
	if len(parsed.Attestations) == 0 {
		return nil, fmt.Errorf("capfeed attest: no attestation recorded for sha256:%s", sha256Hex)
	}

	bundles := make([][]byte, 0, len(parsed.Attestations))
	for _, a := range parsed.Attestations {
		if len(a.Bundle) == 0 {
			continue
		}
		bundles = append(bundles, []byte(a.Bundle))
	}
	if len(bundles) == 0 {
		return nil, fmt.Errorf("capfeed attest: attestations for sha256:%s carry no bundles", sha256Hex)
	}
	return bundles, nil
}

// VerifyFeedProvenance is the fail-closed gate: it returns nil only when at
// least one bundle verifies the index bytes as SLSA provenance signed by the
// pinned capmon publish workflow, with a Rekor transparency-log entry and an
// integrated timestamp, chained to the supplied trusted root. Any other
// outcome is an error and the caller must trust nothing from the feed.
func VerifyFeedProvenance(indexBytes []byte, bundles [][]byte, trustedRootJSON []byte) error {
	if len(bundles) == 0 {
		return errors.New("capfeed verify: no attestation bundle to verify (feed unsigned?)")
	}
	var lastErr error
	for i, raw := range bundles {
		if err := verifyWithIdentity(indexBytes, raw, trustedRootJSON, feedSignerSubject, feedSignerIssuer); err != nil {
			lastErr = fmt.Errorf("capfeed verify: bundle %d: %w", i, err)
			continue
		}
		return nil
	}
	return lastErr
}

// verifyWithIdentity runs the sigstore-go verification of indexBytes against
// one candidate bundle with the given pinned identity. Split out so tests
// can exercise identity mismatch without mocks (the pinned constants stay
// out of the signature elsewhere).
func verifyWithIdentity(indexBytes, bundleBytes, trustedRootJSON []byte, subject, issuer string) error {
	if len(indexBytes) == 0 {
		return errors.New("capfeed verify: index bytes empty")
	}
	if len(trustedRootJSON) == 0 {
		return errors.New("capfeed verify: trusted root bytes empty")
	}
	if len(bundleBytes) == 0 {
		return errors.New("capfeed verify: bundle bytes empty")
	}

	tr, err := root.NewTrustedRootFromJSON(trustedRootJSON)
	if err != nil {
		return fmt.Errorf("parsing trusted root: %w", err)
	}

	b := &sgbundle.Bundle{}
	if err := b.UnmarshalJSON(bundleBytes); err != nil {
		return fmt.Errorf("parsing sigstore bundle: %w", err)
	}

	// Require a transparency-log entry (Rekor inclusion) and an integrated
	// timestamp binding the signature to a point inside cert validity —
	// same posture as moat.VerifyManifest.
	sev, err := verify.NewVerifier(tr,
		verify.WithTransparencyLog(1),
		verify.WithIntegratedTimestamps(1),
	)
	if err != nil {
		return fmt.Errorf("constructing verifier: %w", err)
	}

	certID, err := verify.NewShortCertificateIdentity(issuer, "", subject, "")
	if err != nil {
		return fmt.Errorf("building certificate identity matcher: %w", err)
	}

	// The DSSE statement's subject is a digest, not raw artifact bytes, so
	// the policy matches on the sha256 of the exact index bytes fetched.
	digest := sha256.Sum256(indexBytes)
	policy := verify.NewPolicy(
		verify.WithArtifactDigest("sha256", digest[:]),
		verify.WithCertificateIdentity(certID),
	)

	res, err := sev.Verify(b, policy)
	if err != nil {
		return err
	}
	return checkProvenancePredicate(res)
}

// checkProvenancePredicate confirms the verified DSSE statement actually
// carries SLSA provenance. Runs strictly after signature verification —
// the statement's contents are untrusted until then.
func checkProvenancePredicate(res *verify.VerificationResult) error {
	if res == nil || res.Statement == nil {
		return errors.New("verified bundle carries no in-toto statement")
	}
	if pt := res.Statement.PredicateType; pt != slsaProvenanceV1 {
		return fmt.Errorf("statement predicate type is %q; require %q", pt, slsaProvenanceV1)
	}
	return nil
}
