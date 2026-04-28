package main

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// TestShouldPromptConsent_Branches covers every branch of shouldPromptConsent
// against a synthetic command tree. The function gates a user-visible prompt,
// so a regression in any branch either spams the prompt during scripting or
// silently skips it for real users — both bad outcomes that won't surface in
// any other test.
//
// The TTY check is indirected through isInteractiveStdinStderrFn so we can
// drive both arms of that branch without spawning a pty.
func TestShouldPromptConsent_Branches(t *testing.T) {
	// Build a command tree mirroring the real one in shape but isolated
	// from the global rootCmd: root → install (leaf), root → telemetry →
	// {on, off, status, reset}, root → version, root → help.
	root := &cobra.Command{Use: "syllago-test"}
	install := &cobra.Command{Use: "install"}
	versionLeaf := &cobra.Command{Use: "version"}
	helpLeaf := &cobra.Command{Use: "help"}
	tel := &cobra.Command{Use: "telemetry"}
	telOn := &cobra.Command{Use: "on"}
	telStatus := &cobra.Command{Use: "status"}
	tel.AddCommand(telOn, telStatus)
	root.AddCommand(install, versionLeaf, helpLeaf, tel)

	// Override the TTY check for the duration of this test. Default to
	// "interactive" so each subtest can opt out by setting it to false.
	origTTY := isInteractiveStdinStderrFn
	t.Cleanup(func() { isInteractiveStdinStderrFn = origTTY })

	// output.JSON is a package global; restore after each subtest.
	origJSON := output.JSON
	t.Cleanup(func() { output.JSON = origJSON })

	tests := []struct {
		name        string
		cmd         *cobra.Command
		quiet       bool
		jsonOutput  bool
		interactive bool
		// special: when set, treat cmd as the live rootCmd. This is the
		// only branch that needs the real package-level rootCmd identity.
		useRealRoot bool
		want        bool
	}{
		{
			name: "nil_command_returns_false",
			cmd:  nil,
			want: false,
		},
		{
			name:        "real_root_command_skips_prompt_TUI_handles_it",
			useRealRoot: true,
			interactive: true,
			want:        false,
		},
		{
			name:        "telemetry_parent_skips_prompt",
			cmd:         tel,
			interactive: true,
			want:        false,
		},
		{
			name:        "telemetry_on_subcommand_skips_prompt",
			cmd:         telOn,
			interactive: true,
			want:        false,
		},
		{
			name:        "telemetry_status_subcommand_skips_prompt",
			cmd:         telStatus,
			interactive: true,
			want:        false,
		},
		{
			name:        "version_skips_prompt",
			cmd:         versionLeaf,
			interactive: true,
			want:        false,
		},
		{
			name:        "help_skips_prompt",
			cmd:         helpLeaf,
			interactive: true,
			want:        false,
		},
		{
			name:        "json_output_skips_prompt",
			cmd:         install,
			jsonOutput:  true,
			interactive: true,
			want:        false,
		},
		{
			name:        "quiet_flag_skips_prompt",
			cmd:         install,
			quiet:       true,
			interactive: true,
			want:        false,
		},
		{
			name:        "non_interactive_skips_prompt",
			cmd:         install,
			interactive: false,
			want:        false,
		},
		{
			name: "interactive_normal_command_prompts",
			// All gates clear: regular leaf command, not in scripted
			// mode, with a TTY on both ends. This is the one branch that
			// returns true.
			cmd:         install,
			interactive: true,
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interactive := tt.interactive
			isInteractiveStdinStderrFn = func() bool { return interactive }
			output.JSON = tt.jsonOutput

			cmd := tt.cmd
			if tt.useRealRoot {
				cmd = rootCmd
			}

			got := shouldPromptConsent(cmd, tt.quiet)
			if got != tt.want {
				t.Errorf("shouldPromptConsent(%v, quiet=%v, json=%v, interactive=%v) = %v, want %v",
					cmdName(cmd), tt.quiet, tt.jsonOutput, interactive, got, tt.want)
			}
		})
	}
}

func cmdName(c *cobra.Command) string {
	if c == nil {
		return "<nil>"
	}
	return c.CommandPath()
}
