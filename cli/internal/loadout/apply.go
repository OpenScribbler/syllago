package loadout

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
	"github.com/OpenScribbler/nesco/cli/internal/installer"
	"github.com/OpenScribbler/nesco/cli/internal/provider"
	"github.com/OpenScribbler/nesco/cli/internal/snapshot"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ApplyOptions configures a loadout apply operation.
type ApplyOptions struct {
	Mode        string // "preview", "try", or "keep"
	ProjectRoot string
	HomeDir     string // defaults to os.UserHomeDir() if empty
	RepoRoot    string // catalog repo root for symlink source resolution
}

// ApplyResult describes what happened during apply.
type ApplyResult struct {
	Actions     []PlannedAction // what was done (or planned, for preview)
	SnapshotDir string          // set on success for try/keep modes
	Warnings    []string
}

// Apply resolves, validates, and applies a loadout to the provider.
//
// The sequence is: Resolve -> Validate -> Preview -> Snapshot -> Apply items -> Record.
// If any step after snapshot creation fails, the snapshot is restored (all-or-nothing).
//
// Modes:
//   - "preview": computes actions without touching files. Good for dry runs.
//   - "try": applies changes and injects a SessionEnd hook that auto-reverts on session close.
//   - "keep": applies changes permanently.
//
// Gotchas:
//   - The snapshot is taken BEFORE any changes are made, so rollback always has clean state.
//   - The SessionEnd hook injected for "try" mode is NOT recorded in installed.json --
//     it lives only in the backed-up settings.json and gets reverted with the snapshot.
func Apply(manifest *Manifest, cat *catalog.Catalog, prov provider.Provider, opts ApplyOptions) (*ApplyResult, error) {
	if opts.HomeDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("getting home dir: %w", err)
		}
		opts.HomeDir = home
	}

	// Step 1: Resolve all manifest references to catalog items
	refs, err := Resolve(manifest, cat)
	if err != nil {
		return nil, fmt.Errorf("resolving references: %w", err)
	}

	// Step 2: Validate resolved refs
	issues := Validate(refs, prov)
	if len(issues) > 0 {
		msg := "validation failed:"
		for _, issue := range issues {
			msg += fmt.Sprintf("\n  %s: %s", issue.Ref.Name, issue.Problem)
		}
		return nil, fmt.Errorf("%s", msg)
	}

	// Step 3: Preview what would happen
	actions, err := Preview(refs, prov, opts.ProjectRoot, opts.HomeDir)
	if err != nil {
		return nil, fmt.Errorf("previewing: %w", err)
	}

	// For preview mode, return immediately
	if opts.Mode == "preview" {
		return &ApplyResult{Actions: actions}, nil
	}

	// Check for conflicts before doing anything
	for _, a := range actions {
		if a.Action == "error-conflict" {
			return nil, fmt.Errorf("conflict: %s %s: %s", a.Type.Label(), a.Name, a.Problem)
		}
	}

	// Step 4: Collect files to back up and create snapshot
	filesToBackup := collectBackupFiles(actions, prov, opts)
	var symlinkRecords []snapshot.SymlinkRecord
	var hookScripts []string

	for _, a := range actions {
		if a.Action == "create-symlink" {
			ref := findRefByName(refs, a.Type, a.Name)
			symlinkRecords = append(symlinkRecords, snapshot.SymlinkRecord{
				Path:   a.Detail,
				Target: symlinkSource(*ref),
			})
		}
		if a.Action == "merge-hook" {
			hookScripts = append(hookScripts, a.Name)
		}
	}

	snapshotDir, err := snapshot.Create(opts.ProjectRoot, manifest.Name, opts.Mode,
		filesToBackup, symlinkRecords, hookScripts)
	if err != nil {
		return nil, fmt.Errorf("creating snapshot: %w", err)
	}

	// Step 5: Apply each action. On failure, rollback.
	var warnings []string
	applyErr := applyActions(actions, refs, prov, opts, manifest.Name)
	if applyErr != nil {
		// Rollback: restore snapshot and clean up
		sm, _, loadErr := snapshot.Load(opts.ProjectRoot)
		if loadErr == nil {
			snapshot.Restore(snapshotDir, sm)
			// Remove any symlinks we may have partially created
			for _, sr := range symlinkRecords {
				os.Remove(sr.Path)
			}
		}
		snapshot.Delete(snapshotDir)
		return nil, fmt.Errorf("applying loadout (rolled back): %w", applyErr)
	}

	// Step 6 (C7): For "try" mode, inject SessionEnd hook for auto-revert
	if opts.Mode == "try" {
		if err := injectSessionEndHook(prov, opts.HomeDir); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to inject SessionEnd hook: %v", err))
		}
	}

	return &ApplyResult{
		Actions:     actions,
		SnapshotDir: snapshotDir,
		Warnings:    warnings,
	}, nil
}

// applyActions executes each planned action against the filesystem.
func applyActions(actions []PlannedAction, refs []ResolvedRef, prov provider.Provider, opts ApplyOptions, loadoutName string) error {
	inst, err := installer.LoadInstalled(opts.ProjectRoot)
	if err != nil {
		return fmt.Errorf("loading installed.json: %w", err)
	}

	source := "loadout:" + loadoutName

	for _, a := range actions {
		if a.Action == "skip-exists" {
			continue
		}

		ref := findRefByName(refs, a.Type, a.Name)
		if ref == nil {
			return fmt.Errorf("internal error: no ref found for %s %s", a.Type, a.Name)
		}

		switch a.Action {
		case "create-symlink":
			srcPath := symlinkSource(*ref)
			if err := installer.CreateSymlink(srcPath, a.Detail); err != nil {
				return fmt.Errorf("creating symlink for %s: %w", a.Name, err)
			}
			inst.Symlinks = append(inst.Symlinks, installer.InstalledSymlink{
				Path:        a.Detail,
				Target:      srcPath,
				Source:      source,
				InstalledAt: time.Now(),
			})

		case "merge-hook":
			if err := applyHook(*ref, prov, opts.HomeDir, inst, source); err != nil {
				return fmt.Errorf("merging hook %s: %w", a.Name, err)
			}

		case "merge-mcp":
			if err := applyMCP(*ref, prov, opts.ProjectRoot, inst, source); err != nil {
				return fmt.Errorf("merging MCP %s: %w", a.Name, err)
			}
		}
	}

	// Save all tracking in one write
	if err := installer.SaveInstalled(opts.ProjectRoot, inst); err != nil {
		return fmt.Errorf("saving installed.json: %w", err)
	}

	return nil
}

// applyHook reads a hook JSON file and appends it to settings.json.
// This is the lower-level helper that works with raw hook data, avoiding the
// catalog.ContentItem-dependent installHook in the installer package.
//
// Key trade-off: we duplicate some logic from installer.installHook here rather
// than exporting it. The alternative would be to export appendHookEntry from the
// installer package, but that creates a tighter coupling. Since the loadout engine
// has different tracking needs (source is "loadout:<name>" vs "export"), it's cleaner
// to keep this self-contained.
func applyHook(ref ResolvedRef, prov provider.Provider, homeDir string, inst *installer.Installed, source string) error {
	// Find the hook JSON file in the item directory
	hookFile := findHookFile(ref.Item.Path)
	if hookFile == "" {
		return fmt.Errorf("no hook JSON file found in %s", ref.Item.Path)
	}

	data, err := os.ReadFile(hookFile)
	if err != nil {
		return fmt.Errorf("reading hook file: %w", err)
	}

	// Extract event field
	event := gjson.GetBytes(data, "event").String()
	if event == "" {
		return fmt.Errorf("hook file missing 'event' field")
	}

	// Remove event field to get the matcher group
	matcherGroup, err := sjson.DeleteBytes(data, "event")
	if err != nil {
		return fmt.Errorf("stripping event field: %w", err)
	}

	// Resolve relative command paths to absolute
	matcherGroup = resolveHookCommands(matcherGroup, ref.Item.Path)

	// Append to settings.json
	settingsPath := filepath.Join(homeDir, prov.ConfigDir, "settings.json")
	if err := appendHookEntry(settingsPath, event, matcherGroup); err != nil {
		return err
	}

	// Extract command for tracking
	command := gjson.GetBytes(matcherGroup, "hooks.0.command").String()

	inst.Hooks = append(inst.Hooks, installer.InstalledHook{
		Name:        ref.Name,
		Event:       event,
		Command:     command,
		Source:      source,
		InstalledAt: time.Now(),
	})

	return nil
}

// appendHookEntry appends a matcher group to hooks.<event> in settings.json.
// This is the shared lower-level helper for merging hook JSON.
func appendHookEntry(settingsPath string, event string, matcherGroup []byte) error {
	fileData, err := readJSONFileOrEmpty(settingsPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", settingsPath, err)
	}

	key := "hooks." + event + ".-1"
	fileData, err = sjson.SetRawBytes(fileData, key, matcherGroup)
	if err != nil {
		return fmt.Errorf("appending hook: %w", err)
	}

	if err := writeJSONFileAtomic(settingsPath, fileData); err != nil {
		return fmt.Errorf("writing %s: %w", settingsPath, err)
	}

	return nil
}

// applyMCP reads MCP config.json and merges it into the provider's MCP config file.
func applyMCP(ref ResolvedRef, prov provider.Provider, projectRoot string, inst *installer.Installed, source string) error {
	_ = prov        // v1: Claude Code only, config path derived below
	_ = projectRoot // v1: not needed for home-scoped config

	rawData, err := os.ReadFile(filepath.Join(ref.Item.Path, "config.json"))
	if err != nil {
		return fmt.Errorf("reading config.json: %w", err)
	}

	// Parse and re-serialize to whitelist fields
	var cfg installer.MCPConfig
	if err := json.Unmarshal(rawData, &cfg); err != nil {
		return fmt.Errorf("parsing config.json: %w", err)
	}
	configData, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("serializing config: %w", err)
	}

	// Determine config file path
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home dir: %w", err)
	}

	// Claude Code MCP config is at ~/.claude.json
	cfgPath := filepath.Join(home, ".claude.json")

	fileData, err := readJSONFileOrEmpty(cfgPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", cfgPath, err)
	}

	key := "mcpServers." + ref.Name
	fileData, err = sjson.SetRawBytes(fileData, key, configData)
	if err != nil {
		return fmt.Errorf("setting %s: %w", key, err)
	}

	if err := writeJSONFileAtomic(cfgPath, fileData); err != nil {
		return fmt.Errorf("writing %s: %w", cfgPath, err)
	}

	inst.MCP = append(inst.MCP, installer.InstalledMCP{
		Name:        ref.Name,
		Source:      source,
		InstalledAt: time.Now(),
	})

	return nil
}

// injectSessionEndHook appends a SessionEnd hook that runs "nesco loadout remove --auto".
// This hook is NOT tracked in installed.json -- it gets reverted when the snapshot restores
// settings.json.
func injectSessionEndHook(prov provider.Provider, homeDir string) error {
	settingsPath := filepath.Join(homeDir, prov.ConfigDir, "settings.json")

	// Build the SessionEnd hook entry
	hookEntry := map[string]interface{}{
		"matcher": "",
		"hooks": []map[string]interface{}{
			{
				"type":    "command",
				"command": "nesco loadout remove --auto",
			},
		},
	}

	hookJSON, err := json.Marshal(hookEntry)
	if err != nil {
		return fmt.Errorf("marshaling SessionEnd hook: %w", err)
	}

	return appendHookEntry(settingsPath, "SessionEnd", hookJSON)
}

// collectBackupFiles determines which files need backing up before apply.
func collectBackupFiles(actions []PlannedAction, prov provider.Provider, opts ApplyOptions) []string {
	var files []string
	needsSettings := false
	needsMCPConfig := false

	for _, a := range actions {
		if a.Action == "merge-hook" {
			needsSettings = true
		}
		if a.Action == "merge-mcp" {
			needsMCPConfig = true
		}
	}

	// "try" mode always backs up settings.json (for SessionEnd hook injection)
	if opts.Mode == "try" {
		needsSettings = true
	}

	if needsSettings {
		files = append(files, filepath.Join(opts.HomeDir, prov.ConfigDir, "settings.json"))
	}
	if needsMCPConfig {
		home, err := os.UserHomeDir()
		if err == nil {
			files = append(files, filepath.Join(home, ".claude.json"))
		}
	}

	// Also back up installed.json
	files = append(files, filepath.Join(opts.ProjectRoot, ".nesco", "installed.json"))

	return files
}

// findRefByName finds a ResolvedRef by type and name.
func findRefByName(refs []ResolvedRef, ct catalog.ContentType, name string) *ResolvedRef {
	for i := range refs {
		if refs[i].Type == ct && refs[i].Name == name {
			return &refs[i]
		}
	}
	return nil
}

// findHookFile locates the hook JSON file in an item directory.
// Checks hook.json first, then falls back to any .json file.
func findHookFile(itemDir string) string {
	hookPath := filepath.Join(itemDir, "hook.json")
	if _, err := os.Stat(hookPath); err == nil {
		return hookPath
	}
	entries, err := os.ReadDir(itemDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			return filepath.Join(itemDir, e.Name())
		}
	}
	return ""
}

// resolveHookCommands resolves relative command paths in hook JSON to absolute paths.
func resolveHookCommands(matcherGroup []byte, itemDir string) []byte {
	// Walk through hooks array and resolve command paths
	hooksArray := gjson.GetBytes(matcherGroup, "hooks")
	if !hooksArray.Exists() || !hooksArray.IsArray() {
		return matcherGroup
	}

	result := matcherGroup
	for i, hook := range hooksArray.Array() {
		cmd := hook.Get("command").String()
		if cmd != "" {
			resolved := ResolveHookCommand(itemDir, cmd)
			if resolved != cmd {
				key := fmt.Sprintf("hooks.%d.command", i)
				updated, err := sjson.SetBytes(result, key, resolved)
				if err == nil {
					result = updated
				}
			}
		}
	}
	return result
}

// readJSONFileOrEmpty reads a JSON file, returning {} if it doesn't exist.
func readJSONFileOrEmpty(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []byte("{}"), nil
	}
	return data, err
}

// writeJSONFileAtomic writes JSON data atomically using a temp file and rename.
func writeJSONFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}
