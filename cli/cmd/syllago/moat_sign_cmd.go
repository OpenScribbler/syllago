package main

// `moat sign` subcommand — produce a signature.bundle from a manifest.json
// (syllago-92i4c Phases 2 and 3).
//
// Two modes:
//
//  1. Online (--rekor-raw): assembles a sigstore v0.3 bundle from a raw Rekor
//     API response captured from a real Publisher Action run. The Rekor entry
//     MUST have signed sha256(manifest.json). Round-trips through VerifyManifest
//     before writing to catch mismatched artifacts early.
//
//  2. Dev/offline (--dev-trusted-root): uses an ephemeral offline CA
//     (sigstore-go VirtualSigstore) to sign the manifest and emit a matched
//     (signature.bundle, trusted_root.json) pair. Intended for smoke fixtures
//     and dev test harnesses — NOT for production registries.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/spf13/cobra"
)

var moatSignCmd = &cobra.Command{
	Use:   "sign",
	Short: "Produce a signature.bundle from a manifest (online or dev-offline mode)",
	Long: `Assemble a sigstore v0.3 bundle for a manifest.json.

Online mode (--rekor-raw required):
  Assembles a bundle from a raw Rekor API response captured from a real
  Publisher Action run. The Rekor entry must have signed sha256(manifest.json).
  The bundle is verified against the trusted root before being written.

Dev/offline mode (--dev-trusted-root <dir>):
  Uses an ephemeral offline CA to sign the manifest without any network calls.
  Writes both signature.bundle (at --out) and trusted_root.json (in <dir>).
  The resulting trusted root is development-only — never use it in production.

The --identity flag (online mode only) accepts a JSON file with
{"issuer": "...", "subject": "..."} matching the expected signing identity.
When omitted, the identity is auto-extracted from the Rekor entry cert.`,
	Example: `  # Online: sign with a real Rekor raw response
  syllago moat sign \
    --manifest ./registry/manifest.json \
    --rekor-raw ./ci-artifacts/rekor-response.json \
    --out ./registry/manifest.json.sigstore

  # Dev/offline: generate a smoke fixture bundle + dev trusted root
  syllago moat sign \
    --manifest ./test-registry/manifest.json \
    --dev-trusted-root ./test-registry \
    --out ./test-registry/manifest.json.sigstore`,
	RunE: runMoatSign,
}

func init() {
	moatSignCmd.Flags().String("manifest", "", "Path to manifest.json (required)")
	moatSignCmd.Flags().String("rekor-raw", "", "Path to raw Rekor API response JSON (online mode)")
	moatSignCmd.Flags().String("out", "", "Output path for signature.bundle (default: <manifest>.sigstore)")
	moatSignCmd.Flags().String("identity", "", "Path to signing identity JSON {\"issuer\":\"...\",\"subject\":\"...\"} (online mode, auto-extracted when omitted)")
	moatSignCmd.Flags().String("trusted-root", "", "Path to trusted-root.json for round-trip verify (online mode, default: bundled root)")
	moatSignCmd.Flags().String("dev-trusted-root", "", "Directory for dev-mode: writes trusted_root.json here and signs with an offline CA (mutually exclusive with --rekor-raw)")
	_ = moatSignCmd.MarkFlagRequired("manifest")
	moatCmd.AddCommand(moatSignCmd)
}

func runMoatSign(cmd *cobra.Command, _ []string) error {
	manifestPath, _ := cmd.Flags().GetString("manifest")
	outPath, _ := cmd.Flags().GetString("out")
	devTrustedRootDir, _ := cmd.Flags().GetString("dev-trusted-root")

	if outPath == "" {
		outPath = manifestPath + ".sigstore"
	}

	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("reading manifest: %w", err)
	}

	if devTrustedRootDir != "" {
		return runMoatSignDev(cmd, manifestBytes, outPath, devTrustedRootDir)
	}
	return runMoatSignOnline(cmd, manifestBytes, outPath)
}

// runMoatSignDev is the Phase 3 dev/offline path using VirtualSigstore.
func runMoatSignDev(cmd *cobra.Command, manifestBytes []byte, outPath, trustedRootDir string) error {
	bundleJSON, trustedRootJSON, _, err := moat.SignManifestDev(manifestBytes)
	if err != nil {
		return fmt.Errorf("dev signing: %w", err)
	}

	if err := os.MkdirAll(trustedRootDir, 0755); err != nil {
		return fmt.Errorf("creating dev trusted root dir: %w", err)
	}
	trustedRootPath := filepath.Join(trustedRootDir, "trusted_root.json")
	if err := os.WriteFile(trustedRootPath, trustedRootJSON, 0644); err != nil {
		return fmt.Errorf("writing dev trusted root: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}
	if err := os.WriteFile(outPath, bundleJSON, 0644); err != nil {
		return fmt.Errorf("writing bundle: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "bundle written to %s\n", outPath)
	fmt.Fprintf(cmd.OutOrStdout(), "dev trusted root written to %s\n", trustedRootPath)
	fmt.Fprintf(cmd.ErrOrStderr(), "WARNING: dev trusted root is not suitable for production use\n")
	fmt.Fprintf(cmd.ErrOrStderr(), "  identity: issuer=%s subject=%s\n", moat.DevSigningIssuer, moat.DevSigningSubject)
	return nil
}

// runMoatSignOnline is the Phase 2 online path using a real Rekor raw response.
func runMoatSignOnline(cmd *cobra.Command, manifestBytes []byte, outPath string) error {
	rekorRawPath, _ := cmd.Flags().GetString("rekor-raw")
	identityPath, _ := cmd.Flags().GetString("identity")
	trustedRootPath, _ := cmd.Flags().GetString("trusted-root")

	if rekorRawPath == "" {
		return fmt.Errorf("--rekor-raw is required in online mode (or use --dev-trusted-root for offline signing)")
	}

	rekorRaw, err := os.ReadFile(rekorRawPath)
	if err != nil {
		return fmt.Errorf("reading rekor raw: %w", err)
	}

	// The manifest bytes ARE the signed artifact on the whole-manifest path.
	// The Rekor entry must have signed sha256(manifestBytes).
	bundle, err := moat.BuildBundle(rekorRaw, manifestBytes)
	if err != nil {
		return fmt.Errorf("building bundle: %w", err)
	}

	bundleBytes, err := bundle.MarshalJSON()
	if err != nil {
		return fmt.Errorf("marshaling bundle: %w", err)
	}

	profile, err := resolveSigningProfileForSign(identityPath, rekorRaw)
	if err != nil {
		return fmt.Errorf("resolving signing identity: %w", err)
	}

	trustedRootInfo, err := resolveTrustedRootForSign(trustedRootPath)
	if err != nil {
		return fmt.Errorf("resolving trusted root: %w", err)
	}

	if _, err := moat.VerifyManifest(manifestBytes, bundleBytes, &profile, trustedRootInfo.Bytes); err != nil {
		return fmt.Errorf("round-trip verification failed (bundle does not verify against manifest): %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}
	if err := os.WriteFile(outPath, bundleBytes, 0644); err != nil {
		return fmt.Errorf("writing bundle: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "bundle written to %s\n", outPath)
	return nil
}

// resolveSigningProfileForSign returns the SigningProfile for the round-trip
// verify. When identityPath is provided, parses it as JSON; otherwise
// auto-extracts from the Rekor entry cert.
func resolveSigningProfileForSign(identityPath string, rekorRaw []byte) (moat.SigningProfile, error) {
	if identityPath != "" {
		raw, err := os.ReadFile(identityPath)
		if err != nil {
			return moat.SigningProfile{}, fmt.Errorf("reading identity file: %w", err)
		}
		var p moat.SigningProfile
		if err := json.Unmarshal(raw, &p); err != nil {
			return moat.SigningProfile{}, fmt.Errorf("parsing identity JSON: %w", err)
		}
		if p.Issuer == "" || p.Subject == "" {
			return moat.SigningProfile{}, fmt.Errorf("identity file must contain non-empty issuer and subject fields")
		}
		return p, nil
	}

	issuer, subject, err := moat.ExtractIdentityFromRekorRaw(rekorRaw)
	if err != nil {
		return moat.SigningProfile{}, fmt.Errorf("auto-extracting identity from rekor entry: %w (use --identity to specify it explicitly)", err)
	}
	return moat.SigningProfile{Issuer: issuer, Subject: subject}, nil
}

// resolveTrustedRootForSign loads the trusted root for round-trip verify.
func resolveTrustedRootForSign(path string) (moat.TrustedRootInfo, error) {
	if path != "" {
		return moat.TrustedRootFromPath(path, time.Now())
	}
	return moat.BundledTrustedRoot(time.Now()), nil
}
