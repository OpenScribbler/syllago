package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/OpenScribbler/nesco/cli/internal/installer"
)

// saveEnvToFile writes a KEY=VALUE line to the specified file (e.g., a .env file).
// Creates the file and parent directories if they don't exist.
func saveEnvToFile(name, value, filePath string) error {
	expanded, err := expandHome(filePath)
	if err != nil {
		return err
	}
	filePath = expanded

	parent := filepath.Dir(filePath)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	// Use single quotes to prevent shell expansion ($, `, etc.)
	// Escape embedded single quotes with the '\'' idiom
	escapedValue := strings.ReplaceAll(value, "'", "'\\''")
	line := fmt.Sprintf("%s='%s'\n", name, escapedValue)
	_, err = f.WriteString(line)
	return err
}

// loadEnvFromFile reads a .env file and looks for the specified variable.
// If found, sets it in the current process environment.
func loadEnvFromFile(name, filePath string) error {
	expanded, err := expandHome(filePath)
	if err != nil {
		return err
	}
	filePath = expanded

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Strip "export " prefix if present
		line = strings.TrimPrefix(line, "export ")
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == name {
			value := strings.TrimSpace(parts[1])
			// Strip surrounding quotes
			value = strings.Trim(value, "\"'")
			os.Setenv(name, value)
			return nil
		}
	}

	return fmt.Errorf("%s not found in %s", name, filePath)
}

// unsetEnvVarNames returns a sorted list of env var names that are not currently set.
func (m detailModel) unsetEnvVarNames() []string {
	if m.mcpConfig == nil {
		return nil
	}
	envStatus := installer.CheckEnvVars(m.mcpConfig)
	names := make([]string, 0, len(envStatus))
	for name := range envStatus {
		names = append(names, name)
	}
	sort.Strings(names)
	var unset []string
	for _, name := range names {
		if !envStatus[name] {
			unset = append(unset, name)
		}
	}
	return unset
}

// hasUnsetEnvVars returns true if the MCP config has env vars that aren't set.
func (m detailModel) hasUnsetEnvVars() bool {
	if m.mcpConfig == nil {
		return false
	}
	for k := range m.mcpConfig.Env {
		if _, ok := os.LookupEnv(k); !ok {
			return true
		}
	}
	return false
}
