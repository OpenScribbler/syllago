package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func TestSyncAndExportCommandRegisters(t *testing.T) {
	// Verify the command is registered on rootCmd.
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "sync-and-export" {
			found = true
			break
		}
	}
	if !found {
		t.Error("sync-and-export command not registered on rootCmd")
	}
}

func TestSyncAndExportFlagsDefined(t *testing.T) {
	flags := syncAndExportCmd.Flags()
	for _, name := range []string{"to", "type", "name", "source", "llm-hooks"} {
		if flags.Lookup(name) == nil {
			t.Errorf("missing --%s flag on sync-and-export", name)
		}
	}
}

func TestSyncAndExportNoRegistries(t *testing.T) {
	// When there are no registries, sync is a no-op and export runs normally.
	root := setupExportRepo(t)
	withFakeRepoRoot(t, root)

	installBase := t.TempDir()
	orig := append([]provider.Provider(nil), provider.AllProviders...)
	provider.AllProviders = []provider.Provider{
		{
			Name: "TestProv",
			Slug: "test-prov",
			InstallDir: func(homeDir string, ct catalog.ContentType) string {
				if ct == catalog.Skills {
					return filepath.Join(installBase, string(ct))
				}
				return ""
			},
			SupportsType: func(ct catalog.ContentType) bool {
				return ct == catalog.Skills
			},
		},
	}
	t.Cleanup(func() { provider.AllProviders = orig })

	// Create .syllago/config.json with no registries.
	syllagoDir := filepath.Join(root, ".syllago")
	os.MkdirAll(syllagoDir, 0755)
	os.WriteFile(filepath.Join(syllagoDir, "config.json"), []byte(`{"providers":[]}`), 0644)

	stdout, _ := output.SetForTest(t)

	syncAndExportCmd.Flags().Set("to", "test-prov")
	defer syncAndExportCmd.Flags().Set("to", "")
	syncAndExportCmd.Flags().Set("type", "skills")
	defer syncAndExportCmd.Flags().Set("type", "")
	syncAndExportCmd.Flags().Set("source", "shared")
	defer syncAndExportCmd.Flags().Set("source", "local")

	err := syncAndExportCmd.RunE(syncAndExportCmd, []string{})
	if err != nil {
		t.Fatalf("sync-and-export failed: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "greeting") {
		t.Errorf("expected exported skill 'greeting' in output, got: %s", out)
	}
}
