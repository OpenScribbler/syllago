# Reference Implementations

This directory contains reference implementations of the **Hook Interchange Format Specification**. Each subdirectory is an independent, runnable implementation in a different language. All implementations target the same conformance level and consume the same test vectors.

---

## What these implement

Each implementation covers **Core conformance** (§8.1) for parsing and validation, and **Extended conformance** (§8.2) for exit code resolution and matcher handling:

| Module | Spec sections | Description |
|--------|--------------|-------------|
| `manifest` | §3.1–§3.5 | Parse JSON/YAML manifests; validate required fields; ignore unknown fields per §3.2 |
| `exit_codes` | §4, §5.2–§5.3 | Resolve a hook's exit code and JSON `decision` field to a final `allow`/`block`/`warn_allow`/`ask` result |
| `matchers` | §6.1–§6.5 | Translate canonical tool names, MCP objects, and pattern matchers to provider-native form; reverse-decode native names back to canonical |
| `claude_code` | §7.1, §7.3 | Encode/decode between the canonical format and Claude Code's `settings.json` hook structure |
| `gemini_cli` | §7.1, §7.3 | Encode/decode between the canonical format and Gemini CLI's flat-array hook structure |

---

## Running the Python implementation

```bash
cd python
pip install -e ".[dev]"
pytest tests/
```

**Requirements:** Python 3.11+, pip.

The `[dev]` extras install `pytest`. The `pyyaml` runtime dependency is pulled in automatically.

---

## Running the TypeScript implementation

```bash
cd typescript
npm install
npm test
```

**Requirements:** Node.js 18+, npm.

Tests run with Vitest. The `build` script (`npm run build`) compiles to `dist/` with `tsc`, which is also required before importing the package from another project.

---

## What the conformance tests verify

Tests are organized to mirror the test-vector categories in `../test-vectors/`. See that directory's `README.md` for the full vector index.

**Manifest parsing and validation**
Verify that `parse_manifest` / `parseManifest` accepts valid JSON and YAML inputs and produces structurally equivalent results from either format. Verify that `validate_manifest` / `validateManifest` accepts all canonical vectors in `test-vectors/canonical/` and rejects every document in `test-vectors/invalid/`, checking that each required-field violation produces the correct error.

**Exit code resolution**
Drive `resolve` with the full truth table from §5.3: every combination of `blocking`, exit code `{0, 1, 2, other}`, and `decision` value `{allow, deny, ask, absent}`. Verify non-blocking downgrade (exit code 2 becomes 1 when `blocking` is false) and that out-of-range exit codes normalize to 1.

**Matcher resolution**
Verify bare-string tool vocabulary lookups for both Claude Code and Gemini CLI. Verify MCP object encoding to each provider's combined-string format (`mcp__server__tool` vs `mcp_server_tool`). Verify pattern pass-through. Verify array OR expansion. Verify that unknown canonical names pass through with a warning rather than silently dropping. Verify the reverse decode path for each provider.

**Claude Code adapter**
For each paired `canonical/X.json` + `claude-code/X.json` vector: parse the canonical input, run `encode`, and assert structural equivalence with the provider vector (ignoring `_comment` and `_warnings` metadata keys). Verify round-trip using `roundtrip-source.json` and `roundtrip-canonical.json`: decode the source to canonical, compare against the expected canonical form, re-encode back to Claude Code, and compare against the source.

**Gemini CLI adapter**
Same approach as Claude Code, using `gemini-cli/X.json` provider vectors. Verify timeout unit conversion (canonical seconds ↔ Gemini milliseconds). Verify that array matchers expand to one entry per tool name rather than producing a compound `toolMatcher` string.

---

## How to add a new language

1. Create a subdirectory: `examples/<language>/`.

2. Implement these five modules (names are conventional — use idiomatic naming for your language):

   | Module | Functions to implement |
   |--------|----------------------|
   | `manifest` | `parse_manifest(data, format)` → dict/object; `validate_manifest(manifest)` → list of error strings |
   | `exit_codes` | `resolve(blocking, exit_code, decision)` → result enum/string |
   | `matchers` | `resolve_matcher(matcher, provider)` → string or list; `parse_matcher(native, provider)` → canonical form |
   | `claude_code` | `encode(manifest)` → native dict; `decode(native)` → canonical manifest |
   | `gemini_cli` | `encode(manifest)` → native dict; `decode(native)` → canonical manifest |

3. Write tests that consume the test vectors from `../test-vectors/`:

   - **Invalid vectors** (`invalid/*.json`): each must be rejected by `validate_manifest` with a non-empty error list.
   - **Canonical → provider vectors**: for each `canonical/X.json` + `<provider>/X.json` pair, parse the canonical input, call `encode`, and compare against the provider vector (ignore `_comment` / `_warnings` keys).
   - **Round-trip vectors** (`claude-code/roundtrip-*.json`): decode source → compare → re-encode → compare.

4. Add a `README` or inline comment documenting which conformance level your implementation targets (Core, Extended, or Full per §8.1–§8.3) and any known gaps.

5. Add a run entry to this file following the pattern of the Python and TypeScript sections above.

---

## How to add a new provider adapter

Adding a provider adapter means implementing the encode and decode paths for a new AI coding tool.

**What to implement**

1. An `EVENT_MAP` dict mapping canonical event names (snake_case, from §4 of the spec) to the provider's native event names. Add a `REVERSE_EVENT_MAP` for the decode direction.

2. An `encode(manifest)` function that:
   - Iterates `manifest["hooks"]`
   - Looks up each canonical event name in `EVENT_MAP`; drops hooks with no mapping (silently, per §7.3)
   - Calls `resolve_matcher(hook["matcher"], "<provider-slug>")` to translate the matcher
   - Translates the handler fields, dropping any the provider does not support (`cwd`, `env`, `platform` are commonly unsupported)
   - Converts units where needed (e.g., timeout seconds → milliseconds)
   - Returns a dict in the provider's native settings structure

3. A `decode(native)` function that:
   - Iterates the provider's native hook entries
   - Looks up each native event name in `REVERSE_EVENT_MAP`; skips unknown events
   - Calls `parse_matcher(native_matcher, "<provider-slug>")` to recover the canonical matcher
   - Reconstructs a canonical handler dict, converting units back
   - Infers `blocking` from the event type when the provider has no explicit blocking field
   - Returns `{"spec": "hooks/0.1", "hooks": [...]}`

**Registering tool name mappings**

Add the new provider slug and its native tool names to `TOOL_VOCABULARY` in `matchers` (or the equivalent in your language). Use `None` for tools the provider does not support. The MCP encoding format (`mcp__server__tool`, `mcp_server_tool`, `server/tool`, etc.) belongs in `_MCP_PROVIDER_FORMAT`.

**Which test vectors to target**

- Implement conversion for the canonical vectors that have a `<provider>/` counterpart in `test-vectors/`. If no provider vectors exist yet, add them following the format described in `../test-vectors/README.md`.
- Add a round-trip vector pair (`roundtrip-source.json` + `roundtrip-canonical.json`) under `test-vectors/<provider>/` to verify lossless decode → re-encode.
- If the provider lacks a capability that appears in `canonical/degradation-input-rewrite.json`, add a `test-vectors/<provider>/degradation-input-rewrite.json` vector and implement the degradation strategy from §11.
