package loadout

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/snapshot"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ApplyOptions configures a loadout apply operation.
type ApplyOptions struct {
	Mode        string                  // "preview", "try", or "keep"
	Method      installer.InstallMethod // "symlink" (default) or "copy"
	ProjectRoot string
	HomeDir     string               // defaults to os.UserHomeDir() if empty
	RepoRoot    string               // catalog repo root for symlink source resolution
	Resolver    *config.PathResolver // optional path resolver for custom locations

	// SkipUnsupported applies the loadout without the hooks whose events the
	// target provider has no settings key for, instead of failing. Skipped
	// hooks are reported in ApplyResult.Warnings.
	SkipUnsupported bool
}

// UnsupportedHooksError rejects an apply because the loadout contains hooks
// whose events the target provider cannot read (syllago-xqlc1). Callers can
// detect it with errors.As to suggest --skip-unsupported.
type UnsupportedHooksError struct {
	Provider string
	Problems []string // one "name — problem" line per rejected hook
}

func (e *UnsupportedHooksError) Error() string {
	return fmt.Sprintf("%d hook(s) cannot be applied to %s:\n  %s",
		len(e.Problems), e.Provider, strings.Join(e.Problems, "\n  "))
}

// ApplyResult describes what happened during apply.
type ApplyResult struct {
	Actions     []PlannedAction // what was done (or planned, for preview)
	SnapshotDir string          // set on success for try/keep modes
	Warnings    []string

	// AutoRevertArmed reports whether a session-end auto-revert hook was
	// injected (try mode only). False when the provider has no session_end
	// event — the CLI must not then promise auto-revert.
	AutoRevertArmed bool
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
	refs, err := Resolve(manifest, cat, prov.Slug)
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
	actions, err := Preview(refs, prov, opts.ProjectRoot, opts.HomeDir, opts.Resolver)
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

	// Hooks the provider can't read fail the whole apply unless the caller
	// opted into a partial one — no silent partial coverage (syllago-xqlc1).
	var unsupported []string
	for _, a := range actions {
		if a.Action == "skip-unsupported" {
			unsupported = append(unsupported, fmt.Sprintf("%s — %s", a.Name, a.Problem))
		}
	}
	if len(unsupported) > 0 && !opts.SkipUnsupported {
		return nil, &UnsupportedHooksError{Provider: prov.Name, Problems: unsupported}
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
	for _, a := range actions {
		if a.Action == "skip-unsupported" {
			warnings = append(warnings, fmt.Sprintf("skipped %s: %s", a.Name, a.Problem))
		}
	}
	applyErr := applyActions(actions, refs, prov, opts, manifest.Name)
	if applyErr != nil {
		// Rollback: restore snapshot and clean up
		sm, _, loadErr := snapshot.Load(opts.ProjectRoot)
		if loadErr == nil {
			_ = snapshot.Restore(snapshotDir, sm)
			// Remove any symlinks we may have partially created
			for _, sr := range symlinkRecords {
				_ = os.Remove(sr.Path)
			}
		}
		_ = snapshot.Delete(snapshotDir)
		return nil, fmt.Errorf("applying loadout (rolled back): %w", applyErr)
	}

	// Step 6 (C7): For "try" mode, inject a session-end hook for auto-revert.
	autoRevertArmed := false
	if opts.Mode == "try" {
		injected, err := injectSessionEndHook(prov, opts.HomeDir, opts.Resolver)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to inject session-end hook: %v", err))
		} else if !injected {
			warnings = append(warnings, fmt.Sprintf("%s has no session-end hook event, so this loadout cannot auto-revert; run 'syllago loadout remove' to undo it", prov.Name))
		}
		autoRevertArmed = injected
	}

	return &ApplyResult{
		Actions:         actions,
		SnapshotDir:     snapshotDir,
		Warnings:        warnings,
		AutoRevertArmed: autoRevertArmed,
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
		if a.Action == "skip-exists" || a.Action == "skip-unsupported" {
			continue
		}

		ref := findRefByName(refs, a.Type, a.Name)
		if ref == nil {
			return fmt.Errorf("internal error: no ref found for %s %s", a.Type, a.Name)
		}

		switch a.Action {
		case "create-symlink":
			srcPath := symlinkSource(*ref)
			if opts.Method == installer.MethodCopy {
				if err := installer.CopyContent(srcPath, a.Detail); err != nil {
					return fmt.Errorf("copying %s: %w", a.Name, err)
				}
			} else {
				if err := installer.CreateSymlink(srcPath, a.Detail); err != nil {
					return fmt.Errorf("creating symlink for %s: %w", a.Name, err)
				}
			}
			inst.Symlinks = append(inst.Symlinks, installer.InstalledSymlink{
				Path:        a.Detail,
				Target:      srcPath,
				Source:      source,
				InstalledAt: time.Now(),
			})

		case "merge-hook":
			if err := applyHook(*ref, prov, opts.HomeDir, opts.Resolver, inst, source); err != nil {
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

// settingsPathFor computes the settings.json path for a provider, using the
// resolver's base dir if configured, otherwise falling back to homeDir.
func settingsPathFor(prov provider.Provider, homeDir string, resolver *config.PathResolver) string {
	base := homeDir
	if resolver != nil {
		if bd := resolver.BaseDir(prov.Slug); bd != "" {
			base = bd
		}
	}
	// Crush keeps hooks in its unified crush.json, not a settings.json
	// (mirrors installer.hookSettingsPathImpl). Routing here also covers the
	// snapshot backup list and remove, which build paths via this function.
	filename := "settings.json"
	if prov.Slug == "crush" {
		filename = "crush.json"
	}
	return filepath.Join(base, prov.ConfigDir, filename)
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
func applyHook(ref ResolvedRef, prov provider.Provider, homeDir string, resolver *config.PathResolver, inst *installer.Installed, source string) error {
	// Find the hook JSON file in the item directory
	hookFile := findHookFile(ref.Item.Path)
	if hookFile == "" {
		return fmt.Errorf("no hook JSON file found in %s", ref.Item.Path)
	}

	data, err := os.ReadFile(hookFile)
	if err != nil {
		return fmt.Errorf("reading hook file: %w", err)
	}

	// Parse the canonical hooks/0.1 Manifest and pull out the single hook it
	// contains. Loadouts only emit syllago-written hook.json, which always has
	// exactly one handler per file.
	manifest, err := converter.ParseManifest(data)
	if err != nil {
		return fmt.Errorf("parsing hook manifest: %w", err)
	}
	if len(manifest.Hooks) != 1 {
		return fmt.Errorf("hook file has %d hooks; syllago hook.json must contain exactly 1", len(manifest.Hooks))
	}
	hds, err := converter.HookDataFromManifest(manifest)
	if err != nil {
		return fmt.Errorf("converting manifest: %w", err)
	}
	hd := hds[0]
	event := hd.Event

	// Validate before using the event as an sjson key (mirrors installHook M3:
	// prevents key injection via dots in crafted event names).
	if !converter.IsValidHookEvent(event) {
		return fmt.Errorf("unknown hook event %q: must be a known canonical or provider event name", event)
	}

	// Reject events the provider has no settings key for (mirrors installHook,
	// syllago-xqlc1). Apply already gates these via the skip-unsupported
	// preview action; this is the defense-in-depth backstop at the actual
	// merge point so dead config can never slip through if the two diverge.
	if !converter.ProviderSupportsHookEvent(event, prov.Slug) {
		return fmt.Errorf("hook %q: %s does not support hook event %q", ref.Name, prov.Name, event)
	}

	// Translate canonical names to provider-native before the merge (mirrors
	// installHook): the event becomes the settings key the provider actually
	// reads, and matcher tool names become the names the provider tests its
	// hook regexes against. Already-native names pass through unchanged.
	if nativeEvent, ok := converter.TranslateHookEvent(event, prov.Slug); ok {
		event = nativeEvent
	}
	matcher := hd.Matcher
	if matcher != "" {
		matcher = converter.TranslateMatcher(matcher, prov.Slug)
	}

	// Build the provider-shape matcher group for merging into settings.json.
	group := struct {
		Matcher string                `json:"matcher,omitempty"`
		Hooks   []converter.HookEntry `json:"hooks"`
	}{
		Matcher: matcher,
		Hooks:   hd.Hooks,
	}
	matcherGroup, err := json.Marshal(group)
	if err != nil {
		return fmt.Errorf("building matcher group: %w", err)
	}

	// Resolve relative command paths to absolute
	matcherGroup = resolveHookCommands(matcherGroup, ref.Item.Path)

	// Crush stores flat hook entries ({name, matcher, command, timeout}) in
	// crush.json rather than CC-shape matcher groups (mirrors installHook).
	// Flatten last so command resolution above sees the standard shape.
	commandPath := "hooks.0.command"
	if prov.Slug == "crush" {
		matcherGroup, err = installer.FlattenForCrush(matcherGroup, manifest.Hooks[0].Name)
		if err != nil {
			return fmt.Errorf("hook %q: %w", ref.Name, err)
		}
		commandPath = "command"
	}

	// Append to the provider's hook config file
	settingsPath := settingsPathFor(prov, homeDir, resolver)
	if err := appendHookEntry(settingsPath, event, matcherGroup); err != nil {
		return err
	}

	// Extract command for tracking (crush entries are flat)
	command := gjson.GetBytes(matcherGroup, commandPath).String()

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
	rawData, err := os.ReadFile(filepath.Join(ref.Item.Path, "config.json"))
	if err != nil {
		return fmt.Errorf("reading config.json: %w", err)
	}

	jsonKey := installer.MCPConfigKey(prov)

	// Extract server entries — handles both nested and flat config.json formats
	entries, err := installer.ExtractServerEntries(rawData, ref.Name, jsonKey)
	if err != nil {
		return err
	}

	// Determine config file path
	cfgPath, err := installer.MCPConfigPathFor(prov, projectRoot)
	if err != nil {
		return fmt.Errorf("MCP config path: %w", err)
	}

	fileData, err := readJSONFileOrEmpty(cfgPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", cfgPath, err)
	}

	// Merge each server entry
	var serverNames []string
	for name, configData := range entries {
		key := jsonKey + "." + name
		fileData, err = sjson.SetRawBytes(fileData, key, configData)
		if err != nil {
			return fmt.Errorf("setting %s: %w", key, err)
		}
		serverNames = append(serverNames, name)
	}

	if err := writeJSONFileAtomic(cfgPath, fileData); err != nil {
		return fmt.Errorf("writing %s: %w", cfgPath, err)
	}

	inst.MCP = append(inst.MCP, installer.InstalledMCP{
		Name:        ref.Name,
		ServerNames: serverNames,
		Source:      source,
		InstalledAt: time.Now(),
	})

	return nil
}

// injectSessionEndHook appends a session-end hook that runs
// "syllago loadout remove --auto" for try-mode auto-revert. The hook is NOT
// tracked in installed.json — it gets reverted when the snapshot restores the
// settings file.
//
// Returns injected=false (no error) when the provider has no session_end
// event: the key would be dead config the provider never reads, and for crush
// it would corrupt the real crush.json (wrong key and CC-shape group). Callers
// warn the user to revert manually in that case.
func injectSessionEndHook(prov provider.Provider, homeDir string, resolver *config.PathResolver) (injected bool, err error) {
	// Translate to the provider-native event key; skip if unsupported so we
	// never write a key the provider can't read (syllago-xqlc1).
	event, ok := converter.TranslateHookEvent("session_end", prov.Slug)
	if !ok {
		return false, nil
	}

	settingsPath := settingsPathFor(prov, homeDir, resolver)

	// Build the session-end hook entry
	hookEntry := map[string]interface{}{
		"matcher": "",
		"hooks": []map[string]interface{}{
			{
				"type":    "command",
				"command": "syllago loadout remove --auto",
			},
		},
	}

	hookJSON, err := json.Marshal(hookEntry)
	if err != nil {
		return false, fmt.Errorf("marshaling session-end hook: %w", err)
	}

	if err := appendHookEntry(settingsPath, event, hookJSON); err != nil {
		return false, err
	}
	return true, nil
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

	// "try" mode backs up the settings file only when a session-end auto-revert
	// hook will actually be injected — i.e. the provider has a session_end
	// event. Backing it up otherwise means loadout remove would restore (and
	// so clobber) a file this apply never wrote to; for crush the settings
	// file is the real crush.json (syllago-xqlc1). Hooks in the loadout have
	// already set needsSettings via their merge-hook actions above.
	if opts.Mode == "try" && converter.ProviderSupportsHookEvent("session_end", prov.Slug) {
		needsSettings = true
	}

	if needsSettings {
		files = append(files, settingsPathFor(prov, opts.HomeDir, opts.Resolver))
	}
	if needsMCPConfig {
		mcpPath, err := installer.MCPConfigPathFor(prov, opts.ProjectRoot)
		if err == nil {
			files = append(files, mcpPath)
		}
	}

	// Also back up installed.json
	files = append(files, filepath.Join(opts.ProjectRoot, ".syllago", "installed.json"))

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
// Returns an error if the file exists but contains invalid JSON.
func readJSONFileOrEmpty(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []byte("{}"), nil
	}
	if err != nil {
		return nil, err
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("%s contains invalid JSON; fix or delete the file before applying a loadout", filepath.Base(path))
	}
	return data, nil
}

// writeJSONFileAtomic writes JSON data atomically using a temp file and rename.
func writeJSONFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	// Use random suffix to prevent predictable temp path attacks
	suffix := make([]byte, 8)
	if _, err := cryptorand.Read(suffix); err != nil {
		return fmt.Errorf("generating temp suffix: %w", err)
	}
	tmpPath := path + ".tmp." + hex.EncodeToString(suffix)
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}
