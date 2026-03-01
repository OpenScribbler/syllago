package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

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
func parseHookFile(path string) (event string, matcherGroup []byte, err error) {
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

	if err := backupFile(settingsPath); err != nil {
		return "", fmt.Errorf("backing up %s: %w", settingsPath, err)
	}

	fileData, err := readJSONFile(settingsPath)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", settingsPath, err)
	}

	// Append to hooks.<event> array using sjson's -1 (append) syntax
	key := "hooks." + event + ".-1"
	fileData, err = sjson.SetRawBytes(fileData, key, matcherGroup)
	if err != nil {
		return "", fmt.Errorf("appending hook: %w", err)
	}

	if err := writeJSONFile(settingsPath, fileData); err != nil {
		return "", fmt.Errorf("writing %s: %w", settingsPath, err)
	}

	// Extract command from the hook for tracking
	command := gjson.GetBytes(matcherGroup, "hooks.0.command").String()

	// Record in installed.json
	inst.Hooks = append(inst.Hooks, InstalledHook{
		Name:        item.Name,
		Event:       event,
		Command:     command,
		Source:      "export",
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

	// Find the hook entry in settings.json by matching the command string
	hooksArray := gjson.GetBytes(fileData, "hooks."+event)
	if !hooksArray.Exists() || !hooksArray.IsArray() {
		return "", fmt.Errorf("no hooks.%s array in %s", event, settingsPath)
	}

	found := -1
	if instIdx >= 0 {
		// Match by command string from installed.json
		cmd := inst.Hooks[instIdx].Command
		for i, entry := range hooksArray.Array() {
			if entry.Get("hooks.0.command").String() == cmd {
				found = i
				break
			}
		}
	}
	if found == -1 {
		return "", fmt.Errorf("hook %s not found in hooks.%s (not installed by syllago)", item.Name, event)
	}

	if err := backupFile(settingsPath); err != nil {
		return "", fmt.Errorf("backing up %s: %w", settingsPath, err)
	}

	// Delete by index
	key := fmt.Sprintf("hooks.%s.%d", event, found)
	fileData, err = sjson.DeleteBytes(fileData, key)
	if err != nil {
		return "", fmt.Errorf("deleting hook: %w", err)
	}

	if err := writeJSONFile(settingsPath, fileData); err != nil {
		return "", fmt.Errorf("writing %s: %w", settingsPath, err)
	}

	// Remove from installed.json
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
