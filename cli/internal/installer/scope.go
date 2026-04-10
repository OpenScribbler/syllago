package installer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// SettingsScope represents where a settings.json lives.
type SettingsScope int

const (
	ScopeGlobal SettingsScope = iota
	ScopeProject
)

func (s SettingsScope) String() string {
	if s == ScopeGlobal {
		return "global"
	}
	return "project"
}

// SettingsLocation describes one discovered settings.json file.
type SettingsLocation struct {
	Scope SettingsScope
	Path  string
}

// FindSettingsLocations returns all settings.json paths for the given provider
// that exist on disk, checking global and project-local scopes.
// projectRoot is the nearest git root (or cwd if not in a git repo).
func FindSettingsLocations(prov provider.Provider, projectRoot string) ([]SettingsLocation, error) {
	var locations []SettingsLocation

	// Global scope: ~/.configDir/settings.json
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	globalPath := filepath.Join(home, prov.ConfigDir, "settings.json")
	if _, err := os.Stat(globalPath); err == nil {
		locations = append(locations, SettingsLocation{Scope: ScopeGlobal, Path: globalPath})
	}

	// Project scope: <projectRoot>/.configDir/settings.json (if projectRoot != home)
	if projectRoot != "" && projectRoot != home {
		projectPath := filepath.Join(projectRoot, prov.ConfigDir, "settings.json")
		if _, err := os.Stat(projectPath); err == nil {
			locations = append(locations, SettingsLocation{Scope: ScopeProject, Path: projectPath})
		}
	}

	return locations, nil
}

// FindSettingsLocationsWithBase works like FindSettingsLocations but uses
// baseDir instead of the user's home directory for the global scope path.
func FindSettingsLocationsWithBase(prov provider.Provider, projectRoot, baseDir string) ([]SettingsLocation, error) {
	if baseDir == "" {
		return FindSettingsLocations(prov, projectRoot)
	}

	var locations []SettingsLocation

	globalPath := filepath.Join(baseDir, prov.ConfigDir, "settings.json")
	if _, err := os.Stat(globalPath); err == nil {
		locations = append(locations, SettingsLocation{Scope: ScopeGlobal, Path: globalPath})
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	if projectRoot != "" && projectRoot != home {
		projectPath := filepath.Join(projectRoot, prov.ConfigDir, "settings.json")
		if _, err := os.Stat(projectPath); err == nil {
			locations = append(locations, SettingsLocation{Scope: ScopeProject, Path: projectPath})
		}
	}

	return locations, nil
}

// hookSettingsPathForScope returns the settings.json path for a given scope.
func hookSettingsPathForScope(prov provider.Provider, scope SettingsScope, projectRoot string) (string, error) {
	switch scope {
	case ScopeProject:
		if projectRoot == "" {
			return "", fmt.Errorf("no project root for project-scoped install")
		}
		return filepath.Join(projectRoot, prov.ConfigDir, "settings.json"), nil
	default: // ScopeGlobal
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, prov.ConfigDir, "settings.json"), nil
	}
}
