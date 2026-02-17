package installer

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestWriteJSONFile_Atomic(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "config.json")

	// Write initial content
	initialData := []byte(`{"version": 1}`)
	if err := writeJSONFile(targetFile, initialData); err != nil {
		t.Fatal(err)
	}

	// Verify file exists and has correct content
	data, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(initialData) {
		t.Errorf("content mismatch: got %s", data)
	}
}

func TestWriteJSONFile_NoPartialWrites(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "config.json")

	// Write initial content
	initialData := []byte(`{"original": "data"}`)
	if err := os.WriteFile(targetFile, initialData, 0644); err != nil {
		t.Fatal(err)
	}

	// Monitor the file — it should never be empty or start with non-{
	done := make(chan bool)
	foundPartial := atomic.Bool{}

	go func() {
		for {
			select {
			case <-done:
				return
			default:
				if data, err := os.ReadFile(targetFile); err == nil {
					if len(data) == 0 {
						foundPartial.Store(true)
					}
					if len(data) > 0 && data[0] != '{' {
						foundPartial.Store(true)
					}
				}
				time.Sleep(1 * time.Millisecond)
			}
		}
	}()

	// Perform write
	newData := []byte(`{"updated": "content", "with": "more", "fields": "here"}`)
	if err := writeJSONFile(targetFile, newData); err != nil {
		t.Fatal(err)
	}

	close(done)
	time.Sleep(10 * time.Millisecond)

	if foundPartial.Load() {
		t.Fatal("file was in partial state during write (not atomic)")
	}

	// Verify final content
	finalData, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(finalData) != string(newData) {
		t.Errorf("final content mismatch: got %s", finalData)
	}
}

func TestWriteJSONFileWithPerm_RestrictedPermissions(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	data := []byte(`{"test": true}`)

	// 0600 permissions (home directory files)
	homeFile := filepath.Join(tmpDir, ".claude.json")
	if err := writeJSONFileWithPerm(homeFile, data, 0600); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(homeFile)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0600 {
		t.Errorf("home file permissions = %o, want 0600", mode)
	}

	// 0644 permissions (project files)
	projectFile := filepath.Join(tmpDir, "project", "config.json")
	if err := writeJSONFileWithPerm(projectFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	info2, err := os.Stat(projectFile)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info2.Mode().Perm(); mode != 0644 {
		t.Errorf("project file permissions = %o, want 0644", mode)
	}
}

func TestWriteJSONFile_CreatesParentDirs(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "sub", "dir", "config.json")

	if err := writeJSONFile(targetFile, []byte(`{}`)); err != nil {
		t.Fatalf("writeJSONFile failed to create parent dirs: %v", err)
	}

	if _, err := os.Stat(targetFile); err != nil {
		t.Fatal("file was not created")
	}
}
