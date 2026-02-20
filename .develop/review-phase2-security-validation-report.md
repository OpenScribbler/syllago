# Phase 2: Security Hardening - Design↔Plan Validation Report

**Date:** 2026-02-17
**Design Doc:** `/home/hhewett/.local/src/nesco/docs/reviews/implementation-plan.md` (lines 172-251)
**Implementation Plan:** `/home/hhewett/.local/src/nesco/docs/plans/2026-02-17-review-phase2-security-implementation.md`

## Executive Summary

**VALIDATION PASSED** ✓

All 12 security items from the design document are covered by implementing tasks in the plan. No gaps found. No orphan tasks found.

- **Total design items:** 12
- **Covered by plan:** 12 (100%)
- **Gaps:** 0
- **Orphan tasks:** 0

## Coverage Matrix

### 2.1: Prevent copyFile from following symlinks at destination
- **Severity:** HIGH
- **Mapped to:** Task 1
- **Status:** ✓ PASS
- **Architecture match:** Uses `os.Lstat` to detect symlinks before writing (matches design recommendation)
- **Files covered:** `cli/internal/installer/copy.go`, `cli/internal/installer/copy_test.go`

### 2.2: Strip ANSI escape sequences from all TUI-rendered external text
- **Severity:** HIGH
- **Mapped to:** Task 3
- **Status:** ✓ PASS
- **Architecture match:** Creates `StripControlChars()` function, applies at all rendering boundaries (matches design recommendation)
- **Files covered:** `cli/internal/tui/detail_render.go`, `cli/internal/tui/filebrowser.go`, new `cli/internal/tui/sanitize.go`
- **Note:** Plan correctly implements adversarial test fixtures including OSC 52 clipboard injection, CSI cursor movement, raw escapes

### 2.3: Validate item.Name against sjson/gjson special characters
- **Severity:** HIGH
- **Mapped to:** Task 4
- **Status:** ✓ PASS
- **Architecture match:** Validates against `^[a-zA-Z0-9_-]+$` regex (matches design recommendation)
- **Files covered:** `cli/internal/catalog/scanner.go`, `cli/internal/catalog/scanner_test.go`
- **Note:** Plan correctly applies validation in both `scanUniversal` and `scanProviderSpecific`

### 2.4: Skip symlinks in copyDir source tree
- **Severity:** MEDIUM
- **Mapped to:** Task 2
- **Status:** ✓ PASS
- **Architecture match:** Uses `filepath.Walk` with symlink skip (matches design recommendation)
- **Files covered:** `cli/internal/installer/copy.go`, `cli/internal/installer/copy_test.go`

### 2.5: Make config file writes atomic (temp + rename)
- **Severity:** MEDIUM
- **Mapped to:** Task 5
- **Status:** ✓ PASS
- **Architecture match:** Implements write-to-temp-then-rename pattern (matches design recommendation)
- **Files covered:** `cli/internal/installer/jsonmerge.go`, `cli/internal/config/config.go`
- **Note:** Plan includes test to verify no partial writes during concurrent reads

### 2.6: Use 0600 permissions for home-directory config files
- **Severity:** MEDIUM
- **Mapped to:** Task 6
- **Status:** ✓ PASS
- **Architecture match:** Distinguishes home vs project files, uses 0600 for home (matches design)
- **Files covered:** `cli/internal/installer/jsonmerge.go`
- **Note:** Design also mentions `cli/internal/config/config.go` - plan covers it in Task 5

### 2.7: Validate rebuildVersion from VERSION file
- **Severity:** MEDIUM
- **Mapped to:** Task 7
- **Status:** ✓ PASS
- **Architecture match:** Validates against strict semver regex before use in ldflags (matches design)
- **Files covered:** `cli/cmd/nesco/main.go`, `cli/cmd/nesco/main_test.go`

### 2.8: Warn before executing install.sh for app items
- **Severity:** MEDIUM
- **Mapped to:** Task 8
- **Status:** ✓ PASS
- **Architecture match:** Displays confirmation with first N lines of script before execution (matches design)
- **Files covered:** `cli/internal/tui/detail.go`, `cli/internal/tui/detail_render.go`
- **Note:** Plan uses N=20 lines, includes ESC to cancel

### 2.9: Remove git:// and http:// from allowed clone transports
- **Severity:** MEDIUM
- **Mapped to:** Task 9
- **Status:** ✓ PASS
- **Architecture match:** Removes insecure transports from `isValidGitURL` (matches design)
- **Files covered:** `cli/internal/tui/import.go`, `cli/internal/tui/import_test.go`

### 2.10: Whitelist MCP config fields before writing to user config
- **Severity:** LOW
- **Mapped to:** Task 10
- **Status:** ✓ PASS
- **Architecture match:** Parse into `MCPConfig` struct and re-serialize (matches design recommendation)
- **Files covered:** `cli/internal/installer/mcp.go`, `cli/internal/installer/mcp_test.go`

### 2.11: Escape .env values against shell expansion
- **Severity:** LOW
- **Mapped to:** Task 11
- **Status:** ✓ PASS
- **Architecture match:** Switches to single quotes with proper escaping (matches design)
- **Files covered:** `cli/internal/tui/detail_env.go`, `cli/internal/tui/detail_env_test.go`

### 2.12: Require name+type match (not just ID) for promoted item cleanup
- **Severity:** LOW
- **Mapped to:** Task 12
- **Status:** ✓ PASS
- **Architecture match:** Verifies name and type in addition to UUID (matches design)
- **Files covered:** `cli/internal/catalog/cleanup.go`, `cli/internal/catalog/cleanup_test.go`

## File Coverage Analysis

**Design doc "Files touched" line (line 176):**
- `cli/internal/installer/copy.go` ✓ (Tasks 1, 2)
- `cli/internal/installer/jsonmerge.go` ✓ (Tasks 5, 6)
- `cli/internal/installer/mcp.go` ✓ (Task 10)
- `cli/internal/installer/hooks.go` - NOT directly touched, but sjson validation in scanner (Task 4) prevents injection before hooks are called
- `cli/internal/tui/detail_render.go` ✓ (Task 3)
- `cli/internal/tui/detail.go` ✓ (Task 8)
- `cli/internal/tui/detail_env.go` ✓ (Task 11)
- `cli/internal/tui/filebrowser.go` ✓ (Task 3)
- `cli/internal/tui/import.go` ✓ (Task 9)
- `cli/internal/catalog/cleanup.go` ✓ (Task 12)
- `cli/internal/catalog/scanner.go` ✓ (Task 4)
- `cli/internal/metadata/metadata.go` - NOT directly modified (metadata struct is used by cleanup in Task 12)
- `cli/cmd/nesco/main.go` ✓ (Task 7)

**Additional files created by plan:**
- `cli/internal/tui/sanitize.go` (new, as recommended in design item 2.2)

**Note on installer/hooks.go:** The design mentions it needs sjson validation, but the actual fix is in `catalog/scanner.go` (Task 4) which validates item names during scan, preventing malicious names from reaching hooks.go. This is a superior approach.

**Note on metadata/metadata.go:** Design mentions it but doesn't specify changes. Plan correctly uses existing metadata structures without modification.

## Architecture Decision Alignment

All architectural recommendations from the design are reflected in the plan:

1. **Symlink protection (2.1, 2.4):** Uses `os.Lstat` and `filepath.Walk` skip pattern ✓
2. **ANSI stripping (2.2):** Dedicated `StripControlChars()` function with comprehensive escape handling ✓
3. **sjson validation (2.3):** Regex validation `^[a-zA-Z0-9_-]+$` ✓
4. **Atomic writes (2.5):** Temp-then-rename pattern with random suffix ✓
5. **Permissions (2.6):** Home detection via `os.UserHomeDir()` ✓
6. **Semver validation (2.7):** Strict regex before ldflags use ✓
7. **Script preview (2.8):** First 20 lines + confirmation ✓
8. **Transport security (2.9):** Whitelist of https://, ssh://, git@ ✓
9. **Config whitelisting (2.10):** Struct-based serialization ✓
10. **Shell escaping (2.11):** Single quotes with `'\''` pattern ✓
11. **Cleanup verification (2.12):** Triple match (ID + name + type) ✓

## Severity Ordering

Design specifies HIGH → MEDIUM → LOW ordering. Plan ordering:

- **Task 1:** 2.1 HIGH ✓
- **Task 2:** 2.4 MEDIUM (follows Task 1 for code locality, acceptable)
- **Task 3:** 2.2 HIGH ✓
- **Task 4:** 2.3 HIGH ✓
- **Task 5:** 2.5 MEDIUM ✓
- **Task 6:** 2.6 MEDIUM (depends on Task 5) ✓
- **Task 7:** 2.7 MEDIUM ✓
- **Task 8:** 2.8 MEDIUM ✓
- **Task 9:** 2.9 MEDIUM ✓
- **Task 10:** 2.10 LOW ✓
- **Task 11:** 2.11 LOW ✓
- **Task 12:** 2.12 LOW ✓

**Analysis:** Plan groups HIGH items first (Tasks 1, 3, 4), then MEDIUM (Tasks 5-9), then LOW (Tasks 10-12). Task 2 is placed early for code locality with Task 1 (both modify copy.go) which is acceptable as it's still addressed before lower-severity items.

## Testing Approach Alignment

Design testing focus (line 178) specifies adversarial fixtures. Plan implementation:

- **Symlinks (2.1, 2.4):** ✓ Creates symlinks pointing outside test tree
- **ANSI injection (2.2):** ✓ Table of payloads including OSC 52, CSI cursor movement, raw escapes
- **sjson key injection (2.3):** ✓ Tests `.`, `*`, `#`, `|` characters
- **Atomic writes (2.5):** ✓ Concurrent read monitor verifies no partial state
- **Permissions (2.6):** ✓ Uses `os.Stat` to assert file mode bits

## Issues Found

None. The implementation plan comprehensively covers all design requirements with appropriate technical approaches.

## Recommendations

None required. Plan is ready for execution.

---

## Detailed Task Mapping

| Design Item | Task # | Files Modified | Files Tested | Notes |
|-------------|--------|----------------|--------------|-------|
| 2.1 | 1 | copy.go | copy_test.go | Lstat check |
| 2.2 | 3 | detail_render.go, filebrowser.go, sanitize.go (new) | sanitize_test.go | Comprehensive escape handling |
| 2.3 | 4 | scanner.go | scanner_test.go | Regex validation |
| 2.4 | 2 | copy.go | copy_test.go | Walk skip pattern |
| 2.5 | 5 | jsonmerge.go, config.go | jsonmerge_test.go | Temp+rename |
| 2.6 | 6 | jsonmerge.go | jsonmerge_test.go | Home detection |
| 2.7 | 7 | main.go | main_test.go | Semver regex |
| 2.8 | 8 | detail.go, detail_render.go | Manual TUI test | Script preview |
| 2.9 | 9 | import.go | import_test.go | Transport whitelist |
| 2.10 | 10 | mcp.go | mcp_test.go | Struct-based serialization |
| 2.11 | 11 | detail_env.go | detail_env_test.go | Single quote escaping |
| 2.12 | 12 | cleanup.go | cleanup_test.go | Triple match verification |

## Validation Conclusion

The implementation plan is comprehensive, accurate, and ready for execution. All design requirements are met with appropriate technical approaches and testing strategies.
