package installer

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// InstalledHook records a hook placed into settings.json by syllago.
type InstalledHook struct {
	Name        string    `json:"name"`
	Event       string    `json:"event"`
	GroupHash   string    `json:"groupHash,omitempty"` // SHA256 of the matcher group JSON
	Command     string    `json:"command"`             // kept for display/debugging
	Source      string    `json:"source"`              // "export" or "loadout:<name>"
	Scope       string    `json:"scope,omitempty"`     // "global" or "project"
	Provider    string    `json:"provider,omitempty"`  // slug; empty on entries written before it was tracked
	InstalledAt time.Time `json:"installedAt"`
}

// InstalledMCP records an MCP server placed into a provider config by syllago.
type InstalledMCP struct {
	Name        string    `json:"name"`
	ServerKey   string    `json:"serverKey,omitempty"`   // specific server key (new per-server installs)
	ServerNames []string  `json:"serverNames,omitempty"` // DEPRECATED: legacy bulk installs tracked all keys here
	ContentHash string    `json:"contentHash,omitempty"` // SHA-256 of config content at install time
	Source      string    `json:"source"`
	Provider    string    `json:"provider,omitempty"` // slug; empty on entries written before it was tracked
	InstalledAt time.Time `json:"installedAt"`
}

// serverKeys returns the config keys the entry placed: its server key, the
// server names of a bulk install, or, for the oldest entries, the item name.
func (m InstalledMCP) serverKeys(itemName string) []string {
	if m.ServerKey != "" {
		return []string{m.ServerKey}
	}
	if len(m.ServerNames) > 0 {
		return m.ServerNames
	}
	return []string{itemName}
}

// InstalledSymlink records a symlink placed by syllago.
type InstalledSymlink struct {
	Path        string    `json:"path"`                  // absolute path of the symlink
	Target      string    `json:"target"`                // absolute path it points to
	ContentHash string    `json:"contentHash,omitempty"` // SHA-256 of target content at install time
	Source      string    `json:"source"`
	InstalledAt time.Time `json:"installedAt"`
}

// InstalledRuleAppend records a rule appended to a provider's monolithic file (D14).
type InstalledRuleAppend struct {
	Name        string    `json:"name"`
	LibraryID   string    `json:"libraryId"`
	Provider    string    `json:"provider"`
	TargetFile  string    `json:"targetFile"`
	VersionHash string    `json:"versionHash"` // canonical "<algo>:<hex>" per D11
	Source      string    `json:"source"`
	Scope       string    `json:"scope,omitempty"`
	InstalledAt time.Time `json:"installedAt"`
}

// Installed is the root structure for .syllago/installed.json.
type Installed struct {
	Hooks       []InstalledHook       `json:"hooks,omitempty"`
	MCP         []InstalledMCP        `json:"mcp,omitempty"`
	Symlinks    []InstalledSymlink    `json:"symlinks,omitempty"`
	RuleAppends []InstalledRuleAppend `json:"ruleAppends,omitempty"`
}

const installedFileName = "installed.json"

// installedPath returns the path to .syllago/installed.json.
func installedPath(projectRoot string) string {
	return filepath.Join(config.DirPath(projectRoot), installedFileName)
}

// LoadInstalled reads .syllago/installed.json.
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
	// Entries written before a provider rename carry its retired slug.
	for i := range inst.RuleAppends {
		inst.RuleAppends[i].Provider, _ = provider.ResolveSlugAlias(inst.RuleAppends[i].Provider)
	}
	for i := range inst.Hooks {
		inst.Hooks[i].Provider, _ = provider.ResolveSlugAlias(inst.Hooks[i].Provider)
	}
	for i := range inst.MCP {
		inst.MCP[i].Provider, _ = provider.ResolveSlugAlias(inst.MCP[i].Provider)
	}
	return &inst, nil
}

// SaveInstalled writes .syllago/installed.json atomically.
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

// An entry with an empty Provider was written before entries recorded their
// provider, so it could belong to any provider. The Find methods match such an
// entry for every provider but prefer an entry that names the provider.

// FindHook returns the index of the hook entry for name and event on
// the provider with that slug, or -1 if there is none.
func (inst *Installed) FindHook(name, event, slug string) int {
	legacy := -1
	for i, h := range inst.Hooks {
		if h.Name != name || h.Event != event {
			continue
		}
		if h.Provider == slug {
			return i
		}
		if h.Provider == "" && legacy < 0 {
			legacy = i
		}
	}
	return legacy
}

// FindMCP returns the index of the MCP entry for name on the provider with
// that slug, or -1 if there is none.
func (inst *Installed) FindMCP(name, slug string) int {
	legacy := -1
	for i, m := range inst.MCP {
		if m.Name != name {
			continue
		}
		if m.Provider == slug {
			return i
		}
		if m.Provider == "" && legacy < 0 {
			legacy = i
		}
	}
	return legacy
}

// FindMCPByServerKey returns the index of the MCP entry for name and
// serverKey on the provider with that slug, or -1 if there is none.
func (inst *Installed) FindMCPByServerKey(name, serverKey, slug string) int {
	legacy := -1
	for i, m := range inst.MCP {
		if m.Name != name || m.ServerKey != serverKey {
			continue
		}
		if m.Provider == slug {
			return i
		}
		if m.Provider == "" && legacy < 0 {
			legacy = i
		}
	}
	return legacy
}

// RemoveHook removes the hook entry at the given index.
func (inst *Installed) RemoveHook(idx int) {
	inst.Hooks = append(inst.Hooks[:idx], inst.Hooks[idx+1:]...)
}

// RemoveMCP removes the MCP entry at the given index.
func (inst *Installed) RemoveMCP(idx int) {
	inst.MCP = append(inst.MCP[:idx], inst.MCP[idx+1:]...)
}

// FindRuleAppend returns the index of a rule append entry matching libraryID
// and targetFile, or -1. The (LibraryID, TargetFile) pair is unique per D14.
func (inst *Installed) FindRuleAppend(libraryID, targetFile string) int {
	for i, r := range inst.RuleAppends {
		if r.LibraryID == libraryID && r.TargetFile == targetFile {
			return i
		}
	}
	return -1
}

// RemoveRuleAppend removes the rule append entry at idx.
func (inst *Installed) RemoveRuleAppend(idx int) {
	inst.RuleAppends = append(inst.RuleAppends[:idx], inst.RuleAppends[idx+1:]...)
}
