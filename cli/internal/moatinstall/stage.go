package moatinstall

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
)

// StageIntoLibrary copies a verified MOAT source-cache tree into the global
// library and writes item metadata attributing it to the registry. Returns a
// ContentItem rooted at the library dir with Registry set.
func StageIntoLibrary(cacheDir string, entry *moat.ContentEntry, regName, globalDir string, now time.Time) (catalog.ContentItem, error) {
	if entry == nil {
		return catalog.ContentItem{}, fmt.Errorf("StageIntoLibrary: entry is nil")
	}
	if cacheDir == "" {
		return catalog.ContentItem{}, fmt.Errorf("StageIntoLibrary: cacheDir is empty")
	}
	info, err := os.Stat(cacheDir)
	if err != nil {
		return catalog.ContentItem{}, fmt.Errorf("StageIntoLibrary: cacheDir %q: %w", cacheDir, err)
	}
	if !info.IsDir() {
		return catalog.ContentItem{}, fmt.Errorf("StageIntoLibrary: cacheDir %q is not a directory", cacheDir)
	}

	ct, ok := moat.FromMOATType(entry.Type)
	if !ok {
		return catalog.ContentItem{}, fmt.Errorf("StageIntoLibrary: unknown MOAT type %q", entry.Type)
	}

	destDir := filepath.Join(globalDir, string(ct), entry.Name)
	item := catalog.ContentItem{
		Name:        entry.Name,
		DisplayName: entry.DisplayName,
		Type:        ct,
		Path:        destDir,
		Registry:    regName,
	}

	meta, err := metadata.Load(destDir)
	if err != nil {
		return catalog.ContentItem{}, fmt.Errorf("load library metadata: %w", err)
	}
	exists, err := pathExists(destDir)
	if err != nil {
		return catalog.ContentItem{}, fmt.Errorf("stat library destination: %w", err)
	}

	if exists {
		if meta != nil && meta.SourceRegistry == regName {
			if meta.SourceHash == entry.ContentHash {
				return item, nil
			}
			if err := os.RemoveAll(destDir); err != nil {
				return catalog.ContentItem{}, fmt.Errorf("remove existing registry content: %w", err)
			}
		} else {
			return catalog.ContentItem{}, fmt.Errorf("library already contains %s/%s not sourced from registry %q", string(ct), entry.Name, regName)
		}
	}

	if err := copyDir(cacheDir, destDir); err != nil {
		return catalog.ContentItem{}, fmt.Errorf("stage registry content into library: %w", err)
	}

	m := &metadata.Meta{
		Name:           entry.Name,
		Type:           string(ct),
		SourceType:     "registry",
		SourceRegistry: regName,
		SourceURL:      entry.SourceURI,
		SourceHash:     entry.ContentHash,
		AddedAt:        &now,
		AddedBy:        "syllago moat install",
	}
	if err := metadata.Save(destDir, m); err != nil {
		return catalog.ContentItem{}, fmt.Errorf("save library metadata: %w", err)
	}

	return item, nil
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func copyFile(src, dst string) (err error) {
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}

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

	tmp, err := os.CreateTemp(dstDir, ".syllago-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
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
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, relPath)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}
		return copyFile(path, targetPath)
	})
}
