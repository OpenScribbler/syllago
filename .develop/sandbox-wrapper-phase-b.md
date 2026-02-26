# Sandbox Wrapper — Phase B Deep Analysis

**Date:** 2026-02-25
**Plan source:** `docs/plans/2026-02-25-sandbox-wrapper-implementation.md`
**Codebase root:** `cli/` (module `github.com/OpenScribbler/nesco/cli`)

---

## Task 1.1: Directory safety validation

- [x] Dependencies complete — none required
- [x] Context sufficient — all imports present (`fmt`, `os`, `path/filepath`, `strings`)
- [x] No hidden blockers — pure stdlib, no external tools
- [x] No cross-task conflicts
- [x] Success criteria verifiable — five named test cases with clear pass/fail semantics
- [x] Code compiles — all types and functions are self-contained

**Note:** The plan checks `.git` for `IsDir()` and skips gitdir-file worktrees via `continue`. However, the `--force-dir` bypass warning is only printed in the runner (`runner.go` step 2) rather than in `ValidateDir` itself, so the `dirsafety_test.go` for `TestValidateDir_ForceDir` cannot verify a warning — the success criterion says "always returns nil" which is accurate and testable.

---

## Task 1.2: Environment variable filter

- [x] Dependencies complete — none required
- [x] Context sufficient — all imports present (`os`, `strings`)
- [x] No hidden blockers
- [x] No cross-task conflicts
- [x] Success criteria verifiable — five named test cases, clear pass/fail
- [x] Code compiles — all types used are stdlib

---

## Task 1.3: Extend config schema with sandbox settings

- [x] Dependencies complete — none required
- [x] Context sufficient — the plan shows the exact struct additions
- [x] No hidden blockers
- [ ] BLOCKER: **Schema mismatch with existing `Config` struct.** The plan shows:
  ```go
  type Config struct {
      Providers   []string          `json:"providers"`
      Registries  []Registry        `json:"registries,omitempty"`
      Preferences map[string]string `json:"preferences,omitempty"`
      Sandbox     SandboxConfig     `json:"sandbox,omitempty"`
  }
  ```
  But the actual `cli/internal/config/config.go` `Config` struct has:
  ```go
  type Config struct {
      Providers   []string          `json:"providers"`
      Registries  []Registry        `json:"registries,omitempty"`
      Preferences map[string]string `json:"preferences,omitempty"`
  }
  ```
  The executor needs to add `SandboxConfig` type *and* add the `Sandbox` field to the existing struct, not replace the whole struct. The plan doesn't state this explicitly — it presents the final struct as if it already has the field, which could confuse an executor into replacing rather than patching.
- [x] No cross-task conflicts
- [x] Success criteria verifiable
- [x] Code compiles once the struct is actually modified

**Fix:** The plan should explicitly state: "Add `SandboxConfig` struct before the `Config` struct, then add the `Sandbox SandboxConfig` field to the existing `Config` struct." Avoid implying a full replacement.

---

## Task 2.1: HTTP CONNECT egress proxy

- [x] Dependencies complete — Task 1.2 listed (package cohesion only, not a true dependency)
- [x] Context sufficient — all imports listed (`bufio`, `fmt`, `io`, `log`, `net`, `net/http`, `strings`, `sync`)
- [x] No hidden blockers
- [x] No cross-task conflicts
- [ ] BLOCKER: **Success criteria references `goleak` but it is not in `go.mod`.** The criterion says "use `goleak` or manual `Shutdown()` → conn refused check." `goleak` (`go.uber.org/goleak`) is not a declared dependency in `cli/go.mod`. The plan says "No new Go dependencies." This is a contradiction. The executor must use the manual approach only.
- [x] Code compiles — all imports present and used

**Fix:** Remove the `goleak` option from the success criterion. State "verify with manual `Shutdown()` → `net.Dial` returns error" only.

---

## Task 2.2: Socat bridge

- [x] Dependencies complete — Task 2.1 listed (same package)
- [x] Context sufficient
- [ ] BLOCKER: **`bridge.go` is missing `"strings"` import.** The `shellescape` function uses `strings.ReplaceAll` but the import block only lists `"fmt"`, `"os"`, `"path/filepath"`. The code will not compile.
  ```go
  import (
      "fmt"
      "os"
      "path/filepath"
      // MISSING: "strings"
  )
  ```
- [ ] BLOCKER: **`shellescape` is defined in `bridge.go` but also used in `gitwrapper.go`.** Both files are in the same `sandbox` package, so the function is accessible, but defining a shared helper in `bridge.go` creates a non-obvious coupling. When `gitwrapper.go` is created in Task 3.2 (which also has no `shellescape` definition in its snippet), the executor may try to define it again, causing a "already declared" compile error. The plan should explicitly state that `shellescape` is defined in `bridge.go` and is available package-wide.
- [x] No cross-task conflicts once the above is understood
- [x] Success criteria verifiable

**Fix:** Add `"strings"` to the import block in the `bridge.go` snippet. Add a note: "`shellescape` is defined in `bridge.go` and available to all files in `package sandbox`."

---

## Task 3.1: Provider mount profiles

- [x] Dependencies complete — Task 1.3 (config schema) and `cli/internal/provider` package noted
- [x] Context sufficient — the plan accurately describes the `provider` package structure
- [x] No hidden blockers
- [ ] BLOCKER: **Provider slug mismatch.** `profile.go` defines a `case "copilot":` but the `cli/internal/provider/copilot.go` defines `Slug: "copilot-cli"`. An executor who runs `nesco sandbox run copilot-cli` will get "unknown provider" because the profile switch has `"copilot"` not `"copilot-cli"`. The slug must match exactly what providers report.
- [ ] BLOCKER: **`windsurf` provider is missing.** The `cli/internal/provider/windsurf.go` defines `Slug: "windsurf"` and it is listed in `AllProviders`. There is no `windsurf` case in `ProfileFor`. While a decision could be made to omit it, the plan doesn't mention this omission — an executor will not know whether to add it or leave it out.
- [x] No cross-task conflicts
- [x] Success criteria verifiable — the six tests are unit-level and testable without real binaries for ecosystem domain/cache tests
- [ ] BLOCKER: **`TestProfileFor_*` tests for known providers (`claude-code`, `gemini-cli`, etc.) will fail in CI if the provider binary is not installed.** `ProfileFor` calls `resolveBinary` which calls `exec.LookPath`. The tests listed only test the `UnknownProvider` error case. If an executor tries to add a positive test for a specific provider (which the success criteria doesn't require but a thorough executor might), they'll hit environment-dependent failures. The success criteria should clarify that provider-specific tests are integration-level and require the binary.

**Fix:** Change `"copilot"` to `"copilot-cli"` in the `ProfileFor` switch. Add a `windsurf` case (even as a stub returning an error "windsurf does not support sandbox in v1"). Clarify that the six listed tests do not include positive provider resolution tests.

---

## Task 3.2: Git subcommand allowlist wrapper generator

- [x] Dependencies complete — none listed, standalone
- [x] Context sufficient
- [ ] BLOCKER: **`gitwrapper.go` is missing `"fmt"` import.** The `GitWrapperScript` function uses `fmt.Sprintf` to build the blocked commands string and the script template, but the import block only lists `"os"` and `"path/filepath"`. The code will not compile.
  ```go
  import (
      "os"
      "path/filepath"
      // MISSING: "fmt"
  )
  ```
- [ ] BLOCKER: **`shellescape` is used in `gitwrapper.go` but not defined there.** This function is defined in `bridge.go` (Task 2.2). The plan does not state this dependency. An executor doing Task 3.2 independently of Task 2.2 will see an undefined reference.
- [x] No cross-task conflicts
- [x] Success criteria verifiable
- [x] Code compiles once the two import issues and the `shellescape` dependency are resolved

**Fix:** Add `"fmt"` to `gitwrapper.go` imports. Add a dependency note: "Depends on Task 2.2 (`bridge.go`) for the `shellescape` helper, or define `shellescape` in a shared file."

---

## Task 4.1: Config staging, diff, and approval

- [x] Dependencies complete — none listed, standalone
- [x] Context sufficient — import block is complete except for one missing item (see below)
- [x] No hidden blockers
- [ ] BLOCKER: **`configdiff.go` is missing `"strings"` import.** The `buildDiff` function uses `strings.Builder`, `strings.Split`, `strings.Contains`, and `unifiedDiff` uses `strings.Builder` and `strings.Split`. But the import block only lists `"crypto/sha256"`, `"fmt"`, `"io"`, `"io/fs"`, `"os"`, `"path/filepath"`. The code will not compile.
- [ ] ISSUE: **`unifiedDiff` is a naive non-standard diff.** The function emits all original lines as `-` and all new lines as `+` rather than a true line-level diff. This means even a single character change will show the entire file as replaced. The success criterion `TestComputeDiffs_DirDiff_ShowsChangedFiles` will pass, but the plan calls the output "human-readable unified diff" which is misleading — it is not a unified diff. Executors may notice this and spend time improving it or may file a bug later. This is a documentation issue not a blocker, but worth flagging.
- [ ] ISSUE: **`buildDiff` is called on directory paths using `snap.OriginalPath` as the "original" side.** When comparing a staged copy to the original, `buildDiff(snap.OriginalPath, snap.StagedPath)` reads original data from the live original path — not from the pre-session hash. If the original changed on disk during the session (unlikely but possible), the diff will be misleading. This matches the design intent but is worth documenting.
- [x] No cross-task conflicts — this is the only file touching staging logic
- [x] Success criteria verifiable — ten named test cases are clear and all unit-testable

**Fix:** Add `"strings"` to the `configdiff.go` import block.

---

## Task 5.1: Bubblewrap argument construction

- [x] Dependencies complete — Tasks 3.1, 3.2, 4.1 listed
- [x] Context sufficient
- [x] No hidden blockers
- [ ] BLOCKER: **`bwrap.go` is missing `"os"` and `"strings"` imports.** The import block only lists `"fmt"` and `"path/filepath"`. But `BuildArgs` uses:
  - `os.Stat` (lines for staged config copies and project config dirs)
  - `strings.IndexByte` (env pair parsing)
  The code will not compile.
- [ ] ISSUE: **`--cap-drop ALL` behavior varies by bwrap version and privilege.** On this WSL2 system (`bwrap 0.9.0`), `--cap-drop CAP` drops a *named* capability. `--cap-drop ALL` may work differently depending on whether bwrap is setuid or user-namespace based. The plan does not verify this flag works as intended. However, the success criteria only test for the presence of the string in args, not runtime behavior, so this is not a compile blocker.
- [ ] ISSUE: **`/lib64` may not exist on all distros.** The `--ro-bind-try` flags are used, so missing paths are silently skipped — this is safe by design.
- [x] No cross-task conflicts
- [x] Success criteria verifiable — eight tests check arg slice contents

**Fix:** Add `"os"` and `"strings"` to the `bwrap.go` import block.

---

## Task 5.2: Pre-flight check

- [x] Dependencies complete — Task 3.1 listed
- [x] Context sufficient — imports are correct (`fmt`, `os/exec`, `strings`)
- [x] No hidden blockers
- [ ] ISSUE: **`TestCheck_BwrapMissing` and `TestCheck_SocatMissing` cannot be written without PATH manipulation.** The `Check` function calls `exec.Command("bwrap", "--version")` and `exec.LookPath("socat")` directly with no injection seam. To test the "missing" cases, the test must either modify `PATH` to exclude the binaries or skip on systems where the binary is installed. On the development machine (`bwrap 0.9.0` is installed), these tests will not exercise the "not found" path without PATH manipulation. The plan does not explain how to write these tests. This is a test writability blocker, not a compile blocker.
- [x] No cross-task conflicts
- [x] Success criteria verifiable for the format tests; ambiguous for the missing-tool tests
- [x] Code compiles

**Fix:** Add a note to the test descriptions: "Manipulate `PATH` in the test (set `t.Setenv("PATH", "")`) to simulate missing binaries without system-level changes."

---

## Task 6.1: Staging directory lifecycle

- [x] Dependencies complete — none required
- [x] Context sufficient — all imports present (`crypto/rand`, `encoding/hex`, `fmt`, `os`, `path/filepath`, `strings`)
- [x] No hidden blockers
- [ ] ISSUE: **`CleanStale` removes ALL `/tmp/nesco-sandbox-*` directories on every session start.** If two sandbox sessions run concurrently (two terminal windows), starting a second session will destroy the first session's staging directory while it's in use. The plan mentions "random ID makes concurrent sessions possible" but `CleanStale` directly contradicts this. This is a design-level issue not a compile blocker, but an executor following the spec will introduce a concurrency bug.
- [x] No cross-task conflicts
- [x] Success criteria verifiable — six named unit tests are clear
- [x] Code compiles

**Fix:** `CleanStale` should only remove staging directories older than some threshold (e.g., 24 hours) or should not be called if a live proxy socket exists in the directory. For MVP, this could be dropped from `CleanStale`'s behavior, or it could check if the directory's `proxy.sock` socket is connectable before removing.

---

## Task 6.2: Runner / session orchestrator

- [x] Dependencies complete — all Phase 1–6 tasks listed
- [x] Context sufficient — the step-by-step sequence is detailed
- [x] No hidden blockers for compilation
- [ ] BLOCKER: **`runner.go` references `SandboxConfig` from the `config` package, but does not import it.** The `RunConfig` struct embeds `SandboxConfig SandboxConfig`. The `import` block shown is:
  ```go
  import (
      "context"
      "fmt"
      "os"
      "os/exec"
      "os/signal"
      "path/filepath"
      "strings"
      "syscall"
      "time"
  )
  ```
  Missing: `"github.com/OpenScribbler/nesco/cli/internal/config"` — needed because `RunConfig.SandboxConfig` is typed as `config.SandboxConfig` (or just `SandboxConfig` if it's in the same package, but `SandboxConfig` lives in `config`, not `sandbox`). The plan shows `SandboxConfig SandboxConfig` without clarifying the package qualifier. An executor must use `config.SandboxConfig` and add the import.
- [ ] ISSUE: **Signal handling does not cover the window between staging dir creation and `cmd.Run()`.** `signal.NotifyContext` is set up at step 5 (after staging dir), but the context is only observed by `exec.CommandContext`. If a user presses Ctrl-C during steps 6–14 (staging files, writing scripts, starting proxy), the OS delivers SIGINT to the process, which is now caught by `NotifyContext` (the signal is consumed and `stop()` is called, cancelling ctx), but the steps 6–14 operations are not context-aware. In practice the SIGINT will cause the Go runtime's default SIGINT behavior *before* `NotifyContext` because it only suppresses the signal if the context is observed via `ctx.Done()`. This is subtle but the defers will still fire, so cleanup is not lost — it's just that in-progress file copies may be partially complete. Not a blocker for MVP.
- [ ] ISSUE: **`promptYN` reads from `os.Stdin` hardcoded, not from the `w *os.File` parameter or an injectable reader.** This makes the post-session approval prompt untestable in Task 9.1 without OS-level stdin redirection. The runner test for `TestRunSession_EnvSummaryPrinted` will work, but any test that needs to verify prompt behavior cannot.
- [x] No cross-task conflicts
- [ ] BLOCKER: **`runner.go` success criteria says "smoke test: reaches `bwrap` invocation" but the pre-flight check in step 3 will abort if `bwrap` is not installed** — the criteria says "even if bwrap is not installed, the error should be 'bwrap not found'." This means the test passes only if the check error message is exactly right. Acceptable, but the pre-flight check error message is `fmt.Errorf("pre-flight check failed:\n%s", FormatCheckResult(...))`, which includes the full formatted output. The criteria doesn't state what exact string to check. This is a test specificity issue, not a compile blocker.

**Fix:** Add `"github.com/OpenScribbler/nesco/cli/internal/config"` to runner.go imports and use `config.SandboxConfig` as the type qualifier in `RunConfig`.

---

## Task 7.1: `nesco sandbox` command group

- [x] Dependencies complete — all Phase 1–6 tasks listed, Task 1.3 listed
- [x] Context sufficient — follows `registry_cmd.go` pattern exactly
- [x] No hidden blockers
- [ ] BLOCKER: **`appendUnique`, `removeItem`, `appendUniqueInt`, `removeIntItem`, and `formatList` are defined inside `sandbox_cmd.go` but these function names may conflict with package-level helpers if they existed elsewhere.** Currently there are no such helpers in any cmd/nesco file (confirmed by search), so placing them in `sandbox_cmd.go` is fine. However, `truncateStr` is already defined in `registry_cmd.go` — the executor must not accidentally duplicate it.
- [ ] ISSUE: **`output.Writer` is referenced in the `sandboxAllowDomainCmd` handler but the runner uses `os.Stdout` directly.** Looking at `output.Writer` vs `os.Stdout` — the existing codebase uses `output.Writer` consistently for non-error output in cmd files. The plan's sandbox cmd handlers mix `fmt.Fprintf(output.Writer, ...)` with `fmt.Print(...)` (in `sandboxCheckCmd`). This inconsistency won't break compilation but creates style debt.
- [x] No cross-task conflicts with existing cmd files — no existing file defines `sandboxCmd`
- [x] Success criteria verifiable — five concrete CLI interactions
- [x] Code compiles — all imports listed are correct (`fmt`, `os`, `path/filepath`, `strconv`, `strings`, `github.com/OpenScribbler/nesco/cli/internal/config`, `github.com/OpenScribbler/nesco/cli/internal/output`, `github.com/OpenScribbler/nesco/cli/internal/sandbox`, `github.com/spf13/cobra`)

---

## Task 7.2: Registry add — sandbox allowlist prompt

- [x] Dependencies complete — Task 1.3 listed
- [ ] BLOCKER: **`"net/url"` is not imported in `registry_cmd.go`.** The plan states "The `url` import and `strings` import are already available in the registry command file." This is **incorrect**. The actual imports in `registry_cmd.go` are: `fmt`, `os`, `path/filepath`, `strings`, plus the internal packages. `"net/url"` is not present and must be added.
- [ ] BLOCKER: **`appendUnique` is defined in `sandbox_cmd.go` (Task 7.1), not in `registry_cmd.go`.** The plan says "If it's only in `sandbox_cmd.go`, inline the uniqueness check here." The executor must inline the check — but the plan presents this as an afterthought. This needs to be the primary instruction, not a fallback.
- [x] No cross-task conflicts with `sandbox_cmd.go` — they are in the same `main` package so `appendUnique` from `sandbox_cmd.go` is accessible, but Go packages don't guarantee file load order during testing. In production, it compiles fine. In test files for `registry_cmd_test.go`, calling a function from `sandbox_cmd.go` is valid since they're the same package.
- [x] Success criteria verifiable — three behavioral assertions
- [x] Code compiles once `"net/url"` is added and `appendUnique` dependency is resolved

**Fix:** Add `"net/url"` to the import block instruction for `registry_cmd.go`. Remove the claim that `url` is "already available." Make the inline uniqueness check the primary approach rather than a fallback.

---

## Task 8.1: Sandbox settings TUI screen

- [x] Dependencies complete — Task 1.3 listed, existing `settings.go` patterns referenced
- [x] Context sufficient — mirrors `settingsModel` pattern accurately
- [x] No hidden blockers
- [ ] BLOCKER: **`appendUniqueTUI` and `appendUniqueIntTUI` are defined in `sandbox_settings.go` but `appendUnique` is already defined in `sandbox_cmd.go` (package `main`).** These are in different packages (`tui` vs `main`), so no conflict. But `appendUniqueTUI` duplicates the pattern rather than sharing it. The `_TUI` suffix is an implicit acknowledgment of this — fine for MVP, but the executor should not be confused by the naming.
- [ ] ISSUE: **`keys.Save` is referenced in `sandboxSettingsModel.Update()` but `Save` in `keys.go` is bound to `"s"`.** In the provider detail view (Task 8.3), `"s"` is planned as the sandbox-launch key (`SandboxRun`). If both views use `"s"` for different purposes, there's no conflict because they're separate screens — but the `keys` struct would need a new `SandboxRun` binding added, and the existing `Save` binding reused for the settings screen. This requires no structural conflict but the executor must check that `"s"` as `keys.Save` still makes sense in the sandbox settings context (it does — "s" to save is the same behavior as in `settings.go`).
- [x] No cross-task conflicts — new file, no existing `sandbox_settings.go`
- [x] Success criteria verifiable — "compiles" and "make build passes"
- [x] Code compiles — all imports present (`fmt`, `strconv`, `strings`, `github.com/charmbracelet/bubbles/key`, `tea`, `zone`, `github.com/OpenScribbler/nesco/cli/internal/config`)

---

## Task 8.2: Wire sandbox settings into App and sidebar

- [x] Dependencies complete — Task 8.1 listed
- [x] Context sufficient — the plan gives exact field names, method signatures, and insertion points
- [ ] BLOCKER: **`isSandboxSelected()` uses index `len(m.types) + 5` but the current `totalItems()` returns `len(m.types) + 5` (import=+1, update=+2, settings=+3, registries=+4, plus my-tools=+0 from types end).** The plan says "Sandbox" is at index `len(m.types) + 5`, making `totalItems()` return `len(m.types) + 6`. The instruction to update `totalItems()` says "returns `len(m.types) + 6` (adding 1 for Sandbox)." The executor must:
  1. Update `totalItems()` from `+5` to `+6`
  2. Add "Sandbox" to the `utilItems` slice in `sidebar.go.View()` at index `len(m.types) + 5`
  3. Add the `isSandboxSelected()` method
  These are three separate edits to `sidebar.go` that the plan lists but does not present as a single atomic change — an executor may miss one.
- [ ] BLOCKER: **`a.setError(...)` is called in Task 8.3 but `setError` does not exist as a method on `App`.** `App` has a `statusMessage` field and `statusWarnings` field that are set directly inline in handlers (e.g., `a.statusMessage = fmt.Sprintf(...)`). The plan says `a.setError(fmt.Sprintf(...))` but this method is not defined anywhere in the TUI. An executor implementing Task 8.3 must either define `setError` or set `a.statusMessage` directly.
- [x] No cross-task conflicts — the new screen constant goes after `screenRegistries`, preserving existing iota values
- [x] Success criteria verifiable — four behavioral assertions
- [x] Code compiles once the sidebar edits are complete

**Fix:** List the three sidebar.go edits as numbered steps. Clarify that `setError` does not exist — direct field assignment (`a.statusMessage = ...`) is the correct pattern.

---

## Task 8.3: TUI — Mount profile display and launch sandbox from provider view

- [x] Dependencies complete — Tasks 8.1, 8.2, 6.2, 3.1 listed
- [ ] BLOCKER: **The plan says to modify `cli/internal/tui/provider_detail.go` but this file does not exist.** The provider detail functionality is in `cli/internal/tui/detail.go` (with support files `detail_env.go`, `detail_fileviewer.go`, `detail_provcheck.go`, `detail_render.go`). An executor will not know which file to modify.
- [ ] BLOCKER: **`keys.SandboxRun` is not defined in `keys.go`.** The `keyMap` struct and `keys` var in `cli/internal/tui/keys.go` do not have a `SandboxRun` field. The plan requires adding it. The executor must add the field to the `keyMap` struct and add the binding in the `keys` var — two edits in `keys.go`.
- [ ] BLOCKER: **`"s"` key conflict: `keys.Save` is already bound to `"s"`.** The plan assigns `"s"` to `SandboxRun`. If both `keys.Save` and `keys.SandboxRun` bind to `"s"`, there is no ambiguity in runtime (they are checked in different screen contexts), but the `key.NewBinding` and Cobra help will both show `"s"` for different purposes. More critically, in the provider detail view where `"s"` for save may already be handled, adding sandbox-run on `"s"` will shadow the save behavior. The executor needs explicit guidance on which key to use for sandbox launch — the plan should pick a key that does not conflict (e.g., `"S"` or `ctrl+s`).
- [ ] BLOCKER: **`a.setError(...)` is not a method on `App`** (see Task 8.2 analysis). The plan instructs the executor to handle `sandboxExitMsg` using `a.setError(...)` in `App.Update()`. The correct pattern from the codebase is direct field assignment: `a.statusMessage = fmt.Sprintf(...)`.
- [ ] ISSUE: **`tea.ExecProcess` is available and verified in use.** The plan notes "Verify the project's Bubbletea version supports it before implementation" — Bubbletea v1.3.10 is in use and `tea.ExecProcess` is already used in `detail.go:656` and `import.go:965`. No blocker.
- [x] No cross-task conflicts with other TUI files
- [ ] Success criteria partially not verifiable — "TUI resumes cleanly after the sandbox session ends" is only verifiable by manual testing, not automated tests. The success criteria includes no automated test for this task.

**Fix:** Change `provider_detail.go` to `detail_render.go` or `detail.go` (whichever contains the `View()` method — verify at implementation time). Add the `SandboxRun` key field definition to `keys.go` as an explicit step. Change the key from `"s"` to `"S"` or another non-conflicting key. Replace `a.setError(...)` with `a.statusMessage = fmt.Sprintf(...)`.

---

## Task 9.1: End-to-end smoke test (no bwrap required)

- [x] Dependencies complete — all sandbox package tasks listed
- [ ] BLOCKER: **The test design requires refactoring `runner.go` to add `bwrapRunner` injectable var, but this refactor is not listed as a dependency of runner.go (Task 6.2).** Task 9.1 says "Refactor `RunSession` to use an injectable runner function." This means the executor must go back and modify Task 6.2's output. This is a hidden retroactive dependency — Task 6.2 must produce a version of `runner.go` that already has the `bwrapRunner` seam, or Task 9.1 must be understood as "modify `runner.go` AND write tests."
- [ ] BLOCKER: **`TestRunSession_DirSafetyFails` requires a directory that fails safety checks, but in CI the working directory may be shallow enough to pass (e.g., `/tmp/test123` is only 2 levels below root).** The test must create a directory that fails the depth check or the blocklist. The plan does not describe how to construct such a directory in the test environment.
- [ ] BLOCKER: **`TestRunSession_ProxyStarted` says "proxy socket file exists between start and bwrap injection" but with the injectable `bwrapRunner`, the test replaces bwrap entirely.** The test must inject a function that checks for the socket *before* returning, not after. The plan does not describe how to observe the transient state.
- [x] No cross-task conflicts
- [ ] Success criteria: `TestRunSession_EnvSummaryPrinted` is clear and achievable; the other four tests require careful scaffolding not described

**Fix:** Add explicit note: "Task 9.1 requires modifying Task 6.2's `runner.go` to extract `bwrapRunner` — treat this as a prerequisite step, not a separate file." Provide concrete test scaffolding examples for the directory safety and proxy socket tests.

---

## Task 9.2: CLI command integration tests

- [x] Dependencies complete — Task 7.1 listed
- [x] Context sufficient — the existing `config_cmd_test.go` pattern (using `RunE` directly with `os.Chdir`) is adequate reference
- [x] No hidden blockers
- [ ] ISSUE: **`TestSandboxAllowDomain_WritesConfig` requires a project with a `.nesco/config.json` that has the `Sandbox` field.** The `setupGoProject` helper in `testhelpers_test.go` creates a config with `Providers: []string{"claude-code"}` but no `Sandbox` field. The test must either use a modified setup or call `config.Save` with a `Sandbox`-capable config struct. Since `json.Unmarshal` with `omitempty` will zero-value the field, this is not a blocker — the test will work correctly.
- [x] No cross-task conflicts
- [x] Success criteria verifiable — seven concrete tests
- [x] Code compiles once `sandbox_cmd.go` is in place

---

## Task 9.3: Final verification

- [x] Dependencies complete — all previous tasks
- [x] Context sufficient — four make targets are real (`fmt`, `vet`, `build`, `test`)
- [x] No hidden blockers
- [x] No cross-task conflicts
- [x] Success criteria verifiable — zero/nonzero exit codes
- [x] No code to compile

---

## Summary

**Total tasks:** 18 (1.1, 1.2, 1.3, 2.1, 2.2, 3.1, 3.2, 4.1, 5.1, 5.2, 6.1, 6.2, 7.1, 7.2, 8.1, 8.2, 8.3, 9.1, 9.2, 9.3)

**Tasks with blockers:** 15 of 18

| Task | Blockers | Severity |
|------|----------|----------|
| 1.3 | Schema modification not described as patch — may confuse executor | Low |
| 2.1 | `goleak` referenced but not in go.mod | Low |
| 2.2 | Missing `"strings"` import; `shellescape` coupling not documented | **High** |
| 3.1 | `"copilot"` slug should be `"copilot-cli"`; `windsurf` missing | **High** |
| 3.2 | Missing `"fmt"` import; `shellescape` dependency undeclared | **High** |
| 4.1 | Missing `"strings"` import | **High** |
| 5.1 | Missing `"os"` and `"strings"` imports | **High** |
| 5.2 | Tests for missing tools require PATH manipulation — not described | Medium |
| 6.1 | `CleanStale` destroys concurrent sessions | Medium |
| 6.2 | Missing `config` package import in runner.go | **High** |
| 7.2 | `"net/url"` not already imported; `appendUnique` dependency unclear | **High** |
| 8.2 | `setError` method doesn't exist; sidebar edit steps need explicit enumeration | **High** |
| 8.3 | Wrong file name (`provider_detail.go`); `keys.SandboxRun` undefined; `"s"` key conflict; `setError` doesn't exist | **Critical** |
| 9.1 | `bwrapRunner` seam is an undocumented retroactive dependency on Task 6.2 | **High** |

**Tasks without blockers:** 1.1, 1.2, 7.1, 8.1, 9.2, 9.3 (6 tasks)

---

## Priority Fixes Before Execution

1. **Fix all missing imports** (tasks 2.2, 3.2, 4.1, 5.1, 6.2, 7.2) — these cause compile failures before any logic runs.

2. **Fix the `copilot` slug mismatch** (task 3.1) — the wrong slug silently produces "unknown provider" at runtime.

3. **Correct the wrong file name in task 8.3** — `provider_detail.go` does not exist; the correct target is `detail.go` or `detail_render.go`.

4. **Replace `a.setError(...)` with `a.statusMessage = ...`** (tasks 8.2, 8.3) — `setError` is not defined anywhere in the TUI.

5. **Resolve the `"s"` key conflict** (task 8.3) — `keys.Save` already owns `"s"`; pick a non-conflicting key for `SandboxRun`.

6. **Add the `bwrapRunner` seam to task 6.2** (not just 9.1) — the injectable function must be part of the `runner.go` implementation, not a retroactive refactor.

7. **Add `windsurf` stub to `ProfileFor`** (task 3.1) — or explicitly document the omission so the executor doesn't wonder.
