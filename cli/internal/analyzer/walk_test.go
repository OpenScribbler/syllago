package analyzer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0755)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeTestFile %s: %v", path, err)
	}
}

func TestWalk_EmptyDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	result := Walk(root, nil)
	if len(result.Paths) != 0 {
		t.Errorf("expected 0 paths, got %d: %v", len(result.Paths), result.Paths)
	}
	if len(result.Warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d: %v", len(result.Warnings), result.Warnings)
	}
}

func TestWalk_ExcludesNodeModules(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "src", "app.ts"), "code")
	writeTestFile(t, filepath.Join(root, "node_modules", "pkg", "index.js"), "dep")

	result := Walk(root, nil)
	for _, p := range result.Paths {
		if strings.Contains(p, "node_modules") {
			t.Errorf("node_modules path should be excluded: %s", p)
		}
	}
	if len(result.Paths) != 1 {
		t.Errorf("expected 1 path, got %d: %v", len(result.Paths), result.Paths)
	}
}

func TestWalk_ExtraExcludeDirs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "src", "app.ts"), "code")
	writeTestFile(t, filepath.Join(root, "custom-out", "bundle.js"), "build output")

	result := Walk(root, []string{"custom-out"})
	for _, p := range result.Paths {
		if strings.Contains(p, "custom-out") {
			t.Errorf("extra exclude dir should be excluded: %s", p)
		}
	}
	if len(result.Paths) != 1 {
		t.Errorf("expected 1 path, got %d: %v", len(result.Paths), result.Paths)
	}
}

func TestWalk_ExcludesBinaryFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "app.go"), "code")
	writeTestFile(t, filepath.Join(root, "image.png"), "fake png")
	writeTestFile(t, filepath.Join(root, "tool.exe"), "fake exe")

	result := Walk(root, nil)
	for _, p := range result.Paths {
		ext := filepath.Ext(p)
		if ext == ".png" || ext == ".exe" {
			t.Errorf("binary file should be excluded: %s", p)
		}
	}
	if len(result.Paths) != 1 {
		t.Errorf("expected 1 path (app.go), got %d: %v", len(result.Paths), result.Paths)
	}
}

func TestWalk_DepthLimitWarning(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Create a directory tree deeper than walkMaxDepth (30).
	deepDir := root
	for i := 0; i < 35; i++ {
		deepDir = filepath.Join(deepDir, "d")
	}
	writeTestFile(t, filepath.Join(deepDir, "deep.txt"), "deep content")

	result := Walk(root, nil)

	foundDepthWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "max depth") {
			foundDepthWarning = true
			break
		}
	}
	if !foundDepthWarning {
		t.Error("expected a depth limit warning, got none")
	}
}

func TestWalk_FileLimitWarning(t *testing.T) {
	// Mutates package-level testMaxFiles; see .claude/rules/cli-test-patterns.md
	// "Skip t.Parallel() when mutating globals."
	root := t.TempDir()

	// Create more files than the test limit.
	origLimit := testMaxFiles
	testMaxFiles = 5
	t.Cleanup(func() { testMaxFiles = origLimit })

	for i := 0; i < 10; i++ {
		writeTestFile(t, filepath.Join(root, filepath.Base(root)+string(rune('a'+i))+".txt"), "content")
	}

	result := Walk(root, nil)

	if len(result.Paths) > 5 {
		t.Errorf("expected at most 5 paths, got %d", len(result.Paths))
	}

	foundLimitWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "file limit") {
			foundLimitWarning = true
			break
		}
	}
	if !foundLimitWarning {
		t.Error("expected a file limit warning, got none")
	}
}
