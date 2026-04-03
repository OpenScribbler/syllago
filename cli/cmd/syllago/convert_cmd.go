package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
	"github.com/spf13/cobra"
)

// convertResult is the JSON-serializable output for syllago convert.
type convertResult struct {
	Name     string   `json:"name"`
	From     string   `json:"from"`
	To       string   `json:"to"`
	Output   string   `json:"output,omitempty"`   // path if --output was used
	Content  string   `json:"content,omitempty"`  // rendered content (when no --output)
	Warnings []string `json:"warnings,omitempty"` // portability warnings from conversion
}

var convertCmd = &cobra.Command{
	Use:   "convert <file-or-name>",
	Short: "Convert content between provider formats",
	Long: `Transform content between provider formats using syllago's hub-and-spoke
conversion model (source -> canonical -> target).

Accepts either a file path or a library item name:
  - File path: reads the file directly (requires --from and --to)
  - Library name: looks up the item in your library (--from is optional)

Output goes to stdout by default, or to a file with --output.`,
	Example: `  # Convert a Cursor rule to Claude Code format
  syllago convert ./my-rule.mdc --from cursor --to claude-code

  # Convert a library item to Windsurf format
  syllago convert my-rule --to windsurf

  # Convert and save to a file
  syllago convert my-rule --to cursor --output ./cursor-rule.mdc

  # Convert a Copilot instructions file to Cursor
  syllago convert ./.github/copilot-instructions.md --from copilot-cli --to cursor --type rules`,
	Args: cobra.ExactArgs(1),
	RunE: runConvert,
}

func init() {
	convertCmd.Flags().String("to", "", "Target provider (required)")
	convertCmd.MarkFlagRequired("to")
	convertCmd.Flags().String("from", "", "Source provider (required for file input, optional for library items)")
	convertCmd.Flags().String("type", "rules", "Content type for file input (rules, hooks, skills, agents, commands, mcp)")
	convertCmd.Flags().StringP("output", "o", "", "Write output to this file path (default: stdout)")
	convertCmd.Flags().Bool("diff", false, "Show unified diff between source and converted output")
	rootCmd.AddCommand(convertCmd)
}

func runConvert(cmd *cobra.Command, args []string) error {
	input := args[0]
	toSlug, _ := cmd.Flags().GetString("to")
	fromSlug, _ := cmd.Flags().GetString("from")
	typeStr, _ := cmd.Flags().GetString("type")
	outputPath, _ := cmd.Flags().GetString("output")
	showDiff, _ := cmd.Flags().GetBool("diff")

	toProv := findProviderBySlug(toSlug)
	if toProv == nil {
		slugs := providerSlugs()
		return output.NewStructuredError(output.ErrProviderNotFound, "unknown target provider: "+toSlug, "Available: "+strings.Join(slugs, ", "))
	}

	// Determine mode: file path or library item name.
	var err error
	if isFilePath(input) {
		err = convertFile(input, fromSlug, toSlug, typeStr, outputPath, *toProv, showDiff)
	} else {
		err = convertLibraryItem(input, fromSlug, toSlug, outputPath, *toProv, showDiff)
	}
	if err == nil {
		telemetry.Track("command_executed", map[string]any{
			"command":       "convert",
			"from_provider": fromSlug,
			"to_provider":   toSlug,
			"content_type":  typeStr,
			"success":       true,
		})
	}
	return err
}

// isFilePath returns true if the input exists on disk as a file.
func isFilePath(input string) bool {
	info, err := os.Stat(input)
	return err == nil && !info.IsDir()
}

// convertFile reads a file directly and converts it between providers.
func convertFile(path, fromSlug, toSlug, typeStr, outputPath string, toProv provider.Provider, showDiff bool) error {
	if fromSlug == "" {
		return output.NewStructuredError(output.ErrInputMissing, "--from is required when converting a file", "Example: syllago convert ./rule.mdc --from cursor --to claude-code")
	}

	if findProviderBySlug(fromSlug) == nil {
		slugs := providerSlugs()
		return output.NewStructuredError(output.ErrProviderNotFound, "unknown source provider: "+fromSlug, "Available: "+strings.Join(slugs, ", "))
	}

	ct := catalog.ContentType(typeStr)
	conv := converter.For(ct)
	if conv == nil {
		return output.NewStructuredError(output.ErrConvertNotSupported, fmt.Sprintf("no converter for content type %q", typeStr), "Supported types: rules, hooks, skills, agents, commands, mcp")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrSystemIO, "reading file failed", "Check file path and permissions", err.Error())
	}

	canonical, err := conv.Canonicalize(raw, fromSlug)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrConvertParseFailed, fmt.Sprintf("failed to parse %s as %s format", path, fromSlug), "Check that the file matches the expected provider format", err.Error())
	}

	rendered, err := conv.Render(canonical.Content, toProv)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrConvertRenderFailed, fmt.Sprintf("rendering to %s format failed", toProv.Name), "This content may not be compatible with the target provider", err.Error())
	}

	return emitConvertOutput(path, fromSlug, toSlug, outputPath, rendered, raw, showDiff)
}

// convertLibraryItem looks up an item in the library and converts it.
func convertLibraryItem(name, fromSlug, toSlug, outputPath string, toProv provider.Provider, showDiff bool) error {
	globalDir := catalog.GlobalContentDir()
	cat, err := catalog.ScanWithGlobalAndRegistries(globalDir, globalDir, nil)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrCatalogScanFailed, "scanning library failed", "Check that ~/.syllago/content/ exists and is readable", err.Error())
	}

	var item *catalog.ContentItem
	for i := range cat.Items {
		if cat.Items[i].Name == name {
			item = &cat.Items[i]
			break
		}
	}
	if item == nil {
		return output.NewStructuredError(output.ErrItemNotFound, fmt.Sprintf("no item named %q in your library", name), "Run 'syllago list' to see all library items")
	}

	conv := converter.For(item.Type)
	if conv == nil {
		return output.NewStructuredError(output.ErrConvertNotSupported, fmt.Sprintf("%s does not support format conversion", item.Type.Label()), "Supported types: rules, hooks, skills, agents, commands, mcp")
	}

	contentFile := converter.ResolveContentFile(*item)
	if contentFile == "" {
		return output.NewStructuredError(output.ErrItemNotFound, fmt.Sprintf("cannot locate content file for %s", name), "Ensure the item has a primary content file")
	}
	raw, err := os.ReadFile(contentFile)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrSystemIO, "reading content failed", "Check file permissions", err.Error())
	}

	// Determine source provider: explicit flag > metadata > item directory
	srcProvider := fromSlug
	if srcProvider == "" {
		if item.Meta != nil {
			srcProvider = item.Meta.SourceProvider
		}
		if srcProvider == "" {
			srcProvider = item.Provider
		}
	}

	canonical, err := conv.Canonicalize(raw, srcProvider)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrConvertParseFailed, "canonicalizing content failed", "Check that the content is valid for its source provider format", err.Error())
	}

	rendered, err := conv.Render(canonical.Content, toProv)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrConvertRenderFailed, fmt.Sprintf("rendering to %s format failed", toProv.Name), "This content may not be compatible with the target provider", err.Error())
	}

	displayFrom := srcProvider
	if displayFrom == "" {
		displayFrom = "(canonical)"
	}
	return emitConvertOutput(name, displayFrom, toSlug, outputPath, rendered, raw, showDiff)
}

// emitConvertOutput writes the conversion result to stdout, a file, or JSON.
func emitConvertOutput(name, fromSlug, toSlug, outputPath string, rendered *converter.Result, sourceContent []byte, showDiff bool) error {
	if rendered.Content == nil {
		return output.NewStructuredError(output.ErrConvertNotSupported, fmt.Sprintf("%s is not compatible with %s format", name, toSlug), "Try a different target provider")
	}

	if showDiff {
		return printUnifiedDiff(name, fromSlug, toSlug, sourceContent, rendered.Content)
	}

	if outputPath != "" {
		if err := os.WriteFile(outputPath, rendered.Content, 0644); err != nil {
			return output.NewStructuredErrorDetail(output.ErrSystemIO, "writing output failed", "Check that the output path is writable", err.Error())
		}
		if output.JSON {
			output.Print(convertResult{Name: name, From: fromSlug, To: toSlug, Output: outputPath, Warnings: rendered.Warnings})
		} else if !output.Quiet {
			fmt.Fprintf(output.Writer, "Converted %s (%s -> %s) to %s\n", name, fromSlug, toSlug, outputPath)
		}
	} else if output.JSON {
		output.Print(convertResult{Name: name, From: fromSlug, To: toSlug, Content: string(rendered.Content), Warnings: rendered.Warnings})
	} else {
		os.Stdout.Write(rendered.Content)
	}

	// Surface portability warnings to stderr.
	if !output.JSON && !output.Quiet && len(rendered.Warnings) > 0 {
		fmt.Fprintf(output.ErrWriter, "\n  Portability warnings:\n")
		for _, w := range rendered.Warnings {
			fmt.Fprintf(output.ErrWriter, "    - %s\n", w)
		}
		fmt.Fprintln(output.ErrWriter)
	}

	return nil
}

// printUnifiedDiff shows a simple line-by-line comparison between source and target content.
func printUnifiedDiff(name, fromSlug, toSlug string, source, target []byte) error {
	srcLines := strings.Split(string(source), "\n")
	tgtLines := strings.Split(string(target), "\n")

	fmt.Fprintf(output.Writer, "--- %s (%s)\n", name, fromSlug)
	fmt.Fprintf(output.Writer, "+++ %s (%s)\n", name, toSlug)

	// Simple diff: show all source lines as removed, all target lines as added.
	// For a real unified diff we'd need an LCS algorithm, but for conversion
	// preview, showing the full before/after is more useful than a minimal diff
	// since the formats are structurally different.
	for _, line := range srcLines {
		fmt.Fprintf(output.Writer, "-%s\n", line)
	}
	for _, line := range tgtLines {
		fmt.Fprintf(output.Writer, "+%s\n", line)
	}
	return nil
}
