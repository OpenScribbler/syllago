package installer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
func computeGroupHash(matcherGroup []byte) string {
	hash := sha256.Sum256(matcherGroup)
	return hex.EncodeToString(hash[:])
}

// hookSettingsPath returns the path to the provider's settings.json
func hookSettingsPath(prov provider.Provider) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, prov.ConfigDir, "settings.json"), nil
}

// parseHookFile reads a hook JSON file and extracts the event + the matcher group.
// The event field is stripped from the returned matcher group data.
// If path is a directory, resolves hook.json inside it.
func parseHookFile(path string) (event string, matcherGroup []byte, err error) {
	fi, err := os.Stat(path)
	if err != nil {
		return "", nil, err
	}
	if fi.IsDir() {
		path = filepath.Join(path, "hook.json")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}

	// Extract event field
	event = gjson.GetBytes(data, "event").String()
	if event == "" {
		return "", nil, fmt.Errorf("hook file missing 'event' field")
	}

	// Remove event field to get the matcher group
	matcherGroup, err = sjson.DeleteBytes(data, "event")
	if err != nil {
		return "", nil, fmt.Errorf("stripping event field: %w", err)
	}

	return event, matcherGroup, nil
}

func installHook(item catalog.ContentItem, prov provider.Provider, repoRoot string) (string, error) {
	// item.Path is already absolute (set by scanner)
	event, matcherGroup, err := parseHookFile(item.Path)
	if err != nil {
		return "", fmt.Errorf("parsing hook file: %w", err)
	}

	// M3: Validate event name to prevent sjson key injection via dots
	if !converter.IsValidHookEvent(event) {
		return "", fmt.Errorf("unknown hook event %q: must be a known canonical or provider event name", event)
	}

	// M4: Whitelist-filter the matcher group through a typed struct to strip
	// unknown JSON fields before merging into settings.json.
	matcherGroup, err = whitelistMatcherGroup(matcherGroup)
	if err != nil {
		return "", fmt.Errorf("filtering matcher group: %w", err)
	}

	// M2: Run security scanner on the hook before installing
	warnings := converter.ScanHookSecurityFromRaw(matcherGroup)
	for _, w := range warnings {
		fmt.Fprintf(output.ErrWriter, "  %s WARNING [%s]: %s\n    command: %s\n",
			strings.ToUpper(w.Severity), w.HookName, w.Description, w.Command)
	}

	// Copy script files referenced by hook commands to a stable location.
	// Without this, hooks from registries would point into the registry
	// clone dir which can change on sync or vanish on remove.
	matcherGroup, err = resolveHookScripts(matcherGroup, item, repoRoot)
	if err != nil {
		return "", err
	}

	// Check installed.json for duplicate
	inst, err := LoadInstalled(repoRoot)
	if err != nil {
		return "", fmt.Errorf("loading installed.json: %w", err)
	}
	if inst.FindHook(item.Name, event) >= 0 {
		return "", fmt.Errorf("hook %s already installed for %s event", item.Name, event)
	}

	settingsPath, err := hookSettingsPath(prov)
	if err != nil {
		return "", err
	}

	snapshotDir, err := snapshot.CreateForHook(repoRoot, "hook-install:"+item.Name, []string{settingsPath})
	if err != nil {
		return "", fmt.Errorf("creating snapshot: %w", err)
	}

	fileData, err := readJSONFile(settingsPath)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", settingsPath, err)
	}

	// Compute hash of the matcher group before appending
	hash := sha256.Sum256(matcherGroup)
	groupHash := hex.EncodeToString(hash[:])

	// Append to hooks.<event> array using sjson's -1 (append) syntax
	key := "hooks." + event + ".-1"
	fileData, err = sjson.SetRawBytes(fileData, key, matcherGroup)
	if err != nil {
		return "", fmt.Errorf("appending hook: %w", err)
	}

	if err := writeJSONFile(settingsPath, fileData); err != nil {
		// Auto-rollback using the snapshot we just created
		if manifest, _, loadErr := snapshot.Load(repoRoot); loadErr == nil {
			_ = snapshot.Restore(snapshotDir, manifest)
		}
		return "", fmt.Errorf("writing %s: %w", settingsPath, err)
	}

	// Extract command from the hook for tracking
	command := gjson.GetBytes(matcherGroup, "hooks.0.command").String()

	// Record in installed.json
	inst.Hooks = append(inst.Hooks, InstalledHook{
		Name:        item.Name,
		Event:       event,
		GroupHash:   groupHash,
		Command:     command,
		Source:      "export",
		Scope:       "global",
		InstalledAt: time.Now(),
	})
	if err := SaveInstalled(repoRoot, inst); err != nil {
		return "", fmt.Errorf("saving installed.json: %w", err)
	}

	return fmt.Sprintf("hooks.%s in %s", event, settingsPath), nil
}

func uninstallHook(item catalog.ContentItem, prov provider.Provider, repoRoot string) (string, error) {
	// item.Path is already absolute (set by scanner)
	event, _, err := parseHookFile(item.Path)
	if err != nil {
		return "", fmt.Errorf("parsing hook file: %w", err)
	}

	settingsPath, err := hookSettingsPath(prov)
	if err != nil {
		return "", err
	}

	fileData, err := readJSONFile(settingsPath)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", settingsPath, err)
	}

	// Find entry by installed.json lookup
	inst, err := LoadInstalled(repoRoot)
	if err != nil {
		return "", fmt.Errorf("loading installed.json: %w", err)
	}

	instIdx := inst.FindHook(item.Name, event)

	// Find the hook entry in settings.json
	hooksArray := gjson.GetBytes(fileData, "hooks."+event)
	if !hooksArray.Exists() || !hooksArray.IsArray() {
		return "", fmt.Errorf("no hooks.%s array in %s", event, settingsPath)
	}

	found := -1
	if instIdx >= 0 {
		storedHash := inst.Hooks[instIdx].GroupHash
		if storedHash != "" {
			// Hash-based matching: compare stored hash against hash of each entry
			for i, entry := range hooksArray.Array() {
				entryBytes := []byte(entry.Raw)
				h := sha256.Sum256(entryBytes)
				if hex.EncodeToString(h[:]) == storedHash {
					found = i
					break
				}
			}
			if found == -1 {
				return "", fmt.Errorf("hook %s was modified since installation; use 'syllago restore' to revert", item.Name)
			}
		} else {
			// Fallback: command-string matching for pre-hash installed hooks
			cmd := inst.Hooks[instIdx].Command
			for i, entry := range hooksArray.Array() {
				if entry.Get("hooks.0.command").String() == cmd {
					found = i
					break
				}
			}
		}
	}
	if found == -1 {
		return "", fmt.Errorf("hook %s not found in hooks.%s (not installed by syllago)", item.Name, event)
	}

	snapshotDir, err := snapshot.CreateForHook(repoRoot, "hook-uninstall:"+item.Name, []string{settingsPath})
	if err != nil {
		return "", fmt.Errorf("creating snapshot: %w", err)
	}

	// Delete by index
	key := fmt.Sprintf("hooks.%s.%d", event, found)
	fileData, err = sjson.DeleteBytes(fileData, key)
	if err != nil {
		return "", fmt.Errorf("deleting hook: %w", err)
	}

	if err := writeJSONFile(settingsPath, fileData); err != nil {
		// Auto-rollback using the snapshot we just created
		if manifest, _, loadErr := snapshot.Load(repoRoot); loadErr == nil {
			_ = snapshot.Restore(snapshotDir, manifest)
		}
		return "", fmt.Errorf("writing %s: %w", settingsPath, err)
	}

	// Remove from installed.json only after successful write
	if instIdx >= 0 {
		inst.RemoveHook(instIdx)
		if err := SaveInstalled(repoRoot, inst); err != nil {
			return "", fmt.Errorf("saving installed.json: %w", err)
		}
	}

	return fmt.Sprintf("hooks.%s from %s", event, settingsPath), nil
}

func checkHookStatus(item catalog.ContentItem, prov provider.Provider, repoRoot string) Status {
	// item.Path is already absolute (set by scanner)
	event, _, err := parseHookFile(item.Path)
	if err != nil {
		return StatusNotAvailable
	}

	// Check installed.json first
	inst, err := LoadInstalled(repoRoot)
	if err != nil {
		return StatusNotAvailable
	}
	if inst.FindHook(item.Name, event) >= 0 {
		return StatusInstalled
	}

	// Also check if event array exists in settings.json (installed by other means)
	settingsPath, err := hookSettingsPath(prov)
	if err != nil {
		return StatusNotAvailable
	}

	fileData, err := readJSONFile(settingsPath)
	if err != nil {
		return StatusNotAvailable
	}

	hooksArray := gjson.GetBytes(fileData, "hooks."+event)
	if !hooksArray.Exists() || !hooksArray.IsArray() {
		return StatusNotInstalled
	}

	return StatusNotInstalled
}

// hookScriptsDir returns ~/.syllago/hooks/<name>/ for storing copied scripts.
func hookScriptsDir(name string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".syllago", "hooks", name), nil
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

// hookMatcherGroup is a typed struct for whitelist-filtering hook matcher groups.
// Only known fields are preserved when merging into settings.json, matching
// the approach used by ExtractServerEntries for MCP configs.
type hookMatcherGroup struct {
	Matcher string          `json:"matcher,omitempty"`
	Hooks   json.RawMessage `json:"hooks,omitempty"`
	Timeout int             `json:"timeout,omitempty"`
}

// whitelistMatcherGroup parses a raw matcher group through a typed struct to
// strip unknown JSON fields before merging into provider settings.
func whitelistMatcherGroup(raw []byte) ([]byte, error) {
	var mg hookMatcherGroup
	if err := json.Unmarshal(raw, &mg); err != nil {
		return nil, fmt.Errorf("parsing matcher group: %w", err)
	}
	return json.Marshal(mg)
}
