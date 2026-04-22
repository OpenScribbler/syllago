package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/spf13/cobra"
	"github.com/tidwall/gjson"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect <type>/<name>",
	Short: "Show details about a content item",
	Long: `Display full details about a content item for pre-install auditing.

By default, shows metadata and the primary content file. Use --as to preview
what the content would look like converted to a specific provider's format.

Path formats:
  skills/my-skill                   Universal content (type/name)
  rules/claude-code/my-rule         Provider-specific (type/provider/name)`,
	Example: `  # Inspect a skill (shows metadata + content)
  syllago inspect skills/my-skill

  # Preview what a rule looks like in Cursor format
  syllago inspect rules/coding-standards --as cursor

  # Compare formats side by side
  syllago inspect rules/coding-standards --as claude-code
  syllago inspect rules/coding-standards --as cursor

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
	inspectCmd.Flags().String("as", "", "Preview content converted to a provider's format")
}

// inspectResult is the JSON-serializable output for syllago inspect.
type inspectResult struct {
	Name             string            `json:"name"`
	Type             string            `json:"type"`
	Source           string            `json:"source"`
	Provider         string            `json:"provider,omitempty"`
	Path             string            `json:"path"`
	Description      string            `json:"description,omitempty"`
	Files            []string          `json:"files,omitempty"`
	Risks            []inspectRisk     `json:"risks,omitempty"`
	FileContents     map[string]string `json:"file_contents,omitempty"`
	Compatibility    []compatResult    `json:"compatibility,omitempty"`
	DetailedRisks    []riskDetail      `json:"detailed_risks,omitempty"`
	AsProvider       string            `json:"as_provider,omitempty"`
	AsContent        string            `json:"as_content,omitempty"`
	AsWarnings       []string          `json:"as_warnings,omitempty"`
	Trust            string            `json:"trust,omitempty"`             // collapsed label (Verified/Recalled/"")
	TrustTier        string            `json:"trust_tier,omitempty"`        // full tier (Dual-Attested/Signed/Unsigned/"")
	TrustDescription string            `json:"trust_description,omitempty"` // drill-down text
	Recalled         bool              `json:"recalled,omitempty"`
	RecallReason     string            `json:"recall_reason,omitempty"`
	RecallSource     string            `json:"recall_source,omitempty"`
	RecallIssuer     string            `json:"recall_issuer,omitempty"`
	RecallDetailsURL string            `json:"recall_details_url,omitempty"`
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
		return output.NewStructuredErrorDetail(output.ErrCatalogNotFound, "could not find syllago repo", "Run 'syllago init' to set up a content repository", err.Error())
	}

	projectRoot, _ := findProjectRoot()
	if projectRoot == "" {
		projectRoot = root
	}
	scan, err := moat.LoadAndScan(root, projectRoot, time.Now())
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrCatalogScanFailed, "scanning catalog failed", "Check that the content directory exists and is readable", err.Error())
	}
	cat := scan.Catalog

	item, err := findItemByPath(cat, args[0])
	if err != nil {
		return err
	}

	risks := catalog.RiskIndicators(*item)

	badge := catalog.UserFacingBadge(item.TrustTier, item.Recalled)
	result := inspectResult{
		Name:             item.Name,
		Type:             item.Type.Label(),
		Source:           sourceLabel(*item),
		Provider:         item.Provider,
		Path:             item.Path,
		Description:      item.Description,
		Files:            item.Files,
		Trust:            badge.Label(),
		TrustTier:        item.TrustTier.String(),
		TrustDescription: catalog.TrustDescription(item.TrustTier, item.Recalled, item.RecallReason),
		Recalled:         item.Recalled,
		RecallReason:     item.RecallReason,
		RecallSource:     item.RecallSource,
		RecallIssuer:     item.RecallIssuer,
		RecallDetailsURL: item.RecallDetailsURL,
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

	// --as <provider>: render the content in a provider's native format.
	asSlug, _ := cmd.Flags().GetString("as")
	if asSlug != "" {
		rendered, provName, err := renderAsProvider(*item, asSlug)
		if err != nil {
			return err
		}
		result.AsProvider = provName
		result.AsContent = string(rendered.Content)
		result.AsWarnings = rendered.Warnings
	}

	if output.JSON {
		output.Print(result)
		return nil
	}

	// --as mode: show the rendered content with a header, skip metadata.
	if asSlug != "" {
		fmt.Fprintf(output.Writer, "# %s as %s\n\n", item.Name, result.AsProvider)
		fmt.Fprint(output.Writer, result.AsContent)
		if !strings.HasSuffix(result.AsContent, "\n") {
			fmt.Fprintln(output.Writer)
		}
		if len(result.AsWarnings) > 0 {
			fmt.Fprintf(output.ErrWriter, "\n  Portability warnings:\n")
			for _, w := range result.AsWarnings {
				fmt.Fprintf(output.ErrWriter, "    - %s\n", w)
			}
		}
		return nil
	}

	// Plain text output (default mode).
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
	if result.TrustDescription != "" {
		fmt.Fprintf(output.Writer, "Trust:   %s\n", result.TrustDescription)
		if item.Recalled {
			if item.RecallIssuer != "" {
				fmt.Fprintf(output.Writer, "         issuer: %s\n", item.RecallIssuer)
			}
			if item.RecallSource != "" {
				fmt.Fprintf(output.Writer, "         source: %s\n", item.RecallSource)
			}
			if item.RecallDetailsURL != "" {
				fmt.Fprintf(output.Writer, "         details: %s\n", item.RecallDetailsURL)
			}
		}
	}

	if len(item.Files) > 0 {
		fmt.Fprintf(output.Writer, "\nFiles:\n")
		for _, f := range item.Files {
			fmt.Fprintf(output.Writer, "  %s\n", f)
		}
	}

	// Show primary content by default (without needing --files).
	primaryFile := catalog.PrimaryFileName(item.Files, item.Type)
	if primaryFile != "" {
		content, err := catalog.ReadFileContent(item.Path, primaryFile, 200)
		if err == nil {
			fmt.Fprintf(output.Writer, "\n--- %s ---\n%s\n", primaryFile, content)
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

// renderAsProvider converts a library item to a target provider's format.
func renderAsProvider(item catalog.ContentItem, provSlug string) (*converter.Result, string, error) {
	prov := findProviderBySlug(provSlug)
	if prov == nil {
		slugs := providerSlugs()
		return nil, "", output.NewStructuredError(output.ErrProviderNotFound, "unknown provider: "+provSlug, "Available: "+strings.Join(slugs, ", "))
	}

	conv := converter.For(item.Type)
	if conv == nil {
		return nil, "", output.NewStructuredError(output.ErrConvertNotSupported, fmt.Sprintf("%s does not support format conversion", item.Type.Label()), "")
	}

	contentFile := converter.ResolveContentFile(item)
	if contentFile == "" {
		return nil, "", output.NewStructuredError(output.ErrItemNotFound, fmt.Sprintf("cannot locate content file for %s", item.Name), "")
	}

	raw, err := os.ReadFile(contentFile)
	if err != nil {
		return nil, "", output.NewStructuredErrorDetail(output.ErrSystemIO, "reading content failed", "", err.Error())
	}

	srcProvider := ""
	if item.Meta != nil {
		srcProvider = item.Meta.SourceProvider
	}
	if srcProvider == "" && item.Provider != "" {
		srcProvider = item.Provider
	}

	canonical, err := conv.Canonicalize(raw, srcProvider)
	if err != nil {
		return nil, "", output.NewStructuredErrorDetail(output.ErrConvertParseFailed, "canonicalizing content failed", "", err.Error())
	}

	rendered, err := conv.Render(canonical.Content, *prov)
	if err != nil {
		return nil, "", output.NewStructuredErrorDetail(output.ErrConvertRenderFailed, fmt.Sprintf("rendering to %s format failed", prov.Name), "", err.Error())
	}

	return rendered, prov.Name, nil
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
