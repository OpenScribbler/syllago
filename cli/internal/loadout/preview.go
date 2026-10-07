package loadout

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	Problem string // non-empty if Action == "error-conflict" or "skip-unsupported", or for a "merge-hook" with high-severity scanner findings
}

// Preview computes all actions without modifying any files.
// repoRoot is used to check installed.json for existing merge entries.
// homeDir is used as the base for provider install directories.
//
// How it works:
//   - For symlink types (Rules, Skills, Agents, Commands): computes the target path
//     via InstallDir, then checks if the target already exists with os.Lstat.
//   - For merge types (Hooks, MCP): checks installed.json for an existing entry,
//     and for MCP, whether the merge apply runs would refuse the item.
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
		action, err := previewOne(ref, prov, repoRoot, homeDir, inst, resolver)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
	return actions, nil
}

func previewOne(ref ResolvedRef, prov provider.Provider, repoRoot, homeDir string, inst *installer.Installed, resolver *config.PathResolver) (PlannedAction, error) {
	switch ref.Type {
	case catalog.Hooks:
		return previewHook(ref, prov, repoRoot, homeDir, inst, resolver), nil
	case catalog.MCP:
		return previewMCP(ref, prov, repoRoot, inst), nil
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
func previewHook(ref ResolvedRef, prov provider.Provider, repoRoot, homeDir string, inst *installer.Installed, resolver *config.PathResolver) PlannedAction {
	// Check if a hook with this name is already installed on this provider
	// (any event). An entry with no provider predates provider tracking and
	// counts for every provider.
	for _, h := range inst.Hooks {
		if h.Name == ref.Name && (h.Provider == prov.Slug || h.Provider == "") {
			return PlannedAction{
				Type:   ref.Type,
				Name:   ref.Name,
				Action: "skip-exists",
				Detail: fmt.Sprintf("hook %s already installed for %s event", ref.Name, h.Event),
			}
		}
	}
	// A hook the merge would refuse, such as an unreadable manifest, an
	// unknown event, a script outside its item, or a provider config the
	// merge cannot read, fails the apply before anything changes, under
	// --skip-unsupported too.
	h, err := hookManifest(ref.Item.Path)
	if err != nil {
		return PlannedAction{
			Type:    ref.Type,
			Name:    ref.Name,
			Action:  "error-conflict",
			Detail:  fmt.Sprintf("hook %s not applied", ref.Name),
			Problem: err.Error(),
		}
	}

	// A hook whose event is a real event the provider simply has no settings
	// key for would merge as dead config the provider never reads
	// (syllago-xqlc1). Plan it as skip-unsupported so Apply can gate on it and
	// --skip-unsupported can pass it over. An unknown or malformed event is
	// not skippable: CheckHook below refuses it.
	if converter.IsValidHookEvent(h.Event) && !converter.ProviderSupportsHookEvent(h.Event, prov.Slug) {
		return PlannedAction{
			Type:    ref.Type,
			Name:    ref.Name,
			Action:  "skip-unsupported",
			Detail:  fmt.Sprintf("hook %s not applied", ref.Name),
			Problem: fmt.Sprintf("%s does not support hook event %q", prov.Name, h.Event),
		}
	}
	// settingsPathFor fails only for providers CheckHook refuses itself.
	settingsPath, _ := settingsPathFor(prov, homeDir, resolver)
	// CheckHook also finds a hook the merge would refuse as installed
	// under the legacy root, which is skipped as the loop above skips one.
	if err := installer.CheckHook(ref.Item, h, prov, repoRoot, settingsPath, inst); errors.Is(err, installer.ErrHookInstalled) {
		return PlannedAction{
			Type:   ref.Type,
			Name:   ref.Name,
			Action: "skip-exists",
			Detail: err.Error(),
		}
	} else if err != nil {
		return PlannedAction{
			Type:    ref.Type,
			Name:    ref.Name,
			Action:  "error-conflict",
			Detail:  fmt.Sprintf("hook %s not applied", ref.Name),
			Problem: err.Error(),
		}
	}

	return PlannedAction{
		Type:    ref.Type,
		Name:    ref.Name,
		Action:  "merge-hook",
		Detail:  fmt.Sprintf("merge hook %s into settings.json", ref.Name),
		Problem: highFindings(ref.Item.Path),
	}
}

// highFindings describes the high-severity findings the built-in scanner
// reports for a hook item, which the apply refuses unless forced. It
// returns "" when there are none.
func highFindings(itemPath string) string {
	itemDir := itemPath
	if fi, err := os.Stat(itemPath); err == nil && !fi.IsDir() {
		itemDir = filepath.Dir(itemPath)
	}
	result, _ := converter.RunScanChain(itemDir, nil)
	var found []string
	for _, f := range result.Findings {
		if strings.EqualFold(f.Severity, "high") {
			found = append(found, fmt.Sprintf("%s in %s", f.Description, f.File))
		}
	}
	if len(found) == 0 {
		return ""
	}
	return fmt.Sprintf("high-severity scanner findings: %s", strings.Join(found, ", "))
}

// hookManifest reads the single hook in a hook item's hook.json, and
// errors when the file is missing, malformed, or holds more than one hook.
func hookManifest(itemDir string) (converter.Hook, error) {
	hookFile := findHookFile(itemDir)
	if hookFile == "" {
		return converter.Hook{}, fmt.Errorf("no hook JSON file found in %s", itemDir)
	}
	data, err := os.ReadFile(hookFile)
	if err != nil {
		return converter.Hook{}, fmt.Errorf("reading hook file: %w", err)
	}
	// Loadouts only emit syllago-written hook.json, which always has
	// exactly one handler per file.
	manifest, err := converter.ParseManifest(data)
	if err != nil {
		return converter.Hook{}, fmt.Errorf("parsing hook manifest: %w", err)
	}
	if len(manifest.Hooks) != 1 {
		return converter.Hook{}, fmt.Errorf("hook file has %d hooks; syllago hook.json must contain exactly 1", len(manifest.Hooks))
	}
	return manifest.Hooks[0], nil
}

// previewMCP checks installed.json for an existing MCP entry, then whether
// the merge would refuse the item, such as for a server the user defined.
// A server already installed under the legacy root is skipped, as one in
// this project's installed.json is.
func previewMCP(ref ResolvedRef, prov provider.Provider, repoRoot string, inst *installer.Installed) PlannedAction {
	if inst.FindMCP(ref.Name, prov.Slug) >= 0 {
		return PlannedAction{
			Type:   ref.Type,
			Name:   ref.Name,
			Action: "skip-exists",
			Detail: fmt.Sprintf("MCP server %s already installed", ref.Name),
		}
	}
	err := installer.CheckMCP(ref.Item, prov, repoRoot, inst)
	if errors.Is(err, installer.ErrMCPInstalled) {
		return PlannedAction{
			Type:   ref.Type,
			Name:   ref.Name,
			Action: "skip-exists",
			Detail: err.Error(),
		}
	}
	if err != nil {
		return PlannedAction{
			Type:    ref.Type,
			Name:    ref.Name,
			Action:  "error-conflict",
			Problem: err.Error(),
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
