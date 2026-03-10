package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"gopkg.in/yaml.v3"
)

const gitignoreContent = `# OS files
.DS_Store
Thumbs.db

# Editor files
*.swp
*~
.vscode/
.idea/

# Misc
*.bak
*.tmp
`

const exampleSkillContent = `---
name: Hello World
description: An example skill. Replace this with your own content.
---

# Hello World

This is a placeholder skill. Replace this file with your actual skill content.

## Instructions

Describe what the AI should do here. This text becomes part of the AI's context.
`

const exampleSkillMetaContent = `id: hello-world
name: Hello World
description: An example skill showing the universal content layout.
tags:
  - example
`

const exampleRuleContent = `# Example Rule

This is a placeholder rule. Replace this file with your actual rule content.

Rules define behavior expectations for the AI tool. Each rule should focus
on one specific guideline.
`

const exampleRuleReadmeContent = `# example-rule

An example rule showing the provider-specific directory layout.

## Usage

Export this rule to your Claude Code project:

    syllago export --to claude-code

## Details

Replace this README with a description of what your rule does.
`

const exampleRuleMetaContent = `id: example-rule
name: example-rule
description: An example rule showing the provider-specific directory layout.
tags:
  - example
`

// contributingContent generates the CONTRIBUTING.md body for a new registry.
func contributingContent(name string) string {
	var b strings.Builder
	b.WriteString("# Contributing to " + name + "\n\n")
	b.WriteString("This is a syllago registry. Follow the conventions below to add content.\n\n")
	b.WriteString("## Directory Structure\n\n")
	b.WriteString("### Universal content (works with any AI tool)\n\n")
	b.WriteString("```\n")
	b.WriteString("skills/<name>/\n")
	b.WriteString("    SKILL.md          # Required. Frontmatter (name, description) + instructions.\n")
	b.WriteString("    .syllago.yaml     # Optional metadata (tags, version, author).\n")
	b.WriteString("    README.md         # Optional. Human-readable docs.\n\n")
	b.WriteString("agents/<name>/\n")
	b.WriteString("    AGENT.md          # Required. Frontmatter + agent specification.\n")
	b.WriteString("    .syllago.yaml     # Optional metadata.\n")
	b.WriteString("    README.md         # Optional.\n\n")
	b.WriteString("mcp/<name>/\n")
	b.WriteString("    README.md         # Required. Describes the MCP server configuration.\n")
	b.WriteString("    .syllago.yaml     # Optional metadata.\n")
	b.WriteString("```\n\n")
	b.WriteString("### Provider-specific content\n\n")
	b.WriteString("```\n")
	b.WriteString("rules/<provider>/<name>/\n")
	b.WriteString("    rule.md           # Required. The rule content.\n")
	b.WriteString("    README.md         # Required. Human-readable description.\n")
	b.WriteString("    .syllago.yaml     # Optional metadata.\n\n")
	b.WriteString("hooks/<provider>/<name>/\n")
	b.WriteString("    hook.json         # Required. Hook configuration.\n")
	b.WriteString("    README.md         # Required.\n")
	b.WriteString("    .syllago.yaml     # Optional metadata.\n\n")
	b.WriteString("commands/<provider>/<name>/\n")
	b.WriteString("    command.md        # Required. Command definition.\n")
	b.WriteString("    README.md         # Required.\n")
	b.WriteString("    .syllago.yaml     # Optional metadata.\n")
	b.WriteString("```\n\n")
	b.WriteString("Supported provider slugs: `claude-code`, `cursor`, `copilot`, `windsurf`, `zed`, `aider`, `continue`, `gemini-cli`, `amp`.\n\n")
	b.WriteString("## Naming Conventions\n\n")
	b.WriteString("- Use lowercase letters, numbers, hyphens, and underscores only.\n")
	b.WriteString("- No spaces, dots, or special characters.\n")
	b.WriteString("- Examples: `code-review`, `test_helper`, `my-rule-v2`\n\n")
	b.WriteString("## registry.yaml\n\n")
	b.WriteString("The `registry.yaml` at the root describes this registry:\n\n")
	b.WriteString("```yaml\n")
	b.WriteString("name: " + name + "\n")
	b.WriteString("description: A short description of this registry.\n")
	b.WriteString("version: 0.1.0\n")
	b.WriteString("```\n\n")
	b.WriteString("Bump `version` when you make significant changes.\n\n")
	b.WriteString("## Example Content\n\n")
	b.WriteString("This registry includes example content in `skills/hello-world/` and\n")
	b.WriteString("`rules/claude-code/example-rule/`. These are tagged `example` in their\n")
	b.WriteString("`.syllago.yaml` and can be safely deleted once you add real content.\n")
	return b.String()
}

// Scaffold creates a new registry directory structure at targetDir/name.
// It creates subdirectories for each content type, a registry.yaml manifest,
// and a README.md with basic usage instructions.
//
// Returns an error if the name is invalid or the directory already exists.
func Scaffold(targetDir, name, description string) error {
	if !catalog.IsValidItemName(name) {
		return fmt.Errorf("registry name %q contains invalid characters (use letters, numbers, - and _)", name)
	}

	dir := filepath.Join(targetDir, name)
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("directory %q already exists", dir)
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating registry directory: %w", err)
	}

	// Create content type directories with .gitkeep so git tracks them.
	for _, ct := range catalog.AllContentTypes() {
		ctDir := filepath.Join(dir, string(ct))
		if err := os.MkdirAll(ctDir, 0755); err != nil {
			return fmt.Errorf("creating %s directory: %w", ct, err)
		}
		if err := os.WriteFile(filepath.Join(ctDir, ".gitkeep"), []byte(""), 0644); err != nil {
			return fmt.Errorf("creating .gitkeep in %s: %w", ct, err)
		}
	}

	// Write registry.yaml using the Manifest struct for format consistency.
	desc := description
	if desc == "" {
		desc = fmt.Sprintf("%s registry", name)
	}
	manifest := Manifest{
		Name:        name,
		Description: desc,
		Version:     "0.1.0",
	}
	yamlBytes, err := yaml.Marshal(&manifest)
	if err != nil {
		return fmt.Errorf("marshalling registry.yaml: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "registry.yaml"), yamlBytes, 0644); err != nil {
		return fmt.Errorf("writing registry.yaml: %w", err)
	}

	// Write README.md with basic usage instructions.
	var lines []string
	lines = append(lines, "# "+name, "", desc, "")
	lines = append(lines, "## Using this registry", "")
	lines = append(lines, "```sh", "syllago registry add <git-url>", "syllago registry sync", "```", "")
	lines = append(lines, "## Structure", "")
	for _, ct := range catalog.AllContentTypes() {
		lines = append(lines, fmt.Sprintf("- `%s/` -- %s", ct, ct.Label()))
	}
	lines = append(lines, "")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(strings.Join(lines, "\n")), 0644); err != nil {
		return fmt.Errorf("writing README.md: %w", err)
	}

	// Write .gitignore.
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(gitignoreContent), 0644); err != nil {
		return fmt.Errorf("writing .gitignore: %w", err)
	}

	// Create example skill: skills/hello-world/
	skillDir := filepath.Join(dir, "skills", "hello-world")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("creating example skill directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(exampleSkillContent), 0644); err != nil {
		return fmt.Errorf("writing example SKILL.md: %w", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, ".syllago.yaml"), []byte(exampleSkillMetaContent), 0644); err != nil {
		return fmt.Errorf("writing example skill metadata: %w", err)
	}

	// Create example rule: rules/claude-code/example-rule/
	ruleDir := filepath.Join(dir, "rules", "claude-code", "example-rule")
	if err := os.MkdirAll(ruleDir, 0755); err != nil {
		return fmt.Errorf("creating example rule directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(ruleDir, "rule.md"), []byte(exampleRuleContent), 0644); err != nil {
		return fmt.Errorf("writing example rule.md: %w", err)
	}
	if err := os.WriteFile(filepath.Join(ruleDir, "README.md"), []byte(exampleRuleReadmeContent), 0644); err != nil {
		return fmt.Errorf("writing example rule README.md: %w", err)
	}
	if err := os.WriteFile(filepath.Join(ruleDir, ".syllago.yaml"), []byte(exampleRuleMetaContent), 0644); err != nil {
		return fmt.Errorf("writing example rule metadata: %w", err)
	}

	// Write CONTRIBUTING.md.
	if err := os.WriteFile(filepath.Join(dir, "CONTRIBUTING.md"), []byte(contributingContent(name)), 0644); err != nil {
		return fmt.Errorf("writing CONTRIBUTING.md: %w", err)
	}

	return nil
}
