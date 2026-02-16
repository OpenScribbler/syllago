package emit

import "github.com/holdenhewett/romanesco/cli/internal/model"

// Emitter renders a ContextDocument into a provider-specific format string.
// Emitters are pure functions — no filesystem access, no side effects.
type Emitter interface {
	Name() string
	Format() string // "md", "mdc", "json"
	Emit(doc model.ContextDocument) (string, error)
}

// EmitterForProvider returns the appropriate emitter for a provider slug.
// Known providers get purpose-built emitters; unknown slugs fall back to
// GenericMarkdownEmitter targeting AGENTS.md (the emerging convention for
// agent context files).
func EmitterForProvider(slug string) Emitter {
	switch slug {
	case "claude-code":
		return ClaudeEmitter{}
	case "cursor":
		return CursorEmitter{}
	case "gemini-cli":
		return GenericMarkdownEmitter{ProviderSlug: "gemini-cli", FileName: "GEMINI.md"}
	case "codex":
		return GenericMarkdownEmitter{ProviderSlug: "codex", FileName: "AGENTS.md"}
	case "windsurf":
		return GenericMarkdownEmitter{ProviderSlug: "windsurf", FileName: ".windsurfrules"}
	default:
		return GenericMarkdownEmitter{ProviderSlug: slug, FileName: "AGENTS.md"}
	}
}
