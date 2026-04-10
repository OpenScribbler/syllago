---
paths: ["cli/**/*.go"]
---

# CLI Test Patterns (Non-TUI)

## Provider Stubs

Hand-crafted structs — no mocking library. Hooks return `"__json_merge__"` from `InstallDir`.

```go
prov := provider.Provider{
    Name:      "Claude Code",
    Slug:      "claude-code",
    ConfigDir: ".claude",
    InstallDir: func(home string, ct catalog.ContentType) string {
        switch ct {
        case catalog.Rules:
            return filepath.Join(home, ".claude", "rules")
        case catalog.Hooks:
            return "__json_merge__"
        }
        return ""
    },
    SupportsType: func(ct catalog.ContentType) bool {
        return ct == catalog.Rules || ct == catalog.Hooks
    },
    Detected: true, // optional — set when provider detection matters
}
```

## Global State and Cleanup

Six globals that tests mutate. Always save-restore with `t.Cleanup` (runs even on `t.Fatal`).

| Global | Package | Type |
|--------|---------|------|
| `provider.AllProviders` | provider | `[]Provider` |
| `output.JSON` | output | `bool` |
| `output.ErrWriter` | output | `io.Writer` |
| `findProjectRoot` | cmd/syllago | `func() (string, error)` |
| `catalog.GlobalContentDirOverride` | catalog | `string` |
| `registry.CacheDirOverride` | registry | `string` |

Pattern:

```go
orig := catalog.GlobalContentDirOverride
catalog.GlobalContentDirOverride = tempDir
t.Cleanup(func() { catalog.GlobalContentDirOverride = orig })
```

For `provider.AllProviders`, copy the slice before mutating:

```go
orig := append([]provider.Provider(nil), provider.AllProviders...)
provider.AllProviders = append(provider.AllProviders, testProv)
t.Cleanup(func() { provider.AllProviders = orig })
```

## Cobra Command Testing

```go
stdout, _ := output.SetForTest(t)  // captures stdout + stderr, restores on cleanup
withFakeRepoRoot(t, root)          // overrides findProjectRoot

err := inspectCmd.RunE(inspectCmd, []string{"skills/my-skill"})
out := stdout.String()
```

**Setting flags:** Use `cmd.Flags().Set()` and reset in cleanup to avoid cross-test pollution:

```go
installCmd.Flags().Set("to", "claude-code")
defer installCmd.Flags().Set("to", "")
```

**JSON output:** Set `output.JSON = true` (restored by `SetForTest` cleanup), then parse:

```go
output.JSON = true
err := listCmd.RunE(listCmd, []string{})
var result MyStruct
json.Unmarshal(stdout.Bytes(), &result)
// or for specific fields:
gjson.GetBytes(stdout.Bytes(), "hooks.SessionEnd").String()
```

## Filesystem Fixtures

Always `t.TempDir()` (auto-cleaned). Standard permissions: dirs `0755`, files `0644`.

```go
root := t.TempDir()
os.MkdirAll(filepath.Join(root, "skills", "my-skill"), 0755)
os.WriteFile(filepath.Join(root, "skills", "my-skill", "SKILL.md"), []byte("# Skill"), 0644)
```

**Symlink verification** — use `os.Lstat` (not `os.Stat`, which follows symlinks):

```go
info, err := os.Lstat(symlinkPath)
if info.Mode()&os.ModeSymlink == 0 {
    t.Error("expected symlink")
}
```

**Snapshot HOME gotcha:** The snapshot package stores paths relative to `os.UserHomeDir()`. Tests exercising snapshots must create provider config dirs under the real home, not arbitrary temp dirs — otherwise `filepath.Rel` produces `../../tmp/...` paths that corrupt the snapshot directory.

```go
home, _ := os.UserHomeDir()
homeDir := filepath.Join(home, ".syllago-inttest-"+safeName)
os.MkdirAll(homeDir, 0755)
t.Cleanup(func() { os.RemoveAll(homeDir) })
```

## Integration Test Patterns

**Loadout roundtrip** (`setupIntegrationEnv` in `loadout/integration_test.go`): Creates homeDir (real home subdir) + projectRoot (temp dir) with rule content, hook content, manifest, catalog, and provider stub. Tests the full lifecycle: apply -> verify symlinks/hooks/snapshot -> remove -> verify restore.

**Registry roundtrip** (`createBareRepo` in `registry/integration_test.go`): Creates bare git repos with content layouts via `writeFile` + `git init` + `git clone --bare`. Supports layout variants: `"valid"`, `"kitchen-sink"`, `"native"`, `"empty"`. Network tests gated behind `SYLLAGO_TEST_NETWORK=1`:

```go
if os.Getenv("SYLLAGO_TEST_NETWORK") == "" {
    t.Skip("set SYLLAGO_TEST_NETWORK=1 to run network-dependent tests")
}
```

## Test Isolation

- Use `t.Parallel()` by default. Skip only when mutating globals (`provider.AllProviders`, `output.JSON`, `findProjectRoot`, etc.).
- Each test creates its own fixtures — no shared test data, no ordering dependencies.
- `t.Helper()` on all shared helper functions.
- Prefer `gjson.GetBytes()` for specific JSON field assertions over full `json.Unmarshal`.
