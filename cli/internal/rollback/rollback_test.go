package rollback

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

func TestPlanForErrors(t *testing.T) {
	tests := []struct {
		name      string
		item      func(t *testing.T) catalog.ContentItem
		wantError string
	}{
		{
			name: "not registry sourced",
			item: func(t *testing.T) catalog.ContentItem {
				return catalog.ContentItem{
					Name: "local-skill",
					Type: catalog.Skills,
					Meta: &metadata.Meta{},
				}
			},
			wantError: "is not registry-sourced",
		},
		{
			name: "no install record store",
			item: func(t *testing.T) catalog.ContentItem {
				withConfigDir(t, t.TempDir())
				return catalog.ContentItem{
					Name: "canary-skill",
					Type: catalog.Skills,
					Meta: &metadata.Meta{SourceRegistry: "test-reg"},
				}
			},
			wantError: "no install record",
		},
		{
			name: "previous nil",
			item: func(t *testing.T) catalog.ContentItem {
				configDir := t.TempDir()
				withConfigDir(t, configDir)
				libraryPath := filepath.Join(t.TempDir(), "canary-skill")
				if err := os.MkdirAll(libraryPath, 0755); err != nil {
					t.Fatalf("MkdirAll libraryPath: %v", err)
				}
				if err := os.WriteFile(filepath.Join(libraryPath, "SKILL.md"), []byte("# Canary\n"), 0644); err != nil {
					t.Fatalf("WriteFile library item: %v", err)
				}
				coord := installstore.Coord{
					Registry: "test-reg",
					Type:     string(catalog.Skills),
					Name:     "canary-skill",
				}
				if err := installstore.RecordInstallMeta(filepath.Join(configDir, "installs.json"), coord, libraryPath, installstore.PlacementInput{
					Provider:  "claude-code",
					Mechanism: installstore.MechanismSymlink,
					Path:      filepath.Join(t.TempDir(), "canary-skill"),
				}, installstore.InstallMeta{}, time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)); err != nil {
					t.Fatalf("RecordInstallMeta: %v", err)
				}
				return catalog.ContentItem{
					Name: "canary-skill",
					Type: catalog.Skills,
					Meta: &metadata.Meta{SourceRegistry: "test-reg"},
				}
			},
			wantError: "no rollback data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := PlanFor(tt.item(t))
			if err == nil {
				t.Fatal("PlanFor returned nil error")
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("PlanFor error = %v, want substring %q", err, tt.wantError)
			}
		})
	}
}

func TestCopyTreeCopiesNestedFilesSkipsSymlinksAndRefusesDestinationSymlink(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	dst := filepath.Join(root, "dst")
	nested := filepath.Join(src, "nested")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("MkdirAll nested: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "file.txt"), []byte("contents\n"), 0644); err != nil {
		t.Fatalf("WriteFile source: %v", err)
	}
	if err := os.Symlink(filepath.Join("nested", "file.txt"), filepath.Join(src, "link.txt")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	if err := copyTree(src, dst); err != nil {
		t.Fatalf("copyTree: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dst, "nested", "file.txt"))
	if err != nil {
		t.Fatalf("ReadFile copied file: %v", err)
	}
	if string(data) != "contents\n" {
		t.Fatalf("copied file = %q, want contents", data)
	}
	if _, err := os.Lstat(filepath.Join(dst, "link.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("destination symlink copy err = %v, want not exist", err)
	}

	dstWithSymlink := filepath.Join(root, "dst-with-symlink")
	if err := os.MkdirAll(filepath.Join(dstWithSymlink, "nested"), 0755); err != nil {
		t.Fatalf("MkdirAll dst nested: %v", err)
	}
	if err := os.Symlink(filepath.Join(root, "outside"), filepath.Join(dstWithSymlink, "nested", "file.txt")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	err = copyTree(src, dstWithSymlink)
	if err == nil {
		t.Fatal("copyTree overwrote destination symlink")
	}
	if !strings.Contains(err.Error(), "destination is a symlink") {
		t.Fatalf("copyTree error = %v, want destination symlink error", err)
	}
}

func TestShortSHA(t *testing.T) {
	tests := []struct {
		name string
		sha  string
		want string
	}{
		{
			name: "long",
			sha:  "1234567890abcdef",
			want: "1234567890ab",
		},
		{
			name: "short",
			sha:  "1234567",
			want: "1234567",
		},
		{
			name: "exact",
			sha:  "1234567890ab",
			want: "1234567890ab",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShortSHA(tt.sha); got != tt.want {
				t.Fatalf("ShortSHA(%q) = %q, want %q", tt.sha, got, tt.want)
			}
		})
	}
}

func withConfigDir(t *testing.T, dir string) {
	t.Helper()
	prev := config.GlobalDirOverride
	config.GlobalDirOverride = dir
	t.Cleanup(func() {
		config.GlobalDirOverride = prev
	})
}
