package detectors

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

// ProjectMetadata detects project-level facts like description, license, and CI.
type ProjectMetadata struct{}

func (d ProjectMetadata) Name() string { return "project-metadata" }

func (d ProjectMetadata) Detect(root string) ([]model.Section, error) {
	s := &model.ProjectMetadataSection{
		Origin: model.OriginAuto,
		Title:  "Project Metadata",
	}

	for _, name := range []string{"README.md", "README", "readme.md"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err == nil {
			s.Description = extractDescription(string(data))
			break
		}
	}

	for _, name := range []string{"LICENSE", "LICENSE.md", "LICENSE.txt", "COPYING"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err == nil {
			s.License = detectLicenseType(string(data))
			break
		}
	}

	if _, err := os.Stat(filepath.Join(root, ".github", "workflows")); err == nil {
		s.CI = "GitHub Actions"
	} else if _, err := os.Stat(filepath.Join(root, ".gitlab-ci.yml")); err == nil {
		s.CI = "GitLab CI"
	} else if _, err := os.Stat(filepath.Join(root, ".circleci")); err == nil {
		s.CI = "CircleCI"
	}

	if s.Description == "" && s.License == "" && s.CI == "" {
		return nil, nil
	}
	return []model.Section{*s}, nil
}

func extractDescription(readme string) string {
	for _, line := range strings.Split(readme, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		if len(line) > 200 {
			return line[:200] + "..."
		}
		return line
	}
	return ""
}

func detectLicenseType(content string) string {
	lower := strings.ToLower(content)
	switch {
	case strings.Contains(lower, "mit license"):
		return "MIT"
	case strings.Contains(lower, "apache license"):
		return "Apache-2.0"
	case strings.Contains(lower, "gnu general public license"):
		return "GPL"
	case strings.Contains(lower, "bsd"):
		return "BSD"
	case strings.Contains(lower, "isc license"):
		return "ISC"
	default:
		return "Unknown"
	}
}
