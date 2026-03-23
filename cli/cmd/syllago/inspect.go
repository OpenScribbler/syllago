package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/spf13/cobra"
	"github.com/tidwall/gjson"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect <type>/<name>",
	Short: "Show details about a content item",
	Long: `Display full details about a content item for pre-install auditing.

Path formats:
  skills/my-skill                   Universal content (type/name)
  rules/claude-code/my-rule         Provider-specific (type/provider/name)`,
	Example: `  # Inspect a universal skill
  syllago inspect skills/my-skill

  # Inspect a provider-specific rule
  syllago inspect rules/claude-code/my-rule

  # JSON output for scripting
  syllago inspect --json skills/my-skill`,
	Args: cobra.ExactArgs(1),
	RunE: runInspect,
}

func init() {
	rootCmd.AddCommand(inspectCmd)
	inspectCmd.Flags().Bool("files", false, "Show file contents")
	inspectCmd.Flags().Bool("compatibility", false, "Show per-provider compatibility matrix (hooks only)")
	inspectCmd.Flags().Bool("risk", false, "Show detailed risk analysis")
}

// inspectResult is the JSON-serializable output for syllago inspect.
type inspectResult struct {
	Name          string            `json:"name"`
	Type          string            `json:"type"`
	Source        string            `json:"source"`
	Provider      string            `json:"provider,omitempty"`
	Path          string            `json:"path"`
	Description   string            `json:"description,omitempty"`
	Files         []string          `json:"files,omitempty"`
	Risks         []inspectRisk     `json:"risks,omitempty"`
	FileContents  map[string]string `json:"file_contents,omitempty"`
	Compatibility []compatResult    `json:"compatibility,omitempty"`
	DetailedRisks []riskDetail      `json:"detailed_risks,omitempty"`
}

type inspectRisk struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

type compatResult struct {
	Provider string `json:"provider"`
	Level    string `json:"level"`
	Notes    string `json:"notes,omitempty"`
}

type riskDetail struct {
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Details     []string `json:"details,omitempty"`
}

func runInspect(cmd *cobra.Command, args []string) error {
	root, err := findContentRepoRoot()
	if err != nil {
		return fmt.Errorf("could not find syllago repo: %w", err)
	}

	projectRoot, _ := findProjectRoot()
	if projectRoot == "" {
		projectRoot = root
	}
	cat, err := catalog.ScanWithGlobalAndRegistries(root, projectRoot, nil)
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

	showFiles, _ := cmd.Flags().GetBool("files")

	if showFiles {
		contents := make(map[string]string, len(item.Files))
		for _, f := range item.Files {
			content, err := catalog.ReadFileContent(item.Path, f, 200)
			if err != nil {
				contents[f] = fmt.Sprintf("(error reading file: %v)", err)
			} else {
				contents[f] = content
			}
		}
		result.FileContents = contents
	}

	showCompat, _ := cmd.Flags().GetBool("compatibility")

	if showCompat && item.Type == catalog.Hooks {
		hookData, err := converter.LoadHookData(*item)
		if err == nil {
			for _, prov := range converter.HookProviders() {
				cr := converter.AnalyzeHookCompat(hookData, prov)
				result.Compatibility = append(result.Compatibility, compatResult{
					Provider: cr.Provider,
					Level:    strings.ToLower(cr.Level.Label()),
					Notes:    cr.Notes,
				})
			}
		}
	}

	showRisk, _ := cmd.Flags().GetBool("risk")

	if showRisk {
		result.DetailedRisks = buildDetailedRisks(*item)
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

	if showRisk && len(result.DetailedRisks) > 0 {
		fmt.Fprintf(output.Writer, "\nDetailed risks:\n")
		for _, rd := range result.DetailedRisks {
			fmt.Fprintf(output.Writer, "  ⚠  %s\n", rd.Label)
			for _, d := range rd.Details {
				fmt.Fprintf(output.Writer, "      %s\n", d)
			}
		}
	}

	if showCompat {
		if item.Type != catalog.Hooks {
			fmt.Fprintf(output.Writer, "\nCompatibility: not applicable for %s (hooks only)\n", item.Type.Label())
		} else if len(result.Compatibility) > 0 {
			fmt.Fprintf(output.Writer, "\nCompatibility:\n")
			for _, cr := range result.Compatibility {
				symbol := compatSymbol(cr.Level)
				if cr.Notes != "" {
					fmt.Fprintf(output.Writer, "  %s  %-14s %-10s %s\n", symbol, cr.Provider, cr.Level, cr.Notes)
				} else {
					fmt.Fprintf(output.Writer, "  %s  %-14s %s\n", symbol, cr.Provider, cr.Level)
				}
			}
		}
	}

	if showFiles && len(item.Files) > 0 {
		fmt.Fprintf(output.Writer, "\n")
		for _, f := range item.Files {
			fmt.Fprintf(output.Writer, "--- %s ---\n", f)
			if content, ok := result.FileContents[f]; ok {
				fmt.Fprintf(output.Writer, "%s\n\n", content)
			}
		}
	}

	return nil
}

// buildDetailedRisks returns riskDetail entries with specifics for MCP and hook items.
// For MCP: parses config.json to extract command, args, and env var status.
// For hooks: extracts shell command names from hook event handlers.
// For other types with basic risks (e.g. Bash-referencing skills), returns the
// risk indicators as-is without additional detail lines.
func buildDetailedRisks(item catalog.ContentItem) []riskDetail {
	var details []riskDetail

	switch item.Type {
	case catalog.MCP:
		details = append(details, mcpRiskDetails(item)...)
	case catalog.Hooks:
		details = append(details, hookRiskDetails(item)...)
	default:
		// For skills/agents with Bash risk, surface as a detail entry with no extra lines.
		for _, r := range catalog.RiskIndicators(item) {
			details = append(details, riskDetail{
				Label:       r.Label,
				Description: r.Description,
			})
		}
	}

	return details
}

// mcpRiskDetails parses config.json for an MCP item and returns detail entries
// showing command, args, and env var names with their current set/unset status.
func mcpRiskDetails(item catalog.ContentItem) []riskDetail {
	cfg, err := installer.ParseMCPConfig(item.Path)
	if err != nil {
		// Can't parse — fall back to a bare indicator without specifics.
		return []riskDetail{{
			Label:       "Runs MCP server",
			Description: "MCP server configuration (could not parse config.json)",
		}}
	}

	envStatus := installer.CheckEnvVars(cfg)

	var details []riskDetail

	if cfg.Command != "" || len(cfg.Args) > 0 {
		var cmdLines []string
		if cfg.Command != "" {
			cmdLines = append(cmdLines, "command: "+cfg.Command)
		}
		if len(cfg.Args) > 0 {
			cmdLines = append(cmdLines, "args: "+strings.Join(cfg.Args, ", "))
		}
		details = append(details, riskDetail{
			Label:       "Runs commands",
			Description: "MCP server executes a local process",
			Details:     cmdLines,
		})
	}

	if len(envStatus) > 0 {
		var envLines []string
		for name, set := range envStatus {
			status := "not set"
			if set {
				status = "set"
			}
			envLines = append(envLines, fmt.Sprintf("env: %s (%s)", name, status))
		}
		details = append(details, riskDetail{
			Label:       "Environment variables",
			Description: "MCP server reads environment variables",
			Details:     envLines,
		})
	}

	// Always note network access for MCP.
	details = append(details, riskDetail{
		Label:       "Network access",
		Description: "MCP server communicates over network",
	})

	return details
}

// hookRiskDetails reads hook JSON files and extracts command/URL values from
// each event handler, returning one riskDetail per risk category.
func hookRiskDetails(item catalog.ContentItem) []riskDetail {
	var cmdNames []string
	var hasURL bool

	for _, f := range item.Files {
		if filepath.Ext(f) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(item.Path, f))
		if err != nil {
			continue
		}
		gjson.GetBytes(data, "hooks").ForEach(func(_, eventHooks gjson.Result) bool {
			eventHooks.ForEach(func(_, hook gjson.Result) bool {
				if cmd := hook.Get("command").String(); cmd != "" {
					cmdNames = append(cmdNames, cmd)
				}
				if hook.Get("url").Exists() {
					hasURL = true
				}
				return true
			})
			return true
		})
	}

	var details []riskDetail

	if len(cmdNames) > 0 {
		seen := make(map[string]bool)
		var lines []string
		for _, c := range cmdNames {
			if !seen[c] {
				lines = append(lines, "command: "+c)
				seen[c] = true
			}
		}
		details = append(details, riskDetail{
			Label:       "Runs commands",
			Description: "Hook executes shell commands on your machine",
			Details:     lines,
		})
	}

	if hasURL {
		details = append(details, riskDetail{
			Label:       "Network access",
			Description: "Hook makes HTTP requests",
		})
	}

	return details
}

// compatSymbol returns a colored status symbol for a compat level label.
func compatSymbol(level string) string {
	switch level {
	case "full":
		return "✓"
	case "degraded":
		return "~"
	case "broken":
		return "!"
	case "none":
		return "✗"
	}
	return "?"
}
