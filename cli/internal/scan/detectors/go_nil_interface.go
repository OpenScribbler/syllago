package detectors

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

// GoNilInterface detects a subtle Go bug: returning a typed nil through an
// interface. When a function returns an error interface and the code does
// `var err *MyError; return err`, the returned interface is non-nil (it has a
// type but nil value). This means `if err != nil` is true even though the
// underlying pointer is nil — a classic Go gotcha.
//
// This is a heuristic, not a full static analysis. It looks for functions
// returning `error` where a `var x *SomeType` declaration is followed by
// `return x` (or `return ..., x`).
type GoNilInterface struct{}

func (d GoNilInterface) Name() string { return "go-nil-interface" }

// varPointerPattern matches `var <name> *<Type>`.
var varPointerPattern = regexp.MustCompile(`var\s+(\w+)\s+\*\w+`)

func (d GoNilInterface) Detect(root string) ([]model.Section, error) {
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return nil, nil
	}

	var findings []string

	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		hits := findTypedNilReturns(string(data))
		if len(hits) > 0 {
			rel, _ := filepath.Rel(root, path)
			for _, h := range hits {
				findings = append(findings, fmt.Sprintf("%s: var %s (typed nil returned as error interface)", rel, h))
			}
		}
		return nil
	})

	if len(findings) == 0 {
		return nil, nil
	}

	return []model.Section{
		model.TextSection{
			Category: model.CatSurprise,
			Origin:   model.OriginAuto,
			Title:    "Go Typed Nil Interface Returns",
			Body:     fmt.Sprintf("Potential typed-nil interface bug(s): %s. A *T nil assigned to an interface is non-nil — `if err != nil` will be true.", strings.Join(findings, "; ")),
			Source:   d.Name(),
		},
	}, nil
}

// findTypedNilReturns scans Go source for functions returning error where a
// pointer-typed variable is declared and then returned without assignment.
// Returns the variable names that match the pattern.
func findTypedNilReturns(src string) []string {
	lines := strings.Split(src, "\n")
	var results []string

	// Track whether we're inside a function that returns error.
	inErrorFunc := false
	// Track declared pointer variables in the current function scope.
	var ptrVars map[string]bool
	braceDepth := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect function signatures that return error.
		if strings.HasPrefix(trimmed, "func ") && strings.Contains(trimmed, "error") {
			// Simple heuristic: function signature containing "error" in return.
			if idx := strings.LastIndex(trimmed, ")"); idx != -1 {
				after := trimmed[idx:]
				if strings.Contains(after, "error") {
					inErrorFunc = true
					ptrVars = make(map[string]bool)
					braceDepth = 0
				}
			}
		}

		// Track brace depth within the function.
		if inErrorFunc {
			braceDepth += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")

			// Look for var declarations with pointer types.
			if matches := varPointerPattern.FindStringSubmatch(trimmed); len(matches) >= 2 {
				ptrVars[matches[1]] = true
			}

			// Look for return statements that include a tracked pointer var.
			if strings.HasPrefix(trimmed, "return ") {
				returnExpr := strings.TrimPrefix(trimmed, "return ")
				// Split on comma to handle multi-value returns.
				parts := strings.Split(returnExpr, ",")
				for _, part := range parts {
					name := strings.TrimSpace(part)
					if ptrVars[name] {
						results = append(results, name)
					}
				}
			}

			// End of function.
			if braceDepth <= 0 && strings.Contains(trimmed, "}") {
				inErrorFunc = false
				ptrVars = nil
			}
		}
	}

	return results
}
