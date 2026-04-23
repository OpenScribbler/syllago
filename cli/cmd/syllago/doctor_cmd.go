package main

import (
	"fmt"
	"os"

	"github.com/OpenScribbler/syllago/cli/internal/doctor"
	"github.com/OpenScribbler/syllago/cli/internal/output"
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
	rootCmd.AddCommand(doctorCmd)
}

var osExit = os.Exit

func runDoctor(cmd *cobra.Command, args []string) error {
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
