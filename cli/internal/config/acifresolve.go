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

func matrixInstallDir(providerSlug string, ct catalog.ContentType, homeDir string) (string, bool) {
	contentType := acifContentType(ct)
	if contentType == "" {
		return "", false
	}

	rows, err := acif.InstallEntryRows(providerSlug, contentType)
	if err != nil {
		return "", false
	}
	for _, row := range rows {
		if row.Status != "current" || row.Scope != "user" {
			continue
		}
		if row.Layout == "merged_into_shared_file" {
			if ct != catalog.Hooks && ct != catalog.MCP {
				return "", false
			}
			return provider.JSONMergeSentinel, true
		}
		return matrixTemplateDir(row.PathTemplate, homeDir)
	}
	return "", false
}

func matrixTemplateDir(pathTemplate, homeDir string) (string, bool) {
	resolved := pathTemplate
	if strings.HasPrefix(resolved, "~/") {
		resolved = homeDir + resolved[1:]
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
