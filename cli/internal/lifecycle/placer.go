package lifecycle

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/rulestore"
)

// placer writes one item into one target. installerPlacer is the real
// adapter; lifecycle tests substitute one that fails a chosen target.
type placer interface {
	place(req InstallRequest, t Target) (installer.Placement, error)
}

type installerPlacer struct{}

func (installerPlacer) place(req InstallRequest, t Target) (installer.Placement, error) {
	if req.Method == installer.MethodAppend {
		return placeRuleAppend(req, t)
	}
	if t.Resolver != nil {
		return installer.InstallWithResolver(req.Item, t.Provider, req.ProjectRoot, req.Method, t.Resolver, req.Scan)
	}
	return installer.Install(req.Item, t.Provider, req.ProjectRoot, req.Method, t.BaseDir, req.Scan)
}

// placeRuleAppend appends a library rule to the provider's monolithic rule
// file (CLAUDE.md, AGENTS.md, ...) in the project root.
func placeRuleAppend(req InstallRequest, t Target) (installer.Placement, error) {
	names := provider.MonolithicFilenames(t.Provider.Slug)
	if len(names) == 0 {
		return installer.Placement{}, fmt.Errorf("provider %s does not have a monolithic rule filename", t.Provider.Slug)
	}
	loaded, err := rulestore.LoadRule(req.Item.Path)
	if err != nil {
		return installer.Placement{}, fmt.Errorf("loading library rule: %w", err)
	}
	home, _ := os.UserHomeDir()
	target := filepath.Join(req.ProjectRoot, names[0])
	if err := installer.InstallRuleAppend(req.ProjectRoot, home, t.Provider.Slug, target, req.Source, loaded); err != nil {
		return installer.Placement{}, err
	}
	return installer.Placement{Mechanism: installer.MechanismRuleAppend, Path: target}, nil
}
