package main

// Source-fetcher for the MOAT registry-install Proceed path (ADR 0007
// Phase 2, bead syllago-u128o — fourth slice of parent bead syllago-svdwc).
//
// Scope of this slice:
//
//   - Fetch an HTTPS tarball source artifact from ContentEntry.SourceURI.
//     The sha256 of the fetched bytes MUST equal ContentEntry.ContentHash
//     before any filesystem side-effect happens.
//   - Extract the tarball into a per-registry, per-item content cache under
//     ~/.cache/syllago/moat-sources/<registry>/<item>/<sha256-prefix>/.
//   - Record the install in the project lockfile via
//     installer.RecordInstall and persist with lf.Save. UNSIGNED tier uses
//     a null AttestationBundle; SIGNED/DUAL-ATTESTED tiers require a
//     per-item Rekor bundle that is not yet fetched by Sync — those tiers
//     return a structured MOAT_004 error pending a sibling bead.
//
// What this slice intentionally does NOT do:
//
//   - Provider-level install (copying content into ~/.claude/, symlinks,
//     hooks-JSON merge). `syllago install <registry>/<item>` only pins the
//     registry item into the project lockfile + content cache; a later
//     `syllago install <item>` flows through the existing library path.
//   - Fetch per-item Rekor bundles for SIGNED/DUAL-ATTESTED tiers. Those
//     require a GET to rekor.sigstore.dev/api/v1/log/entries?logIndex=<n>
//     plus re-verification via VerifyAttestationItem — tracked separately
//     since the scope is non-trivial and the MOAT fixtures in use are
//     UNSIGNED.
//   - Support git+https source URIs. Tarball covers the common publisher
//     pipeline; git clone-based publishing is a follow-up.
//
// Cache layout rationale: the sha256-prefix directory name lets the same
// content cache hold multiple versions of an item without name collisions
// while still being cheap to garbage-collect (prune subtrees with no
// corresponding lockfile entry).

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

// moatFetchClient is the HTTP client seam for source-artifact fetches.
// Tests swap this to a httptest.Server client. Default uses a 60s timeout
// to accommodate larger tarballs than manifest fetches.
var moatFetchClient = &http.Client{Timeout: 60 * time.Second}

// moatFetchMaxBytes caps the source-artifact size. 100 MiB is well above
// any reasonable content-item tarball but far below what a runaway or
// malicious registry could use to exhaust disk.
const moatFetchMaxBytes = 100 << 20

// moatSourceCacheDir returns the root cache directory for all MOAT source
// artifacts. Tests override via env var or by swapping this variable.
var moatSourceCacheDir = func() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "syllago", "moat-sources"), nil
}

// fetchAndRecord is the Proceed-branch action: download the source
// artifact, verify the sha256 matches, extract into the per-item cache,
// record in the lockfile, and save. Returns the absolute path of the
// extracted content directory on success.
//
// Caller responsibilities:
//   - gate decision has already cleared (PreInstallCheck returned Proceed
//     or an interactive prompt was accepted).
//   - lf is the in-memory lockfile; fetchAndRecord adds a LockEntry via
//     RecordInstall and calls lf.Save.
//   - registryName is the short config.Registry.Name (for cache pathing);
//     registryURL is the manifest URI that goes into LockEntry.Registry.
func fetchAndRecord(
	ctx context.Context,
	entry *moat.ContentEntry,
	registryName, registryURL, lockfilePath string,
	lf *moat.Lockfile,
) (string, error) {
	if entry == nil {
		return "", errors.New("fetchAndRecord: entry is nil")
	}
	if lf == nil {
		return "", errors.New("fetchAndRecord: lockfile is nil")
	}

	tier := entry.TrustTier()
	if tier != moat.TrustTierUnsigned {
		// SIGNED / DUAL-ATTESTED tiers require a per-item Rekor bundle to
		// satisfy installer.BuildLockEntry (non-null AttestationBundle). The
		// bundle is not carried on SyncResult — fetching it is a separate
		// slice of work. Fail loudly rather than silently accept a weaker
		// lockfile entry.
		return "", output.NewStructuredError(
			output.ErrMoatInvalid,
			fmt.Sprintf("install of %s/%s resolved to trust tier %s; per-item Rekor bundle fetching is not yet wired", registryName, entry.Name, tier.String()),
			"Run `syllago install "+registryName+"/"+entry.Name+" --dry-run` to inspect the resolution, or pin a SIGNED/DUAL-ATTESTED item only after the per-item attestation fetcher ships.",
		)
	}

	if !strings.HasPrefix(entry.SourceURI, "https://") {
		return "", output.NewStructuredError(
			output.ErrMoatInvalid,
			fmt.Sprintf("source_uri scheme not supported for %s/%s (got %q)", registryName, entry.Name, entry.SourceURI),
			"Only https:// tarballs are supported by this release. git+https is planned in a follow-up bead.",
		)
	}

	cacheRoot, err := moatSourceCacheDir()
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

	// Fetch → hash-verify → extract. Each step maps to a specific error
	// code so operators can triage without reading Go.
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

	// Lockfile: UNSIGNED → null bundle. Clock seam shared with the sync
	// path so tests can pin pinned_at deterministically.
	if _, err := installer.RecordInstall(lf, entry, registryURL, nil, moatInstallNow()); err != nil {
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

// downloadTarball GETs url, enforcing moatFetchMaxBytes. Returns the full
// body on success. Short-circuits on non-200 and on oversize responses
// before reading any further.
func downloadTarball(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("User-Agent", moat.DefaultUserAgent)

	resp, err := moatFetchClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http GET: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d (%s)", resp.StatusCode, resp.Status)
	}

	limited := io.LimitReader(resp.Body, moatFetchMaxBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}
	if int64(len(body)) > moatFetchMaxBytes {
		return nil, fmt.Errorf("source artifact exceeds %d bytes", moatFetchMaxBytes)
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

		// Path-traversal defense. filepath.Clean normalizes separators and
		// collapses .., then we require the result to stay inside destDir
		// after a Join. An absolute Name (starts with / on unix) fails the
		// containment check because Join resets the base.
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
			if _, err := io.Copy(f, io.LimitReader(tr, moatFetchMaxBytes)); err != nil {
				_ = f.Close()
				return fmt.Errorf("writing %q: %w", target, err)
			}
			if err := f.Close(); err != nil {
				return fmt.Errorf("closing %q: %w", target, err)
			}
		default:
			// Skip symlinks, devices, FIFOs. A hostile tarball cannot plant a
			// symlink that the next consumer would traverse.
			continue
		}
	}
	return nil
}

// sha256HexOf returns the lowercase hex sha256 of body. The lockfile hash
// format is "sha256:<hex>", but ContentEntry.ContentHash carries the
// prefix already — callers compare the returned hex against the hex half
// after the colon via strings.EqualFold on a trimmed value. Here we
// accept a plain bytes input and return the hex half; the caller handles
// the prefix stripping.
func sha256HexOf(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// shortHashCache returns the first 12 hex chars after the "sha256:"
// prefix. Used for cache-directory pathing where a full 64-char hash
// would make the tree visually noisy in ls output.
func shortHashCache(contentHash string) string {
	stripped := strings.TrimPrefix(contentHash, "sha256:")
	if len(stripped) > 12 {
		return stripped[:12]
	}
	return stripped
}
