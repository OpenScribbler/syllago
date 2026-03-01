package loadout

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// PlannedAction describes one action the loadout apply would take.
type PlannedAction struct {
	Type    catalog.ContentType
	Name    string
	Action  string // "create-symlink", "merge-hook", "merge-mcp", "skip-exists", "error-conflict"
	Detail  string // human-readable path or description
	Problem string // non-empty if Action == "error-conflict"
}

// Preview computes all actions without modifying any files.
// repoRoot is used to check installed.json for existing merge entries.
// homeDir is used as the base for provider install directories.
//
// How it works:
//   - For symlink types (Rules, Skills, Agents, Commands): computes the target path
//     via InstallDir, then checks if the target already exists with os.Lstat.
//   - For merge types (Hooks, MCP): checks installed.json for an existing entry.
//   - Conflicts are encoded in PlannedAction.Action, NOT returned as errors.
//     This lets callers decide whether to abort or show a warning.
func Preview(refs []ResolvedRef, prov provider.Provider, repoRoot string, homeDir string) ([]PlannedAction, error) {
	// Load installed.json once for merge-type checks
	inst, err := installer.LoadInstalled(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("loading installed.json: %w", err)
	}

	var actions []PlannedAction
	for _, ref := range refs {
		action, err := previewOne(ref, prov, homeDir, inst)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
	return actions, nil
}

func previewOne(ref ResolvedRef, prov provider.Provider, homeDir string, inst *installer.Installed) (PlannedAction, error) {
	switch ref.Type {
	case catalog.Hooks:
		return previewHook(ref, inst), nil
	case catalog.MCP:
		return previewMCP(ref, inst), nil
	default:
		return previewSymlink(ref, prov, homeDir)
	}
}

// previewSymlink checks whether a symlink target path already exists.
func previewSymlink(ref ResolvedRef, prov provider.Provider, homeDir string) (PlannedAction, error) {
	installDir := prov.InstallDir(homeDir, ref.Type)
	if installDir == "" || installDir == provider.JSONMergeSentinel || installDir == provider.ProjectScopeSentinel {
		return PlannedAction{
			Type:    ref.Type,
			Name:    ref.Name,
			Action:  "error-conflict",
			Problem: fmt.Sprintf("%s does not support filesystem install for %s", prov.Name, ref.Type.Label()),
		}, nil
	}

	targetPath := resolveSymlinkTarget(installDir, ref)

	info, err := os.Lstat(targetPath)
	if os.IsNotExist(err) {
		return PlannedAction{
			Type:   ref.Type,
			Name:   ref.Name,
			Action: "create-symlink",
			Detail: targetPath,
		}, nil
	}
	if err != nil {
		return PlannedAction{}, fmt.Errorf("stat %s: %w", targetPath, err)
	}

	// Target exists -- check if it's a symlink to the same source
	if info.Mode()&os.ModeSymlink != 0 {
		existing, readErr := os.Readlink(targetPath)
		if readErr == nil {
			// Resolve relative symlinks
			if !filepath.IsAbs(existing) {
				existing = filepath.Join(filepath.Dir(targetPath), existing)
			}
			existing = filepath.Clean(existing)
			source := filepath.Clean(symlinkSource(ref))
			if existing == source {
				return PlannedAction{
					Type:   ref.Type,
					Name:   ref.Name,
					Action: "skip-exists",
					Detail: targetPath,
				}, nil
			}
		}
		return PlannedAction{
			Type:    ref.Type,
			Name:    ref.Name,
			Action:  "error-conflict",
			Detail:  targetPath,
			Problem: "symlink exists pointing to different target",
		}, nil
	}

	// Regular file or directory exists at target
	return PlannedAction{
		Type:    ref.Type,
		Name:    ref.Name,
		Action:  "error-conflict",
		Detail:  targetPath,
		Problem: "file already exists at target path",
	}, nil
}

// previewHook checks installed.json for an existing hook entry.
func previewHook(ref ResolvedRef, inst *installer.Installed) PlannedAction {
	// Check if any hook with this name is already installed (any event)
	for _, h := range inst.Hooks {
		if h.Name == ref.Name {
			return PlannedAction{
				Type:   ref.Type,
				Name:   ref.Name,
				Action: "skip-exists",
				Detail: fmt.Sprintf("hook %s already installed for %s event", ref.Name, h.Event),
			}
		}
	}
	return PlannedAction{
		Type:   ref.Type,
		Name:   ref.Name,
		Action: "merge-hook",
		Detail: fmt.Sprintf("merge hook %s into settings.json", ref.Name),
	}
}

// previewMCP checks installed.json for an existing MCP entry.
func previewMCP(ref ResolvedRef, inst *installer.Installed) PlannedAction {
	if inst.FindMCP(ref.Name) >= 0 {
		return PlannedAction{
			Type:   ref.Type,
			Name:   ref.Name,
			Action: "skip-exists",
			Detail: fmt.Sprintf("MCP server %s already installed", ref.Name),
		}
	}
	return PlannedAction{
		Type:   ref.Type,
		Name:   ref.Name,
		Action: "merge-mcp",
		Detail: fmt.Sprintf("merge MCP server %s into config", ref.Name),
	}
}

// resolveSymlinkTarget computes the target path for a symlink-based install.
func resolveSymlinkTarget(installDir string, ref ResolvedRef) string {
	if ref.Type == catalog.Agents {
		return filepath.Join(installDir, ref.Name+".md")
	}
	if ref.Type.IsUniversal() {
		return filepath.Join(installDir, ref.Name)
	}
	// Provider-specific: use base of item path
	return filepath.Join(installDir, filepath.Base(ref.Item.Path))
}

// symlinkSource returns the source path for a symlink.
// For agents, it's the AGENT.md file inside the item directory.
func symlinkSource(ref ResolvedRef) string {
	if ref.Type == catalog.Agents {
		return filepath.Join(ref.Item.Path, "AGENT.md")
	}
	return ref.Item.Path
}
