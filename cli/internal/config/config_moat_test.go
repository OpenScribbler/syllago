package config

import (
	"encoding/json"
	"testing"
	"time"
)

// TestRegistry_BackCompat_EmptyTypeIsGit verifies that configs written
// before the MOAT schema landed still load cleanly and report as git
// registries. This is the MOST important property — breaking existing
// user configs is not acceptable.
func TestRegistry_BackCompat_EmptyTypeIsGit(t *testing.T) {
	t.Parallel()

	// A registry entry as it would have looked before Type existed.
	legacy := `{
		"name": "my-reg",
		"url": "https://github.com/acme/tools.git",
		"ref": "main",
		"trust": "community",
		"visibility": "public"
	}`

	var r Registry
	if err := json.Unmarshal([]byte(legacy), &r); err != nil {
		t.Fatalf("failed to parse legacy config: %v", err)
	}
	if !r.IsGit() {
		t.Error("legacy (empty Type) registry should report IsGit() = true")
	}
	if r.IsMOAT() {
		t.Error("legacy (empty Type) registry should NOT report IsMOAT() = true")
	}
	if r.Type != "" {
		t.Errorf("Type = %q; want empty string for legacy config", r.Type)
	}
}

// TestRegistry_ExplicitGit matches IsGit() for entries that explicitly
// record Type="git" (future configs written by the MOAT-aware CLI).
func TestRegistry_ExplicitGit(t *testing.T) {
	t.Parallel()

	r := Registry{Type: RegistryTypeGit}
	if !r.IsGit() {
		t.Error("Type=git should report IsGit() = true")
	}
	if r.IsMOAT() {
		t.Error("Type=git should NOT report IsMOAT() = true")
	}
}

// TestRegistry_MOATRoundTrip covers the MOAT-specific fields through a
// full JSON roundtrip — every field must survive Marshal → Unmarshal
// without loss.
func TestRegistry_MOATRoundTrip(t *testing.T) {
	t.Parallel()

	fetched := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	profile := SigningProfile{
		Issuer:  "https://token.actions.githubusercontent.com",
		Subject: "https://github.com/acme/registry/.github/workflows/publish.yml@refs/heads/main",
	}
	original := Registry{
		Name:           "acme-moat",
		URL:            "https://registry.example.com",
		Type:           RegistryTypeMOAT,
		ManifestURI:    "https://registry.example.com/manifest.json",
		SigningProfile: &profile,
		LastFetchedAt:  &fetched,
		Operator:       "Acme Registry Operations",
		ManifestETag:   `"v42"`,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var restored Registry
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if restored.Type != RegistryTypeMOAT {
		t.Errorf("Type = %q; want %q", restored.Type, RegistryTypeMOAT)
	}
	if restored.ManifestURI != original.ManifestURI {
		t.Errorf("ManifestURI = %q; want %q", restored.ManifestURI, original.ManifestURI)
	}
	if restored.SigningProfile == nil || !restored.SigningProfile.Equal(*original.SigningProfile) {
		t.Errorf("SigningProfile not preserved: got %+v want %+v", restored.SigningProfile, original.SigningProfile)
	}
	if restored.LastFetchedAt == nil || !restored.LastFetchedAt.Equal(fetched) {
		t.Errorf("LastFetchedAt not preserved: got %v want %v", restored.LastFetchedAt, fetched)
	}
	if restored.Operator != original.Operator {
		t.Errorf("Operator = %q; want %q", restored.Operator, original.Operator)
	}
	if restored.ManifestETag != original.ManifestETag {
		t.Errorf("ManifestETag = %q; want %q", restored.ManifestETag, original.ManifestETag)
	}
}

// TestRegistry_MOATFieldsOmittedFromGitConfig verifies that a git
// registry does not emit zero-valued MOAT fields in its JSON — readers
// of pre-MOAT configs and downstream tools must not see noise.
func TestRegistry_MOATFieldsOmittedFromGitConfig(t *testing.T) {
	t.Parallel()

	r := Registry{
		Name: "g",
		URL:  "https://github.com/acme/tools.git",
		Type: RegistryTypeGit,
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}

	s := string(data)
	// manifest_uri, signing_profile, last_fetched_at, operator, manifest_etag
	// must all omit when zero.
	for _, banned := range []string{
		`"manifest_uri"`,
		`"signing_profile"`,
		`"last_fetched_at"`,
		`"operator"`,
		`"manifest_etag"`,
	} {
		if containsSubstring(s, banned) {
			t.Errorf("git registry JSON unexpectedly contains %s: %s", banned, s)
		}
	}
}

func containsSubstring(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// TestRegistry_TrustedRootRoundTrip exercises the ADR 0007 forward-compat
// field. We care that the value persists, that the zero value omits from
// JSON (so existing configs don't pick up trailing noise on save), and that
// slice-1 consumers get the same string back they wrote.
func TestRegistry_TrustedRootRoundTrip(t *testing.T) {
	t.Parallel()

	r := Registry{
		Name:        "acme",
		URL:         "https://registry.example.com",
		Type:        RegistryTypeMOAT,
		ManifestURI: "https://registry.example.com/manifest.json",
		TrustedRoot: "/etc/syllago/corp-trusted-root.json",
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !containsSubstring(string(data), `"trusted_root":"/etc/syllago/corp-trusted-root.json"`) {
		t.Errorf("trusted_root must serialize when populated, got: %s", data)
	}

	var restored Registry
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if restored.TrustedRoot != r.TrustedRoot {
		t.Errorf("TrustedRoot not preserved: got %q want %q", restored.TrustedRoot, r.TrustedRoot)
	}

	// Empty value must omit entirely. Stale configs that save-and-reload
	// must not grow a trusted_root:"" field.
	empty := Registry{Name: "g", URL: "u", Type: RegistryTypeGit}
	emptyData, _ := json.Marshal(empty)
	if containsSubstring(string(emptyData), `"trusted_root"`) {
		t.Errorf("empty trusted_root must be omitted, got: %s", emptyData)
	}
}

// TestSigningProfile_NumericIDsRoundTrip — the repository_id /
// repository_owner_id / profile_version / regex fields must persist
// through a Marshal/Unmarshal cycle. This is the forward-compat capture
// path — if these silently drop, TOFU pinning is broken.
func TestSigningProfile_NumericIDsRoundTrip(t *testing.T) {
	t.Parallel()

	orig := SigningProfile{
		Issuer:            "https://token.actions.githubusercontent.com",
		Subject:           "https://github.com/acme/reg/.github/workflows/pub.yml@refs/heads/main",
		ProfileVersion:    1,
		SubjectRegex:      ".+/pub\\.yml@.+",
		IssuerRegex:       "^https://.+",
		RepositoryID:      "987654321",
		RepositoryOwnerID: "123456789",
	}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var restored SigningProfile
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if restored.RepositoryID != orig.RepositoryID {
		t.Errorf("RepositoryID = %q; want %q", restored.RepositoryID, orig.RepositoryID)
	}
	if restored.RepositoryOwnerID != orig.RepositoryOwnerID {
		t.Errorf("RepositoryOwnerID = %q; want %q", restored.RepositoryOwnerID, orig.RepositoryOwnerID)
	}
	if restored.ProfileVersion != orig.ProfileVersion {
		t.Errorf("ProfileVersion = %d; want %d", restored.ProfileVersion, orig.ProfileVersion)
	}
	if restored.SubjectRegex != orig.SubjectRegex {
		t.Errorf("SubjectRegex = %q; want %q", restored.SubjectRegex, orig.SubjectRegex)
	}
	if restored.IssuerRegex != orig.IssuerRegex {
		t.Errorf("IssuerRegex = %q; want %q", restored.IssuerRegex, orig.IssuerRegex)
	}

	// Empty extension fields must omit from JSON (keep legacy configs clean).
	legacy := SigningProfile{Issuer: "i", Subject: "s"}
	legacyData, _ := json.Marshal(legacy)
	for _, banned := range []string{
		`"profile_version"`,
		`"subject_regex"`,
		`"issuer_regex"`,
		`"repository_id"`,
		`"repository_owner_id"`,
	} {
		if containsSubstring(string(legacyData), banned) {
			t.Errorf("legacy-shape SigningProfile JSON must omit %s, got: %s", banned, legacyData)
		}
	}
}

// TestSigningProfile_IsZero covers the tri-state logic: unset (zero),
// partially set (either field empty), and fully populated.
func TestSigningProfile_IsZero(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		sp   SigningProfile
		want bool
	}{
		{"both_empty", SigningProfile{}, true},
		{"only_issuer", SigningProfile{Issuer: "x"}, false},
		{"only_subject", SigningProfile{Subject: "y"}, false},
		{"both_set", SigningProfile{Issuer: "x", Subject: "y"}, false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.sp.IsZero(); got != tt.want {
				t.Errorf("IsZero() = %v; want %v", got, tt.want)
			}
		})
	}
}

// TestSigningProfile_Equal requires issuer+subject+numeric-IDs to match.
// Regexes and ProfileVersion are metadata, not identity — they do not
// participate in equality. A TOFU profile that later gains numeric IDs is
// treated as different from the original per ADR 0007, to force re-approval
// when the trust anchor expands.
func TestSigningProfile_Equal(t *testing.T) {
	t.Parallel()

	base := SigningProfile{
		Issuer:            "iss",
		Subject:           "sub",
		RepositoryID:      "111",
		RepositoryOwnerID: "222",
	}

	tests := []struct {
		name  string
		other SigningProfile
		want  bool
	}{
		{
			"same_all_fields",
			SigningProfile{Issuer: "iss", Subject: "sub", RepositoryID: "111", RepositoryOwnerID: "222"},
			true,
		},
		{
			"issuer_differs",
			SigningProfile{Issuer: "other", Subject: "sub", RepositoryID: "111", RepositoryOwnerID: "222"},
			false,
		},
		{
			"subject_differs",
			SigningProfile{Issuer: "iss", Subject: "other", RepositoryID: "111", RepositoryOwnerID: "222"},
			false,
		},
		{
			"repo_id_differs",
			SigningProfile{Issuer: "iss", Subject: "sub", RepositoryID: "999", RepositoryOwnerID: "222"},
			false,
		},
		{
			"owner_id_differs",
			SigningProfile{Issuer: "iss", Subject: "sub", RepositoryID: "111", RepositoryOwnerID: "999"},
			false,
		},
		{
			"tofu_becomes_pinned",
			// Issuer+subject identical, but one side captured numeric IDs and
			// the other didn't. Per ADR 0007 this is a re-approval event.
			SigningProfile{Issuer: "iss", Subject: "sub"},
			false,
		},
		{
			"regex_metadata_ignored",
			// Regexes are relaxation knobs, not identity — they do not break equality.
			SigningProfile{
				Issuer:            "iss",
				Subject:           "sub",
				RepositoryID:      "111",
				RepositoryOwnerID: "222",
				SubjectRegex:      ".+",
				IssuerRegex:       ".+",
				ProfileVersion:    1,
			},
			true,
		},
		{
			"both_empty_vs_populated",
			SigningProfile{},
			false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := base.Equal(tt.other); got != tt.want {
				t.Errorf("Equal() = %v; want %v", got, tt.want)
			}
		})
	}
}

// TestSigningProfile_AuthorizesIdentity covers the regex-aware authorization
// semantic used by NeedsSigningProfileReapproval. The bundled allowlist
// uses regex form to span several workflow paths under one trust grant —
// the wire manifest's literal subject must be authorized by that regex
// without forcing re-approval.
func TestSigningProfile_AuthorizesIdentity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pinned   SigningProfile
		incoming SigningProfile
		want     bool
	}{
		{
			"literal_match",
			SigningProfile{Issuer: "iss", Subject: "sub", RepositoryID: "1", RepositoryOwnerID: "2"},
			SigningProfile{Issuer: "iss", Subject: "sub", RepositoryID: "1", RepositoryOwnerID: "2"},
			true,
		},
		{
			"literal_subject_differs",
			SigningProfile{Issuer: "iss", Subject: "sub-a", RepositoryID: "1", RepositoryOwnerID: "2"},
			SigningProfile{Issuer: "iss", Subject: "sub-b", RepositoryID: "1", RepositoryOwnerID: "2"},
			false,
		},
		{
			"regex_pinned_authorizes_literal_match",
			SigningProfile{
				Issuer:            "iss",
				Subject:           "",
				SubjectRegex:      `^https://github\.com/Org/repo/\.github/workflows/(a|b|c)\.yml@refs/heads/main$`,
				RepositoryID:      "1",
				RepositoryOwnerID: "2",
			},
			SigningProfile{
				Issuer:            "iss",
				Subject:           "https://github.com/Org/repo/.github/workflows/b.yml@refs/heads/main",
				RepositoryID:      "1",
				RepositoryOwnerID: "2",
			},
			true,
		},
		{
			"regex_pinned_rejects_non_match",
			SigningProfile{
				Issuer:            "iss",
				SubjectRegex:      `^https://github\.com/Org/repo/\.github/workflows/(a|b|c)\.yml@refs/heads/main$`,
				RepositoryID:      "1",
				RepositoryOwnerID: "2",
			},
			SigningProfile{
				Issuer:            "iss",
				Subject:           "https://github.com/Other/repo/.github/workflows/a.yml@refs/heads/main",
				RepositoryID:      "1",
				RepositoryOwnerID: "2",
			},
			false,
		},
		{
			"regex_pinned_authorizes_same_regex_wire",
			SigningProfile{
				Issuer:            "iss",
				SubjectRegex:      `^.+@main$`,
				RepositoryID:      "1",
				RepositoryOwnerID: "2",
			},
			SigningProfile{
				Issuer:            "iss",
				SubjectRegex:      `^.+@main$`,
				RepositoryID:      "1",
				RepositoryOwnerID: "2",
			},
			true,
		},
		{
			"regex_pinned_invalid_regex_fails_closed",
			SigningProfile{
				Issuer:            "iss",
				SubjectRegex:      `[invalid(`,
				RepositoryID:      "1",
				RepositoryOwnerID: "2",
			},
			SigningProfile{
				Issuer:            "iss",
				Subject:           "anything",
				RepositoryID:      "1",
				RepositoryOwnerID: "2",
			},
			false,
		},
		{
			"repo_id_conflict_when_both_set",
			SigningProfile{
				Issuer:            "iss",
				SubjectRegex:      `.+`,
				RepositoryID:      "1",
				RepositoryOwnerID: "2",
			},
			SigningProfile{
				Issuer:            "iss",
				Subject:           "anything",
				RepositoryID:      "999",
				RepositoryOwnerID: "2",
			},
			false,
		},
		{
			"owner_id_conflict_when_both_set",
			SigningProfile{
				Issuer:            "iss",
				SubjectRegex:      `.+`,
				RepositoryID:      "1",
				RepositoryOwnerID: "2",
			},
			SigningProfile{
				Issuer:            "iss",
				Subject:           "anything",
				RepositoryID:      "1",
				RepositoryOwnerID: "999",
			},
			false,
		},
		{
			// OpenScribbler real-world case: bundled allowlist pins numeric IDs;
			// the wire manifest declares only issuer+subject (no IDs). Pinned
			// strictness wins — cert verification still applies the pinned IDs
			// against the cert's OIDC extensions. Authorized.
			"pinned_ids_wire_empty_is_authorized",
			SigningProfile{
				Issuer:            "iss",
				SubjectRegex:      `^https://github\.com/Org/repo/\.github/workflows/(a|b|c)\.yml@refs/heads/main$`,
				RepositoryID:      "1193220959",
				RepositoryOwnerID: "263775997",
			},
			SigningProfile{
				Issuer:  "iss",
				Subject: "https://github.com/Org/repo/.github/workflows/a.yml@refs/heads/main",
			},
			true,
		},
		{
			// ADR 0007: bumping from TOFU (no IDs) to pinned IDs is a tightening
			// of the trust grant the user did not consent to. Re-approval required.
			"wire_adds_ids_pinned_empty_is_reapproval",
			SigningProfile{
				Issuer:  "iss",
				Subject: "sub",
			},
			SigningProfile{
				Issuer:            "iss",
				Subject:           "sub",
				RepositoryID:      "1",
				RepositoryOwnerID: "2",
			},
			false,
		},
		{
			"issuer_regex_authorizes_literal",
			SigningProfile{
				IssuerRegex:       `^https://token\.actions\.githubusercontent\.com$`,
				Subject:           "sub",
				RepositoryID:      "1",
				RepositoryOwnerID: "2",
			},
			SigningProfile{
				Issuer:            "https://token.actions.githubusercontent.com",
				Subject:           "sub",
				RepositoryID:      "1",
				RepositoryOwnerID: "2",
			},
			true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.pinned.AuthorizesIdentity(tt.incoming); got != tt.want {
				t.Errorf("AuthorizesIdentity() = %v; want %v", got, tt.want)
			}
		})
	}
}

// TestRegistry_NeedsSigningProfileReapproval is the core TOFU-vs-reapproval
// decision. Document the four states explicitly so the test reads like a
// requirements table.
func TestRegistry_NeedsSigningProfileReapproval(t *testing.T) {
	t.Parallel()

	recorded := SigningProfile{Issuer: "iss", Subject: "sub"}
	other := SigningProfile{Issuer: "iss", Subject: "DIFFERENT"}

	tests := []struct {
		name     string
		current  *SigningProfile
		incoming SigningProfile
		want     bool
		reason   string
	}{
		{
			name:     "first_time_TOFU_nil",
			current:  nil,
			incoming: recorded,
			want:     false,
			reason:   "no prior profile (nil) → TOFU prompt, NOT a re-approval",
		},
		{
			name:     "first_time_TOFU_zero",
			current:  &SigningProfile{},
			incoming: recorded,
			want:     false,
			reason:   "zero-valued profile → TOFU prompt, NOT a re-approval",
		},
		{
			name:     "same_profile",
			current:  &recorded,
			incoming: recorded,
			want:     false,
			reason:   "unchanged profile never requires re-approval",
		},
		{
			name:     "changed_profile",
			current:  &recorded,
			incoming: other,
			want:     true,
			reason:   "issuer OR subject changed → re-approval required",
		},
		{
			name: "regex_pinned_matches_literal_wire",
			current: &SigningProfile{
				Issuer:            "iss",
				SubjectRegex:      `^https://github\.com/Org/repo/\.github/workflows/(a|b|c)\.yml@refs/heads/main$`,
				RepositoryID:      "1",
				RepositoryOwnerID: "2",
			},
			incoming: SigningProfile{
				Issuer:            "iss",
				Subject:           "https://github.com/Org/repo/.github/workflows/b.yml@refs/heads/main",
				RepositoryID:      "1",
				RepositoryOwnerID: "2",
			},
			want:   false,
			reason: "regex-pinned profile authorizes wire literal that matches → no re-approval",
		},
		{
			// OpenScribbler real-world: bundled allowlist pins regex+IDs; wire
			// declares only literal issuer+subject. No re-approval (regression: bug-445).
			name: "openscribbler_real_world_no_reapproval",
			current: &SigningProfile{
				Issuer:            "https://token.actions.githubusercontent.com",
				SubjectRegex:      `^https://github\.com/OpenScribbler/syllago-meta-registry/\.github/workflows/(moat|moat-publisher|moat-registry)\.yml@refs/heads/(main|master)$`,
				RepositoryID:      "1193220959",
				RepositoryOwnerID: "263775997",
			},
			incoming: SigningProfile{
				Issuer:  "https://token.actions.githubusercontent.com",
				Subject: "https://github.com/OpenScribbler/syllago-meta-registry/.github/workflows/moat-registry.yml@refs/heads/master",
			},
			want:   false,
			reason: "bundled allowlist pin (regex + IDs) authorizes wire literal-only profile (regression: bug-445)",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := &Registry{SigningProfile: tt.current}
			if got := r.NeedsSigningProfileReapproval(tt.incoming); got != tt.want {
				t.Errorf("NeedsSigningProfileReapproval() = %v; want %v (%s)", got, tt.want, tt.reason)
			}
		})
	}
}

// TestRegistry_NameOperatorChangeDoesNotTriggerReapproval captures the
// spec invariant that display-label changes are unrelated to trust. A
// registry that renames itself ("Acme" → "Acme Tools") or swaps operators
// MUST still sync without a re-approval prompt as long as the signing
// profile is unchanged.
func TestRegistry_NameOperatorChangeDoesNotTriggerReapproval(t *testing.T) {
	t.Parallel()

	profile := SigningProfile{Issuer: "iss", Subject: "sub"}
	r := &Registry{
		Name:           "Old Name",
		Operator:       "Old Operator",
		SigningProfile: &profile,
	}

	// Simulate fetching a manifest where name + operator changed, profile didn't.
	if r.NeedsSigningProfileReapproval(profile) {
		t.Error("name/operator change with unchanged profile must not require re-approval")
	}
}
