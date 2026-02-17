# Phase B Analysis: review-phase2-security

Generated: 2026-02-17
Tasks analyzed: 12

## Task 1: Prevent copyFile from following symlinks at destination
- [x] Implicit deps: None
- [x] Missing context: None - implementation is complete with clear test and fix
- [x] Hidden blockers: None - uses only stdlib
- [x] Cross-task conflicts: Task 2 modifies the same file (copy.go), but edits different functions (copyFile vs copyDir) - no conflict
- [x] Success criteria:
  - copyFile function returns error when os.Lstat detects symlink at destination (mode & os.ModeSymlink != 0)
  - Test TestCopyFile_RefusesSymlinkDestination passes
  - Test TestCopyFile_WorksForNormalFiles passes
  - Error message contains "symlink" substring

**Actions taken:**
- None required

## Task 2: Skip symlinks in copyDir source tree
- [x] Implicit deps: Task 1 (depends on copyFile being secure, since copyDir calls copyFile)
- [x] Missing context: None - implementation is complete
- [x] Hidden blockers: None
- [x] Cross-task conflicts: Modifies copy.go (same file as Task 1), but different function - no conflict. Calls copyFile which Task 1 secures.
- [x] Success criteria:
  - copyDir function checks info.Mode()&os.ModeSymlink != 0 and returns nil (skip) for symlinks
  - Test TestCopyDir_SkipsSymlinksInSource passes (verifies sneaky.txt symlink not copied)
  - Test TestCopyDir_NormalDirectories passes
  - Symlink files do not appear in destination directory

**Actions taken:**
- None required

## Task 3: Strip ANSI escape sequences from TUI-rendered text
- [x] Implicit deps: None
- [x] Missing context: Agent needs to know which specific rendering locations to apply StripControlChars. Plan provides 4 locations (detail_render.go lines 580, 589, 605, 618, 637; filebrowser.go line 650), but implementation requires reading code to understand context.
- [x] Hidden blockers: None - implementation uses only stdlib (strings, unicode/utf8)
- [x] Cross-task conflicts: None. Creates new file sanitize.go, modifies detail_render.go and filebrowser.go which no other tasks touch.
- [x] Success criteria:
  - StripControlChars function exists in sanitize.go and passes all 8 test cases in TestStripControlChars
  - OSC 52 clipboard injection test passes (sequence stripped)
  - CSI cursor movement test passes (sequence stripped)
  - All external text in TUI calls StripControlChars before rendering (item names, descriptions, file content, filenames)
  - Unicode characters preserved in test

**Actions taken:**
- None required

## Task 4: Validate item.Name against sjson special characters
- [x] Implicit deps: None
- [x] Missing context: Agent must identify TWO functions to modify (scanUniversal AND scanProviderSpecific). Plan shows both but agent must read scanner.go to understand the two-path architecture.
- [x] Hidden blockers: None - uses stdlib regexp
- [x] Cross-task conflicts: None. Only modifies scanner.go which no other tasks touch.
- [x] Success criteria:
  - isValidItemName function exists and uses regex ^[a-zA-Z0-9_-]+$
  - Both scanUniversal (line ~66) and scanProviderSpecific (line ~150) call isValidItemName and skip invalid names
  - Test TestScan_RejectsInvalidItemNames passes (expects 1 valid item from 6 total)
  - Test TestScan_AcceptsValidItemNames passes (expects 5 items)
  - Directories with names containing . * # | are not added to catalog

**Actions taken:**
- None required

## Task 5: Make config file writes atomic (temp + rename)
- [x] Implicit deps: None
- [x] Missing context: Plan modifies TWO files (jsonmerge.go AND config.go) but agent must realize config.Save needs the same atomic pattern
- [x] Hidden blockers: Requires crypto/rand and encoding/hex imports (not mentioned in plan but shown in implementation)
- [x] Cross-task conflicts: Task 6 depends on Task 5 and modifies the SAME function (writeJSONFile). BLOCKER: Task 6 must run after Task 5 or they will conflict.
- [x] Success criteria:
  - writeJSONFile uses temp file with random suffix (crypto/rand, hex.EncodeToString)
  - writeJSONFile performs os.Rename from temp to target
  - config.Save uses temp-then-rename pattern
  - Test TestWriteJSONFile_Atomic passes
  - Test TestWriteJSONFile_NoPartialWrites passes (monitor goroutine never sees empty/partial file)
  - os.Remove called on temp file if rename fails

**Actions taken:**
- None required (dependency already stated in plan)

## Task 6: Use 0600 permissions for home-directory config files
- [x] Implicit deps: Task 5 (CRITICAL - modifies same function writeJSONFile created in Task 5)
- [x] Missing context: Agent must understand "home file detection" logic (check if path starts with UserHomeDir + separator)
- [x] Hidden blockers: None
- [x] Cross-task conflicts: CRITICAL - Modifies writeJSONFile which Task 5 creates. Plan states "Depends on: Task 5" but this is architectural dependency (same function body). Tasks 5 and 6 should be combined OR Task 6 must strictly follow Task 5.
- [x] Success criteria:
  - writeJSONFile detects home directory paths using os.UserHomeDir and filepath.Separator prefix check
  - writeJSONFileWithPerm function exists and accepts perm parameter
  - Home files written with 0600 mode (os.FileMode)
  - Project files written with 0644 mode
  - Test TestWriteJSONFile_Permissions passes (home file has 0600)
  - Test TestWriteJSONFile_ProjectPermissions passes (project file has 0644)
  - os.Chmod called after rename to ensure permissions set correctly

**Actions taken:**
- None required (dependency already stated)

## Task 7: Validate VERSION file content before use in ldflags
- [x] Implicit deps: None
- [x] Missing context: Agent must locate ensureUpToDate function in main.go (currently line 254) to insert validation. Plan provides function name but not line number.
- [x] Hidden blockers: None - uses stdlib regexp
- [x] Cross-task conflicts: None. Only modifies main.go which no other tasks touch.
- [x] Success criteria:
  - validateVersion function exists with semver regex pattern
  - Regex matches valid semver (0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)...
  - Regex accepts prerelease (1.0.0-alpha) and build metadata (1.0.0+build.123)
  - Regex rejects non-semver (empty, "v1.0.0", "1.0", injection attempts)
  - ensureUpToDate calls validateVersion before using rebuildVersion in ldflags
  - All 12 test cases in TestValidateVersion pass
  - Returns error with message "invalid version format" for bad input

**Actions taken:**
- None required

## Task 8: Warn before executing install.sh for app items
- [x] Implicit deps: None
- [x] Missing context: Agent must understand the TUI state machine. Plan shows adding actionAppScriptConfirm but agent must read detail.go Update() method to understand where to intercept "i" key press (currently around line 456).
- [x] Hidden blockers: None
- [x] Cross-task conflicts: None. Modifies detail.go and detail_render.go which no other tasks touch (Task 3 modifies detail_render.go but different functions).
- [x] Success criteria:
  - actionAppScriptConfirm constant added to detailAction enum
  - appScriptPreview field added to detailModel struct
  - First "i" press reads install.sh, sets m.appScriptPreview (first 20 lines), sets confirmAction=actionAppScriptConfirm
  - Second "i" press executes m.runAppScript("install")
  - ESC during preview sets confirmAction=actionNone and clears appScriptPreview
  - Preview rendering shows WARNING, script preview with "---" borders, and help text "Press i again to execute, esc to cancel"
  - StripControlChars applied to script preview content

**Actions taken:**
- None required

## Task 9: Remove git:// and http:// from allowed clone transports
- [x] Implicit deps: None
- [x] Missing context: None - implementation is straightforward
- [x] Hidden blockers: None
- [x] Cross-task conflicts: None. Only modifies import.go which no other tasks touch.
- [x] Success criteria:
  - isValidGitURL function (currently line 947) modified to only accept https://, ssh://, git@
  - Function returns false for git:// and http:// URLs
  - Test TestIsValidGitURL passes with 9 test cases
  - ext:: transport remains blocked
  - URLs starting with - remain blocked

**Actions taken:**
- None required

## Task 10: Whitelist MCP config fields before writing to user config
- [x] Implicit deps: None
- [x] Missing context: Agent must convert mcpConfigPath from function to var for testing (following pattern from Phase 1). Plan shows this but agent must understand the testing pattern.
- [x] Hidden blockers: Requires github.com/tidwall/gjson import for test verification (already in use). Agent must understand MCPConfig struct fields (Type, Command, Args, URL, Env) are the whitelist.
- [x] Cross-task conflicts: None. Only modifies mcp.go (installer package) which no other tasks touch.
- [x] Success criteria:
  - installMCP parses config.json into MCPConfig struct
  - installMCP re-serializes via json.Marshal to drop unknown fields
  - Only whitelisted fields (type, command, args, url, env) appear in written config
  - Test TestInstallMCP_WhitelistsFields passes
  - malicious_field, unexpected_key, _internal_config are not present in final config
  - _romanesco marker is present (added after serialization)

**Actions taken:**
- None required

## Task 11: Escape .env values against shell expansion
- [x] Implicit deps: None
- [x] Missing context: None - implementation is straightforward single-quote escaping
- [x] Hidden blockers: None
- [x] Cross-task conflicts: None. Only modifies detail_env.go which no other tasks touch.
- [x] Success criteria:
  - saveEnvToFile (line 67) uses single quotes for values: KEY='value'
  - Single quotes in value are escaped using '\'' pattern
  - Test TestSaveEnvToFile_Escaping passes with 6 test cases
  - Dollar signs, backticks, double quotes, command substitution are not expanded (literal in single quotes)
  - Test verifies format starts with "KEY='" prefix

**Actions taken:**
- None required

## Task 12: Require name+type match for promoted item cleanup
- [x] Implicit deps: None
- [x] Missing context: Agent must change data structure from map[string]bool (just IDs) to map[string]sharedItem struct to track name+type. Plan shows this but agent must understand the architectural change.
- [x] Hidden blockers: None
- [x] Cross-task conflicts: None. Only modifies cleanup.go which no other tasks touch.
- [x] Success criteria:
  - CleanupPromotedItems builds sharedByID map containing sharedItem{ID, Name, Type}
  - Function checks ID match AND shared.Name == item.Name AND shared.Type == item.Type
  - Test TestCleanupPromotedItems_RequiresNameAndTypeMatch passes (0 items cleaned on name mismatch)
  - Test TestCleanupPromotedItems_RequiresTypeMatch passes (0 items cleaned on type mismatch)
  - Test TestCleanupPromotedItems_CleansExactMatches passes (1 item cleaned on exact match)
  - Local item directory remains if name or type doesn't match

**Actions taken:**
- None required

## Summary
- Total tasks: 12
- Dependencies added: 0 (all explicit dependencies already in plan)
- New beads created: 0
- Plan updates made: 0
- Success criteria added: 12 (all tasks now have measurable criteria)

### Key Findings

**Critical Dependencies:**
- Task 6 architecturally depends on Task 5 (modifies same function) - already documented in plan
- Task 2 logically depends on Task 1 (calls secured copyFile) - already documented in plan

**No Hidden Blockers Found:**
All external dependencies are standard library. No undocumented third-party packages required.

**No Cross-Task Conflicts:**
Each task modifies distinct files or distinct functions within shared files. Task 5→6 dependency is the only overlapping modification, already documented.

**Missing Context (Minor):**
- Task 3: Agent must identify all TUI rendering locations by reading code
- Task 4: Agent must understand two-path scanner architecture (universal vs provider-specific)
- Task 7: Agent must locate ensureUpToDate function (no line number given)
- Task 8: Agent must understand TUI state machine to intercept key press
- Task 10: Agent must understand testing pattern (var assignment for mocking)
- Task 12: Agent must understand architectural change (map[string]bool → map[string]sharedItem)

These are not blockers - they require normal code reading which an agent would do anyway.

**Success Criteria Quality:**
All tasks now have specific, measurable success criteria including:
- Function signatures and regex patterns
- Test names and expected outcomes
- Specific data validations (file modes, struct fields, error messages)
- Behavioral requirements (what gets stripped/blocked/validated)

**Plan Assessment:**
The implementation plan is comprehensive and well-structured. No hidden dependencies or blockers discovered. The plan is ready for execution in the documented task order (1-12), respecting the Task 5→6 dependency.
