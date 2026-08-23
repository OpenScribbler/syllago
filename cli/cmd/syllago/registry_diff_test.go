package main

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/regdiff"
)

func TestPrintRegistryDiff_NoOutputCases(t *testing.T) {
	cases := []struct {
		name string
		diff *regdiff.Diff
	}{
		{name: "nil", diff: nil},
		{
			name: "empty old ref",
			diff: &regdiff.Diff{
				OldRef: " ",
				Changes: []regdiff.ItemChange{
					{Type: "skills", Name: "new", Kind: regdiff.KindAdded},
				},
			},
		},
		{
			name: "up to date",
			diff: &regdiff.Diff{
				OldRef:   "old",
				UpToDate: true,
				Changes: []regdiff.ItemChange{
					{Type: "skills", Name: "new", Kind: regdiff.KindAdded},
				},
			},
		},
		{name: "empty changes", diff: &regdiff.Diff{OldRef: "old"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			output.SetForTest(t)
			var buf bytes.Buffer
			printRegistryDiff(&buf, tc.diff)
			if got := buf.String(); got != "" {
				t.Fatalf("printRegistryDiff() = %q; want empty", got)
			}
		})
	}
}

func TestPrintRegistryDiff_MixedChanges(t *testing.T) {
	output.SetForTest(t)
	d := &regdiff.Diff{
		OldRef: "old",
		Changes: []regdiff.ItemChange{
			{Type: "skills", Name: "new-thing", Kind: regdiff.KindAdded},
			{Type: "rules", Name: "updated-rule", Kind: regdiff.KindModified},
			{Type: "agents", Name: "old-agent", Kind: regdiff.KindRemoved},
		},
	}

	var buf bytes.Buffer
	printRegistryDiff(&buf, d)
	want := "Changes since last sync:\n" +
		"  + skills/new-thing\n" +
		"  ~ rules/updated-rule\n" +
		"  - agents/old-agent\n"
	if got := buf.String(); got != want {
		t.Fatalf("printRegistryDiff() = %q; want %q", got, want)
	}
}

func TestPrintRegistryDiffLines_RendersBodyWithoutHeaderOrGuards(t *testing.T) {
	output.SetForTest(t)
	d := &regdiff.Diff{
		Changes: []regdiff.ItemChange{
			{Type: "skills", Name: "new-thing.md", Kind: regdiff.KindAdded},
			{Type: "rules", Name: "updated-rule", Kind: regdiff.KindModified},
			{Type: "agents", Name: "old-agent", Kind: regdiff.KindRemoved},
		},
		OtherPaths: []string{"README.md"},
	}

	var buf bytes.Buffer
	printRegistryDiffLines(&buf, d)
	want := "  + skills/new-thing\n" +
		"  ~ rules/updated-rule\n" +
		"  - agents/old-agent\n" +
		"  (plus 1 other changed files)\n"
	if got := buf.String(); got != want {
		t.Fatalf("printRegistryDiffLines() = %q; want %q", got, want)
	}
}

func TestPrintRegistryDiffLines_RendersLogLinesUnderChangedItem(t *testing.T) {
	output.SetForTest(t)
	d := &regdiff.Diff{
		Changes: []regdiff.ItemChange{
			{
				Type:     "skills",
				Name:     "foo",
				Kind:     regdiff.KindModified,
				LogLines: []string{"fix foo prompt wording", "add usage examples"},
			},
			{Type: "skills", Name: "bar", Kind: regdiff.KindAdded},
		},
	}

	var buf bytes.Buffer
	printRegistryDiffLines(&buf, d)
	want := "  ~ skills/foo\n" +
		"      · fix foo prompt wording\n" +
		"      · add usage examples\n" +
		"  + skills/bar\n"
	if got := buf.String(); got != want {
		t.Fatalf("printRegistryDiffLines() = %q; want %q", got, want)
	}
}

func TestPrintRegistryDiff_TrimsKnownExtensionsForDisplay(t *testing.T) {
	output.SetForTest(t)
	d := &regdiff.Diff{
		OldRef: "old",
		Changes: []regdiff.ItemChange{
			{Type: "rules", Name: "gamma.md", Kind: regdiff.KindModified},
			{Type: "rules", Name: "delta.mdc", Kind: regdiff.KindModified},
			{Type: "rules", Name: "docs.markdown", Kind: regdiff.KindModified},
			{Type: "rules", Name: "policy.yaml", Kind: regdiff.KindModified},
			{Type: "rules", Name: "profile.yml", Kind: regdiff.KindModified},
			{Type: "rules", Name: "settings.json", Kind: regdiff.KindModified},
			{Type: "rules", Name: "bundle.toml", Kind: regdiff.KindModified},
			{Type: "rules", Name: "v1.2-rules", Kind: regdiff.KindModified},
		},
	}

	var buf bytes.Buffer
	printRegistryDiff(&buf, d)
	want := "Changes since last sync:\n" +
		"  ~ rules/gamma\n" +
		"  ~ rules/delta\n" +
		"  ~ rules/docs\n" +
		"  ~ rules/policy\n" +
		"  ~ rules/profile\n" +
		"  ~ rules/settings\n" +
		"  ~ rules/bundle\n" +
		"  ~ rules/v1.2-rules\n"
	if got := buf.String(); got != want {
		t.Fatalf("printRegistryDiff() = %q; want %q", got, want)
	}
	if d.Changes[0].Name != "gamma.md" {
		t.Fatalf("printRegistryDiff mutated Diff name to %q", d.Changes[0].Name)
	}
}

func TestPrintRegistryDiff_CapsChangeLines(t *testing.T) {
	output.SetForTest(t)
	d := &regdiff.Diff{OldRef: "old"}
	for i := 1; i <= 23; i++ {
		d.Changes = append(d.Changes, regdiff.ItemChange{
			Type: "skills",
			Name: fmt.Sprintf("item-%02d", i),
			Kind: regdiff.KindAdded,
		})
	}

	var buf bytes.Buffer
	printRegistryDiff(&buf, d)

	want := "Changes since last sync:\n"
	for i := 1; i <= 20; i++ {
		want += fmt.Sprintf("  + skills/item-%02d\n", i)
	}
	want += "  … and 3 more\n"
	if got := buf.String(); got != want {
		t.Fatalf("printRegistryDiff() = %q; want %q", got, want)
	}
}

func TestPrintRegistryDiff_OtherPathsSummary(t *testing.T) {
	output.SetForTest(t)
	d := &regdiff.Diff{
		OldRef: "old",
		Changes: []regdiff.ItemChange{
			{Type: "skills", Name: "alpha", Kind: regdiff.KindModified},
		},
		OtherPaths: []string{"README.md", "registry.yaml"},
	}

	var buf bytes.Buffer
	printRegistryDiff(&buf, d)
	want := "Changes since last sync:\n" +
		"  ~ skills/alpha\n" +
		"  (plus 2 other changed files)\n"
	if got := buf.String(); got != want {
		t.Fatalf("printRegistryDiff() = %q; want %q", got, want)
	}
}

func TestPrintRegistryDiff_QuietAndJSONSuppressOutput(t *testing.T) {
	d := &regdiff.Diff{
		OldRef: "old",
		Changes: []regdiff.ItemChange{
			{Type: "skills", Name: "alpha", Kind: regdiff.KindModified},
		},
	}

	t.Run("quiet", func(t *testing.T) {
		stdout, _ := output.SetForTest(t)
		output.Quiet = true
		printRegistryDiff(stdout, d)
		if got := stdout.String(); got != "" {
			t.Fatalf("quiet output = %q; want empty", got)
		}
	})

	t.Run("json", func(t *testing.T) {
		stdout, _ := output.SetForTest(t)
		output.JSON = true
		printRegistryDiff(stdout, d)
		if got := stdout.String(); got != "" {
			t.Fatalf("json output = %q; want empty", got)
		}
	})
}
