package installer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/snapshot"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// computeGroupHash computes the SHA256 hex hash of a matcher group JSON blob.
// Retained for orphan detection (orphans.go), which hashes raw settings entries.
func computeGroupHash(matcherGroup []byte) string {
	hash := sha256.Sum256(matcherGroup)
	return hex.EncodeToString(hash[:])
}

// hookSettingsPath returns the path to the provider's hook config file.
// Declared as a var so tests can override it (same pattern as mcpConfigPath).
var hookSettingsPath = hookSettingsPathImpl

// hookSettingsPathImpl resolves a provider's hook file rooted at the user's
// home directory, via the shared HookConfigPath resolver (ADR-0020 path table).
func hookSettingsPathImpl(prov provider.Provider) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return HookConfigPath(prov, home)
}

func installHook(item catalog.ContentItem, prov provider.Provider, repoRoot string) (Placement, error) {
	// item.Path is already absolute (set by scanner).
	h, err := readSingleManifestHook(item.Path)
	if err != nil {
		return Placement{}, fmt.Errorf("parsing hook file: %w", err)
	}

	// M3: validate the event name (rejects garbage and prevents key injection).
	if !converter.IsValidHookEvent(h.Event) {
		return Placement{}, fmt.Errorf("unknown hook event %q: must be a known canonical or provider event name", h.Event)
	}

	// ADR-0020: install through the provider's HookAdapter. No adapter (amp,
	// codex) means the hook cannot be serialized — reject rather than write
	// config the provider never reads.
	adapter := converter.AdapterFor(prov.Slug)
	if adapter == nil {
		return Placement{}, fmt.Errorf("hook install not supported for %s (no encoder)", prov.Name)
	}

	model, err := hookStorageModelFor(prov.Slug)
	if err != nil {
		return Placement{}, err
	}

	canonHook, err := manifestHookToCanonical(h)
	if err != nil {
		return Placement{}, fmt.Errorf("building canonical hook: %w", err)
	}
	canonEvent := canonicalizeEvent(h.Event, prov.Slug)
	canonHook.Event = canonEvent

	// Event-support gate: reject events the adapter cannot represent. Adapter
	// capabilities are ADR-0020's source of truth (they see windsurf's
	// split-event support, which ProviderSupportsHookEvent misses).
	if !adapterSupportsEvent(adapter, canonEvent) {
		return Placement{}, fmt.Errorf("hook %q: %s does not support hook event %q", item.Name, prov.Name, h.Event)
	}

	// SECURITY (M2): run the pluggable scanner chain against the source hook
	// directory. High-severity findings block the install unless --force.
	itemDir := item.Path
	if fi, statErr := os.Stat(item.Path); statErr == nil && !fi.IsDir() {
		itemDir = filepath.Dir(item.Path)
	}
	scanResult, _ := converter.RunScanChain(itemDir, scannerChainPaths)
	for _, f := range scanResult.Findings {
		loc := f.File
		if f.Line > 0 {
			loc = fmt.Sprintf("%s:%d", f.File, f.Line)
		}
		fmt.Fprintf(output.ErrWriter, "  %s [%s] %s (scanner=%s)\n",
			strings.ToUpper(f.Severity), loc, f.Description, f.Scanner)
	}
	for _, e := range scanResult.Errors {
		fmt.Fprintf(output.ErrWriter, "  scanner error: %s\n", e)
	}
	if !scannerForceBypass && converter.HighestSeverity(scanResult.Findings) == "high" {
		return Placement{}, fmt.Errorf("hook %q has high-severity security findings; re-run with --force to install anyway", item.Name)
	}

	// SECURITY: copy referenced scripts to a stable location and rewrite the
	// command path (operates on the hook before encode).
	resolvedCmd, err := resolveHookCommandScript(canonHook.Handler.Command, item, repoRoot)
	if err != nil {
		return Placement{}, err
	}
	canonHook.Handler.Command = resolvedCmd

	// Stable identity from the post-round-trip canonical form. Also rejects
	// hooks the adapter drops (e.g. non-command handler on crush) before any
	// file is touched.
	groupHash, err := roundTripIdentity(adapter, canonHook)
	if err != nil {
		return Placement{}, fmt.Errorf("hook %q: %w", item.Name, err)
	}

	nativeEvent := nativeEventFor(canonEvent, prov.Slug)

	// Dedup against installed.json (name + event).
	inst, err := LoadInstalled(repoRoot)
	if err != nil {
		return Placement{}, fmt.Errorf("loading installed.json: %w", err)
	}
	if inst.FindHook(item.Name, nativeEvent) >= 0 {
		return Placement{}, fmt.Errorf("hook %s already installed for %s event", item.Name, nativeEvent)
	}
	if hookInstalledAtLegacyRoot(repoRoot, item.Name, nativeEvent) {
		return Placement{}, fmt.Errorf("hook %s already installed for %s event", item.Name, nativeEvent)
	}

	settingsPath, err := hookSettingsPath(prov)
	if err != nil {
		return Placement{}, err
	}

	snapshotDir, err := snapshot.CreateForHook(repoRoot, "hook-install:"+item.Name, []string{settingsPath})
	if err != nil {
		return Placement{}, fmt.Errorf("creating snapshot: %w", err)
	}

	existing, err := decodeExistingHooks(model, adapter, settingsPath)
	if err != nil {
		return Placement{}, err
	}
	all := make([]converter.CanonicalHook, 0, len(existing)+1)
	all = append(all, existing...)
	all = append(all, canonHook)

	encoded, err := adapter.Encode(&converter.CanonicalHooks{Spec: converter.SpecVersion, Hooks: all})
	if err != nil {
		return Placement{}, fmt.Errorf("encoding hooks: %w", err)
	}
	if err := writeHookFile(model, settingsPath, encoded.Content); err != nil {
		// Auto-rollback using the snapshot we just created.
		if manifest, _, loadErr := snapshot.Load(repoRoot); loadErr == nil {
			_ = snapshot.Restore(snapshotDir, manifest)
		}
		return Placement{}, fmt.Errorf("writing %s: %w", settingsPath, err)
	}

	inst.Hooks = append(inst.Hooks, InstalledHook{
		Name:        item.Name,
		Event:       nativeEvent,
		GroupHash:   groupHash,
		Command:     canonHook.Handler.Command,
		Source:      "export",
		Scope:       "global",
		InstalledAt: time.Now(),
	})
	if err := SaveInstalled(repoRoot, inst); err != nil {
		return Placement{}, fmt.Errorf("saving installed.json: %w", err)
	}

	desc := fmt.Sprintf("hooks.%s in %s", nativeEvent, settingsPath)
	return Placement{
		Mechanism: MechanismHookMerge,
		Path:      settingsPath,
		Keys:      []string{"hooks." + nativeEvent},
		desc:      desc,
	}, nil
}

func uninstallHook(item catalog.ContentItem, prov provider.Provider, repoRoot string) (Placement, error) {
	return uninstallHookAtRoot(item, prov, repoRoot, true)
}

func uninstallHookAtRoot(item catalog.ContentItem, prov provider.Provider, repoRoot string, allowLegacyFallback bool) (Placement, error) {
	h, err := readSingleManifestHook(item.Path)
	if err != nil {
		return Placement{}, fmt.Errorf("parsing hook file: %w", err)
	}

	adapter := converter.AdapterFor(prov.Slug)
	if adapter == nil {
		return Placement{}, fmt.Errorf("hook uninstall not supported for %s (no encoder)", prov.Name)
	}
	model, err := hookStorageModelFor(prov.Slug)
	if err != nil {
		return Placement{}, err
	}

	canonEvent := canonicalizeEvent(h.Event, prov.Slug)
	nativeEvent := nativeEventFor(canonEvent, prov.Slug)

	settingsPath, err := hookSettingsPath(prov)
	if err != nil {
		return Placement{}, err
	}

	inst, err := LoadInstalled(repoRoot)
	if err != nil {
		return Placement{}, fmt.Errorf("loading installed.json: %w", err)
	}
	instIdx := inst.FindHook(item.Name, nativeEvent)
	if instIdx < 0 {
		if allowLegacyFallback {
			if legacyRoot := legacyRootWithHookRecord(repoRoot, item.Name, nativeEvent); legacyRoot != "" {
				return uninstallHookAtRoot(item, prov, legacyRoot, false)
			}
		}
		return Placement{}, fmt.Errorf("hook %s not tracked for %s event (not installed by syllago)", item.Name, nativeEvent)
	}
	storedHash := inst.Hooks[instIdx].GroupHash

	// Identity-based match: decode the file, find the hook whose canonical
	// identity matches the stored hash, drop it, and re-encode.
	existing, err := decodeExistingHooks(model, adapter, settingsPath)
	if err != nil {
		return Placement{}, err
	}
	found := -1
	for i, eh := range existing {
		if hookIdentity(eh) == storedHash {
			found = i
			break
		}
	}
	if found == -1 {
		return Placement{}, fmt.Errorf("hook %s not found in %s (modified since installation; use 'syllago restore' to revert)", item.Name, settingsPath)
	}

	snapshotDir, err := snapshot.CreateForHook(repoRoot, "hook-uninstall:"+item.Name, []string{settingsPath})
	if err != nil {
		return Placement{}, fmt.Errorf("creating snapshot: %w", err)
	}

	remaining := make([]converter.CanonicalHook, 0, len(existing)-1)
	remaining = append(remaining, existing[:found]...)
	remaining = append(remaining, existing[found+1:]...)

	if model == hookStorageDirectory && len(remaining) == 0 {
		if err := os.Remove(settingsPath); err != nil && !os.IsNotExist(err) {
			if manifest, _, loadErr := snapshot.Load(repoRoot); loadErr == nil {
				_ = snapshot.Restore(snapshotDir, manifest)
			}
			return Placement{}, fmt.Errorf("removing %s: %w", settingsPath, err)
		}
	} else {
		encoded, err := adapter.Encode(&converter.CanonicalHooks{Spec: converter.SpecVersion, Hooks: remaining})
		if err != nil {
			return Placement{}, fmt.Errorf("encoding hooks: %w", err)
		}
		if err := writeHookFile(model, settingsPath, encoded.Content); err != nil {
			if manifest, _, loadErr := snapshot.Load(repoRoot); loadErr == nil {
				_ = snapshot.Restore(snapshotDir, manifest)
			}
			return Placement{}, fmt.Errorf("writing %s: %w", settingsPath, err)
		}
	}

	inst.RemoveHook(instIdx)
	if err := SaveInstalled(repoRoot, inst); err != nil {
		return Placement{}, fmt.Errorf("saving installed.json: %w", err)
	}

	desc := fmt.Sprintf("hooks.%s from %s", nativeEvent, settingsPath)
	return Placement{
		Mechanism: MechanismHookMerge,
		Path:      settingsPath,
		Keys:      []string{"hooks." + nativeEvent},
		desc:      desc,
	}, nil
}

func checkHookStatus(item catalog.ContentItem, prov provider.Provider, repoRoot string) Status {
	status := checkHookStatusAtRoot(item, prov, repoRoot)
	if status != StatusNotInstalled {
		return status
	}
	if legacyRoot := legacyInstalledRoot(repoRoot); legacyRoot != "" {
		if legacyStatus := checkHookStatusAtRoot(item, prov, legacyRoot); legacyStatus == StatusInstalled {
			return StatusInstalled
		}
	}
	return status
}

func checkHookStatusAtRoot(item catalog.ContentItem, prov provider.Provider, repoRoot string) Status {
	h, err := readSingleManifestHook(item.Path)
	if err != nil {
		return StatusNotAvailable
	}

	adapter := converter.AdapterFor(prov.Slug)
	if adapter == nil {
		return StatusNotAvailable
	}
	model, err := hookStorageModelFor(prov.Slug)
	if err != nil {
		return StatusNotAvailable
	}

	canonEvent := canonicalizeEvent(h.Event, prov.Slug)
	nativeEvent := nativeEventFor(canonEvent, prov.Slug)

	inst, err := LoadInstalled(repoRoot)
	if err != nil {
		return StatusNotAvailable
	}
	instIdx := inst.FindHook(item.Name, nativeEvent)
	if instIdx < 0 {
		return StatusNotInstalled
	}
	storedHash := inst.Hooks[instIdx].GroupHash

	settingsPath, err := hookSettingsPath(prov)
	if err != nil {
		return StatusNotInstalled
	}
	existing, err := decodeExistingHooks(model, adapter, settingsPath)
	if err != nil {
		return StatusNotInstalled
	}
	for _, eh := range existing {
		if hookIdentity(eh) == storedHash {
			return StatusInstalled
		}
	}
	return StatusNotInstalled
}

func hookInstalledAtLegacyRoot(repoRoot, name, nativeEvent string) bool {
	return legacyRootWithHookRecord(repoRoot, name, nativeEvent) != ""
}

func legacyRootWithHookRecord(repoRoot, name, nativeEvent string) string {
	legacyRoot := legacyInstalledRoot(repoRoot)
	if legacyRoot == "" {
		return ""
	}
	inst, err := LoadInstalled(legacyRoot)
	if err != nil {
		return ""
	}
	if inst.FindHook(name, nativeEvent) < 0 {
		return ""
	}
	return legacyRoot
}

// hookScriptsDir returns ~/.syllago/hooks/<name>/ for storing copied scripts.
func hookScriptsDir(name string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".syllago", "hooks", name), nil
}

// resolveHookCommandScript resolves a single hook command through the
// script-copying security logic (resolveHookScripts operates on a matcher
// group, so we wrap the command in a minimal group and read the rewritten
// command back). An empty command (e.g. a non-command handler) is a no-op.
func resolveHookCommandScript(cmd string, item catalog.ContentItem, repoRoot string) (string, error) {
	if cmd == "" {
		return "", nil
	}
	mg, err := sjson.SetBytes([]byte(`{}`), "hooks.0.command", cmd)
	if err != nil {
		return "", err
	}
	mg, err = resolveHookScripts(mg, item, repoRoot)
	if err != nil {
		return "", err
	}
	return gjson.GetBytes(mg, "hooks.0.command").String(), nil
}

// resolveHookScripts finds script file references in a hook's matcher group,
// copies them to a stable location (~/.syllago/hooks/<name>/), and rewrites
// the command paths in the JSON. This ensures hooks from registries don't
// break when the registry cache changes.
func resolveHookScripts(matcherGroup []byte, item catalog.ContentItem, repoRoot string) ([]byte, error) {
	// Resolve the item directory (hooks can be a file or directory)
	itemDir := item.Path
	fi, err := os.Stat(item.Path)
	if err == nil && !fi.IsDir() {
		itemDir = filepath.Dir(item.Path)
	}

	// Find all command fields in hooks array
	hooksArray := gjson.GetBytes(matcherGroup, "hooks")
	if !hooksArray.Exists() || !hooksArray.IsArray() {
		return matcherGroup, nil
	}

	var scriptsCopied bool
	result := matcherGroup

	for i, entry := range hooksArray.Array() {
		cmd := entry.Get("command").String()
		if cmd == "" {
			continue
		}

		// Use ExtractScriptRef to detect script references, including
		// those behind interpreter prefixes (e.g. "bash ./lint.sh").
		ref := converter.ExtractScriptRef(cmd)
		if ref == "" {
			continue // inline command like "echo lint"
		}

		// Only handle relative paths at install time — these are scripts
		// bundled into the library dir at add-time.
		var scriptPath string
		if strings.HasPrefix(ref, "./") || strings.HasPrefix(ref, "../") {
			scriptPath = filepath.Clean(filepath.Join(itemDir, ref))
			// Resolve symlinks before containment check to prevent symlink-based
			// path traversal (e.g., ./scripts -> /etc via a crafted symlink).
			if resolved, evalErr := filepath.EvalSymlinks(scriptPath); evalErr == nil {
				scriptPath = resolved
			}
			// Verify the resolved path stays within the item directory
			rel, relErr := filepath.Rel(itemDir, scriptPath)
			if relErr != nil || strings.HasPrefix(rel, "..") {
				return nil, fmt.Errorf("hook %q command references path outside item directory: %s", item.Name, ref)
			}
		}

		if scriptPath == "" {
			continue // absolute path — not a bundled script
		}

		// Check if the script exists
		if _, statErr := os.Stat(scriptPath); statErr != nil {
			continue // script doesn't exist, leave command as-is
		}

		// Show security warning on first script
		if !scriptsCopied {
			fmt.Fprintf(output.ErrWriter, "\n  SECURITY WARNING\n")
			fmt.Fprintf(output.ErrWriter, "  Hook %q references executable script files.\n", item.Name)
			fmt.Fprintf(output.ErrWriter, "  Scripts will be copied to ~/.syllago/hooks/%s/\n\n", item.Name)
			scriptsCopied = true
		}

		// Copy script to stable location
		destDir, err := hookScriptsDir(item.Name)
		if err != nil {
			return nil, fmt.Errorf("getting hook scripts dir: %w", err)
		}
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return nil, fmt.Errorf("creating hook scripts dir: %w", err)
		}

		scriptName := filepath.Base(scriptPath)
		destPath := filepath.Join(destDir, scriptName)

		scriptData, readErr := os.ReadFile(scriptPath)
		if readErr != nil {
			return nil, fmt.Errorf("reading script %s: %w", scriptPath, readErr)
		}
		if writeErr := os.WriteFile(destPath, scriptData, 0700); writeErr != nil {
			return nil, fmt.Errorf("copying script to %s: %w", destPath, writeErr)
		}

		// Rewrite command: replace the script ref with the stable absolute path
		newCmd := strings.Replace(cmd, ref, destPath, 1)
		key := fmt.Sprintf("hooks.%d.command", i)
		result, err = sjson.SetBytes(result, key, newCmd)
		if err != nil {
			return nil, fmt.Errorf("rewriting command for %s: %w", item.Name, err)
		}
	}

	return result, nil
}
