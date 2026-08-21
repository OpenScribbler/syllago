package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/doctor"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check your syllago setup for problems",
	Long: `Validates your syllago installation: library, config, providers,
installed content integrity, and registry configuration.

Each check reports [ok], [warn], or [err]. Exit code is 0 if all checks
pass, 1 if there are warnings, 2 if there are errors.`,
	Example: `  syllago doctor
  syllago doctor --json`,
	RunE: runDoctor,
}

func init() {
	doctorCmd.Flags().Bool("fix", false, "Repair broken provider links: re-link to library content or prune dead links")
	doctorCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	rootCmd.AddCommand(doctorCmd)
}

var osExit = os.Exit

func runDoctor(cmd *cobra.Command, args []string) error {
	fix, _ := cmd.Flags().GetBool("fix")
	force, _ := cmd.Flags().GetBool("force")
	if fix && output.JSON {
		return output.NewStructuredError(output.ErrInputConflict, "--fix cannot be used with --json", "Run 'syllago doctor --fix' for repair, or omit --fix for JSON diagnostics")
	}

	projectRoot, _ := findProjectRoot()
	result := doctor.Run(projectRoot)

	if output.JSON {
		output.Print(result)
	} else {
		for _, c := range result.Checks {
			printCheck(c)
		}
		fmt.Fprintln(output.Writer)
		fmt.Fprintf(output.Writer, "  %s\n", result.Summary)
	}

	if fix {
		return runDoctorFix(force)
	}

	errs, warns := 0, 0
	for _, c := range result.Checks {
		switch c.Status {
		case doctor.CheckErr:
			errs++
		case doctor.CheckWarn:
			warns++
		}
	}
	if errs > 0 {
		osExit(2)
		return nil
	}
	if warns > 0 {
		osExit(1)
		return nil
	}
	return nil
}

func runDoctorFix(force bool) error {
	broken, err := doctor.BrokenProviderLinks()
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrSystemHomedir, "cannot determine home directory", "Ensure $HOME is set in your environment", err.Error())
	}

	libraryItems, err := doctorFixLibraryItems()
	if err != nil {
		return err
	}

	actions := installer.PlanLinkFixes(broken, libraryItems)
	if len(actions) == 0 {
		fmt.Fprintln(output.Writer, "Nothing to fix.")
		return nil
	}

	for _, action := range actions {
		switch action.Kind {
		case installer.FixRelink:
			fmt.Fprintf(output.Writer, "relink: %s -> %s\n", action.Link.Path, action.NewSource)
		case installer.FixPrune:
			fmt.Fprintf(output.Writer, "prune:  %s (target gone: %s)\n", action.Link.Path, action.Link.Target)
		}
	}

	if !force {
		if !isInteractive() {
			return output.NewStructuredError(output.ErrInputTerminal, "doctor --fix requires confirmation", "Run 'syllago doctor --fix --force' to apply repairs non-interactively")
		}
		fmt.Fprint(output.Writer, "Continue? [y/N] ")
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			return output.NewStructuredError(output.ErrInputTerminal, "no input received", "Run with --force to skip confirmation")
		}
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if answer != "y" && answer != "yes" {
			fmt.Fprintln(output.Writer, "Cancelled.")
			return nil
		}
	}

	errs := installer.ApplyLinkFixes(actions)
	if len(errs) > 0 {
		for _, err := range errs {
			fmt.Fprintln(output.ErrWriter, err)
		}
		osExit(1)
		return nil
	}

	relinked, pruned := countLinkFixKinds(actions)
	fmt.Fprintf(output.Writer, "Fixed: %d relinked, %d pruned\n", relinked, pruned)
	telemetry.Enrich("action_count", len(actions))
	return nil
}

func doctorFixLibraryItems() ([]catalog.ContentItem, error) {
	emptyProjectRoot, err := os.MkdirTemp("", "syllago-doctor-fix-*")
	if err != nil {
		return nil, output.NewStructuredErrorDetail(output.ErrSystemIO, "creating temp dir failed", "Check filesystem permissions and disk space", err.Error())
	}
	defer func() { _ = os.RemoveAll(emptyProjectRoot) }()

	cat, err := catalog.ScanWithGlobalAndRegistries(emptyProjectRoot, emptyProjectRoot, nil)
	if err != nil {
		return nil, output.NewStructuredErrorDetail(output.ErrCatalogScanFailed, "scanning library failed", "Check that ~/.syllago/content/ exists and is readable", err.Error())
	}

	var items []catalog.ContentItem
	for _, item := range cat.Items {
		if item.Source == "global" {
			items = append(items, item)
		}
	}
	return items, nil
}

func countLinkFixKinds(actions []installer.FixAction) (relinked, pruned int) {
	for _, action := range actions {
		switch action.Kind {
		case installer.FixRelink:
			relinked++
		case installer.FixPrune:
			pruned++
		}
	}
	return relinked, pruned
}

const (
	colorGreen  = "\033[38;2;135;154;57m"
	colorYellow = "\033[38;2;173;131;1m"
	colorRed    = "\033[38;2;209;77;65m"
	colorMuted  = "\033[38;2;135;133;128m"
	colorReset  = "\033[0m"
)

// --- Type aliases and shims so package main tests compile unchanged ---

const (
	checkOK   = doctor.CheckOK
	checkWarn = doctor.CheckWarn
	checkErr  = doctor.CheckErr
)

type checkResult = doctor.CheckResult

type doctorResult = doctor.Result

func checkLibrary() doctor.CheckResult                { return doctor.CheckLibrary() }
func checkConfigWith(r string) doctor.CheckResult     { return doctor.CheckConfigWith(r) }
func checkProviders() doctor.CheckResult              { return doctor.CheckProviders() }
func checkSymlinks(r string) doctor.CheckResult       { return doctor.CheckSymlinks(r) }
func checkContentDrift(r string) doctor.CheckResult   { return doctor.CheckContentDrift(r) }
func checkOrphans(r string) doctor.CheckResult        { return doctor.CheckOrphans(r) }
func checkRegistriesWith(r string) doctor.CheckResult { return doctor.CheckRegistriesWith(r) }
func checkNamingQuality(r string) doctor.CheckResult  { return doctor.CheckNamingQuality(r) }
func joinWords(parts []string) string                 { return doctor.JoinWords(parts) }

// ---

func printCheck(c doctor.CheckResult) {
	var marker string
	switch c.Status {
	case doctor.CheckOK:
		marker = colorGreen + "[ok]" + colorReset
	case doctor.CheckWarn:
		marker = colorYellow + "[warn]" + colorReset
	case doctor.CheckErr:
		marker = colorRed + "[err]" + colorReset
	}
	fmt.Fprintf(output.Writer, "  %s %s\n", marker, c.Message)
	for _, d := range c.Details {
		fmt.Fprintf(output.Writer, "       %s%s%s\n", colorMuted, d, colorReset)
	}
}
