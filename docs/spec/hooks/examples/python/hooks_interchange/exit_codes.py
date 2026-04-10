"""Exit code and decision resolution for hook handlers.

Implements the evaluation order and truth table from the
Hook Interchange Format Specification, Sections 4 and 5.

Conformance level: Extended (Section 8.2).
"""

from __future__ import annotations

from enum import Enum


class Decision(Enum):
    """Structured JSON ``decision`` values from §5.1."""

    ALLOW = "allow"
    DENY = "deny"
    ASK = "ask"


class Result(Enum):
    """Final resolved outcome after applying §5.3 evaluation order."""

    ALLOW = "allow"
    BLOCK = "block"
    WARN_ALLOW = "warn_allow"
    ASK = "ask"


def resolve(blocking: bool, exit_code: int, decision: Decision | None) -> Result:
    """Resolve the combined exit code and JSON decision into a final result.

    Applies the two-step evaluation order mandated by §5.3:

    1. **Non-blocking downgrade** (§4, §5.3 step 1): when ``blocking`` is
       ``False``, exit code 2 MUST be treated as exit code 1 before any
       further evaluation.
    2. **Truth table lookup** (§5.3 step 2): apply the precedence rules from
       §5.2 using the (possibly downgraded) exit code and ``decision``.

    Exit codes outside {0, 1, 2} are normalised to 1 per §4 ("Other: same
    behaviour as exit code 1").

    When ``decision`` is ``None`` (absent from the JSON output) and exit code
    is 0, the result is ``ALLOW`` per §5.2 ("When the ``decision`` field is
    absent and exit code is 0, the implementation MUST treat the result as
    ``decision: 'allow'``").

    Note on invalid JSON stdout: when a hook exits with code 0 and stdout is
    not valid JSON, callers MUST substitute exit code 1 before calling this
    function and pass ``decision=None``. This function does not receive raw
    stdout; JSON parsing and the exit-code substitution are the caller's
    responsibility (§5.2: "implementations MUST treat the result as exit
    code 1").

    Truth table (§5.3):

    +-----------+-----------+------------+-------------------------------+
    | blocking  | exit_code | decision   | result                        |
    +===========+===========+============+===============================+
    | True      | 0         | allow      | ALLOW                         |
    +-----------+-----------+------------+-------------------------------+
    | True      | 0         | deny       | BLOCK                         |
    +-----------+-----------+------------+-------------------------------+
    | True      | 0         | ask        | ASK                           |
    +-----------+-----------+------------+-------------------------------+
    | True      | 0         | None       | ALLOW                         |
    +-----------+-----------+------------+-------------------------------+
    | True      | 2         | allow      | BLOCK  (exit code 2 overrides)|
    +-----------+-----------+------------+-------------------------------+
    | True      | 2         | deny       | BLOCK                         |
    +-----------+-----------+------------+-------------------------------+
    | True      | 1         | any        | WARN_ALLOW                    |
    +-----------+-----------+------------+-------------------------------+
    | False     | 0         | allow      | ALLOW                         |
    +-----------+-----------+------------+-------------------------------+
    | False     | 0         | deny       | BLOCK                         |
    +-----------+-----------+------------+-------------------------------+
    | False     | 0         | None       | ALLOW                         |
    +-----------+-----------+------------+-------------------------------+
    | False     | 2         | any        | WARN_ALLOW  (downgraded to 1) |
    +-----------+-----------+------------+-------------------------------+
    | False     | 1         | any        | WARN_ALLOW                    |
    +-----------+-----------+------------+-------------------------------+

    Args:
        blocking: Whether the hook definition has ``blocking: true``.
        exit_code: The raw exit code returned by the hook process.
        decision: The parsed ``decision`` field from the JSON output, or
            ``None`` when the field is absent or stdout was empty/invalid.

    Returns:
        The resolved ``Result``.
    """
    # Normalise exit codes outside {0, 1, 2} to 1 (§4 "Other: hook error").
    if exit_code not in (0, 1, 2):
        exit_code = 1

    # Step 1 — non-blocking downgrade (§5.3 step 1).
    # When blocking is False, exit code 2 MUST be treated as exit code 1
    # before any further evaluation.
    if not blocking and exit_code == 2:
        exit_code = 1

    # Step 2 — truth table lookup (§5.3 step 2).

    # exit_code 1 (including downgraded 2) → WARN_ALLOW regardless of decision.
    if exit_code == 1:
        return Result.WARN_ALLOW

    # exit_code 2 (only reachable when blocking is True, after step 1).
    # Exit code 2 overrides any JSON decision — the action is always blocked.
    if exit_code == 2:
        return Result.BLOCK

    # exit_code 0: decision field drives the result.
    assert exit_code == 0  # noqa: S101 — unreachable otherwise

    if decision is None or decision is Decision.ALLOW:
        return Result.ALLOW
    if decision is Decision.DENY:
        return Result.BLOCK
    if decision is Decision.ASK:
        return Result.ASK

    # Exhaustive enum coverage — unreachable with a well-formed Decision value.
    raise ValueError(f"Unhandled decision value: {decision!r}")  # pragma: no cover
