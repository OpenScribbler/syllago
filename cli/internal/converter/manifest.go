package converter

import (
	"encoding/json"
	"fmt"
)

// Manifest is the top-level canonical hook manifest defined in
// docs/spec/hooks/hooks.md §3.3. A library hook.json holds exactly one
// Manifest; when a provider settings.json groups N commands under one
// (event, matcher), each becomes its own Manifest with a single Hook.
type Manifest struct {
	Spec  string `json:"spec"`
	Hooks []Hook `json:"hooks"`
}

// Hook is a single hook definition per spec §3.4. One event, one optional
// matcher, one handler. Fields not yet exercised by syllago are omitted.
type Hook struct {
	Name     string          `json:"name,omitempty"`
	Event    string          `json:"event"`
	Matcher  string          `json:"matcher,omitempty"`
	Handler  Handler         `json:"handler"`
	Blocking bool            `json:"blocking,omitempty"`
	Provider json.RawMessage `json:"provider_data,omitempty"`
}

// Handler is the handler definition per spec §3.5. Singular per Hook.
// Timeout is in seconds (canonical unit).
type Handler struct {
	Type          string            `json:"type"`
	Command       string            `json:"command,omitempty"`
	Platform      map[string]string `json:"platform,omitempty"`
	Cwd           string            `json:"cwd,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	Timeout       int               `json:"timeout,omitempty"`
	TimeoutAction string            `json:"timeout_action,omitempty"`
	Async         bool              `json:"async,omitempty"`
	StatusMessage string            `json:"status_message,omitempty"`

	// HTTP handler fields (type: "http")
	URL            string            `json:"url,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	AllowedEnvVars []string          `json:"allowed_env_vars,omitempty"`

	// Prompt handler fields (type: "prompt")
	Prompt string `json:"prompt,omitempty"`
	Model  string `json:"model,omitempty"`

	// Agent handler fields (type: "agent")
	Agent json.RawMessage `json:"agent,omitempty"`
}

// ManifestFromHookData wraps a single-handler HookData in a spec-conforming
// Manifest. HookData must have exactly one entry in Hooks; SplitSettingsHooks
// guarantees this for post-2026-04-23 code. Returns an error otherwise so
// callers catch programming mistakes rather than silently dropping handlers.
func ManifestFromHookData(hd HookData) (Manifest, error) {
	if len(hd.Hooks) != 1 {
		return Manifest{}, fmt.Errorf("ManifestFromHookData: expected 1 handler, got %d", len(hd.Hooks))
	}
	entry := hd.Hooks[0]
	h := Hook{
		Event:   hd.Event,
		Matcher: hd.Matcher,
		Handler: Handler{
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
	if h.Handler.Type == "" {
		h.Handler.Type = "command"
	}
	return Manifest{Spec: SpecVersion, Hooks: []Hook{h}}, nil
}

// HookDataFromManifest converts a spec Manifest back into a HookData for code
// paths (installer settings.json injection, TUI drill-in) that still operate
// on the flat legacy shape. A Manifest with N hooks yields N HookData; usually
// N == 1 because syllago writes one Manifest per handler.
func HookDataFromManifest(m Manifest) ([]HookData, error) {
	if m.Spec != SpecVersion {
		return nil, fmt.Errorf("HookDataFromManifest: unexpected spec %q, want %q", m.Spec, SpecVersion)
	}
	out := make([]HookData, 0, len(m.Hooks))
	for i, h := range m.Hooks {
		if h.Event == "" {
			return nil, fmt.Errorf("HookDataFromManifest: hooks[%d].event is empty", i)
		}
		htype := h.Handler.Type
		if htype == "" {
			htype = "command"
		}
		entry := HookEntry{
			Type:           htype,
			Command:        h.Handler.Command,
			Timeout:        h.Handler.Timeout,
			StatusMessage:  h.Handler.StatusMessage,
			Async:          h.Handler.Async,
			URL:            h.Handler.URL,
			Headers:        h.Handler.Headers,
			AllowedEnvVars: h.Handler.AllowedEnvVars,
			Prompt:         h.Handler.Prompt,
			Model:          h.Handler.Model,
			Agent:          h.Handler.Agent,
		}
		out = append(out, HookData{
			Event:   h.Event,
			Matcher: h.Matcher,
			Hooks:   []HookEntry{entry},
		})
	}
	return out, nil
}

// ParseManifest parses a hook.json payload that conforms to the spec shape.
// Legacy flat HookData files are rejected — callers that need legacy support
// must dispatch on DetectHookFormat first.
func ParseManifest(data []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("parsing manifest: %w", err)
	}
	if m.Spec == "" {
		return Manifest{}, fmt.Errorf("parsing manifest: missing spec field (expected %q)", SpecVersion)
	}
	if len(m.Hooks) == 0 {
		return Manifest{}, fmt.Errorf("parsing manifest: hooks[] is empty")
	}
	return m, nil
}
