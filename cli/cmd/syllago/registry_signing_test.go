package main

// Tests for resolveSigningProfile (ADR 0007 slice-2b).
//
// Five paths we must cover:
//  1. Allowlist match + no flags  → auto-pinned from allowlist.
//  2. Flags present                → pinned from flags (allowlist ignored).
//  3. MOAT requested + no allowlist + no --signing-identity → hard-fail.
//  4. Flags + GitHub issuer missing numeric IDs → validation error.
//  5. No MOAT intent anywhere → legacy git mode, nil profile.

import (
	"errors"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/spf13/cobra"
)

const metaRegistryURL = "https://github.com/OpenScribbler/syllago-meta-registry"

// TestResolveSigningProfile_AllowlistMatch covers path 1 — the zero-config
// happy path for the meta-registry. Verifies both that we auto-pin and
// that the pinned profile carries the numeric IDs from the bundle.
func TestResolveSigningProfile_AllowlistMatch(t *testing.T) {
	t.Parallel()
	flags := signingFlagSet{}
	res, err := resolveSigningProfile(metaRegistryURL, flags)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Profile == nil {
		t.Fatal("expected profile from allowlist, got nil")
	}
	if res.Source != "allowlist" {
		t.Errorf("Source = %q, want %q", res.Source, "allowlist")
	}
	if res.Profile.RepositoryID == "" || res.Profile.RepositoryOwnerID == "" {
		t.Errorf("expected numeric IDs populated, got repo_id=%q owner_id=%q",
			res.Profile.RepositoryID, res.Profile.RepositoryOwnerID)
	}
}

// TestResolveSigningProfile_FlagsOverrideAllowlist covers path 2 — when
// the operator passes flags against a URL that ALSO matches the allowlist,
// the flags win. This is the escape hatch for rotating signing identities
// ahead of an allowlist update.
func TestResolveSigningProfile_FlagsOverrideAllowlist(t *testing.T) {
	t.Parallel()
	flags := signingFlagSet{
		UserRequestedMOAT: true,
		Identity:          "https://github.com/OpenScribbler/syllago-meta-registry/.github/workflows/rotated.yml@refs/heads/main",
		RepositoryID:      "1193220959",
		RepositoryOwnerID: "263775997",
	}
	res, err := resolveSigningProfile(metaRegistryURL, flags)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Profile == nil {
		t.Fatal("expected profile from flags, got nil")
	}
	if res.Source != "flags" {
		t.Errorf("Source = %q, want %q", res.Source, "flags")
	}
	if res.Profile.Subject != flags.Identity {
		t.Errorf("Subject = %q, want %q", res.Profile.Subject, flags.Identity)
	}
	if res.Profile.Issuer != moat.GitHubActionsIssuer {
		t.Errorf("Issuer defaulted wrong: got %q want %q", res.Profile.Issuer, moat.GitHubActionsIssuer)
	}
}

// TestResolveSigningProfile_FlagsOnlyNoAllowlist covers flags against a URL
// that is NOT in the allowlist — the common "I'm adding a new MOAT registry
// my team published" path.
func TestResolveSigningProfile_FlagsOnlyNoAllowlist(t *testing.T) {
	t.Parallel()
	flags := signingFlagSet{
		UserRequestedMOAT: true,
		Identity:          "https://github.com/myteam/our-registry/.github/workflows/moat.yml@refs/heads/main",
		RepositoryID:      "999",
		RepositoryOwnerID: "888",
	}
	res, err := resolveSigningProfile("https://github.com/myteam/our-registry.git", flags)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Profile == nil {
		t.Fatal("expected profile from flags")
	}
	if res.Source != "flags" {
		t.Errorf("Source = %q, want %q", res.Source, "flags")
	}
}

// TestResolveSigningProfile_HardFail covers path 3 — the operator asked
// for MOAT via --moat but didn't supply flags and the URL isn't in the
// allowlist. The error MUST point at the docs page so triage is trivial.
func TestResolveSigningProfile_HardFail(t *testing.T) {
	t.Parallel()
	flags := signingFlagSet{UserRequestedMOAT: true}
	_, err := resolveSigningProfile("https://github.com/unknown/registry.git", flags)
	if err == nil {
		t.Fatal("expected hard-fail, got nil")
	}
	var se output.StructuredError
	if !errors.As(err, &se) {
		t.Fatalf("expected StructuredError, got %T: %v", err, err)
	}
	if se.Code != output.ErrMoatIdentityUnpinned {
		t.Errorf("Code = %q, want %q", se.Code, output.ErrMoatIdentityUnpinned)
	}
	if !strings.Contains(se.Details, MoatPinningDocsURL) {
		t.Errorf("expected docs URL in error Details, got: %s", se.Details)
	}
}

// TestResolveSigningProfile_LegacyGitMode covers path 5 — no --moat, no
// flags, URL not in allowlist. Returns an empty resolution so the caller
// falls through to the existing git clone flow.
func TestResolveSigningProfile_LegacyGitMode(t *testing.T) {
	t.Parallel()
	flags := signingFlagSet{}
	res, err := resolveSigningProfile("https://github.com/unknown/registry.git", flags)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected empty resolution, got nil")
	}
	if res.Profile != nil {
		t.Errorf("expected nil profile in legacy mode, got %+v", res.Profile)
	}
	if res.Source != "" {
		t.Errorf("expected empty Source in legacy mode, got %q", res.Source)
	}
}

// TestResolveSigningProfile_AllowlistUpgradesSilently covers the
// security-by-default case: the operator ran `registry add URL` for a
// known-MOAT URL without --moat. We still upgrade to MOAT because letting
// a known-good registry fall into git mode would be a security downgrade.
func TestResolveSigningProfile_AllowlistUpgradesSilently(t *testing.T) {
	t.Parallel()
	flags := signingFlagSet{} // no --moat, no --signing-*
	res, err := resolveSigningProfile(metaRegistryURL, flags)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Profile == nil {
		t.Fatal("expected auto-upgrade to MOAT for known URL")
	}
	if res.Source != "allowlist" {
		t.Errorf("expected allowlist source, got %q", res.Source)
	}
}

// TestProfileFromFlags_GitHubMissingRepoID covers path 4 — GitHub Actions
// issuer MUST come with both numeric IDs. Omitting either is a flag-parse
// error that surfaces BEFORE any network work.
func TestProfileFromFlags_GitHubMissingRepoID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		flags signingFlagSet
	}{
		{
			name: "missing repository_id",
			flags: signingFlagSet{
				UserRequestedMOAT: true,
				Identity:          "https://github.com/o/r/.github/workflows/x.yml@refs/heads/main",
				RepositoryOwnerID: "123",
			},
		},
		{
			name: "missing repository_owner_id",
			flags: signingFlagSet{
				UserRequestedMOAT: true,
				Identity:          "https://github.com/o/r/.github/workflows/x.yml@refs/heads/main",
				RepositoryID:      "456",
			},
		},
		{
			name: "both missing",
			flags: signingFlagSet{
				UserRequestedMOAT: true,
				Identity:          "https://github.com/o/r/.github/workflows/x.yml@refs/heads/main",
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := resolveSigningProfile("https://github.com/o/r.git", tc.flags)
			if err == nil {
				t.Fatal("expected MOAT_IDENTITY_INVALID error, got nil")
			}
			var se output.StructuredError
			if !errors.As(err, &se) {
				t.Fatalf("expected StructuredError, got %T", err)
			}
			if se.Code != output.ErrMoatIdentityInvalid {
				t.Errorf("Code = %q, want %q", se.Code, output.ErrMoatIdentityInvalid)
			}
		})
	}
}

// TestProfileFromFlags_NonGitHubSkipsIDCheck asserts that non-GitHub
// issuers (future GitLab, Buildkite, etc.) do NOT require numeric IDs.
// The numeric-ID requirement exists specifically to close GitHub's repo-
// transfer forgery vector — other issuers have their own equivalents.
func TestProfileFromFlags_NonGitHubSkipsIDCheck(t *testing.T) {
	t.Parallel()
	flags := signingFlagSet{
		UserRequestedMOAT: true,
		Identity:          "https://gitlab.example.com/group/project//.gitlab-ci.yml@main",
		Issuer:            "https://gitlab.example.com",
	}
	res, err := resolveSigningProfile("https://gitlab.example.com/group/project.git", flags)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Profile == nil {
		t.Fatal("expected profile, got nil")
	}
	if res.Profile.Issuer != "https://gitlab.example.com" {
		t.Errorf("Issuer = %q, want gitlab URL", res.Profile.Issuer)
	}
	if res.Profile.RepositoryID != "" || res.Profile.RepositoryOwnerID != "" {
		t.Error("expected numeric IDs to remain empty for non-GitHub issuer")
	}
}

// TestDescribeProfileSource_EmptyForLegacy ensures no announcement line
// prints when we fell through to git mode — keeps the output quiet in the
// common back-compat case.
func TestDescribeProfileSource_EmptyForLegacy(t *testing.T) {
	t.Parallel()
	if msg := describeProfileSource(nil, "https://x/y"); msg != "" {
		t.Errorf("expected empty, got %q", msg)
	}
	if msg := describeProfileSource(&signingResolution{}, "https://x/y"); msg != "" {
		t.Errorf("expected empty for zero resolution, got %q", msg)
	}
}

// TestDescribeProfileSource_ProducesHumanText covers both announcement
// variants — unlike a raw enum check, this documents the exact wording
// operators will see.
func TestDescribeProfileSource_ProducesHumanText(t *testing.T) {
	t.Parallel()
	p, _ := moat.LookupSigningIdentity(metaRegistryURL)
	if p == nil {
		t.Fatal("meta-registry missing from allowlist")
	}
	allowlistMsg := describeProfileSource(&signingResolution{Profile: p, Source: "allowlist"}, metaRegistryURL)
	if !strings.Contains(allowlistMsg, "allowlist") {
		t.Errorf("allowlist message missing 'allowlist': %q", allowlistMsg)
	}
	flagsMsg := describeProfileSource(&signingResolution{Profile: p, Source: "flags"}, metaRegistryURL)
	if !strings.Contains(flagsMsg, "--signing-") {
		t.Errorf("flags message missing '--signing-': %q", flagsMsg)
	}
}

// TestDescribeProfileSource_UnknownSourceIsSilent covers the defensive
// default branch — an unknown Source string should print nothing rather
// than leak internal state to operators.
func TestDescribeProfileSource_UnknownSourceIsSilent(t *testing.T) {
	t.Parallel()
	p, _ := moat.LookupSigningIdentity(metaRegistryURL)
	if msg := describeProfileSource(&signingResolution{Profile: p, Source: "future-source"}, metaRegistryURL); msg != "" {
		t.Errorf("expected silence for unknown source, got %q", msg)
	}
}

// TestAnySigningFlagSet_Truth covers the implicit-MOAT detection used by
// registryAddCmd to set UserRequestedMOAT.
func TestAnySigningFlagSet_Truth(t *testing.T) {
	t.Parallel()
	cases := []struct {
		flags signingFlagSet
		want  bool
	}{
		{signingFlagSet{}, false},
		{signingFlagSet{Identity: "x"}, true},
		{signingFlagSet{Issuer: "x"}, true},
		{signingFlagSet{RepositoryID: "x"}, true},
		{signingFlagSet{RepositoryOwnerID: "x"}, true},
	}
	for i, tc := range cases {
		if got := anySigningFlagSet(tc.flags); got != tc.want {
			t.Errorf("case %d: got %v want %v", i, got, tc.want)
		}
	}
}

// TestMustStringFlag_ReadsValue exercises the thin cobra-flag accessor.
// Its only failure mode (misspelled flag name) is caught here and in any
// integration test that drives registryAddCmd through RunE.
func TestMustStringFlag_ReadsValue(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{Use: "fake"}
	cmd.Flags().String("thing", "default-value", "")
	if got := mustStringFlag(cmd, "thing"); got != "default-value" {
		t.Errorf("mustStringFlag default = %q, want %q", got, "default-value")
	}
	if err := cmd.Flags().Set("thing", "overridden"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := mustStringFlag(cmd, "thing"); got != "overridden" {
		t.Errorf("mustStringFlag after set = %q, want %q", got, "overridden")
	}
	// Unknown flag returns empty — the "programmer error" path, validated
	// so callers aren't surprised.
	if got := mustStringFlag(cmd, "no-such-flag"); got != "" {
		t.Errorf("mustStringFlag unknown = %q, want empty", got)
	}
}

// TestTrimAllFlagValues covers the copy/paste robustness — a trailing
// newline in --signing-identity is the classic footgun when users paste
// from a CI log.
func TestTrimAllFlagValues(t *testing.T) {
	t.Parallel()
	input := signingFlagSet{
		Identity:          " subject\n",
		Issuer:            "\tissuer ",
		RepositoryID:      "  42 ",
		RepositoryOwnerID: "\n7\n",
	}
	got := trimAllFlagValues(input)
	if got.Identity != "subject" || got.Issuer != "issuer" || got.RepositoryID != "42" || got.RepositoryOwnerID != "7" {
		t.Errorf("trim incomplete: %+v", got)
	}
}
