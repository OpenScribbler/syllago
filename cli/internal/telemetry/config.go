package telemetry

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

const (
	configFileName    = "telemetry.json"
	sysDirPath        = "/etc/syllago"
	sysConfigFileName = "telemetry.json"
)

// Config is the user-level telemetry config stored at ~/.syllago/telemetry.json.
type Config struct {
	Enabled     bool      `json:"enabled"`
	AnonymousID string    `json:"anonymousId"`
	NoticeSeen  bool      `json:"noticeSeen"`
	Endpoint    string    `json:"endpoint,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// SysConfig is the system-level config at /etc/syllago/telemetry.json.
// Only the Enabled field is honoured; all other settings come from the user config.
type SysConfig struct {
	Enabled bool `json:"enabled"`
}

// UserHomeDirFn is the home directory resolver. Exported so tests in other packages
// (e.g., cmd tests) can override it without build constraints.
var UserHomeDirFn = os.UserHomeDir

// userConfigPath returns ~/.syllago/telemetry.json, or an error if home is unknown.
func userConfigPath() (string, error) {
	home, err := UserHomeDirFn()
	if err != nil {
		return "", fmt.Errorf("getting home dir: %w", err)
	}
	return filepath.Join(home, ".syllago", configFileName), nil
}

// sysConfigPath returns /etc/syllago/telemetry.json.
func sysConfigPath() string {
	return filepath.Join(sysDirPath, sysConfigFileName)
}

// loadUserConfig reads ~/.syllago/telemetry.json.
// Returns (nil, nil) if the file does not exist.
// Returns (nil, err) if the file exists but is unreadable or malformed — callers
// must treat this as telemetry disabled.
func loadUserConfig() (*Config, error) {
	path, err := userConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading telemetry config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing telemetry config: %w", err)
	}
	return &cfg, nil
}

// saveUserConfig writes cfg to ~/.syllago/telemetry.json atomically.
func saveUserConfig(cfg *Config) error {
	path, err := userConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	if err := checkWritable(filepath.Dir(path)); err != nil {
		return fmt.Errorf("config dir not writable: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return fmt.Errorf("generating temp suffix: %w", err)
	}
	tmp := path + ".tmp." + hex.EncodeToString(suffix)
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("writing temp config: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("renaming temp config: %w", err)
	}
	return nil
}

// loadSysConfig reads /etc/syllago/telemetry.json.
// Returns (nil, nil) if the file does not exist — absence means no system override.
func loadSysConfig() (*SysConfig, error) {
	data, err := os.ReadFile(sysConfigPath())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading system telemetry config: %w", err)
	}
	var sc SysConfig
	if err := json.Unmarshal(data, &sc); err != nil {
		return nil, fmt.Errorf("parsing system telemetry config: %w", err)
	}
	return &sc, nil
}

// checkWritable returns nil if the given directory is writable by the current process.
func checkWritable(dir string) error {
	probe := filepath.Join(dir, ".write-probe-"+hex.EncodeToString([]byte{0, 1, 2, 3}))
	f, err := os.OpenFile(probe, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_ = f.Close()
	_ = os.Remove(probe)
	return nil
}

// newConfig creates a default config with a fresh pseudonymous ID and the current time.
func newConfig() (*Config, error) {
	id, err := generateID()
	if err != nil {
		return nil, err
	}
	return &Config{
		Enabled:     true,
		AnonymousID: id,
		NoticeSeen:  false,
		CreatedAt:   time.Now().UTC(),
	}, nil
}
