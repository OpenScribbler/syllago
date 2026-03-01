package installer

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/OpenScribbler/nesco/cli/internal/config"
)

// InstalledHook records a hook placed into settings.json by nesco.
type InstalledHook struct {
	Name        string    `json:"name"`
	Event       string    `json:"event"`
	Command     string    `json:"command"`
	Source      string    `json:"source"`      // "export" or "loadout:<name>"
	InstalledAt time.Time `json:"installedAt"`
}

// InstalledMCP records an MCP server placed into .claude.json by nesco.
type InstalledMCP struct {
	Name        string    `json:"name"`
	Source      string    `json:"source"`
	InstalledAt time.Time `json:"installedAt"`
}

// InstalledSymlink records a symlink placed by nesco.
type InstalledSymlink struct {
	Path        string    `json:"path"`   // absolute path of the symlink
	Target      string    `json:"target"` // absolute path it points to
	Source      string    `json:"source"`
	InstalledAt time.Time `json:"installedAt"`
}

// Installed is the root structure for .nesco/installed.json.
type Installed struct {
	Hooks    []InstalledHook    `json:"hooks,omitempty"`
	MCP      []InstalledMCP     `json:"mcp,omitempty"`
	Symlinks []InstalledSymlink `json:"symlinks,omitempty"`
}

const installedFileName = "installed.json"

// installedPath returns the path to .nesco/installed.json.
func installedPath(projectRoot string) string {
	return filepath.Join(config.DirPath(projectRoot), installedFileName)
}

// LoadInstalled reads .nesco/installed.json.
// Returns an empty Installed if the file does not exist.
func LoadInstalled(projectRoot string) (*Installed, error) {
	data, err := os.ReadFile(installedPath(projectRoot))
	if errors.Is(err, fs.ErrNotExist) {
		return &Installed{}, nil
	}
	if err != nil {
		return nil, err
	}
	var inst Installed
	if err := json.Unmarshal(data, &inst); err != nil {
		return nil, err
	}
	return &inst, nil
}

// SaveInstalled writes .nesco/installed.json atomically.
func SaveInstalled(projectRoot string, inst *Installed) error {
	dir := config.DirPath(projectRoot)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(inst, "", "  ")
	if err != nil {
		return err
	}
	target := installedPath(projectRoot)
	return writeJSONFileWithPerm(target, data, 0644)
}

// FindHook returns the index of a hook entry matching name and event, or -1 if not found.
func (inst *Installed) FindHook(name, event string) int {
	for i, h := range inst.Hooks {
		if h.Name == name && h.Event == event {
			return i
		}
	}
	return -1
}

// FindMCP returns the index of an MCP entry matching name, or -1 if not found.
func (inst *Installed) FindMCP(name string) int {
	for i, m := range inst.MCP {
		if m.Name == name {
			return i
		}
	}
	return -1
}

// RemoveHook removes the hook entry at the given index.
func (inst *Installed) RemoveHook(idx int) {
	inst.Hooks = append(inst.Hooks[:idx], inst.Hooks[idx+1:]...)
}

// RemoveMCP removes the MCP entry at the given index.
func (inst *Installed) RemoveMCP(idx int) {
	inst.MCP = append(inst.MCP[:idx], inst.MCP[idx+1:]...)
}
