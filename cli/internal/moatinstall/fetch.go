package moatinstall

// FetchAndRecord — the Proceed-branch action for both CLI and TUI MOAT
// installs. Extracted from cmd/syllago/install_moat_fetch.go so the TUI can
// reach it (cmd/syllago is a main package and cannot be imported).
//
// Behavior is unchanged from the prior in-CLI implementation:
//
//   - SIGNED/DUAL-ATTESTED tiers: fetch the per-item Rekor entry by index,
//     verify against the supplied identity (per-item profile if present,
//     else the registry-level profile), then download + hash-verify +
//     extract + record.
//   - UNSIGNED tier: skip the Rekor + verify step, download + hash-verify +
//     extract + record with a null AttestationBundle.
//
// All failure modes return structured errors with stable codes — the
// caller (CLI or TUI) only has to surface them.

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// Client is the HTTP client used for source-artifact fetches. Tests swap
// this to a httptest.Server-backed client. 60s default timeout matches the
// CLI's pre-extraction value — large enough for typical content tarballs,
// small enough that a hung registry doesn't lock the install indefinitely.
var Client = &http.Client{Timeout: 60 * time.Second}

// MaxBytes caps the source-artifact size. 100 MiB is well above any
// reasonable content-item tarball and far below what a runaway or malicious
// registry could use to exhaust disk.
const MaxBytes = 100 << 20

// SourceCacheDir returns the root cache directory for MOAT source
// artifacts. Tests override by replacing this variable.
var SourceCacheDir = func() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "syllago", "moat-sources"), nil
}

// VerifyItem is the per-item verification seam. Production points at
// moat.VerifyAttestationItem (the full ECDSA → SET → inclusion proof →
// Fulcio identity → repo-ID binding chain). Wiring tests stub this to
// short-circuit the crypto — the real chain is exercised in
// internal/moat/item_verify_test.go.
var VerifyItem = moat.VerifyAttestationItem

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
//   - registryProfile is the manifest-level RegistrySigningProfile, used as
//     the verification identity for SIGNED items (no per-item profile).
//     Ignored for UNSIGNED items.
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
		profile := entry.SigningProfile
		if profile == nil {
			profile = registryProfile
		}
		if profile == nil {
			return "", output.NewStructuredError(
				output.ErrMoatInvalid,
				fmt.Sprintf("cannot verify %s/%s: no signing profile available (per-item or registry-level)", registryName, entry.Name),
				"This indicates a malformed manifest reached install — registry's signing profile must be pinned for SIGNED items. Re-sync the registry to surface the underlying error.",
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
		if _, vErr := VerifyItem(item, profile, raw, trustedRootJSON); vErr != nil {
			return "", output.NewStructuredErrorDetail(
				output.ErrMoatInvalid,
				fmt.Sprintf("attestation verification failed for %s/%s", registryName, entry.Name),
				"The per-item Rekor entry did not validate against the pinned signing profile. The publisher may have rotated identities or the manifest was tampered with — re-sync the registry and retry.",
				vErr.Error(),
			)
		}
		rekorBundle = raw
	}

	if !strings.HasPrefix(entry.SourceURI, "https://") {
		return "", output.NewStructuredError(
			output.ErrMoatInvalid,
			fmt.Sprintf("source_uri scheme not supported for %s/%s (got %q)", registryName, entry.Name, entry.SourceURI),
			"Only https:// tarballs are supported by this release. git+https is planned in a follow-up bead.",
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

	body, err := downloadTarball(ctx, entry.SourceURI)
	if err != nil {
		return "", output.NewStructuredErrorDetail(
			output.ErrMoatInvalid,
			fmt.Sprintf("could not fetch source artifact for %s/%s", registryName, entry.Name),
			"Check network access to the registry source URL.",
			err.Error(),
		)
	}

	actualHash := sha256HexOf(body)
	if !strings.EqualFold(actualHash, entry.ContentHash) {
		return "", output.NewStructuredErrorDetail(
			output.ErrMoatInvalid,
			fmt.Sprintf("source_uri content_hash mismatch for %s/%s", registryName, entry.Name),
			"The bytes served by the publisher do not match the hash the manifest attested. Contact the registry operator — the publisher re-uploaded a release without updating the manifest, or the artifact was tampered with in transit.",
			fmt.Sprintf("expected=%s got=%s", entry.ContentHash, actualHash),
		)
	}

	if err := extractGzipTarball(body, targetDir); err != nil {
		return "", output.NewStructuredErrorDetail(
			output.ErrSystemIO,
			fmt.Sprintf("could not extract source artifact for %s/%s", registryName, entry.Name),
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

// downloadTarball GETs url, enforcing MaxBytes. Returns the full body on
// success. Short-circuits on non-200 and on oversize responses before
// reading any further.
func downloadTarball(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("User-Agent", moat.DefaultUserAgent)

	resp, err := Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http GET: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d (%s)", resp.StatusCode, resp.Status)
	}

	limited := io.LimitReader(resp.Body, MaxBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}
	if int64(len(body)) > MaxBytes {
		return nil, fmt.Errorf("source artifact exceeds %d bytes", MaxBytes)
	}
	return body, nil
}

// extractGzipTarball writes each tar entry under destDir. Rejects path
// traversal (".." components, absolute paths, symlinks escaping destDir)
// and non-regular/non-dir entry types. Creates destDir fresh — an existing
// directory with prior content is removed first so a failed extraction
// cannot leave a half-populated cache.
func extractGzipTarball(body []byte, destDir string) error {
	if err := os.RemoveAll(destDir); err != nil {
		return fmt.Errorf("clearing destination: %w", err)
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("creating destination: %w", err)
	}

	gzr, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("opening gzip reader: %w", err)
	}
	defer func() { _ = gzr.Close() }()

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar header: %w", err)
		}

		cleaned := filepath.Clean(hdr.Name)
		if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, string(os.PathSeparator)+"..") || filepath.IsAbs(cleaned) {
			return fmt.Errorf("tar entry escapes destination: %q", hdr.Name)
		}
		target := filepath.Join(destDir, cleaned)
		if rel, relErr := filepath.Rel(destDir, target); relErr != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("tar entry escapes destination after join: %q", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("mkdir %q: %w", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("mkdir parent of %q: %w", target, err)
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				return fmt.Errorf("creating %q: %w", target, err)
			}
			if _, err := io.Copy(f, io.LimitReader(tr, MaxBytes)); err != nil {
				_ = f.Close()
				return fmt.Errorf("writing %q: %w", target, err)
			}
			if err := f.Close(); err != nil {
				return fmt.Errorf("closing %q: %w", target, err)
			}
		default:
			continue
		}
	}
	return nil
}

// sha256HexOf returns "sha256:<hex>" for body. Caller compares against
// ContentEntry.ContentHash via strings.EqualFold.
func sha256HexOf(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:])
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
