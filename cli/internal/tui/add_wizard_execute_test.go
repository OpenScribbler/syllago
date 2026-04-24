package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/splitter"
)

// TestAddWizard_ExecuteWritesRules covers Task 4.7: writeAcceptedCandidates
// must persist each accepted review candidate into contentRoot/<provider>/<slug>
// with rule.md, .syllago.yaml (metadata), .history/<hash>.md, and .source/<filename>.
// The metadata must carry the source provenance fields (provider, scope, path,
// format, filename, hash, split_method, split_from_section) per D11/D13.
func TestAddWizard_ExecuteWritesRules(t *testing.T) {
	t.Parallel()
	contentRoot := t.TempDir()

	// Source bytes must be big enough to split; keep it stable across calls.
	var sb strings.Builder
	sb.WriteString("# Title\n\n")
	for i := 1; i <= 3; i++ {
		sb.WriteString("## Section ")
		sb.WriteByte('A' + byte(i-1))
		sb.WriteString("\n\nbody\n")
	}
	sourceBytes := []byte(sb.String())

	m := openAddWizard(nil, nil, nil, "/tmp/proj", contentRoot, "")
	m.source = addSourceMonolithic
	m.shell = newWizardShell("Add", m.buildShellLabels())
	m.discoveryCandidates = []monolithicCandidate{
		{
			RelPath:    "CLAUDE.md",
			Filename:   "CLAUDE.md",
			AbsPath:    "/tmp/proj/CLAUDE.md",
			Scope:      "project",
			ProviderID: "claude-code",
			Bytes:      sourceBytes,
		},
	}
	m.selectedCandidates = []int{0}
	m.chosenHeuristic = int(splitter.HeuristicH2)
	// Seed three accepted review candidates manually to keep the test
	// decoupled from splitter thresholds.
	m.reviewCandidates = []reviewCandidate{
		{SourceIdx: 0, Candidate: splitter.SplitCandidate{Name: "section-a", Description: "Section A", Body: "# Section A\n\nbody a\n"}, Accept: true},
		{SourceIdx: 0, Candidate: splitter.SplitCandidate{Name: "section-b", Description: "Section B", Body: "# Section B\n\nbody b\n"}, Accept: true},
		{SourceIdx: 0, Candidate: splitter.SplitCandidate{Name: "section-c", Description: "Section C", Body: "# Section C\n\nbody c\n"}, Accept: true},
	}
	m.reviewAccepted = []bool{true, true, true}
	m.reviewRenames = make([]string, 3)

	results := m.writeAcceptedCandidates()
	if len(results) != 3 {
		t.Fatalf("expected 3 exec results, got %d (%+v)", len(results), results)
	}
	for _, r := range results {
		if r.status != "added" {
			t.Errorf("result for %s: got status=%q err=%v", r.name, r.status, r.err)
		}
	}

	for _, slug := range []string{"section-a", "section-b", "section-c"} {
		ruleDir := filepath.Join(contentRoot, "claude-code", slug)

		// rule.md must exist with candidate body.
		ruleMD := filepath.Join(ruleDir, "rule.md")
		if _, err := os.Stat(ruleMD); err != nil {
			t.Errorf("expected rule.md at %s: %v", ruleMD, err)
		}

		// .syllago.yaml must exist and carry the RuleSource provenance.
		metaPath := filepath.Join(ruleDir, metadata.FileName)
		data, err := os.ReadFile(metaPath)
		if err != nil {
			t.Fatalf("read metadata %s: %v", metaPath, err)
		}
		var meta metadata.RuleMetadata
		if err := yaml.Unmarshal(data, &meta); err != nil {
			t.Fatalf("parse metadata %s: %v", metaPath, err)
		}
		if meta.Source.Provider != "claude-code" {
			t.Errorf("%s source.provider: got %q want claude-code", slug, meta.Source.Provider)
		}
		if meta.Source.Filename != "CLAUDE.md" {
			t.Errorf("%s source.filename: got %q want CLAUDE.md", slug, meta.Source.Filename)
		}
		if meta.Source.Format != "claude-code" {
			t.Errorf("%s source.format: got %q want claude-code", slug, meta.Source.Format)
		}
		if meta.Source.Scope != "project" {
			t.Errorf("%s source.scope: got %q want project", slug, meta.Source.Scope)
		}
		if meta.Source.SplitMethod != "h2" {
			t.Errorf("%s source.split_method: got %q want h2", slug, meta.Source.SplitMethod)
		}
		if meta.Source.Hash == "" || !strings.HasPrefix(meta.Source.Hash, "sha256:") {
			t.Errorf("%s source.hash: got %q, want sha256:<hex>", slug, meta.Source.Hash)
		}

		// .history/<algo>-<hex>.md must exist.
		historyDir := filepath.Join(ruleDir, ".history")
		entries, err := os.ReadDir(historyDir)
		if err != nil {
			t.Fatalf("read history dir %s: %v", historyDir, err)
		}
		if len(entries) == 0 {
			t.Errorf("%s: expected at least one .history entry, got 0", slug)
		}

		// .source/<filename> must exist with the original monolithic bytes.
		srcPath := filepath.Join(ruleDir, ".source", "CLAUDE.md")
		got, err := os.ReadFile(srcPath)
		if err != nil {
			t.Fatalf("read source copy %s: %v", srcPath, err)
		}
		if string(got) != string(sourceBytes) {
			t.Errorf("%s .source/CLAUDE.md mismatch:\ngot: %q\nwant: %q", slug, got, sourceBytes)
		}
	}
}
