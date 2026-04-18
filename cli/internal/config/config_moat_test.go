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

// TestSigningProfile_Equal requires BOTH fields to match — the profile is
// a pair, not an OR. A registry that changes issuer but keeps subject (or
// vice versa) counts as a different signer.
func TestSigningProfile_Equal(t *testing.T) {
	t.Parallel()

	base := SigningProfile{Issuer: "iss", Subject: "sub"}

	tests := []struct {
		name  string
		other SigningProfile
		want  bool
	}{
		{"same", SigningProfile{Issuer: "iss", Subject: "sub"}, true},
		{"issuer_differs", SigningProfile{Issuer: "other", Subject: "sub"}, false},
		{"subject_differs", SigningProfile{Issuer: "iss", Subject: "other"}, false},
		{"both_empty_vs_populated", SigningProfile{}, false},
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
