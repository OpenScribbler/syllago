package tui

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

var updateGolden = flag.Bool("update-golden", false, "update golden files")

// requireGolden compares got against a golden file. If -update-golden is set,
// it writes got to the golden file instead.
func requireGolden(t *testing.T, name, got string) {
	t.Helper()

	// Normalize: strip trailing whitespace per line
	var normalized []string
	for _, line := range strings.Split(got, "\n") {
		normalized = append(normalized, strings.TrimRight(line, " \t"))
	}
	got = strings.Join(normalized, "\n")

	goldenPath := filepath.Join("testdata", name+".golden")

	if *updateGolden {
		err := os.WriteFile(goldenPath, []byte(got), 0644)
		if err != nil {
			t.Fatalf("failed to write golden file: %v", err)
		}
		return
	}

	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("golden file not found: %s\nRun with -update-golden to create it.\n\nGot:\n%s", goldenPath, got)
	}

	if got != string(expected) {
		t.Errorf("output does not match golden file %s\n\nGot:\n%s\n\nExpected:\n%s\n\nTo update:\n  go test ./internal/tui/... -update-golden\n  git diff internal/tui/testdata/",
			goldenPath, got, string(expected))
	}
}

func goldenApp(t *testing.T, w, h int) App {
	t.Helper()
	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "alpha-skill", Type: catalog.Skills, Source: "team-rules", Description: "A helpful skill"},
			{Name: "beta-skill", Type: catalog.Skills, Source: "library"},
			{Name: "gamma-skill", Type: catalog.Skills, Source: "team-rules"},
			{Name: "code-reviewer", Type: catalog.Agents, Source: "team-rules", Description: "Reviews code"},
			{Name: "docs-writer", Type: catalog.Agents, Source: "library"},
			{Name: "postgres-mcp", Type: catalog.MCP, Source: "team-rules"},
			{Name: "strict-types", Type: catalog.Rules, Source: "library"},
			{Name: "pre-commit", Type: catalog.Hooks, Source: "team-rules"},
		},
	}
	provs := []provider.Provider{
		{Name: "Claude Code", Slug: "claude-code", Detected: true},
		{Name: "Gemini CLI", Slug: "gemini-cli", Detected: true},
	}
	app := NewApp(cat, provs, "v1.0.0", false, nil, &config.Config{}, false, "/tmp/test")
	m, _ := app.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m.(App)
}

func TestGolden_FullApp_120x40(t *testing.T) {
	app := goldenApp(t, 120, 40)
	requireGolden(t, "fullapp-120x40", app.View())
}

func TestGolden_FullApp_80x30(t *testing.T) {
	app := goldenApp(t, 80, 30)
	requireGolden(t, "fullapp-80x30", app.View())
}
