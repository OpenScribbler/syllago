package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/OpenScribbler/nesco/cli/internal/config"
)

// RunConfig is the full configuration for a sandbox session.
type RunConfig struct {
	ProviderSlug       string
	ProjectDir         string
	HomeDir            string
	ForceDir           bool
	AdditionalDomains  []string // --allow-domain flags
	AdditionalPorts    []int    // --allow-port flags
	AdditionalEnv      []string // --allow-env flags
	AdditionalMountsRO []string // --mount-ro flags (extra read-only bind mounts)
	NoNetwork          bool     // --no-network flag
	SandboxConfig      config.SandboxConfig
}

// bwrapRunner is the function that actually invokes bwrap.
// Overridable in tests (Task 9.1 depends on this seam).
var bwrapRunner = func(ctx context.Context, args []string) error {
	cmd := exec.CommandContext(ctx, "bwrap", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RunSession executes a full sandboxed session for the given provider.
// It writes progress messages to w (typically os.Stdout) and prompts to w.
func RunSession(cfg RunConfig, w *os.File) error {
	// Step 1: Clean stale sessions from previous crashes.
	CleanStale()

	// Step 2: Validate directory safety.
	if cfg.ForceDir {
		fmt.Fprintf(w, "WARNING: Directory safety checks skipped. The entire directory\n%s will be writable inside the sandbox.\n\n", cfg.ProjectDir)
	}
	if err := ValidateDir(cfg.ProjectDir, cfg.ForceDir); err != nil {
		return err
	}

	// Step 3: Pre-flight check for bwrap, socat, and provider binary.
	checkResult := Check(cfg.ProviderSlug, cfg.HomeDir, cfg.ProjectDir)
	if len(checkResult.Errors) > 0 {
		return fmt.Errorf("pre-flight check failed:\n%s", FormatCheckResult(checkResult, cfg.ProviderSlug))
	}

	// Step 4: Load provider mount profile.
	profile, err := ProfileFor(cfg.ProviderSlug, cfg.HomeDir, cfg.ProjectDir)
	if err != nil {
		return err
	}

	// Step 5: Create staging directory.
	staging, err := NewStagingDir()
	if err != nil {
		return err
	}
	defer staging.Cleanup()

	// Signal handler: clean up on Ctrl-C.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Step 6: Stage provider config files.
	snapshots, err := StageConfigs(staging.Path, profile.GlobalConfigPaths)
	if err != nil {
		return fmt.Errorf("staging configs: %w", err)
	}

	// Step 7: Write sandbox gitconfig.
	if err := staging.WriteGitconfig("Sandbox User", "sandbox@nesco.local"); err != nil {
		return fmt.Errorf("writing gitconfig: %w", err)
	}

	// Step 8: Detect ecosystem and build domain allowlist.
	ecosystemDomains := EcosystemDomains(cfg.ProjectDir)
	ecosystemCaches := EcosystemCacheMounts(cfg.ProjectDir, cfg.HomeDir)

	var allDomains []string
	if cfg.NoNetwork {
		// --no-network: block ALL egress — pass empty allowlist to proxy.
	} else {
		allDomains = append(allDomains, profile.AllowedDomains...)
		allDomains = append(allDomains, cfg.SandboxConfig.AllowedDomains...)
		allDomains = append(allDomains, cfg.AdditionalDomains...)
		allDomains = append(allDomains, ecosystemDomains...)
	}

	allPorts := append(cfg.SandboxConfig.AllowedPorts, cfg.AdditionalPorts...)

	// Step 9: Build env allowlist.
	allExtraEnv := append(profile.ProviderEnvVars, cfg.SandboxConfig.AllowedEnv...)
	allExtraEnv = append(allExtraEnv, cfg.AdditionalEnv...)
	envPairs, envReport := FilterEnv(os.Environ(), allExtraEnv)

	// Step 10: Print sandbox summary.
	fmt.Fprintf(w, "\nSandbox environment:\n")
	fmt.Fprintf(w, "  Forwarded: %s\n", strings.Join(envReport.Forwarded, ", "))
	fmt.Fprintf(w, "  Stripped: %d env vars (%s)\n",
		len(envReport.Stripped), strings.Join(envReport.Stripped, ", "))
	fmt.Fprintf(w, "  Network: %s\n", strings.Join(allDomains, ", "))
	fmt.Fprintf(w, "\n")

	// Step 11: Start egress proxy.
	proxy := NewProxy(staging.SocketPath(), allDomains, allPorts)
	if err := proxy.Start(); err != nil {
		return fmt.Errorf("starting proxy: %w", err)
	}
	defer proxy.Shutdown()

	// Step 12: Write git wrapper script.
	realGit, err := exec.LookPath("git")
	if err != nil {
		realGit = "/usr/bin/git"
	}
	gitWrapperPath, err := WriteGitWrapper(staging.Path, realGit)
	if err != nil {
		return fmt.Errorf("writing git wrapper: %w", err)
	}

	// Step 13: Write in-sandbox wrapper script.
	wrapperPath, err := WriteWrapperScript(staging.Path, staging.SocketPath(), profile.BinaryExec, nil)
	if err != nil {
		return fmt.Errorf("writing wrapper script: %w", err)
	}

	// Step 14: Build bwrap arguments.
	sandboxEnv := map[string]string{
		"PATH":                "/usr/local/bin:/usr/bin:/bin",
		"HTTP_PROXY":          "http://127.0.0.1:3128",
		"HTTPS_PROXY":         "http://127.0.0.1:3128",
		"NO_PROXY":            "",
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_CONFIG_GLOBAL":   staging.GitconfigPath(),
		"GIT_TERMINAL_PROMPT": "0",
		"HOME":                cfg.HomeDir,
	}

	bwrapCfg := BwrapConfig{
		ProjectDir:          cfg.ProjectDir,
		HomeDir:             cfg.HomeDir,
		StagingDir:          staging.Path,
		SocketPath:          staging.SocketPath(),
		GitWrapperPath:      gitWrapperPath,
		WrapperScript:       wrapperPath,
		Profile:             profile,
		Snapshots:           snapshots,
		EcosystemCacheRO:    ecosystemCaches,
		AdditionalMountsRO:  cfg.AdditionalMountsRO,
		EnvPairs:            envPairs,
		SandboxEnvOverrides: sandboxEnv,
	}
	bwrapArgs := BuildArgs(bwrapCfg)

	// Step 15: Launch bubblewrap (via injectable bwrapRunner for testability).
	start := time.Now()
	if err := bwrapRunner(ctx, bwrapArgs); err != nil && ctx.Err() == nil {
		// Non-zero exit from the provider is expected (e.g. user typed "exit").
		// Only surface errors that aren't from the provider itself exiting.
		if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() == -1 {
			return fmt.Errorf("sandbox exited with error: %w", err)
		}
	}

	duration := time.Since(start).Round(time.Second)

	// Step 16: Diff staged configs.
	diffs, err := ComputeDiffs(snapshots)
	if err != nil {
		fmt.Fprintf(w, "Warning: config diff failed: %s\n", err)
	}

	// Step 17: Show diffs and handle approval.
	// High-risk changes (MCP servers, hooks, commands): require explicit approval.
	// Low-risk changes (settings, preferences): auto-approve with notification.
	approved, rejected := 0, 0
	for _, d := range diffs {
		if !d.Changed {
			continue
		}

		if d.IsHighRisk {
			fmt.Fprintf(w, "\nConfig changed: %s [HIGH RISK: MCP servers, hooks, or commands]\n", d.Snapshot.OriginalPath)
			fmt.Fprintf(w, "%s\n", d.DiffText)
			if promptYN(w, "Apply this change?") {
				if err := ApplyDiff(d); err != nil {
					fmt.Fprintf(w, "Error applying diff: %s\n", err)
				} else {
					approved++
				}
			} else {
				rejected++
			}
		} else {
			fmt.Fprintf(w, "\nConfig changed: %s [auto-approved, low risk]\n", d.Snapshot.OriginalPath)
			fmt.Fprintf(w, "%s\n", d.DiffText)
			if err := ApplyDiff(d); err != nil {
				fmt.Fprintf(w, "Error applying diff: %s\n", err)
			} else {
				approved++
			}
		}
	}

	// Step 18: Print session summary.
	blocked := proxy.BlockedDomains()
	fmt.Fprintf(w, "\nSandbox session ended.\n")
	fmt.Fprintf(w, "  Duration: %s\n", duration)
	if len(blocked) > 0 {
		fmt.Fprintf(w, "  Blocked domains: %s\n", strings.Join(blocked, ", "))
	}
	if len(diffs) > 0 {
		fmt.Fprintf(w, "  Config changes: %d approved, %d rejected\n", approved, rejected)
	}

	return nil
}

// promptYN prints a [y/N] prompt and reads a single-line answer.
func promptYN(w *os.File, question string) bool {
	fmt.Fprintf(w, "%s [y/N] ", question)
	var answer string
	fmt.Fscan(os.Stdin, &answer)
	return strings.ToLower(strings.TrimSpace(answer)) == "y"
}
