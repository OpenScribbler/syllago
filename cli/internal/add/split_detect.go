package add

import (
	"os"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/converter/canonical"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/splitter"
)

// DetectSplittable reports whether path names a recognized monolithic rule
// file (CLAUDE.md, AGENTS.md, GEMINI.md, .cursorrules, .clinerules,
// .windsurfrules) whose content passes the H2 splitter pre-check — i.e. the
// file is at least 30 lines and contains at least 3 H2 headings. sectionCount
// is the number of rule candidates the default H2 heuristic would produce.
//
// Returns (false, 0, nil) for non-monolithic filenames. Returns (false, 0,
// error) for read failures. Returns (false, 0, nil) when a monolithic file
// exists but fails the skip-split gate (too small or too few H2 headings) —
// callers that want the skip reason should call splitter.Split directly.
func DetectSplittable(path string) (bool, int, error) {
	if provider.SlugForMonolithicFilename(filepath.Base(path)) == "" {
		return false, 0, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, 0, err
	}
	canon := canonical.Normalize(raw)
	cands, skip := splitter.Split(canon, splitter.Options{Heuristic: splitter.HeuristicH2})
	if skip != nil || len(cands) == 0 {
		return false, 0, nil
	}
	return true, len(cands), nil
}
