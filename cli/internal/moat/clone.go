package moat

// Source-repo cloning helpers, shared by sync-time content cache writes
// (contentcache.go) and install-time fetch (moatinstall/fetch.go).
//
// The MOAT spec defines content[].source_uri as the source repository URI
// (moat-spec.md line 781) and content_hash as a content-tree merkle hash
// computed by the algorithm in moat_hash.py (sorted "<file_hash>  <path>"
// lines, UTF-8 BOM stripped, CRLF normalized for text). Both call sites must
// materialize the source repo and locate the item subdirectory at
// <category_dir>/<entry.name>/ to verify or render the tree.
//
// History: prior versions of moatinstall treated source_uri as a tarball URL
// and hashed the response body with sha256. That collapsed two distinct
// algorithms into one and could only succeed against synthetic fixtures whose
// ContentHash equaled sha256(tarball). Against any conforming registry it
// failed with a content_hash mismatch.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CloneRepoFunc is the test-injectable signature for source-repo
// materialization. Production uses cloneRepoShallow (git clone --depth=1
// with hardened flags). Tests pass a closure that copies a fixture into
// destDir so they exercise the full hash-verify + extract path without
// spawning git.
type CloneRepoFunc func(ctx context.Context, sourceURI, destDir string) error

// CloneRepoFn is the package-level seam for callers that want to swap the
// production cloner without threading a parameter through every layer.
// moatinstall.FetchAndRecord reads this directly. New callers SHOULD prefer
// passing a CloneRepoFunc explicitly (e.g. WriteContentCache) so test
// substitution is local rather than global.
var CloneRepoFn CloneRepoFunc = cloneRepoShallow

// cloneRepoShallow runs `git clone --depth=1` of sourceURI into destDir
// using the same hardening flags as registry.cloneArgs:
//   - core.hooksPath=/dev/null (no clone-side hook execution)
//   - --no-recurse-submodules (no transitive code we didn't ask for)
//   - GIT_CONFIG_NOSYSTEM=1 (no system-wide config influence)
//
// destDir MUST not already exist; the function creates it. Caller is
// responsible for cleaning up on success and failure.
func cloneRepoShallow(ctx context.Context, sourceURI, destDir string) error {
	if err := checkGit(); err != nil {
		return err
	}
	if _, err := os.Stat(destDir); err == nil {
		return fmt.Errorf("clone destination already exists: %s", destDir)
	}
	if err := os.MkdirAll(filepath.Dir(destDir), 0o755); err != nil {
		return fmt.Errorf("creating clone parent dir: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git",
		"-c", "core.hooksPath=/dev/null",
		"clone",
		"--depth=1",
		"--no-recurse-submodules",
		"--quiet",
		sourceURI, destDir,
	)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.RemoveAll(destDir)
		return fmt.Errorf("git clone failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// checkGit returns an error if git is not on PATH.
func checkGit() error {
	_, err := exec.LookPath("git")
	if err != nil {
		return errors.New("git is required for MOAT operations but was not found on PATH")
	}
	return nil
}

// ValidateSourceURI rejects anything other than https:// repo URLs. The
// MOAT spec example uses "https://github.com/owner/repo" (line 750-ish);
// we reject other schemes (git://, ssh://, git+https://) until a use case
// surfaces. URL parse errors and empty paths surface here so the caller
// gets a structured error before paying for a clone attempt.
func ValidateSourceURI(sourceURI string) error {
	if sourceURI == "" {
		return errors.New("source_uri is empty")
	}
	u, err := url.Parse(sourceURI)
	if err != nil {
		return fmt.Errorf("source_uri parse: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("source_uri scheme not supported: %q (only https://)", u.Scheme)
	}
	if u.Host == "" || u.Path == "" || u.Path == "/" {
		return fmt.Errorf("source_uri missing host or path: %s", sourceURI)
	}
	return nil
}

// CopyTree recursively copies srcDir into dstDir. dstDir is removed first
// so a partial copy from a prior failed install never leaves stale files
// alongside fresh ones.
//
// Symlinks are rejected — moat.ContentHash already enforces a no-symlinks
// policy on the source tree, so any symlink encountered here indicates
// something walked outside the verified subtree (a bug or attempted
// escape). Non-regular non-dir entries (devices, fifos, sockets) are
// silently skipped, mirroring the prior tarball-extraction policy.
func CopyTree(srcDir, dstDir string) error {
	if err := os.RemoveAll(dstDir); err != nil {
		return fmt.Errorf("clearing dst: %w", err)
	}
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("creating dst: %w", err)
	}

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(srcDir, path)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dstDir, rel)

		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink rejected during copy: %s", rel)
		}
		switch {
		case info.IsDir():
			return os.MkdirAll(target, info.Mode().Perm()|0o700)
		case info.Mode().IsRegular():
			return copyRegularFile(path, target, info.Mode())
		default:
			return nil
		}
	})
}

// copyRegularFile is a small helper isolated for testability. Modes are
// preserved up to the unix permission bits.
func copyRegularFile(srcPath, dstPath string, mode os.FileMode) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open src %s: %w", srcPath, err)
	}
	defer func() { _ = src.Close() }()

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("mkdir parent of %s: %w", dstPath, err)
	}
	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm()|0o600)
	if err != nil {
		return fmt.Errorf("create dst %s: %w", dstPath, err)
	}
	defer func() { _ = dst.Close() }()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy %s -> %s: %w", srcPath, dstPath, err)
	}
	return nil
}
