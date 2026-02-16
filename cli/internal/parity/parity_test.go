package parity

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
	"github.com/holdenhewett/romanesco/cli/internal/provider"
)

func TestAnalyzeFindsGaps(t *testing.T) {
	tmp := t.TempDir()

	// Create Claude Code rules but no Cursor rules
	os.MkdirAll(filepath.Join(tmp, ".claude", "rules"), 0755)
	os.WriteFile(filepath.Join(tmp, "CLAUDE.md"), []byte("# Rules"), 0644)
	os.MkdirAll(filepath.Join(tmp, ".cursor"), 0755) // empty

	providers := []provider.Provider{
		{
			Name: "Claude Code", Slug: "claude-code",
			DiscoveryPaths: func(root string, ct catalog.ContentType) []string {
				if ct == catalog.Rules {
					return []string{
						filepath.Join(root, "CLAUDE.md"),
						filepath.Join(root, ".claude", "rules"),
					}
				}
				return nil
			},
			SupportsType: func(ct catalog.ContentType) bool {
				return ct == catalog.Rules
			},
		},
		{
			Name: "Cursor", Slug: "cursor",
			DiscoveryPaths: func(root string, ct catalog.ContentType) []string {
				if ct == catalog.Rules {
					return []string{filepath.Join(root, ".cursor", "rules")}
				}
				return nil
			},
			SupportsType: func(ct catalog.ContentType) bool {
				return ct == catalog.Rules
			},
		},
	}

	report := Analyze(providers, tmp)

	if len(report.Gaps) == 0 {
		t.Error("expected at least one gap")
	}
	if len(report.Gaps) > 0 && report.Gaps[0].ContentType != catalog.Rules {
		t.Errorf("gap content type = %q, want rules", report.Gaps[0].ContentType)
	}
}

func TestAnalyzeNoGaps(t *testing.T) {
	tmp := t.TempDir()
	// No content for any provider

	providers := []provider.Provider{
		{
			Name: "Claude Code", Slug: "claude-code",
			DiscoveryPaths: func(root string, ct catalog.ContentType) []string {
				return nil
			},
		},
	}

	report := Analyze(providers, tmp)
	if len(report.Gaps) != 0 {
		t.Errorf("expected no gaps for empty project, got %d", len(report.Gaps))
	}
}
