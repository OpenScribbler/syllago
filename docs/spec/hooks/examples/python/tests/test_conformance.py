"""Conformance test suite for the Hook Interchange Format reference implementation.

Loads shared test vectors from the spec repository and validates that the
Python implementation produces structurally correct results for each case.

Test vector path: four directories up from this file's location, then into
test-vectors/ (tests/ -> python/ -> examples/ -> hooks/ -> test-vectors/).
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

import pytest

from hooks_interchange import manifest as manifest_mod
from hooks_interchange import exit_codes
from hooks_interchange import claude_code
from hooks_interchange import gemini_cli

# ---------------------------------------------------------------------------
# Test vector root
# ---------------------------------------------------------------------------

_TV = Path(__file__).parent.parent.parent.parent / "test-vectors"


def _load(relative: str) -> dict[str, Any]:
    """Load a JSON test vector, stripping _comment and _warnings fields."""
    path = _TV / relative
    raw: dict[str, Any] = json.loads(path.read_text(encoding="utf-8"))
    return _strip_meta(raw)


def _strip_meta(obj: Any) -> Any:
    """Recursively remove _comment and _warnings keys from dicts."""
    if isinstance(obj, dict):
        return {
            k: _strip_meta(v)
            for k, v in obj.items()
            if k not in ("_comment", "_warnings")
        }
    if isinstance(obj, list):
        return [_strip_meta(item) for item in obj]
    return obj


def _normalize_wildcard_matchers(obj: Any) -> None:
    """Recursively replace empty-string matcher fields with absent matcher.

    An empty-string matcher is equivalent to an omitted matcher (wildcard)
    per §6.5. This lets roundtrip comparisons treat both forms as equal.
    Mutates the dict in place.
    """
    if isinstance(obj, dict):
        if obj.get("matcher") == "":
            del obj["matcher"]
        for v in obj.values():
            _normalize_wildcard_matchers(v)
    elif isinstance(obj, list):
        for item in obj:
            _normalize_wildcard_matchers(item)


# ---------------------------------------------------------------------------
# §1  TestManifestParsing
# ---------------------------------------------------------------------------


class TestManifestParsing:
    """Load each canonical test vector, parse it, and confirm it is valid."""

    def test_simple_blocking_parses_without_errors(self) -> None:
        data = _load("canonical/simple-blocking.json")
        errors = manifest_mod.validate_manifest(data)
        assert errors == [], f"Unexpected validation errors: {errors}"

    def test_full_featured_parses_without_errors(self) -> None:
        data = _load("canonical/full-featured.json")
        errors = manifest_mod.validate_manifest(data)
        assert errors == [], f"Unexpected validation errors: {errors}"

    def test_multi_event_parses_without_errors(self) -> None:
        data = _load("canonical/multi-event.json")
        errors = manifest_mod.validate_manifest(data)
        assert errors == [], f"Unexpected validation errors: {errors}"

    def test_degradation_input_rewrite_parses_without_errors(self) -> None:
        data = _load("canonical/degradation-input-rewrite.json")
        errors = manifest_mod.validate_manifest(data)
        assert errors == [], f"Unexpected validation errors: {errors}"

    def test_simple_blocking_has_correct_spec_field(self) -> None:
        data = _load("canonical/simple-blocking.json")
        assert data["spec"] == "hooks/0.1"

    def test_simple_blocking_has_one_hook(self) -> None:
        data = _load("canonical/simple-blocking.json")
        assert len(data["hooks"]) == 1

    def test_multi_event_has_four_hooks(self) -> None:
        data = _load("canonical/multi-event.json")
        assert len(data["hooks"]) == 4

    def test_full_featured_has_three_hooks(self) -> None:
        data = _load("canonical/full-featured.json")
        assert len(data["hooks"]) == 3

    def test_parse_manifest_from_json_string(self) -> None:
        """parse_manifest() should return the same structure as json.loads()."""
        path = _TV / "canonical/simple-blocking.json"
        raw_text = path.read_text(encoding="utf-8")
        result = manifest_mod.parse_manifest(raw_text, format="json")
        assert result["spec"] == "hooks/0.1"
        assert isinstance(result["hooks"], list)

    def test_parse_manifest_rejects_unknown_format(self) -> None:
        with pytest.raises(ValueError, match="Unsupported format"):
            manifest_mod.parse_manifest("{}", format="toml")


# ---------------------------------------------------------------------------
# §2  TestInvalidManifests
# ---------------------------------------------------------------------------


class TestInvalidManifests:
    """Load each invalid test vector and confirm validate_manifest returns errors."""

    def test_empty_hooks_array_is_rejected(self) -> None:
        data = _load("invalid/empty-hooks-array.json")
        errors = manifest_mod.validate_manifest(data)
        assert len(errors) > 0

    def test_empty_hooks_array_error_mentions_hooks(self) -> None:
        data = _load("invalid/empty-hooks-array.json")
        errors = manifest_mod.validate_manifest(data)
        assert any("hooks" in e.lower() for e in errors)

    def test_missing_spec_is_rejected(self) -> None:
        data = _load("invalid/missing-spec.json")
        errors = manifest_mod.validate_manifest(data)
        assert len(errors) > 0

    def test_missing_spec_error_mentions_spec(self) -> None:
        data = _load("invalid/missing-spec.json")
        errors = manifest_mod.validate_manifest(data)
        assert any("spec" in e.lower() for e in errors)

    def test_missing_hooks_field_is_rejected(self) -> None:
        data = _load("invalid/missing-hooks.json")
        errors = manifest_mod.validate_manifest(data)
        assert len(errors) > 0

    def test_missing_event_is_rejected(self) -> None:
        data = _load("invalid/missing-event.json")
        errors = manifest_mod.validate_manifest(data)
        assert len(errors) > 0

    def test_missing_event_error_mentions_event(self) -> None:
        data = _load("invalid/missing-event.json")
        errors = manifest_mod.validate_manifest(data)
        assert any("event" in e.lower() for e in errors)

    def test_missing_handler_is_rejected(self) -> None:
        data = _load("invalid/missing-handler.json")
        errors = manifest_mod.validate_manifest(data)
        assert len(errors) > 0

    def test_missing_handler_error_mentions_handler(self) -> None:
        data = _load("invalid/missing-handler.json")
        errors = manifest_mod.validate_manifest(data)
        assert any("handler" in e.lower() for e in errors)

    def test_valid_manifest_produces_no_errors(self) -> None:
        """Sanity check: the validator does not produce spurious errors."""
        data = _load("canonical/simple-blocking.json")
        errors = manifest_mod.validate_manifest(data)
        assert errors == []


# ---------------------------------------------------------------------------
# §3  TestExitCodes  (truth table from §5.3)
# ---------------------------------------------------------------------------


class TestExitCodes:
    """One test per row of the §5.3 truth table.

    Truth table:
      blocking | exit_code | decision       | result
      ---------|-----------|----------------|------------
      True     | 0         | allow          | ALLOW
      True     | 0         | deny           | BLOCK
      True     | 0         | ask            | ASK
      True     | 0         | None           | ALLOW
      True     | 2         | allow          | BLOCK  (exit code 2 overrides)
      True     | 2         | deny           | BLOCK
      True     | 1         | allow          | WARN_ALLOW
      True     | 1         | None           | WARN_ALLOW
      False    | 0         | allow          | ALLOW
      False    | 0         | deny           | BLOCK
      False    | 0         | None           | ALLOW
      False    | 2         | allow          | WARN_ALLOW (downgraded to 1)
      False    | 2         | deny           | WARN_ALLOW (downgraded to 1)
      False    | 1         | allow          | WARN_ALLOW
      True     | 99        | None           | WARN_ALLOW (non-zero unknown -> 1)
      False    | 99        | None           | WARN_ALLOW (non-zero unknown -> 1)
    """

    D = exit_codes.Decision
    R = exit_codes.Result

    # Blocking hooks — exit code 0
    def test_blocking_exit0_allow_gives_allow(self) -> None:
        assert exit_codes.resolve(True, 0, self.D.ALLOW) == self.R.ALLOW

    def test_blocking_exit0_deny_gives_block(self) -> None:
        assert exit_codes.resolve(True, 0, self.D.DENY) == self.R.BLOCK

    def test_blocking_exit0_ask_gives_ask(self) -> None:
        assert exit_codes.resolve(True, 0, self.D.ASK) == self.R.ASK

    def test_blocking_exit0_no_decision_gives_allow(self) -> None:
        assert exit_codes.resolve(True, 0, None) == self.R.ALLOW

    # Blocking hooks — exit code 2
    def test_blocking_exit2_allow_gives_block(self) -> None:
        """Exit code 2 overrides any JSON decision on blocking hooks."""
        assert exit_codes.resolve(True, 2, self.D.ALLOW) == self.R.BLOCK

    def test_blocking_exit2_deny_gives_block(self) -> None:
        assert exit_codes.resolve(True, 2, self.D.DENY) == self.R.BLOCK

    def test_blocking_exit2_no_decision_gives_block(self) -> None:
        assert exit_codes.resolve(True, 2, None) == self.R.BLOCK

    # Blocking hooks — exit code 1
    def test_blocking_exit1_allow_gives_warn_allow(self) -> None:
        assert exit_codes.resolve(True, 1, self.D.ALLOW) == self.R.WARN_ALLOW

    def test_blocking_exit1_no_decision_gives_warn_allow(self) -> None:
        assert exit_codes.resolve(True, 1, None) == self.R.WARN_ALLOW

    # Non-blocking hooks — exit code 0
    def test_nonblocking_exit0_allow_gives_allow(self) -> None:
        assert exit_codes.resolve(False, 0, self.D.ALLOW) == self.R.ALLOW

    def test_nonblocking_exit0_deny_gives_block(self) -> None:
        assert exit_codes.resolve(False, 0, self.D.DENY) == self.R.BLOCK

    def test_nonblocking_exit0_no_decision_gives_allow(self) -> None:
        assert exit_codes.resolve(False, 0, None) == self.R.ALLOW

    # Non-blocking hooks — exit code 2 (must be downgraded to 1)
    def test_nonblocking_exit2_allow_gives_warn_allow(self) -> None:
        """Non-blocking exit code 2 MUST be downgraded to 1 before evaluation."""
        assert exit_codes.resolve(False, 2, self.D.ALLOW) == self.R.WARN_ALLOW

    def test_nonblocking_exit2_deny_gives_warn_allow(self) -> None:
        assert exit_codes.resolve(False, 2, self.D.DENY) == self.R.WARN_ALLOW

    # Non-blocking hooks — exit code 1
    def test_nonblocking_exit1_allow_gives_warn_allow(self) -> None:
        assert exit_codes.resolve(False, 1, self.D.ALLOW) == self.R.WARN_ALLOW

    # Unknown exit codes are normalised to 1
    def test_blocking_unknown_exit_code_gives_warn_allow(self) -> None:
        """Exit codes outside {0,1,2} are normalised to 1."""
        assert exit_codes.resolve(True, 99, None) == self.R.WARN_ALLOW

    def test_nonblocking_unknown_exit_code_gives_warn_allow(self) -> None:
        assert exit_codes.resolve(False, 99, None) == self.R.WARN_ALLOW


# ---------------------------------------------------------------------------
# §4  TestClaudeCodeVectors
# ---------------------------------------------------------------------------


class TestClaudeCodeVectors:
    """Encode canonical vectors to Claude Code format and compare against expected.

    Also tests decode via the roundtrip vectors.
    """

    def test_encode_simple_blocking(self) -> None:
        canonical = _load("canonical/simple-blocking.json")
        expected = _load("claude-code/simple-blocking.json")
        produced = _strip_meta(claude_code.encode(canonical))
        assert produced == expected

    def test_encode_full_featured(self) -> None:
        canonical = _load("canonical/full-featured.json")
        expected = _load("claude-code/full-featured.json")
        produced = _strip_meta(claude_code.encode(canonical))
        assert produced == expected

    def test_encode_multi_event(self) -> None:
        canonical = _load("canonical/multi-event.json")
        expected = _load("claude-code/multi-event.json")
        produced = _strip_meta(claude_code.encode(canonical))
        assert produced == expected

    def test_encode_simple_blocking_event_key(self) -> None:
        """before_tool_execute must map to PreToolUse."""
        canonical = _load("canonical/simple-blocking.json")
        produced = claude_code.encode(canonical)
        assert "PreToolUse" in produced["hooks"]

    def test_encode_simple_blocking_tool_matcher(self) -> None:
        """Canonical matcher 'shell' must resolve to CC native name 'Bash'."""
        canonical = _load("canonical/simple-blocking.json")
        produced = claude_code.encode(canonical)
        group = produced["hooks"]["PreToolUse"][0]
        assert group["matcher"] == "Bash"

    def test_encode_full_featured_mcp_matcher(self) -> None:
        """MCP matcher must encode as mcp__server__tool (double-underscore format)."""
        canonical = _load("canonical/full-featured.json")
        produced = claude_code.encode(canonical)
        pre_groups = produced["hooks"]["PreToolUse"]
        matchers = [g["matcher"] for g in pre_groups]
        assert "mcp__github__create_issue" in matchers

    def test_encode_multi_event_array_matcher(self) -> None:
        """Array matcher ['shell','file_write'] must encode as pipe-separated 'Bash|Write'."""
        canonical = _load("canonical/multi-event.json")
        produced = claude_code.encode(canonical)
        group = produced["hooks"]["PreToolUse"][0]
        assert group["matcher"] == "Bash|Write"

    def test_roundtrip_decode_source_matches_canonical(self) -> None:
        """Decoding roundtrip-source.json must produce the canonical form."""
        source = _load("claude-code/roundtrip-source.json")
        expected_canonical = _load("claude-code/roundtrip-canonical.json")
        decoded = _strip_meta(claude_code.decode(source))
        assert decoded == expected_canonical

    def test_roundtrip_encode_canonical_matches_source(self) -> None:
        """Re-encoding the decoded canonical must reproduce the original source.

        The re-encoded output may differ from the source only in whether the
        `matcher` field is present for wildcard matchers (absent vs empty
        string) — both forms are equivalent per §6.5.
        """
        source = _load("claude-code/roundtrip-source.json")
        canonical = claude_code.decode(source)
        re_encoded = _strip_meta(claude_code.encode(canonical))
        expected_source = _load("claude-code/roundtrip-source.json")
        # Normalize: empty-string matcher == absent matcher (wildcard per §6.5)
        _normalize_wildcard_matchers(re_encoded)
        _normalize_wildcard_matchers(expected_source)
        assert re_encoded == expected_source

    def test_decode_blocking_inferred_from_pre_tool_use(self) -> None:
        """PreToolUse must decode with blocking=True."""
        source = _load("claude-code/roundtrip-source.json")
        decoded = claude_code.decode(source)
        before_tool_hooks = [
            h for h in decoded["hooks"] if h["event"] == "before_tool_execute"
        ]
        assert all(h["blocking"] is True for h in before_tool_hooks)

    def test_decode_nonblocking_for_session_start(self) -> None:
        """SessionStart must decode with blocking=False."""
        source = _load("claude-code/roundtrip-source.json")
        decoded = claude_code.decode(source)
        session_hooks = [
            h for h in decoded["hooks"] if h["event"] == "session_start"
        ]
        assert all(h["blocking"] is False for h in session_hooks)


# ---------------------------------------------------------------------------
# §5  TestGeminiCliVectors
# ---------------------------------------------------------------------------


class TestGeminiCliVectors:
    """Encode canonical vectors to Gemini CLI format and compare against expected."""

    def test_encode_simple_blocking(self) -> None:
        canonical = _load("canonical/simple-blocking.json")
        expected = _load("gemini-cli/simple-blocking.json")
        produced = _strip_meta(gemini_cli.encode(canonical))
        assert produced == expected

    def test_encode_full_featured(self) -> None:
        canonical = _load("canonical/full-featured.json")
        expected = _load("gemini-cli/full-featured.json")
        produced = _strip_meta(gemini_cli.encode(canonical))
        assert produced == expected

    def test_encode_multi_event(self) -> None:
        canonical = _load("canonical/multi-event.json")
        expected = _load("gemini-cli/multi-event.json")
        produced = _strip_meta(gemini_cli.encode(canonical))
        assert produced == expected

    def test_encode_simple_blocking_event_trigger(self) -> None:
        """before_tool_execute must map to BeforeTool."""
        canonical = _load("canonical/simple-blocking.json")
        produced = gemini_cli.encode(canonical)
        triggers = [entry["trigger"] for entry in produced["hooks"]]
        assert "BeforeTool" in triggers

    def test_encode_simple_blocking_tool_matcher(self) -> None:
        """Canonical matcher 'shell' must resolve to 'run_shell_command'."""
        canonical = _load("canonical/simple-blocking.json")
        produced = gemini_cli.encode(canonical)
        entry = produced["hooks"][0]
        assert entry["toolMatcher"] == "run_shell_command"

    def test_encode_simple_blocking_timeout_converted_to_ms(self) -> None:
        """Canonical timeout (seconds) must be multiplied by 1000 for Gemini CLI."""
        canonical = _load("canonical/simple-blocking.json")
        produced = gemini_cli.encode(canonical)
        entry = produced["hooks"][0]
        assert entry["timeoutMs"] == 10000

    def test_encode_full_featured_mcp_matcher_single_underscore(self) -> None:
        """Gemini CLI MCP format uses single-underscore: mcp_server_tool."""
        canonical = _load("canonical/full-featured.json")
        produced = gemini_cli.encode(canonical)
        tool_matchers = [
            entry.get("toolMatcher")
            for entry in produced["hooks"]
            if "toolMatcher" in entry
        ]
        assert "mcp_github_create_issue" in tool_matchers

    def test_encode_multi_event_array_expands_to_multiple_entries(self) -> None:
        """Array matcher ['shell','file_write'] must expand to two separate hook entries."""
        canonical = _load("canonical/multi-event.json")
        produced = gemini_cli.encode(canonical)
        before_tool_entries = [
            e for e in produced["hooks"] if e["trigger"] == "BeforeTool"
        ]
        assert len(before_tool_entries) == 2

    def test_encode_multi_event_wildcard_omits_tool_matcher(self) -> None:
        """Wildcard matcher (omitted) must produce an entry without toolMatcher."""
        canonical = _load("canonical/multi-event.json")
        produced = gemini_cli.encode(canonical)
        after_tool_entries = [
            e for e in produced["hooks"] if e["trigger"] == "AfterTool"
        ]
        assert len(after_tool_entries) == 1
        assert "toolMatcher" not in after_tool_entries[0]
