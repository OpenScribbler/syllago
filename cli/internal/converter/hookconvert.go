package converter

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ErrNoHookEncoder reports a provider whose hook format syllago cannot write.
var ErrNoHookEncoder = errors.New("syllago cannot write hooks for this provider")

// ConvertHooks reads hook content and encodes it for toSlug through that
// provider's HookAdapter, the encoder install uses, so a conversion shows
// what an install writes. raw is a hooks/0.1 manifest (the Library's
// hook.json), a legacy single-hook hook.json, or fromSlug's own hook file.
//
// A target with no adapter returns ErrNoHookEncoder. A result with nil
// Content means the target can represent none of the hooks.
func ConvertHooks(raw []byte, fromSlug, toSlug string) (*Result, error) {
	hooks, err := DecodeHooks(raw, fromSlug)
	if err != nil {
		return nil, err
	}
	adapter := AdapterFor(toSlug)
	if adapter == nil {
		return nil, fmt.Errorf("%w: %s", ErrNoHookEncoder, toSlug)
	}
	// Install reads an event name it does not know as the target's own, so
	// a conversion does the same.
	for i, h := range hooks.Hooks {
		if _, ok := HookEvents[h.Event]; !ok {
			hooks.Hooks[i].Event = ReverseTranslateHookEvent(h.Event, toSlug)
		}
	}
	enc, err := adapter.Encode(hooks)
	if err != nil {
		return nil, err
	}
	res := &Result{Content: enc.Content, Filename: enc.Filename, ExtraFiles: enc.Scripts}
	for _, w := range append(enc.Warnings, CheckStructuredOutputLoss(fromSlug, toSlug)...) {
		text := w.Description
		if w.Suggestion != "" {
			text += " (" + w.Suggestion + ")"
		}
		res.Warnings = append(res.Warnings, text)
	}
	// An adapter drops what its provider cannot hold, which can be every
	// hook; reading its own output back tells.
	if back, err := adapter.Decode(enc.Content); err != nil || len(back.Hooks) == 0 {
		res.Content = nil
	}
	return res, nil
}

// DecodeHooks reads hook content into canonical hooks: a hooks/0.1
// manifest, a legacy single-hook hook.json, or fromSlug's own hook file.
// fromSlug also names the provider whose event names a manifest or legacy
// hook may still carry.
func DecodeHooks(raw []byte, fromSlug string) (*CanonicalHooks, error) {
	switch DetectHookFormat(raw) {
	case "manifest":
		m, err := ParseManifest(raw)
		if err != nil {
			return nil, err
		}
		return canonicalFromManifest(m.Hooks, fromSlug)
	case "flat":
		hd, err := ParseFlat(raw)
		if err != nil {
			return nil, err
		}
		if fromSlug == "" {
			fromSlug = hd.SourceProvider
		}
		// A legacy hook.json was copied from a provider's settings, so its
		// matcher names that provider's tools.
		matcher := hd.Matcher
		if fromSlug != "" {
			matcher = ReverseTranslateMatcher(matcher, fromSlug)
		}
		var hooks []Hook
		for _, entry := range hd.Hooks {
			m, err := ManifestFromHookData(HookData{Event: hd.Event, Matcher: matcher, Hooks: []HookEntry{entry}})
			if err != nil {
				return nil, err
			}
			hooks = append(hooks, m.Hooks...)
		}
		return canonicalFromManifest(hooks, fromSlug)
	}
	adapter := AdapterFor(fromSlug)
	if adapter == nil {
		if fromSlug == "" {
			return nil, errors.New("the content is a provider's hook file, and no source provider was given")
		}
		return nil, fmt.Errorf("syllago cannot read %s hook files", fromSlug)
	}
	return adapter.Decode(raw)
}

func canonicalFromManifest(hooks []Hook, fromSlug string) (*CanonicalHooks, error) {
	out := &CanonicalHooks{Spec: SpecVersion}
	for _, h := range hooks {
		ch, err := CanonicalHookFromManifest(h)
		if err != nil {
			return nil, err
		}
		if _, ok := HookEvents[ch.Event]; !ok && fromSlug != "" {
			ch.Event = ReverseTranslateHookEvent(ch.Event, fromSlug)
		}
		out.Hooks = append(out.Hooks, ch)
	}
	return out, nil
}

// CanonicalHookFromManifest converts a spec Manifest hook into the enhanced
// canonical form the HookAdapters encode/decode. The manifest matcher is a bare
// string; the canonical matcher is a json.RawMessage (a JSON-encoded string).
func CanonicalHookFromManifest(h Hook) (CanonicalHook, error) {
	ch := CanonicalHook{
		Name:     h.Name,
		Event:    h.Event,
		Blocking: h.Blocking,
		Handler: HookHandler{
			Type:           h.Handler.Type,
			Command:        h.Handler.Command,
			Platform:       h.Handler.Platform,
			CWD:            h.Handler.Cwd,
			Env:            h.Handler.Env,
			Timeout:        h.Handler.Timeout,
			TimeoutAction:  h.Handler.TimeoutAction,
			StatusMessage:  h.Handler.StatusMessage,
			Async:          h.Handler.Async,
			URL:            h.Handler.URL,
			Headers:        h.Handler.Headers,
			AllowedEnvVars: h.Handler.AllowedEnvVars,
			Prompt:         h.Handler.Prompt,
			Model:          h.Handler.Model,
			Agent:          h.Handler.Agent,
		},
	}
	if ch.Handler.Type == "" {
		ch.Handler.Type = "command"
	}
	if h.Matcher != "" {
		m, err := json.Marshal(h.Matcher)
		if err != nil {
			return CanonicalHook{}, err
		}
		ch.Matcher = m
	}
	if len(h.Provider) > 0 {
		var pd map[string]any
		if err := json.Unmarshal(h.Provider, &pd); err == nil {
			ch.ProviderData = pd
		}
	}
	return ch, nil
}
