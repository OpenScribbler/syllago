package installer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// hookStorageModel classifies how a provider persists its hooks on disk.
// ADR-0020 identifies three models; Phase 1 implements the two home-scoped,
// unambiguous ones. Directory-scoped hooks are deferred to Phase 1b.
type hookStorageModel int

const (
	// hookStorageSharedJSON is a shared config file whose top-level `hooks`
	// key must be merged, never overwritten (sibling keys like permissions,
	// env, and MCP servers must survive). claude-code, crush, cursor,
	// gemini-cli, factory-droid.
	hookStorageSharedJSON hookStorageModel = iota
	// hookStorageDedicatedFile is a whole file that holds only hooks and can
	// be written verbatim. windsurf.
	hookStorageDedicatedFile
)

// hookStorageModelFor classifies a provider's hook storage and reports whether
// syllago can install its hooks in ADR-0020 Phase 1.
//
// Directory-scoped providers (copilot-cli → .github/hooks/, kiro →
// .kiro/agents/, pi → .pi/extensions/) have adapters but an unresolved
// project-vs-home layout question, so they are rejected pending Phase 1b rather
// than half-implemented. amp/codex have no adapter and are rejected by the
// caller before reaching here.
func hookStorageModelFor(slug string) (hookStorageModel, error) {
	switch slug {
	case "claude-code", "crush", "cursor", "gemini-cli", "factory-droid":
		return hookStorageSharedJSON, nil
	case "windsurf":
		// Deferred: windsurf's adapter fans one before_tool_execute hook out to
		// four split-events and only merges them back on decode when each has
		// exactly one entry. A second windsurf hook breaks that precondition, so
		// a hook's post-round-trip identity is not stable across installs —
		// uninstall/status/orphans can't reliably match it. Needs a stable
		// per-entry identity (a syllago marker), which is Phase 1b work.
		return 0, fmt.Errorf("hook install for windsurf is not yet supported (pending ADR-0020 Phase 1b: split-event fan-out needs a stable per-entry identity)")
	case "copilot-cli", "kiro", "pi":
		return 0, fmt.Errorf("hook install for %s is not yet supported (pending ADR-0020 Phase 1b: directory-scoped hook layout)", slug)
	default:
		return 0, fmt.Errorf("hook install not supported for %s", slug)
	}
}

// HookConfigPath resolves a provider's hook file per ADR-0020's path table,
// rooted at base (a home directory or a resolver-supplied base dir). It is the
// single source of truth for hook file locations — installer.hookSettingsPathImpl,
// the loadout apply path, and orphan detection all route through it so no flow
// can diverge onto a wrong path. Returns an error for providers whose hooks
// syllago cannot manage in Phase 1 (directory-scoped and adapter-less).
//
// ConfigLocations is intentionally NOT consulted — it is inconsistent across
// providers, which is the root of the "dead config" bug this work fixes.
func HookConfigPath(prov provider.Provider, base string) (string, error) {
	switch prov.Slug {
	case "crush":
		// Crush keeps hooks in its unified crush.json (XDG global config).
		return filepath.Join(base, ".config", "crush", "crush.json"), nil
	case "claude-code", "cursor", "gemini-cli", "factory-droid":
		return filepath.Join(base, prov.ConfigDir, "settings.json"), nil
	case "windsurf":
		// Deferred to Phase 1b — see hookStorageModelFor for why. The Phase 1b
		// path will be base/.windsurf/hooks.json (dedicated file).
		return "", fmt.Errorf("hook install for windsurf is not yet supported (pending ADR-0020 Phase 1b: split-event fan-out needs a stable per-entry identity)")
	case "copilot-cli", "kiro", "pi":
		return "", fmt.Errorf("hook install for %s is not yet supported (pending ADR-0020 Phase 1b: directory-scoped hook layout)", prov.Slug)
	default:
		return "", fmt.Errorf("hook install not supported for %s", prov.Slug)
	}
}

// manifestHookToCanonical converts a spec Manifest hook into the enhanced
// canonical form the HookAdapters encode/decode. The manifest matcher is a bare
// string; the canonical matcher is a json.RawMessage (a JSON-encoded string).
func manifestHookToCanonical(h converter.Hook) (converter.CanonicalHook, error) {
	ch := converter.CanonicalHook{
		Name:     h.Name,
		Event:    h.Event,
		Blocking: h.Blocking,
		Handler: converter.HookHandler{
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
			return converter.CanonicalHook{}, err
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

// canonicalizeEvent normalizes a hook event name to its canonical form. An
// event that is already a canonical key is returned unchanged; a provider-native
// name is reverse-translated using the target provider's mapping.
func canonicalizeEvent(event, slug string) string {
	if _, ok := converter.HookEvents[event]; ok {
		return event
	}
	return converter.ReverseTranslateHookEvent(event, slug)
}

// nativeEventFor returns the provider-native event key for tracking/dedup. When
// the canonical event has no direct mapping for the provider (e.g. windsurf,
// whose adapter synthesizes split events), the canonical name is used as-is;
// install/uninstall/status all compute it the same way, so lookups stay
// consistent.
func nativeEventFor(canonEvent, slug string) string {
	if nv, ok := converter.TranslateHookEvent(canonEvent, slug); ok {
		return nv
	}
	return canonEvent
}

// adapterSupportsEvent reports whether the provider's adapter can represent a
// canonical event. Adapter capabilities are the ADR-0020 canonical source of
// truth for event support; this is broader than ProviderSupportsHookEvent
// because it also recognizes providers (windsurf) whose adapter fans a single
// canonical event out to several provider-native split events.
//
// KNOWN, INTENTIONAL ASYMMETRY: direct `syllago install` gates hook events with
// this adapter-capability check, so e.g. a windsurf before_tool_execute hook
// installs. Loadout Preview still gates with converter.ProviderSupportsHookEvent
// (which does not see split-event support), so the same hook is reported
// skip-unsupported inside a loadout. This is a UX inconsistency, not dead config
// — nothing wrong ever gets written. Reconciling Preview onto adapter
// capabilities is deferred follow-up; do not "fix" it by loosening Preview here.
func adapterSupportsEvent(adapter converter.HookAdapter, canonEvent string) bool {
	for _, e := range adapter.Capabilities().Events {
		if e == canonEvent {
			return true
		}
	}
	return false
}

// hookIdentity hashes a hook's post-round-trip canonical identity
// (event|matcher|command|name). Computed on the decoded form so it is stable
// across lossy adapter transforms (e.g. windsurf wrapping a non-blocking
// command in `(cmd) || true`) and across whole-file re-serialization when other
// hooks are added or removed.
func hookIdentity(h converter.CanonicalHook) string {
	sum := sha256.Sum256([]byte(h.Event + "|" + string(h.Matcher) + "|" + h.Handler.Command + "|" + h.Name))
	return hex.EncodeToString(sum[:])
}

// roundTripIdentity encodes a single canonical hook, decodes it back through the
// same adapter, and returns the identity of the result. Returns an error if the
// adapter drops the hook entirely (e.g. a non-command handler on crush), which
// callers treat as "provider cannot represent this hook".
func roundTripIdentity(adapter converter.HookAdapter, canonHook converter.CanonicalHook) (string, error) {
	enc, err := adapter.Encode(&converter.CanonicalHooks{Spec: converter.SpecVersion, Hooks: []converter.CanonicalHook{canonHook}})
	if err != nil {
		return "", fmt.Errorf("encoding hook: %w", err)
	}
	rt, err := adapter.Decode(enc.Content)
	if err != nil {
		return "", fmt.Errorf("verifying encoded hook: %w", err)
	}
	if len(rt.Hooks) == 0 {
		return "", fmt.Errorf("hook cannot be represented by %s", adapter.ProviderSlug())
	}
	return hookIdentity(rt.Hooks[0]), nil
}

// decodeExistingHooks reads the provider's hook file and decodes it through the
// adapter. For shared-JSON providers the adapter reads only the `hooks` key,
// ignoring sibling config. A missing or empty file yields no hooks.
func decodeExistingHooks(adapter converter.HookAdapter, path string) ([]converter.CanonicalHook, error) {
	data, err := readJSONFile(path) // returns {} when the file is absent
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if len(data) == 0 {
		return nil, nil
	}
	ch, err := adapter.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("decoding existing hooks in %s: %w", path, err)
	}
	return ch.Hooks, nil
}

// KNOWN PHASE-2 CONCERN (decode → encode-all): install/uninstall re-serialize
// the ENTIRE hooks set on every mutation, not just the changed entry. Two
// consequences, both tracked for Phase 2:
//   (a) Fidelity — any provider-native field an adapter does not model is
//       dropped from previously-installed hooks when they are re-encoded.
//   (b) Determinism — adapters iterate Go maps (event → groups), so event-key
//       ordering is non-deterministic across writes, churning users' config
//       diffs even when nothing semantically changed.
// Phase 2 makes adapter encoding deterministic and promotes adapter.Verify to a
// production-load-bearing fidelity check. Until then this is acceptable because
// the routed providers (claude-code, crush, cursor, gemini-cli, factory-droid)
// model the fields syllago writes.

// writeHookFile persists an adapter's encoded hooks according to the storage
// model. Dedicated-file providers get the whole encoded file; shared-JSON
// providers get only the encoded `hooks` object merged into the real file,
// preserving every non-hook key. The dedicated-file branch is Phase 1b infra —
// no Phase 1 provider routes to it (see hookStorageModelFor).
func writeHookFile(model hookStorageModel, path string, encoded []byte) error {
	if model == hookStorageDedicatedFile {
		return writeJSONFile(path, encoded)
	}

	real, err := readJSONFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	if len(real) == 0 {
		real = []byte("{}")
	}
	hooksRaw := []byte("{}")
	if hv := gjson.GetBytes(encoded, "hooks"); hv.Exists() {
		hooksRaw = []byte(hv.Raw)
	}
	real, err = sjson.SetRawBytes(real, "hooks", hooksRaw)
	if err != nil {
		return fmt.Errorf("merging hooks into %s: %w", path, err)
	}
	return writeJSONFile(path, real)
}

// ApplyHookResult is the tracking data produced by merging one hook through the
// adapter path.
type ApplyHookResult struct {
	NativeEvent string // provider-native event key (or canonical when unmapped)
	Command     string // resolved handler command, for installed.json
	GroupHash   string // post-round-trip canonical identity
}

// ApplyCanonicalHook merges one manifest hook into the provider's hook file at
// path via the provider's HookAdapter (ADR-0020 Phase 1), preserving sibling
// keys for shared-JSON providers. The path is caller-supplied so callers can
// use their own resolver; resolvedCommand, when non-empty, overrides the hook's
// handler command (callers resolve script/relative paths first).
//
// This is the loadout apply entry point; installHook uses the same building
// blocks directly because it interleaves snapshotting and dedup.
func ApplyCanonicalHook(prov provider.Provider, h converter.Hook, path, resolvedCommand string) (ApplyHookResult, error) {
	adapter := converter.AdapterFor(prov.Slug)
	if adapter == nil {
		return ApplyHookResult{}, fmt.Errorf("hook install not supported for %s (no encoder)", prov.Name)
	}
	model, err := hookStorageModelFor(prov.Slug)
	if err != nil {
		return ApplyHookResult{}, err
	}

	canonHook, err := manifestHookToCanonical(h)
	if err != nil {
		return ApplyHookResult{}, fmt.Errorf("building canonical hook: %w", err)
	}
	canonEvent := canonicalizeEvent(h.Event, prov.Slug)
	canonHook.Event = canonEvent
	if resolvedCommand != "" {
		canonHook.Handler.Command = resolvedCommand
	}

	if !adapterSupportsEvent(adapter, canonEvent) {
		return ApplyHookResult{}, fmt.Errorf("hook %q: %s does not support hook event %q", h.Name, prov.Name, h.Event)
	}

	groupHash, err := roundTripIdentity(adapter, canonHook)
	if err != nil {
		return ApplyHookResult{}, err
	}

	existing, err := decodeExistingHooks(adapter, path)
	if err != nil {
		return ApplyHookResult{}, err
	}
	all := make([]converter.CanonicalHook, 0, len(existing)+1)
	all = append(all, existing...)
	all = append(all, canonHook)

	encoded, err := adapter.Encode(&converter.CanonicalHooks{Spec: converter.SpecVersion, Hooks: all})
	if err != nil {
		return ApplyHookResult{}, fmt.Errorf("encoding hooks: %w", err)
	}
	if err := writeHookFile(model, path, encoded.Content); err != nil {
		return ApplyHookResult{}, err
	}

	return ApplyHookResult{
		NativeEvent: nativeEventFor(canonEvent, prov.Slug),
		Command:     canonHook.Handler.Command,
		GroupHash:   groupHash,
	}, nil
}

// readSingleManifestHook reads a canonical hook.json (hooks/0.1 Manifest) and
// returns its single hook. If path is a directory, hook.json inside it is used.
// Syllago-written hook.json always contains exactly one hook.
func readSingleManifestHook(path string) (converter.Hook, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return converter.Hook{}, err
	}
	if fi.IsDir() {
		path = filepath.Join(path, "hook.json")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return converter.Hook{}, err
	}
	m, err := converter.ParseManifest(data)
	if err != nil {
		return converter.Hook{}, err
	}
	if len(m.Hooks) != 1 {
		return converter.Hook{}, fmt.Errorf("hook file has %d hooks; syllago hook.json must contain exactly 1", len(m.Hooks))
	}
	return m.Hooks[0], nil
}
