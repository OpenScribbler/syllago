package converter

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSplitterFixturesPresent is a sanity check that every synthesized
// splitter fixture listed in docs/plans/2026-04-23-rules-splitter-decisions.md
// (D19) exists on disk. Phase 2 asserts against the fixture contents; Phase 1
// only guarantees presence.
func TestSplitterFixturesPresent(t *testing.T) {
	fixtures := []string{
		"h2-clean.md",
		"h2-with-preamble.md",
		"h2-numbered-prefix.md",
		"h2-emoji-prefix.md",
		"h3-deep.md",
		"h4-rare.md",
		"marker-literal.md",
		"too-small.md",
		"no-h2.md",
		"delegating-stub.md",
		"table-heavy.md",
		"decorative-hr.md",
		"must-should-may.md",
		"trailing-whitespace.md",
		"crlf-line-endings.md",
		"bom-prefix.md",
		"no-trailing-newline.md",
		"import-line.md",
		"cursorrules-flat-numbered.md",
		"cursorrules-points-elsewhere.md",
		"clinerules-numbered-h2.md",
		"windsurfrules-pointer.md",
		"windsurfrules-numbered-rules.md",
	}
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("testdata", "splitter", name)
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("fixture %s not found: %v", name, err)
			}
			if info.IsDir() {
				t.Fatalf("fixture %s is a directory, want file", name)
			}
		})
	}
}
