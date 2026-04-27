package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestScanRealRepo(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("skipping: cannot determine home directory")
	}
	repoRoot := filepath.Join(home, ".local", "src", "syllago")
	if _, err := os.Stat(repoRoot); os.IsNotExist(err) {
		t.Skip("skipping: real repo not available (CI environment)")
	}
	contentRoot := filepath.Join(repoRoot, "content")
	if _, err := os.Stat(contentRoot); os.IsNotExist(err) {
		contentRoot = repoRoot // fall back for pre-restructure state
	}
	cat, err := Scan(contentRoot, repoRoot)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	counts := cat.CountByType()
	fmt.Printf("Skills: %d, Agents: %d, MCP: %d\n",
		counts[Skills], counts[Agents], counts[MCP])
	fmt.Printf("Rules: %d, Hooks: %d, Commands: %d\n",
		counts[Rules], counts[Hooks], counts[Commands])
	for _, item := range cat.Items {
		if !item.Type.IsUniversal() {
			fmt.Printf("  %-10s %-15s %-25s desc=%q files=%d\n",
				item.Type, item.Provider, item.Name, item.Description, len(item.Files))
		}
	}

	if counts[Hooks] == 0 {
		t.Error("expected at least 1 hook in content/hooks/benchmark/")
	}
	if counts[Rules] == 0 {
		t.Error("expected at least 1 rule in content/rules/")
	}
	if counts[Skills] == 0 {
		t.Error("expected at least 1 skill in content/skills/")
	}

	// Verify all provider-specific items have descriptions
	for _, item := range cat.Items {
		if !item.Type.IsUniversal() && item.Description == "" {
			t.Errorf("%s/%s/%s has empty description", item.Type, item.Provider, item.Name)
		}
	}
}
