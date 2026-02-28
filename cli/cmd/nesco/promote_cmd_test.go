package main

import (
	"testing"
)

func TestPromoteCmdRegisters(t *testing.T) {
	// Verify the promote command is registered on rootCmd.
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "promote" {
			found = true

			// Verify to-registry subcommand is registered.
			subFound := false
			for _, sub := range cmd.Commands() {
				if sub.Name() == "to-registry" {
					subFound = true
					break
				}
			}
			if !subFound {
				t.Error("expected to-registry subcommand under promote")
			}
			break
		}
	}
	if !found {
		t.Error("expected promote command registered on rootCmd")
	}
}

func TestPromoteToRegistryValidatesArgs(t *testing.T) {
	// The command requires exactly 2 args. Verify it rejects wrong counts.
	promoteToRegistryCmd.SilenceUsage = true
	promoteToRegistryCmd.SilenceErrors = true

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no args", []string{}, true},
		{"one arg", []string{"my-registry"}, true},
		{"three args", []string{"my-registry", "skills/foo", "extra"}, true},
		// Two args is the correct count — will fail later in RunE (no repo),
		// but arg validation itself should pass.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := promoteToRegistryCmd.Args(promoteToRegistryCmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Args(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
		})
	}
}
