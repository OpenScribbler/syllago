package catalog

import (
	"fmt"
	"testing"
)

func TestScanRealRepo(t *testing.T) {
	cat, err := Scan("/home/hhewett/.local/src/romanesco")
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
