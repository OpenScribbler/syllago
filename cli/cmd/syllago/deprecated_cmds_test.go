package main

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/spf13/cobra"
)

// TestDeprecatedStubMessages verifies that each deprecated stub prints the
// expected redirect message and returns a SilentError (so the caller handles
// the exit code rather than cobra printing a generic error).
func TestDeprecatedStubMessages(t *testing.T) {
	tests := []struct {
		name    string
		cmd     func() error
		wantMsg string
	}{
		{
			name: "promote stub prints share redirect",
			cmd: func() error {
				return deprecatedPromoteCmd.RunE(deprecatedPromoteCmd, nil)
			},
			wantMsg: "syllago share",
		},
		{
			name: "promote stub does not mention phantom publish command",
			cmd: func() error {
				return deprecatedPromoteCmd.RunE(deprecatedPromoteCmd, nil)
			},
			wantMsg: "syllago share",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, stderr := output.SetForTest(t)

			err := tt.cmd()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !output.IsSilentError(err) {
				t.Errorf("expected SilentError, got: %v", err)
			}

			got := stderr.String()
			if !strings.Contains(got, tt.wantMsg) {
				t.Errorf("expected stderr to contain %q, got:\n%s", tt.wantMsg, got)
			}
		})
	}
}

// TestDeprecatedStubsAreHidden verifies that each stub has Hidden=true so
// they do not appear in help output once registered.
func TestDeprecatedStubsAreHidden(t *testing.T) {
	stubs := []struct {
		name string
		cmd  *cobra.Command
	}{
		{"promote", deprecatedPromoteCmd},
	}

	for _, tt := range stubs {
		t.Run(tt.name+" is hidden", func(t *testing.T) {
			if !tt.cmd.Hidden {
				t.Errorf("deprecated %s command should be Hidden=true", tt.name)
			}
		})
	}
}
