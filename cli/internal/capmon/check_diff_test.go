package capmon_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// makeLines creates a string with n unique lines, each containing a prefix
// and line number so no two lines are equal.
func makeLines(prefix string, n int) []byte {
	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteString(fmt.Sprintf("%s%06d\n", prefix, i))
	}
	return []byte(sb.String())
}

func TestGenerateUnifiedDiff_NoTruncation(t *testing.T) {
	t.Parallel()
	old := makeLines("line-", 50)
	new_ := makeLines("chng-", 50)

	out, err := capmon.GenerateUnifiedDiff(old, new_, "docs/README.md", "documentation")
	if err != nil {
		t.Fatalf("GenerateUnifiedDiff: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty diff for changed content")
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	// With 50 old + 50 new, all different, expect at least 100 content lines.
	// Total should be under 200 (no truncation).
	if strings.Contains(out, "[truncated") {
		t.Errorf("expected no truncation for small diff, but got truncation indicator\n%s", out)
	}
	if len(lines) < 100 {
		t.Errorf("expected at least 100 lines in diff, got %d", len(lines))
	}
}

func TestGenerateUnifiedDiff_SourceCodeTruncation(t *testing.T) {
	t.Parallel()
	// 600 old lines, empty new — produces 600 delete lines + 2 headers = 602 total.
	// source_code truncation limit is 500, so output should be truncated.
	old := makeLines("old-", 600)
	new_ := []byte{}

	out, err := capmon.GenerateUnifiedDiff(old, new_, "src/model.rs", "source_code")
	if err != nil {
		t.Fatalf("GenerateUnifiedDiff: %v", err)
	}
	if !strings.Contains(out, "[truncated") {
		t.Errorf("expected truncation indicator for 600-line source_code diff, got:\n%s", out[:min(200, len(out))])
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) > 501 { // 500 lines + truncation indicator
		t.Errorf("expected at most 501 lines after truncation, got %d", len(lines))
	}
}

func TestGenerateUnifiedDiff_OtherTruncation(t *testing.T) {
	t.Parallel()
	// 300 old lines, empty new — produces 300 delete lines + 2 headers = 302 total.
	// documentation truncation limit is 200, so output should be truncated.
	old := makeLines("doc-", 300)
	new_ := []byte{}

	out, err := capmon.GenerateUnifiedDiff(old, new_, "docs/skills.md", "documentation")
	if err != nil {
		t.Fatalf("GenerateUnifiedDiff: %v", err)
	}
	if !strings.Contains(out, "[truncated") {
		t.Errorf("expected truncation indicator for 300-line documentation diff, got:\n%s", out[:min(200, len(out))])
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) > 201 { // 200 lines + truncation indicator
		t.Errorf("expected at most 201 lines after truncation, got %d", len(lines))
	}
}

func TestGenerateUnifiedDiff_TruncationIndicator(t *testing.T) {
	t.Parallel()
	old := makeLines("x-", 300)
	new_ := []byte{}

	out, err := capmon.GenerateUnifiedDiff(old, new_, "docs/page.md", "documentation")
	if err != nil {
		t.Fatalf("GenerateUnifiedDiff: %v", err)
	}
	// Truncation indicator must contain the line count and bytes-shown info.
	if !strings.Contains(out, "[truncated after 200 lines") {
		t.Errorf("truncation indicator should contain 'truncated after 200 lines', got:\n%s", lastLine(out))
	}
	if !strings.Contains(out, "bytes shown") {
		t.Errorf("truncation indicator should contain 'bytes shown', got:\n%s", lastLine(out))
	}
	if !strings.Contains(out, ".capmon-cache/") {
		t.Errorf("truncation indicator should mention '.capmon-cache/', got:\n%s", lastLine(out))
	}
}

func TestGenerateUnifiedDiff_NoChange(t *testing.T) {
	t.Parallel()
	content := makeLines("same-", 100)
	out, err := capmon.GenerateUnifiedDiff(content, content, "docs/page.md", "documentation")
	if err != nil {
		t.Fatalf("GenerateUnifiedDiff: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty diff for identical content, got %d bytes", len(out))
	}
}

func TestGenerateUnifiedDiff_Headers(t *testing.T) {
	t.Parallel()
	old := []byte("line one\nline two\n")
	new_ := []byte("line one\nline THREE\n")
	out, err := capmon.GenerateUnifiedDiff(old, new_, "docs/example.md", "documentation")
	if err != nil {
		t.Fatalf("GenerateUnifiedDiff: %v", err)
	}
	if !strings.HasPrefix(out, "--- a/docs/example.md\n") {
		t.Errorf("expected --- header, got: %q", out[:min(50, len(out))])
	}
	if !strings.Contains(out, "+++ b/docs/example.md\n") {
		t.Errorf("expected +++ header in: %q", out[:min(100, len(out))])
	}
}

// lastLine returns the last non-empty line of a string.
func lastLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) == 0 {
		return ""
	}
	return lines[len(lines)-1]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
