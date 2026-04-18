package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/spf13/cobra"
)

// checkStatus represents the outcome of a single doctor check.
type checkStatus string

const (
	checkOK   checkStatus = "ok"
	checkWarn checkStatus = "warn"
	checkErr  checkStatus = "error"
)

// checkResult is one doctor check's output.
type checkResult struct {
	Name    string      `json:"name"`
	Status  checkStatus `json:"status"`
	Message string      `json:"message"`
	Details []string    `json:"details,omitempty"`
}

// doctorResult is the JSON-serializable output for syllago doctor.
type doctorResult struct {
	Checks  []checkResult `json:"checks"`
	Summary string        `json:"summary"`
}

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
	var checks []checkResult

	projectRoot, _ := findProjectRoot()

	checks = append(checks, checkLibrary())
	checks = append(checks, checkConfigWith(projectRoot))
	checks = append(checks, checkProviders())

	if projectRoot != "" {
		checks = append(checks, checkSymlinks(projectRoot))
		checks = append(checks, checkContentDrift(projectRoot))
		checks = append(checks, checkOrphans(projectRoot))
	}

	checks = append(checks, checkRegistriesWith(projectRoot))
	checks = append(checks, checkNamingQuality(projectRoot))

	// Compute summary
	warns, errs := 0, 0
	for _, c := range checks {
		switch c.Status {
		case checkWarn:
			warns++
		case checkErr:
			errs++
		}
	}

	summary := "All checks passed"
	if errs > 0 {
		summary = fmt.Sprintf("%d error(s), %d warning(s)", errs, warns)
	} else if warns > 0 {
		summary = fmt.Sprintf("%d warning(s)", warns)
	}

	if output.JSON {
		output.Print(doctorResult{Checks: checks, Summary: summary})
	} else {
		for _, c := range checks {
			printCheck(c)
		}
		fmt.Fprintln(output.Writer)
		fmt.Fprintf(output.Writer, "  %s\n", summary)
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

// --- Flexoki-inspired ANSI colors ---

const (
	colorGreen  = "\033[38;2;135;154;57m"  // Flexoki green (#879A39)
	colorYellow = "\033[38;2;173;131;1m"   // Flexoki yellow (#AD8301)
	colorRed    = "\033[38;2;209;77;65m"   // Flexoki red (#D14D41)
	colorMuted  = "\033[38;2;135;133;128m" // Flexoki tx-3 (#878580)
	colorReset  = "\033[0m"
)

func printCheck(c checkResult) {
	var marker string
	switch c.Status {
	case checkOK:
		marker = colorGreen + "[ok]" + colorReset
	case checkWarn:
		marker = colorYellow + "[warn]" + colorReset
	case checkErr:
		marker = colorRed + "[err]" + colorReset
	}
	fmt.Fprintf(output.Writer, "  %s %s\n", marker, c.Message)
	for _, d := range c.Details {
		fmt.Fprintf(output.Writer, "       %s%s%s\n", colorMuted, d, colorReset)
	}
}

// --- Individual checks ---

func checkLibrary() checkResult {
	dir := catalog.GlobalContentDir()
	if dir == "" {
		return checkResult{Name: "library", Status: checkErr, Message: "Library: cannot determine home directory"}
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return checkResult{Name: "library", Status: checkErr, Message: fmt.Sprintf("Library: %s not found", dir), Details: []string{"Run 'syllago init' to create your library"}}
	}

	// Count items
	count := 0
	for _, ct := range catalog.AllContentTypes() {
		typeDir := filepath.Join(dir, string(ct))
		entries, err := os.ReadDir(typeDir)
		if err == nil {
			count += len(entries)
		}
	}
	return checkResult{Name: "library", Status: checkOK, Message: fmt.Sprintf("Library: %s (%d items)", dir, count)}
}

func checkConfigWith(projectRoot string) checkResult {
	globalCfg, gErr := config.LoadGlobal()
	projectCfg, pErr := config.Load(projectRoot)

	if gErr != nil && pErr != nil {
		return checkResult{Name: "config", Status: checkErr, Message: "Config: failed to load", Details: []string{gErr.Error()}}
	}

	parts := []string{}
	if globalCfg != nil {
		parts = append(parts, "global")
	}
	if projectCfg != nil && projectRoot != "" {
		parts = append(parts, "project")
	}
	if len(parts) == 0 {
		return checkResult{Name: "config", Status: checkWarn, Message: "Config: no config files found", Details: []string{"Run 'syllago init' or create ~/.syllago/config.json"}}
	}

	return checkResult{Name: "config", Status: checkOK, Message: fmt.Sprintf("Config: %s loaded", joinWords(parts))}
}

func checkProviders() checkResult {
	detected := provider.DetectProviders()
	found, notFound := 0, 0
	var missing []string
	for _, p := range detected {
		if p.Detected {
			found++
		} else {
			notFound++
			missing = append(missing, p.Slug)
		}
	}

	if found == 0 {
		return checkResult{Name: "providers", Status: checkWarn, Message: "Providers: none detected", Details: missing}
	}
	msg := fmt.Sprintf("Providers: %d detected", found)
	if notFound > 0 {
		msg += fmt.Sprintf(", %d not found", notFound)
	}
	return checkResult{Name: "providers", Status: checkOK, Message: msg}
}

func checkSymlinks(projectRoot string) checkResult {
	inst, err := installer.LoadInstalled(projectRoot)
	if err != nil {
		return checkResult{Name: "symlinks", Status: checkWarn, Message: "Symlinks: could not load installed.json"}
	}

	if len(inst.Symlinks) == 0 {
		return checkResult{Name: "symlinks", Status: checkOK, Message: "Symlinks: none installed"}
	}

	broken := 0
	var details []string
	for _, s := range inst.Symlinks {
		if _, err := os.Stat(s.Target); err != nil {
			broken++
			details = append(details, fmt.Sprintf("broken: %s -> %s", s.Path, s.Target))
		}
	}

	if broken > 0 {
		return checkResult{Name: "symlinks", Status: checkWarn, Message: fmt.Sprintf("Symlinks: %d of %d broken", broken, len(inst.Symlinks)), Details: details}
	}
	return checkResult{Name: "symlinks", Status: checkOK, Message: fmt.Sprintf("Symlinks: %d valid", len(inst.Symlinks))}
}

func checkContentDrift(projectRoot string) checkResult {
	drifted, err := installer.VerifyIntegrity(projectRoot)
	if err != nil {
		return checkResult{Name: "integrity", Status: checkWarn, Message: "Integrity: could not verify", Details: []string{err.Error()}}
	}

	if len(drifted) == 0 {
		return checkResult{Name: "integrity", Status: checkOK, Message: "Integrity: no content drift detected"}
	}

	var details []string
	for _, d := range drifted {
		details = append(details, fmt.Sprintf("%s: %s (%s)", d.Type, d.Name, d.Status))
	}
	return checkResult{Name: "integrity", Status: checkWarn, Message: fmt.Sprintf("Integrity: %d item(s) modified since install", len(drifted)), Details: details}
}

func checkOrphans(projectRoot string) checkResult {
	inst, err := installer.LoadInstalled(projectRoot)
	if err != nil {
		return checkResult{Name: "orphans", Status: checkWarn, Message: "Orphans: could not load installed.json"}
	}

	libDir := catalog.GlobalContentDir()
	if libDir == "" {
		return checkResult{Name: "orphans", Status: checkOK, Message: "Orphans: skipped (no library)"}
	}

	orphaned := 0
	var details []string
	for _, s := range inst.Symlinks {
		// Check if the symlink target is inside the library
		if _, err := os.Stat(s.Target); err != nil {
			orphaned++
			details = append(details, filepath.Base(s.Path))
		}
	}

	// Check for orphaned hooks/MCP entries in provider settings.json files.
	// These can appear when syllago crashes between writing settings.json and
	// updating installed.json — the "crash window" for merged content.
	// Only check for orphaned merges if there are tracked installs.
	// Without any tracked content, settings.json entries are user-managed.
	detected := provider.DetectProviders()
	var mergeOrphans []installer.OrphanEntry
	var mergeErr error
	if len(inst.Hooks) > 0 || len(inst.MCP) > 0 {
		mergeOrphans, mergeErr = installer.CheckOrphanedMerges(projectRoot, detected)
	}
	if mergeErr == nil && len(mergeOrphans) > 0 {
		for _, o := range mergeOrphans {
			orphaned++
			if o.Type == "hook" {
				details = append(details, fmt.Sprintf("%s: untracked hook in hooks.%s (provider: %s)", o.Type, o.Key, o.Provider))
			} else {
				details = append(details, fmt.Sprintf("%s: untracked server %q (provider: %s)", o.Type, o.Key, o.Provider))
			}
		}
	}

	if orphaned > 0 {
		return checkResult{Name: "orphans", Status: checkWarn, Message: fmt.Sprintf("Orphans: %d installed item(s) missing from disk or untracked", orphaned), Details: details}
	}
	return checkResult{Name: "orphans", Status: checkOK, Message: "Orphans: all installed content accounted for"}
}

func checkRegistriesWith(projectRoot string) checkResult {
	globalCfg, _ := config.LoadGlobal()
	projectCfg, _ := config.Load(projectRoot)
	merged := config.Merge(globalCfg, projectCfg)

	if len(merged.Registries) == 0 {
		return checkResult{Name: "registries", Status: checkOK, Message: "Registries: none configured"}
	}

	pub, priv, unknown := 0, 0, 0
	for _, r := range merged.Registries {
		switch r.Visibility {
		case "public":
			pub++
		case "private":
			priv++
		default:
			unknown++
		}
	}

	parts := []string{}
	if pub > 0 {
		parts = append(parts, fmt.Sprintf("%d public", pub))
	}
	if priv > 0 {
		parts = append(parts, fmt.Sprintf("%d private", priv))
	}
	if unknown > 0 {
		parts = append(parts, fmt.Sprintf("%d unknown", unknown))
	}

	return checkResult{
		Name:    "registries",
		Status:  checkOK,
		Message: fmt.Sprintf("Registries: %d configured (%s)", len(merged.Registries), joinWords(parts)),
	}
}

func checkNamingQuality(projectRoot string) checkResult {
	cat, err := catalog.ScanWithGlobalAndRegistries(projectRoot, projectRoot, nil)
	if err != nil {
		return checkResult{Name: "naming", Status: checkWarn, Message: "Naming: could not scan content", Details: []string{err.Error()}}
	}

	var unnamed int
	var details []string
	for _, item := range cat.Items {
		if item.Type != catalog.Hooks && item.Type != catalog.MCP {
			continue
		}
		if item.DisplayName == "" || item.DisplayName == item.Name {
			unnamed++
			details = append(details, fmt.Sprintf("%s %s: no display name", item.Type, item.Name))
		}
	}

	if unnamed > 0 {
		return checkResult{
			Name:    "naming",
			Status:  checkWarn,
			Message: fmt.Sprintf("Naming: %d hooks/MCP items have no display name", unnamed),
			Details: details,
		}
	}
	return checkResult{Name: "naming", Status: checkOK, Message: "Naming: all hooks/MCP items have display names"}
}

func joinWords(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	default:
		result := parts[0]
		for i := 1; i < len(parts); i++ {
			result += ", " + parts[i]
		}
		return result
	}
}
