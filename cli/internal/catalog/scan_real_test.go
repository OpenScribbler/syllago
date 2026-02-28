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
	repoRoot := filepath.Join(home, ".local", "src", "nesco")
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
	fmt.Printf("Skills: %d, Agents: %d, Prompts: %d, MCP: %d, Apps: %d\n",
		counts[Skills], counts[Agents], counts[Prompts],
		counts[MCP], counts[Apps])
	fmt.Printf("Rules: %d, Hooks: %d, Commands: %d\n",
		counts[Rules], counts[Hooks], counts[Commands])
	for _, item := range cat.Items {
		if !item.Type.IsUniversal() {
			fmt.Printf("  %-10s %-15s %-25s desc=%q files=%d readme=%d\n",
				item.Type, item.Provider, item.Name, item.Description, len(item.Files), len(item.ReadmeBody))
		}
	}

	if counts[Hooks] == 0 {
		t.Error("expected at least 1 hook after restructuring")
	}
	if counts[Rules] == 0 {
		t.Error("expected at least 1 rule after restructuring")
	}
	if counts[Commands] == 0 {
		t.Error("expected at least 1 command after restructuring")
	}

	// Verify all provider-specific items have descriptions
	for _, item := range cat.Items {
		if !item.Type.IsUniversal() && item.Description == "" {
			t.Errorf("%s/%s/%s has empty description", item.Type, item.Provider, item.Name)
		}
	}
}
