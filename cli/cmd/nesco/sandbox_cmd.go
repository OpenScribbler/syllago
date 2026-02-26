package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/OpenScribbler/nesco/cli/internal/config"
	"github.com/OpenScribbler/nesco/cli/internal/output"
	"github.com/OpenScribbler/nesco/cli/internal/sandbox"
	"github.com/spf13/cobra"
)

var sandboxCmd = &cobra.Command{
	Use:   "sandbox",
	Short: "Run and manage AI CLI tools in bubblewrap sandboxes",
	Long: `Sandbox wraps AI CLI tools in bubblewrap to restrict filesystem access,
network egress, and environment variables.

Linux only: requires bubblewrap >= 0.4.0 and socat >= 1.7.0.`,
}

var sandboxRunCmd = &cobra.Command{
	Use:   "run <provider>",
	Short: "Run a provider in a sandbox",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		cfg, _ := config.Load(root)
		if cfg == nil {
			cfg = &config.Config{}
		}

		forceDir, _ := cmd.Flags().GetBool("force-dir")
		noNetwork, _ := cmd.Flags().GetBool("no-network")
		allowDomains, _ := cmd.Flags().GetStringArray("allow-domain")
		allowEnvs, _ := cmd.Flags().GetStringArray("allow-env")
		allowPortsStr, _ := cmd.Flags().GetStringArray("allow-port")
		mountRO, _ := cmd.Flags().GetStringArray("mount-ro")

		var allowPorts []int
		for _, ps := range allowPortsStr {
			p, err := strconv.Atoi(ps)
			if err != nil {
				return fmt.Errorf("invalid port %q: must be an integer", ps)
			}
			allowPorts = append(allowPorts, p)
		}

		return sandbox.RunSession(sandbox.RunConfig{
			ProviderSlug:       args[0],
			ProjectDir:         cwd,
			HomeDir:            home,
			ForceDir:           forceDir,
			AdditionalDomains:  allowDomains,
			AdditionalPorts:    allowPorts,
			AdditionalEnv:      allowEnvs,
			AdditionalMountsRO: mountRO,
			NoNetwork:          noNetwork,
			SandboxConfig:      cfg.Sandbox,
		}, os.Stdout)
	},
}

var sandboxCheckCmd = &cobra.Command{
	Use:   "check [provider]",
	Short: "Verify bubblewrap, socat, and optionally a provider",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		slug := ""
		if len(args) == 1 {
			slug = args[0]
		}
		result := sandbox.Check(slug, home, cwd)
		fmt.Print(sandbox.FormatCheckResult(result, slug))
		if len(result.Errors) > 0 {
			return output.SilentError(fmt.Errorf("pre-flight check failed"))
		}
		return nil
	},
}

var sandboxInfoCmd = &cobra.Command{
	Use:   "info [provider]",
	Short: "Show effective sandbox configuration",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		cfg, _ := config.Load(root)
		if cfg == nil {
			cfg = &config.Config{}
		}
		sb := cfg.Sandbox
		fmt.Printf("Sandbox configuration (.nesco/config.json):\n")
		fmt.Printf("  Allowed domains: %s\n", sandboxFormatList(sb.AllowedDomains))
		fmt.Printf("  Allowed env vars: %s\n", sandboxFormatList(sb.AllowedEnv))
		fmt.Printf("  Allowed ports: %v\n", sb.AllowedPorts)

		if len(args) == 1 {
			home, _ := os.UserHomeDir()
			cwd, _ := os.Getwd()
			profile, err := sandbox.ProfileFor(args[0], home, cwd)
			if err != nil {
				return err
			}
			fmt.Printf("\nProvider %q mount profile:\n", args[0])
			fmt.Printf("  Binary: %s\n", profile.BinaryExec)
			fmt.Printf("  Config files: %s\n", strings.Join(profile.GlobalConfigPaths, ", "))
			fmt.Printf("  Provider domains: %s\n", strings.Join(profile.AllowedDomains, ", "))
		}
		return nil
	},
}

var sandboxAllowDomainCmd = &cobra.Command{
	Use:   "allow-domain <domain>",
	Short: "Add a domain to the sandbox allowlist",
	Args:  cobra.ExactArgs(1),
	RunE: sandboxConfigMutator(func(sb *config.SandboxConfig, args []string) {
		sb.AllowedDomains = appendUnique(sb.AllowedDomains, args[0])
		fmt.Fprintf(output.Writer, "Added domain: %s\n", args[0])
	}),
}

var sandboxDenyDomainCmd = &cobra.Command{
	Use:   "deny-domain <domain>",
	Short: "Remove a domain from the sandbox allowlist",
	Args:  cobra.ExactArgs(1),
	RunE: sandboxConfigMutator(func(sb *config.SandboxConfig, args []string) {
		sb.AllowedDomains = removeItem(sb.AllowedDomains, args[0])
		fmt.Fprintf(output.Writer, "Removed domain: %s\n", args[0])
	}),
}

var sandboxAllowEnvCmd = &cobra.Command{
	Use:   "allow-env <VAR>",
	Short: "Add an env var to the sandbox allowlist",
	Args:  cobra.ExactArgs(1),
	RunE: sandboxConfigMutator(func(sb *config.SandboxConfig, args []string) {
		sb.AllowedEnv = appendUnique(sb.AllowedEnv, args[0])
		fmt.Fprintf(output.Writer, "Added env var: %s\n", args[0])
	}),
}

var sandboxDenyEnvCmd = &cobra.Command{
	Use:   "deny-env <VAR>",
	Short: "Remove an env var from the sandbox allowlist",
	Args:  cobra.ExactArgs(1),
	RunE: sandboxConfigMutator(func(sb *config.SandboxConfig, args []string) {
		sb.AllowedEnv = removeItem(sb.AllowedEnv, args[0])
		fmt.Fprintf(output.Writer, "Removed env var: %s\n", args[0])
	}),
}

var sandboxAllowPortCmd = &cobra.Command{
	Use:   "allow-port <port>",
	Short: "Add a localhost port to the sandbox allowlist",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		port, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid port: %s", args[0])
		}
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		cfg, _ := config.Load(root)
		if cfg == nil {
			cfg = &config.Config{}
		}
		cfg.Sandbox.AllowedPorts = appendUniqueInt(cfg.Sandbox.AllowedPorts, port)
		if err := config.Save(root, cfg); err != nil {
			return err
		}
		fmt.Fprintf(output.Writer, "Added port: %d\n", port)
		return nil
	},
}

var sandboxDenyPortCmd = &cobra.Command{
	Use:   "deny-port <port>",
	Short: "Remove a localhost port from the sandbox allowlist",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		port, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid port: %s", args[0])
		}
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		cfg, _ := config.Load(root)
		if cfg == nil {
			cfg = &config.Config{}
		}
		cfg.Sandbox.AllowedPorts = removeIntItem(cfg.Sandbox.AllowedPorts, port)
		if err := config.Save(root, cfg); err != nil {
			return err
		}
		fmt.Fprintf(output.Writer, "Removed port: %d\n", port)
		return nil
	},
}

var sandboxDomainsCmd = &cobra.Command{
	Use:   "domains",
	Short: "List allowed domains",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		cfg, _ := config.Load(root)
		if cfg == nil {
			cfg = &config.Config{}
		}
		if len(cfg.Sandbox.AllowedDomains) == 0 {
			fmt.Println("No domains configured. Provider defaults apply at runtime.")
			return nil
		}
		for _, d := range cfg.Sandbox.AllowedDomains {
			fmt.Println(d)
		}
		return nil
	},
}

var sandboxEnvCmd = &cobra.Command{
	Use:   "env",
	Short: "List allowed env vars",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		cfg, _ := config.Load(root)
		if cfg == nil {
			cfg = &config.Config{}
		}
		if len(cfg.Sandbox.AllowedEnv) == 0 {
			fmt.Println("No extra env vars configured. Base allowlist applies.")
			return nil
		}
		for _, v := range cfg.Sandbox.AllowedEnv {
			fmt.Println(v)
		}
		return nil
	},
}

var sandboxPortsCmd = &cobra.Command{
	Use:   "ports",
	Short: "List allowed localhost ports",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		cfg, _ := config.Load(root)
		if cfg == nil {
			cfg = &config.Config{}
		}
		if len(cfg.Sandbox.AllowedPorts) == 0 {
			fmt.Println("No extra ports configured. Only the proxy port (3128) is accessible.")
			return nil
		}
		for _, p := range cfg.Sandbox.AllowedPorts {
			fmt.Println(p)
		}
		return nil
	},
}

// sandboxConfigMutator reduces boilerplate for simple config mutations.
func sandboxConfigMutator(mutate func(sb *config.SandboxConfig, args []string)) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		cfg, _ := config.Load(root)
		if cfg == nil {
			cfg = &config.Config{}
		}
		mutate(&cfg.Sandbox, args)
		return config.Save(root, cfg)
	}
}

func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

func removeItem(slice []string, item string) []string {
	var out []string
	for _, s := range slice {
		if s != item {
			out = append(out, s)
		}
	}
	return out
}

func appendUniqueInt(slice []int, item int) []int {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

func removeIntItem(slice []int, item int) []int {
	var out []int
	for _, s := range slice {
		if s != item {
			out = append(out, s)
		}
	}
	return out
}

func sandboxFormatList(items []string) string {
	if len(items) == 0 {
		return "(none)"
	}
	return strings.Join(items, ", ")
}

func init() {
	sandboxRunCmd.Flags().Bool("force-dir", false, "Skip directory safety checks")
	sandboxRunCmd.Flags().Bool("no-network", false, "Block all network egress (no proxy)")
	sandboxRunCmd.Flags().StringArray("allow-domain", nil, "Allow an additional domain for this session")
	sandboxRunCmd.Flags().StringArray("allow-env", nil, "Forward an additional env var into the sandbox")
	sandboxRunCmd.Flags().StringArray("allow-port", nil, "Allow a localhost port inside the sandbox")
	sandboxRunCmd.Flags().StringArray("mount-ro", nil, "Mount additional path read-only inside sandbox")

	sandboxCmd.AddCommand(
		sandboxRunCmd,
		sandboxCheckCmd,
		sandboxInfoCmd,
		sandboxAllowDomainCmd,
		sandboxDenyDomainCmd,
		sandboxAllowEnvCmd,
		sandboxDenyEnvCmd,
		sandboxAllowPortCmd,
		sandboxDenyPortCmd,
		sandboxDomainsCmd,
		sandboxEnvCmd,
		sandboxPortsCmd,
	)
	rootCmd.AddCommand(sandboxCmd)
}
