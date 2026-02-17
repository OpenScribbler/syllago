package detectors

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

// RustFeatures detects non-default Cargo features that gate significant code.
// Cargo features are compile-time flags that enable optional functionality.
// Non-default features are easy to miss — a new contributor might not know they
// need `cargo build --features grpc` to get full functionality.
//
// How it works: parse [features] from Cargo.toml to learn what exists and what's
// default, then walk .rs files for #[cfg(feature = "...")] to find which
// features actually gate code.
type RustFeatures struct{}

func (d RustFeatures) Name() string { return "rust-features" }

func (d RustFeatures) Detect(root string) ([]model.Section, error) {
	cargoPath := filepath.Join(root, "Cargo.toml")
	if _, err := os.Stat(cargoPath); os.IsNotExist(err) {
		return nil, nil
	}

	data, err := os.ReadFile(cargoPath)
	if err != nil {
		return nil, nil
	}

	allFeatures, defaultFeatures := parseCargoFeatures(string(data))
	if len(allFeatures) == 0 {
		return nil, nil
	}

	// Find features referenced in code via #[cfg(feature = "...")]
	usedFeatures := findCfgFeatures(root)

	// Non-default features that are actually used in code
	var nonDefault []string
	for _, f := range allFeatures {
		if !defaultFeatures[f] && usedFeatures[f] {
			nonDefault = append(nonDefault, f)
		}
	}
	if len(nonDefault) == 0 {
		return nil, nil
	}

	sort.Strings(nonDefault)

	return []model.Section{model.TextSection{
		Category: model.CatSurprise,
		Origin:   model.OriginAuto,
		Title:    "Non-Default Cargo Features",
		Body: fmt.Sprintf(
			"These Cargo features are not in the default set but gate code: %s. "+
				"Build with `cargo build --features %s` to enable them.",
			strings.Join(nonDefault, ", "),
			strings.Join(nonDefault, ","),
		),
		Source: d.Name(),
	}}, nil
}

// parseCargoFeatures extracts feature names and the default feature set from
// a Cargo.toml. Returns all feature names and a set of default feature names.
//
// Handles the [features] section format:
//   [features]
//   default = ["foo", "bar"]
//   grpc = ["dep:tonic"]
//   metrics = []
func parseCargoFeatures(content string) (all []string, defaults map[string]bool) {
	defaults = make(map[string]bool)
	inFeatures := false

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)

		if trimmed == "[features]" {
			inFeatures = true
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			inFeatures = false
			continue
		}
		if !inFeatures {
			continue
		}

		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		if name == "" {
			continue
		}

		if name == "default" {
			// Parse the default list to know which features are enabled by default
			for _, quoted := range extractQuotedStrings(parts[1]) {
				defaults[quoted] = true
			}
			continue
		}

		all = append(all, name)
	}

	return all, defaults
}

// extractQuotedStrings pulls double-quoted strings from a line like `["foo", "bar"]`.
func extractQuotedStrings(s string) []string {
	re := regexp.MustCompile(`"([^"]+)"`)
	matches := re.FindAllStringSubmatch(s, -1)
	var result []string
	for _, m := range matches {
		result = append(result, m[1])
	}
	return result
}

// findCfgFeatures walks .rs files for #[cfg(feature = "...")] attributes.
var cfgFeatureRe = regexp.MustCompile(`#\[cfg\(feature\s*=\s*"([^"]+)"\)\]`)

func findCfgFeatures(root string) map[string]bool {
	found := make(map[string]bool)

	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "target" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(d.Name()) != ".rs" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		for _, m := range cfgFeatureRe.FindAllStringSubmatch(string(data), -1) {
			if len(m) >= 2 {
				found[m[1]] = true
			}
		}
		return nil
	})

	return found
}
