package detectors

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

// RustUnsafe detects a Rust project's stance on unsafe code. It checks for
// #![forbid(unsafe_code)] or #![deny(unsafe_code)] in lib.rs/main.rs (crate-
// level attributes that apply globally), and also counts `unsafe` blocks across
// all .rs files.
//
// Why this matters: unsafe code bypasses Rust's safety guarantees. Knowing
// whether a project forbids it entirely, allows it sparingly, or uses it heavily
// is critical context for contributors.
type RustUnsafe struct{}

func (d RustUnsafe) Name() string { return "rust-unsafe" }

func (d RustUnsafe) Detect(root string) ([]model.Section, error) {
	if _, err := os.Stat(filepath.Join(root, "Cargo.toml")); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}

	// Check crate-level unsafe stance in lib.rs and main.rs
	stance := checkUnsafeStance(root)

	// Count unsafe blocks across all .rs files
	unsafeCount := countUnsafeBlocks(root)

	if stance == "" && unsafeCount == 0 {
		return nil, nil
	}

	var body string
	switch {
	case stance == "forbid":
		body = "This project uses #![forbid(unsafe_code)] — unsafe blocks will cause a compile error."
	case stance == "deny":
		body = fmt.Sprintf("This project uses #![deny(unsafe_code)] but can be overridden with #[allow(unsafe_code)]. Found %d unsafe block(s) across the codebase.", unsafeCount)
	case unsafeCount > 0:
		body = fmt.Sprintf("No crate-level unsafe policy found. There are %d unsafe block(s) across the codebase.", unsafeCount)
	default:
		body = "No crate-level unsafe policy and no unsafe blocks found — the project appears to be safe Rust only."
	}

	return []model.Section{model.TextSection{
		Category: model.CatSurprise,
		Origin:   model.OriginAuto,
		Title:    "Rust Unsafe Code Policy",
		Body:     body,
		Source:   d.Name(),
	}}, nil
}

// unsafeStanceRe matches #![forbid(unsafe_code)] or #![deny(unsafe_code)]
var unsafeStanceRe = regexp.MustCompile(`#!\[(forbid|deny)\(unsafe_code\)\]`)

// checkUnsafeStance looks for crate-level unsafe attributes in lib.rs and main.rs.
// Returns "forbid", "deny", or "" if neither is found.
func checkUnsafeStance(root string) string {
	for _, name := range []string{"src/lib.rs", "src/main.rs", "lib.rs", "main.rs"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			continue
		}
		m := unsafeStanceRe.FindStringSubmatch(string(data))
		if len(m) >= 2 {
			return m[1] // "forbid" or "deny"
		}
	}
	return ""
}

// unsafeBlockRe matches `unsafe {` or `unsafe fn` patterns.
var unsafeBlockRe = regexp.MustCompile(`\bunsafe\s*\{|\bunsafe\s+fn\b`)

// countUnsafeBlocks walks all .rs files and counts occurrences of unsafe usage.
func countUnsafeBlocks(root string) int {
	count := 0

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

		count += len(unsafeBlockRe.FindAllString(string(data), -1))
		return nil
	})

	return count
}
