package lifecycle

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/rulestore"
)

// placer writes one item into one target, reports whether a target holds
// it, and removes it again.
// installerPlacer is the real adapter; lifecycle tests substitute one that
// fails a chosen target.
type placer interface {
	place(req InstallRequest, t Target) (installer.Placement, error)
	unplace(req UninstallRequest, t Target) (installer.Placement, error)
	// present reports whether t holds item at the provider default. An
	// error means the check could not tell.
	present(item catalog.ContentItem, projectRoot string, t Target) (bool, error)
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

func (installerPlacer) present(item catalog.ContentItem, projectRoot string, t Target) (bool, error) {
	status, err := installer.StatusOf(item, t.Provider, projectRoot)
	return status == installer.StatusInstalled, err
}

func (installerPlacer) unplace(req UninstallRequest, t Target) (installer.Placement, error) {
	if req.Method == installer.MethodAppend {
		return unplaceRuleAppend(req, t)
	}
	return installer.UninstallFrom(req.Item, t.Provider, req.ProjectRoot, t.BaseDir, t.Resolver)
}

// unplaceRuleAppend removes a library rule from the monolithic rule file
// that installed.json records for the target's provider. A record matches by
// the rule's name as well as its ID, because re-importing a rule gives it a
// new ID while the records keep the old one.
func unplaceRuleAppend(req UninstallRequest, t Target) (installer.Placement, error) {
	loaded, err := rulestore.LoadRule(req.Item.Path)
	if err != nil {
		return installer.Placement{}, fmt.Errorf("loading library rule: %w", err)
	}
	inst, err := installer.LoadInstalled(req.ProjectRoot)
	if err != nil {
		return installer.Placement{}, err
	}
	for _, r := range inst.RuleAppends {
		if r.Provider != t.Provider.Slug || (t.File != "" && r.TargetFile != t.File) {
			continue
		}
		if r.LibraryID != loaded.Meta.ID && r.Name != req.Item.Name {
			continue
		}
		if err := installer.UninstallRuleAppend(req.ProjectRoot, r.LibraryID, r.TargetFile, map[string]*rulestore.Loaded{r.LibraryID: loaded}); err != nil {
			return installer.Placement{}, err
		}
		return installer.Placement{Mechanism: installer.MechanismRuleAppend, Path: r.TargetFile}, nil
	}
	return installer.Placement{}, fmt.Errorf("rule %s is not appended for %s", req.Item.Name, t.Provider.Slug)
}
