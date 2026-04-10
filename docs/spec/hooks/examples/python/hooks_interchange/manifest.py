"""Parse and validate canonical hook manifests.

Implements the parsing and validation requirements from the
Hook Interchange Format Specification, Sections 3.1-3.5.

Conformance level: Core (Section 13.1).
"""

from __future__ import annotations

import json
from typing import Any


def parse_manifest(data: str, format: str = "json") -> dict[str, Any]:
    """Parse a JSON or YAML string into a canonical manifest dict.

    Accepts both JSON and YAML representations per §3.1. Produces an
    identical canonical structure from either format.

    Args:
        data: The manifest content as a string.
        format: Either "json" (default) or "yaml".

    Returns:
        The parsed manifest as a plain dict.

    Raises:
        ValueError: If ``format`` is not "json" or "yaml".
        json.JSONDecodeError: If ``format`` is "json" and ``data`` is not
            valid JSON.
        yaml.YAMLError: If ``format`` is "yaml" and ``data`` is not valid
            YAML.
    """
    fmt = format.lower()
    if fmt == "json":
        return json.loads(data)
    elif fmt == "yaml":
        import yaml  # deferred: optional at import time, required at call time

        return yaml.safe_load(data)
    else:
        raise ValueError(f"Unsupported format {format!r}. Expected 'json' or 'yaml'.")


def validate_manifest(manifest: dict[str, Any]) -> list[str]:
    """Validate a parsed canonical hook manifest.

    Checks the required structural constraints from §3.3, §3.4, and §3.5.
    Unknown fields at any level are silently ignored per the forward-
    compatibility requirement in §3.2.

    Args:
        manifest: A dict produced by :func:`parse_manifest` or equivalent.

    Returns:
        A list of error strings. An empty list means the manifest is valid.
    """
    errors: list[str] = []

    # §3.3 — top-level `spec` field
    spec = manifest.get("spec")
    if spec is None:
        errors.append("missing required field 'spec'")
    elif spec != "hooks/0.1":
        errors.append(
            f"invalid 'spec' value {spec!r}: must be \"hooks/0.1\""
        )

    # §3.3 — top-level `hooks` field: must exist, be a list, and be non-empty
    hooks = manifest.get("hooks")
    if hooks is None:
        errors.append("missing required field 'hooks'")
    elif not isinstance(hooks, list):
        errors.append(
            f"'hooks' must be an array, got {type(hooks).__name__}"
        )
    elif len(hooks) == 0:
        # §3.3: "Implementations MUST reject a manifest with an empty `hooks`
        # array as a validation error."
        errors.append("'hooks' array must be non-empty")
    else:
        # §3.4 / §3.5 — validate each hook definition
        for index, hook in enumerate(hooks):
            label = f"Hook {index + 1}"
            hook_errors = _validate_hook(hook, label)
            errors.extend(hook_errors)

    return errors


# ---------------------------------------------------------------------------
# Internal helpers
# ---------------------------------------------------------------------------


def _validate_hook(hook: Any, label: str) -> list[str]:
    """Validate a single hook definition (§3.4 and §3.5).

    Unknown fields are ignored per §3.2.
    """
    errors: list[str] = []

    if not isinstance(hook, dict):
        errors.append(f"{label}: hook definition must be an object")
        return errors

    # §3.4 — `event` is REQUIRED and must be a string
    event = hook.get("event")
    if event is None:
        errors.append(f"{label}: missing required field 'event'")
    elif not isinstance(event, str):
        errors.append(
            f"{label}: 'event' must be a string, got {type(event).__name__}"
        )

    # §3.4 — `handler` is REQUIRED and must be an object
    handler = hook.get("handler")
    if handler is None:
        errors.append(f"{label}: missing required field 'handler'")
    elif not isinstance(handler, dict):
        errors.append(
            f"{label}: 'handler' must be an object, got {type(handler).__name__}"
        )
    else:
        # §3.5 — `handler.type` is REQUIRED
        handler_type = handler.get("type")
        if handler_type is None:
            errors.append(f"{label}: missing required field 'handler.type'")
        elif not isinstance(handler_type, str):
            errors.append(
                f"{label}: 'handler.type' must be a string, "
                f"got {type(handler_type).__name__}"
            )

    return errors
