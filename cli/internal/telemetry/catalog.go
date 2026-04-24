package telemetry

// EventDef describes a single telemetry event.
type EventDef struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	FiredWhen   string        `json:"firedWhen"`
	Properties  []PropertyDef `json:"properties"`
}

// PropertyDef describes a single property sent with an event.
type PropertyDef struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"` // "string", "int", or "bool"
	Description string   `json:"description"`
	Example     any      `json:"example"`
	Commands    []string `json:"commands"` // which commands set this property; "*" means all
}

// PrivacyEntry describes a category of data that is never collected.
type PrivacyEntry struct {
	Category string `json:"category"`
	Examples string `json:"examples"`
}

// EventCatalog returns the complete list of telemetry events syllago may fire.
// This is the single source of truth for telemetry documentation.
func EventCatalog() []EventDef {
	return []EventDef{
		{
			Name:        "command_executed",
			Description: "Fired when a CLI command completes successfully",
			FiredWhen:   "PersistentPostRun (every non-telemetry command)",
			Properties: []PropertyDef{
				{
					Name:        "command",
					Type:        "string",
					Description: "Command name (cobra command path)",
					Example:     "install",
					Commands:    []string{"*"},
				},
				{
					Name:        "provider",
					Type:        "string",
					Description: "Target provider slug",
					Example:     "claude-code",
					Commands:    []string{"install", "uninstall", "loadout_apply", "sandbox_run", "sync-and-export", "capmon_validate_spec", "capmon_validate_format_doc", "capmon_validate_sources", "capmon_derive", "capmon_check", "capmon_onboard"},
				},
				{
					Name:        "content_type",
					Type:        "string",
					Description: "Content type filter or specific type",
					Example:     "rules",
					Commands:    []string{"install", "add", "convert", "create", "uninstall", "remove", "list", "share", "sync-and-export", "registry_items", "capmon_validate_spec"},
				},
				{
					Name:        "content_count",
					Type:        "int",
					Description: "Number of content items affected",
					Example:     3,
					Commands:    []string{"install", "add"},
				},
				{
					Name:        "dry_run",
					Type:        "bool",
					Description: "Whether --dry-run flag was used",
					Example:     false,
					Commands:    []string{"install", "add", "uninstall", "remove", "sync-and-export"},
				},
				{
					Name:        "from",
					Type:        "string",
					Description: "Source provider slug when adding cross-provider content",
					Example:     "cursor",
					Commands:    []string{"add"},
				},
				{
					Name:        "from_provider",
					Type:        "string",
					Description: "Source provider for conversion",
					Example:     "cursor",
					Commands:    []string{"convert"},
				},
				{
					Name:        "to_provider",
					Type:        "string",
					Description: "Target provider for conversion",
					Example:     "claude-code",
					Commands:    []string{"convert"},
				},
				{
					Name:        "source_filter",
					Type:        "string",
					Description: "Content source filter (library, shared, or registry)",
					Example:     "library",
					Commands:    []string{"list"},
				},
				{
					Name:        "item_count",
					Type:        "int",
					Description: "Number of items in the result set",
					Example:     12,
					Commands:    []string{"list", "registry_items"},
				},
				{
					Name:        "mode",
					Type:        "string",
					Description: "Operational mode (loadout 'try', add 'monolithic')",
					Example:     "try",
					Commands:    []string{"loadout_apply", "add"},
				},
				{
					Name:        "discovery_candidate_count",
					Type:        "int",
					Description: "Number of monolithic rule files considered by add (D18)",
					Example:     3,
					Commands:    []string{"add"},
				},
				{
					Name:        "selected_count",
					Type:        "int",
					Description: "Number of monolithic rule files actually imported (D18)",
					Example:     2,
					Commands:    []string{"add"},
				},
				{
					Name:        "split_method",
					Type:        "string",
					Description: "Splitter heuristic used for monolithic rule imports (D3)",
					Example:     "h2",
					Commands:    []string{"add"},
				},
				{
					Name:        "scope",
					Type:        "string",
					Description: "Scope label for the imported sources (project, global, mixed)",
					Example:     "project",
					Commands:    []string{"add"},
				},
				{
					Name:        "action_count",
					Type:        "int",
					Description: "Number of actions performed by loadout",
					Example:     5,
					Commands:    []string{"loadout_apply"},
				},
				{
					Name:        "registry_count",
					Type:        "int",
					Description: "Number of registries involved",
					Example:     2,
					Commands:    []string{"registry_sync"},
				},
				{
					Name:        "moat_tier",
					Type:        "string",
					Description: "Resolved MOAT trust tier for the item (UNSIGNED, SIGNED, DUAL-ATTESTED)",
					Example:     "SIGNED",
					Commands:    []string{"install"},
				},
				{
					Name:        "moat_gated",
					Type:        "string",
					Description: "MOAT install-gate outcome (proceed, hard-block, publisher-warn, private-prompt, tier-below-policy)",
					Example:     "proceed",
					Commands:    []string{"install"},
				},
				{
					Name:        "verification_state",
					Type:        "string",
					Description: "D16 verification state for rule-append installs (fresh, clean, modified)",
					Example:     "clean",
					Commands:    []string{"install"},
				},
				{
					Name:        "decision_action",
					Type:        "string",
					Description: "D17 decision action taken during re-install (proceed, replace, skip, drop_record, append_fresh, keep)",
					Example:     "replace",
					Commands:    []string{"install"},
				},
			},
		},
		{
			Name:        "tui_session_started",
			Description: "Fired when the TUI exits normally after a session",
			FiredWhen:   "After tea.Program.Run() completes without error (main.go root command)",
			Properties: []PropertyDef{
				{
					Name:        "success",
					Type:        "bool",
					Description: "Whether the TUI exited normally",
					Example:     true,
					Commands:    []string{"(root)"},
				},
			},
		},
	}
}

// StandardProperties returns the properties automatically included in every event.
// These are merged into all Track() calls by the telemetry package itself.
func StandardProperties() []PropertyDef {
	return []PropertyDef{
		{
			Name:        "version",
			Type:        "string",
			Description: "Syllago version",
			Example:     "0.7.0",
			Commands:    []string{"*"},
		},
		{
			Name:        "os",
			Type:        "string",
			Description: "Operating system (runtime.GOOS)",
			Example:     "linux",
			Commands:    []string{"*"},
		},
		{
			Name:        "arch",
			Type:        "string",
			Description: "CPU architecture (runtime.GOARCH)",
			Example:     "amd64",
			Commands:    []string{"*"},
		},
	}
}

// NeverCollected returns the structured privacy guarantees — categories of data
// syllago explicitly does not collect.
func NeverCollected() []PrivacyEntry {
	return []PrivacyEntry{
		{
			Category: "File contents",
			Examples: "Rule text, skill prompts, hook commands, MCP configs",
		},
		{
			Category: "File paths",
			Examples: "/home/user/.claude/rules/my-secret-rule",
		},
		{
			Category: "User identity",
			Examples: "Usernames, hostnames, IP addresses, email",
		},
		{
			Category: "Registry URLs",
			Examples: "Git clone URLs, registry names",
		},
		{
			Category: "Content names",
			Examples: "Names of rules, skills, agents, hooks, or MCP servers you manage",
		},
		{
			Category: "Interaction details",
			Examples: "Keystrokes, mouse clicks, TUI navigation paths",
		},
	}
}
