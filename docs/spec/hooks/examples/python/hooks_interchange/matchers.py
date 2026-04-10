"""Matcher parsing and tool vocabulary for the Hook Interchange Format.

Implements the matcher resolution and parsing requirements from the
Hook Interchange Format Specification, Section 6, and the tool vocabulary
from the Tool Vocabulary registry (tools.md §1).

Conformance level: Extended (Section 8.2) — covers bare string, pattern,
MCP, and array matcher types.
"""

from __future__ import annotations

import re
import warnings
from typing import Any


# ---------------------------------------------------------------------------
# Tool vocabulary (tools.md §1)
# ---------------------------------------------------------------------------

# Maps canonical tool name → provider slug → provider-native tool name.
# A None value means the provider has no equivalent tool for that canonical
# name. Split-event providers (cursor, windsurf) carry event-level notes in
# parentheses in the spec table; those entries are omitted here because they
# are handled at the encode layer (event mapping), not as matcher strings.
TOOL_VOCABULARY: dict[str, dict[str, str | None]] = {
    "shell": {
        "claude-code": "Bash",
        "gemini-cli": "run_shell_command",
        "cursor": "run_terminal_cmd",
        "windsurf": None,   # split-event: maps to pre_run_command event, not a matcher
        "copilot-cli": "bash",
        "kiro": "execute_bash",
        "opencode": "bash",
    },
    "file_read": {
        "claude-code": "Read",
        "gemini-cli": "read_file",
        "cursor": "read_file",
        "windsurf": None,   # split-event: maps to pre_read_code event
        "copilot-cli": "view",
        "kiro": "fs_read",
        "opencode": "read",
    },
    "file_write": {
        "claude-code": "Write",
        "gemini-cli": "write_file",
        "cursor": "edit_file",
        "windsurf": None,   # split-event: maps to pre_write_code event
        "copilot-cli": "create",
        "kiro": "fs_write",
        "opencode": "write",
    },
    "file_edit": {
        "claude-code": "Edit",
        "gemini-cli": "replace",
        "cursor": "edit_file",
        "windsurf": None,   # split-event: maps to pre_write_code event
        "copilot-cli": "edit",
        "kiro": "fs_write",
        "opencode": "edit",
    },
    "search": {
        "claude-code": "Grep",
        "gemini-cli": "grep_search",
        "cursor": "grep_search",
        "windsurf": None,
        "copilot-cli": "grep",
        "kiro": "grep",
        "opencode": "grep",
    },
    "find": {
        "claude-code": "Glob",
        "gemini-cli": "glob",
        "cursor": "file_search",
        "windsurf": None,
        "copilot-cli": "glob",
        "kiro": "glob",
        "opencode": "glob",
    },
    "web_search": {
        "claude-code": "WebSearch",
        "gemini-cli": "google_web_search",
        "cursor": "web_search",
        "windsurf": None,
        "copilot-cli": None,
        "kiro": "web_search",
        "opencode": None,
    },
    "web_fetch": {
        "claude-code": "WebFetch",
        "gemini-cli": "web_fetch",
        "cursor": None,
        "windsurf": None,
        "copilot-cli": "web_fetch",
        "kiro": "web_fetch",
        "opencode": None,
    },
    "agent": {
        "claude-code": "Agent",
        "gemini-cli": None,
        "cursor": None,
        "windsurf": None,
        "copilot-cli": "task",
        "kiro": "use_subagent",
        "opencode": None,
    },
}

# Reverse index: provider → native name → canonical name. Built once at
# module load. Only includes entries where the native name is not None and
# is not shared by multiple canonical names for the same provider (in which
# case the first canonical name encountered wins — callers that care about
# ambiguity should use the forward table directly).
_REVERSE_VOCABULARY: dict[str, dict[str, str]] = {}

for _canonical, _providers in TOOL_VOCABULARY.items():
    for _provider, _native in _providers.items():
        if _native is None:
            continue
        _REVERSE_VOCABULARY.setdefault(_provider, {})
        # First write wins — preserves stable ordering for ambiguous cases.
        _REVERSE_VOCABULARY[_provider].setdefault(_native, _canonical)

# ---------------------------------------------------------------------------
# MCP combined-string encoding/decoding (tools.md §2, hooks.md §6.3)
# ---------------------------------------------------------------------------

# Groups of providers that share a combined MCP string format.
_MCP_FORMAT_GROUPS: dict[str, str] = {
    # format key → regex pattern template used for decode
    "mcp__server__tool": r"^mcp__(?P<server>.+?)__(?P<tool>.+)$",
    "mcp__server":       r"^mcp__(?P<server>.+)$",
    "mcp_server_tool":   r"^mcp_(?P<server>.+?)_(?P<tool>.+)$",
    "mcp_server":        r"^mcp_(?P<server>.+)$",
    "server/tool":       r"^(?P<server>[^/]+)/(?P<tool>.+)$",
    "server__tool":      r"^(?P<server>.+?)__(?P<tool>.+)$",
}

# Maps provider slug → encoding function parameters.
_MCP_PROVIDER_FORMAT: dict[str, str] = {
    "claude-code":  "double_underscore_prefix",   # mcp__<server>__<tool>
    "kiro":         "double_underscore_prefix",
    "gemini-cli":   "single_underscore_prefix",   # mcp_<server>_<tool>
    "copilot-cli":  "slash",                       # <server>/<tool>
    "cursor":       "double_underscore",           # <server>__<tool>
    "windsurf":     "double_underscore",
    # opencode not present in tools.md §2; falls through to pass-through
}


def _encode_mcp(server: str, tool: str | None, provider: str) -> str:
    """Encode an MCP server+tool pair into the provider-native combined string.

    When ``tool`` is None (match all tools on a server), providers that have
    a prefix-based format omit the tool segment; providers that use separator-
    based formats still require a tool component — in that case the bare server
    name is returned and the caller should be aware matching semantics may
    differ.
    """
    fmt = _MCP_PROVIDER_FORMAT.get(provider)
    if fmt == "double_underscore_prefix":
        if tool is not None:
            return f"mcp__{server}__{tool}"
        return f"mcp__{server}"
    elif fmt == "single_underscore_prefix":
        if tool is not None:
            return f"mcp_{server}_{tool}"
        return f"mcp_{server}"
    elif fmt == "slash":
        if tool is not None:
            return f"{server}/{tool}"
        return server
    elif fmt == "double_underscore":
        if tool is not None:
            return f"{server}__{tool}"
        return server
    else:
        # Unknown provider — return a best-effort mcp__server__tool string
        # and let the caller decide what to do with it.
        if tool is not None:
            return f"mcp__{server}__{tool}"
        return f"mcp__{server}"


def _decode_mcp(native: str, provider: str) -> dict[str, Any] | None:
    """Try to parse a provider-native combined MCP string back to canonical form.

    Returns a canonical MCP matcher dict ``{"mcp": {"server": ..., "tool": ...}}``
    when the string matches the expected format for ``provider``, or ``None``
    when it does not look like an MCP matcher for that provider.
    """
    fmt = _MCP_PROVIDER_FORMAT.get(provider)

    if fmt == "double_underscore_prefix":
        # mcp__<server>__<tool>  or  mcp__<server>
        m = re.match(r"^mcp__(?P<server>.+?)__(?P<tool>.+)$", native)
        if m:
            return {"mcp": {"server": m.group("server"), "tool": m.group("tool")}}
        m = re.match(r"^mcp__(?P<server>.+)$", native)
        if m:
            return {"mcp": {"server": m.group("server")}}
        return None

    elif fmt == "single_underscore_prefix":
        # mcp_<server>_<tool>  or  mcp_<server>
        # Greedy split: first token after "mcp_" up to the next "_" is the
        # server; the remainder is the tool. This is inherently ambiguous when
        # server or tool names contain underscores, so we take the shortest
        # server name (non-greedy) to match the encoding behaviour.
        m = re.match(r"^mcp_(?P<server>[^_]+)_(?P<tool>.+)$", native)
        if m:
            return {"mcp": {"server": m.group("server"), "tool": m.group("tool")}}
        m = re.match(r"^mcp_(?P<server>.+)$", native)
        if m:
            return {"mcp": {"server": m.group("server")}}
        return None

    elif fmt == "slash":
        # <server>/<tool>  — a slash is unambiguous
        m = re.match(r"^(?P<server>[^/]+)/(?P<tool>.+)$", native)
        if m:
            return {"mcp": {"server": m.group("server"), "tool": m.group("tool")}}
        return None

    elif fmt == "double_underscore":
        # <server>__<tool>  — no prefix, so only trigger on double-underscore
        m = re.match(r"^(?P<server>.+?)__(?P<tool>.+)$", native)
        if m:
            return {"mcp": {"server": m.group("server"), "tool": m.group("tool")}}
        return None

    return None


# ---------------------------------------------------------------------------
# Public API
# ---------------------------------------------------------------------------


def resolve_matcher(
    matcher: str | dict[str, Any] | list[Any] | None,
    provider: str,
) -> str | list[str] | None:
    """Resolve a canonical matcher to provider-native form.

    Implements the encoding side of the matcher system (§6.1–§6.5).

    Args:
        matcher: A canonical matcher value. May be:
            - A bare string (tool vocabulary lookup, §6.1)
            - A pattern object ``{"pattern": "..."}`` (pass-through, §6.2)
            - An MCP object ``{"mcp": {"server": ..., "tool": ...}}`` (§6.3)
            - An array of any of the above (§6.4)
            - ``None`` or absent (matches all tools, §6.5)
        provider: The target provider slug (e.g., ``"claude-code"``).

    Returns:
        - ``None`` when the matcher is ``None`` (wildcard).
        - A provider-native string for single bare-string or pattern matchers.
        - A provider-native MCP combined string for MCP matchers.
        - A list of resolved values for array matchers (elements that cannot
          be resolved are omitted; callers should check for an empty list).

    Notes:
        - Bare strings not found in the vocabulary are passed through as
          literals with a :mod:`warnings` warning, per §6.1.
        - Pattern matchers are not portable; the ``pattern`` value is returned
          verbatim (§6.2).
        - When a canonical tool name has no equivalent for the target provider
          (``None`` in :data:`TOOL_VOCABULARY`), the canonical name is passed
          through with a warning.
    """
    if matcher is None:
        return None

    if isinstance(matcher, list):
        resolved: list[str] = []
        for element in matcher:
            result = resolve_matcher(element, provider)
            if result is None:
                # An element that resolved to None means "all tools" — keep it
                # only if it is the sole element; in an array context, skip it.
                continue
            if isinstance(result, list):
                resolved.extend(result)
            else:
                resolved.append(result)
        return resolved

    if isinstance(matcher, dict):
        if "pattern" in matcher:
            # §6.2: pattern matchers pass through verbatim — not portable.
            pattern_val = matcher["pattern"]
            if not isinstance(pattern_val, str):
                warnings.warn(
                    f"pattern matcher value is not a string: {pattern_val!r}",
                    stacklevel=2,
                )
            return pattern_val  # type: ignore[return-value]

        if "mcp" in matcher:
            # §6.3: encode to provider-specific combined format.
            mcp = matcher["mcp"]
            server: str = mcp.get("server", "")
            tool: str | None = mcp.get("tool")
            return _encode_mcp(server, tool, provider)

        # Unknown object shape — pass through with warning.
        warnings.warn(
            f"unrecognised matcher object (expected 'pattern' or 'mcp' key): "
            f"{matcher!r}",
            stacklevel=2,
        )
        return str(matcher)

    if isinstance(matcher, str):
        # §6.1: bare string — look up in tool vocabulary.
        provider_map = TOOL_VOCABULARY.get(matcher)
        if provider_map is None:
            # Not a known canonical name — pass through as literal per §6.1.
            warnings.warn(
                f"tool name {matcher!r} is not in the canonical vocabulary; "
                f"passing through as a literal string",
                stacklevel=2,
            )
            return matcher

        native = provider_map.get(provider)
        if native is None:
            # Provider has no equivalent — pass through canonical name with
            # warning so the manifest is not silently corrupted.
            warnings.warn(
                f"canonical tool {matcher!r} has no equivalent for provider "
                f"{provider!r}; passing through canonical name as literal",
                stacklevel=2,
            )
            return matcher

        return native

    # Unexpected type — pass through as string with warning.
    warnings.warn(
        f"unexpected matcher type {type(matcher).__name__!r}: {matcher!r}",
        stacklevel=2,
    )
    return str(matcher)


def parse_matcher(
    native_matcher: str,
    provider: str,
) -> dict[str, Any] | str | None:
    """Parse a provider-native matcher string back into canonical form.

    Implements the decoding side of the matcher system (§6.1–§6.3).

    This is the reverse of :func:`resolve_matcher` for the single-value case.
    Array matchers must be decoded element-by-element by the caller.

    Args:
        native_matcher: A provider-native matcher string (e.g., ``"Bash"``,
            ``"mcp__github__create_issue"``).
        provider: The source provider slug (e.g., ``"claude-code"``).

    Returns:
        - A canonical MCP object ``{"mcp": {"server": ..., "tool": ...}}``
          when the string matches the provider's MCP combined format (§6.3).
        - A canonical bare string (tool name) when the native name maps to a
          known canonical entry in the reverse vocabulary (§6.1).
        - The original ``native_matcher`` string when no canonical mapping is
          found — callers MAY wrap this in a pattern object if desired.
        - ``None`` only when ``native_matcher`` is an empty string, which is
          treated as a wildcard (§6.5).

    Notes:
        - MCP detection takes priority over the tool vocabulary lookup so that
          provider-native combined strings are not misidentified as bare tool
          names.
        - Pattern objects cannot be recovered from a native matcher string;
          this function has no way to distinguish a literal tool name from a
          regex pattern that happens to match a literal.
    """
    if not native_matcher:
        return None

    # Try MCP decode first — provider-specific combined strings must be
    # detected before falling through to the tool vocabulary.
    mcp_result = _decode_mcp(native_matcher, provider)
    if mcp_result is not None:
        return mcp_result

    # Attempt reverse vocabulary lookup.
    provider_reverse = _REVERSE_VOCABULARY.get(provider, {})
    canonical = provider_reverse.get(native_matcher)
    if canonical is not None:
        return canonical

    # Unknown native name — return as-is. Callers that want a pattern object
    # should wrap: {"pattern": parse_matcher(name, provider)}.
    return native_matcher


# ---------------------------------------------------------------------------
# Convenience helpers — provider-specific wrappers used by adapter modules
# ---------------------------------------------------------------------------


def canonical_tool_to_cc(canonical_name: str) -> str:
    """Resolve a canonical tool name to Claude Code native name."""
    native = TOOL_VOCABULARY.get(canonical_name, {}).get("claude-code")
    if native is None:
        warnings.warn(f"No Claude Code mapping for canonical tool {canonical_name!r}")
        return canonical_name
    return native


def cc_tool_to_canonical(cc_name: str) -> str | None:
    """Resolve a Claude Code native tool name to canonical."""
    reverse = _REVERSE_VOCABULARY.get("claude-code", {})
    return reverse.get(cc_name)


def encode_mcp_matcher(mcp: dict[str, Any]) -> str:
    """Encode an MCP matcher dict to Claude Code combined format."""
    server = mcp.get("server", "")
    tool = mcp.get("tool", "")
    return f"mcp__{server}__{tool}" if tool else f"mcp__{server}__"


def decode_mcp_matcher(native: str) -> dict[str, Any] | None:
    """Decode a Claude Code MCP combined string to canonical matcher."""
    return _decode_mcp(native, "claude-code")
