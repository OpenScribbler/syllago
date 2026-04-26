package moatinstall

// FetchAndRecord — the Proceed-branch action for both CLI and TUI MOAT
// installs. Extracted from cmd/syllago/install_moat_fetch.go so the TUI can
// reach it (cmd/syllago is a main package and cannot be imported).
//
// Behavior follows moat-spec.md §"Dual-Attested" and reference
// implementation moat_verify.py (_online_step4 + _online_step5):
//
//   - SIGNED tier: fetch the registry's per-item Rekor entry by index and
//     verify it against the manifest's RegistrySigningProfile. The
//     per-item rekor_log_index is the REGISTRY's entry (spec line 786) —
//     not the publisher's. Then download + hash-verify + extract + record.
//   - DUAL-ATTESTED tier: same as SIGNED for the registry leg, PLUS a
//     separate fetch of moat-attestation.json from the publisher's source
//     repo (moat-attestation branch) and verify the publisher's separate
//     Rekor entry against per-item content[].signing_profile. Both legs
//     must pass.
//   - UNSIGNED tier: skip Rekor + verify, download + hash-verify + extract
//     + record with a null AttestationBundle.
//
// Bug-fix history (syllago-cvwj5, 2026-04-25): prior code passed per-item
// signing_profile as the pin for the registry's per-item Rekor entry,
// which is wrong against spec — the per-item profile pins the publisher's
// SEPARATE attestation, not the registry's.
//
// All failure modes return structured errors with stable codes — the
// caller (CLI or TUI) only has to surface them.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// SourceCacheDir returns the root cache directory for MOAT source
// artifacts. Tests override by replacing this variable.
var SourceCacheDir = func() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "syllago", "moat-sources"), nil
}

// CloneScratchDir returns the parent directory used for scratch source
// clones during install. Default is the OS temp dir; tests override to a
// t.TempDir for hermetic cleanup.
var CloneScratchDir = func() (string, error) {
	return os.TempDir(), nil
}

// VerifyItem is the per-item verification seam. Production points at
// moat.VerifyAttestationItem (the full ECDSA → SET → inclusion proof →
// Fulcio identity → repo-ID binding chain). Wiring tests stub this to
// short-circuit the crypto — the real chain is exercised in
// internal/moat/item_verify_test.go.
var VerifyItem = moat.VerifyAttestationItem

// FetchPublisherAttestationFn is the seam for fetching the publisher's
// moat-attestation.json (Dual-Attested second leg). Production points at
// moat.FetchPublisherAttestation. Wiring tests override to inject offline
// fixtures.
var FetchPublisherAttestationFn = moat.FetchPublisherAttestation

// Now is the clock seam for lockfile pinned_at timestamps. Tests pin to a
// fixed instant for deterministic snapshots.
var Now = time.Now

// FetchAndRecord runs the install side-effect for a verified manifest
// entry. Returns the absolute path of the extracted source-artifact tree
// on success.
//
// Caller responsibilities:
//   - Gate decision has already cleared (PreInstallCheck returned Proceed
//     or an interactive prompt was accepted).
//   - lf is the in-memory lockfile; FetchAndRecord adds a LockEntry via
//     installer.RecordInstall and persists with lf.Save.
//   - registryName is the short config.Registry.Name (drives cache pathing);
//     registryURL is the manifest URI that goes into LockEntry.Registry.
//   - registryProfile is the manifest-level RegistrySigningProfile, used
//     as the verification identity for the registry's per-item Rekor
//     entry on every SIGNED and DUAL-ATTESTED item. Ignored for UNSIGNED
//     items. The publisher's per-item signing_profile (entry.SigningProfile)
//     is used separately, only on the Dual-Attested second leg.
//   - trustedRootJSON is the Sigstore trusted-root bytes (e.g. from
//     moat.BundledTrustedRoot). Required for SIGNED/DUAL-ATTESTED.
func FetchAndRecord(
	ctx context.Context,
	entry *moat.ContentEntry,
	registryName, registryURL, lockfilePath string,
	lf *moat.Lockfile,
	registryProfile *moat.SigningProfile,
	trustedRootJSON []byte,
) (string, error) {
	if entry == nil {
		return "", errors.New("FetchAndRecord: entry is nil")
	}
	if lf == nil {
		return "", errors.New("FetchAndRecord: lockfile is nil")
	}

	tier := entry.TrustTier()
	var rekorBundle []byte
	if tier != moat.TrustTierUnsigned {
		// Spec model: per-item rekor_log_index is the REGISTRY's per-item
		// Rekor entry (moat-spec.md line 786). It MUST be pinned against
		// the manifest-level RegistrySigningProfile, regardless of whether
		// per-item signing_profile is present. The per-item profile pins
		// the publisher's SEPARATE Rekor entry from moat-attestation.json
		// — handled below for Dual-Attested only.
		if registryProfile == nil {
			return "", output.NewStructuredError(
				output.ErrMoatInvalid,
				fmt.Sprintf("cannot verify %s/%s: registry signing profile not pinned", registryName, entry.Name),
				"The manifest reached install without a registry_signing_profile — re-sync the registry to surface the underlying error.",
			)
		}

		raw, fetchErr := moat.FetchRekorEntry(ctx, *entry.RekorLogIndex)
		if fetchErr != nil {
			return "", output.NewStructuredErrorDetail(
				output.ErrMoatInvalid,
				fmt.Sprintf("could not fetch Rekor entry for %s/%s", registryName, entry.Name),
				"Check network access to rekor.sigstore.dev. The registry's manifest references a transparency-log entry that we could not retrieve.",
				fetchErr.Error(),
			)
		}

		item := moat.AttestationItem{
			Name:          entry.Name,
			ContentHash:   entry.ContentHash,
			SourceRef:     entry.SourceURI,
			RekorLogIndex: *entry.RekorLogIndex,
		}
		if _, vErr := VerifyItem(item, registryProfile, raw, trustedRootJSON); vErr != nil {
			return "", output.NewStructuredErrorDetail(
				output.ErrMoatInvalid,
				fmt.Sprintf("attestation verification failed for %s/%s", registryName, entry.Name),
				"The registry's per-item Rekor entry did not validate against the pinned registry_signing_profile. Re-sync the registry and retry.",
				vErr.Error(),
			)
		}
		rekorBundle = raw

		// Dual-Attested second leg: fetch the publisher's separate Rekor
		// entry from moat-attestation.json on the source repo's
		// moat-attestation branch and verify it against per-item
		// signing_profile. Mirrors moat_verify.py:_online_step5.
		if tier == moat.TrustTierDualAttested {
			if entry.SigningProfile == nil {
				// Defense-in-depth — TrustTier() already implies non-nil.
				return "", output.NewStructuredError(
					output.ErrMoatInvalid,
					fmt.Sprintf("cannot verify publisher attestation for %s/%s: per-item signing_profile missing", registryName, entry.Name),
					"This indicates a malformed manifest reached install. Re-sync the registry to surface the underlying error.",
				)
			}

			attBytes, attErr := FetchPublisherAttestationFn(ctx, entry.SourceURI)
			if attErr != nil {
				return "", output.NewStructuredErrorDetail(
					output.ErrMoatInvalid,
					fmt.Sprintf("could not fetch publisher attestation for %s/%s", registryName, entry.Name),
					"The Dual-Attested tier requires moat-attestation.json on the publisher's moat-attestation branch. Check network access to raw.githubusercontent.com.",
					attErr.Error(),
				)
			}

			pubLogIndex, lookupErr := moat.FindPublisherEntry(attBytes, entry.ContentHash)
			if lookupErr != nil {
				return "", output.NewStructuredErrorDetail(
					output.ErrMoatInvalid,
					fmt.Sprintf("publisher attestation does not cover %s/%s", registryName, entry.Name),
					"The publisher's moat-attestation.json has no items[] entry matching this content_hash — the registry indexed content the publisher never attested.",
					lookupErr.Error(),
				)
			}

			pubRaw, pubFetchErr := moat.FetchRekorEntry(ctx, pubLogIndex)
			if pubFetchErr != nil {
				return "", output.NewStructuredErrorDetail(
					output.ErrMoatInvalid,
					fmt.Sprintf("could not fetch publisher Rekor entry for %s/%s", registryName, entry.Name),
					"Check network access to rekor.sigstore.dev.",
					pubFetchErr.Error(),
				)
			}

			pubItem := moat.AttestationItem{
				Name:          entry.Name,
				ContentHash:   entry.ContentHash,
				SourceRef:     entry.SourceURI,
				RekorLogIndex: pubLogIndex,
			}
			if _, vErr := VerifyItem(pubItem, entry.SigningProfile, pubRaw, trustedRootJSON); vErr != nil {
				return "", output.NewStructuredErrorDetail(
					output.ErrMoatInvalid,
					fmt.Sprintf("publisher attestation verification failed for %s/%s", registryName, entry.Name),
					"The publisher's Rekor entry did not validate against the pinned per-item signing_profile.",
					vErr.Error(),
				)
			}
		}
	}

	if err := moat.ValidateSourceURI(entry.SourceURI); err != nil {
		return "", output.NewStructuredErrorDetail(
			output.ErrMoatInvalid,
			fmt.Sprintf("%s for %s/%s", err.Error(), registryName, entry.Name),
			"MOAT source_uri must be an https:// repo URL. Other schemes are not supported.",
			err.Error(),
		)
	}

	categoryDir, ok := moat.CategoryDirForMOATType(entry.Type)
	if !ok {
		return "", output.NewStructuredError(
			output.ErrMoatInvalid,
			fmt.Sprintf("unsupported MOAT content type %q for %s/%s", entry.Type, registryName, entry.Name),
			"Only skill, agent, rules, and command are normative MOAT types. hook and mcp are deferred.",
		)
	}

	cacheRoot, err := SourceCacheDir()
	if err != nil {
		return "", output.NewStructuredErrorDetail(
			output.ErrSystemHomedir,
			"cannot resolve user cache dir for moat-sources",
			"Set $XDG_CACHE_HOME or ensure your home directory is readable.",
			err.Error(),
		)
	}

	hashPrefix := shortHashCache(entry.ContentHash)
	targetDir := filepath.Join(cacheRoot, registryName, entry.Name, hashPrefix)

	// Spec model (moat-spec.md §"Repository Layout" + line 781):
	// source_uri is the source REPOSITORY URI; content_hash is the
	// merkle hash of the item subdirectory at <category>/<name>/.
	// Materialize the repo, locate the subdir, recompute the tree hash,
	// then copy the verified subdir into the install cache.
	scratchRoot, err := CloneScratchDir()
	if err != nil {
		return "", output.NewStructuredErrorDetail(
			output.ErrSystemHomedir,
			"cannot resolve scratch dir for source clone",
			"Ensure $TMPDIR is writable.",
			err.Error(),
		)
	}
	cloneDir, err := os.MkdirTemp(scratchRoot, "syllago-moat-src-*")
	if err != nil {
		return "", output.NewStructuredErrorDetail(
			output.ErrSystemIO,
			"could not create temp dir for source clone",
			"Ensure $TMPDIR is writable.",
			err.Error(),
		)
	}
	// MkdirTemp creates the dir; the cloner expects an absent path. Remove
	// the empty dir before cloning, and ensure cleanup runs whether the
	// install succeeds or fails.
	_ = os.Remove(cloneDir)
	defer func() { _ = os.RemoveAll(cloneDir) }()

	if cloneErr := moat.CloneRepoFn(ctx, entry.SourceURI, cloneDir); cloneErr != nil {
		return "", output.NewStructuredErrorDetail(
			output.ErrMoatInvalid,
			fmt.Sprintf("could not clone source repo for %s/%s", registryName, entry.Name),
			"Check network access to the source repository and that the repo exists at the URL in the manifest.",
			cloneErr.Error(),
		)
	}

	itemDir := filepath.Join(cloneDir, categoryDir, entry.Name)
	if info, statErr := os.Stat(itemDir); statErr != nil || !info.IsDir() {
		return "", output.NewStructuredError(
			output.ErrMoatInvalid,
			fmt.Sprintf("item not found in source repo for %s/%s", registryName, entry.Name),
			fmt.Sprintf("Expected directory %s/%s in the source repo. The publisher may have moved or renamed the item, or the repo uses a non-canonical layout (.moat/publisher.yml is not yet supported by syllago install).", categoryDir, entry.Name),
		)
	}

	actualHash, hashErr := moat.ContentHash(itemDir)
	if hashErr != nil {
		return "", output.NewStructuredErrorDetail(
			output.ErrMoatInvalid,
			fmt.Sprintf("could not compute content hash for %s/%s", registryName, entry.Name),
			"The cloned source tree could not be hashed (often a symlink or a path-collision under NFC normalization).",
			hashErr.Error(),
		)
	}
	if !strings.EqualFold(actualHash, entry.ContentHash) {
		return "", output.NewStructuredErrorDetail(
			output.ErrMoatInvalid,
			fmt.Sprintf("content_hash mismatch for %s/%s", registryName, entry.Name),
			"The current source-tree hash does not match the hash the manifest attested. The publisher's source has drifted since the registry indexed it — re-sync the registry or contact the registry operator.",
			fmt.Sprintf("expected=%s got=%s", entry.ContentHash, actualHash),
		)
	}

	if err := moat.CopyTree(itemDir, targetDir); err != nil {
		return "", output.NewStructuredErrorDetail(
			output.ErrSystemIO,
			fmt.Sprintf("could not stage verified source for %s/%s", registryName, entry.Name),
			"Check filesystem permissions on the syllago cache directory.",
			err.Error(),
		)
	}

	if _, err := installer.RecordInstall(lf, entry, registryURL, rekorBundle, Now()); err != nil {
		return "", output.NewStructuredErrorDetail(
			output.ErrMoatInvalid,
			fmt.Sprintf("could not record install of %s/%s in lockfile", registryName, entry.Name),
			"This is a programmer error; re-run with --dry-run and report the issue.",
			err.Error(),
		)
	}
	if err := lf.Save(lockfilePath); err != nil {
		return "", output.NewStructuredErrorDetail(
			output.ErrMoatInvalid,
			fmt.Sprintf("could not save moat lockfile after recording %s/%s", registryName, entry.Name),
			"Check filesystem permissions on .syllago/moat-lockfile.json.",
			err.Error(),
		)
	}

	return targetDir, nil
}

// shortHashCache returns the first 12 hex chars after "sha256:". Used for
// cache-directory pathing where a full 64-char hash would make the tree
// visually noisy in ls output.
func shortHashCache(contentHash string) string {
	stripped := strings.TrimPrefix(contentHash, "sha256:")
	if len(stripped) > 12 {
		return stripped[:12]
	}
	return stripped
}
