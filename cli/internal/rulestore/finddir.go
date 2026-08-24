package rulestore

import (
	"io/fs"
	"os"
	"path/filepath"
)

// FindRuleDir locates <rulesRoot>/*/<name>/ by iterating source-provider
// subdirectories. Returns fs.ErrNotExist if no match is found. First-match
// wins when the same rule name exists under multiple source providers.
func FindRuleDir(rulesRoot, name string) (string, error) {
	entries, err := os.ReadDir(rulesRoot)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		candidate := filepath.Join(rulesRoot, e.Name(), name)
		if info, serr := os.Stat(candidate); serr == nil && info.IsDir() {
			return candidate, nil
		}
	}
	return "", fs.ErrNotExist
}
