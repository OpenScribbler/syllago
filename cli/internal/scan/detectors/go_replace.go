package detectors

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

// GoReplace parses go.mod for `replace` directives that use local filesystem
// paths (containing `../` or `./`). These break builds outside the developer's
// machine — CI, other contributors, and module proxies can't resolve local
// paths. Remote replacements (pointing to another module version) are fine and
// are ignored.
type GoReplace struct{}

func (d GoReplace) Name() string { return "go-replace" }

func (d GoReplace) Detect(root string) ([]model.Section, error) {
	gomodPath := filepath.Join(root, "go.mod")
	if _, err := os.Stat(gomodPath); err != nil {
		return nil, nil
	}

	data, err := os.ReadFile(gomodPath)
	if err != nil {
		return nil, nil
	}

	localReplaces := findLocalReplaces(string(data))
	if len(localReplaces) == 0 {
		return nil, nil
	}

	return []model.Section{
		model.TextSection{
			Category: model.CatSurprise,
			Origin:   model.OriginAuto,
			Title:    "Local Path Replacements in go.mod",
			Body:     fmt.Sprintf("go.mod contains %d local-path replace directive(s): %s. These only work on this machine and will break CI, module proxies, and other contributors' builds.", len(localReplaces), strings.Join(localReplaces, "; ")),
			Source:   d.Name(),
		},
	}, nil
}

// findLocalReplaces extracts replace directives that point to local paths.
// Handles both single-line (`replace A => ../B`) and block syntax
// (`replace ( ... )`).
func findLocalReplaces(gomod string) []string {
	var results []string
	lines := strings.Split(gomod, "\n")
	inReplaceBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Start of replace block.
		if strings.HasPrefix(trimmed, "replace (") || trimmed == "replace (" {
			inReplaceBlock = true
			continue
		}

		// End of replace block.
		if inReplaceBlock && trimmed == ")" {
			inReplaceBlock = false
			continue
		}

		// Single-line replace directive.
		if strings.HasPrefix(trimmed, "replace ") && !inReplaceBlock {
			if local := extractLocalPath(trimmed); local != "" {
				results = append(results, local)
			}
			continue
		}

		// Line inside a replace block.
		if inReplaceBlock && strings.Contains(trimmed, "=>") {
			if local := extractLocalPath(trimmed); local != "" {
				results = append(results, local)
			}
		}
	}

	return results
}

// extractLocalPath checks if a replace line points to a local path and returns
// a human-readable description, or "" if it's a remote replacement.
func extractLocalPath(line string) string {
	parts := strings.SplitN(line, "=>", 2)
	if len(parts) < 2 {
		return ""
	}
	target := strings.TrimSpace(parts[1])
	if strings.HasPrefix(target, "../") || strings.HasPrefix(target, "./") {
		module := strings.TrimSpace(strings.TrimPrefix(parts[0], "replace"))
		return fmt.Sprintf("%s => %s", strings.TrimSpace(module), target)
	}
	return ""
}
