# Syllago Benchmark Installer — Implementation Plan

**Design doc:** `docs/plans/2026-03-31-syllago-benchmark-installer-design.md`
**Date:** 2026-03-31

---

## Overview

Four layers:

| Layer | Type | Scope |
|-------|------|-------|
| 1 | Code | `--to-all` flag on `syllago install` (~50 lines Go) |
| 2 | Manual | Benchmark registry setup and verification |
| 3 | Doc | Workflow doc in `docs/research/skill-behavior-checks/` |
| 4 | Roadmap | Multi-provider loadout bead |

Dependencies: Layer 2 depends on Layer 1 completing first. Layers 3 and 4 are independent.

---

## Layer 1: `--to-all` flag

### Task 1.1 — Write failing tests for `--to-all`

**File:** `cli/cmd/syllago/install_cmd_test.go`

Add these test cases at the end of the existing test file. They must be added before implementation so they fail on the current code.

**Tests to add:**

```go
func TestInstallToAllConflictsWithTo(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)
	_, _ = output.SetForTest(t)

	installCmd.Flags().Set("to", "claude-code")
	defer installCmd.Flags().Set("to", "")
	installCmd.Flags().Set("to-all", "true")
	defer installCmd.Flags().Set("to-all", "false")
	installCmd.Flags().Set("all", "true")
	defer installCmd.Flags().Set("all", "false")

	err := installCmd.RunE(installCmd, []string{})
	if err == nil {
		t.Fatal("expected error when both --to and --to-all are specified")
	}
	if !strings.Contains(err.Error(), "--to-all") {
		t.Errorf("expected '--to-all' in error message, got: %v", err)
	}
	if !strings.Contains(err.Error(), "--to") {
		t.Errorf("expected '--to' in error message, got: %v", err)
	}
}

func TestInstallToAllNoProvidersDetected(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)
	_, stderr := output.SetForTest(t)

	// Override AllProviders with a single undetected provider.
	orig := append([]provider.Provider(nil), provider.AllProviders...)
	provider.AllProviders = []provider.Provider{
		{
			Name:     "No One",
			Slug:     "no-one",
			Detected: false,
			Detect:   func(string) bool { return false },
			InstallDir: func(_ string, ct catalog.ContentType) string {
				if ct == catalog.Skills {
					return "/tmp/no-one/skills"
				}
				return ""
			},
			SupportsType: func(ct catalog.ContentType) bool { return ct == catalog.Skills },
		},
	}
	t.Cleanup(func() { provider.AllProviders = orig })

	installCmd.Flags().Set("to-all", "true")
	defer installCmd.Flags().Set("to-all", "false")
	installCmd.Flags().Set("type", "skills")
	defer installCmd.Flags().Set("type", "")

	err := installCmd.RunE(installCmd, []string{})
	if err != nil {
		t.Fatalf("--to-all with no detected providers should not error: %v", err)
	}

	errOut := stderr.String()
	if !strings.Contains(errOut, "no providers detected") {
		t.Errorf("expected 'no providers detected' message, got: %s", errOut)
	}
}

func TestInstallToAllPartialSuccess(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)

	installBase := t.TempDir()

	// Two providers: one detected (succeeds), one whose install dir is a non-writable path.
	okBase := filepath.Join(installBase, "ok")
	os.MkdirAll(okBase, 0755)

	orig := append([]provider.Provider(nil), provider.AllProviders...)
	provider.AllProviders = []provider.Provider{
		{
			Name:     "Good Provider",
			Slug:     "good-provider",
			Detected: true,
			Detect:   func(string) bool { return true },
			InstallDir: func(_ string, ct catalog.ContentType) string {
				if ct == catalog.Skills {
					return filepath.Join(okBase, "skills")
				}
				return ""
			},
			SupportsType: func(ct catalog.ContentType) bool { return ct == catalog.Skills },
		},
		{
			Name:     "Bad Provider",
			Slug:     "bad-provider",
			Detected: true,
			Detect:   func(string) bool { return true },
			InstallDir: func(_ string, ct catalog.ContentType) string {
				// Return a path under /proc which is unwritable in test environments.
				if ct == catalog.Skills {
					return "/proc/syllago-test/skills"
				}
				return ""
			},
			SupportsType: func(ct catalog.ContentType) bool { return ct == catalog.Skills },
		},
	}
	t.Cleanup(func() { provider.AllProviders = orig })

	stdout, _ := output.SetForTest(t)

	installCmd.Flags().Set("to-all", "true")
	defer installCmd.Flags().Set("to-all", "false")
	installCmd.Flags().Set("type", "skills")
	defer installCmd.Flags().Set("type", "")

	err := installCmd.RunE(installCmd, []string{})
	// Partial failure: RunE returns a non-nil error because at least one provider failed.
	if err == nil {
		t.Fatal("expected non-nil error when at least one provider fails")
	}

	out := stdout.String()
	// Good provider should appear as installed.
	if !strings.Contains(out, "Good Provider") {
		t.Errorf("expected 'Good Provider' in output, got: %s", out)
	}
	// Bad provider should appear as failed.
	if !strings.Contains(out, "Bad Provider") {
		t.Errorf("expected 'Bad Provider' in output, got: %s", out)
	}
}

func TestInstallToAllAllSkipped(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)

	orig := append([]provider.Provider(nil), provider.AllProviders...)
	// Two detected providers, neither supports skills.
	provider.AllProviders = []provider.Provider{
		{
			Name:         "No Skills A",
			Slug:         "no-skills-a",
			Detected:     true,
			Detect:       func(string) bool { return true },
			InstallDir:   func(_ string, _ catalog.ContentType) string { return "" },
			SupportsType: func(_ catalog.ContentType) bool { return false },
		},
		{
			Name:         "No Skills B",
			Slug:         "no-skills-b",
			Detected:     true,
			Detect:       func(string) bool { return true },
			InstallDir:   func(_ string, _ catalog.ContentType) string { return "" },
			SupportsType: func(_ catalog.ContentType) bool { return false },
		},
	}
	t.Cleanup(func() { provider.AllProviders = orig })

	stdout, _ := output.SetForTest(t)

	installCmd.Flags().Set("to-all", "true")
	defer installCmd.Flags().Set("to-all", "false")
	installCmd.Flags().Set("type", "skills")
	defer installCmd.Flags().Set("type", "")

	// All skip → no error (skips are not failures).
	err := installCmd.RunE(installCmd, []string{})
	if err != nil {
		t.Fatalf("all-skipped --to-all should not error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "skipped") {
		t.Errorf("expected 'skipped' summary in output, got: %s", out)
	}
}

func TestInstallToAllDryRun(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)

	installBase := t.TempDir()
	addTestProviderOpts(t, "dryrun-prov-a", "DryRun A", installBase, true)
	addTestProviderOpts(t, "dryrun-prov-b", "DryRun B", installBase, true)

	stdout, _ := output.SetForTest(t)

	installCmd.Flags().Set("to-all", "true")
	defer installCmd.Flags().Set("to-all", "false")
	installCmd.Flags().Set("type", "skills")
	defer installCmd.Flags().Set("type", "")
	installCmd.Flags().Set("dry-run", "true")
	defer installCmd.Flags().Set("dry-run", "false")

	err := installCmd.RunE(installCmd, []string{})
	if err != nil {
		t.Fatalf("--to-all --dry-run failed: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "dry-run") {
		t.Errorf("expected 'dry-run' in output, got: %s", out)
	}
	// Nothing written anywhere.
	entries, _ := os.ReadDir(installBase)
	if len(entries) > 0 {
		t.Errorf("dry-run should not write files")
	}
}

func TestInstallFlagsRegisteredToAll(t *testing.T) {
	if installCmd.Flags().Lookup("to-all") == nil {
		t.Error("expected --to-all flag to be registered on installCmd")
	}
}
```

Run to confirm failure:
```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./cmd/syllago/ -run 'TestInstallToAll' 2>&1 | head -30
```

**Success criteria:**
- `go test ./cmd/syllago/ -run 'TestInstallToAll'` → exit code 1 (test fails)
- Test output contains exact string "flag accessed but not defined: to-all"

---

### Task 1.2 — Remove `MarkFlagRequired("to")` and add `--to-all` flag

**File:** `cli/cmd/syllago/install_cmd.go`

**Why:** `MarkFlagRequired` is enforced by cobra before `RunE` runs, so we cannot do mutual-exclusion validation inside `RunE` if cobra already rejects the command. We remove the required marker and add the runtime check ourselves.

**Change the `init()` function** — remove the `installCmd.MarkFlagRequired("to")` call (line 64) and add the `--to-all` flag registration, keeping the `--to` flag as optional. The exact line to remove is:

```diff
-     installCmd.MarkFlagRequired("to")
+     // Note: --to is no longer MarkFlagRequired — mutual exclusion with --to-all
+     // is enforced at runtime in RunE.
+     installCmd.Flags().Bool("to-all", false, "Install to all detected providers")
```

```go
func init() {
	installCmd.Flags().String("to", "", "Provider to install into")
	// Note: --to is no longer MarkFlagRequired — mutual exclusion with --to-all
	// is enforced at runtime in RunE.
	installCmd.Flags().Bool("to-all", false, "Install to all detected providers")
	installCmd.Flags().String("type", "", "Filter to a specific content type")
	installCmd.Flags().String("method", "symlink", "Install method: symlink (default) or copy")
	installCmd.Flags().Bool("all", false, "Install all library content (cannot combine with a positional name)")
	installCmd.Flags().BoolP("dry-run", "n", false, "Show what would happen without making changes")
	installCmd.Flags().String("base-dir", "", "Override base directory for content installation")
	installCmd.Flags().Bool("no-input", false, "Disable interactive prompts, use defaults")
	rootCmd.AddCommand(installCmd)
}
```

**Update the Example string** in the command definition to include the new flag:

```go
Example: `  # Install a skill to Claude Code
  syllago install my-skill --to claude-code

  # Install with a standalone copy instead of symlink
  syllago install my-skill --to cursor --method copy

  # Install all skills to a provider
  syllago install --to claude-code --type skills

  # Install to every detected provider at once
  syllago install --type skills --to-all

  # Install everything from your library
  syllago install --all --to claude-code

  # Preview what would happen
  syllago install my-skill --to claude-code --dry-run`,
```

**Success criteria:**
- `cd cli && go build ./cmd/syllago/` → passes (no compile error)
- `go test ./cmd/syllago/ -run 'TestInstallFlagsRegisteredToAll'` → pass
- `go test ./cmd/syllago/ -run 'TestInstallFlagsRegistered'` → still pass (existing test)

---

### Task 1.3 — Add mutual-exclusion validation and require `--to` when `--to-all` absent

**File:** `cli/cmd/syllago/install_cmd.go`

**Why:** Since we removed `MarkFlagRequired`, we must add a runtime check. We also need to guard against calling `--to-all` together with `--to`.

Replace the top of `runInstall` (lines 74–105 roughly) with the updated validation block. Read the current lines first and make a targeted edit:

**Current block (lines 74–105):**
```go
func runInstall(cmd *cobra.Command, args []string) error {
	toSlug, _ := cmd.Flags().GetString("to")
	typeFilter, _ := cmd.Flags().GetString("type")
	methodStr, _ := cmd.Flags().GetString("method")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	baseDir, _ := cmd.Flags().GetString("base-dir")
	installAll, _ := cmd.Flags().GetBool("all")

	// --all and a positional name are mutually exclusive.
	if installAll && len(args) > 0 {
		return output.NewStructuredError(output.ErrInputConflict, "cannot specify both a name and --all", "Use either a positional argument or --all, not both")
	}

	// Require explicit intent for bulk installs: name, --all, or --type.
	if len(args) == 0 && !installAll && typeFilter == "" {
		return output.NewStructuredError(output.ErrInputMissing, "specify a name, --all, or --type to install", "Examples:\n  syllago install my-skill --to <provider>\n  syllago install --all --to <provider>\n  syllago install --type rules --to <provider>")
	}

	method := installer.MethodSymlink
	if methodStr == "copy" {
		method = installer.MethodCopy
	}

	prov := findProviderBySlug(toSlug)
	if prov == nil {
		slugs := providerSlugs()
		return output.NewStructuredError(
			output.ErrProviderNotFound,
			"unknown provider: "+toSlug,
			"Available: "+strings.Join(slugs, ", "),
		)
	}
```

**Replace with:**
```go
func runInstall(cmd *cobra.Command, args []string) error {
	toSlug, _ := cmd.Flags().GetString("to")
	toAll, _ := cmd.Flags().GetBool("to-all")
	typeFilter, _ := cmd.Flags().GetString("type")
	methodStr, _ := cmd.Flags().GetString("method")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	baseDir, _ := cmd.Flags().GetString("base-dir")
	installAll, _ := cmd.Flags().GetBool("all")

	// --to-all and --to are mutually exclusive.
	if toAll && toSlug != "" {
		return output.NewStructuredError(output.ErrInputConflict, "--to-all and --to are mutually exclusive", "Use --to-all to install to every detected provider, or --to <provider> for a specific one")
	}

	// Without --to-all, --to is required.
	if !toAll && toSlug == "" {
		slugs := providerSlugs()
		return output.NewStructuredError(output.ErrInputMissing, "--to is required (or use --to-all)", "Available providers: "+strings.Join(slugs, ", "))
	}

	// --all and a positional name are mutually exclusive.
	if installAll && len(args) > 0 {
		return output.NewStructuredError(output.ErrInputConflict, "cannot specify both a name and --all", "Use either a positional argument or --all, not both")
	}

	// Require explicit intent for bulk installs: name, --all, or --type.
	if len(args) == 0 && !installAll && typeFilter == "" {
		return output.NewStructuredError(output.ErrInputMissing, "specify a name, --all, or --type to install", "Examples:\n  syllago install my-skill --to <provider>\n  syllago install --all --to <provider>\n  syllago install --type rules --to <provider>")
	}

	method := installer.MethodSymlink
	if methodStr == "copy" {
		method = installer.MethodCopy
	}

	// --to-all path: detect providers and delegate.
	if toAll {
		return runInstallToAll(cmd, args, typeFilter, methodStr, method, dryRun, baseDir, installAll)
	}

	prov := findProviderBySlug(toSlug)
	if prov == nil {
		slugs := providerSlugs()
		return output.NewStructuredError(
			output.ErrProviderNotFound,
			"unknown provider: "+toSlug,
			"Available: "+strings.Join(slugs, ", "),
		)
	}
```

**Success criteria:**
- `go test ./cmd/syllago/ -run 'TestInstallToAllConflictsWithTo'` → pass
- `go test ./cmd/syllago/ -run 'TestInstallRequiresExplicitIntent'` → still pass (no regression)
- `go test ./cmd/syllago/ -run 'TestInstallUnknownProvider'` → still pass

---

### Task 1.4 — Extract `installToProvider` helper

**File:** `cli/cmd/syllago/install_cmd.go`

**Why:** The install loop inside `runInstall` (lines 174–250 roughly) is the core per-provider work. Rather than duplicating it inside `runInstallToAll`, we extract it as a helper that takes a `provider.Provider` and returns `(installResult, error)`. The main `runInstall` function calls the helper for its single provider; `runInstallToAll` calls it for each detected provider.

**Add the helper function** after `runInstall`. This is a new function, so add it after the closing brace of `runInstall`:

```go
// installToProvider installs the given items to a single provider and returns
// the result. Returns (result, nil) in all normal cases — individual item
// failures are recorded in result.Skipped, not propagated as a return error.
// Returns (result, error) only if the install operation itself cannot proceed
// (e.g., config corruption, unrecoverable system error). Per-item errors such
// as "does not support" or permission-denied on a single file are always
// recorded in result.Skipped so the caller can continue to the next provider.
func installToProvider(
	items []catalog.ContentItem,
	prov provider.Provider,
	globalDir string,
	method installer.InstallMethod,
	dryRun bool,
	resolver *config.PathResolver,
	toSlug string, // used for audit log
	projectRoot string,
) (installResult, error) {
	var result installResult

	for _, item := range items {
		if dryRun {
			if !output.Quiet {
				m := "symlink"
				if installer.IsJSONMerge(prov, item.Type) {
					m = "json-merge"
				}
				fmt.Fprintf(output.Writer, "[dry-run] would install %s (%s) to %s via %s\n", item.Name, item.Type.Label(), prov.Name, m)

				if installer.IsJSONMerge(prov, item.Type) {
					contentFile := converter.ResolveContentFile(item)
					if contentFile != "" {
						if preview, err := os.ReadFile(contentFile); err == nil {
							fmt.Fprintf(output.Writer, "  content preview:\n")
							for _, line := range strings.SplitN(string(preview), "\n", 10) {
								fmt.Fprintf(output.Writer, "    %s\n", line)
							}
						}
					}
				}
			}
			continue
		}

		desc, err := installer.InstallWithResolver(item, prov, globalDir, method, resolver)
		if err != nil {
			result.Skipped = append(result.Skipped, skippedItem{Name: item.Name, Reason: err.Error()})
			if !output.JSON {
				fmt.Fprintf(output.ErrWriter, "  skip %s: %s\n", item.Name, err)
			}
			continue
		}

		// Check for portability warnings by running the converter.
		var warnings []string
		if conv := converter.For(item.Type); conv != nil {
			contentFile := converter.ResolveContentFile(item)
			if contentFile != "" {
				if raw, readErr := os.ReadFile(contentFile); readErr == nil {
					srcProv := ""
					if item.Meta != nil {
						srcProv = item.Meta.SourceProvider
					}
					if canonical, cErr := conv.Canonicalize(raw, srcProv); cErr == nil {
						if rendered, rErr := conv.Render(canonical.Content, prov); rErr == nil {
							warnings = rendered.Warnings
						}
					}
				}
			}
		}

		result.Installed = append(result.Installed, installedItem{
			Name:     item.Name,
			Type:     string(item.Type),
			Method:   string(method),
			Path:     desc,
			Warnings: warnings,
		})

		// Audit log (best-effort).
		if auditLogger, aErr := audit.NewLogger(audit.DefaultLogPath(projectRoot)); aErr == nil {
			_ = auditLogger.LogContent(audit.EventContentInstall, item.Name, string(item.Type), toSlug)
			_ = auditLogger.Close()
		}

		if !output.JSON && !output.Quiet {
			if method == installer.MethodSymlink {
				fmt.Fprintf(output.Writer, "  Symlinked %s to %s\n", item.Name, desc)
			} else {
				fmt.Fprintf(output.Writer, "  Copied %s to %s\n", item.Name, desc)
			}
			for _, w := range warnings {
				fmt.Fprintf(output.ErrWriter, "    - %s\n", w)
			}
		}
	}

	return result, nil
}
```

**Update `runInstall`** to call the helper instead of the inline loop. Replace the loop section (from `var result installResult` through the post-loop JSON/hint section):

```go
	// ... (existing catalog scan, item filtering, etc.) ...

	if len(items) > 1 && !output.Quiet && !output.JSON {
		fmt.Fprintf(output.Writer, "Installing %d items to %s...\n", len(items), prov.Name)
	}

	result, _ := installToProvider(items, *prov, globalDir, method, dryRun, resolver, toSlug, projectRoot)

	if output.JSON {
		output.Print(result)
		return nil
	}

	if len(result.Installed) > 0 && !output.Quiet {
		fmt.Fprintf(output.Writer, "\n  # Install to another provider\n")
		fmt.Fprintf(output.Writer, "  syllago install %s --to <provider>\n", firstArg(args))
		fmt.Fprintf(output.Writer, "\n  # Convert for sharing\n")
		fmt.Fprintf(output.Writer, "  syllago convert %s --to <provider>\n", firstArg(args))
	}

	return nil
```

**Success criteria:**
- `cd cli && go build ./cmd/syllago/` → passes
- `go test ./cmd/syllago/ -run 'TestInstallAllInstallsEverything'` → pass (regression: helper still works for single provider)
- `go test ./cmd/syllago/ -run 'TestInstallJSONOutputOnSuccess'` → pass
- Individual item errors (e.g., permission denied on a single file) are recorded in `result.Skipped`, not returned as a function error
- A per-item skip does not cause `installToProvider` to return a non-nil error

---

### Task 1.5 — Implement `runInstallToAll`

**File:** `cli/cmd/syllago/install_cmd.go`

**Why:** This is the core of the `--to-all` feature. It detects providers, loops over them, calls `installToProvider` for each, accumulates per-provider results, prints a summary table, and returns a non-nil error if any provider had failures (not skips).

**Add after `installToProvider`:**

```go
// providerInstallResult holds the per-provider outcome for --to-all.
type providerInstallResult struct {
	Provider string
	Slug     string
	Status   string // "installed", "skipped", "failed", "no-items"
	Count    int
	Details  string // short reason for skipped/failed
}

// runInstallToAll handles the --to-all branch of runInstall.
// It detects providers, installs to each, and prints a summary.
// Returns a non-nil error if any provider had install failures.
func runInstallToAll(
	_ *cobra.Command,
	args []string,
	typeFilter string,
	_ string,
	method installer.InstallMethod,
	dryRun bool,
	baseDir string,
	installAll bool,
) error {
	// Load config (mirrors the single-provider path).
	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrConfigInvalid, "loading global config", "Check ~/.syllago/config.json syntax", err.Error())
	}
	projectRoot, _ := findProjectRoot()
	projectCfg, err := config.Load(projectRoot)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrConfigNotFound, "loading project config", "Run 'syllago init' to create project config", err.Error())
	}
	mergedCfg := config.Merge(globalCfg, projectCfg)
	resolver := config.NewResolver(mergedCfg, baseDir)
	if err := resolver.ExpandPaths(); err != nil {
		return output.NewStructuredErrorDetail(output.ErrConfigPath, "expanding paths", "Check path overrides in config", err.Error())
	}

	detected := provider.DetectProvidersWithResolver(resolver)
	var active []provider.Provider
	for _, p := range detected {
		if p.Detected {
			active = append(active, p)
		}
	}

	if len(active) == 0 {
		fmt.Fprintln(output.ErrWriter, "no providers detected — install an AI coding tool and retry")
		return nil
	}

	// Scan global library once, filter items once.
	globalDir := catalog.GlobalContentDir()
	if globalDir == "" {
		return output.NewStructuredError(output.ErrSystemHomedir, "cannot determine home directory", "Set the HOME environment variable")
	}
	globalCat, err := catalog.Scan(globalDir, globalDir)
	if err != nil {
		return output.NewStructuredErrorDetail(output.ErrCatalogScanFailed, "scanning library", "Check file permissions in ~/.syllago/content/", err.Error())
	}

	var items []catalog.ContentItem
	for _, item := range globalCat.Items {
		if len(args) == 1 && item.Name != args[0] {
			continue
		}
		if typeFilter != "" && string(item.Type) != typeFilter {
			continue
		}
		items = append(items, item)
	}

	if len(items) == 0 {
		if len(args) == 1 {
			hint := typeFilter
			if hint == "" {
				hint = "skills"
			}
			return output.NewStructuredError(output.ErrInstallItemNotFound, fmt.Sprintf("no item named %q found in your library", args[0]), "Hint: syllago list --type "+hint)
		}
		fmt.Fprintln(output.ErrWriter, "no items found in library matching filters")
		return nil
	}

	if !output.Quiet && !output.JSON {
		fmt.Fprintf(output.Writer, "Installing %d item(s) to %d detected provider(s)...\n\n", len(items), len(active))
	}

	var (
		provResults   []providerInstallResult
		anyFailed     bool
	)

	for _, prov := range active {
		if !output.Quiet && !output.JSON {
			fmt.Fprintf(output.Writer, "→ %s\n", prov.Name)
		}

		result, _ := installToProvider(items, prov, globalDir, method, dryRun, resolver, prov.Slug, projectRoot)

		pr := providerInstallResult{
			Provider: prov.Name,
			Slug:     prov.Slug,
		}

		switch {
		case len(result.Installed) > 0:
			pr.Status = "installed"
			pr.Count = len(result.Installed)
		case len(result.Skipped) > 0:
			// All skipped — determine whether skips are expected (content type not
			// supported by this provider) or unexpected (actual install failure).
			//
			// Expected-skip reasons (do NOT set anyFailed):
			//   "does not support" — provider doesn't support this content type
			//   "project-scoped"   — content type is project-scoped, not global
			//   "JSON merge"       — merge target already exists, no write needed
			//
			// Any other reason is treated as an actual failure (sets anyFailed=true).
			allUnsupported := true
			for _, s := range result.Skipped {
				if !strings.Contains(s.Reason, "does not support") &&
					!strings.Contains(s.Reason, "project-scoped") &&
					!strings.Contains(s.Reason, "JSON merge") {
					// Unrecognized skip reason (e.g., permission denied, corrupt file) → failure.
					allUnsupported = false
					anyFailed = true
				}
			}
			if allUnsupported {
				pr.Status = "skipped"
				pr.Details = "content type not supported"
			} else {
				pr.Status = "failed"
				pr.Count = len(result.Skipped)
				if len(result.Skipped) > 0 {
					pr.Details = result.Skipped[0].Reason
				}
			}
		default:
			pr.Status = "no-items"
		}

		provResults = append(provResults, pr)

		if !output.Quiet && !output.JSON {
			fmt.Fprintf(output.Writer, "  %s: %s", prov.Name, pr.Status)
			if pr.Count > 0 {
				fmt.Fprintf(output.Writer, " (%d items)", pr.Count)
			}
			if pr.Details != "" {
				fmt.Fprintf(output.Writer, " — %s", pr.Details)
			}
			fmt.Fprintln(output.Writer)
		}
	}

	if output.JSON {
		output.Print(provResults)
		return nil
	}

	if !output.Quiet {
		fmt.Fprintln(output.Writer)
		for _, pr := range provResults {
			status := pr.Status
			if pr.Count > 0 {
				fmt.Fprintf(output.Writer, "  %-20s  %-10s  %d items\n", pr.Provider, status, pr.Count)
			} else {
				fmt.Fprintf(output.Writer, "  %-20s  %-10s  %s\n", pr.Provider, status, pr.Details)
			}
		}
	}

	if anyFailed {
		return output.NewStructuredError(output.ErrInstallNotWritable, "one or more providers had install failures", "Check the output above for details per provider")
	}
	return nil
}
```

**Success criteria:**
- `go test ./cmd/syllago/ -run 'TestInstallToAllNoProvidersDetected'` → pass
- `go test ./cmd/syllago/ -run 'TestInstallToAllAllSkipped'` → pass
- `go test ./cmd/syllago/ -run 'TestInstallToAllDryRun'` → pass
- Error classification matches the documented contract:
  - "does not support" in skip reason → `allUnsupported=true`, `anyFailed` unchanged
  - "project-scoped" in skip reason → `allUnsupported=true`, `anyFailed` unchanged
  - "JSON merge" in skip reason → `allUnsupported=true`, `anyFailed` unchanged
  - Any other reason → `allUnsupported=false`, `anyFailed=true`
- `go test ./cmd/syllago/ -run 'TestInstallToAllPartialSuccess'` → pass (verifies that a permission error — not in the expected-skip list — sets `anyFailed=true` and causes a non-nil return)

---

### Task 1.6 — Run full test suite and fix regressions

**Before starting this task:** Verify existing install tests pass on current `main` before applying any Layer 1 changes. If any test in `TestInstall*` is already failing, fix it first — those failures are not caused by Layer 1 and should not be masked as regressions.

```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./cmd/syllago/ -run 'TestInstall' 2>&1 | tail -5
```

If this shows failures before your changes, stop and fix them before proceeding.

**Commands:**
```bash
cd /home/hhewett/.local/src/syllago/cli && make fmt && make test
```

**Why fmt first:** The pre-commit hook blocks on unformatted Go. Running fmt before testing ensures the working state is clean.

Any failures in the existing `TestInstall*` tests indicate a regression introduced in Tasks 1.2–1.5. Common causes:

- `TestInstallUnknownProvider`: still expects an error when `--to` is set to an unknown slug → should still pass because the provider lookup returns nil and we return the structured error.
- `TestInstallRequiresExplicitIntent`: now also needs `--to` set (since it's no longer required by cobra). Check that the test sets `installCmd.Flags().Set("to", "claude-code")` — if it doesn't, update the test to add it.
- `TestInstallFlagsRegistered`: expects `["to", "type", "method", "dry-run", "base-dir", "no-input", "all"]` — `to-all` is not in this list. No change needed (the new test `TestInstallFlagsRegisteredToAll` handles `to-all` separately).

**Success criteria:**
- `make test` → all tests pass, zero failures
- `make fmt` → exits 0, no diff (code is formatted)

---

### Task 1.7 — Build and smoke test

```bash
cd /home/hhewett/.local/src/syllago && make build
syllago install --help
syllago install --to-all --type skills --dry-run
```

Expected output of `--help`: `--to-all` flag appears in the flag list.
Expected output of `--dry-run` with `--to-all`: lists detected providers and what would be installed to each (or "no providers detected" if none are installed on the machine running the test).

**Success criteria:**
- `make build` → exits 0
- `syllago install --help` → output contains `--to-all`
- `syllago install --to-all --type skills --dry-run` → exit 0, AND output contains either `"Installing"` (providers detected) or `"no providers detected"` (none installed) — both are valid depending on the environment
- `syllago install --to-all --type skills --dry-run 2>&1 | grep -E '(Installing|no providers detected)'` → exit 0
- `syllago install --to claude-code --to-all --type skills --dry-run` → exit non-zero, error output contains "mutually exclusive"

---

### Task 1.8 — Commit Layer 1

```bash
cd /home/hhewett/.local/src/syllago/cli && make fmt
cd /home/hhewett/.local/src/syllago && git add cli/cmd/syllago/install_cmd.go cli/cmd/syllago/install_cmd_test.go
git commit -m "feat(install): add --to-all flag for multi-provider install"
```

**Success criteria:**
- `git log --oneline -1` → commit message starts with `feat(install):`
- Pre-commit hook passes (no unformatted Go, tests pass)

---

## Layer 2: Benchmark Registry Setup (Manual)

This layer is manual verification work, not code. Execute each step in order and verify before proceeding.

### Task 2.1 — Add the benchmark registry

```bash
syllago registry add agent-ecosystem/agent-skill-implementation
```

**Expected:** Git clone to `~/.syllago/registries/agent-skill-implementation/` succeeds. The command exits 0.

**Verification:**
```bash
ls ~/.syllago/registries/agent-skill-implementation/
syllago registry list
```

**Success criteria:**
- `ls ~/.syllago/registries/agent-skill-implementation/` → pass — directory exists with repo contents
- `syllago registry list` → pass — `agent-skill-implementation` appears in output

---

### Task 2.2a — Verify scanner finds skills in `benchmark-skills/`

**Background from design doc:** The benchmark repo's skills live in `benchmark-skills/` not `.agents/skills/`. The syllago scanner does recursive SKILL.md discovery, so it may find them regardless.

```bash
syllago list --type skills --from agent-skill-implementation
```

**Success criteria:**
- `syllago list --type skills --from agent-skill-implementation` → exit 0 — output lists 17+ skills
- If 0 skills returned → **ABORT** — do not proceed to Task 2.3. Run Task 2.2b to debug before continuing.

---

### Task 2.2b — Debug Scanner (run only if Task 2.2a found 0 skills)

**Precondition:** Only execute this task if Task 2.2a returned 0 results.

```bash
ls ~/.syllago/registries/agent-skill-implementation/
find ~/.syllago/registries/agent-skill-implementation/ -name "SKILL.md"
```

Use the output to determine where SKILL.md files are located relative to the repo root.

Document findings in `.develop/benchmark-scanner-debug.md` covering:
- Actual directory layout
- Where SKILL.md files exist
- Whether this is a scanner config issue or a missing SKILL.md in the repo

**Decision point:** Based on findings, either:
1. Fix scanner path configuration and re-run Task 2.2a, OR
2. Note that SKILL.md files are absent from the repo and file an issue before proceeding

**Success criteria:**
- `.develop/benchmark-scanner-debug.md` exists and documents the repo directory structure
- A clear decision is made: fix-and-retry or file-issue-and-abort

---

### Task 2.3 — Import all 17 benchmark skills

```bash
syllago add skills --all --from agent-skill-implementation
```

**Expected:** All 17 benchmark skills imported to `~/.syllago/content/skills/`.

**Verification:**
```bash
syllago list --type skills | grep -c "probe-"
```

Should return at least the probe-* skills (there are multiple).

```bash
syllago list --type skills
```

**Success criteria:**
- Command exits 0
- `syllago list --type skills` → pass — 17 items (or more if library already has skills)
- `ls ~/.syllago/content/skills/ | wc -l` → count includes 17 new entries

---

### Task 2.4 — Install to all detected providers

```bash
syllago install --type skills --to-all
```

**Expected:** Skills installed to each detected provider (claude-code, gemini-cli, cursor, amp minimum — others if installed).

**Verification per provider:**

| Provider | Verify command |
|----------|----------------|
| Claude Code | `ls ~/.claude/skills/ | head -5` |
| Gemini CLI | `ls ~/.gemini/skills/ | head -5` |
| Cursor | `ls ~/.cursor/skills/ | head -5` |
| Amp | `ls ~/.config/agents/skills/ | head -5` |

**Success criteria:**
- `syllago install --type skills --to-all` → exit 0 (or non-zero only if a provider truly fails, not just skips)
- At least one provider skill dir shows skill directories

---

### Task 2.5 — Canary phrase verification (Claude Code)

```bash
# Open a new Claude Code session (not this one — no skill preloading)
claude
```

In the new session, type: `Do you know CARDINAL-ZEBRA-7742?`

**Expected:** Claude Code responds affirmatively, demonstrating the skill was loaded from the skills directory.

**Success criteria:**
- Claude Code session responds "yes" or acknowledges the phrase → pass — skill loading works end-to-end
- Claude Code session has no knowledge of the phrase → fail — skill not loaded, investigate

---

## Layer 3: Workflow Doc

### Task 3.1 — Write the workflow doc

**File:** `docs/research/skill-behavior-checks/workflow-syllago-install.md`

**Content:**

```markdown
# Installing Benchmark Skills via Syllago

Step-by-step guide for using syllago to install the 17 agentskillimplementation.com benchmark skills to all supported AI coding agents.

## Prerequisites

- syllago installed: `syllago --version` shows a version
- At least one target agent installed (Claude Code, Gemini CLI, Cursor, Amp, Windsurf, Roo Code, OpenCode, or Codex)
- For BYOK agents (Roo Code, OpenCode): a Google AI Studio API key from [aistudio.google.com](https://aistudio.google.com/)

## Step 1: Add the Benchmark Registry

Register the benchmark repo as a syllago content source:

```bash
syllago registry add agent-ecosystem/agent-skill-implementation
```

Verify it was added:

```bash
syllago registry list
```

## Step 2: Import Benchmark Skills to Your Library

Import all 17 benchmark skills to your local syllago library:

```bash
syllago add skills --all --from agent-skill-implementation
```

Verify the import:

```bash
syllago list --type skills
```

You should see 17 skills, including `probe-loading`, `probe-linked-resources`, and others.

## Step 3: Install to All Agents

Install the skills to every detected AI coding agent in one command:

```bash
syllago install --type skills --to-all
```

Syllago will:
1. Detect which agents are installed on your system
2. Skip agents that don't support skills (Cline) or use project scope (Kiro)
3. Install to each detected agent, reporting success/skip/failure per agent

## Step 4: Verify Per-Agent Installation

Check that skills appear in each agent's skill directory:

| Agent | Verify |
|-------|--------|
| Claude Code | `ls ~/.claude/skills/` |
| Gemini CLI | `ls ~/.gemini/skills/` |
| Cursor | `ls ~/.cursor/skills/` |
| Amp | `ls ~/.config/agents/skills/` |
| Windsurf | `ls ~/.codeium/windsurf/skills/` |
| Roo Code | `ls ~/.roo/skills/` |
| OpenCode | `ls ~/.config/opencode/skills/` |
| Codex | `ls ~/.agents/skills/` |

## Step 5: Canary Check

Open a fresh session in each agent (no existing context) and ask:

> Do you know CARDINAL-ZEBRA-7742?

A "yes" response confirms the agent loaded the skill from its skills directory.

## Step 6: Run the 28 Behavioral Checks

With skills installed, run the empirical benchmark. See the [benchmark plan](./benchmark-plan.md) for the full check matrix.

**Model selection:** Most checks (22 of 28) test platform behavior — any model works. The 6 model-sensitive checks (`cross-skill-invocation`, `invocation-depth-limit`, `invocation-language-sensitivity`, `circular-invocation-handling`, `informal-dependency-resolution`, `missing-dependency-behavior`) may differ between models. Run cheapest-available first, then re-run with a capable model for the delta.

## Agent Coverage Notes

**Excluded from syllago install:**
- **Cline**: syllago's Cline provider does not support the Skills content type
- **Kiro**: skills are project-scoped (`.kiro/steering/`), not user-scoped
- **Junie CLI**: no syllago provider
- **GitHub Copilot (VS Code extension)**: syllago supports `copilot-cli` (the CLI), not the VS Code extension

These agents require manual skill installation.

## Troubleshooting

**Skills not detected after install:**
- Verify the agent reads from the skills directory at startup (not session-wide context)
- Some agents require a session restart to pick up new skills

**"no providers detected" from --to-all:**
- Run `syllago install --to <slug>` for a specific agent to confirm the path configuration
- Use `syllago config paths --provider <slug> --path /your/path` if installed at a non-default location
```

**Success criteria:**
- `test -f docs/research/skill-behavior-checks/workflow-syllago-install.md` → exit 0 (file exists)
- `grep -c '^\`\`\`bash' docs/research/skill-behavior-checks/workflow-syllago-install.md` → returns a count ≥ 6 (one bash block per step)
- `grep 'to-all' docs/research/skill-behavior-checks/workflow-syllago-install.md` → exit 0 (Layer 1 flag present)
- `grep 'registry add' docs/research/skill-behavior-checks/workflow-syllago-install.md` → exit 0 (Layer 2 registry step present)
- `grep 'CARDINAL-ZEBRA-7742' docs/research/skill-behavior-checks/workflow-syllago-install.md` → exit 0 (canary phrase documented)

---

## Layer 4: Roadmap Bead

### Task 4.1 — Create the multi-provider loadout bead

```bash
cd /home/hhewett/.local/src/syllago
bd create
```

When prompted, use:
- **Name:** Multi-provider loadout format
- **Description:** Support a `providers: [all | list]` field in loadout.yaml to install a loadout to multiple providers in one command. Design should cover: provider-specific overrides, partial install (skip unsupported types), rollback on failure, and the `providers: all` magic string as equivalent to `--to-all`.

**Success criteria:**
- `bd list` → new bead appears
- Bead is in the appropriate state (todo/design)

---

### Task 4.2 — Update README.md roadmap section

**File:** `README.md`

Find the roadmap section (search for "Roadmap" or "Planned" heading) and add an entry for multi-provider loadouts.

**Entry to add:**
```markdown
- **Multi-provider loadouts** — Define a loadout once and deploy it to your entire tool fleet. Specify `providers: all` in `loadout.yaml` to install to every detected agent, or list specific slugs for targeted distribution.
```

**Success criteria:**
- README.md contains the new roadmap entry
- `git diff README.md` → shows the addition

---

## Commit Sequence

```
feat(install): add --to-all flag for multi-provider install    ← Layer 1
docs: workflow guide for benchmark skill install via syllago   ← Layer 3
chore(roadmap): add multi-provider loadout bead and entry      ← Layer 4
```

Layer 2 is manual verification with no commit — it produces observations that feed Layer 3.

---

## Coverage Check

After Layer 1:

```bash
cd /home/hhewett/.local/src/syllago/cli && \
  go test ./cmd/syllago/... -coverprofile=cov.out && \
  go tool cover -func=cov.out | grep -E '(install_cmd|total)'
```

Target: `install_cmd.go` coverage stays above 80%. The new `runInstallToAll` and `installToProvider` functions are covered by the new tests added in Task 1.1.
