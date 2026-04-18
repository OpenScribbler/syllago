// Package contentformat declares canonical enum values for .syllago.yaml
// metadata fields and related content descriptors.
//
// These values are the source of truth for the content-format.json artifact
// emitted alongside providers.json on every release. The syllago-docs site
// fetches that artifact to generate reference tables, preventing drift
// between the CLI and docs.
//
// When adding a new enum value used in .syllago.yaml or canonical hook
// handlers, update the relevant slice here — the _gencontentformat command
// serializes these slices verbatim into the release artifact.
package contentformat

// Effort levels used by commands, agents, and skills to hint at resource cost.
var Effort = []string{"low", "medium", "high", "max"}

// PermissionMode values for agents. Mirrors Claude Code's permission model;
// other providers embed as prose or degrade.
var PermissionMode = []string{
	"default",
	"acceptEdits",
	"plan",
	"dontAsk",
	"bypassPermissions",
}

// SourceType records the upstream origin of imported content.
var SourceType = []string{"git", "filesystem", "registry", "provider"}

// SourceVisibility records visibility at import time.
var SourceVisibility = []string{"public", "private", "unknown"}

// SourceScope records whether content was added globally or per-project.
var SourceScope = []string{"global", "project"}

// ContentType enumerates the content categories syllago manages.
// Mirrors catalog.AllContentTypes() but is declared here for docs emission
// without pulling in the catalog package's scan-time dependencies.
var ContentType = []string{
	"rules",
	"skills",
	"agents",
	"mcp",
	"hooks",
	"commands",
	"loadouts",
}

// HookHandlerType enumerates the canonical hook handler types in
// docs/spec/hooks.md. Each type corresponds to a distinct set of handler
// fields on canonical HookHandler records.
var HookHandlerType = []string{"command", "http", "prompt", "agent"}
