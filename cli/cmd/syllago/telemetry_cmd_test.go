package main

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
)

func TestTelemetryStatusCmd_HumanReadable(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	overrideTelemetryHome(t)

	if err := telemetryStatusCmd.RunE(telemetryStatusCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Telemetry:") {
		t.Errorf("missing Telemetry: line; got:\n%s", out)
	}
	if !strings.Contains(out, "Anonymous ID:") {
		t.Errorf("missing Anonymous ID: line; got:\n%s", out)
	}
	if !strings.Contains(out, "https://syllago.dev/telemetry") {
		t.Errorf("missing docs URL; got:\n%s", out)
	}
}

func TestTelemetryStatusCmd_JSON(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	output.JSON = true
	overrideTelemetryHome(t)

	if err := telemetryStatusCmd.RunE(telemetryStatusCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, `"enabled"`) {
		t.Errorf("JSON missing enabled field; got:\n%s", out)
	}
	if !strings.Contains(out, `"anonymousId"`) {
		t.Errorf("JSON missing anonymousId field; got:\n%s", out)
	}
}

func TestTelemetryOnCmd(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	overrideTelemetryHome(t)

	if err := telemetryOffCmd.RunE(telemetryOffCmd, nil); err != nil {
		t.Fatalf("off command failed: %v", err)
	}
	if err := telemetryOnCmd.RunE(telemetryOnCmd, nil); err != nil {
		t.Fatalf("on command failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "enabled") {
		t.Errorf("expected 'enabled' in output; got: %s", stdout.String())
	}

	cfg := telemetry.Status()
	if !cfg.Enabled {
		t.Error("expected telemetry enabled after 'on'")
	}
}

func TestTelemetryOffCmd(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	overrideTelemetryHome(t)

	if err := telemetryOffCmd.RunE(telemetryOffCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "disabled") {
		t.Errorf("expected 'disabled' in output; got: %s", stdout.String())
	}

	cfg := telemetry.Status()
	if cfg.Enabled {
		t.Error("expected telemetry disabled after 'off'")
	}
}

func TestTelemetryResetCmd(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	overrideTelemetryHome(t)

	// Seed initial config.
	telemetry.SetEnabled(true)
	before := telemetry.Status()

	if err := telemetryResetCmd.RunE(telemetryResetCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "rotated") {
		t.Errorf("missing 'rotated' in output; got: %s", out)
	}
	if !strings.Contains(out, "not deleted") {
		t.Errorf("missing deletion note in output; got: %s", out)
	}

	after := telemetry.Status()
	if after.AnonymousID == before.AnonymousID {
		t.Error("ID should change after reset")
	}
}

func TestTelemetryResetCmd_JSON(t *testing.T) {
	stdout, _ := output.SetForTest(t)
	output.JSON = true
	overrideTelemetryHome(t)

	if err := telemetryResetCmd.RunE(telemetryResetCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), `"anonymousId"`) {
		t.Errorf("JSON missing anonymousId; got: %s", stdout.String())
	}
}

// overrideTelemetryHome creates a temp dir and wires it as the home dir for
// the telemetry package for the duration of this test.
func overrideTelemetryHome(t *testing.T) {
	t.Helper()
	temp := t.TempDir()
	orig := telemetry.UserHomeDirFn
	telemetry.UserHomeDirFn = func() (string, error) { return temp, nil }
	t.Cleanup(func() { telemetry.UserHomeDirFn = orig })
}
