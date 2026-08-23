package moatinstall

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
)

func TestStageIntoLibraryFreshStage(t *testing.T) {
	t.Parallel()

	cacheDir := writeStageCache(t, map[string]string{
		"SKILL.md":        "# My Skill\n",
		"nested/info.txt": "details\n",
	})
	globalDir := t.TempDir()
	now := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
	entry := stageEntry("my-skill", "skill", "sha256:aaaaaaaa", "https://github.com/example/repo")
	entry.DisplayName = "My Skill"

	item, err := StageIntoLibrary(cacheDir, entry, "example", globalDir, now)
	if err != nil {
		t.Fatalf("StageIntoLibrary: %v", err)
	}

	destDir := filepath.Join(globalDir, "skills", "my-skill")
	if item.Name != "my-skill" || item.DisplayName != "My Skill" || item.Type != catalog.Skills || item.Path != destDir || item.Registry != "example" {
		t.Fatalf("item = %#v, want staged skill item rooted at %s with Registry example", item, destDir)
	}
	assertFileContent(t, filepath.Join(destDir, "SKILL.md"), "# My Skill\n")
	assertFileContent(t, filepath.Join(destDir, "nested", "info.txt"), "details\n")

	meta, err := metadata.Load(destDir)
	if err != nil {
		t.Fatalf("metadata.Load: %v", err)
	}
	if meta == nil {
		t.Fatal("metadata missing")
	}
	if meta.Name != "my-skill" ||
		meta.Type != string(catalog.Skills) ||
		meta.SourceType != "registry" ||
		meta.SourceRegistry != "example" ||
		meta.SourceURL != "https://github.com/example/repo" ||
		meta.SourceHash != "sha256:aaaaaaaa" ||
		meta.AddedBy != "syllago moat install" {
		t.Fatalf("metadata = %#v", meta)
	}
	if meta.AddedAt == nil || !meta.AddedAt.Equal(now) {
		t.Fatalf("AddedAt = %v, want %v", meta.AddedAt, now)
	}
}

func TestStageIntoLibraryIdempotentSameHashDoesNotRewrite(t *testing.T) {
	t.Parallel()

	cacheDir := writeStageCache(t, map[string]string{"SKILL.md": "# v1\n"})
	globalDir := t.TempDir()
	now := time.Date(2026, 8, 22, 11, 0, 0, 0, time.UTC)
	entry := stageEntry("my-skill", "skill", "sha256:aaaaaaaa", "https://github.com/example/repo")

	if _, err := StageIntoLibrary(cacheDir, entry, "example", globalDir, now); err != nil {
		t.Fatalf("first StageIntoLibrary: %v", err)
	}
	destDir := filepath.Join(globalDir, "skills", "my-skill")
	canary := filepath.Join(destDir, "canary.txt")
	if err := os.WriteFile(canary, []byte("keep me\n"), 0o644); err != nil {
		t.Fatalf("write canary: %v", err)
	}
	metaPath := metadata.MetaPath(destDir)
	oldTime := time.Date(2026, 8, 21, 1, 2, 3, 0, time.UTC)
	if err := os.Chtimes(metaPath, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes metadata: %v", err)
	}
	before, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}

	item, err := StageIntoLibrary(cacheDir, entry, "example", globalDir, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("second StageIntoLibrary: %v", err)
	}
	after, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read metadata after: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("metadata was rewritten\nbefore:\n%s\nafter:\n%s", before, after)
	}
	info, err := os.Stat(metaPath)
	if err != nil {
		t.Fatalf("stat metadata: %v", err)
	}
	if !info.ModTime().Equal(oldTime) {
		t.Fatalf("metadata mtime = %v, want unchanged %v", info.ModTime(), oldTime)
	}
	assertFileContent(t, canary, "keep me\n")
	if item.Path != destDir || item.Registry != "example" {
		t.Fatalf("item = %#v, want existing staged item", item)
	}
}

func TestStageIntoLibraryUpdatesSameRegistryDifferentHash(t *testing.T) {
	t.Parallel()

	globalDir := t.TempDir()
	firstCache := writeStageCache(t, map[string]string{"old.txt": "old\n"})
	entry := stageEntry("my-skill", "skill", "sha256:aaaaaaaa", "https://github.com/example/repo")
	if _, err := StageIntoLibrary(firstCache, entry, "example", globalDir, time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("first StageIntoLibrary: %v", err)
	}

	secondCache := writeStageCache(t, map[string]string{"new.txt": "new\n"})
	entry.ContentHash = "sha256:bbbbbbbb"
	if _, err := StageIntoLibrary(secondCache, entry, "example", globalDir, time.Date(2026, 8, 22, 13, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("second StageIntoLibrary: %v", err)
	}

	destDir := filepath.Join(globalDir, "skills", "my-skill")
	if _, err := os.Stat(filepath.Join(destDir, "old.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old file stat error = %v, want not exist", err)
	}
	assertFileContent(t, filepath.Join(destDir, "new.txt"), "new\n")
	meta, err := metadata.Load(destDir)
	if err != nil {
		t.Fatalf("metadata.Load: %v", err)
	}
	if meta.SourceHash != "sha256:bbbbbbbb" {
		t.Fatalf("SourceHash = %q, want updated hash", meta.SourceHash)
	}
}

func TestStageIntoLibraryConflicts(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		setup func(t *testing.T, destDir string)
	}{
		{
			name: "no metadata",
			setup: func(t *testing.T, destDir string) {
				t.Helper()
				if err := os.MkdirAll(destDir, 0o755); err != nil {
					t.Fatalf("mkdir dest: %v", err)
				}
				if err := os.WriteFile(filepath.Join(destDir, "local.txt"), []byte("local\n"), 0o644); err != nil {
					t.Fatalf("write local file: %v", err)
				}
			},
		},
		{
			name: "different source registry",
			setup: func(t *testing.T, destDir string) {
				t.Helper()
				if err := metadata.Save(destDir, &metadata.Meta{
					Name:           "my-skill",
					Type:           string(catalog.Skills),
					SourceRegistry: "other",
					SourceHash:     "sha256:aaaaaaaa",
				}); err != nil {
					t.Fatalf("metadata.Save: %v", err)
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cacheDir := writeStageCache(t, map[string]string{"SKILL.md": "# registry\n"})
			globalDir := t.TempDir()
			destDir := filepath.Join(globalDir, "skills", "my-skill")
			tc.setup(t, destDir)

			_, err := StageIntoLibrary(cacheDir, stageEntry("my-skill", "skill", "sha256:aaaaaaaa", "https://github.com/example/repo"), "example", globalDir, time.Now())
			if err == nil {
				t.Fatal("StageIntoLibrary returned nil error")
			}
			if !strings.Contains(err.Error(), `library already contains skills/my-skill not sourced from registry "example"`) {
				t.Fatalf("error = %q", err)
			}
		})
	}
}

func TestStageIntoLibraryRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	cacheDir := writeStageCache(t, map[string]string{"SKILL.md": "# registry\n"})
	globalDir := t.TempDir()

	cases := []struct {
		name     string
		cacheDir string
		entry    *moat.ContentEntry
		want     string
	}{
		{
			name:     "nil entry",
			cacheDir: cacheDir,
			entry:    nil,
			want:     "entry is nil",
		},
		{
			name:     "missing cache dir",
			cacheDir: filepath.Join(t.TempDir(), "missing"),
			entry:    stageEntry("my-skill", "skill", "sha256:aaaaaaaa", "https://github.com/example/repo"),
			want:     "cacheDir",
		},
		{
			name:     "unknown type",
			cacheDir: cacheDir,
			entry:    stageEntry("my-skill", "loadout", "sha256:aaaaaaaa", "https://github.com/example/repo"),
			want:     `unknown MOAT type "loadout"`,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := StageIntoLibrary(tc.cacheDir, tc.entry, "example", globalDir, time.Now())
			if err == nil {
				t.Fatal("StageIntoLibrary returned nil error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want substring %q", err, tc.want)
			}
		})
	}
}

func writeStageCache(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir parent: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write cache file: %v", err)
		}
	}
	return dir
}

func stageEntry(name, typ, hash, sourceURI string) *moat.ContentEntry {
	return &moat.ContentEntry{
		Name:        name,
		DisplayName: name,
		Type:        typ,
		ContentHash: hash,
		SourceURI:   sourceURI,
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("%s = %q, want %q", path, data, want)
	}
}
