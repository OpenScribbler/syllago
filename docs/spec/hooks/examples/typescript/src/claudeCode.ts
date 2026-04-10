/**
 * claudeCode.ts — Claude Code adapter for the Hook Interchange Format.
 *
 * Implements encode (canonical → Claude Code native) and decode (Claude Code
 * native → canonical) following the conversion pipeline in §7.
 *
 * Claude Code native format (lives in .claude/settings.json):
 *
 *   {
 *     "hooks": {
 *       "PreToolUse": [
 *         { "matcher": "Bash", "hooks": [{ "type": "command", "command": "...", "timeout": 10 }] }
 *       ]
 *     }
 *   }
 *
 * Event mapping:  events.md §4
 * Tool mapping:   tools.md §1 (claude-code column)
 * MCP format:     `mcp__<server>__<tool>` (tools.md §2)
 */

import type { HookDefinition, HookManifest, MatcherObject } from "./manifest.js";
import { parseMatcher, resolveMatcher } from "./matchers.js";

// ---------------------------------------------------------------------------
// Event map — canonical → Claude Code native   (events.md §4)
// ---------------------------------------------------------------------------

/**
 * Maps every canonical event name that Claude Code supports to its native name.
 * Provider-exclusive entries (§3) and events with no Claude Code support are
 * excluded from encoding but are handled in the reverse map for decode.
 */
export const EVENT_MAP: ReadonlyMap<string, string> = new Map([
  // §1 Core events
  ["before_tool_execute", "PreToolUse"],
  ["after_tool_execute", "PostToolUse"],
  ["session_start", "SessionStart"],
  ["session_end", "SessionEnd"],
  ["before_prompt", "UserPromptSubmit"],
  ["agent_stop", "Stop"],
  // §2 Extended events supported by Claude Code
  ["before_compact", "PreCompact"],
  ["notification", "Notification"],
  ["error_occurred", "StopFailure"],
  ["tool_use_failure", "PostToolUseFailure"],
  ["file_changed", "FileChanged"],
  ["subagent_start", "SubagentStart"],
  ["subagent_stop", "SubagentStop"],
  ["permission_request", "PermissionRequest"],
  // §3 Provider-exclusive (Claude Code origin)
  ["config_change", "ConfigChange"],
]);

/** Reverse map: Claude Code native event name → canonical. */
const EVENT_MAP_REVERSE: ReadonlyMap<string, string> = new Map(
  Array.from(EVENT_MAP.entries()).map(([canonical, native]) => [native, canonical]),
);

// ---------------------------------------------------------------------------
// Tool vocabulary — canonical → Claude Code native   (tools.md §1)
// ---------------------------------------------------------------------------

const TOOL_VOCAB: ReadonlyMap<string, string> = new Map([
  ["shell", "Bash"],
  ["file_read", "Read"],
  ["file_write", "Write"],
  ["file_edit", "Edit"],
  ["search", "Grep"],
  ["find", "Glob"],
  ["web_search", "WebSearch"],
  ["web_fetch", "WebFetch"],
  ["agent", "Agent"],
]);

const TOOL_VOCAB_REVERSE: ReadonlyMap<string, string> = new Map(
  Array.from(TOOL_VOCAB.entries()).map(([canonical, native]) => [native, canonical]),
);

// ---------------------------------------------------------------------------
// MCP helpers — `mcp__<server>__<tool>` format   (tools.md §2)
// ---------------------------------------------------------------------------

function encodeMcp(server: string, tool: string | undefined): string {
  return tool !== undefined ? `mcp__${server}__${tool}` : `mcp__${server}`;
}

function isMcpString(s: string): boolean {
  return s.startsWith("mcp__");
}

function decodeMcp(s: string): { server: string; tool?: string } {
  // Format: mcp__<server>__<tool>  or  mcp__<server>
  const rest = s.slice("mcp__".length); // strip leading "mcp__"
  const sep = rest.indexOf("__");
  if (sep === -1) {
    return { server: rest };
  }
  return { server: rest.slice(0, sep), tool: rest.slice(sep + 2) };
}

// ---------------------------------------------------------------------------
// Claude Code native format types
// ---------------------------------------------------------------------------

export interface ClaudeCodeHookEntry {
  /** Claude Code-native tool matcher string. Empty string = wildcard. */
  matcher?: string;
  hooks: ClaudeCodeHookItem[];
}

export interface ClaudeCodeHookItem {
  type: string;
  command?: string;
  timeout?: number;
  [key: string]: unknown;
}

/**
 * The shape of the `hooks` block inside `.claude/settings.json`.
 * Keys are Claude Code native event names (e.g. "PreToolUse").
 */
export interface ClaudeCodeConfig {
  hooks: Record<string, ClaudeCodeHookEntry[]>;
}

// ---------------------------------------------------------------------------
// encode — canonical HookManifest → ClaudeCodeConfig
// ---------------------------------------------------------------------------

/**
 * Encode a canonical hook manifest into Claude Code's native format.
 *
 * Per §7.3 the adapter:
 *   1. Translates canonical event names to provider-native names.
 *   2. Translates canonical tool names in matchers to Claude Code names.
 *   3. Timeout is already in seconds — no unit conversion needed.
 *   4. Renders `provider_data["claude-code"]` if present (ignored otherwise).
 *   5. Drops `platform`, `cwd`, `env` — Claude Code does not support them.
 *
 * Hooks whose canonical event has no Claude Code mapping are silently dropped
 * (they should be caught by the validate stage before this is called).
 */
export function encode(manifest: HookManifest): ClaudeCodeConfig {
  const result: Record<string, ClaudeCodeHookEntry[]> = {};

  for (const hook of manifest.hooks) {
    const nativeEvent = EVENT_MAP.get(hook.event);
    if (nativeEvent === undefined) {
      // No Claude Code mapping for this event — skip.
      continue;
    }

    const nativeMatcher = resolveCanonicalMatcher(hook.matcher);
    const item = buildHookItem(hook);

    const entries = result[nativeEvent] ?? [];
    entries.push({ matcher: nativeMatcher, hooks: [item] });
    result[nativeEvent] = entries;
  }

  return { hooks: result };
}

/** Resolve a canonical matcher to the Claude Code native string. */
function resolveCanonicalMatcher(
  matcher: string | MatcherObject | Array<string | MatcherObject> | undefined,
): string {
  if (matcher === undefined) return "";
  const resolved = resolveMatcher(matcher, "claude-code");
  return resolved.join("|") || "";
}

/** Build a single Claude Code hook item from a canonical HookDefinition. */
function buildHookItem(hook: HookDefinition): ClaudeCodeHookItem {
  const item: ClaudeCodeHookItem = { type: hook.handler.type };

  if (hook.handler.command !== undefined) {
    item.command = hook.handler.command;
  }

  if (hook.handler.timeout !== undefined) {
    item.timeout = hook.handler.timeout;
  }

  // Merge any provider_data["claude-code"] fields into the item (§3.6).
  const pd = hook.provider_data?.["claude-code"];
  if (pd !== undefined) {
    for (const [k, v] of Object.entries(pd)) {
      if (!(k in item)) {
        item[k] = v;
      }
    }
  }

  // Dropped fields (no Claude Code support):
  //   hook.handler.platform  — platform_commands capability
  //   hook.handler.cwd       — configurable_cwd capability
  //   hook.handler.env       — custom_env capability

  return item;
}

// ---------------------------------------------------------------------------
// decode — ClaudeCodeConfig → canonical HookManifest
// ---------------------------------------------------------------------------

/**
 * Decode a Claude Code native hook configuration into a canonical HookManifest.
 *
 * Per §7.1 the adapter:
 *   1. Translates native event names to canonical names.
 *   2. Translates native tool names in matchers to canonical names.
 *   3. Timeout is in seconds — no unit conversion needed.
 *   4. Preserves unknown native fields in `provider_data["claude-code"]`.
 *
 * Unknown native event names are passed through as-is (forward compatibility).
 */
export function decode(native: ClaudeCodeConfig): HookManifest {
  const hooks: HookDefinition[] = [];

  for (const [nativeEvent, entries] of Object.entries(native.hooks)) {
    const canonicalEvent = EVENT_MAP_REVERSE.get(nativeEvent) ?? nativeEvent;

    for (const entry of entries) {
      for (const item of entry.hooks) {
        const hook = buildHookDefinition(canonicalEvent, entry.matcher, item);
        hooks.push(hook);
      }
    }
  }

  return { spec: "hooks/0.1", hooks };
}

/** Build a canonical HookDefinition from a decoded Claude Code entry. */
function buildHookDefinition(
  canonicalEvent: string,
  nativeMatcher: string | undefined,
  item: ClaudeCodeHookItem,
): HookDefinition {
  const canonicalMatcher = nativeMatcher
    ? parseMatcher(nativeMatcher, "claude-code")
    : undefined;

  // Separate known handler fields from extras (potential provider_data).
  const { type, command, timeout, ...extras } = item;

  const hook: HookDefinition = {
    event: canonicalEvent,
    handler: { type },
    blocking: isBlockingEvent(canonicalEvent),
  };

  if (canonicalMatcher !== undefined) {
    hook.matcher = canonicalMatcher;
  }

  if (command !== undefined) {
    hook.handler.command = command;
  }

  if (timeout !== undefined) {
    hook.handler.timeout = timeout;
  }

  // Preserve unrecognized fields in provider_data["claude-code"] (§3.6, §7.1).
  if (Object.keys(extras).length > 0) {
    hook.provider_data = { "claude-code": extras as Record<string, unknown> };
  }

  return hook;
}

/**
 * Infer whether a hook is blocking based on its canonical event.
 *
 * Per the blocking matrix, `before_tool_execute` and `before_prompt` are the
 * primary blocking events. All other events are observational (non-blocking)
 * by default. Callers may override after decode if the source format carries
 * explicit blocking state.
 */
function isBlockingEvent(canonicalEvent: string): boolean {
  return canonicalEvent === "before_tool_execute" || canonicalEvent === "before_prompt";
}
