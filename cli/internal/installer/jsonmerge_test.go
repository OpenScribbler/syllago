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

func TestReadJSONFile_ExistingFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.json")
	os.WriteFile(filePath, []byte(`{"key":"value"}`), 0644)

	data, err := readJSONFile(filePath)
	if err != nil {
		t.Fatalf("readJSONFile: %v", err)
	}
	if string(data) != `{"key":"value"}` {
		t.Errorf("got %q, want %q", string(data), `{"key":"value"}`)
	}
}

func TestReadJSONFile_MissingFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	data, err := readJSONFile(filepath.Join(tmpDir, "nonexistent.json"))
	if err != nil {
		t.Fatalf("readJSONFile should not error for missing file: %v", err)
	}
	if string(data) != "{}" {
		t.Errorf("expected empty JSON object, got %q", string(data))
	}
}

func TestBackupFile_ExistingFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(filePath, []byte(`{"original":"data"}`), 0644)

	if err := backupFile(filePath); err != nil {
		t.Fatalf("backupFile: %v", err)
	}

	bakData, err := os.ReadFile(filePath + ".bak")
	if err != nil {
		t.Fatalf("reading backup: %v", err)
	}
	if string(bakData) != `{"original":"data"}` {
		t.Errorf("backup content = %q", string(bakData))
	}
}

func TestBackupFile_MissingFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	// Backing up a non-existent file should succeed (nothing to back up)
	if err := backupFile(filepath.Join(tmpDir, "nonexistent.json")); err != nil {
		t.Fatalf("backupFile should not error for missing file: %v", err)
	}
}

func TestWriteJSONFile_OverwritesExisting(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "config.json")
	os.WriteFile(filePath, []byte(`{"old":"data"}`), 0644)

	if err := writeJSONFile(filePath, []byte(`{"new":"data"}`)); err != nil {
		t.Fatalf("writeJSONFile: %v", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != `{"new":"data"}` {
		t.Errorf("content = %q, want %q", string(data), `{"new":"data"}`)
	}
}
