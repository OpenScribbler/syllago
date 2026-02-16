package detectors

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

// VersionMismatch detects when dependency versions conflict with filesystem
// or code patterns. For example, a Next.js 13+ project that has both pages/
// and app/ directories is using mixed routing — worth flagging because it's a
// common source of confusion during migration.
type VersionMismatch struct{}

func (d VersionMismatch) Name() string { return "version-mismatch" }

func (d VersionMismatch) Detect(root string) ([]model.Section, error) {
	var sections []model.Section

	if s := detectNextMixedRouting(root); s != nil {
		sections = append(sections, *s)
	}

	return sections, nil
}

// detectNextMixedRouting flags Next.js >= 13 projects that have both pages/
// and app/ directories. The App Router (app/) was introduced in Next.js 13
// alongside the legacy Pages Router (pages/). Having both is valid but
// surprising — it means two routing systems coexist, which affects middleware,
// layouts, and data fetching patterns.
func detectNextMixedRouting(root string) *model.TextSection {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return nil
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}

	// Check both deps and devDeps for next.
	nextVer := pkg.Dependencies["next"]
	if nextVer == "" {
		nextVer = pkg.DevDependencies["next"]
	}
	if nextVer == "" {
		return nil
	}

	major := parseMajorVersion(nextVer)
	if major < 13 {
		return nil
	}

	// Check for both routing directories.
	_, pagesErr := os.Stat(filepath.Join(root, "pages"))
	_, appErr := os.Stat(filepath.Join(root, "app"))
	// Also check under src/ — a common Next.js convention.
	if pagesErr != nil {
		_, pagesErr = os.Stat(filepath.Join(root, "src", "pages"))
	}
	if appErr != nil {
		_, appErr = os.Stat(filepath.Join(root, "src", "app"))
	}

	if pagesErr != nil || appErr != nil {
		return nil // only one (or neither) router present
	}

	return &model.TextSection{
		Category: model.CatSurprise,
		Origin:   model.OriginAuto,
		Title:    "Next.js Mixed Routing",
		Body:     fmt.Sprintf("Next.js %d uses both pages/ and app/ directories — the Pages Router and App Router coexist. This is valid during migration but affects routing, layouts, and data fetching.", major),
		Source:   "version-mismatch",
	}
}

// parseMajorVersion extracts the major version number from a semver-ish
// string, stripping common prefixes like ^ and ~.
func parseMajorVersion(ver string) int {
	ver = strings.TrimLeft(ver, "^~>=<v ")
	parts := strings.SplitN(ver, ".", 2)
	n, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0
	}
	return n
}
