package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
	"github.com/OpenScribbler/syllago/cli/internal/capmon/capyaml"
	"github.com/spf13/cobra"
)

// capmonCapabilitiesDirOverride allows tests to redirect the verify command
// to a temp directory instead of the repo's docs/provider-capabilities/.
var capmonCapabilitiesDirOverride string

var capmonCmd = &cobra.Command{
	Use:   "capmon",
	Short: "Capability monitor pipeline",
	Long:  "Fetch, extract, diff, and report on AI provider capability drift.",
}

var capmonVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Validate provider-capabilities YAML against JSON Schema",
	RunE: func(cmd *cobra.Command, args []string) error {
		stalenessCheck, _ := cmd.Flags().GetBool("staleness-check")
		thresholdHours, _ := cmd.Flags().GetInt("threshold-hours")
		cacheRoot, _ := cmd.Flags().GetString("cache-root")
		migrationWindow, _ := cmd.Flags().GetBool("migration-window")
		if cacheRoot == "" {
			cacheRoot = ".capmon-cache"
		}

		// Staleness check: read last-run.json and open issue if stale or missing.
		if stalenessCheck {
			manifest, err := capmon.ReadLastRunManifest(cacheRoot)
			if err != nil || time.Since(manifest.FinishedAt) > time.Duration(thresholdHours)*time.Hour {
				reason := "last-run.json missing or unreadable"
				if err == nil {
					reason = fmt.Sprintf("last run was %.1f hours ago (threshold: %d)", time.Since(manifest.FinishedAt).Hours(), thresholdHours)
				}
				_, ghErr := capmon.GHRunner("issue", "create",
					"--title", "capmon: pipeline staleness detected",
					"--label", "capmon,staleness",
					"--body", fmt.Sprintf("Capability monitor pipeline appears stale. %s.", reason),
				)
				return ghErr
			}
			return nil
		}

		dir := capmonCapabilitiesDirOverride
		if dir == "" {
			dir = "docs/provider-capabilities"
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil // empty dir is valid
			}
			return fmt.Errorf("read capabilities dir: %w", err)
		}
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
				continue
			}
			path := filepath.Join(dir, e.Name())
			if err := capyaml.ValidateAgainstSchema(path, migrationWindow); err != nil {
				return fmt.Errorf("validate %s: %w", e.Name(), err)
			}
		}
		return nil
	},
}

func init() {
	capmonVerifyCmd.Flags().Bool("staleness-check", false, "Check last-run.json age and open issue if stale")
	capmonVerifyCmd.Flags().Int("threshold-hours", 36, "Hours before a run is considered stale (used with --staleness-check)")
	capmonVerifyCmd.Flags().String("cache-root", "", "Path to .capmon-cache/ (default: .capmon-cache)")
	capmonVerifyCmd.Flags().Bool("migration-window", false, "Accept current-minus-one schema_version during schema migrations")
	capmonCmd.AddCommand(capmonVerifyCmd)
}
