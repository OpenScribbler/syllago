package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
	"github.com/OpenScribbler/syllago/cli/internal/tui"
	"github.com/OpenScribbler/syllago/cli/internal/updater"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
)

// Set at build time via -ldflags.
var (
	repoRoot    string
	buildCommit string
	version     string
)

var rootCmd = &cobra.Command{
	Use:     "syllago",
	Aliases: []string{"syl"},
	Short:   "AI coding tool content manager",
	Long: `Syllago manages AI tool configurations across providers.

Run without arguments for interactive mode (TUI). Use subcommands for
automation and scripting.

Workflow:
  1. syllago add       Bring content into your library from a provider or registry
  2. syllago install    Install library content to a provider's location
  3. syllago share      Contribute library content to a shared git repo (PR workflow)

Content lives in your global library (~/.syllago/content/) after adding.
Syllago handles format conversion automatically — a Claude Code skill becomes
a Kiro steering file, a Cursor rule becomes a Windsurf rule, etc.

Other useful commands:
  syllago convert      Convert content between provider formats
  syllago remove       Remove content from your library
  syllago uninstall    Deactivate content from a provider

Browse registries with "syllago registry items" and sync with "syllago registry sync".

Exit codes: 0=success, 1=error, 2=usage error`,
	RunE:          runTUI,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&output.JSON, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().Bool("no-color", false, "Disable color output")
	rootCmd.PersistentFlags().BoolP("quiet", "q", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Verbose output")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		noColor, _ := cmd.Flags().GetBool("no-color")
		if noColor || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
			lipgloss.SetColorProfile(termenv.Ascii)
		}

		quiet, _ := cmd.Flags().GetBool("quiet")
		output.Quiet = quiet

		verbose, _ := cmd.Flags().GetBool("verbose")
		output.Verbose = verbose

		// Initialize telemetry after output flags are set so the first-run
		// notice (if any) respects --no-color via lipgloss profile above.
		telemetry.Init()

		return nil
	}

	rootCmd.PersistentPostRun = func(cmd *cobra.Command, args []string) {
		// Build a dotted command name: "registry add" → "registry_add".
		// Skip telemetry's own subcommands to avoid recursion/noise.
		name := commandPath(cmd)
		if name != "" && !strings.HasPrefix(name, "telemetry") {
			telemetry.TrackCommand(name)
		}
		telemetry.Shutdown()
	}

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(backfillCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(capmonCmd)
	rootCmd.AddCommand(moatCmd)
}

var versionCmd = &cobra.Command{
	Use:     "version",
	Short:   "Print syllago version",
	Example: `  syllago version`,
	Run: func(cmd *cobra.Command, args []string) {
		v := version
		if v == "" {
			v = "(dev build)"
		}
		cmd.Println(v)
	},
}

var backfillCmd = &cobra.Command{
	Use:    "backfill",
	Short:  "Generate .syllago.yaml for items without metadata",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findContentRepoRoot()
		if err != nil {
			return err
		}

		projectRoot, _ := findProjectRoot()
		if projectRoot == "" {
			projectRoot = root
		}
		cat, err := catalog.Scan(root, projectRoot)
		if err != nil {
			return output.NewStructuredErrorDetail(output.ErrCatalogScanFailed, "scanning catalog failed", "Check that the content directory exists and is readable", err.Error())
		}

		// Get git author
		author := ""
		out, err := exec.Command("git", "config", "user.name").Output()
		if err == nil {
			author = strings.TrimSpace(string(out))
		}

		count := 0
		for _, item := range cat.Items {
			if item.Library || item.Meta != nil {
				continue // skip library items and items that already have metadata
			}
			// Only backfill universal items (they have a directory)
			if !item.Type.IsUniversal() {
				continue
			}
			if err := metadata.Backfill(item.Path, item.Name, string(item.Type), author); err != nil {
				fmt.Fprintf(os.Stderr, "Error backfilling %s: %s\n", item.Name, err)
				continue
			}
			fmt.Printf("Backfilled: %s (%s)\n", item.Name, item.Type)
			count++
		}

		if count == 0 {
			fmt.Println("No items need backfilling.")
		} else {
			fmt.Printf("Backfilled %d items.\n", count)
		}
		return nil
	},
}

var updateCmd = &cobra.Command{
	Use:     "update",
	Short:   "Update syllago to the latest release",
	Example: `  syllago update`,
	RunE:    runUpdate,
}

func runUpdate(cmd *cobra.Command, args []string) error {
	if buildCommit != "" {
		// Dev build — self-rebuild handles updates via ensureUpToDate()
		cmd.Println("Self-update is only available for release builds.")
		cmd.Println("You're running a dev build. Use `make build` to rebuild from source.")
		return nil
	}
	v := version
	if v == "" {
		v = "0.0.0"
	}
	return updater.Update(v, func(msg string) {
		cmd.Println(msg)
	})
}

func main() {
	// Self-rebuild: if source has changed since this binary was built, rebuild and re-exec.
	if buildCommit != "" {
		ensureUpToDate()
	}
	telemetry.SetVersion(version)
	if err := rootCmd.Execute(); err != nil {
		printExecuteError(err)
		os.Exit(output.ExitError)
	}
}

// printExecuteError prints err to the error writer unless it's a SilentError
// (meaning the command already printed its own error message).
// In JSON mode, non-silent errors are wrapped in a structured JSON envelope.
func printExecuteError(err error) {
	if output.IsSilentError(err) {
		return
	}
	// If the error is a StructuredError returned directly (not via SilentError),
	// print it using the structured formatter.
	var se output.StructuredError
	if errors.As(err, &se) {
		output.PrintStructuredError(se)
		return
	}
	// In JSON mode, wrap unstructured errors in a JSON envelope for consistency.
	if output.JSON {
		output.PrintStructuredError(output.StructuredError{
			Code:    "UNKNOWN_001",
			Message: err.Error(),
		})
		return
	}
	fmt.Fprintln(output.ErrWriter, err)
}

// wrapTTYError wraps bubbletea TTY errors with user-facing guidance.
func wrapTTYError(err error) error {
	if err == nil {
		return nil
	}
	errMsg := err.Error()
	if strings.Contains(errMsg, "TTY") || strings.Contains(errMsg, "tty") {
		return output.NewStructuredError(output.ErrInputTerminal, "syllago requires a terminal for interactive mode", "Use a subcommand for non-interactive usage (e.g., syllago list --json)")
	}
	return err
}

func runTUI(cmd *cobra.Command, args []string) error {
	root, err := findContentRepoRoot()
	if err != nil {
		return output.NewStructuredError(output.ErrCatalogNotFound, "could not find syllago content repository", "Run 'syllago init' to create a new content repo in the current directory")
	}

	// Load project config to get registry list and preferences
	projectCfg, cfgErr := config.Load(root)
	if cfgErr != nil {
		projectCfg = &config.Config{}
	}

	// Load global config and merge with project config
	globalCfg, _ := config.LoadGlobal()
	if globalCfg == nil {
		globalCfg = &config.Config{}
	}
	cfg := config.Merge(globalCfg, projectCfg)

	// Auto-sync registries if enabled (5-second timeout; failure is non-fatal)
	if cfgErr == nil && cfg.Preferences["registryAutoSync"] == "true" && len(cfg.Registries) > 0 {
		names := make([]string, len(cfg.Registries))
		for i, r := range cfg.Registries {
			names[i] = r.Name
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = ctx // The goroutine and underlying git process are intentionally abandoned on timeout — git will finish on its own.
		syncDone := make(chan struct{})
		go func() {
			registry.SyncAll(names)
			close(syncDone)
		}()
		select {
		case <-syncDone:
		case <-ctx.Done():
			fmt.Fprintf(os.Stderr, "Registry auto-sync timed out, using cached content\n")
		}
		cancel()
	}

	// Build registry sources from config
	var regSources []catalog.RegistrySource
	for _, r := range cfg.Registries {
		if registry.IsCloned(r.Name) {
			dir, _ := registry.CloneDir(r.Name)
			regSources = append(regSources, catalog.RegistrySource{Name: r.Name, Path: dir})
		}
	}

	projectRoot, _ := findProjectRoot()
	if projectRoot == "" {
		projectRoot = root
	}

	// MOAT enrichment inputs — matches the TUI rescan path (app.go) so trust
	// surfaces appear on first-load instead of only after an implicit rescan.
	// Fix for syllago-scgjl: startup used to call ScanWithGlobalAndRegistries
	// directly, bypassing EnrichFromMOATManifests. A non-MOAT config is a
	// no-op here.
	cacheDir, _ := config.GlobalDirPath()
	lf, _ := moat.LoadLockfile(moat.LockfilePath(projectRoot))

	cat, err := moat.ScanAndEnrich(cfg, root, projectRoot, regSources, lf, cacheDir, time.Now())
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrCatalogScanFailed, "catalog scan failed", "Check that the content directory exists and is readable", err.Error())
	}

	// Auto-cleanup: remove local items whose ID matches a shared item
	cleaned, _ := catalog.CleanupPromotedItems(cat)
	if len(cleaned) > 0 {
		for _, c := range cleaned {
			fmt.Fprintf(os.Stderr, "Cleaned up promoted item: %s (%s)\n", c.Name, c.Type)
		}
		// Rescan after cleanup
		cat, err = moat.ScanAndEnrich(cfg, root, projectRoot, regSources, lf, cacheDir, time.Now())
		if err != nil {
			return output.NewStructuredErrorDetail(output.ErrCatalogScanFailed, "error rescanning catalog", "Check that the content directory exists and is readable", err.Error())
		}
	}

	resolver := config.NewResolver(cfg, "")
	if err := resolver.ExpandPaths(); err != nil {
		resolver = nil // non-fatal, fall back to standard detection
	}
	providers := provider.DetectProvidersWithResolver(resolver)

	// Check if auto-update is enabled in project config
	autoUpdate := cfgErr == nil && cfg.Preferences["autoUpdate"] == "true"

	isReleaseBuild := buildCommit == "" && version != ""
	app := tui.NewApp(cat, providers, version, autoUpdate, regSources, cfg, isReleaseBuild, root, projectRoot)
	zone.NewGlobal()
	p := tea.NewProgram(app,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		return wrapTTYError(err)
	}
	telemetry.Track("tui_session_started", map[string]any{
		"success": true,
	})
	return nil
}

// findContentRepoRoot returns the path syllago uses as its content root. It tries:
// 1. Build-time embedded path (via -ldflags)
// 2. Config-aware resolution from the project root
func findContentRepoRoot() (string, error) {
	if repoRoot != "" {
		if _, err := os.Stat(repoRoot); err == nil {
			return resolveContentRoot(repoRoot)
		}
	}

	projectRoot, err := findProjectRoot()
	if err != nil {
		return "", output.NewStructuredError(output.ErrCatalogNotFound, "could not find syllago content repository", "Run 'syllago init' to set up a content repository")
	}

	return resolveContentRoot(projectRoot)
}

// resolveContentRoot applies the config-aware resolution order:
// 1. If .syllago/config.json exists with contentRoot → use <projectRoot>/<contentRoot>
// 2. Else if any content directory exists at project root → use project root
// 3. Else → use project root (scanner handles empty gracefully)
func resolveContentRoot(projectRoot string) (string, error) {
	cfg, err := config.Load(projectRoot)
	if err == nil && cfg.ContentRoot != "" {
		return filepath.Join(projectRoot, cfg.ContentRoot), nil
	}

	for _, ct := range catalog.AllContentTypes() {
		if _, err := os.Stat(filepath.Join(projectRoot, string(ct))); err == nil {
			return projectRoot, nil
		}
	}

	return projectRoot, nil
}

// semverRegex validates strict semver format (no 'v' prefix).
var semverRegex = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

// validateVersion checks if a string is a valid semver version.
func validateVersion(v string) error {
	if !semverRegex.MatchString(v) {
		return output.NewStructuredError(output.ErrInputInvalid, fmt.Sprintf("invalid version format: %q (must be semver like 1.0.0)", v), "Use semantic versioning format like 1.0.0 or 1.2.3-beta")
	}
	return nil
}

// commandPath returns a snake_case command name from a cobra command.
// "syllago registry add" → "registry_add", "syllago install" → "install".
func commandPath(cmd *cobra.Command) string {
	parts := strings.Fields(cmd.CommandPath())
	if len(parts) <= 1 {
		return "" // root command (TUI) — tracked separately
	}
	// Drop "syllago" prefix, join with underscore.
	return strings.Join(parts[1:], "_")
}

// ensureUpToDate checks if the binary's embedded commit matches the repo HEAD.
// If not, it rebuilds the binary and re-execs — replacing this process seamlessly.
// Every failure is graceful: the old binary just keeps running.
func ensureUpToDate() {
	// Use the project root (where cli/ lives), not the content root.
	// findContentRepoRoot() may return a subdirectory like content/.
	root := repoRoot
	if root == "" {
		var err error
		root, err = findProjectRoot()
		if err != nil {
			return
		}
	}

	// Get current repo HEAD
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return // git not available or not a repo
	}
	currentCommit := strings.TrimSpace(string(out))

	if currentCommit == buildCommit {
		return // binary is current
	}

	fmt.Fprintf(os.Stderr, "Source updated, rebuilding syllago...\n")

	// Resolve the path to this binary (follows symlinks)
	binPath, err := os.Executable()
	if err != nil {
		return
	}
	binPath, err = filepath.EvalSymlinks(binPath)
	if err != nil {
		return
	}

	// Read version from VERSION file, validate before use in ldflags
	rebuildVersion := version
	if vb, err := os.ReadFile(filepath.Join(root, "VERSION")); err == nil {
		candidate := strings.TrimSpace(string(vb))
		if validateVersion(candidate) == nil {
			rebuildVersion = candidate
		} else {
			fmt.Fprintf(os.Stderr, "Warning: VERSION file has invalid format %q, using existing version\n", candidate)
		}
	}

	// Rebuild with the new commit and version embedded
	ldflags := fmt.Sprintf("-X main.repoRoot=%s -X main.buildCommit=%s -X main.version=%s", root, currentCommit, rebuildVersion)
	build := exec.Command("go", "build", "-ldflags", ldflags, "-o", binPath, "./cmd/syllago")
	build.Dir = filepath.Join(root, "cli")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Rebuild failed: %s (running stale binary)\n", err)
		return
	}

	// Replace this process with the newly built binary
	execErr := execSelf(os.Args)
	// Only reached if Exec fails
	fmt.Fprintf(os.Stderr, "Restart failed: %s\n", execErr)
}
