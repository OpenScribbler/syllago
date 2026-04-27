package provider

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// binaryOnPath returns true if name resolves to an executable on PATH.
// Used by provider Detect() functions as the primary "is the app installed"
// signal for CLI tools.
func binaryOnPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// fileExists returns true if path is a regular file (not a directory).
// Used by provider Detect() functions to check for app-only state files
// like ~/.claude.json or ~/.codex/auth.json that syllago never writes.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// dirExists returns true if path is an existing directory.
// Used by provider Detect() functions to check OS-specific app-data dirs
// (e.g., ~/.config/Cursor/) that are distinct from syllago's install paths.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// appDataDir returns the OS-specific application data directory for the
// named app, joined under homeDir. Used by Electron-based provider Detect
// functions (Cursor, Windsurf, Kiro) to find paths the IDE itself creates
// — distinct from ~/.<slug>/ which syllago also writes into.
//
//   - Linux:   <homeDir>/.config/<appName>
//   - macOS:   <homeDir>/Library/Application Support/<appName>
//   - other:   "" (Windows + WSL host detection deferred — see syllago-rh3u4)
func appDataDir(homeDir, appName string) string {
	switch runtime.GOOS {
	case "linux":
		return filepath.Join(homeDir, ".config", appName)
	case "darwin":
		return filepath.Join(homeDir, "Library", "Application Support", appName)
	default:
		return ""
	}
}

// vscodeExtensionInstalled returns true if homeDir contains an installed
// VS Code extension whose directory name starts with extensionID + "-".
// VS Code stores extensions at ~/.vscode/extensions/<id>-<version>/.
// Used by provider Detect() functions for VS Code extension-based providers
// (Cline, Roo Code).
func vscodeExtensionInstalled(homeDir, extensionID string) bool {
	extDir := filepath.Join(homeDir, ".vscode", "extensions")
	entries, err := os.ReadDir(extDir)
	if err != nil {
		return false
	}
	prefix := extensionID + "-"
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), prefix) {
			return true
		}
	}
	return false
}

// ghLookPath and ghRunCommand are package-level overrides so ghExtensionInstalled
// can be tested deterministically without requiring gh on the test host.
var (
	ghLookPath   = exec.LookPath
	ghRunCommand = func(name string, args ...string) ([]byte, error) {
		return exec.Command(name, args...).Output()
	}
)

// ghExtensionInstalled returns true if `gh extension list` output contains name.
// Returns false (without error) when gh is not on PATH or the command fails.
// Used by provider Detect() functions for gh-extension-based providers
// (Copilot CLI).
func ghExtensionInstalled(name string) bool {
	if _, err := ghLookPath("gh"); err != nil {
		return false
	}
	out, err := ghRunCommand("gh", "extension", "list")
	if err != nil {
		return false
	}
	return strings.Contains(string(out), name)
}
