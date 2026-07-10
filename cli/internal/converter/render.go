package converter

import (
	"bytes"
	"fmt"
	"strings"
)

// renderFrontmatterDoc marshals meta as YAML frontmatter and joins it to
// body with a blank line, producing the standard document layout shared by
// every frontmatter-emitting renderer and canonical builder:
//
//	---
//	<yaml>
//	---
//
//	<body>
func renderFrontmatterDoc(meta any, body string) ([]byte, error) {
	fm, err := renderFrontmatter(meta)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.Write(fm)
	buf.WriteString("\n")
	buf.WriteString(body)
	buf.WriteString("\n")
	return buf.Bytes(), nil
}

// renderWithFrontmatter is the shared render tail: frontmatter document plus
// output filename. Callers that emit warnings set Result.Warnings afterwards.
func renderWithFrontmatter(meta any, body, filename string) (*Result, error) {
	content, err := renderFrontmatterDoc(meta, body)
	if err != nil {
		return nil, err
	}
	return &Result{Content: content, Filename: filename}, nil
}

// plainRuleSpec describes a "plain markdown" rule renderer: providers whose
// rule output is the bare body, with activation scope carried either by a
// small glob-only frontmatter block, a prose scope note, or an HTML comment
// plus conversion warnings.
type plainRuleSpec struct {
	// filename is the output filename for the plain (non-glob-frontmatter)
	// branches.
	filename string
	// filenameFromDescription derives the filename from
	// slugify(meta.Description)+".md" when a description is present
	// (Roo Code convention). filename is the fallback.
	filenameFromDescription bool
	// globMeta, when non-nil, builds provider-native frontmatter for
	// glob-scoped rules; the result is written to globFilename. When nil,
	// globs fall through to the prose scope note (or globsWarning).
	globMeta     func(globs []string) any
	globFilename string
	// commentScope switches to the Zed/Roo Code shape: description rendered
	// as an HTML comment header, unsupported activation fields surfaced as
	// warnings instead of prose scope notes.
	commentScope bool
	// globsWarning is a fmt string (one %s: joined globs) emitted when
	// commentScope is set and globs are present.
	globsWarning string
	// conditionalWarning is emitted when commentScope is set and the rule
	// is not always-apply.
	conditionalWarning string
}

// plainRuleSpecs is the descriptor table for the plain-markdown renderer
// family. Windsurf, Kiro, and Cursor renderers stay bespoke — they have
// real frontmatter/trigger logic.
var plainRuleSpecs = map[string]plainRuleSpec{
	// Claude Code: glob-scoped rules use native paths frontmatter
	// (.claude/rules/*.md format); always-apply rules are plain markdown.
	"claude-code": {
		filename:     "rule.md",
		globFilename: "rule.md",
		globMeta: func(globs []string) any {
			return claudeCodePathsFrontmatter{Paths: globs}
		},
	},
	// Copilot CLI: glob-scoped rules use applyTo frontmatter
	// (.instructions.md); always-apply rules become copilot-instructions.md.
	"copilot-cli": {
		filename:     "copilot-instructions.md",
		globFilename: ".instructions.md",
		globMeta: func(globs []string) any {
			return copilotFrontmatter{ApplyTo: strings.Join(globs, ", ")}
		},
	},
	// Amp: glob-scoped rules use a globs array in AGENTS.md frontmatter,
	// with the implicit **/ prefix stripped to avoid double-prefixing.
	"amp": {
		filename:     "AGENTS.md",
		globFilename: "AGENTS.md",
		globMeta: func(globs []string) any {
			ampGlobs := make([]string, len(globs))
			for i, g := range globs {
				ampGlobs[i] = stripImplicitGlobPrefix(g)
			}
			return ampRuleFrontmatter{Globs: ampGlobs}
		},
	},
	// Codex, Gemini CLI: flat markdown file; scope embedded as prose.
	"codex":      {filename: "rule.md"},
	"gemini-cli": {filename: "rule.md"},
	// OpenCode: plain markdown AGENTS.md; scope embedded as prose.
	"opencode": {filename: "AGENTS.md"},
	// Zed: plain .rules file; no conditional activation support.
	"zed": {
		filename:           ".rules",
		commentScope:       true,
		globsWarning:       "Zed does not support glob-scoped rules; globs (%s) will be ignored",
		conditionalWarning: "Zed does not support conditional activation; rule will be applied unconditionally",
	},
	// Roo Code: plain markdown in .roo/rules/; mode-based scoping only.
	"roo-code": {
		filename:                "rule.md",
		filenameFromDescription: true,
		commentScope:            true,
		globsWarning:            "Roo Code uses mode-based scoping, not glob scoping; globs (%s) will be ignored",
	},
}

// renderPlainMarkdownRule renders a rule for the plain-markdown provider
// family according to spec. See plainRuleSpecs for the per-provider
// strategies.
func renderPlainMarkdownRule(meta RuleMeta, body string, spec plainRuleSpec) (*Result, error) {
	if spec.globMeta != nil && len(meta.Globs) > 0 {
		return renderWithFrontmatter(spec.globMeta(meta.Globs), body, spec.globFilename)
	}

	filename := spec.filename
	if spec.filenameFromDescription && meta.Description != "" {
		filename = slugify(meta.Description) + ".md"
	}

	if spec.commentScope {
		var warnings []string
		if spec.globsWarning != "" && len(meta.Globs) > 0 {
			warnings = append(warnings, fmt.Sprintf(spec.globsWarning, strings.Join(meta.Globs, ", ")))
		}
		if spec.conditionalWarning != "" && !meta.AlwaysApply {
			warnings = append(warnings, spec.conditionalWarning)
		}

		var buf bytes.Buffer
		if meta.Description != "" {
			buf.WriteString("<!-- ")
			buf.WriteString(meta.Description)
			buf.WriteString(" -->\n\n")
		}
		buf.WriteString(body)
		buf.WriteString("\n")
		return &Result{Content: buf.Bytes(), Filename: filename, Warnings: warnings}, nil
	}

	if meta.AlwaysApply {
		// Always-active rules get body only — no frontmatter
		return &Result{Content: []byte(body + "\n"), Filename: filename}, nil
	}

	// Embed activation scope as prose
	var notes []string
	switch {
	case spec.globMeta == nil && len(meta.Globs) > 0:
		notes = append(notes, fmt.Sprintf("**Scope:** Apply only when working with files matching: %s", strings.Join(meta.Globs, ", ")))
	case meta.Description != "":
		notes = append(notes, fmt.Sprintf("**Scope:** Apply when: %s", meta.Description))
	default:
		notes = append(notes, "**Scope:** Apply only when explicitly asked.")
	}

	notesBlock := BuildConversionNotes("syllago", notes)
	result := AppendNotes(body, notesBlock)
	return &Result{Content: []byte(result + "\n"), Filename: filename}, nil
}
