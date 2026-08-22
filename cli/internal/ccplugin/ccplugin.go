package ccplugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Plugin is one installed Claude Code plugin scope-entry, read-only.
type Plugin struct {
	Name        string
	Marketplace string
	ID          string
	Enabled     bool
	Scope       string
	InstallPath string
	Version     string
	InstalledAt time.Time
	Skills      []Skill
}

// Skill is a skill directory a plugin provides.
type Skill struct {
	Name string
	Path string
}

type installedPluginsLedger struct {
	Plugins map[string][]installedPluginEntry `json:"plugins"`
}

type installedPluginEntry struct {
	Scope       string `json:"scope"`
	InstallPath string `json:"installPath"`
	Version     string `json:"version"`
	InstalledAt string `json:"installedAt"`
}

type settingsFile struct {
	EnabledPlugins map[string]bool `json:"enabledPlugins"`
}

// Load reads Claude Code's installed plugin ledger and settings under home.
func Load(home string) ([]Plugin, error) {
	claudeDir := filepath.Join(home, ".claude")

	enabled, err := loadEnabledPlugins(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		return nil, err
	}

	ledgerPath := filepath.Join(claudeDir, "plugins", "installed_plugins.json")
	body, err := os.ReadFile(ledgerPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Plugin{}, nil
		}
		return nil, fmt.Errorf("read Claude Code plugin ledger %s: %w", ledgerPath, err)
	}

	var ledger installedPluginsLedger
	if err := json.Unmarshal(body, &ledger); err != nil {
		return nil, fmt.Errorf("parse Claude Code plugin ledger %s: %w", ledgerPath, err)
	}

	plugins := make([]Plugin, 0)
	for id, entries := range ledger.Plugins {
		name, marketplace := splitPluginID(id)
		for _, entry := range entries {
			plugins = append(plugins, Plugin{
				Name:        name,
				Marketplace: marketplace,
				ID:          id,
				Enabled:     enabled[id],
				Scope:       entry.Scope,
				InstallPath: entry.InstallPath,
				Version:     entry.Version,
				InstalledAt: parseRFC3339(entry.InstalledAt),
				Skills:      loadSkills(entry.InstallPath),
			})
		}
	}

	sort.Slice(plugins, func(i, j int) bool {
		if plugins[i].Marketplace != plugins[j].Marketplace {
			return plugins[i].Marketplace < plugins[j].Marketplace
		}
		if plugins[i].Name != plugins[j].Name {
			return plugins[i].Name < plugins[j].Name
		}
		if plugins[i].Scope != plugins[j].Scope {
			return plugins[i].Scope < plugins[j].Scope
		}
		if plugins[i].InstallPath != plugins[j].InstallPath {
			return plugins[i].InstallPath < plugins[j].InstallPath
		}
		return plugins[i].Version < plugins[j].Version
	})

	return plugins, nil
}

func loadEnabledPlugins(path string) (map[string]bool, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]bool{}, nil
		}
		return nil, fmt.Errorf("read Claude Code settings %s: %w", path, err)
	}

	var settings settingsFile
	if err := json.Unmarshal(body, &settings); err != nil {
		return nil, fmt.Errorf("parse Claude Code settings %s: %w", path, err)
	}
	if settings.EnabledPlugins == nil {
		return map[string]bool{}, nil
	}
	return settings.EnabledPlugins, nil
}

func splitPluginID(id string) (string, string) {
	idx := strings.LastIndex(id, "@")
	if idx < 0 {
		return id, ""
	}
	return id[:idx], id[idx+1:]
}

func parseRFC3339(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func loadSkills(installPath string) []Skill {
	if installPath == "" {
		return nil
	}
	info, err := os.Stat(installPath)
	if err != nil || !info.IsDir() {
		return nil
	}

	form, ok := loadManifestSkillsForm(installPath)
	if !ok {
		return conventionSkills(installPath)
	}

	switch value := form.(type) {
	case string:
		return skillsUnderDirectory(pluginPath(installPath, value))
	case []string:
		return skillsFromDirectories(installPath, value)
	default:
		return conventionSkills(installPath)
	}
}

func loadManifestSkillsForm(installPath string) (any, bool) {
	manifestPath := filepath.Join(installPath, ".claude-plugin", "plugin.json")
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, false
	}

	var manifest map[string]json.RawMessage
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, false
	}
	rawSkills, ok := manifest["skills"]
	if !ok {
		return nil, false
	}

	var directory string
	if err := json.Unmarshal(rawSkills, &directory); err == nil {
		return directory, true
	}

	var directories []string
	if err := json.Unmarshal(rawSkills, &directories); err == nil {
		return directories, true
	}

	return nil, false
}

func conventionSkills(installPath string) []Skill {
	return skillsUnderDirectory(filepath.Join(installPath, "skills"))
}

func skillsUnderDirectory(skillsDir string) []Skill {
	matches, err := filepath.Glob(filepath.Join(skillsDir, "*", "SKILL.md"))
	if err != nil {
		return nil
	}

	skills := make([]Skill, 0, len(matches))
	for _, match := range matches {
		dir := filepath.Dir(match)
		skills = append(skills, Skill{
			Name: filepath.Base(dir),
			Path: dir,
		})
	}
	sortSkills(skills)
	return skills
}

func skillsFromDirectories(installPath string, directories []string) []Skill {
	skills := make([]Skill, 0, len(directories))
	for _, directory := range directories {
		dir := pluginPath(installPath, directory)
		info, err := os.Stat(filepath.Join(dir, "SKILL.md"))
		if err != nil || info.IsDir() {
			continue
		}
		skills = append(skills, Skill{
			Name: filepath.Base(dir),
			Path: dir,
		})
	}
	sortSkills(skills)
	return skills
}

// pluginPath resolves a manifest-relative content path. Manifests are
// third-party data, so a path escaping the plugin's own install directory
// resolves to "" (callers then find no skills there).
func pluginPath(installPath, manifestPath string) string {
	if manifestPath == "" {
		return installPath
	}
	joined := filepath.Clean(filepath.Join(installPath, filepath.FromSlash(manifestPath)))
	rel, err := filepath.Rel(installPath, joined)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ""
	}
	return joined
}

func sortSkills(skills []Skill) {
	sort.Slice(skills, func(i, j int) bool {
		if skills[i].Name != skills[j].Name {
			return skills[i].Name < skills[j].Name
		}
		return skills[i].Path < skills[j].Path
	})
}
