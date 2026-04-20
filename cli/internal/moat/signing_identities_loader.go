package moat

// Bundled allowlist of known-good signing identities per ADR 0007 slice-2a.
//
// The allowlist exists so syllago can verify its own meta-registry (and
// future "trusted" registries) with zero configuration at first run — no
// TOFU prompt, no --signing-identity flags. Operators who add other MOAT
// registries fall through to the slice-2b CLI-flag / TOFU paths.
//
// Why bundle, not fetch:
//   1. syllago's binary already carries the Sigstore trusted_root.json for
//      the same reproducibility reason. Carrying the allowlist alongside it
//      means a single signed release pins the full trust surface — root
//      keys + known identities — at that release's SHA.
//   2. Fetching the allowlist live would add a chicken-and-egg problem:
//      verifying that fetch requires trusting an identity before the
//      allowlist that names the identity is loaded.
//
// The cost is that adding a new known-good registry requires a syllago
// release. That's intentional — the bar for shipping in the allowlist is
// higher than for landing in a community registry, and each addition is a
// reviewed code change with its own commit SHA and release notes.
//
// Refresh procedure:
//   1. Add the entry to signing_identities.json with the issuer, subject
//      (or subject_regex), and numeric repository_id / repository_owner_id.
//      The numeric IDs come from `gh api repos/OWNER/REPO --jq '.id, .owner.id'`.
//   2. Run the tests — malformed JSON is a hard failure at first use.
//   3. Cross-reference the addition from ADR 0007 and the docs page named in
//      slice-2b's error text.

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/OpenScribbler/syllago/cli/internal/config"
)

// bundledSigningIdentities is the verbatim JSON allowlist committed in this
// repository. See signing_identities.json for the source of truth.
//
//go:embed signing_identities.json
var bundledSigningIdentities []byte

// SigningIdentitiesVersion is the supported schema version for the embedded
// allowlist. A bump requires a loader update — refusing to boot on unknown
// versions prevents silent downgrade of allowlist semantics.
const SigningIdentitiesVersion = 1

// signingIdentityEntry is the on-disk JSON shape of a single allowlist entry.
// The Profile field deserializes straight into config.SigningProfile so the
// lookup result is already in the shape slice-2b writes into the Registry
// record.
type signingIdentityEntry struct {
	RegistryURL string                `json:"registry_url"`
	Description string                `json:"description,omitempty"`
	Profile     config.SigningProfile `json:"profile"`
}

// signingIdentitiesDoc is the on-disk JSON shape of the full allowlist file.
type signingIdentitiesDoc struct {
	Version     int                    `json:"_version"`
	Description string                 `json:"description,omitempty"`
	Identities  []signingIdentityEntry `json:"identities"`
}

var (
	identityIndex     map[string]*config.SigningProfile
	identityIndexOnce sync.Once
)

// LookupSigningIdentity reports the known-good signing profile for the given
// registry URL, if one is bundled. The returned profile is a fresh copy
// callers may mutate without affecting future lookups.
//
// URL matching is case-insensitive on scheme+host and tolerates:
//   - trailing ".git" (git-style remotes)
//   - trailing "/"
//   - URL fragment and query (both dropped before lookup)
//
// Returns (nil, false) when no match is found. Slice-2b callers then fall
// back to explicit CLI flags, and finally to TOFU or hard-fail.
//
// First call parses and validates the embedded JSON; malformed bundle
// panics with a descriptive message. Subsequent calls are lock-free.
func LookupSigningIdentity(registryURL string) (*config.SigningProfile, bool) {
	idx := loadSigningIdentityIndex()
	key := normalizeRegistryURL(registryURL)
	if key == "" {
		return nil, false
	}
	p, ok := idx[key]
	if !ok {
		return nil, false
	}
	clone := *p
	return &clone, true
}

// loadSigningIdentityIndex lazily parses the embedded allowlist on first
// call. Parse failure panics rather than returning an error — the embedded
// bytes are a build-time asset, so a malformed value is a build bug that
// CI and unit tests must catch, not a runtime condition callers handle.
func loadSigningIdentityIndex() map[string]*config.SigningProfile {
	identityIndexOnce.Do(func() {
		identityIndex = mustParseSigningIdentities(bundledSigningIdentities)
	})
	return identityIndex
}

// mustParseSigningIdentities is the panic-on-failure wrapper around
// parseSigningIdentities. Extracted so tests can exercise the panic text
// directly without fighting the package-level sync.Once.
func mustParseSigningIdentities(data []byte) map[string]*config.SigningProfile {
	idx, err := parseSigningIdentities(data)
	if err != nil {
		panic(fmt.Sprintf("moat: bundled signing identities malformed: %v", err))
	}
	return idx
}

// parseSigningIdentities converts the raw JSON bytes into a normalized-URL
// lookup map. Exposed for tests that exercise the malformed-input paths
// without having to swap the embedded bytes at runtime.
func parseSigningIdentities(data []byte) (map[string]*config.SigningProfile, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty bundle")
	}

	var doc signingIdentitiesDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	if doc.Version != SigningIdentitiesVersion {
		return nil, fmt.Errorf("unsupported _version: got %d want %d", doc.Version, SigningIdentitiesVersion)
	}

	idx := make(map[string]*config.SigningProfile, len(doc.Identities))
	for i, e := range doc.Identities {
		key := normalizeRegistryURL(e.RegistryURL)
		if key == "" {
			return nil, fmt.Errorf("entry %d: registry_url %q is empty or unparseable", i, e.RegistryURL)
		}
		if e.Profile.Issuer == "" {
			return nil, fmt.Errorf("entry %d (%s): issuer is required", i, key)
		}
		if e.Profile.Subject == "" && e.Profile.SubjectRegex == "" {
			return nil, fmt.Errorf("entry %d (%s): subject or subject_regex is required", i, key)
		}
		if e.Profile.SubjectRegex != "" {
			if _, err := regexp.Compile(e.Profile.SubjectRegex); err != nil {
				return nil, fmt.Errorf("entry %d (%s): subject_regex invalid: %w", i, key, err)
			}
		}
		if e.Profile.IssuerRegex != "" {
			if _, err := regexp.Compile(e.Profile.IssuerRegex); err != nil {
				return nil, fmt.Errorf("entry %d (%s): issuer_regex invalid: %w", i, key, err)
			}
		}
		// GitHub Actions issuer requires numeric-ID binding per ADR 0007 —
		// refusing allowlist entries without it closes the repo-transfer
		// forgery vector before slice-2b can write the profile to config.
		if e.Profile.Issuer == GitHubActionsIssuer {
			if e.Profile.RepositoryID == "" || e.Profile.RepositoryOwnerID == "" {
				return nil, fmt.Errorf("entry %d (%s): GitHub Actions issuer requires repository_id and repository_owner_id", i, key)
			}
		}
		if _, dup := idx[key]; dup {
			return nil, fmt.Errorf("entry %d (%s): duplicate registry_url after normalization", i, key)
		}
		p := e.Profile
		idx[key] = &p
	}
	return idx, nil
}

// normalizeRegistryURL reduces a registry URL to its canonical lookup key.
// Returns empty string if the URL cannot be parsed or is missing the scheme
// or host. Normalization rules:
//
//   - whitespace is trimmed
//   - scheme and host are lowercased
//   - fragment and query are dropped
//   - trailing ".git" on the path is stripped (git-style remotes)
//   - trailing "/" on the path is stripped
//
// Path segments retain their original case — GitHub is case-preserving on
// owner and repo names, and Git remote URLs are case-sensitive even when
// the web UI redirects mixed-case URLs.
func normalizeRegistryURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Host)
	if scheme == "" || host == "" {
		return ""
	}
	path := strings.TrimSuffix(u.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	return scheme + "://" + host + path
}
