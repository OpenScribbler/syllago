package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const DirName = ".syllago"
const FileName = "config.json"

// Registry represents a git-based content source registered in this project.
type Registry struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Ref  string `json:"ref,omitempty"` // branch/tag/commit, defaults to default branch
}

// SandboxConfig holds project-level sandbox policy.
// Git-tracked so teams share the same sandbox settings.
type SandboxConfig struct {
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	AllowedEnv     []string `json:"allowed_env,omitempty"`
	AllowedPorts   []int    `json:"allowed_ports,omitempty"`
}

type Config struct {
	Providers         []string          `json:"providers"`              // enabled provider slugs
	ContentRoot       string            `json:"content_root,omitempty"` // relative path to content directory (default: project root)
	Registries        []Registry        `json:"registries,omitempty"`
	AllowedRegistries []string          `json:"allowed_registries,omitempty"` // URL allowlist; empty means any URL is permitted
	Preferences       map[string]string `json:"preferences,omitempty"`
	Sandbox           SandboxConfig     `json:"sandbox,omitempty"`
}

// IsRegistryAllowed returns true if url is permitted given the config.
// When AllowedRegistries is empty, any URL is allowed (solo-user default).
// When non-empty, url must appear in the list (exact string match).
func (c *Config) IsRegistryAllowed(url string) bool {
	if len(c.AllowedRegistries) == 0 {
		return true
	}
	for _, allowed := range c.AllowedRegistries {
		if allowed == url {
			return true
		}
	}
	return false
}

func DirPath(projectRoot string) string {
	return filepath.Join(projectRoot, DirName)
}

func FilePath(projectRoot string) string {
	return filepath.Join(projectRoot, DirName, FileName)
}

func Load(projectRoot string) (*Config, error) {
	data, err := os.ReadFile(FilePath(projectRoot))
	if errors.Is(err, fs.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func Save(projectRoot string, cfg *Config) error {
	dir := DirPath(projectRoot)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	// Atomic write: temp file then rename
	target := FilePath(projectRoot)
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return fmt.Errorf("generating temp suffix: %w", err)
	}
	tempPath := target + ".tmp." + hex.EncodeToString(suffix)

	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tempPath, target); err != nil {
		os.Remove(tempPath)
		return err
	}
	return nil
}

func Exists(projectRoot string) bool {
	_, err := os.Stat(FilePath(projectRoot))
	return err == nil
}

// GlobalDirPath returns the global syllago config directory (~/.syllago/).
func GlobalDirPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, DirName), nil
}

// GlobalFilePath returns the path to the global config file (~/.syllago/config.json).
func GlobalFilePath() (string, error) {
	dir, err := GlobalDirPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, FileName), nil
}

// LoadGlobal loads the global config from ~/.syllago/config.json.
// Returns an empty Config if the file does not exist.
func LoadGlobal() (*Config, error) {
	path, err := GlobalFilePath()
	if err != nil {
		return nil, fmt.Errorf("global config path: %w", err)
	}
	return LoadFromPath(path)
}

// LoadFromPath loads a config from an explicit file path.
// Returns an empty Config if the file does not exist.
func LoadFromPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SaveGlobal writes cfg to ~/.syllago/config.json, creating the directory if needed.
func SaveGlobal(cfg *Config) error {
	dir, err := GlobalDirPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	target := filepath.Join(dir, FileName)
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return fmt.Errorf("generating temp suffix: %w", err)
	}
	tempPath := target + ".tmp." + hex.EncodeToString(suffix)

	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tempPath, target); err != nil {
		os.Remove(tempPath)
		return err
	}
	return nil
}
