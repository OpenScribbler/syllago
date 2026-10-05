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
	"github.com/OpenScribbler/syllago/cli/internal/provider"
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
// home directory, via the shared HookConfigPath resolver.
func hookSettingsPathImpl(prov provider.Provider) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return HookConfigPath(prov, home)
}

// hookSourceProvider names the provider an item's hooks were written for.
func hookSourceProvider(item catalog.ContentItem) string {
	if item.Meta != nil && item.Meta.SourceProvider != "" {
		return item.Meta.SourceProvider
	}
	return item.Provider
}

func installHook(item catalog.ContentItem, prov provider.Provider, repoRoot string, scan ScanOptions) (Placement, error) {
	// item.Path is already absolute (set by scanner).
	h, err := readSingleManifestHook(item.Path)
	if err != nil {
		return Placement{}, fmt.Errorf("parsing hook file: %w", err)
	}

	// M3: validate the event name (rejects garbage and prevents key injection).
	if !converter.IsValidHookEvent(h.Event) {
		return Placement{}, fmt.Errorf("unknown hook event %q: must be a known canonical or provider event name", h.Event)
	}

	// Install through the provider's HookAdapter. No adapter (amp,
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

	canonHook, err := converter.CanonicalHookFromManifest(h)
	if err != nil {
		return Placement{}, fmt.Errorf("building canonical hook: %w", err)
	}
	canonEvent := converter.CanonicalHookEvent(h.Event, hookSourceProvider(item), prov.Slug)
	canonHook.Event = canonEvent

	// Event-support gate: reject events the adapter cannot represent. Adapter
	// capabilities are the source of truth.
	if !adapterSupportsEvent(adapter, canonEvent) {
		return Placement{}, fmt.Errorf("hook %q: %s does not support hook event %q", item.Name, prov.Name, h.Event)
	}

	// SECURITY (M2): run the pluggable scanner chain against the source hook
	// directory. High-severity findings block the install unless --force.
	itemDir := item.Path
	if fi, statErr := os.Stat(item.Path); statErr == nil && !fi.IsDir() {
		itemDir = filepath.Dir(item.Path)
	}
	scanResult, _ := converter.RunScanChain(itemDir, scan.Scanners)
	var notices []Notice
	for _, f := range scanResult.Findings {
		loc := f.File
		if f.Line > 0 {
			loc = fmt.Sprintf("%s:%d", f.File, f.Line)
		}
		notices = append(notices, Notice{
			Kind:     NoticeScannerFinding,
			Severity: f.Severity,
			Message:  fmt.Sprintf("[%s] %s (scanner=%s)", loc, f.Description, f.Scanner),
		})
	}
	for _, e := range scanResult.Errors {
		notices = append(notices, Notice{Kind: NoticeScannerError, Message: e})
	}
	if !scan.Force && converter.HighestSeverity(scanResult.Findings) == "high" {
		return Placement{Notices: notices}, fmt.Errorf("hook %q has high-severity security findings; re-run with --force to install anyway", item.Name)
	}

	// SECURITY: copy referenced scripts to a stable location and rewrite the
	// command path (operates on the hook before encode).
	resolvedCmd, copied, err := resolveHookCommandScript(canonHook.Handler.Command, item, repoRoot)
	if copied {
		notices = append(notices, Notice{
			Kind:    NoticeScriptSecurity,
			Message: fmt.Sprintf("Hook %q references executable script files.\nScripts will be copied to ~/.syllago/hooks/%s/", item.Name, item.Name),
		})
	}
	if err != nil {
		return Placement{Notices: notices}, err
	}
	canonHook.Handler.Command = resolvedCmd

	// Stable identity from the post-round-trip canonical form. Also rejects
	// hooks the adapter drops (e.g. non-command handler on crush) before any
	// file is touched.
	groupHash, encodeWarnings, err := roundTripIdentity(adapter, canonHook)
	if err != nil {
		return Placement{Notices: notices}, fmt.Errorf("hook %q: %w", item.Name, err)
	}
	notices = append(notices, conversionNotices(item, converter.HookWarnings(encodeWarnings, hookSourceProvider(item), prov.Slug))...)

	nativeEvent := nativeEventFor(canonEvent, prov.Slug)

	settingsPath, err := hookSettingsPath(prov)
	if err != nil {
		return Placement{Notices: notices}, err
	}
	existing, err := decodeExistingHooks(model, adapter, settingsPath)
	if err != nil {
		return Placement{Notices: notices}, err
	}

	// Dedup against installed.json (name + event + provider).
	inst, err := LoadInstalled(repoRoot)
	if err != nil {
		return Placement{Notices: notices}, fmt.Errorf("loading installed.json: %w", err)
	}
	if hookTracked(inst, item.Name, nativeEvent, prov.Slug, existing) {
		return Placement{Notices: notices}, fmt.Errorf("hook %s already installed for %s event", item.Name, nativeEvent)
	}
	if hookTrackedAtLegacyRoot(repoRoot, item.Name, nativeEvent, prov.Slug, existing) {
		return Placement{Notices: notices}, fmt.Errorf("hook %s already installed for %s event", item.Name, nativeEvent)
	}

	all := make([]converter.CanonicalHook, 0, len(existing)+1)
	all = append(all, existing...)
	all = append(all, canonHook)

	encoded, err := adapter.Encode(&converter.CanonicalHooks{Spec: converter.SpecVersion, Hooks: all})
	if err != nil {
		return Placement{Notices: notices}, fmt.Errorf("encoding hooks: %w", err)
	}
	if err := writeHookFile(model, settingsPath, encoded.Content); err != nil {
		return Placement{Notices: notices}, fmt.Errorf("writing %s: %w", settingsPath, err)
	}

	inst.Hooks = append(inst.Hooks, InstalledHook{
		Name:        item.Name,
		Event:       nativeEvent,
		GroupHash:   groupHash,
		Command:     canonHook.Handler.Command,
		Source:      "export",
		Scope:       "global",
		Provider:    prov.Slug,
		InstalledAt: time.Now(),
	})
	if err := SaveInstalled(repoRoot, inst); err != nil {
		return Placement{Notices: notices}, fmt.Errorf("saving installed.json: %w", err)
	}

	desc := fmt.Sprintf("hooks.%s in %s", nativeEvent, settingsPath)
	return Placement{
		Mechanism: MechanismHookMerge,
		Path:      settingsPath,
		Keys:      []string{"hooks." + nativeEvent},
		Notices:   notices,
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

	canonEvent := converter.CanonicalHookEvent(h.Event, hookSourceProvider(item), prov.Slug)
	nativeEvent := nativeEventFor(canonEvent, prov.Slug)

	settingsPath, err := hookSettingsPath(prov)
	if err != nil {
		return Placement{}, err
	}

	inst, err := LoadInstalled(repoRoot)
	if err != nil {
		return Placement{}, fmt.Errorf("loading installed.json: %w", err)
	}
	instIdx := inst.FindHook(item.Name, nativeEvent, prov.Slug)
	if instIdx < 0 {
		if allowLegacyFallback {
			if legacyRoot := legacyRootWithHookRecord(repoRoot, item.Name, nativeEvent, prov.Slug); legacyRoot != "" {
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
		return Placement{}, fmt.Errorf("hook %s not found in %s; it was changed or removed after it was installed", item.Name, settingsPath)
	}

	remaining := make([]converter.CanonicalHook, 0, len(existing)-1)
	remaining = append(remaining, existing[:found]...)
	remaining = append(remaining, existing[found+1:]...)

	if model == hookStorageDirectory && len(remaining) == 0 {
		if err := os.Remove(settingsPath); err != nil && !os.IsNotExist(err) {
			return Placement{}, fmt.Errorf("removing %s: %w", settingsPath, err)
		}
	} else {
		encoded, err := adapter.Encode(&converter.CanonicalHooks{Spec: converter.SpecVersion, Hooks: remaining})
		if err != nil {
			return Placement{}, fmt.Errorf("encoding hooks: %w", err)
		}
		if err := writeHookFile(model, settingsPath, encoded.Content); err != nil {
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

// hookStatus is checkHookStatus with the error that left its status a
// guess, as StatusOf describes.
func hookStatus(item catalog.ContentItem, prov provider.Provider, repoRoot string) (Status, error) {
	status, err := hookStatusAtRoot(item, prov, repoRoot)
	if status != StatusNotInstalled {
		return status, err
	}
	if legacyRoot := legacyInstalledRoot(repoRoot); legacyRoot != "" {
		legacyStatus, legacyErr := hookStatusAtRoot(item, prov, legacyRoot)
		if legacyStatus == StatusInstalled {
			return StatusInstalled, nil
		}
		if err == nil {
			err = legacyErr
		}
	}
	return status, err
}

func checkHookStatus(item catalog.ContentItem, prov provider.Provider, repoRoot string) Status {
	status, _ := hookStatus(item, prov, repoRoot)
	return status
}

func hookStatusAtRoot(item catalog.ContentItem, prov provider.Provider, repoRoot string) (Status, error) {
	h, err := readSingleManifestHook(item.Path)
	if err != nil {
		return StatusNotAvailable, err
	}

	adapter := converter.AdapterFor(prov.Slug)
	if adapter == nil {
		return StatusNotAvailable, nil
	}
	model, err := hookStorageModelFor(prov.Slug)
	if err != nil {
		return StatusNotAvailable, nil
	}

	canonEvent := converter.CanonicalHookEvent(h.Event, hookSourceProvider(item), prov.Slug)
	nativeEvent := nativeEventFor(canonEvent, prov.Slug)

	inst, err := LoadInstalled(repoRoot)
	if err != nil {
		return StatusNotAvailable, err
	}
	instIdx := inst.FindHook(item.Name, nativeEvent, prov.Slug)
	if instIdx < 0 {
		return StatusNotInstalled, nil
	}
	storedHash := inst.Hooks[instIdx].GroupHash

	settingsPath, err := hookSettingsPath(prov)
	if err != nil {
		return StatusNotInstalled, nil
	}
	existing, err := decodeExistingHooks(model, adapter, settingsPath)
	if err != nil {
		return StatusNotInstalled, err
	}
	for _, eh := range existing {
		if hookIdentity(eh) == storedHash {
			return StatusInstalled, nil
		}
	}
	return StatusNotInstalled, nil
}

// hookTracked reports whether inst records hook name for event as installed
// on the provider with slug provSlug. An entry with no provider predates
// provider tracking and could belong to any provider, so it counts only when
// the provider's own hooks, existing, already hold a hook with its identity.
// Counting it everywhere would block a hook installed to one provider from
// ever being installed to another.
func hookTracked(inst *Installed, name, event, provSlug string, existing []converter.CanonicalHook) bool {
	idx := inst.FindHook(name, event, provSlug)
	if idx < 0 {
		return false
	}
	entry := inst.Hooks[idx]
	if entry.Provider != "" {
		return true
	}
	for _, eh := range existing {
		if hookIdentity(eh) == entry.GroupHash {
			return true
		}
	}
	return false
}

func hookTrackedAtLegacyRoot(repoRoot, name, event, provSlug string, existing []converter.CanonicalHook) bool {
	legacyRoot := legacyInstalledRoot(repoRoot)
	if legacyRoot == "" {
		return false
	}
	inst, err := LoadInstalled(legacyRoot)
	if err != nil {
		return false
	}
	return hookTracked(inst, name, event, provSlug, existing)
}

func legacyRootWithHookRecord(repoRoot, name, nativeEvent, provSlug string) string {
	legacyRoot := legacyInstalledRoot(repoRoot)
	if legacyRoot == "" {
		return ""
	}
	inst, err := LoadInstalled(legacyRoot)
	if err != nil {
		return ""
	}
	if inst.FindHook(name, nativeEvent, provSlug) < 0 {
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
// copied reports whether a script was copied, also when a later step failed.
func resolveHookCommandScript(cmd string, item catalog.ContentItem, repoRoot string) (resolved string, copied bool, err error) {
	if cmd == "" {
		return "", false, nil
	}
	mg, err := sjson.SetBytes([]byte(`{}`), "hooks.0.command", cmd)
	if err != nil {
		return "", false, err
	}
	mg, copied, err = resolveHookScripts(mg, item, repoRoot)
	if err != nil {
		return "", copied, err
	}
	return gjson.GetBytes(mg, "hooks.0.command").String(), copied, nil
}

// resolveHookScripts finds script file references in a hook's matcher group,
// copies them to a stable location (~/.syllago/hooks/<name>/), and rewrites
// the command paths in the JSON. This ensures hooks from registries don't
// break when the registry cache changes. The bool reports whether any script
// was copied, so the caller can warn that the hook runs bundled scripts.
func resolveHookScripts(matcherGroup []byte, item catalog.ContentItem, repoRoot string) ([]byte, bool, error) {
	// Resolve the item directory (hooks can be a file or directory)
	itemDir := item.Path
	fi, err := os.Stat(item.Path)
	if err == nil && !fi.IsDir() {
		itemDir = filepath.Dir(item.Path)
	}

	// Find all command fields in hooks array
	hooksArray := gjson.GetBytes(matcherGroup, "hooks")
	if !hooksArray.Exists() || !hooksArray.IsArray() {
		return matcherGroup, false, nil
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
				return nil, scriptsCopied, fmt.Errorf("hook %q command references path outside item directory: %s", item.Name, ref)
			}
		}

		if scriptPath == "" {
			continue // absolute path — not a bundled script
		}

		// Check if the script exists
		if _, statErr := os.Stat(scriptPath); statErr != nil {
			continue // script doesn't exist, leave command as-is
		}

		scriptsCopied = true

		// Copy script to stable location
		destDir, err := hookScriptsDir(item.Name)
		if err != nil {
			return nil, scriptsCopied, fmt.Errorf("getting hook scripts dir: %w", err)
		}
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return nil, scriptsCopied, fmt.Errorf("creating hook scripts dir: %w", err)
		}

		scriptName := filepath.Base(scriptPath)
		destPath := filepath.Join(destDir, scriptName)

		scriptData, readErr := os.ReadFile(scriptPath)
		if readErr != nil {
			return nil, scriptsCopied, fmt.Errorf("reading script %s: %w", scriptPath, readErr)
		}
		if writeErr := os.WriteFile(destPath, scriptData, 0700); writeErr != nil {
			return nil, scriptsCopied, fmt.Errorf("copying script to %s: %w", destPath, writeErr)
		}

		// Rewrite command: replace the script ref with the stable absolute path
		newCmd := strings.Replace(cmd, ref, destPath, 1)
		key := fmt.Sprintf("hooks.%d.command", i)
		result, err = sjson.SetBytes(result, key, newCmd)
		if err != nil {
			return nil, scriptsCopied, fmt.Errorf("rewriting command for %s: %w", item.Name, err)
		}
	}

	return result, scriptsCopied, nil
}
