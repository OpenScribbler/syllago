package installer

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHashFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("hello world"), 0644)

	hash, err := HashFile(path)
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}
	// SHA-256 of "hello world"
	if hash != "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9" {
		t.Errorf("unexpected hash: %s", hash)
	}
}

func TestHashFileNotFound(t *testing.T) {
	t.Parallel()
	_, err := HashFile("/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestHashBytes(t *testing.T) {
	t.Parallel()
	hash := HashBytes([]byte("hello world"))
	if hash != "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9" {
		t.Errorf("unexpected hash: %s", hash)
	}
}

func TestHashBytesEmpty(t *testing.T) {
	t.Parallel()
	hash := HashBytes([]byte{})
	// SHA-256 of empty input
	if hash != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Errorf("unexpected hash for empty input: %s", hash)
	}
}

func TestVerifyIntegrity_NoInstalled(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	drifted, err := VerifyIntegrity(dir)
	if err != nil {
		t.Fatalf("VerifyIntegrity: %v", err)
	}
	if len(drifted) != 0 {
		t.Errorf("expected no drift for empty installed.json, got %d", len(drifted))
	}
}

func TestVerifyIntegrity_Clean(t *testing.T) {
	dir := t.TempDir()

	// Create a target file
	targetFile := filepath.Join(dir, "content.md")
	os.WriteFile(targetFile, []byte("original content"), 0644)
	hash, _ := HashFile(targetFile)

	// Write installed.json with matching hash
	inst := &Installed{
		Symlinks: []InstalledSymlink{
			{
				Path:        filepath.Join(dir, "link"),
				Target:      targetFile,
				ContentHash: hash,
				Source:      "test",
				InstalledAt: time.Now(),
			},
		},
	}
	SaveInstalled(dir, inst)

	drifted, err := VerifyIntegrity(dir)
	if err != nil {
		t.Fatalf("VerifyIntegrity: %v", err)
	}
	if len(drifted) != 0 {
		t.Errorf("expected no drift, got %d entries: %+v", len(drifted), drifted)
	}
}

func TestVerifyIntegrity_Modified(t *testing.T) {
	dir := t.TempDir()

	targetFile := filepath.Join(dir, "content.md")
	os.WriteFile(targetFile, []byte("original content"), 0644)
	hash, _ := HashFile(targetFile)

	inst := &Installed{
		Symlinks: []InstalledSymlink{
			{
				Path:        filepath.Join(dir, "link"),
				Target:      targetFile,
				ContentHash: hash,
				Source:      "test",
				InstalledAt: time.Now(),
			},
		},
	}
	SaveInstalled(dir, inst)

	// Tamper with the file
	os.WriteFile(targetFile, []byte("tampered content"), 0644)

	drifted, err := VerifyIntegrity(dir)
	if err != nil {
		t.Fatalf("VerifyIntegrity: %v", err)
	}
	if len(drifted) != 1 {
		t.Fatalf("expected 1 drift entry, got %d", len(drifted))
	}
	if drifted[0].Status != "modified" {
		t.Errorf("expected status 'modified', got %q", drifted[0].Status)
	}
	if drifted[0].ActualHash == "" {
		t.Error("expected actual hash to be populated")
	}
}

func TestVerifyIntegrity_Missing(t *testing.T) {
	dir := t.TempDir()

	inst := &Installed{
		Symlinks: []InstalledSymlink{
			{
				Path:        filepath.Join(dir, "link"),
				Target:      filepath.Join(dir, "nonexistent.md"),
				ContentHash: "abc123",
				Source:      "test",
				InstalledAt: time.Now(),
			},
		},
	}
	SaveInstalled(dir, inst)

	drifted, err := VerifyIntegrity(dir)
	if err != nil {
		t.Fatalf("VerifyIntegrity: %v", err)
	}
	if len(drifted) != 1 {
		t.Fatalf("expected 1 drift entry, got %d", len(drifted))
	}
	if drifted[0].Status != "missing" {
		t.Errorf("expected status 'missing', got %q", drifted[0].Status)
	}
}

func TestVerifyIntegrity_SkipsNoHash(t *testing.T) {
	dir := t.TempDir()

	// Entry without ContentHash (pre-existing install)
	inst := &Installed{
		Symlinks: []InstalledSymlink{
			{
				Path:   filepath.Join(dir, "link"),
				Target: filepath.Join(dir, "whatever"),
				Source: "test",
			},
		},
	}
	SaveInstalled(dir, inst)

	drifted, err := VerifyIntegrity(dir)
	if err != nil {
		t.Fatalf("VerifyIntegrity: %v", err)
	}
	if len(drifted) != 0 {
		t.Errorf("expected no drift for entries without hash, got %d", len(drifted))
	}
}
