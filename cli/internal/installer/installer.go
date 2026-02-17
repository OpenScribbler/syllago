package installer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
	"github.com/holdenhewett/romanesco/cli/internal/provider"
)

// InstallMethod controls how content is placed in the target directory.
type InstallMethod string

const (
	MethodSymlink InstallMethod = "symlink"
	MethodCopy    InstallMethod = "copy"
)

// Status represents the install status of an item for a provider.
type Status int

const (
	StatusNotAvailable Status = iota // provider doesn't support this content type
	StatusNotInstalled               // available but not installed
	StatusInstalled                  // installed (symlink points to our repo)
)

func (s Status) String() string {
	switch s {
	case StatusNotAvailable:
		return "-"
	case StatusNotInstalled:
		return "○"
	case StatusInstalled:
		return "●"
	}
	return "?"
}

// IsJSONMerge returns true if the provider uses JSON merge for the given content type.
func IsJSONMerge(prov provider.Provider, itemType catalog.ContentType) bool {
	return prov.InstallDir("", itemType) == provider.JSONMergeSentinel
}

// resolveTarget computes the target path for an item in a provider's install directory.
// Returns an error if the provider doesn't support the content type or uses JSON merge.
func resolveTarget(item catalog.ContentItem, prov provider.Provider) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	installDir := prov.InstallDir(home, item.Type)
	if installDir == "" {
		return "", fmt.Errorf("%s does not support %s", prov.Name, item.Type.Label())
	}
	if installDir == provider.JSONMergeSentinel {
		return "", fmt.Errorf("%s uses JSON merge for %s (not filesystem install)", prov.Name, item.Type.Label())
	}
	if item.Type == catalog.Agents {
		return filepath.Join(installDir, item.Name+".md"), nil
	}
	if item.Type.IsUniversal() {
		return filepath.Join(installDir, item.Name), nil
	}
	return filepath.Join(installDir, filepath.Base(item.Path)), nil
}

// CheckStatus checks whether an item is installed for a given provider.
func CheckStatus(item catalog.ContentItem, prov provider.Provider, repoRoot string) Status {
	// Dispatch to JSON merge handlers for types that need it
	if IsJSONMerge(prov, item.Type) {
		switch item.Type {
		case catalog.MCP:
			return checkMCPStatus(item, prov, repoRoot)
		case catalog.Hooks:
			return checkHookStatus(item, prov, repoRoot)
		}
		return StatusNotAvailable
	}

	targetPath, err := resolveTarget(item, prov)
	if err != nil {
		return StatusNotAvailable
	}

	if IsSymlinkedTo(targetPath, repoRoot) {
		return StatusInstalled
	}

	// Also check if target exists as a regular file (e.g., installed via copy)
	if _, err := os.Lstat(targetPath); err == nil {
		return StatusInstalled
	}

	return StatusNotInstalled
}

// Install places the given item under the provider's install directory.
// For JSON merge types (MCP, hooks), it merges into the provider's config file.
// For filesystem types, it creates a symlink or copy depending on the method.
// Returns a description of what was installed on success.
func Install(item catalog.ContentItem, prov provider.Provider, repoRoot string, method InstallMethod) (string, error) {
	// Dispatch to JSON merge handlers for types that need it
	if IsJSONMerge(prov, item.Type) {
		switch item.Type {
		case catalog.MCP:
			return installMCP(item, prov, repoRoot)
		case catalog.Hooks:
			return installHook(item, prov, repoRoot)
		}
		return "", fmt.Errorf("%s does not support %s via JSON merge", prov.Name, item.Type.Label())
	}

	targetPath, err := resolveTarget(item, prov)
	if err != nil {
		return "", err
	}

	// Agents install the AGENT.md file, not the whole directory
	sourcePath := item.Path
	if item.Type == catalog.Agents {
		sourcePath = filepath.Join(item.Path, "AGENT.md")
	}

	switch method {
	case MethodCopy:
		return targetPath, CopyContent(sourcePath, targetPath)
	default:
		if IsWindowsMount(targetPath) {
			fmt.Fprintf(os.Stderr, "note: %s is on a Windows mount, using copy instead of symlink\n", targetPath)
			return targetPath, CopyContent(sourcePath, targetPath)
		}
		return targetPath, CreateSymlink(sourcePath, targetPath)
	}
}

// Uninstall removes the given item from the provider's install directory.
// For JSON merge types, it removes the entry from the provider's config file.
// For filesystem types, it removes the symlink.
// Returns a description of what was removed on success.
func Uninstall(item catalog.ContentItem, prov provider.Provider, repoRoot string) (string, error) {
	// Dispatch to JSON merge handlers for types that need it
	if IsJSONMerge(prov, item.Type) {
		switch item.Type {
		case catalog.MCP:
			return uninstallMCP(item, prov, repoRoot)
		case catalog.Hooks:
			return uninstallHook(item, prov, repoRoot)
		}
		return "", fmt.Errorf("%s does not support %s via JSON merge", prov.Name, item.Type.Label())
	}

	targetPath, err := resolveTarget(item, prov)
	if err != nil {
		return "", err
	}

	info, err := os.Lstat(targetPath)
	if err != nil {
		return "", fmt.Errorf("not installed: %s", targetPath)
	}

	// Remove symlinks or regular files (copies)
	if info.Mode()&os.ModeSymlink != 0 || info.Mode().IsRegular() {
		return targetPath, os.Remove(targetPath)
	}

	// Remove directories (copy-installed content)
	if info.IsDir() {
		return targetPath, os.RemoveAll(targetPath)
	}

	return "", fmt.Errorf("unexpected file type at %s, remove manually", targetPath)
}
