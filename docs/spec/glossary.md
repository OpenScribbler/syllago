# Glossary

This glossary defines terms used in the [Hook Interchange Format Specification](hooks-v1.md). Terms are listed alphabetically.

---

**adapter**
A software component that converts between a provider's native hook format and the canonical format defined by this specification. Each provider has a corresponding adapter that implements decode (native to canonical) and encode (canonical to native) operations.

**blocking**
A property of a hook that determines whether it can prevent the triggering action from proceeding. A blocking hook that returns exit code 2 or `decision: "deny"` stops the action. A non-blocking hook runs for observational purposes only.

**canonical format**
The provider-neutral JSON representation of hooks defined by this specification. The canonical format serves as the interchange hub through which hooks are converted between providers.

**capability**
An optional semantic feature beyond the core hook model (event binding, handler execution, blocking). Each capability describes an intent that one or more providers implement. Examples: `structured_output`, `input_rewrite`, `platform_commands`. Capabilities are inferred by tooling from manifest fields, not declared by hook authors.

**conformance level**
One of three tiers (Core, Extended, Full) that define what an implementation must support. See Section 13 of the specification.

**core event**
An event with near-universal provider support that is required for Core conformance. The six core events are: `before_tool_execute`, `after_tool_execute`, `session_start`, `session_end`, `before_prompt`, `agent_stop`.

**decode**
The process of reading a provider-native hook configuration and producing a canonical hook manifest. Performed by a provider's adapter.

**degradation strategy**
The behavior applied when converting a hook to a target provider that lacks support for a capability the hook uses. One of: `block` (prevent the action), `warn` (run with reduced functionality), `exclude` (omit the hook entirely).

**encode**
The process of writing a canonical hook manifest into a provider-native configuration format. Performed by a provider's adapter.

**event**
A named lifecycle moment in an AI coding tool's execution that can trigger a hook. Events are identified by canonical names in `snake_case` format (e.g., `before_tool_execute`, `session_start`).

**exit code**
The numeric status returned by a hook process upon termination. Exit code 0 indicates success, 1 indicates a hook error, 2 indicates a block request, and other values are treated as errors.

**extended event**
An event with partial provider support that is not required for Core conformance but is supported at the Extended conformance level.

**handler**
The executable component of a hook that runs when the hook's event fires. The most common handler type is `"command"` (shell script execution). Other handler types (`"http"`, `"prompt"`, `"agent"`) are defined as capabilities.

**hook**
A user-defined action that executes at a specific lifecycle point in an AI coding tool. A hook consists of an event binding, a handler, and optional configuration (matcher, blocking flag, degradation strategies, provider data).

**hook manifest**
A JSON (or YAML) document conforming to this specification that declares one or more hooks. The top-level structure includes a `spec` version field and a `hooks` array.

**matcher**
An expression on a hook definition that filters which tools trigger the hook. Matcher types include bare strings (tool vocabulary lookup), pattern objects (regex), MCP objects (structured MCP tool reference), arrays (OR of multiple matchers), and omitted (match all).

**MCP (Model Context Protocol)**
A protocol for connecting AI models to external tools and data sources. MCP tools are identified by a server name and tool name pair. The canonical format uses structured objects for MCP references to avoid the parsing ambiguity of combined string formats.

**provider**
An AI coding tool that implements a hook system. Each provider is identified by a canonical slug.

**provider data**
An opaque JSON object on a hook definition, keyed by provider slug, that holds provider-specific configuration with no canonical equivalent. Provider data is preserved during round-trip conversion and rendered only for the matching target provider.

**provider slug**
A unique string identifier for a provider. The canonical slugs are: `claude-code`, `gemini-cli`, `cursor`, `windsurf`, `vs-code-copilot`, `copilot-cli`, `kiro`, `opencode`.

**provider-exclusive event**
An event that exists in only one or two providers. Included in the event registry for lossless round-tripping but expected to be dropped or degraded during cross-provider conversion.

**RE2**
A regular expression syntax defined by Google's RE2 library. RE2 is recommended for cross-language compatibility because it guarantees linear-time matching and is supported in Go, Rust, Python, JavaScript (via libraries), and other languages.

**round-trip**
The process of decoding a hook from provider P into canonical format and encoding it back to provider P. A Full-conformant implementation must produce structurally equivalent output.

**split-event provider**
A provider that maps the canonical `before_tool_execute` event to multiple category-specific native events based on tool type. Cursor and Windsurf are split-event providers.

**tool vocabulary**
The set of canonical tool names that abstract over provider-specific naming. Used in bare string matchers. Examples: `shell`, `file_read`, `file_write`, `file_edit`, `search`, `find`.

**verification**
The optional fourth stage of the conversion pipeline where the encoded output is re-decoded to confirm structural fidelity. Required for Full conformance.
