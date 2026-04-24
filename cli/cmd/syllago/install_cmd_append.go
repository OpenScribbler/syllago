package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installcheck"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/rulestore"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
	"github.com/spf13/cobra"
)

// D17 exact error strings — tests assert these literal substrings, and they
// are the user-facing contract for the non-interactive re-install flow.
const (
	errCleanStateNeedsFlag    = "rule already installed at clean state; specify --on-clean=replace|skip"
	errModifiedStateNeedsFlag = "rule install record is stale; specify --on-modified=drop-record|append-fresh|keep"
)

// runInstallAppend handles `syllago install <name> --to <slug> --method=append`.
// It installs a rule by appending its canonical body to the provider's first
// monolithic filename (D5, D10, D14). Target file scope is project by default
// — per-entry `Scope` in installed.json is resolved from the target path vs
// home/project roots by `installer.ResolveAppendScope`.
//
// Library rules are loaded directly via rulestore.LoadRule rather than
// catalog.Scan because rulestore's D13 .syllago.yaml shape (nested source:)
// differs from catalog's Meta schema. Catalog integration for library rules
// is its own follow-up.
func runInstallAppend(cmd *cobra.Command, args []string, toSlug, typeFilter string) error {
	// --method=append only applies to rules.
	if typeFilter != "" && typeFilter != string(catalog.Rules) {
		return output.NewStructuredError(
			output.ErrInputConflict,
			"--method=append only supports --type rules",
			"Rerun with --type rules or use --method symlink / copy.",
		)
	}

	monoNames := provider.MonolithicFilenames(toSlug)
	if len(monoNames) == 0 {
		return output.NewStructuredError(
			output.ErrInputConflict,
			fmt.Sprintf("provider %s does not have a monolithic rule filename", toSlug),
			"Use --method symlink (default) or --method copy to install into a file-rules directory.",
		)
	}

	projectRoot, err := findProjectRoot()
	if err != nil {
		return output.NewStructuredErrorDetail(
			output.ErrSystemHomedir,
			"locating project root",
			"Run from within a project directory or pass --base-dir.",
			err.Error(),
		)
	}

	homeDir, _ := os.UserHomeDir()

	globalDir := catalog.GlobalContentDir()
	if globalDir == "" {
		return output.NewStructuredError(output.ErrSystemHomedir, "cannot determine home directory", "Set the HOME environment variable")
	}
	rulesRoot := filepath.Join(globalDir, string(catalog.Rules))

	// Enumerate library rule directories under rulesRoot/<sourceProvider>/<slug>.
	// Without a positional name, --method=append refuses bulk operations —
	// require an explicit target to avoid accidentally appending every rule
	// into a single file.
	if len(args) != 1 {
		return output.NewStructuredError(
			output.ErrInputMissing,
			"--method=append requires a rule name",
			"Example: syllago install my-rule --to claude-code --method=append",
		)
	}
	ruleDir, err := findLibraryRuleDir(rulesRoot, args[0])
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return output.NewStructuredError(
				output.ErrInstallItemNotFound,
				fmt.Sprintf("no rule named %q found in your library", args[0]),
				"Hint: syllago list --type rules",
			)
		}
		return output.NewStructuredErrorDetail(
			output.ErrCatalogScanFailed,
			"locating library rule",
			"Check file permissions in ~/.syllago/content/rules/",
			err.Error(),
		)
	}

	loaded, lerr := rulestore.LoadRule(ruleDir)
	if lerr != nil {
		return output.NewStructuredErrorDetail(
			output.ErrCatalogScanFailed,
			"loading library rule",
			"The library rule directory is malformed; check .syllago.yaml and .history/.",
			lerr.Error(),
		)
	}

	target := filepath.Join(projectRoot, monoNames[0])

	// D17 routing: verify first, then branch on (State, Reason). The library
	// map is a single-entry map keyed by the loaded rule's LibraryID — scan
	// cross-references installed.json by that ID.
	library := map[string]*rulestore.Loaded{loaded.Meta.ID: loaded}
	inst, ierr := installer.LoadInstalled(projectRoot)
	if ierr != nil {
		return output.NewStructuredErrorDetail(
			output.ErrCatalogScanFailed,
			"loading installed records",
			"Check ~/.syllago/installed.json or project .syllago/installed.json",
			ierr.Error(),
		)
	}
	scan := installcheck.Scan(inst, library)
	key := installcheck.RecordKey{LibraryID: loaded.Meta.ID, TargetFile: target}
	pts, hasRecord := scan.PerRecord[key]

	onClean, _ := cmd.Flags().GetString("on-clean")
	onModified, _ := cmd.Flags().GetString("on-modified")

	// stateString / actionString are captured for telemetry enrichment after
	// the decision branch; default to "fresh"/"proceed" so every branch
	// produces a valid telemetry pair even if the branch returns early.
	stateString := "fresh"
	actionString := "proceed"

	installed := []installedItem{} // populated by the branch that mutates

	switch {
	case !hasRecord || pts.State == installcheck.StateFresh:
		// Fresh install — no record or no clean match.
		if err := installer.InstallRuleAppend(projectRoot, homeDir, toSlug, target, "manual", loaded); err != nil {
			return output.NewStructuredErrorDetail(
				output.ErrInstallNotWritable,
				"appending rule to target file",
				"Check write permissions on the target file.",
				err.Error(),
			)
		}
		installed = append(installed, installedItem{
			Name:   loaded.Meta.Name,
			Type:   string(catalog.Rules),
			Method: "append",
			Path:   target,
		})
		if !output.JSON && !output.Quiet {
			fmt.Fprintf(output.Writer, "  Appended %s to %s\n", loaded.Meta.Name, target)
		}

	case pts.State == installcheck.StateClean:
		stateString = "clean"
		switch onClean {
		case "":
			return output.NewStructuredError(
				output.ErrInputConflict,
				errCleanStateNeedsFlag,
				"Replace rewrites the block in place; Skip leaves the file unchanged.",
			)
		case "skip":
			actionString = "skip"
			if !output.JSON && !output.Quiet {
				fmt.Fprintln(output.Writer, "skipping: already installed")
			}
		case "replace":
			actionString = "replace"
			// Ensure the new body is recorded as a version in the library so
			// ReplaceRuleAppend's history lookup can find it later. No-op when
			// the current version is already present.
			newBody := loaded.History[loaded.Meta.CurrentVersion]
			if aerr := rulestore.AppendVersion(ruleDir, newBody); aerr != nil {
				return output.NewStructuredErrorDetail(
					output.ErrInstallNotWritable,
					"updating library rule history",
					"Check write permissions on ~/.syllago/content/rules/",
					aerr.Error(),
				)
			}
			if rerr := installer.ReplaceRuleAppend(projectRoot, loaded.Meta.ID, target, newBody, library); rerr != nil {
				return output.NewStructuredErrorDetail(
					output.ErrInstallNotWritable,
					"replacing rule block in target file",
					"Check write permissions on the target file.",
					rerr.Error(),
				)
			}
			installed = append(installed, installedItem{
				Name:   loaded.Meta.Name,
				Type:   string(catalog.Rules),
				Method: "append",
				Path:   target,
			})
			if !output.JSON && !output.Quiet {
				fmt.Fprintf(output.Writer, "  Replaced %s in %s\n", loaded.Meta.Name, target)
			}
		}

	case pts.State == installcheck.StateModified:
		stateString = "modified"
		switch onModified {
		case "":
			return output.NewStructuredError(
				output.ErrInputConflict,
				errModifiedStateNeedsFlag,
				"Drop-record clears the stale record; Append-fresh appends a fresh copy; Keep leaves everything as-is.",
			)
		case "keep":
			actionString = "keep"
			if !output.JSON && !output.Quiet {
				fmt.Fprintln(output.Writer, "keeping: install record and file left unchanged")
			}
		case "drop-record":
			actionString = "drop_record"
			idx := inst.FindRuleAppend(loaded.Meta.ID, target)
			if idx >= 0 {
				inst.RemoveRuleAppend(idx)
				if serr := installer.SaveInstalled(projectRoot, inst); serr != nil {
					return output.NewStructuredErrorDetail(
						output.ErrInstallNotWritable,
						"saving installed records",
						"Check write permissions on .syllago/installed.json",
						serr.Error(),
					)
				}
			}
			if !output.JSON && !output.Quiet {
				fmt.Fprintf(output.Writer, "  Dropped stale install record for %s\n", loaded.Meta.Name)
			}
		case "append-fresh":
			actionString = "append_fresh"
			// D20's AppendRuleToTarget creates the file if missing and handles
			// the non-empty newline rules. InstallRuleAppend also appends a
			// new record, so drop the stale one first to preserve D14's
			// (LibraryID, TargetFile) uniqueness.
			if idx := inst.FindRuleAppend(loaded.Meta.ID, target); idx >= 0 {
				inst.RemoveRuleAppend(idx)
				if serr := installer.SaveInstalled(projectRoot, inst); serr != nil {
					return output.NewStructuredErrorDetail(
						output.ErrInstallNotWritable,
						"saving installed records",
						"Check write permissions on .syllago/installed.json",
						serr.Error(),
					)
				}
			}
			if err := installer.InstallRuleAppend(projectRoot, homeDir, toSlug, target, "manual", loaded); err != nil {
				return output.NewStructuredErrorDetail(
					output.ErrInstallNotWritable,
					"appending rule to target file",
					"Check write permissions on the target file.",
					err.Error(),
				)
			}
			installed = append(installed, installedItem{
				Name:   loaded.Meta.Name,
				Type:   string(catalog.Rules),
				Method: "append",
				Path:   target,
			})
			if !output.JSON && !output.Quiet {
				fmt.Fprintf(output.Writer, "  Appended fresh copy of %s to %s\n", loaded.Meta.Name, target)
			}
		}
	}

	// D10: print per-provider hint once to stderr when present.
	if hint := provider.MonolithicHint(toSlug); hint != "" && !output.JSON && !output.Quiet {
		fmt.Fprintf(output.ErrWriter, "NOTE: %s\n", hint)
	}

	result := installResult{Installed: installed}
	if output.JSON {
		output.Print(result)
	}

	telemetry.Enrich("provider", toSlug)
	telemetry.Enrich("content_type", string(catalog.Rules))
	telemetry.Enrich("content_count", len(result.Installed))
	telemetry.Enrich("verification_state", stateString)
	telemetry.Enrich("decision_action", actionString)
	return nil
}

// findLibraryRuleDir locates <rulesRoot>/*/<name>/ by iterating source-provider
// subdirectories. Returns fs.ErrNotExist if no match is found. First-match
// wins when the same rule name exists under multiple source providers — D14
// uniqueness is enforced per (LibraryID, TargetFile), not per name.
func findLibraryRuleDir(rulesRoot, name string) (string, error) {
	entries, err := os.ReadDir(rulesRoot)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		candidate := filepath.Join(rulesRoot, e.Name(), name)
		if info, serr := os.Stat(candidate); serr == nil && info.IsDir() {
			return candidate, nil
		}
	}
	return "", fs.ErrNotExist
}
