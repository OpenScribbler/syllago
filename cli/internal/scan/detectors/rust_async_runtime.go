package detectors

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

// RustAsyncRuntime detects which async runtime a Rust project uses. The Rust
// ecosystem has multiple competing async runtimes (tokio, async-std, smol) that
// are not interchangeable. Knowing the canonical runtime tells contributors
// which executor, channels, and IO primitives to use.
//
// How it works: check Cargo.toml [dependencies] for known runtimes, then scan
// .rs files for runtime-specific entry point attributes like #[tokio::main].
type RustAsyncRuntime struct{}

func (d RustAsyncRuntime) Name() string { return "rust-async-runtime" }

func (d RustAsyncRuntime) Detect(root string) ([]model.Section, error) {
	cargoPath := filepath.Join(root, "Cargo.toml")
	if _, err := os.Stat(cargoPath); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}

	data, err := os.ReadFile(cargoPath)
	if err != nil {
		return nil, nil
	}

	runtimes := detectAsyncRuntimes(string(data))
	entryPoints := findAsyncEntryPoints(root)

	// Merge: a runtime counts if it's in deps or found as an entry point attr
	allRuntimes := make(map[string]bool)
	for _, r := range runtimes {
		allRuntimes[r] = true
	}
	for _, r := range entryPoints {
		allRuntimes[r] = true
	}

	if len(allRuntimes) == 0 {
		return nil, nil
	}

	var names []string
	for r := range allRuntimes {
		names = append(names, r)
	}
	sort.Strings(names)

	var body string
	if len(names) == 1 {
		body = fmt.Sprintf("This project uses %s as its async runtime.", names[0])
	} else {
		body = fmt.Sprintf("Multiple async runtimes detected: %s. This can cause compatibility issues — async runtimes are generally not interchangeable.", strings.Join(names, ", "))
	}

	return []model.Section{model.TextSection{
		Category: model.CatSurprise,
		Origin:   model.OriginAuto,
		Title:    "Rust Async Runtime",
		Body:     body,
		Source:   d.Name(),
	}}, nil
}

// knownAsyncRuntimes maps Cargo dependency names to their display names.
var knownAsyncRuntimes = map[string]string{
	"tokio":     "tokio",
	"async-std": "async-std",
	"smol":      "smol",
}

// detectAsyncRuntimes checks Cargo.toml [dependencies] for known async runtimes.
func detectAsyncRuntimes(content string) []string {
	inDeps := false
	var found []string

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[dependencies]" || trimmed == "[dev-dependencies]" {
			inDeps = true
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			inDeps = false
			continue
		}
		if !inDeps || !strings.Contains(trimmed, "=") {
			continue
		}

		name := strings.TrimSpace(strings.SplitN(trimmed, "=", 2)[0])
		if display, ok := knownAsyncRuntimes[name]; ok {
			found = append(found, display)
		}
	}

	return found
}

// asyncEntryRe matches #[tokio::main], #[async_std::main], etc.
var asyncEntryRe = regexp.MustCompile(`#\[(tokio|async_std)::main\]`)

// findAsyncEntryPoints scans .rs files for runtime-specific main attributes.
func findAsyncEntryPoints(root string) []string {
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

		for _, m := range asyncEntryRe.FindAllStringSubmatch(string(data), -1) {
			if len(m) >= 2 {
				// Normalize: async_std -> async-std for display
				name := strings.ReplaceAll(m[1], "_", "-")
				found[name] = true
			}
		}
		return nil
	})

	var result []string
	for r := range found {
		result = append(result, r)
	}
	return result
}
