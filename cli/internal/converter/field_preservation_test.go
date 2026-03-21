package converter

import (
	"encoding/json"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// Field preservation tests verify that every canonical field is accounted for
// when converting between providers. Each field must either:
//   - Survive as structured data (frontmatter/JSON field)
//   - Be embedded as prose (natural language in body)
//   - Be translated (e.g., tool names remapped)
//   - Generate a warning (field can't be represented, user notified)
//
// A field that silently disappears — no prose, no warning — is a bug.

type fieldTest struct {
	name     string
	target   provider.Provider
	contains []string // must appear in output
	absent   []string // must NOT appear in output
	minWarns int      // minimum expected warnings
	filename string   // expected filename (empty = skip check)
}

func runFieldTests(t *testing.T, conv interface {
	Render([]byte, provider.Provider) (*Result, error)
}, canonical []byte, tests []fieldTest) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := conv.Render(canonical, tt.target)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			out := string(result.Content)
			for _, c := range tt.contains {
				assertContains(t, out, c)
			}
			for _, a := range tt.absent {
				assertNotContains(t, out, a)
			}
			if len(result.Warnings) < tt.minWarns {
				t.Errorf("expected at least %d warnings, got %d: %v",
					tt.minWarns, len(result.Warnings), result.Warnings)
			}
			if tt.filename != "" {
				assertEqual(t, tt.filename, result.Filename)
			}
		})
	}
}

// =============================================================================
// RULES — Scoped (alwaysApply:false + globs)
// =============================================================================

func TestFieldPreservation_RulesScoped(t *testing.T) {
	input := []byte("---\ndescription: TypeScript conventions\nalwaysApply: false\nglobs:\n    - \"*.ts\"\n    - \"*.tsx\"\n---\n\nUse strict TypeScript with no-any rule.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "cursor")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	runFieldTests(t, conv, canonical.Content, []fieldTest{
		{
			name:     "to Cursor",
			target:   provider.Cursor,
			contains: []string{"alwaysApply: false", "*.ts", "*.tsx", "description: TypeScript conventions", "Use strict TypeScript"},
			filename: "rule.mdc",
		},
		{
			name:     "to Windsurf",
			target:   provider.Windsurf,
			contains: []string{"trigger: glob", "*.ts", "description: TypeScript conventions", "Use strict TypeScript"},
			absent:   []string{"alwaysApply"},
			filename: "rule.md",
		},
		{
			name:     "to Claude",
			target:   provider.ClaudeCode,
			contains: []string{"Use strict TypeScript", "paths:", "*.ts", "*.tsx"},
			absent:   []string{"alwaysApply:", "trigger:", "globs:"},
		},
		{
			name:     "to Gemini",
			target:   provider.GeminiCLI,
			contains: []string{"Use strict TypeScript", "**Scope:**", "*.ts"},
			absent:   []string{"alwaysApply:", "trigger:"},
		},
		{
			name:     "to Codex",
			target:   provider.Codex,
			contains: []string{"Use strict TypeScript", "**Scope:**", "*.ts"},
		},
		{
			name:     "to Copilot",
			target:   provider.CopilotCLI,
			contains: []string{"Use strict TypeScript", "**Scope:**", "*.ts"},
		},
		{
			name:     "to Zed",
			target:   provider.Zed,
			contains: []string{"Use strict TypeScript"},
			minWarns: 1,
			filename: ".rules",
		},
		{
			name:     "to Cline",
			target:   provider.Cline,
			contains: []string{"paths:", "*.ts", "*.tsx", "Use strict TypeScript"},
			absent:   []string{"globs:", "alwaysApply:"},
		},
		{
			name:     "to Roo Code",
			target:   provider.RooCode,
			contains: []string{"Use strict TypeScript"},
			minWarns: 1,
		},
		{
			name:     "to OpenCode",
			target:   provider.OpenCode,
			contains: []string{"Use strict TypeScript", "**Scope:**", "*.ts"},
		},
		{
			name:     "to Kiro",
			target:   provider.Kiro,
			contains: []string{"Use strict TypeScript", "**Scope:**", "*.ts"},
		},
	})
}

// =============================================================================
// RULES — Always apply (no scope needed)
// =============================================================================

func TestFieldPreservation_RulesAlwaysApply(t *testing.T) {
	input := []byte("---\ndescription: Global conventions\nalwaysApply: true\n---\n\nFollow these conventions always.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "cursor")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	runFieldTests(t, conv, canonical.Content, []fieldTest{
		{
			name:     "to Cursor",
			target:   provider.Cursor,
			contains: []string{"alwaysApply: true", "Follow these conventions"},
			filename: "rule.mdc",
		},
		{
			name:     "to Windsurf",
			target:   provider.Windsurf,
			contains: []string{"trigger: always_on", "Follow these conventions"},
			absent:   []string{"alwaysApply"},
			filename: "rule.md",
		},
		{
			name:     "to Claude",
			target:   provider.ClaudeCode,
			contains: []string{"Follow these conventions"},
			absent:   []string{"---", "Scope:"},
		},
		{
			name:     "to Gemini",
			target:   provider.GeminiCLI,
			contains: []string{"Follow these conventions"},
			absent:   []string{"---", "Scope:"},
		},
		{
			name:     "to Zed",
			target:   provider.Zed,
			contains: []string{"Follow these conventions"},
			absent:   []string{"---"},
			filename: ".rules",
		},
		{
			name:     "to Cline",
			target:   provider.Cline,
			contains: []string{"Follow these conventions"},
		},
		{
			name:     "to Roo Code",
			target:   provider.RooCode,
			contains: []string{"Follow these conventions"},
		},
		{
			name:     "to OpenCode",
			target:   provider.OpenCode,
			contains: []string{"Follow these conventions"},
		},
		{
			name:     "to Kiro",
			target:   provider.Kiro,
			contains: []string{"Follow these conventions"},
		},
	})
}

// =============================================================================
// AGENTS — Kitchen sink with every field populated
// =============================================================================

func TestFieldPreservation_Agents(t *testing.T) {
	input := []byte("---\nname: Kitchen Sink Agent\ndescription: Agent testing all converter fields\ntools:\n  - Read\n  - Write\n  - Bash\n  - Glob\n  - Grep\ndisallowedTools:\n  - WebSearch\nmodel: opus\nmaxTurns: 25\npermissionMode: plan\nskills:\n  - review\nmcpServers:\n  - github\nmemory: project\nbackground: true\nisolation: worktree\n---\n\nYou are a comprehensive test agent.\n")

	conv := &AgentsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	runFieldTests(t, conv, canonical.Content, []fieldTest{
		{
			name:   "to Gemini",
			target: provider.GeminiCLI,
			contains: []string{
				"name: Kitchen Sink Agent",
				"read_file",                  // Read translated
				"write_file",                 // Write translated
				"run_shell_command",          // Bash translated
				"model: opus",                // preserved
				"max_turns: 25",              // preserved
				"read-only exploration mode", // permissionMode → prose
				"background task",            // background → prose
				"separate git worktree",      // isolation → prose
				"Do not use these tools",     // disallowedTools → prose
				"syllago:converted",          // conversion marker
				"comprehensive test agent",   // body preserved
			},
			absent:   []string{"permissionMode:", "isolation: worktree"},
			filename: "agent.md",
		},
		{
			name:   "to Copilot",
			target: provider.CopilotCLI,
			contains: []string{
				"name: Kitchen Sink Agent",
				"model: opus",              // model in frontmatter
				"Limit to 25 turns",        // maxTurns → prose
				"comprehensive test agent", // body preserved
			},
			filename: "kitchen-sink-agent.agent.md",
		},
		{
			name:   "to Roo Code",
			target: provider.RooCode,
			contains: []string{
				"slug: kitchen-sink-agent",
				"name: Kitchen Sink Agent",
				"roleDefinition:",
				"whenToUse: Agent testing all converter fields",
				"read",    // tool group
				"edit",    // tool group (Write → edit)
				"command", // tool group (Bash → command)
			},
			absent:   []string{"permissionMode", "maxTurns"},
			minWarns: 5, // many dropped fields
		},
		{
			name:   "to OpenCode",
			target: provider.OpenCode,
			contains: []string{
				"Kitchen Sink Agent",
				"comprehensive test agent",
			},
			absent:   []string{"permissionMode"},
			minWarns: 1, // permissionMode dropped
		},
		{
			name:   "to Kiro",
			target: provider.Kiro,
			contains: []string{
				"name: Kitchen Sink Agent",
				"model: opus",
				"- read",     // Read → read
				"- fs_write", // Write → fs_write
				"- shell",    // Bash → shell
				"comprehensive test agent", // body preserved
			},
			minWarns: 1, // maxTurns not supported
		},
	})

	// Kiro agents inline the prompt — no ExtraFiles
	t.Run("Kiro Inline Prompt", func(t *testing.T) {
		result, err := conv.Render(canonical.Content, provider.Kiro)
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		if result.ExtraFiles != nil {
			t.Errorf("expected no ExtraFiles (prompt inlined), got %d", len(result.ExtraFiles))
		}
		// Prompt body should be in the markdown content
		out := string(result.Content)
		if !containsStr(out, "comprehensive test agent") {
			t.Error("expected prompt body in markdown content")
		}
	})
}

// =============================================================================
// SKILLS — Kitchen sink with all metadata fields
// =============================================================================

func TestFieldPreservation_Skills(t *testing.T) {
	input := []byte("---\nname: code-review\ndescription: Code review skill\nallowed-tools:\n  - Read\n  - Grep\ndisallowed-tools:\n  - Bash\nmodel: opus\neffort: high\ncontext: fork\nuser-invocable: true\nargument-hint: \"<pr-url>\"\nhooks:\n  pre_tool_use:\n    - command: \"echo check\"\n---\n\nReview code for best practices and security.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	runFieldTests(t, conv, canonical.Content, []fieldTest{
		{
			name:   "to Claude",
			target: provider.ClaudeCode,
			contains: []string{
				"name: code-review",
				"allowed-tools:",
				"Read",
				"Grep",
				"model: opus",
				"effort: high",
				"context: fork",
				"hooks:",
				"echo check",
				"Review code for best practices",
			},
			filename: "SKILL.md",
		},
		{
			name:   "to Gemini",
			target: provider.GeminiCLI,
			contains: []string{
				"name: code-review",
				"description: Code review skill",
				"read_file",          // allowed-tools translated
				"grep_search",        // allowed-tools translated
				"model: opus",        // embedded as prose
				"Effort level: high", // effort embedded as prose
				"isolated context",   // context: fork → prose
				"command menu",       // user-invocable → prose
				"<pr-url>",           // argument-hint → prose
				"run_shell_command",  // disallowed-tools translated
				"Hooks:",             // hooks embedded as prose
				"Review code for best practices",
				"syllago:converted",
			},
			absent:   []string{"allowed-tools:", "context: fork", "user-invocable:", "echo check"},
			filename: "SKILL.md",
		},
		{
			name:   "to OpenCode",
			target: provider.OpenCode,
			contains: []string{
				"# code-review",
				"Review code for best practices",
				"Tool restriction",   // allowed-tools embedded as prose
				"isolated context",   // context: fork embedded as prose
				"model: opus",        // model embedded as prose
				"Effort level: high", // effort embedded as prose
				"command menu",       // user-invocable embedded as prose
				"Hooks:",             // hooks embedded as prose
			},
			absent:   []string{"allowed-tools:", "echo check"},
			filename: "code-review.md",
		},
		{
			name:   "to Kiro",
			target: provider.Kiro,
			contains: []string{
				"# code-review",
				"Review code for best practices",
				"Tool restriction", // allowed-tools embedded as prose
				"Do not use",       // disallowed-tools embedded as prose
				"command menu",     // user-invocable embedded as prose
				"Hooks:",           // hooks embedded as prose
			},
			absent:   []string{"allowed-tools:", "echo check"},
			filename: "code-review.md",
		},
	})
}

// =============================================================================
// COMMANDS — Kitchen sink with behavioral fields
// =============================================================================

func TestFieldPreservation_Commands(t *testing.T) {
	input := []byte("---\nname: deploy\ndescription: Deploy to production\nallowed-tools:\n  - Read\n  - Bash\nagent: Explore\nmodel: opus\ncontext: fork\n---\n\nDeploy $ARGUMENTS to production environment.\n")

	conv := &CommandsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	runFieldTests(t, conv, canonical.Content, []fieldTest{
		{
			name:   "to Claude",
			target: provider.ClaudeCode,
			contains: []string{
				"name: deploy",
				"description: Deploy to production",
				"Deploy",
				"$ARGUMENTS",
			},
			filename: "command.md",
		},
		{
			name:   "to Gemini",
			target: provider.GeminiCLI,
			contains: []string{
				"Deploy to production",
				"read_file",         // allowed-tools translated
				"run_shell_command", // Bash translated
				"explore-focused",   // agent: Explore → prose
				"model: opus",       // embedded as prose
				"isolated context",  // context: fork → prose
				"{{args}}",          // $ARGUMENTS → {{args}}
				"syllago:converted",
			},
			absent:   []string{"$ARGUMENTS", "allowed-tools:", "context: fork"},
			filename: "command.toml",
		},
		{
			name:   "to OpenCode",
			target: provider.OpenCode,
			contains: []string{
				"Deploy",
				"production",
			},
		},
	})
}

// =============================================================================
// HOOKS — Kitchen sink with matcher, timeout, multiple events
// =============================================================================

func TestFieldPreservation_Hooks(t *testing.T) {
	input := []byte(`{
		"hooks": {
			"PreToolUse": [
				{
					"matcher": "Bash",
					"hooks": [
						{"type": "command", "command": "echo safety-check", "timeout": 5000, "statusMessage": "Checking safety..."}
					]
				}
			],
			"SessionStart": [
				{
					"hooks": [
						{"type": "command", "command": "echo session-init"}
					]
				}
			]
		}
	}`)

	conv := &HooksConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// Claude Code — identity (everything preserved)
	t.Run("to Claude", func(t *testing.T) {
		result, err := conv.Render(canonical.Content, provider.ClaudeCode)
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		out := string(result.Content)
		assertContains(t, out, "PreToolUse")
		assertContains(t, out, "SessionStart")
		assertContains(t, out, `"Bash"`)
		assertContains(t, out, "echo safety-check")
		assertContains(t, out, "5000")
		assertContains(t, out, "Checking safety...")
		assertContains(t, out, "echo session-init")
	})

	// Gemini CLI — events translated, matchers translated
	t.Run("to Gemini", func(t *testing.T) {
		result, err := conv.Render(canonical.Content, provider.GeminiCLI)
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		out := string(result.Content)
		assertContains(t, out, "BeforeTool")        // PreToolUse → BeforeTool
		assertContains(t, out, "run_shell_command") // Bash → run_shell_command
		assertContains(t, out, "echo safety-check") // command preserved
		assertNotContains(t, out, "PreToolUse")     // not leaked
		assertNotContains(t, out, `"Bash"`)         // not leaked
	})

	// Copilot CLI — events translated, matchers preserved, version field present
	t.Run("to Copilot", func(t *testing.T) {
		result, err := conv.Render(canonical.Content, provider.CopilotCLI)
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		out := string(result.Content)
		assertContains(t, out, "echo safety-check")
		assertNotContains(t, out, "PreToolUse")
		// Matchers should be preserved (translated to Copilot tool name)
		assertContains(t, out, "\"matcher\": \"bash\"") // Bash → bash
		// Version field present
		assertContains(t, out, "\"version\": 1")
		// Type field present on entries
		assertContains(t, out, "\"type\": \"command\"")
	})

	// Kiro — wrapped in agent file, events translated
	t.Run("to Kiro", func(t *testing.T) {
		result, err := conv.Render(canonical.Content, provider.Kiro)
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		out := string(result.Content)
		assertContains(t, out, `"name": "syllago-hooks"`)
		assertContains(t, out, "echo safety-check")
		assertContains(t, out, "echo session-init")
		// Events translated to Kiro format
		assertNotContains(t, out, "PreToolUse")
		assertNotContains(t, out, "SessionStart")
		assertEqual(t, "syllago-hooks.json", result.Filename)
	})
}

// =============================================================================
// MCP — Mixed stdio + HTTP servers
// =============================================================================

func TestFieldPreservation_MCPMixed(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"local-tool": {
				"command": "npx",
				"args": ["-y", "my-mcp-server"],
				"env": {"API_KEY": "secret"},
				"cwd": "/workspace",
				"autoApprove": ["read_file"]
			},
			"remote-api": {
				"url": "https://api.example.com/mcp",
				"type": "sse",
				"headers": {"Authorization": "Bearer token"}
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// Claude Code — full preservation
	t.Run("to Claude", func(t *testing.T) {
		result, err := conv.Render(canonical.Content, provider.ClaudeCode)
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		out := string(result.Content)
		assertContains(t, out, "local-tool")
		assertContains(t, out, "remote-api")
		assertContains(t, out, "npx")
		assertContains(t, out, "API_KEY")
		assertContains(t, out, "/workspace")
		assertContains(t, out, "autoApprove")
		assertContains(t, out, "api.example.com")
		assertContains(t, out, "sse")
	})

	// Gemini — drops autoApprove, preserves rest
	t.Run("to Gemini", func(t *testing.T) {
		result, err := conv.Render(canonical.Content, provider.GeminiCLI)
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		out := string(result.Content)
		assertContains(t, out, "npx")
		assertContains(t, out, "API_KEY")
		assertNotContains(t, out, "autoApprove")
		if len(result.Warnings) == 0 {
			t.Error("expected warning about dropped autoApprove")
		}
	})

	// Zed — stdio only, HTTP servers dropped with warning
	t.Run("to Zed", func(t *testing.T) {
		result, err := conv.Render(canonical.Content, provider.Zed)
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		out := string(result.Content)
		assertContains(t, out, "context_servers")    // Zed key name
		assertContains(t, out, "npx")                // stdio server preserved
		assertNotContains(t, out, "api.example.com") // HTTP server dropped
		if len(result.Warnings) == 0 {
			t.Error("expected warning about dropped HTTP server")
		}
	})

	// Cline — stdio only, autoApprove → alwaysAllow, HTTP dropped
	t.Run("to Cline", func(t *testing.T) {
		result, err := conv.Render(canonical.Content, provider.Cline)
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		out := string(result.Content)
		assertContains(t, out, "npx")
		assertContains(t, out, "alwaysAllow") // autoApprove → alwaysAllow
		assertNotContains(t, out, "autoApprove")
		assertNotContains(t, out, "api.example.com") // HTTP dropped
		if len(result.Warnings) == 0 {
			t.Error("expected warning about dropped HTTP server")
		}
	})

	// Roo Code — HTTP preserved, autoApprove dropped, cwd dropped
	t.Run("to Roo Code", func(t *testing.T) {
		result, err := conv.Render(canonical.Content, provider.RooCode)
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		out := string(result.Content)
		assertContains(t, out, "npx")
		assertContains(t, out, "api.example.com") // HTTP preserved
		assertNotContains(t, out, "autoApprove")
		assertNotContains(t, out, "cwd")
		if len(result.Warnings) == 0 {
			t.Error("expected warnings about dropped fields")
		}
	})

	// OpenCode — command array, environment key, type: local/remote
	t.Run("to OpenCode", func(t *testing.T) {
		result, err := conv.Render(canonical.Content, provider.OpenCode)
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		out := string(result.Content)
		assertContains(t, out, `"mcp"`) // OpenCode key
		assertNotContains(t, out, `"mcpServers"`)
		assertContains(t, out, `"command": [`)  // array format
		assertContains(t, out, `"environment"`) // env → environment
		assertNotContains(t, out, `"env"`)
		assertContains(t, out, "api.example.com")  // HTTP preserved
		assertContains(t, out, `"type": "remote"`) // sse → remote
	})

	// Copilot — preserves both stdio and HTTP, drops autoApprove
	t.Run("to Copilot", func(t *testing.T) {
		result, err := conv.Render(canonical.Content, provider.CopilotCLI)
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		out := string(result.Content)
		assertContains(t, out, "npx")
		assertContains(t, out, "api.example.com") // HTTP preserved
		assertNotContains(t, out, "autoApprove")
	})

	// Kiro — preserves autoApprove, drops Gemini-specific fields
	t.Run("to Kiro", func(t *testing.T) {
		result, err := conv.Render(canonical.Content, provider.Kiro)
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		out := string(result.Content)
		assertContains(t, out, "npx")
		assertContains(t, out, "autoApprove")     // preserved!
		assertContains(t, out, "api.example.com") // HTTP preserved
	})
}

// =============================================================================
// CANONICALIZE — Provider-specific formats to canonical
// =============================================================================

func TestCanonicalize_WindsurfTriggerFormats(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectAlways bool
		expectGlobs  bool
		expectDesc   string
	}{
		{
			name:         "always_on",
			input:        "---\ntrigger: always_on\ndescription: Global\n---\n\nContent.\n",
			expectAlways: true,
			expectDesc:   "Global",
		},
		{
			name:         "glob",
			input:        "---\ntrigger: glob\nglobs: \"*.ts, *.tsx\"\ndescription: TS\n---\n\nTS content.\n",
			expectAlways: false,
			expectGlobs:  true,
			expectDesc:   "TS",
		},
		{
			name:         "model_decision",
			input:        "---\ntrigger: model_decision\ndescription: Refactoring\n---\n\nRefactor.\n",
			expectAlways: false,
			expectGlobs:  false,
			expectDesc:   "Refactoring",
		},
		{
			name:         "manual",
			input:        "---\ntrigger: manual\n---\n\nManual.\n",
			expectAlways: false,
			expectGlobs:  false,
		},
	}

	conv := &RulesConverter{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := conv.Canonicalize([]byte(tt.input), "windsurf")
			if err != nil {
				t.Fatalf("Canonicalize: %v", err)
			}
			meta, _, err := parseCanonical(result.Content)
			if err != nil {
				t.Fatalf("parseCanonical: %v", err)
			}
			if meta.AlwaysApply != tt.expectAlways {
				t.Errorf("alwaysApply: expected %v, got %v", tt.expectAlways, meta.AlwaysApply)
			}
			if tt.expectGlobs && len(meta.Globs) == 0 {
				t.Error("expected globs to be populated")
			}
			if tt.expectDesc != "" && meta.Description != tt.expectDesc {
				t.Errorf("description: expected %q, got %q", tt.expectDesc, meta.Description)
			}
		})
	}
}

func TestCanonicalize_ClinePaths(t *testing.T) {
	input := []byte("---\npaths:\n    - \"*.go\"\n    - \"*.mod\"\n    - \"internal/**\"\n---\n\nGo conventions.\n")

	conv := &RulesConverter{}
	result, err := conv.Canonicalize(input, "cline")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "globs:")
	assertNotContains(t, out, "paths:")
	assertContains(t, out, "*.go")
	assertContains(t, out, "internal/**")
}

func TestCanonicalize_OpenCodeMCPFormat(t *testing.T) {
	input := []byte(`{
		"mcp": {
			"db": {
				"type": "local",
				"command": ["node", "db-server.js"],
				"environment": {"DB_URL": "postgres://localhost"},
				"enabled": false,
				"timeout": 5000
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Canonicalize(input, "opencode")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "mcpServers")
	assertContains(t, out, `"command": "node"`) // array → command + args
	assertContains(t, out, `"args"`)
	assertContains(t, out, `"env"`) // environment → env
	assertNotContains(t, out, `"environment"`)
	assertContains(t, out, "DB_URL")
	assertContains(t, out, `"disabled": true`) // enabled:false → disabled:true
}

func TestCanonicalize_ZedContextServers(t *testing.T) {
	input := []byte(`{
		"context_servers": {
			"mytool": {
				"source": "custom",
				"command": "npx",
				"args": ["-y", "mcp-server"]
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Canonicalize(input, "zed")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "mcpServers")
	assertNotContains(t, out, "context_servers")
	assertNotContains(t, out, "source") // Zed-specific field stripped
	assertContains(t, out, "npx")
}

func TestCanonicalize_ClineAlwaysAllow(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"gh": {
				"command": "gh-mcp",
				"alwaysAllow": ["search", "read"]
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Canonicalize(input, "cline")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "autoApprove")
	assertNotContains(t, out, "alwaysAllow")
	assertContains(t, out, "search")
	assertContains(t, out, "read")
}

func TestCanonicalize_GeminiHttpUrl(t *testing.T) {
	input := []byte(`{
		"mcpServers": {
			"api": {
				"httpUrl": "https://api.example.com/sse",
				"trust": "high",
				"includeTools": ["search"]
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Canonicalize(input, "gemini-cli")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	var cfg mcpConfig
	json.Unmarshal(result.Content, &cfg)

	server := cfg.MCPServers["api"]
	assertEqual(t, "https://api.example.com/sse", server.URL)
	assertEqual(t, "", server.HTTPUrl) // normalized away
	assertEqual(t, "sse", server.Type) // inferred from httpUrl
}

func TestCanonicalize_GeminiHookEvents(t *testing.T) {
	input := []byte(`{
		"hooks": {
			"BeforeTool": [
				{
					"matcher": "run_shell_command",
					"hooks": [
						{"type": "command", "command": "echo check"}
					]
				}
			],
			"AfterTool": [
				{
					"hooks": [
						{"type": "command", "command": "echo done"}
					]
				}
			]
		}
	}`)

	conv := &HooksConverter{}
	result, err := conv.Canonicalize(input, "gemini-cli")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "PreToolUse")  // BeforeTool → PreToolUse
	assertContains(t, out, "PostToolUse") // AfterTool → PostToolUse
	assertContains(t, out, `"Bash"`)      // run_shell_command → Bash
	assertNotContains(t, out, "BeforeTool")
	assertNotContains(t, out, "AfterTool")
	assertNotContains(t, out, "run_shell_command")
}

func TestCanonicalize_CopilotHookFormat(t *testing.T) {
	// Copilot hooks use matcher groups: {"version":1, "hooks":{"event":[{"matcher":"...","hooks":[...]}]}}
	input := []byte(`{
		"version": 1,
		"hooks": {
			"preToolUse": [
				{
					"hooks": [
						{
							"type": "command",
							"bash": "echo verify",
							"timeoutSec": 10,
							"comment": "Safety verification"
						}
					]
				}
			]
		}
	}`)

	conv := &HooksConverter{}
	result, err := conv.Canonicalize(input, "copilot-cli")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "PreToolUse") // preToolUse → PreToolUse
	assertContains(t, out, "echo verify")
	assertContains(t, out, `"timeout": 10`) // 10 sec stays 10 sec (canonical unit is seconds)
	assertContains(t, out, "Safety verification")
}

// =============================================================================
// ROUND-TRIP — Verify lossless conversion cycles
// =============================================================================

func TestRoundTrip_RulesCursorClaude(t *testing.T) {
	// alwaysApply:true rule should survive Cursor → Claude → Cursor
	original := []byte("---\ndescription: Go conventions\nalwaysApply: true\n---\n\nUse gofmt.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(original, "cursor")
	if err != nil {
		t.Fatalf("Canonicalize from Cursor: %v", err)
	}

	// Claude Code for alwaysApply:true strips frontmatter → body only
	claudeResult, err := conv.Render(canonical.Content, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render to Claude: %v", err)
	}

	// Back from Claude → canonical
	backToCanonical, err := conv.Canonicalize(claudeResult.Content, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize from Claude: %v", err)
	}

	meta, body, err := parseCanonical(backToCanonical.Content)
	if err != nil {
		t.Fatalf("parseCanonical: %v", err)
	}

	// Body must survive
	assertContains(t, body, "Use gofmt.")
	// alwaysApply defaults to true for plain markdown
	if !meta.AlwaysApply {
		t.Error("expected alwaysApply:true after round-trip through Claude")
	}
}

func TestRoundTrip_MCPClaudeKiro(t *testing.T) {
	// Stdio server with autoApprove should survive Claude → Kiro → Claude
	// because Kiro preserves autoApprove
	original := []byte(`{
		"mcpServers": {
			"tool": {
				"command": "mytool",
				"args": ["--verbose"],
				"autoApprove": ["search"]
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(original, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	kiroResult, err := conv.Render(canonical.Content, provider.Kiro)
	if err != nil {
		t.Fatalf("Render to Kiro: %v", err)
	}

	// Back from Kiro
	backToCanonical, err := conv.Canonicalize(kiroResult.Content, "kiro")
	if err != nil {
		t.Fatalf("Canonicalize from Kiro: %v", err)
	}

	var cfg mcpConfig
	json.Unmarshal(backToCanonical.Content, &cfg)

	server := cfg.MCPServers["tool"]
	assertEqual(t, "mytool", server.Command)
	if len(server.AutoApprove) == 0 || server.AutoApprove[0] != "search" {
		t.Error("expected autoApprove to survive Kiro round-trip")
	}
}

func TestRoundTrip_HooksClaudeGemini(t *testing.T) {
	// Command hook (not LLM) should survive Claude → Gemini → Claude
	original := []byte(`{
		"hooks": {
			"PreToolUse": [
				{
					"matcher": "Bash",
					"hooks": [
						{"type": "command", "command": "echo check", "timeout": 3000}
					]
				}
			]
		}
	}`)

	conv := &HooksConverter{}
	canonical, err := conv.Canonicalize(original, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	geminiResult, err := conv.Render(canonical.Content, provider.GeminiCLI)
	if err != nil {
		t.Fatalf("Render to Gemini: %v", err)
	}

	backToCanonical, err := conv.Canonicalize(geminiResult.Content, "gemini-cli")
	if err != nil {
		t.Fatalf("Canonicalize from Gemini: %v", err)
	}

	out := string(backToCanonical.Content)
	assertContains(t, out, "PreToolUse") // event translated back
	assertContains(t, out, `"Bash"`)     // matcher translated back
	assertContains(t, out, "echo check") // command preserved
	assertContains(t, out, `"timeout": 3`) // timeout preserved (canonical seconds: 3000ms → 3s)
}

// =============================================================================
// CROSS-PROVIDER CHAINS — Gemini → canonical → target (not just Claude→X)
// =============================================================================

func TestCrossProvider_GeminiAgentToKiro(t *testing.T) {
	// Gemini agent → canonical → Kiro: tests double translation
	input := []byte("---\nname: researcher\ndescription: Research agent\ntools:\n  - read_file\n  - grep_search\ntemperature: 0.7\ntimeout_mins: 10\n---\n\nResearch and summarize findings.\n")

	conv := &AgentsConverter{}
	canonical, err := conv.Canonicalize(input, "gemini-cli")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Kiro)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "name: researcher")
	// Gemini read_file → canonical Read → Kiro read
	assertContains(t, out, "- read")
	// Prompt body in markdown after frontmatter
	assertContains(t, out, "Research and summarize")
}

func TestCrossProvider_GeminiAgentToRooCode(t *testing.T) {
	input := []byte("---\nname: coder\ndescription: Coding agent\ntools:\n  - read_file\n  - write_file\n  - run_shell_command\ntemperature: 0.5\n---\n\nWrite quality code.\n")

	conv := &AgentsConverter{}
	canonical, err := conv.Canonicalize(input, "gemini-cli")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.RooCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "slug: coder")
	assertContains(t, out, "name: coder")
	assertContains(t, out, "read")    // tool group
	assertContains(t, out, "edit")    // Write → edit group
	assertContains(t, out, "command") // Bash → command group
	// Temperature not supported by Roo Code
	assertNotContains(t, out, "temperature")
}

func TestCrossProvider_GeminiRulesToCline(t *testing.T) {
	// Gemini has no native scoping → comes through as alwaysApply:true
	input := []byte("Follow Go conventions.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "gemini-cli")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Cline)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "Follow Go conventions.")
}

func TestCrossProvider_CursorRulesToKiro(t *testing.T) {
	input := []byte("---\ndescription: Python rule\nalwaysApply: false\nglobs:\n  - \"*.py\"\n---\n\nUse type hints.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "cursor")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Kiro)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "Use type hints.")
	assertContains(t, out, "**Scope:**") // globs embedded as prose
	assertContains(t, out, "*.py")
}

func TestCrossProvider_CursorRulesToOpenCode(t *testing.T) {
	input := []byte("---\ndescription: JS conventions\nalwaysApply: false\nglobs:\n  - \"*.js\"\n---\n\nUse const over let.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "cursor")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.OpenCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "Use const over let.")
	assertContains(t, out, "*.js")
}

func TestCrossProvider_WindsurfRulesToRooCode(t *testing.T) {
	input := []byte("---\ntrigger: glob\nglobs: \"*.rs\"\ndescription: Rust rules\n---\n\nUse clippy lints.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "windsurf")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.RooCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "Use clippy lints.")
	// Roo Code drops globs with warning
	if len(result.Warnings) == 0 {
		t.Error("expected warning about dropped globs for Roo Code")
	}
}

// =============================================================================
// EDGE CASES — Empty fields, minimal inputs, all-HTTP MCP
// =============================================================================

func TestEdgeCase_AgentNoTools(t *testing.T) {
	// Agent with no tools specified should still render
	input := []byte("---\nname: minimal\ndescription: Minimal agent\n---\n\nJust instructions.\n")

	conv := &AgentsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	targets := []struct {
		name   string
		target provider.Provider
	}{
		{"Gemini", provider.GeminiCLI},
		{"Copilot", provider.CopilotCLI},
		{"RooCode", provider.RooCode},
		{"OpenCode", provider.OpenCode},
		{"Kiro", provider.Kiro},
	}

	for _, tt := range targets {
		t.Run(tt.name, func(t *testing.T) {
			result, err := conv.Render(canonical.Content, tt.target)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			out := string(result.Content)
			assertContains(t, out, "minimal")
		})
	}
}

func TestEdgeCase_MCPOnlyHTTPServers(t *testing.T) {
	// Config with ONLY HTTP servers — stdio-only providers should produce
	// empty/minimal output with warnings
	input := []byte(`{
		"mcpServers": {
			"api1": {"url": "https://a.example.com", "type": "sse"},
			"api2": {"url": "https://b.example.com", "type": "sse"}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// Zed and Cline should drop everything and warn
	for _, tt := range []struct {
		name   string
		target provider.Provider
	}{
		{"Zed", provider.Zed},
		{"Cline", provider.Cline},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := conv.Render(canonical.Content, tt.target)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			out := string(result.Content)
			assertNotContains(t, out, "a.example.com")
			assertNotContains(t, out, "b.example.com")
			if len(result.Warnings) < 2 {
				t.Errorf("expected at least 2 warnings (one per HTTP server), got %d", len(result.Warnings))
			}
		})
	}

	// OpenCode and Kiro should preserve both
	for _, tt := range []struct {
		name   string
		target provider.Provider
	}{
		{"OpenCode", provider.OpenCode},
		{"Kiro", provider.Kiro},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := conv.Render(canonical.Content, tt.target)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			out := string(result.Content)
			assertContains(t, out, "a.example.com")
			assertContains(t, out, "b.example.com")
		})
	}
}

func TestEdgeCase_RuleEmptyBody(t *testing.T) {
	// Rule with only frontmatter and no body
	input := []byte("---\ndescription: Empty body rule\nalwaysApply: true\n---\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(input, "cursor")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// Should still render without error
	for _, tt := range []struct {
		name   string
		target provider.Provider
	}{
		{"Claude", provider.ClaudeCode},
		{"Gemini", provider.GeminiCLI},
		{"Zed", provider.Zed},
		{"Kiro", provider.Kiro},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := conv.Render(canonical.Content, tt.target)
			if err != nil {
				t.Fatalf("Render should not error on empty body: %v", err)
			}
		})
	}
}

func TestEdgeCase_HookUnsupportedEvent(t *testing.T) {
	// Claude-only events should produce warnings when targeting other providers
	input := []byte(`{
		"hooks": {
			"SubagentStart": [
				{"hooks": [{"type": "command", "command": "echo sub"}]}
			],
			"SubagentCompleted": [
				{"hooks": [{"type": "command", "command": "echo done"}]}
			]
		}
	}`)

	conv := &HooksConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	for _, tt := range []struct {
		name   string
		target provider.Provider
	}{
		{"Gemini", provider.GeminiCLI},
		{"Copilot", provider.CopilotCLI},
		{"Kiro", provider.Kiro},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := conv.Render(canonical.Content, tt.target)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if len(result.Warnings) == 0 {
				t.Error("expected warning about unsupported event")
			}
		})
	}
}

func TestEdgeCase_TimeoutUnitPrecision(t *testing.T) {
	// Test that ms → sec → ms round-trip preserves values
	// Claude uses ms, Copilot uses sec
	input := []byte(`{
		"hooks": {
			"PreToolUse": [
				{
					"hooks": [
						{"type": "command", "command": "echo test", "timeout": 7500}
					]
				}
			]
		}
	}`)

	conv := &HooksConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// Claude → Copilot
	copilotResult, err := conv.Render(canonical.Content, provider.CopilotCLI)
	if err != nil {
		t.Fatalf("Render to Copilot: %v", err)
	}
	out := string(copilotResult.Content)
	// 7500ms → 7 or 8 seconds (truncation behavior)
	assertContains(t, out, "echo test")

	// Copilot → canonical (back)
	backToCanonical, err := conv.Canonicalize(copilotResult.Content, "copilot-cli")
	if err != nil {
		t.Fatalf("Canonicalize from Copilot: %v", err)
	}

	backOut := string(backToCanonical.Content)
	assertContains(t, backOut, "echo test")
	assertContains(t, backOut, "PreToolUse")
}

func TestEdgeCase_SkillMinimalToAllProviders(t *testing.T) {
	// Skill with only body, no frontmatter — tests default handling
	input := []byte("Help with coding tasks.\n")

	conv := &SkillsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	targets := []struct {
		name   string
		target provider.Provider
	}{
		{"Gemini", provider.GeminiCLI},
		{"OpenCode", provider.OpenCode},
		{"Kiro", provider.Kiro},
	}

	for _, tt := range targets {
		t.Run(tt.name, func(t *testing.T) {
			result, err := conv.Render(canonical.Content, tt.target)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			assertContains(t, string(result.Content), "coding tasks")
		})
	}
}

func TestEdgeCase_CommandMinimalToAllProviders(t *testing.T) {
	// Command with no frontmatter
	input := []byte("Run the tests.\n")

	conv := &CommandsConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	targets := []struct {
		name     string
		target   provider.Provider
		filename string
	}{
		{"Gemini", provider.GeminiCLI, "command.toml"},
		{"Codex", provider.Codex, "command.md"},
		{"OpenCode", provider.OpenCode, ""},
	}

	for _, tt := range targets {
		t.Run(tt.name, func(t *testing.T) {
			result, err := conv.Render(canonical.Content, tt.target)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			assertContains(t, string(result.Content), "Run the tests.")
			if tt.filename != "" {
				assertEqual(t, tt.filename, result.Filename)
			}
		})
	}
}

// =============================================================================
// TRICKY PATTERN: Filename slugification across providers
// =============================================================================

func TestFilenameSlugification(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		target provider.Provider
		conv   interface {
			Render([]byte, provider.Provider) (*Result, error)
		}
		expected string
	}{
		{
			name:     "RooCode agent slug",
			input:    "---\nname: My Cool Agent\ndescription: Does things\n---\n\nInstructions.\n",
			target:   provider.RooCode,
			conv:     &AgentsConverter{},
			expected: "my-cool-agent.yaml",
		},
		{
			name:     "Kiro agent slug",
			input:    "---\nname: AWS Expert\ndescription: AWS specialist\n---\n\nAWS stuff.\n",
			target:   provider.Kiro,
			conv:     &AgentsConverter{},
			expected: "aws-expert.md",
		},
		{
			name:     "OpenCode agent slug",
			input:    "---\nname: Refactor Bot\ndescription: Refactoring\n---\n\nRefactor.\n",
			target:   provider.OpenCode,
			conv:     &AgentsConverter{},
			expected: "refactor-bot.md",
		},
		{
			name:     "RooCode rule from description",
			input:    "---\ndescription: Go Best Practices\nalwaysApply: true\n---\n\nGofmt.\n",
			target:   provider.RooCode,
			conv:     &RulesConverter{},
			expected: "go-best-practices.md",
		},
		{
			name:     "Cline rule from description",
			input:    "---\ndescription: TypeScript Guidelines\nalwaysApply: true\n---\n\nStrict mode.\n",
			target:   provider.Cline,
			conv:     &RulesConverter{},
			expected: "typescript-guidelines.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Canonicalize from claude-code provider format
			type canonicalizer interface {
				Canonicalize([]byte, string) (*Result, error)
			}
			c := tt.conv.(canonicalizer)
			canonical, err := c.Canonicalize([]byte(tt.input), "claude-code")
			if err != nil {
				t.Fatalf("Canonicalize: %v", err)
			}
			result, err := tt.conv.Render(canonical.Content, tt.target)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			assertEqual(t, tt.expected, result.Filename)
		})
	}
}

// =============================================================================
// TRICKY PATTERN: LLM hooks generate mode
// =============================================================================

func TestLLMHookGenerateMode_AllProviders(t *testing.T) {
	input := []byte(`{
		"hooks": {
			"PreToolUse": [
				{
					"hooks": [
						{"type": "prompt", "command": "Is this command safe?"}
					]
				}
			]
		}
	}`)

	conv := &HooksConverter{LLMHooksMode: LLMHooksModeGenerate}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// All providers that support hooks should convert LLM hooks to wrapper scripts
	for _, tt := range []struct {
		name   string
		target provider.Provider
	}{
		{"Gemini", provider.GeminiCLI},
		{"Copilot", provider.CopilotCLI},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := conv.Render(canonical.Content, tt.target)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}

			// Should produce ExtraFiles with wrapper script
			if result.ExtraFiles == nil || len(result.ExtraFiles) == 0 {
				t.Fatal("expected ExtraFiles with wrapper script")
			}

			// Verify wrapper script contents
			for name, content := range result.ExtraFiles {
				assertContains(t, name, "syllago-llm-hook")
				script := string(content)
				assertContains(t, script, "#!/bin/bash")
				assertContains(t, script, "Is this command safe")
			}

			// Should have conversion warning (not drop warning)
			hasConvertedWarning := false
			for _, w := range result.Warnings {
				if containsStr(w, "converted to wrapper script") {
					hasConvertedWarning = true
					break
				}
			}
			if !hasConvertedWarning {
				t.Errorf("expected 'converted to wrapper script' warning, got: %v", result.Warnings)
			}
		})
	}
}

func TestLLMHookSkipMode_WarnsAboutGenerate(t *testing.T) {
	input := []byte(`{
		"hooks": {
			"PreToolUse": [
				{
					"hooks": [
						{"type": "prompt", "command": "Check safety"}
					]
				}
			]
		}
	}`)

	// Default (empty) LLMHooksMode = skip
	conv := &HooksConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	for _, tt := range []struct {
		name   string
		target provider.Provider
	}{
		{"Gemini", provider.GeminiCLI},
		{"Copilot", provider.CopilotCLI},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := conv.Render(canonical.Content, tt.target)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}

			// Hook should be dropped
			// Warning should mention --llm-hooks=generate
			hasHint := false
			for _, w := range result.Warnings {
				if containsStr(w, "--llm-hooks=generate") {
					hasHint = true
					break
				}
			}
			if !hasHint {
				t.Errorf("expected warning mentioning --llm-hooks=generate, got: %v", result.Warnings)
			}
		})
	}
}

// =============================================================================
// TRICKY PATTERN: Gemini-specific fields survive canonicalization
// =============================================================================

func TestGeminiFieldsPreserved(t *testing.T) {
	// Gemini-specific agent fields (temperature, timeoutMins) should
	// survive canonicalization and re-render to Gemini
	input := []byte("---\nname: warm-agent\ndescription: Creative agent\ntemperature: 1.5\ntimeout_mins: 15\n---\n\nBe creative.\n")

	conv := &AgentsConverter{}
	canonical, err := conv.Canonicalize(input, "gemini-cli")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// Back to Gemini — should preserve
	result, err := conv.Render(canonical.Content, provider.GeminiCLI)
	if err != nil {
		t.Fatalf("Render to Gemini: %v", err)
	}
	out := string(result.Content)
	assertContains(t, out, "temperature: 1.5")
	assertContains(t, out, "timeout_mins: 15")

	// To Claude — should embed as prose
	claudeResult, err := conv.Render(canonical.Content, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render to Claude: %v", err)
	}
	claudeOut := string(claudeResult.Content)
	assertContains(t, claudeOut, "temperature: 1.5")
	assertContains(t, claudeOut, "Limit execution to 15 minutes")
}

// =============================================================================
// TRICKY PATTERN: OpenCode JSONC handling
// =============================================================================

func TestOpenCodeJSONCComments(t *testing.T) {
	// JSONC with mixed comment styles should canonicalize cleanly
	input := []byte(`{
		// Server configuration
		"mcp": {
			/* Primary database connection */
			"postgres": {
				"type": "local",
				"command": ["pg-mcp"], // runs locally
				"environment": {
					"PG_HOST": "localhost" /* default host */
				}
			}
		}
	}`)

	conv := &MCPConverter{}
	result, err := conv.Canonicalize(input, "opencode")
	if err != nil {
		t.Fatalf("Canonicalize with JSONC: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "mcpServers")
	assertContains(t, out, "postgres")
	assertContains(t, out, "PG_HOST")
	// Comments should be stripped
	assertNotContains(t, out, "Server configuration")
	assertNotContains(t, out, "Primary database")
	assertNotContains(t, out, "runs locally")
	assertNotContains(t, out, "default host")
}

// =============================================================================
// TRICKY PATTERN: MCP field polarity inversions
// =============================================================================

func TestMCPFieldPolarity(t *testing.T) {
	// OpenCode uses "enabled: false" where canonical uses "disabled: true"
	// This tests the polarity flip in both directions

	t.Run("enabled_false_to_disabled_true", func(t *testing.T) {
		input := []byte(`{
			"mcp": {
				"db": {
					"type": "local",
					"command": ["db-mcp"],
					"enabled": false
				}
			}
		}`)

		conv := &MCPConverter{}
		result, err := conv.Canonicalize(input, "opencode")
		if err != nil {
			t.Fatalf("Canonicalize: %v", err)
		}
		out := string(result.Content)
		assertContains(t, out, `"disabled": true`)
	})

	t.Run("disabled_true_to_enabled_false", func(t *testing.T) {
		input := []byte(`{
			"mcpServers": {
				"db": {
					"command": "db-mcp",
					"disabled": true
				}
			}
		}`)

		conv := &MCPConverter{}
		canonical, err := conv.Canonicalize(input, "claude-code")
		if err != nil {
			t.Fatalf("Canonicalize: %v", err)
		}

		result, err := conv.Render(canonical.Content, provider.OpenCode)
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		out := string(result.Content)
		assertContains(t, out, `"enabled": false`)
		assertNotContains(t, out, `"disabled"`)
	})
}

// =============================================================================
// ADDITIONAL ROUND-TRIPS
// =============================================================================

func TestRoundTrip_MCPClaudeOpenCode(t *testing.T) {
	// Stdio server: Claude → OpenCode → Claude
	// Tests command+args ↔ command array, env ↔ environment
	original := []byte(`{
		"mcpServers": {
			"tool": {
				"command": "node",
				"args": ["server.js", "--port", "3000"],
				"env": {"API_KEY": "secret"}
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(original, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	openCodeResult, err := conv.Render(canonical.Content, provider.OpenCode)
	if err != nil {
		t.Fatalf("Render to OpenCode: %v", err)
	}

	// Verify OpenCode format
	openOut := string(openCodeResult.Content)
	assertContains(t, openOut, `"command": [`)  // array format
	assertContains(t, openOut, `"environment"`) // env → environment

	// Back to canonical
	backToCanonical, err := conv.Canonicalize(openCodeResult.Content, "opencode")
	if err != nil {
		t.Fatalf("Canonicalize from OpenCode: %v", err)
	}

	var cfg mcpConfig
	json.Unmarshal(backToCanonical.Content, &cfg)

	server := cfg.MCPServers["tool"]
	assertEqual(t, "node", server.Command)
	if len(server.Args) < 2 {
		t.Fatalf("expected args to survive round-trip, got %v", server.Args)
	}
	assertContains(t, server.Args[0], "server.js")
	if server.Env["API_KEY"] != "secret" {
		t.Errorf("expected env to survive, got %v", server.Env)
	}
}

func TestRoundTrip_MCPClineAutoApprove(t *testing.T) {
	// autoApprove ↔ alwaysAllow polarity
	original := []byte(`{
		"mcpServers": {
			"gh": {
				"command": "gh-mcp",
				"autoApprove": ["search", "read_file"]
			}
		}
	}`)

	conv := &MCPConverter{}
	canonical, err := conv.Canonicalize(original, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	clineResult, err := conv.Render(canonical.Content, provider.Cline)
	if err != nil {
		t.Fatalf("Render to Cline: %v", err)
	}

	out := string(clineResult.Content)
	assertContains(t, out, "alwaysAllow")
	assertNotContains(t, out, "autoApprove")

	// Back to canonical
	backToCanonical, err := conv.Canonicalize(clineResult.Content, "cline")
	if err != nil {
		t.Fatalf("Canonicalize from Cline: %v", err)
	}

	backOut := string(backToCanonical.Content)
	assertContains(t, backOut, "autoApprove")
	assertNotContains(t, backOut, "alwaysAllow")
	assertContains(t, backOut, "search")
	assertContains(t, backOut, "read_file")
}

func TestRoundTrip_RulesCursorWindsurfGlobs(t *testing.T) {
	// Globs should survive Cursor → Windsurf → Cursor
	original := []byte("---\ndescription: TS rule\nalwaysApply: false\nglobs:\n    - \"*.ts\"\n    - \"*.tsx\"\n---\n\nStrict mode.\n")

	conv := &RulesConverter{}
	canonical, err := conv.Canonicalize(original, "cursor")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	windsurfResult, err := conv.Render(canonical.Content, provider.Windsurf)
	if err != nil {
		t.Fatalf("Render to Windsurf: %v", err)
	}

	backToCanonical, err := conv.Canonicalize(windsurfResult.Content, "windsurf")
	if err != nil {
		t.Fatalf("Canonicalize from Windsurf: %v", err)
	}

	meta, body, err := parseCanonical(backToCanonical.Content)
	if err != nil {
		t.Fatalf("parseCanonical: %v", err)
	}

	if meta.AlwaysApply {
		t.Error("expected alwaysApply:false after round-trip")
	}
	if len(meta.Globs) < 2 {
		t.Errorf("expected 2 globs after round-trip, got %v", meta.Globs)
	}
	assertContains(t, body, "Strict mode.")
}
