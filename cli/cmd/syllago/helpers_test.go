package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/output"
)

func TestFindProjectRootFallbackWarning(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(string)
		wantWarning bool
	}{
		{
			name: "with go.mod marker",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)
			},
			wantWarning: false,
		},
		{
			name: "with package.json marker",
			setup: func(dir string) {
				os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644)
			},
			wantWarning: false,
		},
		{
			name:        "no project markers - fallback to cwd",
			setup:       func(dir string) {},
			wantWarning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			tt.setup(tmp)

			var stderr bytes.Buffer
			origErr := output.ErrWriter
			output.ErrWriter = &stderr
			defer func() { output.ErrWriter = origErr }()

			// Bound the walk to tmp so an ambient marker outside the test
			// sandbox (e.g. a stray /tmp/go.mod from another tool) cannot
			// silently satisfy the search and suppress the expected warning.
			root, err := findProjectRootFrom(tmp, tmp)
			if err != nil {
				t.Fatalf("findProjectRootFrom failed: %v", err)
			}

			stderrStr := stderr.String()
			hasWarning := strings.Contains(stderrStr, "Warning")

			if tt.wantWarning && !hasWarning {
				t.Error("expected warning but got none")
			}
			if !tt.wantWarning && hasWarning {
				t.Errorf("unexpected warning: %s", stderrStr)
			}

			// Should still return valid path
			if root == "" {
				t.Error("expected non-empty root path")
			}
		})
	}
}
