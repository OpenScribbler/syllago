package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/nesco/cli/internal/config"
)

// setupGoProject creates a temporary Go project with nesco config for testing.
func setupGoProject(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module example.com/proj\n\ngo 1.22\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644)
	// Create .nesco config so scan doesn't prompt
	nescoDir := filepath.Join(tmp, ".nesco")
	os.MkdirAll(nescoDir, 0755)
	cfg := config.Config{Providers: []string{"claude-code"}}
	data, _ := json.Marshal(cfg)
	os.WriteFile(filepath.Join(nescoDir, "config.json"), data, 0644)
	return tmp
}
