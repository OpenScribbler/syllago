"""Claude Code adapter: encode/decode canonical hook manifests.

Implements the conversion pipeline stages (§7.1 Decode, §7.3 Encode) for the
Claude Code provider. Translates between the canonical hook manifest format and
Claude Code's native settings.json hook structure.

Claude Code native format::

    {
        "hooks": {
            "PreToolUse": [
                {"matcher": "Bash", "hooks": [{"type": "command", "command": "..."}]}
            ]
        }
    }

Conformance level: Core (§8.1).

Event support: all events that Claude Code maps to a native name (§4 Event Name
Mapping). Events with no Claude Code mapping are silently dropped on encode and
cannot be produced by decode.

Blocking semantics: Claude Code does not carry a per-hook ``blocking`` field in
its native format. On encode the field is dropped. On decode, blocking is inferred
from the event: ``PreToolUse`` and ``UserPromptSubmit`` are treated as
``blocking: true``; all other events are ``blocking: false``.
"""

from __future__ import annotations

from typing import Any

from .matchers import (
    canonical_tool_to_cc,
    cc_tool_to_canonical,
    encode_mcp_matcher,
    decode_mcp_matcher,
)

# ---------------------------------------------------------------------------
# Event name maps  (§4 Event Name Mapping — claude-code column)
# ---------------------------------------------------------------------------

EVENT_MAP: dict[str, str] = {
    # §1 Core events
    "before_tool_execute": "PreToolUse",
    "after_tool_execute": "PostToolUse",
    "session_start": "SessionStart",
    "session_end": "SessionEnd",
    "before_prompt": "UserPromptSubmit",
    "agent_stop": "Stop",
    # §2 Extended events supported by Claude Code
    "before_compact": "PreCompact",
    "notification": "Notification",
    "error_occurred": "StopFailure",
    "tool_use_failure": "PostToolUseFailure",
    "file_changed": "FileChanged",
    "subagent_start": "SubagentStart",
    "subagent_stop": "SubagentStop",
    "permission_request": "PermissionRequest",
    # §3 Provider-exclusive events
    "config_change": "ConfigChange",
}

REVERSE_EVENT_MAP: dict[str, str] = {v: k for k, v in EVENT_MAP.items()}

# Events where blocking: true is the decode default (PreToolUse + UserPromptSubmit).
_BLOCKING_EVENTS: frozenset[str] = frozenset({"PreToolUse", "UserPromptSubmit"})


# ---------------------------------------------------------------------------
# Encode: canonical manifest → Claude Code native format
# ---------------------------------------------------------------------------


def encode(manifest: dict[str, Any]) -> dict[str, Any]:
    """Convert a canonical hook manifest to Claude Code native format.

    The output is suitable for merging into a Claude Code ``settings.json``
    under the ``"hooks"`` key.

    Args:
        manifest: A parsed canonical hook manifest (``spec`` + ``hooks``
            array). Unknown fields are ignored per §3.2.

    Returns:
        A dict with a single top-level ``"hooks"`` key whose value is a dict
        mapping Claude Code event names to lists of matcher-group objects.

    Notes:
        - Hooks whose canonical event has no Claude Code mapping are dropped.
        - The ``blocking`` field is consumed but not written to the output
          (Claude Code does not carry it natively).
        - Handler fields ``cwd``, ``env``, and ``platform`` are not written —
          Claude Code does not support these capabilities.
        - ``provider_data["claude-code"]`` is merged into each handler object
          if present on the hook definition.
    """
    # Accumulate matcher-groups per CC event name.
    # events_groups: { cc_event_name: [matcher_group, ...] }
    events_groups: dict[str, list[dict[str, Any]]] = {}

    for hook in manifest.get("hooks", []):
        canonical_event: str = hook.get("event", "")
        cc_event = EVENT_MAP.get(canonical_event)
        if cc_event is None:
            # No CC equivalent — drop silently per §7.3 degradation.
            continue

        matcher_str = _encode_matcher(hook.get("matcher"), cc_event)
        handler = _encode_handler(hook.get("handler", {}), hook.get("provider_data"))

        matcher_group: dict[str, Any] = {
            "matcher": matcher_str,
            "hooks": [handler],
        }

        events_groups.setdefault(cc_event, []).append(matcher_group)

    return {"hooks": events_groups}


def _encode_matcher(
    matcher: Any,
    cc_event: str,  # noqa: ARG001  — reserved for future split-event logic
) -> str:
    """Translate a canonical matcher value to a Claude Code matcher string.

    Returns:
        A CC-native matcher string. Empty string for wildcard (omitted matcher).
    """
    if matcher is None:
        # §6.5 — omitted matcher = wildcard
        return ""

    if isinstance(matcher, str):
        # §6.1 — bare string: tool vocabulary lookup
        return canonical_tool_to_cc(matcher)

    if isinstance(matcher, dict):
        if "mcp" in matcher:
            # §6.3 — MCP object: encode as mcp__server__tool
            return encode_mcp_matcher(matcher["mcp"])
        if "pattern" in matcher:
            # §6.2 — pattern object: pass through as-is
            return str(matcher["pattern"])

    if isinstance(matcher, list):
        # §6.4 — array (OR): join resolved names with "|"
        parts: list[str] = []
        for element in matcher:
            if isinstance(element, str):
                parts.append(canonical_tool_to_cc(element))
            elif isinstance(element, dict) and "mcp" in element:
                parts.append(encode_mcp_matcher(element["mcp"]))
            elif isinstance(element, dict) and "pattern" in element:
                parts.append(str(element["pattern"]))
            else:
                # Unknown element — pass through repr as fallback
                parts.append(str(element))
        return "|".join(parts)

    # Fallback: stringify whatever was given
    return str(matcher)


def _encode_handler(
    handler: dict[str, Any],
    provider_data: dict[str, Any] | None,
) -> dict[str, Any]:
    """Build a CC-native handler object from a canonical handler dict.

    Copies ``type``, ``command``, ``timeout``, and ``async``/``status_message``
    (when present). Drops ``cwd``, ``env``, and ``platform`` — Claude Code does
    not support these capabilities. Merges ``provider_data["claude-code"]`` if
    present (§3.6).
    """
    cc_handler: dict[str, Any] = {}

    for field in ("type", "command", "timeout", "async", "status_message"):
        if field in handler:
            cc_handler[field] = handler[field]

    # §3.6 — render provider_data for the target provider
    if provider_data and isinstance(provider_data, dict):
        cc_pd = provider_data.get("claude-code")
        if isinstance(cc_pd, dict):
            cc_handler.update(cc_pd)

    return cc_handler


# ---------------------------------------------------------------------------
# Decode: Claude Code native format → canonical manifest
# ---------------------------------------------------------------------------


def decode(native: dict[str, Any]) -> dict[str, Any]:
    """Convert a Claude Code native hook configuration to a canonical manifest.

    Args:
        native: A dict containing a ``"hooks"`` key with CC-native structure,
            as read from ``settings.json``.

    Returns:
        A canonical hook manifest with ``spec`` and ``hooks`` array. Unknown
        CC fields with no canonical equivalent are preserved in
        ``provider_data["claude-code"]`` on the hook definition.
    """
    canonical_hooks: list[dict[str, Any]] = []

    native_hooks: dict[str, Any] = native.get("hooks", {})
    for cc_event_name, matcher_groups in native_hooks.items():
        canonical_event = REVERSE_EVENT_MAP.get(cc_event_name)
        if canonical_event is None:
            # Unknown CC event — skip; cannot map to canonical.
            continue

        if not isinstance(matcher_groups, list):
            continue

        blocking = cc_event_name in _BLOCKING_EVENTS

        for group in matcher_groups:
            if not isinstance(group, dict):
                continue

            raw_matcher = group.get("matcher")
            hooks_list = group.get("hooks", [])

            if not isinstance(hooks_list, list):
                continue

            for cc_handler in hooks_list:
                if not isinstance(cc_handler, dict):
                    continue

                canonical_hook: dict[str, Any] = {
                    "event": canonical_event,
                }

                # Decode matcher
                decoded_matcher = _decode_matcher(raw_matcher, canonical_event)
                if decoded_matcher is not None:
                    canonical_hook["matcher"] = decoded_matcher

                # Decode handler
                canonical_hook["handler"] = _decode_handler(cc_handler)
                canonical_hook["blocking"] = blocking

                canonical_hooks.append(canonical_hook)

    return {
        "spec": "hooks/0.1",
        "hooks": canonical_hooks,
    }


def _decode_matcher(raw: Any, canonical_event: str) -> Any:
    """Translate a CC-native matcher string to a canonical matcher value.

    Returns:
        A canonical matcher (string, dict, or list) or ``None`` for wildcard
        (empty string or absent matcher).
    """
    # Non-tool events don't use matchers
    if canonical_event not in ("before_tool_execute", "after_tool_execute"):
        return None

    if raw is None or raw == "":
        # Wildcard — omit matcher in canonical form
        return None

    if not isinstance(raw, str):
        return None

    # Check for MCP combined string: mcp__<server>__<tool>
    if raw.startswith("mcp__"):
        return decode_mcp_matcher(raw)

    # Check for array matcher encoded as "A|B|C"
    if "|" in raw:
        parts = [p.strip() for p in raw.split("|") if p.strip()]
        if len(parts) > 1:
            return [_decode_single_tool_matcher(p) for p in parts]
        raw = parts[0] if parts else raw

    return _decode_single_tool_matcher(raw)


def _decode_single_tool_matcher(cc_name: str) -> Any:
    """Resolve a single CC tool name to its canonical form.

    Returns a bare string (canonical tool name) when the CC name is in the
    vocabulary, or a pattern object as a fallback for unrecognized names.
    """
    canonical = cc_tool_to_canonical(cc_name)
    if canonical is not None:
        return canonical
    # Unrecognized CC tool name — preserve as a pass-through pattern
    return {"pattern": cc_name}


def _decode_handler(cc_handler: dict[str, Any]) -> dict[str, Any]:
    """Build a canonical handler dict from a CC-native handler object.

    Copies ``type``, ``command``, ``timeout``, ``async``, and
    ``status_message``. Any remaining fields with no canonical equivalent are
    preserved in the returned dict (forward-compatibility, §3.2).
    """
    canonical_fields = {"type", "command", "timeout", "async", "status_message"}
    handler: dict[str, Any] = {}

    for field in canonical_fields:
        if field in cc_handler:
            handler[field] = cc_handler[field]

    # Preserve unrecognised CC fields as opaque pass-through (§3.2 / §3.6).
    # These are not written into provider_data here; the caller may promote
    # them if implementing Full conformance.
    for field, value in cc_handler.items():
        if field not in canonical_fields:
            handler.setdefault("_cc_extra", {})[field] = value

    return handler
