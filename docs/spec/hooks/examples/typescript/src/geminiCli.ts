/**
 * geminiCli.ts — Gemini CLI Hook Adapter
 * Reference implementation: Extended conformance (§8.2)
 *
 * Covers:
 *   §7.1  Decode — native Gemini CLI hooks → canonical HookManifest
 *   §7.3  Encode — canonical HookManifest → native Gemini CLI hooks
 *   §6.1  Bare-string matchers via tool vocabulary
 *   §6.3  MCP matchers: combined format `mcp_<server>_<tool>` (single underscore)
 *   §6.4  Array matchers: expanded to one hook entry per element during encode
 *
 * Gemini CLI-specific behaviour:
 *   - Timeout is in MILLISECONDS (canonical unit is seconds; multiply × 1000 on
 *     encode, divide ÷ 1000 on decode).
 *   - Native configuration lives in `.gemini/settings.json` under the `hooks` key.
 *   - Non-tool events (e.g. SessionStart) carry no `toolMatcher` field.
 *   - Unsupported fields (platform, cwd, env, provider_data for other providers)
 *     are silently dropped on encode.
 */

import type { HookDefinition, HookManifest } from "./manifest.js";
import { parseMatcher, resolveMatcher } from "./matchers.js";

// ---------------------------------------------------------------------------
// Native Gemini CLI types
// ---------------------------------------------------------------------------

/** A single hook entry in `.gemini/settings.json`. */
export interface GeminiHook {
  /** Native event name (e.g. "BeforeTool", "SessionStart"). */
  trigger: string;
  /** Provider-native tool name or MCP combined string. Absent for non-tool events. */
  toolMatcher?: string;
  /** Shell command to execute. */
  command: string;
  /** Maximum execution time in milliseconds. */
  timeoutMs: number;
}

/** Top-level structure of the `hooks` key in `.gemini/settings.json`. */
export interface GeminiCliConfig {
  hooks: GeminiHook[];
}

// ---------------------------------------------------------------------------
// Event map: canonical ↔ Gemini CLI native  (events.md §4, gemini-cli column)
// ---------------------------------------------------------------------------

/**
 * Maps canonical event names to Gemini CLI native event names.
 *
 * Events absent from this map are not supported by Gemini CLI. Unsupported
 * events should be caught by the validate stage (§7.2) before encode is
 * called; the encoder silently drops them.
 */
export const EVENT_MAP: ReadonlyMap<string, string> = new Map([
  // §1 Core events
  ["before_tool_execute", "BeforeTool"],
  ["after_tool_execute", "AfterTool"],
  ["session_start", "SessionStart"],
  ["session_end", "SessionEnd"],
  ["before_prompt", "BeforeAgent"],
  ["agent_stop", "AfterAgent"],
  // §2 Extended events supported by Gemini CLI
  ["before_compact", "PreCompress"],
  ["notification", "Notification"],
  ["before_model", "BeforeModel"],
  ["after_model", "AfterModel"],
  ["before_tool_selection", "BeforeToolSelection"],
]);

/** Reverse map: Gemini CLI native event name → canonical event name. */
const EVENT_MAP_REVERSE: ReadonlyMap<string, string> = new Map(
  Array.from(EVENT_MAP.entries()).map(([canonical, native]) => [native, canonical]),
);

// ---------------------------------------------------------------------------
// Encode: canonical HookManifest → GeminiCliConfig
// ---------------------------------------------------------------------------

/**
 * Encode a canonical HookManifest into Gemini CLI native format.
 *
 * Per §7.3 the adapter:
 *   1. Translates canonical event names to Gemini CLI native names.
 *   2. Translates canonical tool names (and MCP matchers) to Gemini CLI native
 *      names via the shared tool vocabulary (provider slug: "gemini-cli").
 *   3. Converts timeout from seconds → milliseconds.
 *   4. Expands array matchers (§6.4) into one hook entry per resolved element.
 *   5. Skips hooks whose canonical event has no Gemini CLI mapping.
 *   6. Omits `toolMatcher` for non-tool events.
 *
 * Dropped fields (no Gemini CLI support): `platform`, `cwd`, `env`,
 * `provider_data` for non-gemini-cli providers, `blocking` (Gemini CLI
 * does not carry an explicit blocking flag per-entry).
 */
export function encode(manifest: HookManifest): GeminiCliConfig {
  const nativeHooks: GeminiHook[] = [];

  for (const hook of manifest.hooks) {
    const nativeEvent = EVENT_MAP.get(hook.event);
    if (nativeEvent === undefined) {
      // Event not supported by Gemini CLI — skip.
      continue;
    }

    const isToolEvent =
      hook.event === "before_tool_execute" || hook.event === "after_tool_execute";

    if (isToolEvent && hook.matcher !== undefined) {
      // Resolve the matcher to an array of native tool strings (§6.4 expands arrays).
      const resolvedMatchers = resolveMatcher(hook.matcher, "gemini-cli");

      if (resolvedMatchers.length > 0) {
        // One hook entry per resolved matcher element.
        for (const toolMatcherStr of resolvedMatchers) {
          nativeHooks.push(buildNativeHook(hook, nativeEvent, toolMatcherStr));
        }
      } else {
        // Matcher resolved to nothing for this provider — emit a wildcard entry.
        nativeHooks.push(buildNativeHook(hook, nativeEvent, undefined));
      }
    } else {
      // Non-tool event or omitted matcher — no toolMatcher field.
      nativeHooks.push(buildNativeHook(hook, nativeEvent, undefined));
    }
  }

  return { hooks: nativeHooks };
}

/**
 * Build a single GeminiHook from a canonical HookDefinition, a resolved
 * native event name, and an optional resolved tool matcher string.
 */
function buildNativeHook(
  hook: HookDefinition,
  nativeEvent: string,
  toolMatcher: string | undefined,
): GeminiHook {
  const command = hook.handler.command ?? "";
  // Canonical timeout is in seconds; Gemini CLI uses milliseconds (§7.3 step 3).
  const timeoutMs = (hook.handler.timeout ?? 0) * 1000;

  const nativeHook: GeminiHook = { trigger: nativeEvent, command, timeoutMs };

  if (toolMatcher !== undefined) {
    nativeHook.toolMatcher = toolMatcher;
  }

  return nativeHook;
}

// ---------------------------------------------------------------------------
// Decode: GeminiCliConfig → canonical HookManifest
// ---------------------------------------------------------------------------

/**
 * Decode a Gemini CLI native configuration into a canonical HookManifest.
 *
 * Per §7.1 the adapter:
 *   1. Translates Gemini CLI native event names to canonical names.
 *   2. Parses `toolMatcher` (plain tool name or MCP combined string) to a
 *      canonical matcher via the shared vocabulary (provider slug: "gemini-cli").
 *   3. Converts timeout from milliseconds → seconds.
 *   4. Preserves unrecognised event names as-is (forward compatibility, §3.2).
 */
export function decode(native: GeminiCliConfig): HookManifest {
  const hooks: HookDefinition[] = native.hooks.map((entry) => decodeHook(entry));
  return { spec: "hooks/0.1", hooks };
}

function decodeHook(entry: GeminiHook): HookDefinition {
  // Translate native event name → canonical (fall back to native string per §3.2).
  const canonicalEvent = EVENT_MAP_REVERSE.get(entry.trigger) ?? entry.trigger;

  // Convert milliseconds → seconds (canonical unit).
  const timeoutSeconds = entry.timeoutMs > 0 ? entry.timeoutMs / 1000 : undefined;

  const hook: HookDefinition = {
    event: canonicalEvent,
    handler: {
      type: "command",
      command: entry.command,
      ...(timeoutSeconds !== undefined ? { timeout: timeoutSeconds } : {}),
    },
  };

  // Parse the toolMatcher into a canonical matcher expression (§6.1, §6.3).
  if (entry.toolMatcher !== undefined && entry.toolMatcher !== "") {
    hook.matcher = parseMatcher(entry.toolMatcher, "gemini-cli");
  }

  return hook;
}
