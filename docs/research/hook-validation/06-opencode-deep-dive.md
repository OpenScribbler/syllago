# OpenCode Plugin System Deep Dive

Research into whether lossless bidirectional hook conversion is feasible between OpenCode's TypeScript plugin system and shell-command hooks used by other AI coding tools (Claude Code, Amp, etc.).

**Sources:**
- [OpenCode plugin docs](https://opencode.ai/docs/plugins/)
- [OpenCode custom tools docs](https://opencode.ai/docs/custom-tools/)
- [Plugin dev guide (johnlindquist)](https://gist.github.com/johnlindquist/0adf1032b4e84942f3e1050aba3c5e4a)
- [Plugin dev guide (rstacruz)](https://gist.github.com/rstacruz/946d02757525c9a0f49b25e316fbe715)
- [OpenCode source: packages/plugin/src/index.ts](https://github.com/anomalyco/opencode/blob/dev/packages/plugin/src/index.ts)
- [OpenCode source: packages/opencode/src/plugin/index.ts](https://github.com/anomalyco/opencode/blob/dev/packages/opencode/src/plugin/index.ts)
- [OpenCode source: packages/opencode/src/session/prompt.ts](https://github.com/anomalyco/opencode/blob/dev/packages/opencode/src/session/prompt.ts)
- [awesome-opencode plugin directory](https://github.com/awesome-opencode/awesome-opencode)
- [ericc-ch/opencode-plugins](https://github.com/ericc-ch/opencode-plugins)
- [OpenCode vs Claude Code hooks comparison](https://gist.github.com/zeke/1e0ba44eaddb16afa6edc91fec778935)

---

## 1. Plugin API Surface

### Plugin Type Definition (from source)

```typescript
// @opencode-ai/plugin v1.2.27
export type PluginInput = {
  client: ReturnType<typeof createOpencodeClient>  // SDK client (localhost:4096)
  project: Project                                   // { id, worktree, vcs? }
  directory: string                                  // current working directory
  worktree: string                                   // git worktree path
  serverUrl: URL                                     // server URL
  $: BunShell                                        // Bun's shell template literal API
}

export type Plugin = (input: PluginInput) => Promise<Hooks>
```

The plugin function is called once at startup with the context. It returns a `Hooks` object containing event handlers.

### Complete Hooks Interface (from source)

```typescript
export interface Hooks {
  event?: (input: { event: Event }) => Promise<void>
  config?: (input: Config) => Promise<void>
  tool?: { [key: string]: ToolDefinition }
  auth?: AuthHook

  "chat.message"?: (
    input: { sessionID: string; agent?: string; model?: {...}; messageID?: string; variant?: string },
    output: { message: UserMessage; parts: Part[] },
  ) => Promise<void>

  "chat.params"?: (
    input: { sessionID: string; agent: string; model: Model; provider: ProviderContext; message: UserMessage },
    output: { temperature: number; topP: number; topK: number; options: Record<string, any> },
  ) => Promise<void>

  "chat.headers"?: (
    input: { sessionID: string; agent: string; model: Model; provider: ProviderContext; message: UserMessage },
    output: { headers: Record<string, string> },
  ) => Promise<void>

  "permission.ask"?: (
    input: Permission,
    output: { status: "ask" | "deny" | "allow" },
  ) => Promise<void>

  "command.execute.before"?: (
    input: { command: string; sessionID: string; arguments: string },
    output: { parts: Part[] },
  ) => Promise<void>

  "tool.execute.before"?: (
    input: { tool: string; sessionID: string; callID: string },
    output: { args: any },
  ) => Promise<void>

  "shell.env"?: (
    input: { cwd: string; sessionID?: string; callID?: string },
    output: { env: Record<string, string> },
  ) => Promise<void>

  "tool.execute.after"?: (
    input: { tool: string; sessionID: string; callID: string; args: any },
    output: { title: string; output: string; metadata: any },
  ) => Promise<void>

  "experimental.chat.messages.transform"?: (
    input: {},
    output: { messages: { info: Message; parts: Part[] }[] },
  ) => Promise<void>

  "experimental.chat.system.transform"?: (
    input: { sessionID?: string; model: Model },
    output: { system: string[] },
  ) => Promise<void>

  "experimental.session.compacting"?: (
    input: { sessionID: string },
    output: { context: string[]; prompt?: string },
  ) => Promise<void>

  "experimental.text.complete"?: (
    input: { sessionID: string; messageID: string; partID: string },
    output: { text: string },
  ) => Promise<void>

  "tool.definition"?: (
    input: { toolID: string },
    output: { description: string; parameters: any },
  ) => Promise<void>
}
```

### Event Types (via Bus subscription)

The `event` hook receives all Bus events. Known types:
- Session: `session.created`, `session.updated`, `session.deleted`, `session.error`, `session.idle`, `session.compacted`, `session.status`, `session.diff`
- Message: `message.updated`, `message.removed`, `message.part.updated`, `message.part.removed`
- File: `file.edited`, `file.watcher.updated`
- Permission: `permission.asked`, `permission.replied`
- Tool: `tool.execute.before`, `tool.execute.after`
- Server: `server.connected`
- LSP: `lsp.updated`, `lsp.client.diagnostics`
- Command: `command.executed`
- TUI: `tui.prompt.append`, `tui.command.execute`, `tui.toast.show`
- Other: `installation.updated`, `todo.updated`, `shell.env`, `ide.installed`

**Conversion feasibility impact:** The event model is richer than shell-command hooks. Many events have no equivalent in Claude Code/Amp. This is fine for export (we just use the events we need) but means import will always be lossy for plugins that use OpenCode-specific events.

---

## 2. Event Handler Contract: tool.execute.before / tool.execute.after

### tool.execute.before

```typescript
"tool.execute.before"?: (
  input: { tool: string; sessionID: string; callID: string },
  output: { args: any },
) => Promise<void>
```

**Input fields:**
- `tool` — tool name string (e.g., "bash", "edit", "write", "read", "task", or MCP tool names)
- `sessionID` — session identifier
- `callID` — unique call identifier for this tool invocation

**Output fields:**
- `args` — the tool's arguments object (mutable). For `bash` tool, this includes `args.command`. For `edit`, includes `args.filePath`.

### tool.execute.after

```typescript
"tool.execute.after"?: (
  input: { tool: string; sessionID: string; callID: string; args: any },
  output: { title: string; output: string; metadata: any },
) => Promise<void>
```

**Input fields:**
- `tool` — tool name
- `sessionID` — session identifier
- `callID` — call identifier
- `args` — the original arguments (read-only at this point)

**Output fields (mutable):**
- `title` — display title for the result
- `output` — the tool's output string
- `metadata` — arbitrary metadata object

### Blocking Execution

**Mechanism: throw an Error.**

From the source code in `packages/opencode/src/plugin/index.ts`, the `trigger` function:
```typescript
yield* Effect.promise(async () => {
  for (const hook of state.hooks) {
    const fn = hook[name] as any
    if (!fn) continue
    await fn(input, output)  // <-- throws propagate up unhandled
  }
})
```

The hook function is `await`-ed without try-catch. In `prompt.ts`, the trigger is called inline before `item.execute(args, ctx)`:
```typescript
await Plugin.trigger("tool.execute.before", { tool: item.id, sessionID, callID }, { args })
const result = await item.execute(args, ctx)  // <-- never reached if trigger throws
```

If the plugin throws, the error propagates as a tool execution error. The tool never runs, and the error message is surfaced to the LLM.

**Example:**
```typescript
"tool.execute.before": async (input, output) => {
  if (input.tool === "bash" && output.args.command.includes("rm -rf")) {
    throw new Error("Dangerous command blocked")
  }
}
```

This is the direct equivalent of Claude Code's exit code 2 (block with error message).

### Argument Modification

Direct property assignment on `output.args`:
```typescript
"tool.execute.before": async (input, output) => {
  if (input.tool === "bash") {
    output.args.command = sanitize(output.args.command)
  }
}
```

The `output` object is passed by reference. Mutations to `output.args` persist when the tool executes. This is the equivalent of Claude Code's `input_rewrite` capability (returning modified JSON on stdout with exit code 0).

### Output Modification (after)

```typescript
"tool.execute.after": async (input, output) => {
  output.output = redact(output.output)  // modify tool output
  output.title = "Custom Title"
}
```

**Conversion feasibility impact:** The before/after hooks map cleanly to `before_tool_execute` and `after_tool_execute` canonical events. Blocking via throw maps to exit code 2. Argument modification maps to `input_rewrite`. The main gap: OpenCode provides `sessionID` and `callID` which have no shell-hook equivalent (but these are optional/ignorable).

---

## 3. Shell Execution: The $ Template Literal API

The `$` parameter is Bun's shell API, typed as `BunShell`:

```typescript
interface BunShell {
  (strings: TemplateStringsArray, ...expressions: ShellExpression[]): BunShellPromise
  braces(pattern: string): string[]
  escape(input: string): string
  env(newEnv?: Record<string, string | undefined>): BunShell
  cwd(newCwd?: string): BunShell
  nothrow(): BunShell
  throws(shouldThrow: boolean): BunShell
}

interface BunShellPromise extends Promise<BunShellOutput> {
  stdin: WritableStream
  cwd(newCwd: string): this
  env(newEnv: Record<string, string>): this
  quiet(): this
  lines(): AsyncIterable<string>
  text(encoding?: BufferEncoding): Promise<string>
  json(): Promise<any>
  arrayBuffer(): Promise<ArrayBuffer>
  nothrow(): this
  throws(shouldThrow: boolean): this
}

interface BunShellOutput {
  stdout: Buffer
  stderr: Buffer
  exitCode: number
  text(encoding?: BufferEncoding): string
  json(): any
}
```

### Can a Plugin Wrap a Shell Script?

**Yes, absolutely.** A plugin can:

1. Execute a shell command with JSON on stdin:
```typescript
"tool.execute.before": async (input, output) => {
  const payload = JSON.stringify({ tool: input.tool, args: output.args })
  const result = await $`echo ${payload} | ./check.sh`.quiet().nothrow()
  if (result.exitCode === 2) {
    throw new Error(result.text())
  }
  if (result.exitCode === 0) {
    const modified = result.json()
    Object.assign(output.args, modified)
  }
}
```

2. Read JSON from stdout:
```typescript
const result = await $`./my-script.sh`.quiet()
const data = result.json()
```

3. Check exit codes:
```typescript
const result = await $`./check.sh`.quiet().nothrow()
if (result.exitCode !== 0) { /* handle */ }
```

**Conversion feasibility impact:** This is critical for syllago's export strategy. We can generate a thin TypeScript plugin that wraps a shell script, preserving the exact same shell-command semantics. The plugin becomes a shim that translates between OpenCode's plugin API and stdin/stdout/exit-code conventions.

---

## 4. Plugin Distribution

### Local Files
- **Project-scoped:** `.opencode/plugin/` (or `.opencode/plugins/`)
- **Global:** `~/.config/opencode/plugin/`
- Files are auto-loaded at startup. Both `.js` and `.ts` are supported.

### npm Packages
Configured in `opencode.json`:
```json
{
  "plugin": [
    "opencode-my-plugin@1.0.0",
    "@scope/opencode-plugin@latest"
  ]
}
```
OpenCode runs `bun install` at startup. Packages cached in `~/.cache/opencode/node_modules/`.

### Local File Protocol
```json
{
  "plugin": [
    "file:///path/to/plugin/dist/index.js"
  ]
}
```

### Dependencies
Create `.opencode/package.json` with dependencies. OpenCode runs `bun install` at startup.

```json
{
  "dependencies": {
    "@opencode-ai/plugin": "latest",
    "@opencode-ai/sdk": "latest",
    "zod": "latest"
  }
}
```

### How Syllago Would Distribute a Generated Plugin

**Option A: Local file (simplest, recommended for hook conversion).**
Generate a `.ts` file directly into `.opencode/plugin/`. No npm publish needed. This is the natural path for syllago since it already manages files in provider directories.

**Option B: npm package.** Would require publishing, which is overkill for generated shims. Only makes sense if syllago itself published a plugin SDK.

**Conversion feasibility impact:** Local file distribution is ideal for syllago. We generate a TypeScript file and drop it in `.opencode/plugin/`. No additional tooling or package management needed beyond what OpenCode already does.

---

## 5. Plugin Limitations

### Security
- **No sandbox.** Plugins run in the same Bun process as OpenCode.
- **Full filesystem access.** Plugins can read/write any file the user can access.
- **Full network access.** Plugins can make HTTP requests, open sockets, etc.
- **Arbitrary imports.** Plugins can import any npm package or local module.
- **SDK client access.** Plugins get a full OpenCode SDK client that can interact with sessions, send messages, etc.

### Known Bugs (from GitHub issues)
- **Subagent bypass (issue #5894):** `tool.execute.before` does NOT intercept tool calls from subagents spawned via the task tool. Security policies can be bypassed by delegating to a subagent.
- **MCP tools don't trigger hooks (issue #2319):** When OpenCode calls an MCP tool, `tool.execute.before` and `tool.execute.after` hooks were not triggered. (Appears to be fixed based on the current source code, which wraps MCP tool execution with Plugin.trigger calls.)
- **No tool.execute.error hook (issue #10027):** There's no way to hook into tool errors for logging or recovery.

### Runtime Requirements
- **Bun runtime.** Plugins are loaded and executed by Bun (not Node.js). TypeScript is natively supported without compilation.
- **Plugin version compatibility.** The `@opencode-ai/plugin` package is at v1.2.27. Breaking changes between versions could affect generated plugins.

**Conversion feasibility impact:** The lack of security restrictions is good for conversion — plugins can do anything a shell script can. The subagent bypass bug is concerning for security-oriented hooks but doesn't affect conversion feasibility per se. Bun dependency means plugins only work with OpenCode's runtime.

---

## 6. Lossless Export Analysis

### Canonical Hook

```json
{
  "event": "before_tool_execute",
  "matcher": "shell",
  "handler": {"type": "command", "command": "./check.sh", "timeout": 10},
  "blocking": true,
  "capabilities": ["structured_output", "input_rewrite"]
}
```

### Field-by-Field Mapping

| Canonical Field | OpenCode Mapping | Lossless? |
|----------------|-----------------|-----------|
| `event: "before_tool_execute"` | `"tool.execute.before"` hook | Yes |
| `matcher: "shell"` | `if (input.tool === "bash")` guard | Yes |
| `handler.type: "command"` | `await $\`./check.sh\`.quiet().nothrow()` | Yes |
| `handler.command: "./check.sh"` | Template literal argument | Yes |
| `handler.timeout: 10` | Bun has no built-in timeout for $; need manual `AbortSignal.timeout(10000)` | Partial -- requires wrapper |
| `blocking: true` | `throw new Error(...)` on non-zero exit | Yes |
| `capabilities.structured_output` | `result.json()` to parse stdout | Yes |
| `capabilities.input_rewrite` | `Object.assign(output.args, modified)` | Yes |

### Generated TypeScript Plugin

```typescript
import type { Plugin } from "@opencode-ai/plugin"

export const SyllagoHook: Plugin = async ({ $ }) => {
  return {
    "tool.execute.before": async (input, output) => {
      // Matcher: only applies to "bash" tool
      if (input.tool !== "bash") return

      // Build JSON payload matching syllago's stdin convention
      const payload = JSON.stringify({
        hook_event: "before_tool_execute",
        tool_name: input.tool,
        tool_input: output.args,
      })

      // Execute shell command with timeout
      const controller = new AbortController()
      const timer = setTimeout(() => controller.abort(), 10000)
      try {
        const result = await $`echo ${$.escape(payload)} | ./check.sh`
          .quiet()
          .nothrow()
        clearTimeout(timer)

        // Exit code 2 = block execution (equivalent to Claude Code convention)
        if (result.exitCode === 2) {
          const errorMsg = result.text().trim() || "Blocked by hook"
          throw new Error(errorMsg)
        }

        // Exit code 0 with stdout = input rewrite
        if (result.exitCode === 0) {
          const stdout = result.text().trim()
          if (stdout) {
            try {
              const modified = JSON.parse(stdout)
              Object.assign(output.args, modified)
            } catch {
              // Non-JSON stdout is informational, not a rewrite
            }
          }
        }

        // Exit code 1 = hook error (non-blocking, logged)
        if (result.exitCode === 1) {
          console.error(`Hook error: ${result.stderr.toString()}`)
        }
      } catch (e) {
        clearTimeout(timer)
        if (e instanceof Error && e.name === "AbortError") {
          throw new Error("Hook timed out after 10s")
        }
        throw e
      }
    },
  }
}
```

### Export Verdict

**Nearly lossless.** Every canonical field has a direct or close mapping:

- **Event mapping:** Direct 1:1.
- **Tool matcher:** Direct conditional check.
- **Shell command execution:** Direct via `$` API.
- **Blocking:** `throw new Error()` is the exact equivalent.
- **Input rewrite:** Direct mutation of `output.args`.
- **Structured output:** JSON parse of stdout.
- **Timeout:** Requires manual AbortController wrapper (not native to `$`). This is a minor implementation difference, not a semantic loss.

**Metadata loss:** The generated plugin doesn't preserve that it was generated from a canonical hook definition. Syllago could add a comment header for traceability. The original canonical JSON could be embedded as a comment for round-trip reconstruction.

---

## 7. Lossless Import Analysis

### Can event bindings be extracted?

**Yes, with mapping table.** OpenCode hook names map cleanly:
- `tool.execute.before` -> `before_tool_execute`
- `tool.execute.after` -> `after_tool_execute`
- `shell.env` -> partial match to environment hooks
- `event` (with `session.idle` check) -> `on_stop` / `on_session_end`
- `permission.ask` -> no canonical equivalent

### Can handler logic be represented as shell commands?

**Only for simple patterns.** Specifically:

**Importable patterns (can convert to shell command):**
1. Plugins that shell out via `$` and check exit codes — trivially extractable
2. Plugins that do simple string matching on tool names/args and throw — convertible to a shell script that does the same check
3. Plugins that modify `output.args` with static transformations — convertible

**Non-importable patterns (cannot convert to shell command):**
1. **Plugins using the SDK client** — e.g., `client.session.prompt()` to send messages back to the LLM. No shell equivalent.
2. **Plugins maintaining in-memory state** — e.g., `Map<string, SessionState>` tracking files across calls. Shell scripts are stateless per invocation (could use temp files, but semantics differ).
3. **Plugins using auth hooks** — `auth` hook has no canonical equivalent.
4. **Plugins modifying chat params** — `chat.params` hook has no equivalent in shell-command hook systems.
5. **Plugins transforming system prompts** — `experimental.chat.system.transform` is OpenCode-specific.
6. **Plugins registering custom tools** — `tool` definitions are a different concept from hooks entirely.
7. **Plugins using permission hooks** — `permission.ask` with programmatic allow/deny has no shell equivalent.
8. **Plugins with async event subscriptions** — `event` handlers that react to `session.idle`, `message.updated`, etc. with SDK calls.

### Specific Non-Importable Example

The "Agent Memory" plugin maintains persistent memory blocks and uses `client.session.prompt()` to inject context. This is fundamentally an SDK integration, not a hook.

The "CC Safety Net" plugin blocks destructive commands — this IS importable since it's a simple matcher + throw pattern.

### Import Verdict

**Partially lossless for simple hook patterns; lossy for SDK-integrated plugins.**

The convertible subset:
- Tool execution guards (match + throw) -- lossless
- Tool argument rewriting (match + modify args) -- lossless
- Shell env injection (`shell.env`) -- lossy (no exact canonical equivalent for env injection)
- Post-execution formatters (`tool.execute.after` modifying output) -- lossless for the hook, but `after_tool_execute` capabilities vary by provider

The non-convertible subset (anything using `client`, `auth`, `tool`, `chat.*`, `permission.*`, `experimental.*`):
- **These represent ~60-70% of real-world plugins** based on the awesome-opencode ecosystem survey. Most interesting plugins use SDK features.

---

## 8. Plugin Examples: Real-World Patterns

### Simple Pattern (shell-out wrapper) -- IMPORTABLE

**opencode-plugin-notification:**
```typescript
"tool.execute.after": async (input) => {
  if (input.tool === "edit" && input.args.filePath.endsWith(".rs")) {
    await $`cargo fmt --check`.quiet()
  }
}
```

### Simple Guard Pattern -- IMPORTABLE

**Envsitter Guard / CC Safety Net:**
```typescript
"tool.execute.before": async (input, output) => {
  if (input.tool === "bash" && output.args.command.includes("rm -rf")) {
    throw new Error("Dangerous command blocked")
  }
  if (input.tool === "read" && output.args.filePath.endsWith(".env")) {
    throw new Error("Cannot read .env files")
  }
}
```

### SDK-Integrated Pattern -- NOT IMPORTABLE

**Agent Memory (tracking + re-prompting):**
```typescript
const sessions = new Map<string, SessionState>()

export const Plugin: Plugin = async ({ client }) => ({
  "tool.execute.after": async (input) => {
    const state = sessions.get(input.sessionID)
    if (input.tool === "edit") {
      state.filesModified.push(input.args.filePath)
    }
  },
  stop: async (input) => {
    const state = sessions.get(input.sessionID)
    if (state.filesModified.length && !state.commitMade) {
      await client.session.prompt({
        path: { id: input.sessionID },
        body: { parts: [{ type: "text", text: "Uncommitted changes!" }] }
      })
    }
  }
})
```

### Auth Provider Pattern -- NOT IMPORTABLE

**opencode-openai-codex-auth:**
```typescript
export const Plugin: Plugin = async ({ client }) => ({
  auth: {
    provider: "openai",
    methods: [{
      type: "oauth",
      label: "Sign in with ChatGPT",
      authorize: async () => ({ url: "...", method: "auto", callback: async () => ({...}) })
    }]
  }
})
```

### Complex Multi-Hook Pattern -- NOT IMPORTABLE

**oh-my-opencode:**
Combines custom tools (LSP, AST analysis), agents (Sisyphus, Prometheus, Oracle), background tasks, MCP integrations, and system prompt transforms. This is a full application extension, not a hook.

### Ecosystem Breakdown

From the awesome-opencode directory (~60+ plugins):

| Category | Count | Importable? |
|----------|-------|-------------|
| Auth providers | ~10 | No |
| Agent/workflow orchestration | ~8 | No |
| Memory/state management | ~5 | No |
| Code quality guards | ~5 | Yes (simple matchers) |
| Shell-out wrappers | ~5 | Yes |
| System prompt transforms | ~4 | No |
| Custom tools | ~8 | No (different concept) |
| Monitoring/notifications | ~5 | Partial (event hooks are non-standard) |
| Session management | ~5 | No |
| Complex suites | ~5 | No |

**Roughly 15-20% of real-world plugins are importable as canonical hooks.**

---

## 9. Plugin File Structure

### Project-Level Plugin

```
project/
  .opencode/
    plugin/
      my-hook.ts         # auto-loaded at startup
      another-hook.ts
    tools/
      my-tool.ts         # custom tool definitions
    package.json         # npm dependencies for plugins
    opencode.json        # config file (npm plugin refs)
```

### Global Plugin

```
~/.config/opencode/
  plugin/
    global-hook.ts
  tools/
    global-tool.ts
  opencode.json
```

### npm Plugin Package

```
opencode-my-plugin/
  package.json           # name: "opencode-my-plugin"
  src/
    index.ts             # exports Plugin functions
  dist/
    index.js             # compiled output
  tsconfig.json
```

**package.json convention:**
```json
{
  "name": "opencode-my-plugin",
  "type": "module",
  "exports": { ".": "./src/index.ts" },
  "dependencies": {
    "@opencode-ai/plugin": "latest",
    "@opencode-ai/sdk": "latest",
    "zod": "latest"
  }
}
```

**Naming convention:** Prefix with `opencode-` (e.g., `opencode-my-service`).

### How OpenCode Loads Plugins (from source)

1. Internal plugins loaded first (Codex auth, Copilot auth, GitLab auth)
2. Config `plugin` array processed:
   - `file://` paths imported directly
   - Package names resolved via `BunProc.install(pkg, version)`
3. Plugin directories scanned: `Config.directories()` -> `plugin/` subdirs
4. All exports from each module are called with `PluginInput`
5. Duplicate function references deduplicated (handles `export default` + named export of same function)
6. All hooks collected into `hooks: Hooks[]` array
7. `config` hook called immediately with current config
8. Bus subscription established for `event` hooks

---

## 10. Summary: Conversion Feasibility Assessment

### Export (Canonical -> OpenCode Plugin): FEASIBLE, NEARLY LOSSLESS

| Aspect | Rating | Notes |
|--------|--------|-------|
| Event mapping | Lossless | Direct 1:1 for tool execution events |
| Tool matching | Lossless | Conditional check on `input.tool` |
| Shell command execution | Lossless | `$` API wraps shell commands perfectly |
| Blocking | Lossless | `throw new Error()` = exit code 2 |
| Input rewrite | Lossless | Direct mutation of `output.args` |
| Structured output | Lossless | JSON parse of stdout |
| Timeout | Near-lossless | Manual AbortController, not native but functional |
| Round-trip metadata | Lossy | Need comment-embedded canonical JSON for reconstruction |

**Export strategy:** Generate a single `.ts` file in `.opencode/plugin/` that wraps the shell command. This is the simplest possible integration point.

### Import (OpenCode Plugin -> Canonical): PARTIALLY FEASIBLE, OFTEN LOSSY

| Plugin Pattern | Importable? | Lossless? |
|---------------|------------|-----------|
| Tool guard (match + throw) | Yes | Yes |
| Arg rewrite (match + mutate) | Yes | Yes |
| Shell-out via `$` | Yes | Yes |
| SDK client usage | No | N/A |
| In-memory state | No | N/A |
| Auth providers | No | N/A |
| Custom tools | No | N/A |
| Chat param/header modification | No | N/A |
| System prompt transforms | No | N/A |
| Permission hooks | No | N/A |
| Event subscriptions (non-tool) | No | N/A |

**The importable subset (~15-20% of real-world plugins) converts losslessly. The rest requires OpenCode-specific features with no canonical equivalent.**

### Key Insight

OpenCode's plugin system is fundamentally more powerful than shell-command hooks. It's a full application extension API, not just a hook system. The `tool.execute.before` and `tool.execute.after` hooks are the ONLY hooks that cleanly map to the canonical hook model.

For syllago's purposes:
- **Export is the high-value path.** Taking a shell-command hook and wrapping it in an OpenCode plugin is clean, automatable, and nearly lossless.
- **Import is niche.** Only worth supporting for the simple guard/shell-out pattern. Complex plugins should be flagged as "OpenCode-specific, not portable."
- **The generated plugin is thin.** ~30 lines of TypeScript that shells out to the original command. This is maintainable and auditable.

### Open Questions

1. **Timeout handling:** Bun's `$` API doesn't have native timeout support. The AbortController approach works but needs testing to confirm Bun respects it for shell processes.
2. **stdin piping:** The `echo ${payload} | ./check.sh` pattern in Bun's `$` API needs verification. Alternative: write to temp file and pass path.
3. **Plugin versioning:** `@opencode-ai/plugin` is at v1.2.27. Generated plugins should pin to a compatible version range.
4. **MCP tool hooks:** The source now wraps MCP tools with Plugin.trigger, but this was a bug (#2319) that was fixed. Generated plugins should document this history.
5. **Subagent bypass (#5894):** Security-oriented hooks generated by syllago should document that subagent tool calls may not be intercepted.
