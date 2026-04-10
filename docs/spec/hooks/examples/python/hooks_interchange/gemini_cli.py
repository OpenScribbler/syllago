"""Gemini CLI adapter: encode/decode canonical hook manifests.

Implements the conversion pipeline stages (§7.1 Decode, §7.3 Encode) for the
Gemini CLI provider. Translates between the canonical hook manifest format and
Gemini CLI's native settings.json hook structure.

Gemini CLI native format::

    {
        "hooks": [
            {
                "trigger": "BeforeTool",
                "toolMatcher": "run_shell_command",
                "command": "./safety-check.sh",
                "timeoutMs": 10000
            }
        ]
    }

Conformance level: Core (§8.1).

Key provider characteristics:
- Flat array structure (not grouped by event like Claude Code).
- Timeout is in milliseconds; canonical format uses seconds.
- MCP tool names use single-underscore prefix: ``mcp_<server>_<tool>``.
- No native ``blocking`` field — Gemini CLI infers blocking from the trigger
  event. On encode the field is dropped; on decode it defaults to ``False``.
- Array matchers (§6.4) are expanded to one hook entry per tool name because
  Gemini CLI's ``toolMatcher`` is a single string, not a list.
"""

from __future__ import annotations

from typing import Any

from .matchers import parse_matcher, resolve_matcher

# ---------------------------------------------------------------------------
# Event name map  (§4 Event Name Mapping — gemini-cli column)
# ---------------------------------------------------------------------------

EVENT_MAP: dict[str, str] = {
    # §1 Core events
    "before_tool_execute": "BeforeTool",
    "after_tool_execute": "AfterTool",
    "session_start": "SessionStart",
    "session_end": "SessionEnd",
    "before_prompt": "BeforeAgent",
    "agent_stop": "AfterAgent",
    # §2 Extended events supported by Gemini CLI
    "before_compact": "PreCompress",
    "notification": "Notification",
    "before_model": "BeforeModel",
    "after_model": "AfterModel",
    "before_tool_selection": "BeforeToolSelection",
}

REVERSE_EVENT_MAP: dict[str, str] = {v: k for k, v in EVENT_MAP.items()}

# Tool events for which ``toolMatcher`` is applicable.
_TOOL_EVENTS: frozenset[str] = frozenset({"before_tool_execute", "after_tool_execute"})

_PROVIDER = "gemini-cli"


# ---------------------------------------------------------------------------
# Encode: canonical manifest → Gemini CLI native format
# ---------------------------------------------------------------------------


def encode(manifest: dict[str, Any]) -> dict[str, Any]:
    """Convert a canonical hook manifest to Gemini CLI native format.

    The output is suitable for merging into a Gemini CLI ``settings.json``
    under the ``"hooks"`` key.

    Args:
        manifest: A parsed canonical hook manifest (``spec`` + ``hooks``
            array). Unknown fields are ignored per §3.2.

    Returns:
        A dict with a single top-level ``"hooks"`` key whose value is a flat
        array of Gemini CLI hook objects.

    Notes:
        - Hooks whose canonical event has no Gemini CLI mapping are dropped.
        - Array matchers (§6.4) expand to one hook entry per resolved tool
          name, because Gemini CLI does not support compound ``toolMatcher``
          values.
        - The ``blocking`` field is consumed but not written to the output.
        - Handler fields ``cwd``, ``env``, and ``platform`` are dropped —
          Gemini CLI does not support these capabilities.
        - ``provider_data["gemini-cli"]`` is merged into each hook object if
          present on the hook definition.
        - Timeout is converted from seconds (canonical) to milliseconds.
    """
    native_hooks: list[dict[str, Any]] = []

    for hook in manifest.get("hooks", []):
        canonical_event: str = hook.get("event", "")
        gemini_trigger = EVENT_MAP.get(canonical_event)
        if gemini_trigger is None:
            # No Gemini CLI equivalent — drop silently per §7.3.
            continue

        is_tool_event = canonical_event in _TOOL_EVENTS
        matcher = hook.get("matcher")
        provider_data: dict[str, Any] | None = hook.get("provider_data")

        if is_tool_event and matcher is not None:
            # Resolve the matcher to one or more native tool name strings.
            resolved = resolve_matcher(matcher, _PROVIDER)

            if isinstance(resolved, list):
                # Array matcher — emit one entry per tool name (§6.4).
                for tool_name in resolved:
                    entry = _build_entry(
                        gemini_trigger,
                        tool_name,
                        hook.get("handler", {}),
                        provider_data,
                    )
                    native_hooks.append(entry)
            else:
                # Single resolved tool name (may be None for wildcard).
                entry = _build_entry(
                    gemini_trigger,
                    resolved,
                    hook.get("handler", {}),
                    provider_data,
                )
                native_hooks.append(entry)
        else:
            # Non-tool event, or tool event with omitted matcher (wildcard).
            entry = _build_entry(
                gemini_trigger,
                None,
                hook.get("handler", {}),
                provider_data,
            )
            native_hooks.append(entry)

    return {"hooks": native_hooks}


def _build_entry(
    trigger: str,
    tool_matcher: str | None,
    handler: dict[str, Any],
    provider_data: dict[str, Any] | None,
) -> dict[str, Any]:
    """Build a single Gemini CLI hook object.

    Args:
        trigger:      Native Gemini CLI event name (e.g. ``"BeforeTool"``).
        tool_matcher: Native tool name string, or ``None`` for wildcard / non-
                      tool events.
        handler:      Canonical handler dict.
        provider_data: Full ``provider_data`` dict from the canonical hook
                       definition. Only ``provider_data["gemini-cli"]`` is used.

    Returns:
        A Gemini CLI hook object dict.
    """
    entry: dict[str, Any] = {"trigger": trigger}

    if tool_matcher is not None:
        entry["toolMatcher"] = tool_matcher

    command = handler.get("command")
    if command is not None:
        entry["command"] = command

    timeout_sec = handler.get("timeout")
    if timeout_sec is not None:
        entry["timeoutMs"] = int(timeout_sec * 1000)

    # §3.6 — render provider_data for the target provider.
    if provider_data and isinstance(provider_data, dict):
        gemini_pd = provider_data.get(_PROVIDER)
        if isinstance(gemini_pd, dict):
            entry.update(gemini_pd)

    return entry


# ---------------------------------------------------------------------------
# Decode: Gemini CLI native format → canonical manifest
# ---------------------------------------------------------------------------


def decode(native: dict[str, Any]) -> dict[str, Any]:
    """Convert a Gemini CLI native hook configuration to a canonical manifest.

    Args:
        native: A dict containing a ``"hooks"`` key with a flat list of Gemini
                CLI hook objects, as read from ``settings.json``.

    Returns:
        A canonical hook manifest with ``spec`` and ``hooks`` array.
    """
    canonical_hooks: list[dict[str, Any]] = []

    for entry in native.get("hooks", []):
        if not isinstance(entry, dict):
            continue

        gemini_trigger: str = entry.get("trigger", "")
        canonical_event = REVERSE_EVENT_MAP.get(gemini_trigger)
        if canonical_event is None:
            # Unknown Gemini CLI trigger — skip.
            continue

        canonical_hook: dict[str, Any] = {"event": canonical_event}

        # Decode matcher when present and applicable to a tool event.
        raw_tool_matcher = entry.get("toolMatcher")
        if canonical_event in _TOOL_EVENTS and raw_tool_matcher:
            decoded = parse_matcher(raw_tool_matcher, _PROVIDER)
            if decoded is not None:
                canonical_hook["matcher"] = decoded

        # Decode handler.
        canonical_hook["handler"] = _decode_handler(entry)

        # Gemini CLI has no native blocking field — default to False.
        canonical_hook["blocking"] = False

        canonical_hooks.append(canonical_hook)

    return {
        "spec": "hooks/0.1",
        "hooks": canonical_hooks,
    }


def _decode_handler(entry: dict[str, Any]) -> dict[str, Any]:
    """Build a canonical handler dict from a Gemini CLI hook object.

    Extracts ``command`` and ``timeoutMs`` (converted back to seconds).
    The ``trigger`` and ``toolMatcher`` fields are handled by the caller and
    are not included in the handler.

    Args:
        entry: A single Gemini CLI hook object dict.

    Returns:
        A canonical handler dict with at least ``type: "command"``.
    """
    handler: dict[str, Any] = {"type": "command"}

    command = entry.get("command")
    if command is not None:
        handler["command"] = command

    timeout_ms = entry.get("timeoutMs")
    if timeout_ms is not None:
        # Canonical unit is seconds. Use int when the result is a whole number
        # so round-trips through integer milliseconds produce integer seconds.
        secs = timeout_ms / 1000
        handler["timeout"] = int(secs) if secs == int(secs) else secs

    return handler
