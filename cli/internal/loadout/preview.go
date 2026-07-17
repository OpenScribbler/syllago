package loadout

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// PlannedAction describes one action the loadout apply would take.
type PlannedAction struct {
	Type    catalog.ContentType
	Name    string
	Action  string // "create-symlink", "merge-hook", "merge-mcp", "skip-exists", "skip-unsupported", "error-conflict"
	Detail  string // human-readable path or description
	Problem string // non-empty if Action == "error-conflict" or "skip-unsupported"
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
func Preview(refs []ResolvedRef, prov provider.Provider, repoRoot string, homeDir string, resolver *config.PathResolver) ([]PlannedAction, error) {
	// Load installed.json once for merge-type checks
	inst, err := installer.LoadInstalled(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("loading installed.json: %w", err)
	}

	var actions []PlannedAction
	for _, ref := range refs {
		action, err := previewOne(ref, prov, homeDir, inst, resolver)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
	return actions, nil
}

func previewOne(ref ResolvedRef, prov provider.Provider, homeDir string, inst *installer.Installed, resolver *config.PathResolver) (PlannedAction, error) {
	switch ref.Type {
	case catalog.Hooks:
		return previewHook(ref, prov, inst), nil
	case catalog.MCP:
		return previewMCP(ref, inst), nil
	default:
		return previewSymlink(ref, prov, homeDir, resolver)
	}
}

// previewSymlink checks whether a symlink target path already exists.
func previewSymlink(ref ResolvedRef, prov provider.Provider, homeDir string, resolver *config.PathResolver) (PlannedAction, error) {
	var installDir string
	if resolver != nil {
		installDir = resolver.InstallDir(prov, ref.Type, homeDir)
	} else {
		var defaultResolver *config.PathResolver
		installDir = defaultResolver.InstallDir(prov, ref.Type, homeDir)
	}
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

// previewHook checks installed.json for an existing hook entry and whether
// the target provider can read the hook's event at all.
func previewHook(ref ResolvedRef, prov provider.Provider, inst *installer.Installed) PlannedAction {
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

	// A hook whose event is a real event the provider simply has no settings
	// key for would merge as dead config the provider never reads
	// (syllago-xqlc1). Plan it as skip-unsupported so Apply can gate on it and
	// --skip-unsupported can pass it over. The IsValidHookEvent guard is
	// deliberate: an unknown/malformed event is NOT skippable — it stays a
	// merge-hook action so applyHook raises the hard injection-guard error,
	// which fires even under --skip-unsupported. A missing or unparseable hook
	// file likewise stays merge-hook so applyHook reports the real error.
	if event, ok := hookEvent(ref.Item.Path); ok &&
		converter.IsValidHookEvent(event) &&
		!converter.ProviderSupportsHookEvent(event, prov.Slug) {
		return PlannedAction{
			Type:    ref.Type,
			Name:    ref.Name,
			Action:  "skip-unsupported",
			Detail:  fmt.Sprintf("hook %s not applied", ref.Name),
			Problem: fmt.Sprintf("%s does not support hook event %q", prov.Name, event),
		}
	}

	return PlannedAction{
		Type:   ref.Type,
		Name:   ref.Name,
		Action: "merge-hook",
		Detail: fmt.Sprintf("merge hook %s into settings.json", ref.Name),
	}
}

// hookEvent reads the event name from a hook item's hook.json. Returns
// ok=false when the file is missing or malformed.
func hookEvent(itemDir string) (string, bool) {
	hookFile := findHookFile(itemDir)
	if hookFile == "" {
		return "", false
	}
	data, err := os.ReadFile(hookFile)
	if err != nil {
		return "", false
	}
	manifest, err := converter.ParseManifest(data)
	if err != nil || len(manifest.Hooks) != 1 {
		return "", false
	}
	return manifest.Hooks[0].Event, true
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
