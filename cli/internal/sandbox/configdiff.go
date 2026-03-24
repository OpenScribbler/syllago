package sandbox

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// ConfigSnapshot records the pre-sandbox state of a config file.
type ConfigSnapshot struct {
	OriginalPath string // absolute path to the original file/dir
	StagedPath   string // absolute path to the copy in staging
	OriginalHash []byte // SHA-256 of original content (file) or merkle of dir
}

// StageConfigs copies globalConfigPaths into stagingDir/config/ and records hashes.
// Paths that do not exist are skipped (provider may not have created them yet).
func StageConfigs(stagingDir string, globalConfigPaths []string) ([]ConfigSnapshot, error) {
	destBase := filepath.Join(stagingDir, "config")
	if err := os.MkdirAll(destBase, 0700); err != nil {
		return nil, fmt.Errorf("creating config staging dir: %w", err)
	}

	var snapshots []ConfigSnapshot
	for _, src := range globalConfigPaths {
		info, err := os.Stat(src)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", src, err)
		}

		// Derive a unique dest path (preserve base name to avoid collisions).
		dest := filepath.Join(destBase, filepath.Base(src))

		if info.IsDir() {
			if err := copyDir(src, dest); err != nil {
				return nil, fmt.Errorf("copying dir %s: %w", src, err)
			}
		} else {
			if err := copyFile(src, dest); err != nil {
				return nil, fmt.Errorf("copying file %s: %w", src, err)
			}
		}

		hash, err := hashPath(src)
		if err != nil {
			return nil, fmt.Errorf("hashing %s: %w", src, err)
		}

		snapshots = append(snapshots, ConfigSnapshot{
			OriginalPath: src,
			StagedPath:   dest,
			OriginalHash: hash,
		})
	}
	return snapshots, nil
}

// protectStagedFiles makes specific files read-only in staged config directories.
// This prevents providers from deleting credential files during keychain migration
// while still allowing reads. Patterns are matched against file basenames.
func protectStagedFiles(snapshots []ConfigSnapshot, patterns []string) {
	for _, snap := range snapshots {
		info, err := os.Stat(snap.StagedPath)
		if err != nil || !info.IsDir() {
			continue
		}
		_ = filepath.WalkDir(snap.StagedPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			base := filepath.Base(path)
			for _, pattern := range patterns {
				if matched, _ := filepath.Match(pattern, base); matched {
					_ = os.Chmod(path, 0444)
				}
			}
			return nil
		})
	}
}

// shouldSkipDiff checks if a config path matches any of the skip patterns.
// Patterns are matched against the file's base name.
func shouldSkipDiff(originalPath string, patterns []string) bool {
	base := filepath.Base(originalPath)
	for _, pattern := range patterns {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}
	return false
}

// DiffResult describes changes to one config path after the sandbox session.
type DiffResult struct {
	Snapshot   ConfigSnapshot
	Changed    bool
	IsHighRisk bool   // true if diff contains MCP server or hook changes
	DiffText   string // human-readable unified diff
}

// ComputeDiffs compares staged copies against their recorded original hashes.
// Returns one DiffResult per snapshot that was changed.
func ComputeDiffs(snapshots []ConfigSnapshot) ([]DiffResult, error) {
	var results []DiffResult
	for _, snap := range snapshots {
		currentHash, err := hashPath(snap.StagedPath)
		if err != nil {
			// Staged file deleted: treat as high-risk change (config removed).
			results = append(results, DiffResult{
				Snapshot:   snap,
				Changed:    true,
				IsHighRisk: true,
				DiffText:   "(config file was deleted inside sandbox)",
			})
			continue
		}

		if string(currentHash) == string(snap.OriginalHash) {
			continue // unchanged
		}

		diff, highRisk := buildDiff(snap.OriginalPath, snap.StagedPath)
		results = append(results, DiffResult{
			Snapshot:   snap,
			Changed:    true,
			IsHighRisk: highRisk,
			DiffText:   diff,
		})
	}
	return results, nil
}

// ApplyDiff copies the staged version back to the original path.
// If the staged copy no longer exists, the change is skipped — we never delete
// the user's original config based on a missing staged file.
// Call this only after user approval.
func ApplyDiff(result DiffResult) error {
	info, err := os.Stat(result.Snapshot.StagedPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("staged copy no longer exists — skipping to protect original at %s", result.Snapshot.OriginalPath)
		}
		return fmt.Errorf("staged path error: %w", err)
	}
	if info.IsDir() {
		return copyDir(result.Snapshot.StagedPath, result.Snapshot.OriginalPath)
	}
	return copyFile(result.Snapshot.StagedPath, result.Snapshot.OriginalPath)
}

// hashPath returns SHA-256 of a file, or a deterministic hash of a directory tree.
func hashPath(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	if info.IsDir() {
		err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			// Skip symlinks — they may be broken and can't be hashed by content.
			if d.Type()&os.ModeSymlink != 0 {
				return nil
			}
			rel, _ := filepath.Rel(path, p)
			fmt.Fprintf(h, "%s\x00", rel)
			if !d.IsDir() {
				f, err := os.Open(p)
				if err != nil {
					return err
				}
				defer func() { _ = f.Close() }()
				if _, err := io.Copy(h, f); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer func() { _ = f.Close() }()
		if _, err := io.Copy(h, f); err != nil {
			return nil, err
		}
	}
	return h.Sum(nil), nil
}

// buildDiff returns a human-readable diff and whether it's high-risk.
// High-risk: either version contains "mcpServers", "hooks", or "commands" keys.
// Handles both file and directory paths — for directories it walks both sides
// to detect changed, new, and deleted files.
func buildDiff(orig, staged string) (string, bool) {
	info, err := os.Stat(staged)
	if err != nil {
		return "(staged path unreadable)", false
	}

	if info.IsDir() {
		var sb strings.Builder
		highRisk := false

		// Walk staged to find changed and new files.
		_ = filepath.WalkDir(staged, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(staged, p)
			origFile := filepath.Join(orig, rel)
			origData, _ := os.ReadFile(origFile)
			stagedData, _ := os.ReadFile(p)
			if string(origData) != string(stagedData) {
				origLabel := origFile
				if len(origData) == 0 {
					origLabel = "/dev/null"
				}
				fmt.Fprintf(&sb, "--- %s\n+++ %s\n%s", origLabel, p, unifiedDiff(origData, stagedData))
				if isHighRiskDiff(origData, stagedData) {
					highRisk = true
				}
			}
			return nil
		})

		// Walk original to find files deleted inside the sandbox.
		_ = filepath.WalkDir(orig, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(orig, p)
			stagedFile := filepath.Join(staged, rel)
			if _, statErr := os.Stat(stagedFile); errors.Is(statErr, fs.ErrNotExist) {
				origData, _ := os.ReadFile(p)
				fmt.Fprintf(&sb, "--- %s\n+++ /dev/null\n%s", p, deletedDiff(origData))
				if hasHighRiskKeys(origData) {
					highRisk = true
				}
			}
			return nil
		})

		return sb.String(), highRisk
	}

	// File path.
	origData, _ := os.ReadFile(orig)
	stagedData, _ := os.ReadFile(staged)
	diff := fmt.Sprintf("--- %s\n+++ %s\n%s",
		orig, staged,
		unifiedDiff(origData, stagedData),
	)
	highRisk := isHighRiskDiff(origData, stagedData)
	return diff, highRisk
}

// hasHighRiskKeys checks if data contains MCP server, hooks, or commands definitions.
func hasHighRiskKeys(data []byte) bool {
	s := string(data)
	return strings.Contains(s, `"mcpServers"`) ||
		strings.Contains(s, `"hooks"`) ||
		strings.Contains(s, `"commands"`)
}

// isHighRiskDiff returns true if either the original or staged content contains
// high-risk keys (MCP servers, hooks, commands). Conservative: any change to a
// file containing these keys requires explicit approval, even if the change
// doesn't touch the high-risk sections directly.
func isHighRiskDiff(origData, stagedData []byte) bool {
	return hasHighRiskKeys(origData) || hasHighRiskKeys(stagedData)
}

// deletedDiff formats removed lines for a deleted file.
func deletedDiff(data []byte) string {
	var out strings.Builder
	for _, line := range strings.Split(string(data), "\n") {
		out.WriteString("-" + line + "\n")
	}
	return out.String()
}

// unifiedDiff produces a simple line-diff between two byte slices.
func unifiedDiff(a, b []byte) string {
	aLines := strings.Split(string(a), "\n")
	bLines := strings.Split(string(b), "\n")
	var out strings.Builder
	for _, line := range aLines {
		out.WriteString("-" + line + "\n")
	}
	for _, line := range bLines {
		out.WriteString("+" + line + "\n")
	}
	return out.String()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	_, err = io.Copy(out, in)
	return err
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)

		// Handle symlinks: preserve valid ones that stay within the source tree.
		// Symlinks pointing outside the source directory are a security risk —
		// a malicious project could use them to overwrite arbitrary files when
		// ApplyDiff copies staged content back.
		if d.Type()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return nil // skip unreadable symlinks
			}
			// Resolve the symlink target to an absolute, clean path.
			absTarget := linkTarget
			if !filepath.IsAbs(linkTarget) {
				absTarget = filepath.Join(filepath.Dir(path), linkTarget)
			}
			absTarget, err = filepath.EvalSymlinks(absTarget)
			if err != nil {
				return nil // skip broken/unresolvable symlinks
			}
			// Verify the resolved target is within the source directory.
			absSrc, err := filepath.EvalSymlinks(src)
			if err != nil {
				return nil
			}
			if !strings.HasPrefix(absTarget, absSrc+string(filepath.Separator)) && absTarget != absSrc {
				log.Printf("sandbox: skipping symlink %s → %s (escapes source dir %s)", path, absTarget, absSrc)
				return nil
			}
			if _, err := os.Stat(absTarget); err != nil {
				return nil // skip broken symlinks
			}
			return os.Symlink(linkTarget, target)
		}

		if d.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		return copyFile(path, target)
	})
}
