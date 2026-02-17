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

const DirName = ".nesco"
const FileName = "config.json"

type Config struct {
	Providers         []string          `json:"providers"`                   // enabled provider slugs
	DisabledDetectors []string          `json:"disabledDetectors,omitempty"`
	Preferences       map[string]string `json:"preferences,omitempty"`
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
