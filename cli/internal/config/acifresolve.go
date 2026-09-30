package config

import (
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/acif"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func acifContentType(ct catalog.ContentType) string {
	switch ct {
	case catalog.Skills:
		return "skill"
	case catalog.Agents:
		return "agent"
	case catalog.MCP:
		return "mcp_config"
	case catalog.Rules:
		return "rule"
	case catalog.Hooks:
		return "hook"
	case catalog.Commands:
		return "command"
	default:
		return ""
	}
}

// matrixInstallDir returns the install directory the ACIF matrix gives for
// the first current user-scope row that applies on goos. appDataDir is what
// a leading <appdata> token expands to (%APPDATA% on Windows); those rows
// are windows-only, so OS filtering removes them everywhere else.
func matrixInstallDir(providerSlug string, ct catalog.ContentType, homeDir, goos, appDataDir string) (string, bool) {
	contentType := acifContentType(ct)
	if contentType == "" {
		return "", false
	}

	rows, err := acif.InstallEntryRows(providerSlug, contentType)
	if err != nil {
		return "", false
	}
	for _, row := range acif.FilterInstallRowsByOS(rows, goos) {
		if row.Status != "current" || row.Scope != "user" {
			continue
		}
		if row.Layout == "merged_into_shared_file" {
			if ct != catalog.Hooks && ct != catalog.MCP {
				return "", false
			}
			return provider.JSONMergeSentinel, true
		}
		return matrixTemplateDir(row.PathTemplate, homeDir, appDataDir)
	}
	return "", false
}

func matrixTemplateDir(pathTemplate, homeDir, appDataDir string) (string, bool) {
	resolved := pathTemplate
	switch {
	case strings.HasPrefix(resolved, "~/"):
		resolved = homeDir + resolved[1:]
	case strings.HasPrefix(resolved, acif.AppDataPrefix):
		// An unset %APPDATA% leaves nothing to anchor to; the caller falls
		// back to the provider's own install directory.
		if appDataDir == "" {
			return "", false
		}
		resolved = strings.TrimRight(appDataDir, `/\`) + "/" + resolved[len(acif.AppDataPrefix):]
	}

	trimmed := strings.TrimRight(resolved, "/")
	lastSlash := strings.LastIndex(trimmed, "/")
	if lastSlash < 0 {
		return "", false
	}

	parent := trimmed[:lastSlash]
	finalSegment := trimmed[lastSlash+1:]
	if parent == "" || !strings.Contains(finalSegment, "<content-name>") {
		return "", false
	}
	if strings.Contains(parent, "<content-name>") {
		return "", false
	}
	return filepath.Clean(parent), true
}
