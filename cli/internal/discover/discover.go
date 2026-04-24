// Package discover finds monolithic rule files (CLAUDE.md, AGENTS.md,
// GEMINI.md, .cursorrules, .clinerules, .windsurfrules) under a project
// root and the user's home directory (D2).
package discover

import (
	"io/fs"
	"os"
	"path/filepath"
)

// Candidate is one discovered monolithic rule file.
type Candidate struct {
	AbsPath  string // absolute path to the file
	Scope    string // "project" | "global"
	Filename string // basename (e.g. "CLAUDE.md")
}

// DiscoverMonolithicRules walks projectRoot for any filename in filenames,
// stopping at nested .git boundaries, plus checks homeDir for the same set
// at its root. Each match becomes one Candidate. Symlinks are followed.
// homeDir may be "" to skip the global scan.
func DiscoverMonolithicRules(projectRoot, homeDir string, filenames []string) ([]Candidate, error) {
	set := make(map[string]struct{}, len(filenames))
	for _, f := range filenames {
		set[f] = struct{}{}
	}
	var out []Candidate
	if projectRoot != "" {
		if err := filepath.WalkDir(projectRoot, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // skip unreadable subtree
			}
			if d.IsDir() {
				// Stop at nested .git boundaries (but not at projectRoot/.git — same dir).
				if p != projectRoot && d.Name() == ".git" {
					return fs.SkipDir
				}
				return nil
			}
			if _, ok := set[d.Name()]; ok {
				abs, _ := filepath.Abs(p)
				out = append(out, Candidate{AbsPath: abs, Scope: "project", Filename: d.Name()})
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}
	if homeDir != "" {
		for name := range set {
			candidate := filepath.Join(homeDir, name)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				abs, _ := filepath.Abs(candidate)
				out = append(out, Candidate{AbsPath: abs, Scope: "global", Filename: name})
			}
		}
	}
	return out, nil
}
