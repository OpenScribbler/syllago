package main

import (
	"context"
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
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
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
	Aliases: []string{"syll"},
	Short:   "AI coding tool content manager",
	Long: `Syllago manages AI tool configurations across providers.

Run without arguments for interactive mode (TUI). Use subcommands for
automation and scripting.

Workflow:
  1. syllago import    Bring content in from a provider, path, or git URL
  2. syllago export    Send content out to any provider's install location
  3. syllago promote   Share local content to a registry (PR workflow)

Content lives in local/ after import. Syllago handles format conversion
automatically — a Claude Code skill becomes a Kiro steering file, a Cursor
rule becomes a Windsurf rule, etc.

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

		return nil
	}

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(backfillCmd)
	rootCmd.AddCommand(updateCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print syllago version",
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
			return fmt.Errorf("scanning catalog: %w", err)
		}

		// Get git author
		author := ""
		out, err := exec.Command("git", "config", "user.name").Output()
		if err == nil {
			author = strings.TrimSpace(string(out))
		}

		count := 0
		for _, item := range cat.Items {
			if item.Local || item.Meta != nil {
				continue // skip local items and items that already have metadata
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
	Use:   "update",
	Short: "Update syllago to the latest release",
	RunE:  runUpdate,
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
	if err := rootCmd.Execute(); err != nil {
		printExecuteError(err)
		os.Exit(output.ExitError)
	}
}

// printExecuteError prints err to the error writer unless it's a SilentError
// (meaning the command already printed its own error message).
func printExecuteError(err error) {
	if !output.IsSilentError(err) {
		fmt.Fprintln(output.ErrWriter, err)
	}
}

// wrapTTYError wraps bubbletea TTY errors with user-facing guidance.
func wrapTTYError(err error) error {
	if err == nil {
		return nil
	}
	errMsg := err.Error()
	if strings.Contains(errMsg, "TTY") || strings.Contains(errMsg, "tty") {
		return fmt.Errorf("syllago requires a terminal for interactive mode. Use a subcommand for non-interactive usage")
	}
	return err
}

func runTUI(cmd *cobra.Command, args []string) error {
	root, err := findContentRepoRoot()
	if err != nil {
		return fmt.Errorf("could not find syllago content repository.\n\nTo get started:\n  syllago init    Create a new content repo in the current directory\n\nFor more info: syllago --help")
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

	cat, err := catalog.ScanWithGlobalAndRegistries(root, projectRoot, regSources)
	if err != nil {
		return fmt.Errorf("catalog scan failed: %w", err)
	}

	// Auto-cleanup: remove local items whose ID matches a shared item
	cleaned, _ := catalog.CleanupPromotedItems(cat)
	if len(cleaned) > 0 {
		for _, c := range cleaned {
			fmt.Fprintf(os.Stderr, "Cleaned up promoted item: %s (%s)\n", c.Name, c.Type)
		}
		// Rescan after cleanup
		cat, err = catalog.ScanWithGlobalAndRegistries(root, projectRoot, regSources)
		if err != nil {
			return fmt.Errorf("error rescanning catalog: %w", err)
		}
	}

	providers := provider.DetectProviders()

	// Check if auto-update is enabled in project config
	autoUpdate := false
	if cfgErr == nil && cfg.Preferences["autoUpdate"] == "true" {
		autoUpdate = true
	}

	isReleaseBuild := buildCommit == "" && version != ""
	app := tui.NewApp(cat, providers, version, autoUpdate, regSources, cfg, isReleaseBuild, projectRoot)
	zone.NewGlobal()
	p := tea.NewProgram(app,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		return wrapTTYError(err)
	}
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
		return "", fmt.Errorf("could not find syllago content repository")
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
		return fmt.Errorf("invalid version format: %q (must be semver like 1.0.0)", v)
	}
	return nil
}

// ensureUpToDate checks if the binary's embedded commit matches the repo HEAD.
// If not, it rebuilds the binary and re-execs — replacing this process seamlessly.
// Every failure is graceful: the old binary just keeps running.
func ensureUpToDate() {
	root, err := findContentRepoRoot()
	if err != nil {
		return
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
