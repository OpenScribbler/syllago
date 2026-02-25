// cli/internal/tui/golden_test.go
package tui

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// tempDirRe matches Go test temp directory paths like /tmp/TestFoo123456789/001.
// Used to normalize non-deterministic paths in golden file snapshots.
var tempDirRe = regexp.MustCompile(`/tmp/Test[A-Za-z0-9_]+/\d+`)

// updateGolden controls whether golden files are regenerated.
// Named "update-golden" to avoid colliding with charmbracelet/x/exp/golden's "-update" flag.
// Usage: go test -update-golden ./cli/internal/tui/...
var updateGolden = flag.Bool("update-golden", false, "update golden files")

// stripANSI removes ANSI escape sequences from s.
// NO_COLOR=1 (set in init()) handles most cases; this is belt-and-suspenders.
func stripANSI(s string) string {
	return ansi.Strip(s)
}

// requireGolden compares actual against a golden file at testdata/<name>.golden.
// Pass -update to regenerate: go test -update ./cli/internal/tui/...
func requireGolden(t *testing.T, name string, actual string) {
	t.Helper()
	goldenPath := filepath.Join("testdata", name+".golden")

	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("create testdata dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(actual), 0o644); err != nil {
			t.Fatalf("write golden file %s: %v", goldenPath, err)
		}
		t.Logf("updated golden: %s", goldenPath)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("golden file %s not found — run with -update-golden to create it: %v", goldenPath, err)
	}

	if string(expected) != actual {
		t.Errorf("golden file mismatch: %s\n\n"+
			"If this UI change was intentional, update the golden files:\n"+
			"  go test ./cli/internal/tui/... -update-golden\n"+
			"  git diff cli/internal/tui/testdata/   # review the changes\n\n"+
			"Diff:\n%s",
			goldenPath, diffStrings(string(expected), actual))
	}
}

// diffStrings produces a simple line-by-line diff between want and got.
func diffStrings(want, got string) string {
	wLines := strings.Split(want, "\n")
	gLines := strings.Split(got, "\n")

	var sb strings.Builder
	max := len(wLines)
	if len(gLines) > max {
		max = len(gLines)
	}
	for i := 0; i < max; i++ {
		var w, g string
		if i < len(wLines) {
			w = wLines[i]
		}
		if i < len(gLines) {
			g = gLines[i]
		}
		if w != g {
			fmt.Fprintf(&sb, "line %d:\n  want: %q\n  got:  %q\n", i+1, w, g)
		}
	}
	return sb.String()
}

// normalizeSnapshot replaces non-deterministic content in a rendered TUI snapshot:
// - Go test temp dir paths (e.g. /tmp/TestFoo123/001) → <TESTDIR>
// - Trailing whitespace on each line (varies with path length after substitution)
func normalizeSnapshot(s string) string {
	s = tempDirRe.ReplaceAllString(s, "<TESTDIR>")
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return strings.Join(lines, "\n")
}

// TestGoldenSmoke verifies the golden infrastructure compiles and runs.
func TestGoldenSmoke(t *testing.T) {
	// Just verify the flag is registered and the helpers compile.
	_ = *updateGolden
	result := stripANSI("\x1b[31mred\x1b[0m")
	if result != "red" {
		t.Fatalf("stripANSI: got %q, want %q", result, "red")
	}
}
