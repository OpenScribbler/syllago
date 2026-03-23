package converter

import (
	"encoding/json"

	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// CanonicalHooks is the spec-compliant canonical representation (hooks/1.0).
// This is the enhanced format with all spec fields. The older hooksConfig/HookData
// types are used internally for backward compatibility with existing code paths.
type CanonicalHooks struct {
	Spec  string          `json:"spec"`
	Hooks []CanonicalHook `json:"hooks"`
}

// CanonicalHook is a single hook definition in the canonical format.
type CanonicalHook struct {
	Event        string            `json:"event"`
	Matcher      json.RawMessage   `json:"matcher,omitempty"`
	Handler      HookHandler       `json:"handler"`
	Blocking     bool              `json:"blocking,omitempty"`
	Degradation  map[string]string `json:"degradation,omitempty"`
	ProviderData map[string]any    `json:"provider_data,omitempty"`
}

// HookHandler is the handler definition within a canonical hook.
// Timeout is in seconds (canonical unit).
type HookHandler struct {
	Type          string            `json:"type"`
	Command       string            `json:"command,omitempty"`
	Platform      map[string]string `json:"platform,omitempty"`
	CWD           string            `json:"cwd,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	Timeout       int               `json:"timeout,omitempty"`
	StatusMessage string            `json:"statusMessage,omitempty"`
	Async         bool              `json:"async,omitempty"`

	// HTTP handler fields (type: "http")
	URL            string            `json:"url,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	AllowedEnvVars []string          `json:"allowedEnvVars,omitempty"`

	// Prompt handler fields (type: "prompt")
	Prompt string `json:"prompt,omitempty"`
	Model  string `json:"model,omitempty"`

	// Agent handler fields (type: "agent")
	Agent json.RawMessage `json:"agent,omitempty"`
}

// HookAdapter handles encoding and decoding hooks for a specific provider.
// Each provider implements this interface. The HooksConverter dispatches
// to the appropriate adapter based on provider slug.
type HookAdapter interface {
	// ProviderSlug returns the provider identifier (e.g., "claude-code").
	ProviderSlug() string

	// Decode reads provider-native hook content and returns canonical hooks.
	Decode(content []byte) (*CanonicalHooks, error)

	// Encode writes canonical hooks to provider-native format.
	Encode(hooks *CanonicalHooks) (*EncodedResult, error)

	// Capabilities returns what hook features this provider supports.
	Capabilities() ProviderCapabilities
}

// EncodedResult is the output of encoding canonical hooks to a provider format.
type EncodedResult struct {
	Content  []byte            // the encoded hook config
	Filename string            // target filename (e.g., "hooks.json")
	Scripts  map[string][]byte // extra files (generated wrapper scripts, etc.)
	Warnings []ConversionWarning
}

// ConversionWarning describes a data loss or behavioral change during conversion.
type ConversionWarning struct {
	Severity    string // "info", "warning", "error"
	Capability  string // which capability was affected (empty for general warnings)
	Description string // human-readable explanation
	Suggestion  string // what the user can do about it (empty if none)
}

// ProviderCapabilities describes what hook features a provider supports.
// Used by the conversion pipeline for degradation decisions and portability scoring.
type ProviderCapabilities struct {
	// Events lists the canonical event names this provider supports.
	Events []string

	// SupportsMatchers indicates whether the provider supports tool matchers.
	SupportsMatchers bool

	// SupportsAsync indicates whether hooks can run asynchronously.
	SupportsAsync bool

	// SupportsStatusMessage indicates whether hooks can show status text.
	SupportsStatusMessage bool

	// SupportsStructuredOutput indicates whether hook stdout is parsed as JSON.
	SupportsStructuredOutput bool

	// SupportsBlocking indicates whether hooks can block the triggering action.
	SupportsBlocking bool

	// TimeoutUnit is the native timeout unit ("seconds" or "milliseconds").
	TimeoutUnit string

	// SupportsPlatform indicates per-OS command override support.
	SupportsPlatform bool

	// SupportsCWD indicates configurable working directory support.
	SupportsCWD bool

	// SupportsEnv indicates custom environment variable support.
	SupportsEnv bool

	// SupportsLLMHooks indicates whether prompt/agent hook types are supported.
	SupportsLLMHooks bool

	// SupportsHTTPHooks indicates whether HTTP webhook hooks are supported.
	SupportsHTTPHooks bool
}

// adapterRegistry maps provider slugs to their hook adapters.
var adapterRegistry = map[string]HookAdapter{}

// RegisterAdapter adds a hook adapter to the registry.
func RegisterAdapter(a HookAdapter) {
	adapterRegistry[a.ProviderSlug()] = a
}

// AdapterFor returns the hook adapter for a provider slug, or nil if none registered.
func AdapterFor(slug string) HookAdapter {
	return adapterRegistry[slug]
}

// Adapters returns all registered hook adapters.
func Adapters() map[string]HookAdapter {
	return adapterRegistry
}

// SpecVersion is the current canonical hook specification version.
const SpecVersion = "hooks/1.0"

// --- Conversion between legacy and spec formats ---

// ToLegacyHooksConfig converts a CanonicalHooks to the legacy hooksConfig format.
// This bridges between the spec format and the existing converter code paths.
func (ch *CanonicalHooks) ToLegacyHooksConfig() hooksConfig {
	cfg := hooksConfig{Hooks: make(map[string][]hookMatcher)}

	for _, hook := range ch.Hooks {
		matcher := ""
		if hook.Matcher != nil {
			// For legacy compat, only handle string matchers
			var s string
			if json.Unmarshal(hook.Matcher, &s) == nil {
				matcher = s
			}
		}

		entry := HookEntry{
			Type:           hook.Handler.Type,
			Command:        hook.Handler.Command,
			Timeout:        hook.Handler.Timeout,
			StatusMessage:  hook.Handler.StatusMessage,
			Async:          hook.Handler.Async,
			URL:            hook.Handler.URL,
			Headers:        hook.Handler.Headers,
			AllowedEnvVars: hook.Handler.AllowedEnvVars,
			Prompt:         hook.Handler.Prompt,
			Model:          hook.Handler.Model,
			Agent:          hook.Handler.Agent,
		}

		cfg.Hooks[hook.Event] = append(cfg.Hooks[hook.Event], hookMatcher{
			Matcher: matcher,
			Hooks:   []HookEntry{entry},
		})
	}

	return cfg
}

// FromLegacyHooksConfig converts a legacy hooksConfig to the spec format.
func FromLegacyHooksConfig(cfg hooksConfig) *CanonicalHooks {
	ch := &CanonicalHooks{
		Spec: SpecVersion,
	}

	for event, matchers := range cfg.Hooks {
		for _, m := range matchers {
			for _, entry := range m.Hooks {
				hook := CanonicalHook{
					Event: event,
					Handler: HookHandler{
						Type:           entry.Type,
						Command:        entry.Command,
						Timeout:        entry.Timeout,
						StatusMessage:  entry.StatusMessage,
						Async:          entry.Async,
						URL:            entry.URL,
						Headers:        entry.Headers,
						AllowedEnvVars: entry.AllowedEnvVars,
						Prompt:         entry.Prompt,
						Model:          entry.Model,
						Agent:          entry.Agent,
					},
				}

				if m.Matcher != "" {
					matcherJSON, _ := json.Marshal(m.Matcher)
					hook.Matcher = matcherJSON
				}

				ch.Hooks = append(ch.Hooks, hook)
			}
		}
	}

	return ch
}

// Verify re-decodes encoded output with the target adapter to check fidelity.
// Returns nil if verification passes, or an error describing the mismatch.
func Verify(encoded *EncodedResult, adapter HookAdapter, original *CanonicalHooks) error {
	if encoded == nil || len(encoded.Content) == 0 {
		return nil // nothing to verify (e.g., hookless provider)
	}

	decoded, err := adapter.Decode(encoded.Content)
	if err != nil {
		return &VerifyError{
			Provider: adapter.ProviderSlug(),
			Detail:   "failed to re-decode encoded output: " + err.Error(),
		}
	}

	// Compare structural fidelity: event count and handler commands
	if len(decoded.Hooks) != len(original.Hooks) {
		return &VerifyError{
			Provider: adapter.ProviderSlug(),
			Detail:   "hook count mismatch after round-trip",
			Expected: len(original.Hooks),
			Got:      len(decoded.Hooks),
		}
	}

	return nil
}

// VerifyError describes a verification failure after encode → decode round-trip.
type VerifyError struct {
	Provider string
	Detail   string
	Expected int
	Got      int
}

func (e *VerifyError) Error() string {
	if e.Expected > 0 || e.Got > 0 {
		return "hook verify " + e.Provider + ": " + e.Detail +
			" (expected " + itoa(e.Expected) + ", got " + itoa(e.Got) + ")"
	}
	return "hook verify " + e.Provider + ": " + e.Detail
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	return s
}

// providerBySlug finds a provider.Provider by slug from the global AllProviders list.
func providerBySlug(slug string) provider.Provider {
	for _, p := range provider.AllProviders {
		if p.Slug == slug {
			return p
		}
	}
	return provider.Provider{Slug: slug, Name: slug}
}

// legacyResultToEncoded converts a legacy converter.Result to an EncodedResult.
func legacyResultToEncoded(r *Result) *EncodedResult {
	er := &EncodedResult{
		Content:  r.Content,
		Filename: r.Filename,
		Scripts:  r.ExtraFiles,
	}
	for _, w := range r.Warnings {
		er.Warnings = append(er.Warnings, ConversionWarning{
			Severity:    "warning",
			Description: w,
		})
	}
	return er
}
