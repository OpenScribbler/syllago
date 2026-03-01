package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/config"
)

// setupGoProject creates a temporary Go project with syllago config for testing.
func setupGoProject(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module example.com/proj\n\ngo 1.22\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644)
	// Create .syllago config so scan doesn't prompt
	syllagoDir := filepath.Join(tmp, ".syllago")
	os.MkdirAll(syllagoDir, 0755)
	cfg := config.Config{Providers: []string{"claude-code"}}
	data, _ := json.Marshal(cfg)
	os.WriteFile(filepath.Join(syllagoDir, "config.json"), data, 0644)
	return tmp
}
