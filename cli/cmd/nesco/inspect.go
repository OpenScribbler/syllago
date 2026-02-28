package main

import (
	"fmt"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
	"github.com/OpenScribbler/nesco/cli/internal/output"
	"github.com/spf13/cobra"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect <type>/<name>",
	Short: "Show details about a content item",
	Long: `Display full details about a content item for pre-install auditing.

Path formats:
  skills/my-skill                   Universal content (type/name)
  rules/claude-code/my-rule         Provider-specific (type/provider/name)

Examples:
  nesco inspect skills/my-skill
  nesco inspect rules/claude-code/my-rule
  nesco inspect --json skills/my-skill`,
	Args: cobra.ExactArgs(1),
	RunE: runInspect,
}

func init() {
	rootCmd.AddCommand(inspectCmd)
}

// inspectResult is the JSON-serializable output for nesco inspect.
type inspectResult struct {
	Name        string               `json:"name"`
	Type        string               `json:"type"`
	Source      string               `json:"source"`
	Provider    string               `json:"provider,omitempty"`
	Path        string               `json:"path"`
	Description string               `json:"description,omitempty"`
	Files       []string             `json:"files,omitempty"`
	Risks       []inspectRisk        `json:"risks,omitempty"`
}

type inspectRisk struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

func runInspect(cmd *cobra.Command, args []string) error {
	root, err := findContentRepoRoot()
	if err != nil {
		return fmt.Errorf("could not find nesco repo: %w", err)
	}

	projectRoot, _ := findProjectRoot()
	if projectRoot == "" {
		projectRoot = root
	}
	cat, err := catalog.Scan(root, projectRoot)
	if err != nil {
		return fmt.Errorf("scanning catalog: %w", err)
	}

	item, err := findItemByPath(cat, args[0])
	if err != nil {
		return err
	}

	risks := catalog.RiskIndicators(*item)

	result := inspectResult{
		Name:        item.Name,
		Type:        item.Type.Label(),
		Source:      sourceLabel(*item),
		Provider:    item.Provider,
		Path:        item.Path,
		Description: item.Description,
		Files:       item.Files,
	}
	for _, r := range risks {
		result.Risks = append(result.Risks, inspectRisk{
			Label:       r.Label,
			Description: r.Description,
		})
	}

	if output.JSON {
		output.Print(result)
		return nil
	}

	// Plain text output.
	fmt.Fprintf(output.Writer, "Name:    %s\n", item.Name)
	fmt.Fprintf(output.Writer, "Type:    %s\n", item.Type.Label())
	fmt.Fprintf(output.Writer, "Source:  %s\n", sourceLabel(*item))
	if item.Provider != "" {
		fmt.Fprintf(output.Writer, "Provider: %s\n", item.Provider)
	}
	fmt.Fprintf(output.Writer, "Path:    %s\n", item.Path)
	if item.Description != "" {
		fmt.Fprintf(output.Writer, "Desc:    %s\n", item.Description)
	}

	if len(item.Files) > 0 {
		fmt.Fprintf(output.Writer, "\nFiles:\n")
		for _, f := range item.Files {
			fmt.Fprintf(output.Writer, "  %s\n", f)
		}
	}

	if len(risks) > 0 {
		fmt.Fprintf(output.Writer, "\nRisk indicators:\n")
		for _, r := range risks {
			fmt.Fprintf(output.Writer, "  ⚠  %s — %s\n", r.Label, r.Description)
		}
	}

	return nil
}

