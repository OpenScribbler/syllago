package installer

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// CopyContent copies a file or directory from src to dst.
func CopyContent(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}
	if info.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst)
}

func copyFile(src, dst string) (err error) {
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	// Defense-in-depth: refuse to write through a symlink at the destination
	// (prevents arbitrary file overwrite when processing untrusted content).
	// The atomic rename below is the real TOCTOU fix — this check is an
	// early-exit for a clear error message.
	if info, err := os.Lstat(dst); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("destination is a symlink: %s (refusing to follow for security)", dst)
		}
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	// Write to a temp file in the same directory as dst, then atomically
	// rename. This eliminates the TOCTOU window between the symlink check
	// and file creation — os.Rename replaces the name (including symlinks)
	// rather than following them.
	tmp, err := os.CreateTemp(dstDir, ".syllago-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		// Clean up the temp file on any error path.
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err = io.Copy(tmp, in); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpPath, dst)
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip symlinks in source tree to prevent information disclosure
		// from untrusted content repositories.
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, relPath)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}
		return copyFile(path, targetPath)
	})
}
