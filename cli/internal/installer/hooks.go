package installer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
	"github.com/holdenhewett/romanesco/cli/internal/provider"
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

func installHook(item catalog.ContentItem, prov provider.Provider, _ string) (string, error) {
	// item.Path is already absolute (set by scanner)
	event, matcherGroup, err := parseHookFile(item.Path)
	if err != nil {
		return "", fmt.Errorf("parsing hook file: %w", err)
	}

	// Add _romanesco marker with item name for identification
	matcherGroup, err = sjson.SetBytes(matcherGroup, "_romanesco", item.Name)
	if err != nil {
		return "", fmt.Errorf("adding marker: %w", err)
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

	// Check if this hook is already installed (by _romanesco marker)
	hooksArray := gjson.GetBytes(fileData, "hooks."+event)
	if hooksArray.Exists() && hooksArray.IsArray() {
		for _, entry := range hooksArray.Array() {
			if entry.Get("_romanesco").String() == item.Name {
				return "", fmt.Errorf("hook %s already installed for %s event", item.Name, event)
			}
		}
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

	return fmt.Sprintf("hooks.%s in %s", event, settingsPath), nil
}

func uninstallHook(item catalog.ContentItem, prov provider.Provider, _ string) (string, error) {
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

	// Find and remove the entry with matching _romanesco marker
	hooksArray := gjson.GetBytes(fileData, "hooks."+event)
	if !hooksArray.Exists() || !hooksArray.IsArray() {
		return "", fmt.Errorf("no hooks.%s array in %s", event, settingsPath)
	}

	found := -1
	for i, entry := range hooksArray.Array() {
		if entry.Get("_romanesco").String() == item.Name {
			found = i
			break
		}
	}
	if found == -1 {
		return "", fmt.Errorf("hook %s not found in hooks.%s (not installed by nesco)", item.Name, event)
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

	return fmt.Sprintf("hooks.%s from %s", event, settingsPath), nil
}

func checkHookStatus(item catalog.ContentItem, prov provider.Provider, _ string) Status {
	// item.Path is already absolute (set by scanner)
	event, _, err := parseHookFile(item.Path)
	if err != nil {
		return StatusNotAvailable
	}

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

	for _, entry := range hooksArray.Array() {
		if entry.Get("_romanesco").String() == item.Name {
			return StatusInstalled
		}
	}

	return StatusNotInstalled
}
