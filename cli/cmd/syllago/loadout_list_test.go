package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// setupLoadoutRepo creates a temp syllago repo with loadout content.
// Loadouts are provider-specific, so the structure is:
//
//	<root>/loadouts/<provider>/<name>/loadout.yaml
//
// The provider and name parameters are encoded in the key as "provider/name".
// Returns the repo root path.
func setupLoadoutRepo(t *testing.T, loadouts map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for key, manifest := range loadouts {
		// key is "provider/name"
		parts := strings.SplitN(key, "/", 2)
		if len(parts) != 2 {
			t.Fatalf("setupLoadoutRepo key must be provider/name, got %q", key)
		}
		prov, name := parts[0], parts[1]
		dir := filepath.Join(root, "loadouts", prov, name)
		os.MkdirAll(dir, 0755)
		if manifest != "" {
			os.WriteFile(filepath.Join(dir, "loadout.yaml"), []byte(manifest), 0644)
		}
	}
	return root
}

func TestRunLoadoutList(t *testing.T) {
	validManifest := `kind: loadout
version: 1
name: my-loadout
description: A test loadout
provider: claude-code
rules:
  - my-rule
skills:
  - my-skill
`

	tests := []struct {
		name       string
		loadouts   map[string]string
		jsonOutput bool
		wantErr    bool
		check      func(t *testing.T, out string)
	}{
		{
			name: "valid manifest shows table row",
			loadouts: map[string]string{
				"claude-code/my-loadout": validManifest,
			},
			check: func(t *testing.T, out string) {
				t.Helper()
				if !strings.Contains(out, "my-loadout") {
					t.Errorf("expected loadout name in output, got:\n%s", out)
				}
				if !strings.Contains(out, "NAME") {
					t.Errorf("expected table header, got:\n%s", out)
				}
				if !strings.Contains(out, "A test loadout") {
					t.Errorf("expected description in output, got:\n%s", out)
				}
			},
		},
		{
			name:     "empty loadout directory",
			loadouts: map[string]string{},
			check: func(t *testing.T, out string) {
				t.Helper()
				if !strings.Contains(out, "No loadouts found") {
					t.Errorf("expected 'No loadouts found', got: %s", out)
				}
			},
		},
		{
			name: "valid manifest JSON output",
			loadouts: map[string]string{
				"claude-code/my-loadout": validManifest,
			},
			jsonOutput: true,
			check: func(t *testing.T, out string) {
				t.Helper()
				var entries []loadoutListEntry
				if err := json.Unmarshal([]byte(out), &entries); err != nil {
					t.Fatalf("failed to parse JSON: %v\noutput: %s", err, out)
				}
				if len(entries) != 1 {
					t.Fatalf("expected 1 entry, got %d", len(entries))
				}
				if entries[0].Name != "my-loadout" {
					t.Errorf("name = %q, want %q", entries[0].Name, "my-loadout")
				}
				if entries[0].ItemCount != 2 {
					t.Errorf("itemCount = %d, want 2", entries[0].ItemCount)
				}
			},
		},
		{
			name:       "empty JSON output (no loadouts)",
			loadouts:   map[string]string{},
			jsonOutput: true,
			check: func(t *testing.T, out string) {
				t.Helper()
				// null or empty array are both acceptable for empty slice JSON
				out = strings.TrimSpace(out)
				if out != "null" && out != "[]" {
					t.Errorf("expected null or [], got: %s", out)
				}
			},
		},
		{
			name: "corrupt manifest still lists item with zero count",
			loadouts: map[string]string{
				"claude-code/bad-loadout": "this is not valid yaml: [[[",
			},
			check: func(t *testing.T, out string) {
				t.Helper()
				// Item should appear but with 0 items since manifest parse fails
				if !strings.Contains(out, "bad-loadout") {
					t.Error("expected loadout name in output even with corrupt manifest")
				}
				if !strings.Contains(out, "0") {
					t.Error("expected 0 item count for corrupt manifest")
				}
			},
		},
		{
			name: "missing manifest (no loadout.yaml) shows zero count",
			loadouts: map[string]string{
				"claude-code/no-manifest": "", // setupLoadoutRepo skips writing file when empty
			},
			check: func(t *testing.T, out string) {
				t.Helper()
				if !strings.Contains(out, "no-manifest") {
					t.Error("expected loadout name in output")
				}
			},
		},
		{
			name: "long description is truncated",
			loadouts: map[string]string{
				"claude-code/long-desc": `kind: loadout
version: 1
name: long-desc
description: This is a very long description that exceeds the fifty character limit for table display
rules:
  - a-rule
`,
			},
			check: func(t *testing.T, out string) {
				t.Helper()
				if !strings.Contains(out, "...") {
					t.Error("expected truncated description with ellipsis")
				}
			},
		},
		{
			name: "multiple loadouts listed",
			loadouts: map[string]string{
				"claude-code/alpha": `kind: loadout
version: 1
name: alpha
description: First loadout
rules:
  - rule-a
`,
				"claude-code/beta": `kind: loadout
version: 1
name: beta
description: Second loadout
skills:
  - skill-b
  - skill-c
`,
			},
			check: func(t *testing.T, out string) {
				t.Helper()
				if !strings.Contains(out, "alpha") {
					t.Error("expected alpha in output")
				}
				if !strings.Contains(out, "beta") {
					t.Error("expected beta in output")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Not parallel — mutates globals (findProjectRoot, output.JSON)
			root := setupLoadoutRepo(t, tt.loadouts)
			stdout, _ := output.SetForTest(t)
			withFakeRepoRoot(t, root)

			if tt.jsonOutput {
				output.JSON = true
			}

			err := loadoutListCmd.RunE(loadoutListCmd, []string{})

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			tt.check(t, stdout.String())
		})
	}
}
