/**
 * matchers.ts — Matcher parsing and tool vocabulary
 * Reference implementation: Extended conformance (§8.2)
 *
 * Covers:
 *   §6.1  Bare String — canonical tool vocabulary lookup
 *   §6.2  Pattern Object — RE2 regex against provider-native tool name
 *   §6.3  MCP Object — structured MCP server/tool selector + combined formats
 *   §6.4  Array (OR) — OR of any matcher types
 *   §6.5  Omitted — wildcard (matches all tools)
 *   tools.md §1  Canonical Tool Names table (all 9 entries)
 *   tools.md §2  MCP combined format encoding rules
 */

import type { MatcherObject, McpMatcher, PatternMatcher } from "./manifest.js";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

/** Canonical provider slugs (§3.6). */
export type ProviderSlug =
  | "claude-code"
  | "gemini-cli"
  | "cursor"
  | "windsurf"
  | "copilot-cli"
  | "kiro"
  | "opencode";

/**
 * A canonical matcher as it appears in a HookDefinition.
 * Re-exported here so callers can import from a single module.
 */
export type CanonicalMatcher = string | MatcherObject | Array<string | MatcherObject>;

/**
 * Result of resolving a single canonical matcher element to a provider-native
 * string.
 *
 * - `string`: the resolved native tool name (or pattern, or MCP combined string)
 * - `null`: the provider has no equivalent for this canonical name (tool absent
 *   on the provider, or the tool maps to an event split rather than a matcher)
 */
export type ResolvedMatcher = string | null;

// ---------------------------------------------------------------------------
// §1 Tool Vocabulary (tools.md §1)
// ---------------------------------------------------------------------------

/**
 * Complete mapping of canonical tool names to provider-native names.
 *
 * Keys are the 9 canonical names defined in tools.md §1. Values are records
 * mapping provider slugs to their native tool name string.
 *
 * Providers with no equivalent tool (`--` in the spec table) are omitted from
 * the inner record — `resolveMatcher` returns `null` for those combinations.
 *
 * For split-event providers (cursor, windsurf) where a canonical tool maps to
 * a native event rather than a tool-name matcher, the entry is also omitted
 * because no matcher string exists to encode. The conversion pipeline handles
 * event-splitting separately during the encode stage (§7.3).
 */
export const TOOL_VOCABULARY: Record<string, Partial<Record<ProviderSlug, string>>> = {
  shell: {
    "claude-code": "Bash",
    "gemini-cli": "run_shell_command",
    // cursor:  maps to event pre_run_terminal_cmd — no tool-name matcher
    // windsurf: maps to event pre_run_command — no tool-name matcher
    "copilot-cli": "bash",
    kiro: "execute_bash",
    opencode: "bash",
  },
  file_read: {
    "claude-code": "Read",
    "gemini-cli": "read_file",
    cursor: "read_file",
    // windsurf: maps to event pre_read_code — no tool-name matcher
    "copilot-cli": "view",
    kiro: "fs_read",
    opencode: "read",
  },
  file_write: {
    "claude-code": "Write",
    "gemini-cli": "write_file",
    cursor: "edit_file",
    // windsurf: maps to event pre_write_code — no tool-name matcher
    "copilot-cli": "create",
    kiro: "fs_write",
    opencode: "write",
  },
  file_edit: {
    "claude-code": "Edit",
    "gemini-cli": "replace",
    cursor: "edit_file",
    // windsurf: maps to event pre_write_code — no tool-name matcher
    "copilot-cli": "edit",
    kiro: "fs_write",
    opencode: "edit",
  },
  search: {
    "claude-code": "Grep",
    "gemini-cli": "grep_search",
    cursor: "grep_search",
    // windsurf: no equivalent
    "copilot-cli": "grep",
    kiro: "grep",
    opencode: "grep",
  },
  find: {
    "claude-code": "Glob",
    "gemini-cli": "glob",
    cursor: "file_search",
    // windsurf: no equivalent
    "copilot-cli": "glob",
    kiro: "glob",
    opencode: "glob",
  },
  web_search: {
    "claude-code": "WebSearch",
    "gemini-cli": "google_web_search",
    cursor: "web_search",
    // windsurf:    no equivalent
    // copilot-cli: no equivalent
    kiro: "web_search",
    // opencode: no equivalent
  },
  web_fetch: {
    "claude-code": "WebFetch",
    "gemini-cli": "web_fetch",
    // cursor:   no equivalent
    // windsurf: no equivalent
    "copilot-cli": "web_fetch",
    kiro: "web_fetch",
    // opencode: no equivalent
  },
  agent: {
    "claude-code": "Agent",
    // gemini-cli: no equivalent
    // cursor:     no equivalent
    // windsurf:   no equivalent
    "copilot-cli": "task",
    kiro: "use_subagent",
    // opencode: no equivalent
  },
};

// ---------------------------------------------------------------------------
// §6.3 MCP combined format encoders (tools.md §2)
// ---------------------------------------------------------------------------

/**
 * Encode a canonical MCP matcher `{"mcp": {"server": ..., "tool": ...}}` into
 * the provider-specific combined string format.
 *
 * Returns `null` when the provider has no defined MCP combined format.
 *
 * Combined format rules (§6.3 / tools.md §2):
 *   claude-code, kiro  →  mcp__<server>__<tool>
 *   gemini-cli         →  mcp_<server>_<tool>
 *   copilot-cli        →  <server>/<tool>
 *   cursor, windsurf   →  <server>__<tool>
 *
 * When `tool` is omitted the returned string is the server-only prefix that
 * the provider uses to match all tools on that server.
 */
export function encodeMcpMatcher(
  server: string,
  tool: string | undefined,
  provider: ProviderSlug,
): string | null {
  switch (provider) {
    case "claude-code":
    case "kiro":
      return tool !== undefined ? `mcp__${server}__${tool}` : `mcp__${server}__`;

    case "gemini-cli":
      return tool !== undefined ? `mcp_${server}_${tool}` : `mcp_${server}_`;

    case "copilot-cli":
      return tool !== undefined ? `${server}/${tool}` : `${server}/`;

    case "cursor":
    case "windsurf":
      return tool !== undefined ? `${server}__${tool}` : `${server}__`;

    case "opencode":
      // opencode has no defined MCP combined format in the spec
      return null;
  }
}

/**
 * Parse a provider-native string back into a canonical McpMatcher when the
 * string matches the provider's MCP combined format.
 *
 * Returns `null` when the string does not match the provider's MCP format.
 */
export function parseMcpString(
  native: string,
  provider: ProviderSlug,
): McpMatcher | null {
  switch (provider) {
    case "claude-code":
    case "kiro": {
      // Format: mcp__<server>__<tool>
      if (!native.startsWith("mcp__")) return null;
      const rest = native.slice("mcp__".length);
      const sep = rest.indexOf("__");
      if (sep === -1) return null;
      const server = rest.slice(0, sep);
      const tool = rest.slice(sep + 2) || undefined;
      if (!server) return null;
      return { mcp: tool ? { server, tool } : { server } };
    }

    case "gemini-cli": {
      // Format: mcp_<server>_<tool>  (single underscore — must not start with mcp__)
      if (!native.startsWith("mcp_") || native.startsWith("mcp__")) return null;
      const rest = native.slice("mcp_".length);
      const sep = rest.indexOf("_");
      if (sep === -1) return null;
      const server = rest.slice(0, sep);
      const tool = rest.slice(sep + 1) || undefined;
      if (!server) return null;
      return { mcp: tool ? { server, tool } : { server } };
    }

    case "copilot-cli": {
      // Format: <server>/<tool>
      const sep = native.indexOf("/");
      if (sep === -1) return null;
      const server = native.slice(0, sep);
      const tool = native.slice(sep + 1) || undefined;
      if (!server) return null;
      return { mcp: tool ? { server, tool } : { server } };
    }

    case "cursor":
    case "windsurf": {
      // Format: <server>__<tool>  (must not be an mcp__ prefixed string)
      if (native.startsWith("mcp__")) return null;
      const sep = native.indexOf("__");
      if (sep === -1) return null;
      const server = native.slice(0, sep);
      const tool = native.slice(sep + 2) || undefined;
      if (!server) return null;
      return { mcp: tool ? { server, tool } : { server } };
    }

    case "opencode":
      return null;
  }
}

// ---------------------------------------------------------------------------
// §6 Matcher resolution and parsing
// ---------------------------------------------------------------------------

/**
 * Resolve a single canonical matcher element to the provider-native string.
 *
 * Handles all element types:
 *   - bare string     → tool vocabulary lookup (§6.1); unknown names pass through
 *   - PatternMatcher  → regex value passed through unchanged (§6.2)
 *   - McpMatcher      → encoded to provider combined format (§6.3)
 *
 * Returns `null` when:
 *   - A known canonical name has no equivalent on this provider
 *   - A McpMatcher cannot be encoded for this provider
 */
export function resolveMatcherElement(
  matcher: string | MatcherObject,
  provider: ProviderSlug,
): ResolvedMatcher {
  if (typeof matcher === "string") {
    // §6.1 — bare string: canonical tool vocabulary lookup
    if (matcher in TOOL_VOCABULARY) {
      return TOOL_VOCABULARY[matcher]![provider] ?? null;
    }
    // Unknown canonical name — pass through as literal (forward compatibility §6.1)
    return matcher;
  }

  if (isMcpMatcher(matcher)) {
    // §6.3 — MCP object: encode to provider combined format
    return encodeMcpMatcher(matcher.mcp.server, matcher.mcp.tool, provider);
  }

  if (isPatternMatcher(matcher)) {
    // §6.2 — pattern object: pass through the regex value unchanged
    return matcher.pattern;
  }

  return null;
}

/**
 * Resolve a canonical `CanonicalMatcher` to an array of provider-native strings.
 *
 * Handles:
 *   - bare string   → single-element array
 *   - MatcherObject → single-element array
 *   - Array (§6.4)  → one resolved entry per element
 *
 * Elements that resolve to `null` (no provider equivalent) are excluded.
 * An empty result means no matcher is applicable on this provider.
 */
export function resolveMatcher(
  matcher: CanonicalMatcher,
  provider: ProviderSlug,
): string[] {
  const elements: Array<string | MatcherObject> = Array.isArray(matcher)
    ? matcher
    : [matcher];

  const results: string[] = [];
  for (const el of elements) {
    const resolved = resolveMatcherElement(el, provider);
    if (resolved !== null) {
      results.push(resolved);
    }
  }
  return results;
}

/**
 * Parse a provider-native matcher string back into a canonical matcher.
 *
 * Attempts (in order):
 *   1. MCP combined format (§6.3) → McpMatcher
 *   2. Reverse vocabulary lookup  → canonical bare string (§6.1)
 *   3. Fall back to PatternMatcher wrapping the native string (§6.2 escape hatch)
 *
 * Returns a single `string | MatcherObject`. Callers collecting multiple
 * native matchers should wrap results in an array for §6.4 (OR) semantics.
 */
export function parseMatcher(
  nativeMatcher: string,
  provider: ProviderSlug,
): string | MatcherObject {
  // 1. Try MCP combined format
  const mcpResult = parseMcpString(nativeMatcher, provider);
  if (mcpResult !== null) {
    return mcpResult;
  }

  // 2. Reverse vocabulary lookup: find the canonical name whose native form
  //    matches nativeMatcher for this provider.
  for (const [canonicalName, nativeMap] of Object.entries(TOOL_VOCABULARY)) {
    if (nativeMap[provider] === nativeMatcher) {
      return canonicalName;
    }
  }

  // 3. Not in vocabulary and not MCP — preserve as a pattern matcher (§6.2)
  //    so the string is not lost during a round-trip.
  const patternMatcher: PatternMatcher = { pattern: nativeMatcher };
  return patternMatcher;
}

// ---------------------------------------------------------------------------
// Type guards
// ---------------------------------------------------------------------------

/** Returns true when `m` is a PatternMatcher (has a `pattern` string field). */
export function isPatternMatcher(m: unknown): m is PatternMatcher {
  return (
    typeof m === "object" &&
    m !== null &&
    "pattern" in m &&
    typeof (m as PatternMatcher).pattern === "string"
  );
}

/** Returns true when `m` is a McpMatcher (has an `mcp` object field). */
export function isMcpMatcher(m: unknown): m is McpMatcher {
  return (
    typeof m === "object" &&
    m !== null &&
    "mcp" in m &&
    typeof (m as McpMatcher).mcp === "object"
  );
}
