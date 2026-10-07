package installer

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

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
	m, err := readSingleManifest(item.Path)
	if err == nil {
		err = converter.CheckInstallSpec(m)
	}
	if err != nil {
		return Placement{}, fmt.Errorf("parsing hook file: %w", err)
	}
	h := m.Hooks[0]
	settingsPath, err := hookSettingsPath(prov)
	if err != nil {
		return Placement{}, err
	}
	scriptsDir, err := hookScriptsDir(prov.Slug, item.Name)
	if err != nil {
		return Placement{}, fmt.Errorf("getting hook scripts dir: %w", err)
	}
	inst, err := LoadInstalled(repoRoot)
	if err != nil {
		return Placement{}, fmt.Errorf("loading installed.json: %w", err)
	}
	placement, err := PlaceHook(item, h, prov, repoRoot, settingsPath, scriptsDir, inst, "export", scan)
	if err != nil {
		return placement, err
	}
	if err := SaveInstalled(repoRoot, inst); err != nil {
		return Placement{Notices: placement.Notices}, fmt.Errorf("saving installed.json: %w", err)
	}
	return placement, nil
}

// PlaceHook merges hook h, read from item, into the provider's hook file at
// settingsPath and appends its record to inst under source. It refuses an
// item with high-severity scanner findings unless scan.Force, and copies
// the scripts the hook runs into scriptsDir, so the scanned copy is the one
// that runs. The caller loads and saves inst.
func PlaceHook(item catalog.ContentItem, h converter.Hook, prov provider.Provider, repoRoot, settingsPath, scriptsDir string, inst *Installed, source string, scan ScanOptions) (Placement, error) {
	adapter, model, canonHook, err := checkHook(item, h, prov)
	if err != nil {
		return Placement{}, err
	}
	canonEvent := canonHook.Event

	// SECURITY (M2): run the pluggable scanner chain against the source hook
	// directory. High-severity findings block the install unless --force.
	scanDir, cleanup, err := hookScanDir(item, h.Handler.Command)
	if err != nil {
		return Placement{}, err
	}
	scanResult, _ := converter.RunScanChain(scanDir, scan.Scanners)
	cleanup()
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
		var high []string
		for _, f := range scanResult.Findings {
			if strings.EqualFold(f.Severity, "high") {
				high = append(high, fmt.Sprintf("%s in %s", f.Description, f.File))
			}
		}
		return Placement{Notices: notices}, fmt.Errorf("hook %q has high-severity security findings (%s); re-run with --force to install anyway", item.Name, strings.Join(high, ", "))
	}

	nativeEvent := nativeEventFor(canonEvent, prov.Slug)

	existing, err := decodeExistingHooks(model, adapter, settingsPath)
	if err != nil {
		return Placement{Notices: notices}, err
	}

	// Dedup against installed.json (name + event + provider), before the
	// script copy below can overwrite the installed hook's scripts.
	if err := hookInstalled(inst, repoRoot, item.Name, nativeEvent, prov.Slug, existing); err != nil {
		return Placement{Notices: notices}, err
	}

	// SECURITY: copy referenced scripts to a stable location and rewrite the
	// command path (operates on the hook before encode).
	resolvedCmd, copied, err := resolveHookCommandScript(canonHook.Handler.Command, item, scriptsDir)
	if copied {
		notices = append(notices, Notice{
			Kind:    NoticeScriptSecurity,
			Message: fmt.Sprintf("Hook %q references executable script files.\nScripts will be copied to %s/", item.Name, homeRelative(scriptsDir)),
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
		Source:      source,
		Scope:       "global",
		Provider:    prov.Slug,
		InstalledAt: time.Now(),
	})
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
	m, err := readSingleManifest(item.Path)
	if err != nil {
		return Placement{}, fmt.Errorf("parsing hook file: %w", err)
	}
	h := m.Hooks[0]

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
	m, err := readSingleManifest(item.Path)
	if err != nil {
		return StatusNotAvailable, err
	}
	h := m.Hooks[0]

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

// ErrHookInstalled is wrapped by the error for a hook PlaceHook refuses
// because installed.json, in the current or the legacy root, records it.
var ErrHookInstalled = errors.New("already installed")

// hookInstalled returns an error wrapping ErrHookInstalled when the hook
// named name is installed for event on provSlug.
func hookInstalled(inst *Installed, repoRoot, name, event, provSlug string, existing []converter.CanonicalHook) error {
	if hookTracked(inst, name, event, provSlug, existing) || hookTrackedAtLegacyRoot(repoRoot, name, event, provSlug, existing) {
		return fmt.Errorf("hook %s %w for %s event", name, ErrHookInstalled, event)
	}
	return nil
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

// hookScriptsDir returns ~/.syllago/hooks/<provider>/<name>/ for storing
// copied scripts. The whole item is copied there, so a hook of the same
// name installed for another provider gets its own directory rather than
// having its scripts overwritten.
func hookScriptsDir(provSlug, name string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".syllago", "hooks", provSlug, name), nil
}

// homeRelative shows a path under the home directory as ~/<rest>.
func homeRelative(p string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if rel, err := filepath.Rel(home, p); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.Join("~", rel)
	}
	return p
}

// resolveHookCommandScript resolves a single hook command through the
// script-copying security logic (resolveHookScripts operates on a matcher
// group, so we wrap the command in a minimal group and read the rewritten
// command back). An empty command (e.g. a non-command handler) is a no-op.
// copied reports whether a script was copied, also when a later step failed.
func resolveHookCommandScript(cmd string, item catalog.ContentItem, destDir string) (resolved string, copied bool, err error) {
	if cmd == "" {
		return "", false, nil
	}
	mg, err := sjson.SetBytes([]byte(`{}`), "hooks.0.command", cmd)
	if err != nil {
		return "", false, err
	}
	mg, copied, err = resolveHookScripts(mg, item, destDir)
	if err != nil {
		return "", copied, err
	}
	return gjson.GetBytes(mg, "hooks.0.command").String(), copied, nil
}

// resolveHookScripts finds script file references in a hook's matcher group,
// copies them to destDir, and rewrites
// the command paths in the JSON. This ensures hooks from registries don't
// break when the registry cache changes. The bool reports whether any script
// was copied, so the caller can warn that the hook runs bundled scripts.
func resolveHookScripts(matcherGroup []byte, item catalog.ContentItem, destDir string) ([]byte, bool, error) {
	itemDir := hookItemDir(item)
	singleFile := isSingleFileHook(item)

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

		ref, scriptPath, rel, err := hookScriptRef(itemDir, item.Name, cmd)
		if err != nil {
			return nil, scriptsCopied, err
		}
		if scriptPath == "" {
			continue // inline command, or an absolute path — not a bundled script
		}

		// Check if the script exists
		scriptInfo, statErr := os.Stat(scriptPath)
		if statErr != nil {
			continue // script doesn't exist, leave command as-is
		}

		// The script runs beside the rest of its item, which it may source
		// or require, so the whole item is copied once, keeping its layout.
		// A single-file hook's directory holds its provider's other hooks,
		// so only the script is copied.
		destPath := filepath.Join(destDir, rel)
		if singleFile {
			if err := copyHookFile(scriptPath, destPath, scriptInfo); err != nil {
				return nil, true, fmt.Errorf("copying hook script to %s: %w", destPath, err)
			}
		} else if !scriptsCopied {
			if err := copyHookItem(itemDir, destDir); err != nil {
				return nil, true, fmt.Errorf("copying hook scripts to %s: %w", destDir, err)
			}
		}
		scriptsCopied = true

		// Run the script by the name the command gave it, so a script
		// reached through an in-item symlink finds the files beside that
		// name. The resolved path stands in when that name was not copied:
		// one through a directory symlink, or one that leaves the item and
		// comes back. The test reads the item, not destDir, which can hold
		// files an earlier install left.
		if entry, err := filepath.Rel(itemDir, filepath.Join(itemDir, ref)); err == nil && !singleFile && entry != ".." && !strings.HasPrefix(entry, ".."+string(filepath.Separator)) {
			parent := filepath.Join(itemDir, filepath.Dir(entry))
			if resolved, err := filepath.EvalSymlinks(parent); err == nil && resolved == parent {
				destPath = filepath.Join(destDir, entry)
			}
		}

		if err := os.Chmod(destPath, 0700); err != nil {
			return nil, scriptsCopied, fmt.Errorf("making %s executable: %w", destPath, err)
		}

		// Rewrite command: replace the script ref, with any quotes around
		// it, by the stable absolute path. Quoting inside the command's own
		// quotes would leave the path's quotes literal.
		start, end := scriptRefSpan(cmd, ref)
		if start < 0 {
			continue
		}
		newCmd := cmd[:start] + shellQuote(destPath) + cmd[end:]
		key := fmt.Sprintf("hooks.%d.command", i)
		result, err = sjson.SetBytes(result, key, newCmd)
		if err != nil {
			return nil, scriptsCopied, fmt.Errorf("rewriting command for %s: %w", item.Name, err)
		}
	}

	return result, scriptsCopied, nil
}

// CheckHook returns why PlaceHook would refuse item's hook h for prov
// before it changes anything: a name that is not one path element, an
// event the provider cannot take, a hook its adapter cannot represent, a
// command naming a script outside the item, or scripts the copy cannot
// read. Scanner findings are reported
// apart, because --force overrides them. A non-empty settingsPath is
// decoded too, so a provider config the merge cannot read refuses here,
// and a hook inst or the legacy root under repoRoot records as installed
// returns an error wrapping ErrHookInstalled.
func CheckHook(item catalog.ContentItem, h converter.Hook, prov provider.Provider, repoRoot, settingsPath string, inst *Installed) error {
	adapter, model, canonHook, err := checkHook(item, h, prov)
	if err != nil {
		return err
	}
	// An installed hook is skipped before its scripts are copied, as
	// PlaceHook does, so a script it cannot read does not refuse the apply.
	if settingsPath != "" {
		existing, err := decodeExistingHooks(model, adapter, settingsPath)
		if err != nil {
			return err
		}
		if err := hookInstalled(inst, repoRoot, item.Name, nativeEventFor(canonHook.Event, prov.Slug), prov.Slug, existing); err != nil {
			return err
		}
	}
	// A script copy that fails, such as on a file it cannot read, would
	// stop the apply after an earlier provider had changed, so the copy is
	// tried here into a directory that is then removed.
	trial, err := os.MkdirTemp("", "syllago-hook-check-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(trial) }()
	if _, _, err := resolveHookCommandScript(h.Handler.Command, item, trial); err != nil {
		if inner := errors.Unwrap(err); inner != nil {
			err = inner // the trial directory's path would only mislead
		}
		return fmt.Errorf("hook %q: copying its scripts: %w", item.Name, err)
	}
	return nil
}

// checkHook runs CheckHook's checks and returns what PlaceHook builds on:
// the provider's adapter and storage model, and h as a canonical hook with
// the provider's event.
func checkHook(item catalog.ContentItem, h converter.Hook, prov provider.Provider) (converter.HookAdapter, hookStorageModel, converter.CanonicalHook, error) {
	var none converter.CanonicalHook
	// The name names the scripts directory, and a registry index can give
	// an item any name, so one that is not a single path element is refused.
	if item.Name == "" || item.Name == "." || item.Name == ".." || item.Name != filepath.Base(item.Name) {
		return nil, 0, none, fmt.Errorf("hook name %q is not a valid directory name", item.Name)
	}
	// M3: validate the event name (rejects garbage and prevents key injection).
	if !converter.IsValidHookEvent(h.Event) {
		return nil, 0, none, fmt.Errorf("unknown hook event %q: must be a known canonical or provider event name", h.Event)
	}

	// Install through the provider's HookAdapter. No adapter (amp,
	// codex) means the hook cannot be serialized — reject rather than write
	// config the provider never reads.
	adapter := converter.AdapterFor(prov.Slug)
	if adapter == nil {
		return nil, 0, none, fmt.Errorf("hook install not supported for %s (no encoder)", prov.Name)
	}

	model, err := hookStorageModelFor(prov.Slug)
	if err != nil {
		return nil, 0, none, err
	}

	canonHook, err := converter.CanonicalHookFromManifest(h)
	if err != nil {
		return nil, 0, none, fmt.Errorf("building canonical hook: %w", err)
	}
	canonEvent := converter.CanonicalHookEvent(h.Event, hookSourceProvider(item), prov.Slug)
	canonHook.Event = canonEvent

	// Event-support gate: reject events the adapter cannot represent. Adapter
	// capabilities are the source of truth.
	if !adapterSupportsEvent(adapter, canonEvent) {
		return nil, 0, none, fmt.Errorf("hook %q: %s does not support hook event %q", item.Name, prov.Name, h.Event)
	}
	if _, _, _, err := hookScriptRef(hookItemDir(item), item.Name, h.Handler.Command); err != nil {
		return nil, 0, none, err
	}
	// A hook the adapter drops (e.g. a non-command handler on crush) is
	// refused here; PlaceHook repeats the round trip on the rewritten
	// command for the hook's identity.
	if _, _, err := roundTripIdentity(adapter, canonHook); err != nil {
		return nil, 0, none, fmt.Errorf("hook %q: %w", item.Name, err)
	}
	return adapter, model, canonHook, nil
}

// isSingleFileHook reports whether item is a hook manifest file rather
// than a hook directory.
func isSingleFileHook(item catalog.ContentItem) bool {
	fi, err := os.Stat(item.Path)
	return err == nil && !fi.IsDir()
}

// hookScanDir returns the directory the scanners read for item's hook
// command cmd, and a function that removes it when it was made for the
// scan. A hook directory is scanned whole, because the whole of it is
// copied. A single-file hook shares its directory with every other hook of
// its provider, so its manifest and the script cmd runs are staged in a
// temporary directory, which keeps the other hooks out of its scan.
func hookScanDir(item catalog.ContentItem, cmd string) (string, func(), error) {
	itemDir := hookItemDir(item)
	if !isSingleFileHook(item) {
		return itemDir, func() {}, nil
	}
	ref, scriptPath, rel, err := hookScriptRef(itemDir, item.Name, cmd)
	if err != nil {
		return "", nil, err
	}
	// The scanner picks a script's language by its file name, so a script
	// reached through a symlink is staged under the name the command gave
	// it as well as under its target's, as a directory scan would see it.
	var scripts []string
	if scriptPath != "" {
		if _, statErr := os.Stat(scriptPath); statErr == nil {
			scripts = append(scripts, rel)
			if entry, err := filepath.Rel(itemDir, filepath.Join(itemDir, ref)); err == nil && entry != rel && entry != ".." && !strings.HasPrefix(entry, ".."+string(filepath.Separator)) {
				scripts = append(scripts, entry)
			}
		}
	}
	// The scanner reads hook config only from .json files, so the manifest
	// is staged under a .json name whatever the item calls it, and under
	// one no staged script path starts with.
	taken := make(map[string]bool)
	for _, p := range scripts {
		taken[strings.SplitN(filepath.ToSlash(p), "/", 2)[0]] = true
	}
	manifest := "hook.json"
	for i := 1; taken[manifest]; i++ {
		manifest = fmt.Sprintf("hook-%d.json", i)
	}
	stage, err := os.MkdirTemp("", "syllago-hook-scan-")
	if err != nil {
		return "", nil, fmt.Errorf("staging hook %q for its scan: %w", item.Name, err)
	}
	cleanup := func() { _ = os.RemoveAll(stage) }
	if err := copyFile(item.Path, filepath.Join(stage, manifest)); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("staging hook %q for its scan: %w", item.Name, err)
	}
	for _, p := range scripts {
		if err := copyFile(scriptPath, filepath.Join(stage, p)); err != nil {
			cleanup()
			return "", nil, fmt.Errorf("staging hook %q for its scan: %w", item.Name, err)
		}
	}
	return stage, cleanup, nil
}

// hookItemDir returns the directory holding a hook item, which can be a
// file or a directory, with its symlinks resolved so a content root reached
// through a symlink does not make every script look outside it.
func hookItemDir(item catalog.ContentItem) string {
	itemDir := item.Path
	if fi, err := os.Stat(item.Path); err == nil && !fi.IsDir() {
		itemDir = filepath.Dir(item.Path)
	}
	if resolved, err := filepath.EvalSymlinks(itemDir); err == nil {
		itemDir = resolved
	}
	return itemDir
}

// hookScriptRef finds the bundled script a hook command runs: its reference
// in cmd, its path with symlinks resolved, and that path relative to
// itemDir. Only a relative reference names a bundled script, so scriptPath
// is "" for an inline command or an absolute path. A reference that
// resolves outside itemDir is an error.
func hookScriptRef(itemDir, name, cmd string) (ref, scriptPath, rel string, err error) {
	// ExtractScriptRef detects script references, including those behind
	// interpreter prefixes (e.g. "bash ./lint.sh").
	ref = converter.ExtractScriptRef(cmd)
	if !strings.HasPrefix(ref, "./") && !strings.HasPrefix(ref, "../") {
		return ref, "", "", nil
	}
	scriptPath = filepath.Clean(filepath.Join(itemDir, ref))
	// Resolve symlinks before the containment check to prevent symlink-based
	// path traversal (e.g., ./scripts -> /etc via a crafted symlink).
	if resolved, evalErr := filepath.EvalSymlinks(scriptPath); evalErr == nil {
		scriptPath = resolved
	}
	rel, relErr := filepath.Rel(itemDir, scriptPath)
	if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ref, "", "", fmt.Errorf("hook %q command references path outside item directory: %s", name, ref)
	}
	return ref, scriptPath, rel, nil
}

// copyHookItem copies the regular files of a hook item into destDir with
// their permissions. The scan followed symlinks and a script may load a
// file through one, so a symlink to a file inside the item is recreated as
// a symlink to that file's copy. A copied file would move the script: node
// and python resolve a script's own symlink to find the files it loads,
// while $0 keeps the name it ran by. Where no symlink can be made, the file
// is copied in its place. One that leaves the item is skipped.
func copyHookItem(itemDir, destDir string) error {
	return filepath.WalkDir(itemDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(itemDir, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(destDir, rel)
		// An earlier install can leave a file where this version has a
		// directory, or the reverse, which would block the copy.
		if fi, err := os.Lstat(dest); err == nil && fi.IsDir() != d.IsDir() {
			if err := os.RemoveAll(dest); err != nil {
				return err
			}
		}
		switch {
		case d.IsDir():
			return os.MkdirAll(dest, 0755)
		case d.Type()&fs.ModeSymlink != 0:
			target, err := filepath.EvalSymlinks(path)
			if err != nil {
				return nil
			}
			r, err := filepath.Rel(itemDir, target)
			if err != nil || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
				return nil
			}
			info, err := os.Stat(target)
			if err != nil || !info.Mode().IsRegular() {
				return nil
			}
			link, err := filepath.Rel(filepath.Dir(rel), r)
			if err != nil {
				return err
			}
			if createHookSymlink(link, dest) == nil {
				return nil
			}
			return copyHookFile(target, dest, info)
		case d.Type().IsRegular():
			info, err := d.Info()
			if err != nil {
				return err
			}
			return copyHookFile(path, dest, info)
		}
		return nil
	})
}

// createHookSymlink is replaced in tests to fail as it does where no
// symlink can be made.
var createHookSymlink = CreateSymlink

// copyHookFile copies src to dest with info's permissions. The copy will
// not write through a symlink, and one at dest is a link an earlier install
// made where the item now has a file, so the link itself is removed first.
func copyHookFile(src, dest string, info fs.FileInfo) error {
	if fi, err := os.Lstat(dest); err == nil && fi.Mode()&fs.ModeSymlink != 0 {
		if err := os.Remove(dest); err != nil {
			return err
		}
	}
	if err := copyFile(src, dest); err != nil {
		return err
	}
	return os.Chmod(dest, info.Mode().Perm())
}

// scriptRefSpan returns the byte span of the first whitespace-separated
// field of cmd that is ref, bare or in one pair of matching quotes, or -1,
// -1. That is the field ExtractScriptRef read ref from: the fields before it
// are an interpreter, its flags, and a subcommand, none of which is a
// relative path, and a later one is an argument.
func scriptRefSpan(cmd, ref string) (int, int) {
	for start := 0; start < len(cmd); {
		r, size := utf8.DecodeRuneInString(cmd[start:])
		if unicode.IsSpace(r) {
			start += size
			continue
		}
		end := start
		for end < len(cmd) {
			r, size := utf8.DecodeRuneInString(cmd[end:])
			if unicode.IsSpace(r) {
				break
			}
			end += size
		}
		f := cmd[start:end]
		if f == ref || len(f) == len(ref)+2 && (f[0] == '"' || f[0] == '\'') && f[len(f)-1] == f[0] && f[1:len(f)-1] == ref {
			return start, end
		}
		start = end
	}
	return -1, -1
}

// shellQuote quotes p so the shell running the hook reads it as one word.
func shellQuote(p string) string {
	return shellQuoteFor(runtime.GOOS, p)
}

// shellQuoteFor single-quotes p for a POSIX shell when it holds anything but
// characters a shell reads literally. On Windows a hook may run under
// cmd.exe, which reads single quotes literally, so the path stays bare as it
// always has and takes double quotes only to keep a space inside one word
// or a cmd.exe metacharacter out of the command line.
func shellQuoteFor(goos, p string) string {
	if goos == "windows" {
		if strings.ContainsAny(p, " &|<>^()") {
			return `"` + p + `"`
		}
		return p
	}
	safe := func(r rune) bool {
		return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("/._-+:@%,=", r)
	}
	if strings.IndexFunc(p, func(r rune) bool { return !safe(r) }) < 0 {
		return p
	}
	return "'" + strings.ReplaceAll(p, "'", `'\''`) + "'"
}
