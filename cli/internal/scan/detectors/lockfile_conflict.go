package detectors

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

// LockFileConflict detects when a project has multiple lock files for the same
// ecosystem (e.g. both package-lock.json and yarn.lock). This signals ambiguity
// about which package manager is authoritative and can cause subtle dependency
// drift between developers.
type LockFileConflict struct{}

func (d LockFileConflict) Name() string { return "lock-file-conflict" }

// conflictPair defines two lock files that shouldn't coexist.
type conflictPair struct {
	a, b    string
	manager string // ecosystem label for the message
}

var lockConflicts = []conflictPair{
	{"package-lock.json", "yarn.lock", "npm/Yarn"},
	{"package-lock.json", "pnpm-lock.yaml", "npm/pnpm"},
	{"yarn.lock", "pnpm-lock.yaml", "Yarn/pnpm"},
	{"Pipfile.lock", "poetry.lock", "Pipenv/Poetry"},
	{"Pipfile.lock", "uv.lock", "Pipenv/uv"},
}

func (d LockFileConflict) Detect(root string) ([]model.Section, error) {
	var found []string

	for _, pair := range lockConflicts {
		aExists := fileExists(filepath.Join(root, pair.a))
		bExists := fileExists(filepath.Join(root, pair.b))
		if aExists && bExists {
			found = append(found, fmt.Sprintf("Both %s and %s exist (%s)", pair.a, pair.b, pair.manager))
		}
	}

	if len(found) == 0 {
		return nil, nil
	}

	return []model.Section{model.TextSection{
		Category: model.CatSurprise,
		Origin:   model.OriginAuto,
		Title:    "Conflicting Lock Files",
		Body:     strings.Join(found, "; ") + ". Only one package manager lock file should be committed.",
		Source:   d.Name(),
	}}, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
