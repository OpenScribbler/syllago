package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/loadout"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// --- truncateStr (40% coverage) ---

func TestTruncateStr(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		in   string
		max  int
		want string
	}{
		{"short", 10, "short"},
		{"exact", 5, "exact"},
		{"longstring", 7, "long..."},
		{"longstring", 3, "lon"}, // max <= 3 hard cuts
		{"x", 1, "x"},
		{"abcdef", 1, "a"},
	} {
		if got := truncateStr(tc.in, tc.max); got != tc.want {
			t.Errorf("truncateStr(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
		}
	}
}

// --- appendUnique / appendUniqueInt (50% each) ---

func TestAppendUnique(t *testing.T) {
	t.Parallel()
	got := appendUnique([]string{"a", "b"}, "c")
	if len(got) != 3 || got[2] != "c" {
		t.Errorf("got %v, want [a b c]", got)
	}
	// Adding duplicate should leave slice unchanged.
	got = appendUnique([]string{"a", "b"}, "a")
	if len(got) != 2 {
		t.Errorf("got %v, want unchanged 2-element slice", got)
	}
	// Empty slice, new item.
	got = appendUnique(nil, "x")
	if len(got) != 1 || got[0] != "x" {
		t.Errorf("got %v, want [x]", got)
	}
}

func TestAppendUniqueInt(t *testing.T) {
	t.Parallel()
	got := appendUniqueInt([]int{1, 2}, 3)
	if len(got) != 3 || got[2] != 3 {
		t.Errorf("got %v, want [1 2 3]", got)
	}
	got = appendUniqueInt([]int{1, 2}, 1)
	if len(got) != 2 {
		t.Errorf("got %v, want unchanged", got)
	}
	got = appendUniqueInt(nil, 5)
	if len(got) != 1 || got[0] != 5 {
		t.Errorf("got %v, want [5]", got)
	}
}

// --- trustGlyph (50%) ---

func TestTrustGlyph(t *testing.T) {
	t.Parallel()
	if got := trustGlyph("Verified"); got != "✓" {
		t.Errorf("Verified → %q, want ✓", got)
	}
	if got := trustGlyph("Revoked"); got != "R" {
		t.Errorf("Revoked → %q, want R", got)
	}
	if got := trustGlyph(""); got != "" {
		t.Errorf("empty → %q, want empty", got)
	}
	if got := trustGlyph("Unknown"); got != "" {
		t.Errorf("Unknown → %q, want empty", got)
	}
}

// --- isChecked (66.7%) ---

func TestIsChecked(t *testing.T) {
	t.Parallel()
	w := initWizard{checks: []bool{true, false, true}}
	if !w.isChecked(0) {
		t.Error("isChecked(0) = false, want true")
	}
	if w.isChecked(1) {
		t.Error("isChecked(1) = true, want false")
	}
	if !w.isChecked(2) {
		t.Error("isChecked(2) = false, want true")
	}
	// Out-of-range returns false.
	if w.isChecked(-1) {
		t.Error("isChecked(-1) = true, want false")
	}
	if w.isChecked(99) {
		t.Error("isChecked(99) = true, want false")
	}
}

// --- printLoadoutActions (60%) ---

func TestPrintLoadoutActions(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	actions := []loadout.PlannedAction{
		{Type: catalog.Rules, Name: "rule-a", Action: "create-symlink", Detail: "/path"},
		{Type: catalog.Hooks, Name: "hook-b", Action: "merge-hook", Detail: "/hook"},
		{Type: catalog.MCP, Name: "mcp-c", Action: "merge-mcp", Detail: "/mcp"},
		{Type: catalog.Skills, Name: "skill-d", Action: "skip-exists", Detail: "/skill"},
		{Type: catalog.Rules, Name: "rule-e", Action: "error-conflict", Detail: "/rule", Problem: "already exists"},
		{Type: catalog.Agents, Name: "agent-f", Action: "unknown-action", Detail: "/agent"},
	}
	printLoadoutActions(actions)
	out := stdout.String()
	// Each action symbol class should appear at least once.
	for _, want := range []string{"+", "*", "=", "!", "rule-a", "hook-b", "mcp-c", "skill-d", "rule-e", "agent-f", "already exists"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

// --- printAddResults (63.4%) ---

func TestPrintAddResults_AllStatuses(t *testing.T) {
	stdout, stderr := output.SetForTest(t)

	results := []add.AddResult{
		{Name: "added-item", Type: catalog.Rules, Status: add.AddStatusAdded},
		{Name: "updated-item", Type: catalog.Rules, Status: add.AddStatusUpdated},
		{Name: "uptodate-item", Type: catalog.Rules, Status: add.AddStatusUpToDate},
		{Name: "skipped-item", Type: catalog.Rules, Status: add.AddStatusSkipped},
		{Name: "error-item", Type: catalog.Rules, Status: add.AddStatusError, Error: errors.New("disk full")},
	}

	if err := printAddResults(results, false, "Claude Code"); err != nil {
		t.Fatalf("printAddResults: %v", err)
	}

	out := stdout.String()
	for _, want := range []string{"added-item", "added", "updated-item", "updated", "uptodate-item", "up to date", "skipped-item", "use --force", "Added 1 rules from Claude Code", "1 already up to date", "1 has updates"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q\nfull stdout:\n%s", want, out)
		}
	}
	errOut := stderr.String()
	if !strings.Contains(errOut, "error-item") || !strings.Contains(errOut, "disk full") {
		t.Errorf("stderr missing error info: %q", errOut)
	}
}

func TestPrintAddResults_DryRun(t *testing.T) {
	stdout, _ := output.SetForTest(t)

	results := []add.AddResult{
		{Name: "new-item", Type: catalog.Skills, Status: add.AddStatusAdded},
		{Name: "modified-item", Type: catalog.Skills, Status: add.AddStatusUpdated},
	}

	if err := printAddResults(results, true, ""); err != nil {
		t.Fatalf("printAddResults: %v", err)
	}
	out := stdout.String()
	for _, want := range []string{"[dry-run] would add", "[dry-run] would update", "would add 1 skills", "would update 1"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q\nfull stdout:\n%s", want, out)
		}
	}
	// No "from <provider>" when providerName is empty.
	if strings.Contains(out, " from ") {
		t.Errorf("output has unwanted 'from' clause: %s", out)
	}
}

func TestPrintAddResults_QuietSuppresses(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	output.Quiet = true
	t.Cleanup(func() { output.Quiet = false })

	results := []add.AddResult{
		{Name: "x", Type: catalog.Rules, Status: add.AddStatusAdded},
	}
	if err := printAddResults(results, false, "X"); err != nil {
		t.Fatal(err)
	}
	if stdout.Len() != 0 {
		t.Errorf("expected no output in quiet mode, got %q", stdout.String())
	}
}

// --- printExecuteError (60%) ---

func TestPrintExecuteError_PlainError(t *testing.T) {
	_, stderr := output.SetForTest(t)
	printExecuteError(fmt.Errorf("boom"))
	if !strings.Contains(stderr.String(), "boom") {
		t.Errorf("stderr = %q, want 'boom'", stderr.String())
	}
}

func TestPrintExecuteError_SilentError(t *testing.T) {
	_, stderr := output.SetForTest(t)
	printExecuteError(output.SilentError(fmt.Errorf("hidden")))
	if stderr.Len() != 0 {
		t.Errorf("expected silent (no output), got %q", stderr.String())
	}
}

func TestPrintExecuteError_StructuredError(t *testing.T) {
	stdout, stderr := output.SetForTest(t)
	se := output.NewStructuredError("TEST_001", "thing failed", "try again")
	printExecuteError(se)
	// Structured errors render via PrintStructuredError — output may go to either stream.
	combined := stdout.String() + stderr.String()
	if !strings.Contains(combined, "thing failed") {
		t.Errorf("expected message in output, got stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestPrintExecuteError_JSONMode(t *testing.T) {
	_, stderr := output.SetForTest(t)
	output.JSON = true
	t.Cleanup(func() { output.JSON = false })

	printExecuteError(fmt.Errorf("kaboom"))
	out := stderr.String()
	if !strings.Contains(out, "UNKNOWN_001") || !strings.Contains(out, "kaboom") {
		t.Errorf("expected JSON envelope with UNKNOWN_001 and kaboom, got %q", out)
	}
}
