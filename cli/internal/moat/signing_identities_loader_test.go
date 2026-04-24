package moat

// Tests for the bundled signing-identity allowlist (ADR 0007 slice-2a).
//
// Strategy: exercise parseSigningIdentities directly for edge cases
// (malformed JSON, missing fields, GitHub numeric-ID enforcement) so we
// don't have to mutate the embedded bytes. LookupSigningIdentity is
// covered with the real bundle to guarantee the shipped asset parses and
// the meta-registry entry round-trips through every URL variant we
// tolerate.

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestBundledAllowlist_Parses asserts the shipped allowlist is valid —
// a malformed commit to signing_identities.json would otherwise only
// surface at first production lookup.
func TestBundledAllowlist_Parses(t *testing.T) {
	t.Parallel()

	idx, err := parseSigningIdentities(bundledSigningIdentities)
	if err != nil {
		t.Fatalf("bundled allowlist must parse: %v", err)
	}
	if len(idx) == 0 {
		t.Fatal("bundled allowlist must have at least one entry")
	}
}

// TestBundledAllowlist_HasMetaRegistry asserts the slice-2a seed entry
// (OpenScribbler/syllago-meta-registry) is present with the numeric IDs
// the verifier requires under the GitHub Actions issuer.
func TestBundledAllowlist_HasMetaRegistry(t *testing.T) {
	t.Parallel()

	entry, ok := LookupSigningIdentity("https://github.com/OpenScribbler/syllago-meta-registry")
	if !ok {
		t.Fatal("expected meta-registry to be in the allowlist")
	}
	if entry.Profile.Issuer != GitHubActionsIssuer {
		t.Errorf("issuer = %q want %q", entry.Profile.Issuer, GitHubActionsIssuer)
	}
	if entry.Profile.SubjectRegex == "" {
		t.Error("meta-registry profile must populate subject_regex")
	}
	if entry.Profile.RepositoryID == "" || entry.Profile.RepositoryOwnerID == "" {
		t.Errorf("meta-registry profile must pin numeric IDs: repo_id=%q owner_id=%q",
			entry.Profile.RepositoryID, entry.Profile.RepositoryOwnerID)
	}
}

// TestLookupSigningIdentity_URLVariants asserts the normalization surface
// tolerates the URL shapes we expect from `syllago registry add` callers.
// Failing any of these would force users to retype the canonical form —
// the allowlist should forgive the paste, not the reviewer.
func TestLookupSigningIdentity_URLVariants(t *testing.T) {
	t.Parallel()

	variants := []string{
		"https://github.com/OpenScribbler/syllago-meta-registry",
		"https://github.com/OpenScribbler/syllago-meta-registry/",
		"https://github.com/OpenScribbler/syllago-meta-registry.git",
		"https://github.com/OpenScribbler/syllago-meta-registry.git/",
		"https://GitHub.com/OpenScribbler/syllago-meta-registry",
		"HTTPS://github.com/OpenScribbler/syllago-meta-registry",
		"https://github.com/OpenScribbler/syllago-meta-registry#readme",
		"https://github.com/OpenScribbler/syllago-meta-registry?ref=main",
		"  https://github.com/OpenScribbler/syllago-meta-registry  ",
	}
	for _, v := range variants {
		v := v
		t.Run(v, func(t *testing.T) {
			t.Parallel()
			if _, ok := LookupSigningIdentity(v); !ok {
				t.Errorf("expected match for %q", v)
			}
		})
	}
}

// TestLookupSigningIdentity_CaseSensitivePath asserts that owner/repo case
// is preserved — GitHub may redirect mixed-case URLs in the web UI but
// Git remotes are case-sensitive, and matching the wrong case would let
// an attacker register "openscribbler/syllago-meta-registry" (lowercase)
// and ride our allowlist entry.
func TestLookupSigningIdentity_CaseSensitivePath(t *testing.T) {
	t.Parallel()
	if _, ok := LookupSigningIdentity("https://github.com/openscribbler/syllago-meta-registry"); ok {
		t.Error("path casing must matter — lowercase owner must not match")
	}
}

// TestLookupSigningIdentity_Miss confirms non-allowlisted URLs return
// (nil, false) without panicking. The common case is every non-bundled
// registry, so the miss path is load-bearing.
func TestLookupSigningIdentity_Miss(t *testing.T) {
	t.Parallel()
	cases := []string{
		"https://github.com/other/some-registry",
		"https://example.com/fake/meta-registry",
		"",
		"   ",
		"not-a-url",
		"://broken",
	}
	for _, c := range cases {
		c := c
		t.Run(c, func(t *testing.T) {
			t.Parallel()
			if p, ok := LookupSigningIdentity(c); ok || p != nil {
				t.Errorf("expected miss, got ok=%v profile=%+v", ok, p)
			}
		})
	}
}

// TestLookupSigningIdentity_ResultIsCopy asserts callers mutating the
// returned profile don't corrupt the cached index for the next lookup.
// Slice-2b will likely hand the pointer off to config.Save which may
// zero or mutate fields during serialization.
func TestLookupSigningIdentity_ResultIsCopy(t *testing.T) {
	t.Parallel()
	e1, ok := LookupSigningIdentity("https://github.com/OpenScribbler/syllago-meta-registry")
	if !ok {
		t.Fatal("expected meta-registry match")
	}
	originalIssuer := e1.Profile.Issuer
	e1.Profile.Issuer = "https://evil.example.com/"

	e2, ok := LookupSigningIdentity("https://github.com/OpenScribbler/syllago-meta-registry")
	if !ok {
		t.Fatal("second lookup must also match")
	}
	if e2.Profile.Issuer != originalIssuer {
		t.Errorf("mutation leaked into cached index: got %q want %q", e2.Profile.Issuer, originalIssuer)
	}
}

// TestParseSigningIdentities_Valid builds a synthetic allowlist with both
// exact-subject and regex-subject entries and asserts both index properly.
func TestParseSigningIdentities_Valid(t *testing.T) {
	t.Parallel()

	raw := `{
		"_version": 1,
		"identities": [
			{
				"registry_url": "https://github.com/A/one",
				"profile": {
					"issuer": "https://token.actions.githubusercontent.com",
					"subject": "https://github.com/A/one/.github/workflows/x.yml@refs/heads/main",
					"repository_id": "1",
					"repository_owner_id": "2"
				}
			},
			{
				"registry_url": "https://gitlab.example.com/B/two",
				"profile": {
					"issuer": "https://gitlab.example.com",
					"subject_regex": "^https://gitlab\\.example\\.com/B/two/.*$"
				}
			}
		]
	}`

	idx, err := parseSigningIdentities([]byte(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(idx) != 2 {
		t.Errorf("expected 2 entries, got %d", len(idx))
	}
	if _, ok := idx["https://github.com/A/one"]; !ok {
		t.Error("missing exact-subject entry")
	}
	if _, ok := idx["https://gitlab.example.com/B/two"]; !ok {
		t.Error("missing regex-subject entry")
	}
}

// TestParseSigningIdentities_Errors covers every rejection path. Each
// case is a specific guarantee the allowlist loader makes — the error
// message should point at the offending entry and the failing field.
func TestParseSigningIdentities_Errors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		raw      string
		errMatch string
	}{
		{
			name:     "empty bytes",
			raw:      "",
			errMatch: "empty bundle",
		},
		{
			name:     "malformed JSON",
			raw:      "{not json",
			errMatch: "parse",
		},
		{
			name:     "wrong version",
			raw:      `{"_version": 2, "identities": []}`,
			errMatch: "unsupported _version",
		},
		{
			name: "missing version",
			raw:  `{"identities": []}`,
			// _version absent is treated as 0, which trips the version check.
			errMatch: "unsupported _version",
		},
		{
			name: "unparseable registry_url",
			raw: `{"_version": 1, "identities": [
				{"registry_url": "", "profile": {"issuer": "x", "subject": "y"}}
			]}`,
			errMatch: "registry_url",
		},
		{
			name: "missing issuer",
			raw: `{"_version": 1, "identities": [
				{"registry_url": "https://example.com/a/b", "profile": {"subject": "y"}}
			]}`,
			errMatch: "issuer is required",
		},
		{
			name: "missing subject AND subject_regex",
			raw: `{"_version": 1, "identities": [
				{"registry_url": "https://example.com/a/b", "profile": {"issuer": "x"}}
			]}`,
			errMatch: "subject or subject_regex is required",
		},
		{
			name: "invalid subject_regex",
			raw: `{"_version": 1, "identities": [
				{"registry_url": "https://example.com/a/b", "profile": {"issuer": "x", "subject_regex": "["}}
			]}`,
			errMatch: "subject_regex invalid",
		},
		{
			name: "invalid issuer_regex",
			raw: `{"_version": 1, "identities": [
				{"registry_url": "https://example.com/a/b", "profile": {"issuer": "x", "subject": "y", "issuer_regex": "(["}}
			]}`,
			errMatch: "issuer_regex invalid",
		},
		{
			name: "GitHub issuer without repository_id",
			raw: `{"_version": 1, "identities": [
				{"registry_url": "https://github.com/o/r", "profile": {
					"issuer": "https://token.actions.githubusercontent.com",
					"subject": "https://github.com/o/r/.github/workflows/x.yml@refs/heads/main",
					"repository_owner_id": "9"
				}}
			]}`,
			errMatch: "repository_id and repository_owner_id",
		},
		{
			name: "GitHub issuer without repository_owner_id",
			raw: `{"_version": 1, "identities": [
				{"registry_url": "https://github.com/o/r", "profile": {
					"issuer": "https://token.actions.githubusercontent.com",
					"subject": "https://github.com/o/r/.github/workflows/x.yml@refs/heads/main",
					"repository_id": "9"
				}}
			]}`,
			errMatch: "repository_id and repository_owner_id",
		},
		{
			name: "duplicate after normalization",
			raw: `{"_version": 1, "identities": [
				{"registry_url": "https://example.com/a/b.git", "profile": {"issuer": "x", "subject": "y"}},
				{"registry_url": "https://example.com/a/b/", "profile": {"issuer": "x", "subject": "y"}}
			]}`,
			errMatch: "duplicate registry_url",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseSigningIdentities([]byte(tc.raw))
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.errMatch)
			}
			if !strings.Contains(err.Error(), tc.errMatch) {
				t.Errorf("expected error containing %q, got %q", tc.errMatch, err.Error())
			}
		})
	}
}

// TestNormalizeRegistryURL_Cases pins the canonicalization rules the
// allowlist relies on. Changes here ripple through the lookup semantics,
// so each case is deliberate and the table reads as policy.
func TestNormalizeRegistryURL_Cases(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
	}{
		{"https://github.com/A/B", "https://github.com/A/B"},
		{"https://github.com/A/B/", "https://github.com/A/B"},
		{"https://github.com/A/B.git", "https://github.com/A/B"},
		{"https://github.com/A/B.git/", "https://github.com/A/B"},
		{"HTTPS://GitHub.com/A/B", "https://github.com/A/B"},
		{"https://github.com/A/B#readme", "https://github.com/A/B"},
		{"https://github.com/A/B?foo=bar", "https://github.com/A/B"},
		{"  https://github.com/A/B  ", "https://github.com/A/B"},
		{"", ""},
		{"   ", ""},
		{"not-a-url", ""},
		{"file:///local/path", ""}, // no host
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got := normalizeRegistryURL(tc.in)
			if got != tc.want {
				t.Errorf("normalizeRegistryURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestMustParseSigningIdentities_PanicsOnInvalid asserts the "refuse to
// boot" contract: if signing_identities.json is ever committed in a
// malformed state, the panic message should point engineers at the file.
func TestMustParseSigningIdentities_PanicsOnInvalid(t *testing.T) {
	t.Parallel()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("mustParseSigningIdentities must panic on invalid input")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic, got %T: %v", r, r)
		}
		if !strings.Contains(msg, "bundled signing identities malformed") {
			t.Errorf("panic message = %q, want substring %q", msg, "bundled signing identities malformed")
		}
	}()
	mustParseSigningIdentities([]byte("{not json"))
}

// TestMustParseSigningIdentities_ReturnsIndexOnValid is the happy-path
// counterpart — the helper is the wrapper both the sync.Once path and
// tests go through, so it must return the index unchanged on success.
func TestMustParseSigningIdentities_ReturnsIndexOnValid(t *testing.T) {
	t.Parallel()
	raw := `{"_version": 1, "identities": [
		{"registry_url": "https://example.com/a/b", "profile": {"issuer": "x", "subject": "y"}}
	]}`
	idx := mustParseSigningIdentities([]byte(raw))
	if len(idx) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(idx))
	}
	if _, ok := idx["https://example.com/a/b"]; !ok {
		t.Error("missing expected entry")
	}
}

// TestBundledAllowlist_IsValidJSON guards against commits that break the
// raw-JSON shape even before the loader's validation rules come into play.
// A plain JSON parse here gives a more actionable failure than a nested
// "parse: unexpected end of JSON" from the loader.
func TestBundledAllowlist_IsValidJSON(t *testing.T) {
	t.Parallel()
	var probe map[string]any
	if err := json.Unmarshal(bundledSigningIdentities, &probe); err != nil {
		t.Fatalf("signing_identities.json must be valid JSON: %v", err)
	}
}

// TestLookupSigningIdentity_MetaRegistryHasManifestURI asserts the bundled
// meta-registry entry carries a non-empty ManifestURI pointing to the
// moat-registry branch. This is the URI syllago writes into the registry
// config so moat.Sync can fetch and verify the signed manifest.
func TestLookupSigningIdentity_MetaRegistryHasManifestURI(t *testing.T) {
	t.Parallel()
	entry, ok := LookupSigningIdentity("https://github.com/OpenScribbler/syllago-meta-registry")
	if !ok {
		t.Fatal("expected allowlist match for meta-registry URL")
	}
	if entry.ManifestURI == "" {
		t.Error("expected non-empty ManifestURI for meta-registry allowlist entry")
	}
	wantPrefix := "https://raw.githubusercontent.com/OpenScribbler/syllago-meta-registry/"
	if !strings.HasPrefix(entry.ManifestURI, wantPrefix) {
		t.Errorf("ManifestURI %q does not start with expected prefix %q", entry.ManifestURI, wantPrefix)
	}
	if entry.Profile == nil {
		t.Error("expected non-nil Profile")
	}
}

// TestParseSigningIdentities_ManifestURIOptional asserts that entries without
// manifest_uri parse correctly — back-compat for allowlist entries that
// predate the field.
func TestParseSigningIdentities_ManifestURIOptional(t *testing.T) {
	t.Parallel()
	data := []byte(`{
		"_version": 1,
		"identities": [{
			"registry_url": "https://github.com/example/repo",
			"profile": {
				"issuer": "https://token.actions.githubusercontent.com",
				"subject": "https://github.com/example/repo/.github/workflows/moat.yml@refs/heads/main",
				"profile_version": 1,
				"repository_id": "12345",
				"repository_owner_id": "67890"
			}
		}]
	}`)
	idx, err := parseSigningIdentities(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entry := idx[normalizeRegistryURL("https://github.com/example/repo")]
	if entry == nil {
		t.Fatal("expected entry in index")
	}
	if entry.ManifestURI != "" {
		t.Errorf("expected empty ManifestURI for entry without manifest_uri field, got %q", entry.ManifestURI)
	}
}
