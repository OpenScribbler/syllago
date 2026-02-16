package config

import (
	"encoding/json"
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
	if os.IsNotExist(err) {
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
	return os.WriteFile(FilePath(projectRoot), data, 0644)
}

func Exists(projectRoot string) bool {
	_, err := os.Stat(FilePath(projectRoot))
	return err == nil
}
