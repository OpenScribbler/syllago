package analyzer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

// ValidationIssue describes a disagreement between registry.yaml and file content.
type ValidationIssue struct {
	ItemName     string
	DeclaredType string
	DetectedType catalog.ContentType
	Path         string
	Severity     string // "warning", "error"
	Message      string
}

// ValidateManifest cross-checks an authored registry.yaml against actual file content.
func ValidateManifest(m *registry.Manifest, repoRoot string) []ValidationIssue {
	if m == nil {
		return nil
	}
	var issues []ValidationIssue
	for _, item := range m.Items {
		absPath := filepath.Join(repoRoot, item.Path)

		// Boundary check.
		resolved, err := filepath.EvalSymlinks(absPath)
		if err != nil || !isWithinRoot(resolved, repoRoot) {
			issues = append(issues, ValidationIssue{
				ItemName: item.Name,
				Path:     item.Path,
				Severity: "error",
				Message:  fmt.Sprintf("path %q does not resolve within repository boundary", item.Path),
			})
			continue
		}

		// Existence check.
		if _, err := os.Stat(absPath); err != nil {
			issues = append(issues, ValidationIssue{
				ItemName: item.Name,
				Path:     item.Path,
				Severity: "error",
				Message:  fmt.Sprintf("path %q not found", item.Path),
			})
			continue
		}

		// Type plausibility check.
		if issue := checkTypePlausibility(item, absPath); issue != nil {
			issues = append(issues, *issue)
		}
	}
	return issues
}

// checkTypePlausibility performs a lightweight check that the file content is
// consistent with the declared content type.
func checkTypePlausibility(item registry.ManifestItem, absPath string) *ValidationIssue {
	ext := filepath.Ext(absPath)
	declaredType := catalog.ContentType(item.Type)

	switch declaredType {
	case catalog.Hooks:
		validExts := map[string]bool{".json": true, ".ts": true, ".js": true, ".py": true, ".sh": true}
		if !validExts[ext] {
			return &ValidationIssue{
				ItemName:     item.Name,
				DeclaredType: item.Type,
				Path:         item.Path,
				Severity:     "warning",
				Message:      fmt.Sprintf("declared as hook but file extension %q is unusual for hooks", ext),
			}
		}
	case catalog.MCP:
		if ext != ".json" {
			return &ValidationIssue{
				ItemName:     item.Name,
				DeclaredType: item.Type,
				Path:         item.Path,
				Severity:     "warning",
				Message:      fmt.Sprintf("declared as MCP config but file extension is %q (expected .json)", ext),
			}
		}
	case catalog.Skills, catalog.Agents, catalog.Rules, catalog.Commands:
		if ext != ".md" && ext != ".mdc" {
			return &ValidationIssue{
				ItemName:     item.Name,
				DeclaredType: item.Type,
				Path:         item.Path,
				Severity:     "warning",
				Message:      fmt.Sprintf("declared as %s but file extension is %q (expected .md)", item.Type, ext),
			}
		}
	}
	return nil
}

func isWithinRoot(resolved, root string) bool {
	return resolved == root || len(resolved) > len(root) &&
		resolved[len(root)] == filepath.Separator &&
		resolved[:len(root)] == root
}
