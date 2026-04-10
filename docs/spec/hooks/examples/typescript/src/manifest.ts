/**
 * manifest.ts — Hook Interchange Format Specification
 * Reference implementation: Core conformance (§13.1)
 *
 * Covers:
 *   §3.1  File Format  — JSON and YAML accepted
 *   §3.2  Forward Compatibility  — unknown fields ignored at every level
 *   §3.3  Top-Level Structure  — `spec` and `hooks` required fields
 *   §3.4  Hook Definition  — per-hook required/optional fields
 *   §3.5  Handler Definition  — handler required/optional fields
 */

import yaml from "js-yaml";

// ---------------------------------------------------------------------------
// Type definitions
// ---------------------------------------------------------------------------

export interface PatternMatcher {
  pattern: string;
}

export interface McpMatcher {
  mcp: {
    server: string;
    tool?: string;
  };
}

export type MatcherObject = PatternMatcher | McpMatcher;

export interface HandlerDefinition {
  /** Handler type. MUST be "command" for shell-based handlers (§3.5). */
  type: string;
  /** Shell command or script path. REQUIRED when type is "command" (§3.5). */
  command?: string;
  /** Per-OS command overrides. Keys: "windows" | "linux" | "darwin" (§3.5). */
  platform?: Record<string, string>;
  /** Working directory for the hook process, relative to project root (§3.5). */
  cwd?: string;
  /** Environment variables passed to the hook process (§3.5). */
  env?: Record<string, string>;
  /** Maximum execution time in seconds. 0 = no timeout (§3.5). */
  timeout?: number;
  /** Behavior on timeout: "warn" | "block". Default: "warn" (§3.5). */
  timeout_action?: "warn" | "block";
  /** Fire-and-forget execution. Default: false (§3.5). */
  async?: boolean;
  /** Human-readable status text shown while the hook runs (§3.5). */
  status_message?: string;
}

export interface HookDefinition {
  /** Human-readable identifier. When omitted, refer by position (§3.4). */
  name?: string;
  /** Canonical event name from the Event Registry (§3.4, §7). */
  event: string;
  /**
   * Tool matcher expression (§3.4, §6).
   * Bare string: canonical tool vocabulary lookup.
   * PatternMatcher: RE2 regex against provider-native tool name.
   * McpMatcher: structured MCP server/tool selector.
   * Array: OR of any of the above.
   * Omitted: matches all tools.
   */
  matcher?: string | MatcherObject | Array<string | MatcherObject>;
  /** Handler definition (§3.4, §3.5). */
  handler: HandlerDefinition;
  /** Whether this hook can prevent the triggering action. Default: false (§3.4). */
  blocking?: boolean;
  /** Per-capability fallback strategies (§3.4, §11). */
  degradation?: Record<string, string>;
  /** Opaque provider-specific data, keyed by provider slug (§3.4, §3.6). */
  provider_data?: Record<string, Record<string, unknown>>;
  /**
   * Informational capability identifiers (§3.4, §9).
   * Implementations SHOULD infer capabilities from manifest fields rather
   * than relying on this array.
   */
  capabilities?: string[];
}

export interface HookManifest {
  /** Specification version identifier. MUST be "hooks/0.1" for this version (§3.3). */
  spec: string;
  /** Non-empty array of hook definition objects (§3.3). */
  hooks: HookDefinition[];
}

// ---------------------------------------------------------------------------
// Parsing
// ---------------------------------------------------------------------------

/**
 * Parse a JSON or YAML string into a HookManifest.
 *
 * Per §3.1: conforming implementations MUST accept both JSON and YAML and
 * MUST produce identical canonical structures from either.
 *
 * When `format` is omitted the function attempts JSON first; if that fails
 * it falls back to YAML. This covers the common case where the caller does
 * not know the serialisation format ahead of time.
 *
 * @throws {Error} when the string cannot be parsed as the requested format.
 */
export function parseManifest(
  data: string,
  format?: "json" | "yaml",
): HookManifest {
  let parsed: unknown;

  if (format === "json") {
    parsed = JSON.parse(data);
  } else if (format === "yaml") {
    parsed = yaml.load(data);
  } else {
    // Auto-detect: try JSON first, then YAML.
    try {
      parsed = JSON.parse(data);
    } catch {
      parsed = yaml.load(data);
    }
  }

  return parsed as HookManifest;
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

/**
 * Validate a parsed (but untyped) manifest value against the Hook Interchange
 * Format Specification.
 *
 * Returns an array of human-readable error strings. An empty array means the
 * manifest is valid.
 *
 * Conformance notes:
 *   §3.2  Unknown fields at any level are silently ignored (forward compat).
 *   §3.3  `spec` MUST exist and equal "hooks/0.1".
 *   §3.3  `hooks` MUST exist, be an array, and be non-empty.
 *   §3.4  Each hook MUST have `event` (string) and `handler` (object).
 *   §3.5  Each handler MUST have `type` (string).
 */
export function validateManifest(manifest: unknown): string[] {
  const errors: string[] = [];

  if (typeof manifest !== "object" || manifest === null || Array.isArray(manifest)) {
    errors.push("Manifest must be a non-null object.");
    return errors;
  }

  const m = manifest as Record<string, unknown>;

  // §3.3 — `spec` field
  if (!("spec" in m)) {
    errors.push('Missing required field: "spec".');
  } else if (m["spec"] !== "hooks/0.1") {
    errors.push(
      `Invalid "spec" value: expected "hooks/0.1", got ${JSON.stringify(m["spec"])}.`,
    );
  }

  // §3.3 — `hooks` field
  if (!("hooks" in m)) {
    errors.push('Missing required field: "hooks".');
  } else if (!Array.isArray(m["hooks"])) {
    errors.push('"hooks" must be an array.');
  } else if ((m["hooks"] as unknown[]).length === 0) {
    errors.push('"hooks" array must not be empty (§3.3).');
  } else {
    // §3.4 — per-hook validation
    const hooks = m["hooks"] as unknown[];
    for (let i = 0; i < hooks.length; i++) {
      const hookErrors = validateHookDefinition(hooks[i], i);
      errors.push(...hookErrors);
    }
  }

  return errors;
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

/** Validate a single hook definition at position `index` in the hooks array. */
function validateHookDefinition(hook: unknown, index: number): string[] {
  const errors: string[] = [];
  const label = `Hook[${index}]`;

  if (typeof hook !== "object" || hook === null || Array.isArray(hook)) {
    errors.push(`${label}: each hook must be a non-null object.`);
    return errors;
  }

  const h = hook as Record<string, unknown>;

  // §3.4 — `event` is REQUIRED and must be a string
  if (!("event" in h)) {
    errors.push(`${label}: missing required field "event".`);
  } else if (typeof h["event"] !== "string") {
    errors.push(`${label}: "event" must be a string.`);
  }

  // §3.4 — `handler` is REQUIRED and must be an object
  if (!("handler" in h)) {
    errors.push(`${label}: missing required field "handler".`);
  } else {
    const handlerErrors = validateHandlerDefinition(h["handler"], label);
    errors.push(...handlerErrors);
  }

  // All other fields (name, matcher, blocking, degradation, provider_data,
  // capabilities) are OPTIONAL — unknown fields are also ignored per §3.2.

  return errors;
}

/** Validate a handler definition belonging to the hook identified by `hookLabel`. */
function validateHandlerDefinition(handler: unknown, hookLabel: string): string[] {
  const errors: string[] = [];
  const label = `${hookLabel}.handler`;

  if (typeof handler !== "object" || handler === null || Array.isArray(handler)) {
    errors.push(`${label}: "handler" must be a non-null object.`);
    return errors;
  }

  const hd = handler as Record<string, unknown>;

  // §3.5 — `type` is REQUIRED and must be a string
  if (!("type" in hd)) {
    errors.push(`${label}: missing required field "type".`);
  } else if (typeof hd["type"] !== "string") {
    errors.push(`${label}: "type" must be a string.`);
  }

  // All other handler fields are OPTIONAL per §3.5.
  // Unknown fields are silently ignored per §3.2.

  return errors;
}
