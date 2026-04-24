package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter/canonical"
	"github.com/OpenScribbler/syllago/cli/internal/discover"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/rulestore"
	"github.com/OpenScribbler/syllago/cli/internal/splitter"
)

// monolithicCandidate is one discovered monolithic rule file surfaced on the
// Discovery step when addSourceMonolithic is active (D2, D18). One row
// renders per candidate with path, line count, H2 heading count, scope, and
// an "in library" badge when the file's bytes hash matches any existing
// library rule's source.hash (D11).
type monolithicCandidate struct {
	AbsPath    string
	RelPath    string // relative to projectRoot when applicable; else absolute
	Filename   string
	Scope      string // "project" | "global"
	Lines      int
	H2Count    int
	InLibrary  bool
	Bytes      []byte
	SizeErr    string // non-empty if reading the file failed
	ProviderID string // provider slug derived from filename (e.g. "claude-code")
}

// reviewCandidate is one split candidate displayed on the Review step.
// SourceIdx points back into selectedCandidates. When SkipSplit is true
// the candidate is a whole-file import and SkipReason carries the splitter
// signal's human-readable reason.
type reviewCandidate struct {
	SourceIdx  int
	Candidate  splitter.SplitCandidate
	SkipSplit  bool
	SkipReason string
	Accept     bool
	RenameSlug string // overrides Candidate.Name when non-empty
}

// filenameToProviderSlug reverses provider.MonolithicFilenames. V1 covers
// the canonical six monolithic filenames; unknown filenames fall back to
// "" (no provider slug), which the caller treats as the "local" source.
func filenameToProviderSlug(filename string) string {
	switch filename {
	case "CLAUDE.md":
		return "claude-code"
	case "AGENTS.md":
		return "codex"
	case "GEMINI.md":
		return "gemini-cli"
	case ".cursorrules":
		return "cursor"
	case ".clinerules":
		return "cline"
	case ".windsurfrules":
		return "windsurf"
	}
	return ""
}

// discoverMonolithicCandidates builds the full list of monolithic rule
// files under projectRoot + homeDir, reads each file's bytes, computes
// line/H2 counts, and tags "in library" rows whose bytes hash matches a
// rule's source.hash (D11). Errors reading individual files populate
// SizeErr and do not abort discovery.
func discoverMonolithicCandidates(projectRoot, contentRoot string) ([]monolithicCandidate, error) {
	homeDir, _ := os.UserHomeDir()
	cands, err := discover.DiscoverMonolithicRules(projectRoot, homeDir, provider.AllMonolithicFilenames())
	if err != nil {
		return nil, err
	}
	inLibHashes := loadLibraryRuleSourceHashes(contentRoot)

	out := make([]monolithicCandidate, 0, len(cands))
	for _, c := range cands {
		mc := monolithicCandidate{
			AbsPath:    c.AbsPath,
			Filename:   c.Filename,
			Scope:      c.Scope,
			ProviderID: filenameToProviderSlug(c.Filename),
		}
		if rel, err := filepath.Rel(projectRoot, c.AbsPath); err == nil && !strings.HasPrefix(rel, "..") {
			mc.RelPath = rel
		} else {
			mc.RelPath = c.AbsPath
		}
		data, rerr := os.ReadFile(c.AbsPath)
		if rerr != nil {
			mc.SizeErr = rerr.Error()
		} else {
			mc.Bytes = data
			mc.Lines = countLines(data)
			mc.H2Count = countH2Headings(data)
			h := rulestore.HashBody(data)
			if _, ok := inLibHashes[h]; ok {
				mc.InLibrary = true
			}
		}
		out = append(out, mc)
	}
	return out, nil
}

// loadLibraryRuleSourceHashes walks contentRoot/<provider>/<slug>/ looking
// for .syllago.yaml files and returns the set of source.hash values present.
// Non-rule content and unreadable entries are silently skipped.
func loadLibraryRuleSourceHashes(contentRoot string) map[string]struct{} {
	out := make(map[string]struct{})
	if contentRoot == "" {
		return out
	}
	providerDirs, err := os.ReadDir(contentRoot)
	if err != nil {
		return out
	}
	for _, pd := range providerDirs {
		if !pd.IsDir() {
			continue
		}
		slugDirs, err := os.ReadDir(filepath.Join(contentRoot, pd.Name()))
		if err != nil {
			continue
		}
		for _, sd := range slugDirs {
			if !sd.IsDir() {
				continue
			}
			yamlPath := filepath.Join(contentRoot, pd.Name(), sd.Name(), metadata.FileName)
			if _, err := os.Stat(yamlPath); err != nil {
				continue
			}
			meta, err := metadata.LoadRuleMetadata(yamlPath)
			if err != nil || meta.Source.Hash == "" {
				continue
			}
			out[meta.Source.Hash] = struct{}{}
		}
	}
	return out
}

func countLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	n := 1
	for _, b := range data {
		if b == '\n' {
			n++
		}
	}
	// Trailing newline doesn't add a new line in display terms.
	if data[len(data)-1] == '\n' {
		n--
	}
	return n
}

func countH2Headings(data []byte) int {
	count := 0
	for _, ln := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(ln, "## ") && !strings.HasPrefix(ln, "### ") {
			count++
		}
	}
	return count
}

// heuristicFromInt converts the wizard model's int-valued chosenHeuristic
// back to the typed splitter.Heuristic. Kept as a helper so add_wizard.go
// doesn't need a direct splitter import.
func heuristicFromInt(h int) splitter.Heuristic {
	return splitter.Heuristic(h)
}

// heuristicString returns the string form of a heuristic for metadata and
// telemetry. Matches the D11/D18 vocabulary: "h2|h3|h4|marker|single".
func heuristicString(h int) string {
	switch splitter.Heuristic(h) {
	case splitter.HeuristicH2:
		return "h2"
	case splitter.HeuristicH3:
		return "h3"
	case splitter.HeuristicH4:
		return "h4"
	case splitter.HeuristicMarker:
		return "marker"
	case splitter.HeuristicSingle:
		return "single"
	}
	return ""
}

// buildReviewCandidates splits every selected source file under the chosen
// heuristic and flattens the result into a grouped-flat review list per D18.
// Candidates from a source that triggers splitter.SkipSplitSignal are
// emitted as one single-candidate row with SkipSplit=true + SkipReason set.
func buildReviewCandidates(sources []monolithicCandidate, selected []int, heuristic int, marker string) []reviewCandidate {
	var out []reviewCandidate
	opts := splitter.Options{Heuristic: heuristicFromInt(heuristic), MarkerLiteral: marker}
	for _, idx := range selected {
		if idx < 0 || idx >= len(sources) {
			continue
		}
		src := sources[idx]
		if len(src.Bytes) == 0 {
			// Unreadable file — emit a single skip-split row so the user sees it.
			out = append(out, reviewCandidate{
				SourceIdx:  idx,
				Candidate:  splitter.SplitCandidate{Name: fallbackSlugFromFilename(src.Filename), Description: "", Body: ""},
				SkipSplit:  true,
				SkipReason: "unreadable",
				Accept:     false,
			})
			continue
		}
		normalized := canonical.Normalize(src.Bytes)
		cands, skip := splitter.Split(normalized, opts)
		if skip != nil {
			out = append(out, reviewCandidate{
				SourceIdx: idx,
				Candidate: splitter.SplitCandidate{
					Name:        fallbackSlugFromFilename(src.Filename),
					Description: "",
					Body:        string(normalized),
				},
				SkipSplit:  true,
				SkipReason: skipReasonHuman(skip.Reason),
				Accept:     true,
			})
			continue
		}
		if heuristicFromInt(heuristic) == splitter.HeuristicSingle {
			// Single-heuristic path: one whole-file candidate with no split.
			if len(cands) == 1 {
				cands[0].Name = fallbackSlugFromFilename(src.Filename)
			}
		}
		for _, c := range cands {
			out = append(out, reviewCandidate{
				SourceIdx: idx,
				Candidate: c,
				SkipSplit: false,
				Accept:    true,
			})
		}
	}
	return out
}

// skipReasonHuman maps the splitter's internal reason string to the D4
// human-readable label rendered on the Review group header.
func skipReasonHuman(reason string) string {
	switch reason {
	case "too_small":
		return "file too small"
	case "too_few_h2":
		return "too few H2 headings"
	}
	return reason
}

// fallbackSlugFromFilename produces a slug for whole-file imports where the
// splitter didn't have a heading to work from. "CLAUDE.md" -> "claude".
func fallbackSlugFromFilename(filename string) string {
	name := strings.TrimPrefix(filename, ".")
	name = strings.TrimSuffix(name, ".md")
	name = strings.TrimSuffix(name, "rules")
	name = strings.ToLower(name)
	name = strings.TrimSuffix(name, "-")
	name = strings.TrimSuffix(name, "_")
	if name == "" {
		name = "rule"
	}
	return name
}

// acceptedReviewCandidates returns the subset of review candidates the user
// has kept ticked, with any rename override applied to the Name field.
func (m *addWizardModel) acceptedReviewCandidates() []reviewCandidate {
	var out []reviewCandidate
	for i, rc := range m.reviewCandidates {
		if i >= len(m.reviewAccepted) || !m.reviewAccepted[i] {
			continue
		}
		if i < len(m.reviewRenames) && m.reviewRenames[i] != "" {
			rc.Candidate.Name = m.reviewRenames[i]
			rc.RenameSlug = m.reviewRenames[i]
		}
		out = append(out, rc)
	}
	return out
}

// writeAcceptedCandidates writes each accepted candidate to the rule library
// under contentRoot/<providerSlug>/<slug>. Each call populates RuleSource
// with provider, scope, path, filename, hash, split_method, and
// split_from_section per D11/D13. On error, stops and returns the partial
// results so the TUI can surface a toast.
func (m *addWizardModel) writeAcceptedCandidates() []addExecResult {
	accepted := m.acceptedReviewCandidates()
	results := make([]addExecResult, 0, len(accepted))
	for _, rc := range accepted {
		if rc.SourceIdx < 0 || rc.SourceIdx >= len(m.discoveryCandidates) {
			continue
		}
		src := m.discoveryCandidates[rc.SourceIdx]
		providerSlug := src.ProviderID
		if providerSlug == "" {
			providerSlug = "local"
		}
		slug := rc.Candidate.Name
		if slug == "" {
			slug = fallbackSlugFromFilename(src.Filename)
		}
		meta := buildRuleMetadataForCandidate(src, rc, m.chosenHeuristic)
		// WriteRuleWithSource normalizes body + captures source bytes. The
		// splitter already returned a canonical body, so re-normalization is
		// a no-op but kept here for the post-rename/edit future path.
		err := rulestore.WriteRuleWithSource(
			m.contentRoot,
			providerSlug,
			slug,
			meta,
			[]byte(rc.Candidate.Body),
			src.Filename,
			src.Bytes,
		)
		res := addExecResult{name: slug}
		if err != nil {
			res.status = "error"
			res.err = fmt.Errorf("writing %s: %w", slug, err)
		} else {
			res.status = "added"
		}
		results = append(results, res)
	}
	return results
}

// buildRuleMetadataForCandidate constructs a RuleMetadata with a populated
// RuleSource block so WriteRuleWithSource can persist provenance per D11/D13.
// Called once per accepted review candidate at Execute time.
func buildRuleMetadataForCandidate(src monolithicCandidate, rc reviewCandidate, heuristic int) metadata.RuleMetadata {
	name := rc.Candidate.Name
	if name == "" {
		name = fallbackSlugFromFilename(src.Filename)
	}
	desc := rc.Candidate.Description
	format := filenameToFormat(src.Filename)
	return metadata.RuleMetadata{
		FormatVersion: metadata.CurrentFormatVersion,
		Name:          name,
		Description:   desc,
		Type:          "rule",
		Source: metadata.RuleSource{
			Provider:         src.ProviderID,
			Scope:            src.Scope,
			Path:             src.AbsPath,
			Format:           format,
			Filename:         src.Filename,
			Hash:             rulestore.HashBody(src.Bytes),
			SplitMethod:      heuristicString(heuristic),
			SplitFromSection: desc,
		},
	}
}

// filenameToFormat returns the "format" value written into source.format for
// a given monolithic rule file. Mirrors D11's schema.
func filenameToFormat(filename string) string {
	switch filename {
	case "CLAUDE.md":
		return "claude-code"
	case "AGENTS.md":
		return "codex"
	case "GEMINI.md":
		return "gemini-cli"
	case ".cursorrules":
		return "cursor"
	case ".clinerules":
		return "cline"
	case ".windsurfrules":
		return "windsurf"
	}
	return "markdown"
}

// visibleMonolithicRows returns the rendered rows of the Discovery step for
// test assertions. Each row is a fully-rendered line. Exported via a method
// on the wizard so tests can assert on the user-visible representation
// without going through a full View() call. Kept here because it's a
// concern specific to the monolithic discovery layout.
func (m *addWizardModel) renderMonolithicDiscoveryRows(width int) []string {
	_ = catalog.Rules // keep catalog import anchored; not otherwise referenced here
	rows := make([]string, 0, len(m.discoveryCandidates))
	for i, c := range m.discoveryCandidates {
		mark := " "
		for _, sel := range m.selectedCandidates {
			if sel == i {
				mark = "x"
				break
			}
		}
		cursor := " "
		if i == m.discoveryCandidateCurs {
			cursor = ">"
		}
		var libTag string
		if c.InLibrary {
			libTag = " ✓ in library"
		}
		row := fmt.Sprintf("%s [%s] %s  %dL  %dH2  [%s]%s", cursor, mark, c.RelPath, c.Lines, c.H2Count, c.Scope, libTag)
		if c.SizeErr != "" {
			row += "  (" + c.SizeErr + ")"
		}
		rows = append(rows, row)
	}
	return rows
}
