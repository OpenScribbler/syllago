package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
	"github.com/holdenhewett/romanesco/cli/internal/config"
	"github.com/holdenhewett/romanesco/cli/internal/metadata"
	"github.com/holdenhewett/romanesco/cli/internal/output"
	"github.com/holdenhewett/romanesco/cli/internal/provider"
	"github.com/holdenhewett/romanesco/cli/internal/scan/detectors"
	"github.com/holdenhewett/romanesco/cli/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	Use:           "nesco",
	Short:         "AI coding tool content manager and codebase scanner",
	Long:          "Nesco manages AI tool configurations and scans codebases for context that helps AI agents produce correct code.",
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
		return nil
	}

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(backfillCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print nesco version",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Println(version)
	},
}

var backfillCmd = &cobra.Command{
	Use:    "backfill",
	Short:  "Generate .romanesco.yaml for items without metadata",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findContentRepoRoot()
		if err != nil {
			return err
		}

		cat, err := catalog.Scan(root)
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

func main() {
	// Self-rebuild: if source has changed since this binary was built, rebuild and re-exec.
	if buildCommit != "" {
		ensureUpToDate()
	}
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runTUI(cmd *cobra.Command, args []string) error {
	root, err := findContentRepoRoot()
	if err != nil {
		return fmt.Errorf("could not find romanesco repo: %w", err)
	}

	cat, err := catalog.Scan(root)
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
		cat, err = catalog.Scan(root)
		if err != nil {
			return fmt.Errorf("error rescanning catalog: %w", err)
		}
	}

	providers := provider.DetectProviders()

	// Check if auto-update is enabled in project config
	autoUpdate := false
	cfg, cfgErr := config.Load(root)
	if cfgErr == nil && cfg.Preferences["autoUpdate"] == "true" {
		autoUpdate = true
	}

	app := tui.NewApp(cat, providers, detectors.AllDetectors(), version, autoUpdate)
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

// findContentRepoRoot returns the repo root path. It tries:
// 1. Build-time embedded path (via -ldflags)
// 2. Walk up from cwd looking for a "skills/" directory
// 3. Walk up from the binary's own location (handles aliases, PATH installs, bare go build)
func findContentRepoRoot() (string, error) {
	if repoRoot != "" {
		if _, err := os.Stat(repoRoot); err == nil {
			return repoRoot, nil
		}
	}

	// Try walking up from CWD
	if cwd, err := os.Getwd(); err == nil {
		if root, err := findSkillsDir(cwd); err == nil {
			return root, nil
		}
	}

	// Try walking up from the binary's own location (resolves symlinks)
	if binPath, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(binPath); err == nil {
			if root, err := findSkillsDir(filepath.Dir(resolved)); err == nil {
				return root, nil
			}
		}
	}

	cwd, _ := os.Getwd()
	return "", fmt.Errorf("could not find repo root (no skills/ directory found above %s or binary location)", cwd)
}

// findSkillsDir walks up from dir looking for a "skills/" directory.
func findSkillsDir(dir string) (string, error) {
	for {
		if _, err := os.Stat(filepath.Join(dir, "skills")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("no skills/ directory found above %s", dir)
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

	fmt.Fprintf(os.Stderr, "Source updated, rebuilding nesco...\n")

	// Resolve the path to this binary (follows symlinks)
	binPath, err := os.Executable()
	if err != nil {
		return
	}
	binPath, err = filepath.EvalSymlinks(binPath)
	if err != nil {
		return
	}

	// Read version from VERSION file
	rebuildVersion := version
	if vb, err := os.ReadFile(filepath.Join(root, "VERSION")); err == nil {
		rebuildVersion = strings.TrimSpace(string(vb))
	}

	// Rebuild with the new commit and version embedded
	ldflags := fmt.Sprintf("-X main.repoRoot=%s -X main.buildCommit=%s -X main.version=%s", root, currentCommit, rebuildVersion)
	build := exec.Command("go", "build", "-ldflags", ldflags, "-o", binPath, "./cmd/nesco")
	build.Dir = filepath.Join(root, "cli")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Rebuild failed: %s (running stale binary)\n", err)
		return
	}

	// Replace this process with the newly built binary
	execErr := syscall.Exec(binPath, os.Args, os.Environ())
	// Only reached if Exec fails
	fmt.Fprintf(os.Stderr, "Restart failed: %s\n", execErr)
}
