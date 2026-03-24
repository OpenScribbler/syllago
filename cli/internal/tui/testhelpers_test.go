package tui

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

var updateGolden = flag.Bool("update-golden", false, "update golden files")

// --- Key helpers ---

func keyRune(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func keyPress(k tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: k}
}

var (
	keyUp    = keyPress(tea.KeyUp)
	keyDown  = keyPress(tea.KeyDown)
	keyEnter = keyPress(tea.KeyEnter)
	keyEsc   = keyPress(tea.KeyEsc)
	keyTab   = keyPress(tea.KeyTab)
)

func pressN(m tea.Model, key tea.Msg, n int) tea.Model {
	for i := 0; i < n; i++ {
		m, _ = m.Update(key)
	}
	return m
}

// --- Test data ---

func testCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	return &catalog.Catalog{}
}

func testProviders() []provider.Provider {
	return nil
}

func testConfig() *config.Config {
	return &config.Config{}
}

// --- App construction ---

func testApp(t *testing.T) App {
	return testAppSize(t, 80, 30)
}

func testAppSize(t *testing.T, w, h int) App {
	t.Helper()
	app := NewApp(testCatalog(t), testProviders(), "0.0.0-test", false, nil, testConfig(), false, "")
	m, _ := app.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m.(App)
}

// --- Golden file helpers ---

var tempDirRe = regexp.MustCompile(`/tmp/Test[A-Za-z0-9_]+/\d+`)

func normalizeSnapshot(s string) string {
	s = ansi.Strip(s)
	s = tempDirRe.ReplaceAllString(s, "<TESTDIR>")
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return strings.Join(lines, "\n")
}

func snapshotApp(t *testing.T, app App) string {
	t.Helper()
	return normalizeSnapshot(app.View())
}

func requireGolden(t *testing.T, name string, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *updateGolden {
		os.MkdirAll("testdata", 0o755)
		os.WriteFile(path, []byte(got), 0o644)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden file %s not found (run with -update-golden to create)", path)
	}
	if string(want) != got {
		t.Errorf("golden mismatch for %s:\n%s", name, diffStrings(string(want), got))
	}
}

func diffStrings(want, got string) string {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")
	var sb strings.Builder
	maxLen := len(wantLines)
	if len(gotLines) > maxLen {
		maxLen = len(gotLines)
	}
	for i := 0; i < maxLen; i++ {
		wl, gl := "", ""
		if i < len(wantLines) {
			wl = wantLines[i]
		}
		if i < len(gotLines) {
			gl = gotLines[i]
		}
		if wl != gl {
			sb.WriteString(fmt.Sprintf("--- want line %d:\n  %s\n+++ got  line %d:\n  %s\n", i, wl, i, gl))
		}
	}
	if len(wantLines) != len(gotLines) {
		sb.WriteString(fmt.Sprintf("line count: want %d, got %d\n", len(wantLines), len(gotLines)))
	}
	return sb.String()
}

// --- Assertion helpers ---

func assertContains(t *testing.T, view, substr string) {
	t.Helper()
	stripped := ansi.Strip(view)
	if !strings.Contains(stripped, substr) {
		t.Errorf("view does not contain %q\n\nView:\n%s", substr, stripped)
	}
}

func assertNotContains(t *testing.T, view, substr string) {
	t.Helper()
	stripped := ansi.Strip(view)
	if strings.Contains(stripped, substr) {
		t.Errorf("view unexpectedly contains %q", substr)
	}
}
