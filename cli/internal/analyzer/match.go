package analyzer

import "path/filepath"

// CandidateMatch records one detector's claim on a file path.
type CandidateMatch struct {
	Path     string
	Pattern  DetectionPattern
	Detector ContentDetector
}

// globalExcludedBasenames are filenames excluded from all detectors regardless of path.
// Prevents false positives from documentation files that happen to match content patterns.
// Exception: CLAUDE.md, GEMINI.md, AGENTS.md are NOT excluded — they are legitimate content.
var globalExcludedBasenames = map[string]bool{
	"README.md":          true,
	"CHANGELOG.md":       true,
	"LICENSE.md":         true,
	"CONTRIBUTING.md":    true,
	"CODE_OF_CONDUCT.md": true,
}

// MatchPatterns evaluates all detectors' patterns against the path index.
// Returns one CandidateMatch per (detector, path) pair where the glob matched.
// paths are relative to repoRoot (as returned by Walk), normalized to forward
// slashes via filepath.ToSlash before matching (B4: ensures patterns work
// on all platforms since filepath.Match uses the OS separator on Windows).
func MatchPatterns(paths []string, detectors []ContentDetector) []CandidateMatch {
	var matches []CandidateMatch
	for _, det := range detectors {
		for _, pat := range det.Patterns() {
			for _, p := range paths {
				if globalExcludedBasenames[filepath.Base(p)] {
					continue
				}
				normalized := filepath.ToSlash(p)
				ok, err := filepath.Match(pat.Glob, normalized)
				if err != nil {
					continue
				}
				if ok {
					matches = append(matches, CandidateMatch{
						Path:     p,
						Pattern:  pat,
						Detector: det,
					})
				}
			}
		}
	}
	return matches
}
