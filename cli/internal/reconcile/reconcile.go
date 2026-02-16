package reconcile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Conflict represents a discrepancy between curated and scanned content.
type Conflict struct {
	Section string `json:"section"`
	Message string `json:"message"`
}

// Result holds the reconciliation output.
type Result struct {
	Output    string     `json:"output"`
	Conflicts []Conflict `json:"conflicts,omitempty"`
}

// Reconcile merges new emitter output with an existing file.
//   - Auto sections are replaced with new output
//   - Human sections are preserved verbatim
//   - Unmarked content is preserved
//   - New auto sections are appended
func Reconcile(existingContent, newEmitterOutput string, format MarkerFormat) Result {
	if existingContent == "" {
		return Result{Output: newEmitterOutput}
	}

	existing := Parse(existingContent, format)
	fresh := Parse(newEmitterOutput, format)

	// Build a map of new auto sections by name
	freshAutoMap := make(map[string]Section)
	for _, s := range fresh.Sections {
		if s.IsAuto {
			freshAutoMap[s.Name] = s
		}
	}

	used := make(map[string]bool)

	var b strings.Builder

	// Output leading unmarked content
	for _, u := range existing.Unmarked {
		if u.Index == 0 {
			b.WriteString(u.Content)
			b.WriteString("\n")
		}
	}

	for _, s := range existing.Sections {
		if s.IsAuto {
			if freshSection, ok := freshAutoMap[s.Name]; ok {
				writeSection(&b, freshSection, format)
				used[s.Name] = true
			}
			// If not in fresh output, section was removed — don't emit
		} else if s.IsHuman {
			writeSection(&b, s, format)
		}
	}

	// Append new auto sections that weren't in the existing file
	for _, s := range fresh.Sections {
		if s.IsAuto && !used[s.Name] {
			writeSection(&b, s, format)
		}
	}

	return Result{Output: b.String()}
}

func writeSection(b *strings.Builder, s Section, format MarkerFormat) {
	if format == FormatYAML {
		b.WriteString(fmt.Sprintf("# nesco:%s\n", s.Name))
		b.WriteString(s.Content)
		b.WriteString(fmt.Sprintf("\n# /nesco:%s\n\n", s.Name))
	} else {
		b.WriteString(fmt.Sprintf("<!-- nesco:%s -->\n", s.Name))
		b.WriteString(s.Content)
		b.WriteString(fmt.Sprintf("\n<!-- /nesco:%s -->\n\n", s.Name))
	}
}

// ReconcileAndWrite performs reconciliation and writes the result to disk.
func ReconcileAndWrite(outputPath, newEmitterOutput string, format MarkerFormat) (Result, error) {
	existing, _ := os.ReadFile(outputPath) // ok if file doesn't exist

	result := Reconcile(string(existing), newEmitterOutput, format)

	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return result, err
	}

	return result, os.WriteFile(outputPath, []byte(result.Output), 0644)
}
