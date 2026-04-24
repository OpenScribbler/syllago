// Package splitter splits monolithic rule files (CLAUDE.md, AGENTS.md,
// GEMINI.md, .cursorrules, .clinerules, .windsurfrules) into atomic
// SplitCandidates for library storage. Deterministic path per D3/D4;
// LLM path is a parallel producer (D9) that returns the same type.
package splitter

import "bytes"

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
	// Real split logic lives in follow-up tasks; stub returns empty success.
	return nil, nil
}
