package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installcheck"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

// TestMetaPanel_RuleShowsPerTargetStatus verifies the D16 per-target
// breakdown in the metapanel for a rule item: one line per
// InstalledRuleAppend record with the looked-up PerTargetState rendered as
// a human-readable string ("Clean", "Modified · edited",
// "Modified · missing", "Modified · unreadable").
func TestMetaPanel_RuleShowsPerTargetStatus(t *testing.T) {
	t.Parallel()

	item := catalog.ContentItem{
		Name: "rule-with-targets",
		Type: catalog.Rules,
		Meta: &metadata.Meta{ID: "lib-three-targets"},
	}

	records := []ruleTargetStatus{
		{TargetFile: "/tmp/proj/A.md", State: installcheck.PerTargetState{
			State: installcheck.StateClean, Reason: installcheck.ReasonNone,
		}},
		{TargetFile: "/tmp/proj/B.md", State: installcheck.PerTargetState{
			State: installcheck.StateModified, Reason: installcheck.ReasonEdited,
		}},
		{TargetFile: "/tmp/proj/C.md", State: installcheck.PerTargetState{
			State: installcheck.StateModified, Reason: installcheck.ReasonMissing,
		}},
	}

	data := metaPanelData{
		installed:   "--",
		ruleRecords: records,
	}
	out := ansi.Strip(renderMetaPanel(&item, data, 160))

	if !strings.Contains(out, "Installed at:") {
		t.Errorf("expected \"Installed at:\" section header, got:\n%s", out)
	}
	if !strings.Contains(out, "A.md") || !strings.Contains(out, "Clean") {
		t.Errorf("expected A.md to render with status \"Clean\", got:\n%s", out)
	}
	if !strings.Contains(out, "B.md") || !strings.Contains(out, "Modified · edited") {
		t.Errorf("expected B.md to render with status \"Modified · edited\", got:\n%s", out)
	}
	if !strings.Contains(out, "C.md") || !strings.Contains(out, "Modified · missing") {
		t.Errorf("expected C.md to render with status \"Modified · missing\", got:\n%s", out)
	}
}

// TestMetaPanel_RuleUnreadableStatusString verifies the "unreadable"
// divergence renders as "Modified · unreadable" — separated from
// TestMetaPanel_RuleShowsPerTargetStatus to keep that table focused on the
// 3 typical reasons (clean, edited, missing).
func TestMetaPanel_RuleUnreadableStatusString(t *testing.T) {
	t.Parallel()

	item := catalog.ContentItem{
		Name: "unreadable-rule",
		Type: catalog.Rules,
		Meta: &metadata.Meta{ID: "lib-unreadable"},
	}
	records := []ruleTargetStatus{
		{TargetFile: "/tmp/proj/D.md", State: installcheck.PerTargetState{
			State: installcheck.StateModified, Reason: installcheck.ReasonUnreadable,
		}},
	}
	data := metaPanelData{installed: "--", ruleRecords: records}
	out := ansi.Strip(renderMetaPanel(&item, data, 160))
	if !strings.Contains(out, "Modified · unreadable") {
		t.Errorf("expected \"Modified · unreadable\", got:\n%s", out)
	}
}
