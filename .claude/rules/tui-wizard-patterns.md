# Wizard Step Machine Pattern

TUI wizards use typed step enums with deterministic transitions enforced by `validateStep()` assertions.

## Pattern Overview

Each wizard model has:
1. A typed step enum (e.g., `importStep`, `clStep`, `installStep`) defined with `iota`
2. A `validateStep()` method that checks entry-prerequisites for the current step
3. A call to `validateStep()` at the top of `Update()`

## validateStep() Scope

**Checks entry-prerequisites ONLY.** These are conditions that MUST be true when entering a step — programmer errors if violated.

**Does NOT check:**
- Cursor positions or UI state (mutated by App before calling sub-model Update)
- Step-output state (data produced by message handlers at the current step)
- Constructor state (validated by the constructor itself)

**Panics with descriptive messages:**
```go
panic("wizard invariant: stepProvider entered with empty providerNames")
```

## Placement

- **Full-screen wizards** (importModel, createLoadoutScreen, updateModel): call `validateStep()` unconditionally at the top of `Update()`
- **Modal wizards** (envSetupModal, installModal): call `validateStep()` AFTER the `if !m.active { return }` guard

## Covered Wizards (5 total)

| File | Model | Steps |
|------|-------|-------|
| import.go | importModel | 15 |
| loadout_create.go | createLoadoutScreen | 6 |
| modal.go | installModal | 3 |
| modal.go | envSetupModal | 4 |
| update.go | updateModel | 4 |

## Test Enforcement

All wizard invariants are tested in `wizard_invariant_test.go`:
- Forward-path tests verify step transitions without panics
- Esc/back-path tests verify asymmetric back-navigation
- Special case tests cover conditional branches
- Parallel array tests verify correlated arrays stay in sync
- Conflict/batch tests cover the import conflict resolution paths

The PostToolUse hook (`wizard-invariant-gate.sh`) runs these tests after any TUI file edit.

## Checklist for Adding a New Wizard

1. Define step enum with `iota`
2. Add `validateStep()` method with entry-prerequisites per step
3. Call `validateStep()` at top of `Update()` (after active guard for modals)
4. Add forward-path tests to `wizard_invariant_test.go`
5. Add Esc/back-path tests
6. Add special case tests for conditional branches
7. Add parallel array tests if the wizard uses correlated slices
