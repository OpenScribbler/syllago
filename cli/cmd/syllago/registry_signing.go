package main

// Signing-identity resolution for `syllago registry add` (ADR 0007 slice-2b).
//
// The goal: every MOAT registry must pin a signing identity at add-time. The
// three acceptable paths, in precedence order:
//
//  1. Bundled allowlist match (zero-config for well-known registries).
//  2. Explicit --signing-identity (+ --signing-repository-id /
//     --signing-repository-owner-id for GitHub) flags on the CLI.
//  3. Hard fail with MOAT_IDENTITY_UNPINNED pointing operators at the
//     syllago-docs page explaining how to pin or submit an allowlist entry.
//
// What we deliberately do NOT do: interactive TOFU prompts, silent
// accept-first-cert, or "come back later" deferred pinning. Slice-2a
// already bundles the meta-registry identity; anything else ships with the
// operator signing off on the profile in the commit that runs `registry
// add`.

import (
	"fmt"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/spf13/cobra"
)

// mustStringFlag returns the string value of a flag known to be registered
// on the command. Flag lookup only fails when the flag name is misspelled,
// which is a programmer error caught in tests, not a runtime condition.
func mustStringFlag(cmd *cobra.Command, name string) string {
	v, _ := cmd.Flags().GetString(name)
	return v
}

// MoatPinningDocsURL is the syllago-docs page this command points operators
// at when the hard-fail fires. The page must exist before slice-2b merges —
// see the syllago-docs bead (syllago-docs-6cv).
const MoatPinningDocsURL = "https://openscribbler.github.io/syllago-docs/moat/registry-add-signing-identity/"

// signingFlagSet holds the raw --signing-* flag values plus a "user passed
// anything" bit so we can distinguish "user explicitly requested MOAT" from
// "default empty values."
type signingFlagSet struct {
	UserRequestedMOAT bool // true when --moat OR any --signing-* flag is set
	Identity          string
	Issuer            string
	RepositoryID      string
	RepositoryOwnerID string
}

// signingResolution is the output of resolveSigningProfile: either a
// populated profile + source label + manifest URI, or a (nil, "") pair
// meaning the caller should continue in legacy git-mode.
type signingResolution struct {
	Profile     *config.SigningProfile
	ManifestURI string // non-empty only for "allowlist" source; empty for "flags" and legacy git
	Source      string // "allowlist", "flags", or "" when legacy git
}

// resolveSigningProfile applies the allowlist-then-flags-then-fail policy
// for `registry add`. Returns:
//
//   - (profile, "allowlist", nil) when the URL is in the bundled allowlist
//     AND the caller did not pass --signing-identity to override.
//   - (profile, "flags", nil) when --signing-identity is set (allowlist
//     match is informational in this case; flags always win).
//   - (nil, "", nil) when MOAT was NOT requested — the caller continues
//     in legacy git mode.
//   - (nil, "", err) with MOAT_IDENTITY_UNPINNED when MOAT WAS requested
//     but no allowlist entry AND no --signing-identity.
//   - (nil, "", err) with MOAT_IDENTITY_INVALID when flags are present but
//     incomplete (e.g. GitHub issuer without numeric IDs).
//
// "MOAT was requested" means any of:
//   - the caller asked for --moat explicitly, or
//   - the caller passed any --signing-* flag, or
//   - the URL matches the bundled allowlist (zero-config upgrade).
func resolveSigningProfile(gitURL string, flags signingFlagSet) (*signingResolution, error) {
	allowlistEntry, hasAllowlistEntry := moat.LookupSigningIdentity(gitURL)

	// Case 1: no MOAT intent anywhere → legacy git mode, no profile captured.
	if !flags.UserRequestedMOAT && !hasAllowlistEntry {
		return &signingResolution{}, nil
	}

	// Case 2: flags always win when present — they represent an explicit
	// operator decision, possibly to override a stale allowlist entry.
	if flags.Identity != "" {
		profile, err := profileFromFlags(flags)
		if err != nil {
			return nil, err
		}
		return &signingResolution{Profile: profile, Source: "flags"}, nil
	}

	// Case 3: no --signing-identity, but allowlist has a match → auto-pin.
	if hasAllowlistEntry {
		return &signingResolution{
			Profile:     allowlistEntry.Profile,
			ManifestURI: allowlistEntry.ManifestURI, // may be empty for pre-manifest_uri entries
			Source:      "allowlist",
		}, nil
	}

	// Case 4: MOAT requested (--moat or partial --signing-* flags) but the
	// caller provided neither a complete flag set nor a matching allowlist
	// entry. Hard fail.
	return nil, output.NewStructuredErrorDetail(
		output.ErrMoatIdentityUnpinned,
		fmt.Sprintf("registry at %s has no pinned signing identity", gitURL),
		"Pass --signing-identity <workflow-san> and --signing-repository-id / --signing-repository-owner-id (required for GitHub Actions issuers), or request an allowlist entry.",
		"See "+MoatPinningDocsURL+" for the full workflow and allowlist contribution process.",
	)
}

// profileFromFlags validates the --signing-* flag set and constructs a
// config.SigningProfile. Returns MOAT_IDENTITY_INVALID when flags are
// incomplete. Non-GitHub issuers do not require numeric IDs; the GitHub
// Actions issuer requires BOTH repository_id and repository_owner_id per
// ADR 0007.
func profileFromFlags(flags signingFlagSet) (*config.SigningProfile, error) {
	issuer := flags.Issuer
	if issuer == "" {
		issuer = moat.GitHubActionsIssuer
	}

	profile := &config.SigningProfile{
		Issuer:            issuer,
		Subject:           flags.Identity,
		ProfileVersion:    moat.ProfileVersionV1,
		RepositoryID:      flags.RepositoryID,
		RepositoryOwnerID: flags.RepositoryOwnerID,
	}

	if issuer == moat.GitHubActionsIssuer {
		if flags.RepositoryID == "" || flags.RepositoryOwnerID == "" {
			return nil, output.NewStructuredError(
				output.ErrMoatIdentityInvalid,
				"GitHub Actions issuer requires --signing-repository-id and --signing-repository-owner-id",
				"Find numeric IDs with `gh api repos/OWNER/REPO --jq '.id, .owner.id'`. See "+MoatPinningDocsURL+".",
			)
		}
	}

	return profile, nil
}

// describeProfileSource produces a one-line announcement suitable for
// printing after allowlist or flags resolution. Returns an empty string
// when no profile was captured (legacy git mode).
func describeProfileSource(res *signingResolution, gitURL string) string {
	if res == nil || res.Profile == nil {
		return ""
	}
	switch res.Source {
	case "allowlist":
		return fmt.Sprintf("Using bundled signing identity for %s (allowlist entry).", gitURL)
	case "flags":
		return fmt.Sprintf("Using signing identity from --signing-* flags for %s.", gitURL)
	default:
		return ""
	}
}

// anySigningFlagSet returns true when any --signing-* flag has a non-empty
// value. Used alongside the explicit --moat bool to infer MOAT intent.
func anySigningFlagSet(flags signingFlagSet) bool {
	return flags.Identity != "" ||
		flags.Issuer != "" ||
		flags.RepositoryID != "" ||
		flags.RepositoryOwnerID != ""
}

// trimAllFlagValues normalizes user input for the signing flags — trailing
// whitespace from copy/paste is a real footgun when matching SANs.
func trimAllFlagValues(flags signingFlagSet) signingFlagSet {
	flags.Identity = strings.TrimSpace(flags.Identity)
	flags.Issuer = strings.TrimSpace(flags.Issuer)
	flags.RepositoryID = strings.TrimSpace(flags.RepositoryID)
	flags.RepositoryOwnerID = strings.TrimSpace(flags.RepositoryOwnerID)
	return flags
}
