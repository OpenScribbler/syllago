// Package splitter splits monolithic rule files (CLAUDE.md, AGENTS.md,
// GEMINI.md, .cursorrules, .clinerules, .windsurfrules) into atomic
// SplitCandidates for library storage. Deterministic path per D3/D4;
// LLM path is a parallel producer (D9) that returns the same type.
package splitter

import (
	"bytes"
	"regexp"
	"strings"
)

// SplitCandidate is one atomic rule produced by the splitter.
// Downstream pipeline (library write, install) is indifferent to which
// heuristic (H2/H3/H4/marker/single/LLM) produced the candidate.
type SplitCandidate struct {
	Name          string // slug, suitable for library dir name
	Description   string // original heading text (pre-slugify) or "" for whole-file imports
	Body          string // candidate body bytes (canonical form applied by the caller)
	OriginalRange [2]int // [start_line, end_line_exclusive) in the source file
}

// Heuristic selects the split mode.
type Heuristic int

const (
	HeuristicH2     Heuristic = iota // default — split at every ##
	HeuristicH3                      // split at every ###
	HeuristicH4                      // split at every ####
	HeuristicMarker                  // split at a literal-string line match
	HeuristicSingle                  // no split — import as single rule
)

// Options controls splitter behavior for a single call.
type Options struct {
	Heuristic     Heuristic
	MarkerLiteral string // only used when Heuristic == HeuristicMarker
}

// SkipSplitSignal is returned alongside an empty candidate slice when the
// splitter determines the file is not a good split target per D4:
//   - fewer than 30 lines, OR
//   - fewer than 3 H2 headings
//
// The wizard surfaces this as a "import as single rule" suggestion;
// the CLI errors out unless --split=single is passed.
type SkipSplitSignal struct {
	Reason string // "too_small" | "too_few_h2"
}

// Split returns atomic SplitCandidates according to opts, or a SkipSplitSignal
// when D4's skip-split heuristic fires (fewer than 30 lines OR fewer than 3
// H2 headings). Only one of the two returns is non-nil.
func Split(source []byte, opts Options) ([]SplitCandidate, *SkipSplitSignal) {
	if opts.Heuristic == HeuristicSingle {
		return []SplitCandidate{{
			Name:          "", // caller provides slug for whole-file import
			Description:   "",
			Body:          string(source),
			OriginalRange: [2]int{0, bytes.Count(source, []byte{'\n'}) + 1},
		}}, nil
	}
	lines := bytes.Split(source, []byte{'\n'})
	if len(lines) < 30 {
		return nil, &SkipSplitSignal{Reason: "too_small"}
	}
	// Literal-marker heuristic is a separate path — no header promotion, no
	// slugify. Regions separated by exact-match lines become candidates.
	if opts.Heuristic == HeuristicMarker {
		if opts.MarkerLiteral == "" {
			return nil, nil
		}
		return splitByMarker(lines, opts.MarkerLiteral), nil
	}
	// H2-default skip-split: if H2 heuristic selected and <3 H2 headings, skip.
	// Opt-in H3/H4/Marker heuristics skip only on the <30 lines branch.
	if opts.Heuristic == HeuristicH2 {
		h2Count := 0
		for _, ln := range lines {
			if bytes.HasPrefix(ln, []byte("## ")) && !bytes.HasPrefix(ln, []byte("### ")) {
				h2Count++
			}
		}
		if h2Count < 3 {
			return nil, &SkipSplitSignal{Reason: "too_few_h2"}
		}
	}
	prefix := headingPrefix(opts.Heuristic)
	if prefix == nil {
		// Unknown / unsupported heuristic — treat as no-op.
		return nil, nil
	}
	return splitByHeadingPrefix(lines, prefix), nil
}

// headingPrefix returns the byte prefix (including trailing space) that
// identifies a heading line at the given heuristic's level. Returns nil for
// non-heading heuristics (Marker, Single, or unknown values).
func headingPrefix(h Heuristic) []byte {
	switch h {
	case HeuristicH2:
		return []byte("## ")
	case HeuristicH3:
		return []byte("### ")
	case HeuristicH4:
		return []byte("#### ")
	}
	return nil
}

type section struct {
	headingLine int
	headingText string
	bodyStart   int
	bodyEnd     int
}

// splitByHeadingPrefix walks lines looking for exact heading-level matches
// (e.g. "## " for H2), collects each section, and emits SplitCandidates with
// header promotion (heading becomes H1) and slug/description handling per D4.
func splitByHeadingPrefix(lines [][]byte, prefix []byte) []SplitCandidate {
	var sections []section
	for i, ln := range lines {
		if !isExactHeading(ln, prefix) {
			continue
		}
		if len(sections) > 0 {
			sections[len(sections)-1].bodyEnd = i
		}
		sections = append(sections, section{
			headingLine: i,
			headingText: string(bytes.TrimPrefix(ln, prefix)),
			bodyStart:   i,
		})
	}
	if len(sections) == 0 {
		return nil
	}
	sections[len(sections)-1].bodyEnd = len(lines)

	// Preamble: lines before the first heading are prepended to the first
	// candidate body (after the promoted H1) per D4.
	var preamble [][]byte
	if sections[0].headingLine > 0 {
		preamble = lines[0:sections[0].headingLine]
	}

	out := make([]SplitCandidate, 0, len(sections))
	for i, s := range sections {
		var pre [][]byte
		if i == 0 {
			pre = preamble
		}
		body := rebuildBodyWithPreamble(lines[s.bodyStart:s.bodyEnd], s.headingText, pre)
		out = append(out, SplitCandidate{
			Name:          slugify(s.headingText),
			Description:   s.headingText,
			Body:          body,
			OriginalRange: [2]int{s.headingLine, s.bodyEnd},
		})
	}
	return out
}

// isExactHeading returns true iff ln starts with prefix and the next char
// after the prefix is NOT '#' (so "## Foo" matches for H2 prefix but "### Foo"
// does not). prefix must end with a single space per markdown convention.
func isExactHeading(ln, prefix []byte) bool {
	if !bytes.HasPrefix(ln, prefix) {
		return false
	}
	// Reject deeper headings: if the char before the final space was '#' and
	// there's another '#' after the prefix... actually prefix already ends in
	// ' ', so check: if the char after the prefix is '#', this is a deeper
	// heading. Ex: prefix="## ", ln="### Foo" → HasPrefix("## ", "### Foo")?
	// No — "## " is not a prefix of "### Foo" because char[2]=='#', not ' '.
	// So HasPrefix already filters deeper headings correctly.
	return true
}

var numberedPrefixRe = regexp.MustCompile(`^\d+\.\s*`)
var nonSlugRe = regexp.MustCompile(`[^a-z0-9]+`)

// slugify lowercases, strips numbered prefixes ("1. " -> ""), and replaces
// runs of non [a-z0-9] with "-". Called on the heading text only.
func slugify(heading string) string {
	h := strings.TrimSpace(heading)
	// Strip leading "N. " or "N." numbered prefix (D4).
	if m := numberedPrefixRe.FindString(h); m != "" {
		h = h[len(m):]
	}
	h = strings.ToLower(h)
	h = nonSlugRe.ReplaceAllString(h, "-")
	h = strings.Trim(h, "-")
	return h
}

// rebuildBodyWithPreamble promotes the section heading from ## to # and
// returns the body as a single string. If preamble is non-nil, the preamble
// lines are written immediately after the promoted H1 and before the original
// body lines (D4 preamble handling). preamble SHOULD only be passed for the
// first section.
func rebuildBodyWithPreamble(sectionLines [][]byte, headingText string, preamble [][]byte) string {
	if len(sectionLines) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("# ")
	sb.WriteString(strings.TrimSpace(headingText))
	sb.WriteByte('\n')
	// Write preamble lines (if any) immediately after the promoted heading.
	for _, ln := range preamble {
		sb.Write(ln)
		sb.WriteByte('\n')
	}
	// Append body lines after the heading (excluding the heading line itself).
	for _, ln := range sectionLines[1:] {
		sb.Write(ln)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// splitByMarker walks lines and emits one SplitCandidate per region separated
// by lines whose entire content is exactly markerLiteral. No header promotion
// (there's no heading), no slugification — Name and Description are empty and
// the caller is expected to prompt the user for slugs at review time.
// The region before the first marker is emitted as a candidate if non-empty.
// Empty regions (two adjacent markers, or leading/trailing marker with no
// content) are skipped.
func splitByMarker(lines [][]byte, markerLiteral string) []SplitCandidate {
	marker := []byte(markerLiteral)
	var out []SplitCandidate
	regionStart := 0
	emit := func(start, end int) {
		if start >= end {
			return
		}
		// Trim purely-empty regions (end-of-file trailing lines, etc.).
		hasContent := false
		for _, ln := range lines[start:end] {
			if len(bytes.TrimSpace(ln)) > 0 {
				hasContent = true
				break
			}
		}
		if !hasContent {
			return
		}
		var sb strings.Builder
		for _, ln := range lines[start:end] {
			sb.Write(ln)
			sb.WriteByte('\n')
		}
		out = append(out, SplitCandidate{
			Name:          "",
			Description:   "",
			Body:          sb.String(),
			OriginalRange: [2]int{start, end},
		})
	}
	for i, ln := range lines {
		if bytes.Equal(ln, marker) {
			emit(regionStart, i)
			regionStart = i + 1
		}
	}
	emit(regionStart, len(lines))
	return out
}
