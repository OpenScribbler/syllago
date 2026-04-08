/**
 * conformance.test.ts — Hook Interchange Format conformance suite
 *
 * Loads shared test vectors from docs/spec/hooks/test-vectors/ and exercises:
 *   - Manifest parsing and validation (§3.1–§3.5)
 *   - Exit code resolution truth table (§4, §5.3)
 *   - Claude Code encode/decode (§7)
 *   - Gemini CLI encode/decode (§7)
 */

import * as fs from "fs";
import * as path from "path";
import { fileURLToPath } from "url";
import { describe, it, expect } from "vitest";

import { parseManifest, validateManifest } from "../src/manifest.js";
import { resolve } from "../src/exitCodes.js";
import * as cc from "../src/claudeCode.js";
import * as gc from "../src/geminiCli.js";
import type { HookManifest } from "../src/manifest.js";
import type { ClaudeCodeConfig } from "../src/claudeCode.js";
import type { GeminiCliConfig } from "../src/geminiCli.js";

// ---------------------------------------------------------------------------
// Path helpers
// ---------------------------------------------------------------------------

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

/** Resolve a path relative to the shared test-vectors directory. */
function tv(...segments: string[]): string {
  return path.resolve(__dirname, "..", "..", "..", "test-vectors", ...segments);
}

/** Load and parse a JSON test vector, stripping annotation-only fields. */
function loadJson(filePath: string): unknown {
  const raw = JSON.parse(fs.readFileSync(filePath, "utf8"));
  return stripAnnotations(raw);
}

/**
 * Recursively strip `_comment` and `_warnings` fields from a parsed JSON value.
 * These fields are documentation annotations — not part of the data contract.
 */
function stripAnnotations(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(stripAnnotations);
  }
  if (typeof value === "object" && value !== null) {
    const out: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
      if (k !== "_comment" && k !== "_warnings") {
        out[k] = stripAnnotations(v);
      }
    }
    return out;
  }
  return value;
}

// ---------------------------------------------------------------------------
// 1. Manifest Parsing
// ---------------------------------------------------------------------------

describe("Manifest Parsing", () => {
  it("parses canonical/simple-blocking.json as valid JSON", () => {
    const raw = fs.readFileSync(tv("canonical", "simple-blocking.json"), "utf8");
    const manifest = parseManifest(raw, "json");
    expect(manifest.spec).toBe("hooks/0.1");
    expect(manifest.hooks).toHaveLength(1);
  });

  it("parses canonical/full-featured.json and finds 3 hooks", () => {
    const raw = fs.readFileSync(tv("canonical", "full-featured.json"), "utf8");
    const manifest = parseManifest(raw, "json");
    expect(manifest.spec).toBe("hooks/0.1");
    expect(manifest.hooks).toHaveLength(3);
  });

  it("parses canonical/multi-event.json and finds 4 hooks", () => {
    const raw = fs.readFileSync(tv("canonical", "multi-event.json"), "utf8");
    const manifest = parseManifest(raw, "json");
    expect(manifest.spec).toBe("hooks/0.1");
    expect(manifest.hooks).toHaveLength(4);
  });

  it("auto-detects JSON when format is omitted", () => {
    const raw = fs.readFileSync(tv("canonical", "simple-blocking.json"), "utf8");
    const manifest = parseManifest(raw);
    expect(manifest.spec).toBe("hooks/0.1");
  });

  it("simple-blocking first hook has correct event, matcher, and blocking flag", () => {
    const raw = fs.readFileSync(tv("canonical", "simple-blocking.json"), "utf8");
    const manifest = parseManifest(raw, "json");
    const hook = manifest.hooks[0]!;
    expect(hook.event).toBe("before_tool_execute");
    expect(hook.matcher).toBe("shell");
    expect(hook.blocking).toBe(true);
    expect(hook.handler.type).toBe("command");
    expect(hook.handler.command).toBe("./safety-check.sh");
    expect(hook.handler.timeout).toBe(10);
  });

  it("full-featured has MCP matcher on second hook", () => {
    const raw = fs.readFileSync(tv("canonical", "full-featured.json"), "utf8");
    const manifest = parseManifest(raw, "json");
    const hook = manifest.hooks[1]!;
    expect(hook.matcher).toEqual({ mcp: { server: "github", tool: "create_issue" } });
  });

  it("full-featured has session_start non-blocking hook", () => {
    const raw = fs.readFileSync(tv("canonical", "full-featured.json"), "utf8");
    const manifest = parseManifest(raw, "json");
    const hook = manifest.hooks[2]!;
    expect(hook.event).toBe("session_start");
    expect(hook.blocking).toBe(false);
  });

  it("multi-event has array matcher on first hook", () => {
    const raw = fs.readFileSync(tv("canonical", "multi-event.json"), "utf8");
    const manifest = parseManifest(raw, "json");
    const hook = manifest.hooks[0]!;
    expect(hook.matcher).toEqual(["shell", "file_write"]);
  });
});

// ---------------------------------------------------------------------------
// 2. Invalid Manifests
// ---------------------------------------------------------------------------

describe("Invalid Manifests", () => {
  it("empty-hooks-array.json produces a validation error", () => {
    const raw = loadJson(tv("invalid", "empty-hooks-array.json"));
    const errors = validateManifest(raw);
    expect(errors.length).toBeGreaterThan(0);
    expect(errors.some((e) => e.includes("empty"))).toBe(true);
  });

  it("missing-spec.json produces a validation error mentioning 'spec'", () => {
    const raw = loadJson(tv("invalid", "missing-spec.json"));
    const errors = validateManifest(raw);
    expect(errors.length).toBeGreaterThan(0);
    expect(errors.some((e) => e.toLowerCase().includes("spec"))).toBe(true);
  });

  it("missing-hooks.json produces a validation error mentioning 'hooks'", () => {
    const raw = loadJson(tv("invalid", "missing-hooks.json"));
    const errors = validateManifest(raw);
    expect(errors.length).toBeGreaterThan(0);
    expect(errors.some((e) => e.toLowerCase().includes("hooks"))).toBe(true);
  });

  it("missing-event.json produces a validation error mentioning 'event'", () => {
    const raw = loadJson(tv("invalid", "missing-event.json"));
    const errors = validateManifest(raw);
    expect(errors.length).toBeGreaterThan(0);
    expect(errors.some((e) => e.toLowerCase().includes("event"))).toBe(true);
  });

  it("missing-handler.json produces a validation error mentioning 'handler'", () => {
    const raw = loadJson(tv("invalid", "missing-handler.json"));
    const errors = validateManifest(raw);
    expect(errors.length).toBeGreaterThan(0);
    expect(errors.some((e) => e.toLowerCase().includes("handler"))).toBe(true);
  });

  it("a valid canonical manifest produces zero validation errors", () => {
    const raw = loadJson(tv("canonical", "simple-blocking.json"));
    const errors = validateManifest(raw);
    expect(errors).toHaveLength(0);
  });

  it("full-featured canonical manifest produces zero validation errors", () => {
    const raw = loadJson(tv("canonical", "full-featured.json"));
    const errors = validateManifest(raw);
    expect(errors).toHaveLength(0);
  });
});

// ---------------------------------------------------------------------------
// 3. Exit Code Resolution (§5.3 truth table)
// ---------------------------------------------------------------------------

describe("Exit Code Resolution", () => {
  // Non-blocking hooks (blocking = false)
  it("non-blocking, exit 0, no decision → allow", () => {
    expect(resolve(false, 0, null)).toBe("allow");
  });

  it("non-blocking, exit 0, decision allow → allow", () => {
    expect(resolve(false, 0, "allow")).toBe("allow");
  });

  it("non-blocking, exit 0, decision deny → block", () => {
    expect(resolve(false, 0, "deny")).toBe("block");
  });

  it("non-blocking, exit 0, decision ask → ask", () => {
    expect(resolve(false, 0, "ask")).toBe("ask");
  });

  it("non-blocking, exit 1, no decision → warn_allow", () => {
    expect(resolve(false, 1, null)).toBe("warn_allow");
  });

  it("non-blocking, exit 1, decision deny → warn_allow (exit 1 overrides decision)", () => {
    expect(resolve(false, 1, "deny")).toBe("warn_allow");
  });

  it("non-blocking, exit 2 → warn_allow (downgraded to 1 for non-blocking)", () => {
    expect(resolve(false, 2, null)).toBe("warn_allow");
  });

  // Blocking hooks (blocking = true)
  it("blocking, exit 0, no decision → allow", () => {
    expect(resolve(true, 0, null)).toBe("allow");
  });

  it("blocking, exit 0, decision deny → block", () => {
    expect(resolve(true, 0, "deny")).toBe("block");
  });

  it("blocking, exit 0, decision ask → ask", () => {
    expect(resolve(true, 0, "ask")).toBe("ask");
  });

  it("blocking, exit 1, no decision → warn_allow", () => {
    expect(resolve(true, 1, null)).toBe("warn_allow");
  });

  it("blocking, exit 2, no decision → block", () => {
    expect(resolve(true, 2, null)).toBe("block");
  });

  it("blocking, exit 2, decision allow → block (exit 2 overrides decision)", () => {
    expect(resolve(true, 2, "allow")).toBe("block");
  });

  // Normalization: out-of-range exit codes are treated as 1
  it("non-blocking, exit 3, no decision → warn_allow (code 3 normalizes to 1)", () => {
    expect(resolve(false, 3, null)).toBe("warn_allow");
  });

  it("blocking, exit 42, no decision → warn_allow (out-of-range normalizes to 1)", () => {
    expect(resolve(true, 42, null)).toBe("warn_allow");
  });
});

// ---------------------------------------------------------------------------
// 4. Claude Code Vectors
// ---------------------------------------------------------------------------

describe("Claude Code Vectors", () => {
  it("encode canonical/simple-blocking.json matches claude-code/simple-blocking.json", () => {
    const canonical = loadJson(tv("canonical", "simple-blocking.json")) as HookManifest;
    const expected = loadJson(tv("claude-code", "simple-blocking.json")) as ClaudeCodeConfig;
    const actual = cc.encode(canonical);
    expect(actual).toEqual(expected);
  });

  it("encode canonical/full-featured.json matches claude-code/full-featured.json", () => {
    const canonical = loadJson(tv("canonical", "full-featured.json")) as HookManifest;
    const expected = loadJson(tv("claude-code", "full-featured.json")) as ClaudeCodeConfig;
    const actual = cc.encode(canonical);
    expect(actual).toEqual(expected);
  });

  it("encode canonical/multi-event.json matches claude-code/multi-event.json", () => {
    const canonical = loadJson(tv("canonical", "multi-event.json")) as HookManifest;
    const expected = loadJson(tv("claude-code", "multi-event.json")) as ClaudeCodeConfig;
    const actual = cc.encode(canonical);
    expect(actual).toEqual(expected);
  });

  it("decode roundtrip-source.json produces roundtrip-canonical.json", () => {
    const source = loadJson(tv("claude-code", "roundtrip-source.json")) as ClaudeCodeConfig;
    const expectedCanonical = loadJson(tv("claude-code", "roundtrip-canonical.json")) as HookManifest;
    const decoded = cc.decode(source);
    expect(decoded).toEqual(expectedCanonical);
  });

  it("decode then re-encode roundtrip-source.json is structurally equivalent to source", () => {
    const source = loadJson(tv("claude-code", "roundtrip-source.json")) as ClaudeCodeConfig;
    const decoded = cc.decode(source);
    const reEncoded = cc.encode(decoded);
    // Re-encode should reconstruct the PreToolUse+Bash and SessionStart entries.
    expect(reEncoded.hooks["PreToolUse"]).toBeDefined();
    expect(reEncoded.hooks["SessionStart"]).toBeDefined();
    expect(reEncoded.hooks["PreToolUse"]![0]!.matcher).toBe("Bash");
    expect(reEncoded.hooks["PreToolUse"]![0]!.hooks[0]!.command).toBe("./audit-shell.sh");
  });

  it("EVENT_MAP contains all core canonical events", () => {
    const coreEvents = [
      "before_tool_execute",
      "after_tool_execute",
      "session_start",
      "session_end",
      "before_prompt",
      "agent_stop",
    ];
    for (const event of coreEvents) {
      expect(cc.EVENT_MAP.has(event)).toBe(true);
    }
  });
});

// ---------------------------------------------------------------------------
// 5. Gemini CLI Vectors
// ---------------------------------------------------------------------------

describe("Gemini CLI Vectors", () => {
  it("encode canonical/simple-blocking.json matches gemini-cli/simple-blocking.json", () => {
    const canonical = loadJson(tv("canonical", "simple-blocking.json")) as HookManifest;
    const expected = loadJson(tv("gemini-cli", "simple-blocking.json")) as GeminiCliConfig;
    const actual = gc.encode(canonical);
    expect(actual).toEqual(expected);
  });

  it("encode canonical/full-featured.json matches gemini-cli/full-featured.json", () => {
    const canonical = loadJson(tv("canonical", "full-featured.json")) as HookManifest;
    const expected = loadJson(tv("gemini-cli", "full-featured.json")) as GeminiCliConfig;
    const actual = gc.encode(canonical);
    expect(actual).toEqual(expected);
  });

  it("encode canonical/multi-event.json matches gemini-cli/multi-event.json", () => {
    const canonical = loadJson(tv("canonical", "multi-event.json")) as HookManifest;
    const expected = loadJson(tv("gemini-cli", "multi-event.json")) as GeminiCliConfig;
    const actual = gc.encode(canonical);
    expect(actual).toEqual(expected);
  });

  it("timeout is converted from seconds to milliseconds on encode", () => {
    const canonical = loadJson(tv("canonical", "simple-blocking.json")) as HookManifest;
    const encoded = gc.encode(canonical);
    // canonical has timeout: 10 (seconds) → Gemini expects 10000 ms
    expect(encoded.hooks[0]!.timeoutMs).toBe(10000);
  });

  it("array matcher expands to one entry per tool in Gemini CLI output", () => {
    const canonical = loadJson(tv("canonical", "multi-event.json")) as HookManifest;
    const encoded = gc.encode(canonical);
    // multi-event hook[0] has matcher: ["shell", "file_write"] → two BeforeTool entries
    const beforeToolEntries = encoded.hooks.filter((h) => h.trigger === "BeforeTool");
    expect(beforeToolEntries.length).toBeGreaterThanOrEqual(2);
  });

  it("MCP matcher encodes with single-underscore format for Gemini CLI", () => {
    const canonical = loadJson(tv("canonical", "full-featured.json")) as HookManifest;
    const encoded = gc.encode(canonical);
    const mcpEntry = encoded.hooks.find((h) => h.toolMatcher?.startsWith("mcp_github"));
    expect(mcpEntry).toBeDefined();
    expect(mcpEntry!.toolMatcher).toBe("mcp_github_create_issue");
  });

  it("non-tool events omit toolMatcher field", () => {
    const canonical = loadJson(tv("canonical", "full-featured.json")) as HookManifest;
    const encoded = gc.encode(canonical);
    const sessionEntry = encoded.hooks.find((h) => h.trigger === "SessionStart");
    expect(sessionEntry).toBeDefined();
    expect(sessionEntry!.toolMatcher).toBeUndefined();
  });
});
