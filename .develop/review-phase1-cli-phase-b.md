# Phase B: Deep Blocker Analysis

## Task 1A: Write test for --no-color flag disabling ANSI codes
- [x] No implicit dependencies
- [x] Context complete
- [x] No hidden blockers
- [x] No cross-task conflicts
- [x] Success criteria verifiable

## Task 1B: Wire --no-color flag to lipgloss
- [x] No implicit dependencies
- [x] ✅ **VERIFIED**: lipgloss v1.1.1 and termenv v0.16.0 are already in go.mod
- [x] ✅ **VERIFIED**: No subcommands use PersistentPreRunE (grep found zero matches)
- [x] No cross-task conflicts
- [x] Success criteria verifiable

## Task 2A: Add Quiet global and update Print() function
- [x] No implicit dependencies
- [x] Context complete
- [x] No hidden blockers
- [ ] **RACE CONDITION**: Multiple tasks (2A, 3A) modify output.go globals block - could conflict if not done sequentially
- [x] Success criteria verifiable

## Task 2B: Wire --quiet flag in main.go
- [x] No implicit dependencies
- [x] Context complete
- [x] No hidden blockers
- [ ] **CROSS-TASK CONFLICT**: Modifies same PersistentPreRunE function as Task 3B - these must be sequential, not parallel
- [x] Success criteria verifiable

## Task 3A: Add Verbose global and PrintVerbose function
- [x] No implicit dependencies
- [x] Context complete
- [x] No hidden blockers
- [ ] **RACE CONDITION**: Modifies same globals block in output.go as Task 2A - must be sequential
- [x] Success criteria verifiable

## Task 3B: Wire --verbose flag in main.go
- [x] No implicit dependencies
- [x] Context complete
- [x] No hidden blockers
- [ ] **CROSS-TASK CONFLICT**: Modifies same PersistentPreRunE function as Task 2B - these must be sequential
- [x] Success criteria verifiable

## Task 4: Write test for version command with dev builds
- [x] No implicit dependencies
- [x] ✅ **VERIFIED**: No tests use t.Parallel() - package-level variable mutations are safe
- [x] No hidden blockers
- [x] No cross-task conflicts
- [x] Success criteria verifiable

## Task 5: Update info command to use "(dev build)" for version
- [x] No implicit dependencies
- [x] ✅ **VERIFIED**: No tests use t.Parallel() - package-level variable mutations are safe
- [x] No hidden blockers
- [x] No cross-task conflicts
- [x] Success criteria verifiable

## Task 6: Create sentinel error type to prevent duplicate error messages
- [x] No implicit dependencies
- [x] Context complete
- [x] No hidden blockers
- [x] No cross-task conflicts
- [x] Success criteria verifiable

## Task 7: Update main() to skip printing silent errors
- [x] No implicit dependencies
- [ ] **CONTEXT GAP**: Test uses os.Pipe() to capture stderr - needs to ensure this works in test environment and doesn't conflict with other tests
- [x] No hidden blockers
- [ ] **CROSS-TASK CONFLICT**: Modifies main() function which is also modified by Task 20 - must be sequential
- [x] Success criteria verifiable

## Task 8: Update scan command to use SilentError
- [x] No implicit dependencies
- [x] Context complete
- [x] No hidden blockers
- [x] No cross-task conflicts
- [x] Success criteria verifiable

## Task 9: Improve TUI error message when content repo not found
- [x] No implicit dependencies
- [x] ✅ **VERIFIED**: findSkillsDir is only called from findContentRepoRoot (same file) - safe to convert to var
- [ ] **IMPLICIT DEPENDENCY**: Depends on understanding of how runTUI is called and error handling flow
- [x] No hidden blockers
- [x] No cross-task conflicts
- [x] Success criteria verifiable

## Task 10: Add warning when findProjectRoot falls back to CWD
- [x] No implicit dependencies
- [ ] **CONTEXT GAP**: Need to import fmt and output packages in helpers.go - not listed in imports section
- [x] No hidden blockers
- [x] No cross-task conflicts
- [x] Success criteria verifiable

## Task 11: Handle swallowed config.Save error in scan command
- [x] No implicit dependencies
- [x] Context complete
- [x] No hidden blockers
- [ ] **CROSS-TASK CONFLICT**: Modifies scan.go which is also modified by Task 8 - line numbers may shift
- [x] Success criteria verifiable

## Task 12: Add help text mentioning TUI mode
- [x] No implicit dependencies
- [x] Context complete
- [x] No hidden blockers
- [ ] **CROSS-TASK CONFLICT**: Modifies rootCmd Long field which is also modified by Task 21 - must be sequential
- [x] Success criteria verifiable

## Task 13: Add success confirmations for config add and remove
- [x] No implicit dependencies
- [ ] **CONTEXT GAP**: Need to import bytes and strings in config_cmd_test.go - not shown in test setup
- [x] No hidden blockers
- [ ] **CROSS-TASK CONFLICT**: Modifies config_cmd.go which is also modified by Tasks 15A and 15B
- [x] Success criteria verifiable

## Task 14: Wrap bubbletea TTY error with user-facing message
- [x] No implicit dependencies
- [ ] **CONTEXT GAP**: Test verifies error wrapping but doesn't test integration with runTUI - may miss edge cases
- [x] No hidden blockers
- [x] No cross-task conflicts
- [x] Success criteria verifiable

## Task 15A: Add basic slug validation and warning
- [x] No implicit dependencies
- [x] Context complete
- [x] No hidden blockers
- [ ] **CROSS-TASK CONFLICT**: Modifies configAddCmd which is also modified by Task 13 - line numbers will shift
- [x] Success criteria verifiable

## Task 15B: Enhance warning to list all known providers
- [x] No implicit dependencies
- [x] Context complete
- [x] No hidden blockers
- [ ] **CROSS-TASK CONFLICT**: Modifies same warning block as Task 15A - these MUST be done in exact sequence
- [x] Success criteria verifiable

## Task 16: Fix info providers using slugs instead of display names
- [x] No implicit dependencies
- [x] ✅ **VERIFIED**: catalog.ContentType.Label() exists in cli/internal/catalog/types.go:37
- [x] No hidden blockers
- [x] No cross-task conflicts
- [x] Success criteria verifiable

## Task 17: Add provider-to-format mapping in info formats
- [x] No implicit dependencies
- [x] Context complete
- [x] No hidden blockers
- [ ] **CROSS-TASK CONFLICT**: Modifies info.go which is also modified by Tasks 5, 16, and 18 - line numbers will shift
- [x] Success criteria verifiable

## Task 18: Note standalone content types in info output
- [x] No implicit dependencies
- [x] Context complete
- [x] No hidden blockers
- [ ] **CROSS-TASK CONFLICT**: Modifies infoCmd RunE function which is also modified by Task 5
- [x] Success criteria verifiable

## Task 19: Define exit code constants
- [x] No implicit dependencies
- [x] Context complete
- [x] No hidden blockers
- [x] No cross-task conflicts
- [x] Success criteria verifiable

## Task 20: Update main() to use exit code constants
- [x] No implicit dependencies
- [x] Context complete
- [x] No hidden blockers
- [ ] **CROSS-TASK CONFLICT**: Modifies main() function which is also modified by Task 7
- [x] Success criteria verifiable

## Task 21: Document exit codes in root command help
- [x] No implicit dependencies
- [x] Context complete
- [x] No hidden blockers
- [ ] **CROSS-TASK CONFLICT**: Modifies rootCmd Long field which is also modified by Task 12
- [x] Success criteria verifiable

## Task 22: Run full test suite and go vet
- [x] No implicit dependencies
- [x] Context complete
- [x] No hidden blockers
- [x] No cross-task conflicts
- [x] Success criteria verifiable

---

## Summary

### Total Blockers Found: 2 Critical, 12 Minor

**Critical Blockers:**
1. **Tasks 2A & 3A**: Race condition on output.go globals block (must be sequential)
2. **Tasks 15A & 15B**: Direct modification conflict (15B modifies exact same lines as 15A)

**Minor Issues (line number shifts from sequential modifications):**
- Multiple tasks modify PersistentPreRunE (1B, 2B, 3B) - must be sequential
- Multiple tasks modify main() (7, 20) - line number shifts
- Multiple tasks modify rootCmd.Long (12, 21) - line number shifts
- Multiple tasks modify scan.go (8, 11) - line number shifts
- Multiple tasks modify config_cmd.go (13, 15A, 15B) - line number shifts
- Multiple tasks modify info.go (5, 16, 17, 18) - line number shifts
- Missing import statements in several tasks (10, 13)

### Recommended Execution Order Adjustments

**Group 1: Output package foundation (sequential)**
1. Task 2A (Add Quiet global)
2. Task 3A (Add Verbose global)
3. Task 6 (Create SilentError type)
4. Task 19 (Define exit code constants)

**Group 2: Main.go flag wiring (sequential)**
1. Task 1A (Test --no-color flag)
2. Task 1B (Wire --no-color flag) - **verify lipgloss deps first**
3. Task 2B (Wire --quiet flag)
4. Task 3B (Wire --verbose flag)

**Group 3: Main.go error handling (sequential)**
1. Task 7 (Skip printing silent errors in main())
2. Task 20 (Use exit code constants in main())

**Group 4: Version handling (can parallelize)**
- Task 4 (Version command dev build test)
- Task 5 (Info command dev build)

**Group 5: Scan command (sequential)**
1. Task 8 (Use SilentError in scan)
2. Task 11 (Handle config.Save error)

**Group 6: Config command (sequential)**
1. Task 13 (Add success confirmations)
2. Task 15A (Basic slug validation)
3. Task 15B (List known providers in warning) - **depends directly on 15A**

**Group 7: Info command (sequential)**
1. Task 16 (Fix providers display names)
2. Task 17 (Provider-to-format mapping)
3. Task 18 (Note standalone types)

**Group 8: Help text (sequential)**
1. Task 12 (Help text mentioning TUI)
2. Task 21 (Document exit codes in help)

**Group 9: Error messages (can parallelize)**
- Task 9 (TUI error message improvement)
- Task 10 (findProjectRoot fallback warning)
- Task 14 (Wrap bubbletea TTY error)

**Final:**
- Task 22 (Run full test suite)

### Tasks That Should Be Merged

**Merge Task 15A and 15B:**
These tasks modify the exact same warning message in configAddCmd. Doing them separately creates unnecessary churn. Combined task should:
1. Add validation check
2. Print warning with provider list
3. Add both tests

**Merge Task 2B and 3B:**
Both modify PersistentPreRunE. Instead of two separate modifications, do them together:
```go
rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
    // All flag wiring in one place
    noColor, _ := cmd.Flags().GetBool("no-color")
    if noColor || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
        lipgloss.SetColorProfile(termenv.Ascii)
    }
    quiet, _ := cmd.Flags().GetBool("quiet")
    output.Quiet = quiet
    verbose, _ := cmd.Flags().GetBool("verbose")
    output.Verbose = verbose
    return nil
}
```

**Merge Task 12 and 21:**
Both modify rootCmd.Long field. Do as single update:
```go
Long: `Nesco manages AI tool configurations and scans codebases for context that helps AI agents produce correct code.

Run without arguments for interactive mode (TUI). Use subcommands for automation and scripting.

Exit codes: 0=success, 1=error, 2=usage error, 3=drift detected`,
```

### Critical Pre-Flight Checks ✅ ALL COMPLETED

1. ✅ **COMPLETED**: Verified lipgloss v1.1.1 and termenv v0.16.0 in go.mod
2. ✅ **COMPLETED**: Verified no subcommands use PersistentPreRunE
3. ✅ **COMPLETED**: Verified findSkillsDir only called from same file
4. ✅ **COMPLETED**: Verified catalog.ContentType.Label() exists
5. ✅ **COMPLETED**: Verified test suite doesn't use t.Parallel() - package-level mutations safe

### Estimated Impact

**If blockers are not addressed:**
- Tasks 2A/3A could have merge conflicts requiring manual resolution
- Task 15B would overwrite 15A's changes, losing work
- Multiple tasks will have line number mismatches requiring manual adjustment

**With recommended changes:**
- Reduce total commits from 22 to ~16 (by merging related tasks)
- Eliminate 3 guaranteed merge conflicts
- Reduce cognitive load by grouping related changes
- Make review easier with logical groupings
