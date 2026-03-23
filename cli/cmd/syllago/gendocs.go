package main

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// CommandManifest is the top-level JSON structure output by _gendocs.
type CommandManifest struct {
	Version        string         `json:"version"`
	GeneratedAt    string         `json:"generatedAt"`
	SyllagoVersion string         `json:"syllagoVersion"`
	Commands       []CommandEntry `json:"commands"`
}

// CommandEntry represents a single CLI command in the manifest.
type CommandEntry struct {
	Name            string   `json:"name"`
	DisplayName     string   `json:"displayName"`
	Slug            string   `json:"slug"`
	Parent          *string  `json:"parent"`
	Synopsis        string   `json:"synopsis"`
	Description     string   `json:"description"`
	LongDescription *string  `json:"longDescription"`
	Aliases         []string `json:"aliases"`
	Flags           []Flag   `json:"flags"`
	InheritedFlags  []Flag   `json:"inheritedFlags"`
	Subcommands     []string `json:"subcommands"`
	SeeAlso         []string `json:"seeAlso"`
	Examples        *string  `json:"examples"`
	Source          string   `json:"source"`
}

// Flag describes a single command-line flag.
type Flag struct {
	Name        string  `json:"name"`
	Shorthand   *string `json:"shorthand"`
	Type        string  `json:"type"`
	Default     *string `json:"default"`
	Required    bool    `json:"required"`
	Description string  `json:"description"`
}

var gendocsCmd = &cobra.Command{
	Use:    "_gendocs",
	Short:  "Generate commands.json manifest",
	Hidden: true,
	RunE:   runGendocs,
}

func init() {
	rootCmd.AddCommand(gendocsCmd)
}

func runGendocs(cmd *cobra.Command, args []string) error {
	var entries []CommandEntry
	walkCommands(rootCmd, nil, &entries)

	v := version
	if v == "" {
		v = "dev"
	}

	manifest := CommandManifest{
		Version:        "1",
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		SyllagoVersion: v,
		Commands:       entries,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(manifest)
}

// walkCommands recursively walks the command tree and appends entries.
// It skips hidden commands (backfill, _gendocs itself) and the root command
// (which is the TUI launcher, not a real CLI command).
func walkCommands(cmd *cobra.Command, parentName *string, entries *[]CommandEntry) {
	for _, child := range cmd.Commands() {
		if child.Hidden {
			continue
		}
		// Skip help — Cobra auto-generates it
		if child.Name() == "help" {
			continue
		}

		entry := buildEntry(child, parentName)
		*entries = append(*entries, entry)

		// Recurse into subcommands
		if child.HasSubCommands() {
			name := entry.Name
			walkCommands(child, &name, entries)
		}
	}
}

func buildEntry(cmd *cobra.Command, parentName *string) CommandEntry {
	name := commandFullName(cmd)
	slug := strings.ReplaceAll(name, " ", "-")

	// Display name: title-case each word
	displayName := toDisplayName(name)

	// Synopsis: the full usage line.
	// cmd.UseLine() already includes the full path from root (e.g. "syllago export [flags]"),
	// so we use it directly instead of prepending "syllago".
	synopsis := cmd.UseLine()

	// Long description
	var longDesc *string
	if cmd.Long != "" {
		s := cmd.Long
		longDesc = &s
	}

	// Aliases (skip if empty)
	aliases := cmd.Aliases
	if aliases == nil {
		aliases = []string{}
	}

	// Flags
	flags := extractFlags(cmd.LocalNonPersistentFlags(), cmd)
	inheritedFlags := extractFlags(cmd.InheritedFlags(), cmd)

	// Subcommands
	var subcommands []string
	for _, sub := range cmd.Commands() {
		if !sub.Hidden && sub.Name() != "help" {
			subcommands = append(subcommands, sub.Name())
		}
	}
	if subcommands == nil {
		subcommands = []string{}
	}

	// See also: parent + siblings for leaf commands
	seeAlso := buildSeeAlso(cmd)

	// Examples
	var examples *string
	if cmd.Example != "" {
		s := cmd.Example
		examples = &s
	}

	// Source file: derive from command structure
	source := deriveSourceFile(cmd)

	return CommandEntry{
		Name:            name,
		DisplayName:     displayName,
		Slug:            slug,
		Parent:          parentName,
		Synopsis:        synopsis,
		Description:     cmd.Short,
		LongDescription: longDesc,
		Aliases:         aliases,
		Flags:           flags,
		InheritedFlags:  inheritedFlags,
		Subcommands:     subcommands,
		SeeAlso:         seeAlso,
		Examples:        examples,
		Source:          source,
	}
}

// commandFullName returns the space-separated full name like "registry sync".
func commandFullName(cmd *cobra.Command) string {
	parts := []string{}
	for c := cmd; c != nil && c != rootCmd; c = c.Parent() {
		parts = append([]string{c.Name()}, parts...)
	}
	return strings.Join(parts, " ")
}

// toDisplayName converts "registry sync" to "Registry Sync".
func toDisplayName(name string) string {
	words := strings.Fields(name)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func extractFlags(fs *pflag.FlagSet, cmd *cobra.Command) []Flag {
	var flags []Flag
	fs.VisitAll(func(f *pflag.Flag) {
		flag := Flag{
			Name:        "--" + f.Name,
			Type:        f.Value.Type(),
			Description: f.Usage,
		}
		if f.Shorthand != "" {
			s := "-" + f.Shorthand
			flag.Shorthand = &s
		}
		if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "[]" {
			flag.Default = &f.DefValue
		}

		// Check if flag is marked required via Cobra annotations
		if ann := cmd.Flags().Lookup(f.Name); ann != nil {
			if vals, ok := ann.Annotations[cobra.BashCompOneRequiredFlag]; ok {
				for _, v := range vals {
					if v == "true" {
						flag.Required = true
					}
				}
			}
		}

		flags = append(flags, flag)
	})
	if flags == nil {
		flags = []Flag{}
	}
	return flags
}

// buildSeeAlso returns related commands for cross-linking.
// For leaf commands: includes parent and siblings.
// For parent commands: includes its children.
func buildSeeAlso(cmd *cobra.Command) []string {
	var related []string

	if cmd.HasParent() && cmd.Parent() != rootCmd {
		// Add parent
		parentName := commandFullName(cmd.Parent())
		related = append(related, parentName)

		// Add siblings
		for _, sibling := range cmd.Parent().Commands() {
			if sibling != cmd && !sibling.Hidden && sibling.Name() != "help" {
				related = append(related, commandFullName(sibling))
			}
		}
	}

	if related == nil {
		related = []string{}
	}
	return related
}

// deriveSourceFile maps a command to its likely source file path.
// Follows the codebase convention: top-level commands are <name>.go,
// parent commands are <name>_cmd.go, subcommands are <parent>_<name>.go.
func deriveSourceFile(cmd *cobra.Command) string {
	base := "cli/cmd/syllago/"

	if cmd.Parent() == rootCmd {
		// Top-level command
		if cmd.HasSubCommands() {
			return base + cmd.Name() + "_cmd.go"
		}
		return base + cmd.Name() + ".go"
	}

	// Subcommand: check for special patterns
	parent := cmd.Parent()
	if parent != nil {
		parentName := parent.Name()
		childName := cmd.Name()

		// Some subcommands have their own files: loadout_apply.go, etc.
		// Others are inline in the parent _cmd.go file.
		// We'll point to the parent _cmd.go since that's where most are defined,
		// but check for specific file patterns that exist.
		specificFile := parentName + "_" + childName + ".go"
		fullPath := base + specificFile

		// For known multi-file parents, use the specific file
		switch parentName {
		case "loadout":
			return fullPath
		default:
			// Default: parent _cmd.go contains the subcommand definitions
			return base + parentName + "_cmd.go"
		}
	}

	return base + cmd.Name() + ".go"
}
