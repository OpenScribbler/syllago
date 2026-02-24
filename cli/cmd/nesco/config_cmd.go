package main

import (
	"fmt"
	"strings"

	"github.com/OpenScribbler/nesco/cli/internal/config"
	"github.com/OpenScribbler/nesco/cli/internal/output"
	"github.com/OpenScribbler/nesco/cli/internal/provider"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View and edit nesco configuration",
	Long:  "Manage provider selection and preferences in .nesco/config.json.",
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		cfg, err := config.Load(root)
		if err != nil {
			return err
		}
		if output.JSON {
			output.Print(cfg)
		} else {
			if len(cfg.Providers) == 0 {
				fmt.Println("No providers configured. Run `nesco init` to set up.")
			} else {
				fmt.Println("Configured providers:")
				for _, p := range cfg.Providers {
					fmt.Printf("  - %s\n", p)
				}
			}
		}
		return nil
	},
}

var configAddCmd = &cobra.Command{
	Use:   "add <provider-slug>",
	Short: "Add a provider to the configuration",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		cfg, err := config.Load(root)
		if err != nil {
			return err
		}
		slug := args[0]
		for _, p := range cfg.Providers {
			if p == slug {
				return fmt.Errorf("provider %q already configured", slug)
			}
		}
		// Warn about unknown provider slugs (but still allow them)
		if findProviderBySlug(slug) == nil {
			var slugs []string
			for _, p := range provider.AllProviders {
				slugs = append(slugs, p.Slug)
			}
			fmt.Fprintf(output.ErrWriter, "Warning: '%s' is not a known provider slug.\n", slug)
			fmt.Fprintf(output.ErrWriter, "  Known providers: %s\n", strings.Join(slugs, ", "))
			fmt.Fprintf(output.ErrWriter, "  Adding anyway — unknown providers are ignored during scan.\n")
		}

		cfg.Providers = append(cfg.Providers, slug)
		if err := config.Save(root, cfg); err != nil {
			return err
		}
		fmt.Fprintf(output.Writer, "Added provider: %s\n", slug)
		return nil
	},
}

var configRemoveCmd = &cobra.Command{
	Use:   "remove <provider-slug>",
	Short: "Remove a provider from the configuration",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := findProjectRoot()
		if err != nil {
			return err
		}
		cfg, err := config.Load(root)
		if err != nil {
			return err
		}
		slug := args[0]
		var filtered []string
		found := false
		for _, p := range cfg.Providers {
			if p == slug {
				found = true
				continue
			}
			filtered = append(filtered, p)
		}
		if !found {
			return fmt.Errorf("provider %q not found in config", slug)
		}
		cfg.Providers = filtered
		if err := config.Save(root, cfg); err != nil {
			return err
		}
		fmt.Fprintf(output.Writer, "Removed provider: %s\n", slug)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configListCmd, configAddCmd, configRemoveCmd)
	rootCmd.AddCommand(configCmd)
}
