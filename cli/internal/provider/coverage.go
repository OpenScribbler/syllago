package provider

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"gopkg.in/yaml.v3"
)

// CoverageDrift describes a single mismatch detected by CheckCoverage. Drift can be
// internal to Go (assertions 3, 4) or between Go and the documentation YAMLs
// (assertions 1, 2).
type CoverageDrift struct {
	Provider    string
	ContentType catalog.ContentType
	Assertion   string
	Message     string
}

func (d CoverageDrift) String() string {
	return fmt.Sprintf("%s/%s [%s]: %s", d.Provider, d.ContentType, d.Assertion, d.Message)
}

// Assertion names used by CheckCoverage. Kept as exported constants so tests
// and telemetry can filter on them without string-matching.
const (
	AssertionGoVsSourceManifest     = "go-vs-source-manifest"
	AssertionGoVsFormatYAML         = "go-vs-format-yaml"
	AssertionConfigLocationsVsGo    = "configlocations-vs-supportstype"
	AssertionInstallDirVsSupportsGo = "installdir-vs-supportstype"
)

// CoverageContentTypes is the fixed set of content types evaluated by CheckCoverage.
// Loadouts is intentionally excluded — it's a meta-type composed of other content,
// not a per-provider install target.
var CoverageContentTypes = []catalog.ContentType{
	catalog.Rules,
	catalog.Skills,
	catalog.Agents,
	catalog.Commands,
	catalog.Hooks,
	catalog.MCP,
}

// sourceManifest mirrors the subset of docs/provider-sources/<slug>.yaml that
// CheckCoverage needs. Only the content_types map is consulted.
type sourceManifest struct {
	Slug         string                                `yaml:"slug"`
	ContentTypes map[string]sourceManifestContentEntry `yaml:"content_types"`
}

// sourceManifestContentEntry captures the two ways a source manifest asserts
// support: `supported: false` explicitly, or `sources: [...]` implicitly.
type sourceManifestContentEntry struct {
	Supported *bool         `yaml:"supported,omitempty"`
	Sources   []interface{} `yaml:"sources,omitempty"`
}

// supportAssertion returns nil if the manifest does not make a claim about
// this content type, otherwise the asserted value.
func (e sourceManifestContentEntry) supportAssertion() *bool {
	if e.Supported != nil {
		return e.Supported
	}
	if len(e.Sources) > 0 {
		t := true
		return &t
	}
	return nil
}

// formatYAML mirrors the subset of docs/provider-formats/<slug>.yaml that
// CheckCoverage needs.
type formatYAML struct {
	Provider     string                            `yaml:"provider"`
	ContentTypes map[string]formatYAMLContentEntry `yaml:"content_types"`
}

type formatYAMLContentEntry struct {
	Status string `yaml:"status,omitempty"`
}

// supportAssertion reports whether the format YAML asserts supported/unsupported
// for this content type. An empty status is treated as "not asserted" so stub
// entries don't generate false drift.
func (e formatYAMLContentEntry) supportAssertion() *bool {
	switch e.Status {
	case "supported":
		t := true
		return &t
	case "unsupported":
		f := false
		return &f
	default:
		return nil
	}
}

// CheckCoverage validates provider coverage across three axes: Go internal
// consistency (assertions 3, 4), Go vs source manifest (assertion 1), and Go
// vs format YAML (assertion 2). It returns every drift it finds; callers can
// render the full picture in one pass.
//
// repoRoot must be the repository root (the directory containing docs/ and cli/).
// Use FindRepoRoot to locate it from a test's working directory.
func CheckCoverage(repoRoot string) ([]CoverageDrift, error) {
	if repoRoot == "" {
		return nil, fmt.Errorf("repoRoot is empty")
	}

	sourceManifests, err := loadSourceManifests(filepath.Join(repoRoot, "docs", "provider-sources"))
	if err != nil {
		return nil, fmt.Errorf("load source manifests: %w", err)
	}
	formatYAMLs, err := loadFormatYAMLs(filepath.Join(repoRoot, "docs", "provider-formats"))
	if err != nil {
		return nil, fmt.Errorf("load format YAMLs: %w", err)
	}

	const testHome = "/home/covtestuser"
	var drifts []CoverageDrift

	for _, prov := range AllProviders {
		if prov.SupportsType == nil {
			continue
		}
		for _, ct := range CoverageContentTypes {
			goSupported := prov.SupportsType(ct)

			// Assertion 3: ConfigLocations[ct] set ⇒ SupportsType(ct) == true.
			if loc, ok := prov.ConfigLocations[ct]; ok && loc != "" && !goSupported {
				drifts = append(drifts, CoverageDrift{
					Provider:    prov.Slug,
					ContentType: ct,
					Assertion:   AssertionConfigLocationsVsGo,
					Message:     fmt.Sprintf("ConfigLocations[%s]=%q but SupportsType(%s)=false", ct, loc, ct),
				})
			}

			// Assertion 4: InstallDir(home, ct) != "" ⇔ SupportsType(ct) == true.
			var installDir string
			if prov.InstallDir != nil {
				installDir = prov.InstallDir(testHome, ct)
			}
			installPresent := installDir != ""
			if installPresent != goSupported {
				drifts = append(drifts, CoverageDrift{
					Provider:    prov.Slug,
					ContentType: ct,
					Assertion:   AssertionInstallDirVsSupportsGo,
					Message:     fmt.Sprintf("InstallDir(home,%s)=%q, SupportsType(%s)=%v (must be equivalent)", ct, installDir, ct, goSupported),
				})
			}

			// Assertion 1: Go ↔ source manifest.
			if sm, ok := sourceManifests[prov.Slug]; ok {
				if entry, has := sm.ContentTypes[string(ct)]; has {
					if asserted := entry.supportAssertion(); asserted != nil && *asserted != goSupported {
						drifts = append(drifts, CoverageDrift{
							Provider:    prov.Slug,
							ContentType: ct,
							Assertion:   AssertionGoVsSourceManifest,
							Message:     fmt.Sprintf("source manifest says supported=%v but Go SupportsType(%s)=%v", *asserted, ct, goSupported),
						})
					}
				}
			}

			// Assertion 2: Go ↔ format YAML.
			if fy, ok := formatYAMLs[prov.Slug]; ok {
				if entry, has := fy.ContentTypes[string(ct)]; has {
					if asserted := entry.supportAssertion(); asserted != nil && *asserted != goSupported {
						drifts = append(drifts, CoverageDrift{
							Provider:    prov.Slug,
							ContentType: ct,
							Assertion:   AssertionGoVsFormatYAML,
							Message:     fmt.Sprintf("format YAML says supported=%v but Go SupportsType(%s)=%v", *asserted, ct, goSupported),
						})
					}
				}
			}
		}
	}

	return drifts, nil
}

// loadSourceManifests reads every *.yaml file in dir and returns a map keyed
// by the manifest's slug. The _template.yaml file is skipped.
func loadSourceManifests(dir string) (map[string]*sourceManifest, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}
	out := make(map[string]*sourceManifest, len(entries))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		if e.Name() == "_template.yaml" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		var sm sourceManifest
		if err := yaml.Unmarshal(data, &sm); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		if sm.Slug == "" {
			continue
		}
		out[sm.Slug] = &sm
	}
	return out, nil
}

// loadFormatYAMLs reads every *.yaml file in dir and returns a map keyed by
// the format YAML's provider slug.
func loadFormatYAMLs(dir string) (map[string]*formatYAML, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}
	out := make(map[string]*formatYAML, len(entries))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		var fy formatYAML
		if err := yaml.Unmarshal(data, &fy); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		if fy.Provider == "" {
			continue
		}
		out[fy.Provider] = &fy
	}
	return out, nil
}

// FindRepoRoot walks up from start looking for a directory containing both
// a cli/ subdirectory and a docs/ subdirectory (the repo layout markers).
// Returns "" if not found. Callers should treat empty as a hard error —
// CheckCoverage cannot run without a repo root.
func FindRepoRoot(start string) string {
	dir := start
	for {
		_, cliErr := os.Stat(filepath.Join(dir, "cli", "go.mod"))
		_, docsErr := os.Stat(filepath.Join(dir, "docs"))
		if cliErr == nil && docsErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
