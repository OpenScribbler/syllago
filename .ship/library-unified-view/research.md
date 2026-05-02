# Research: library-unified-view

## Q1: How does app.go currently pass items to the library sub-model SetItems? What is the exact call site and what slice does it pass?

There is one call site for `library.SetItems` in `app.go`:

- `cli/internal/tui/app.go:358` — inside `refreshContent()`:
  ```go
  a.library.SetItems(a.catalog.Items)
  ```
  It passes `a.catalog.Items`, which is `[]catalog.ContentItem` — the full unsegmented catalog slice containing all item types, all sources (Library, Project Content, and Registry Clone items combined).

`refreshContent()` at `app.go:354–375` only calls `library.SetItems` when `a.isLibraryTab()` is true. The isLibraryTab check is `ActiveGroupLabel() == "Collections" && ActiveTabLabel() == "Library"` (`app.go:220–222`).

`newLibraryModel` at `app.go:176` also passes `cat.Items` (the same unfiltered slice) at construction time:
```go
library: newLibraryModel(cat.Items, providers, projectRoot),
```
`cli/internal/tui/library.go:74–79`.

`libraryModel.SetItems` at `cli/internal/tui/library.go:130–134` delegates directly to `l.table.SetItems(items)` and resets `mode = libraryBrowse`, `detailItem = nil`. No filtering occurs inside `SetItems`.

---

## Q2: What fields on catalog.ContentItem discriminate between Library items, Project Content, and Registry Clone items — and what are their exact types, zero-values, and invariants?

All fields are defined at `cli/internal/catalog/types.go:67–119`.

| Field | Type | Zero value | Discrimination rule |
|---|---|---|---|
| `Library` | `bool` | `false` | `true` iff item lives in `~/.syllago/content/` (global content library). Set to `true` at `cli/internal/catalog/scanner.go:1268` for every item discovered under `GlobalContentDir()`. |
| `Registry` | `string` | `""` | Non-empty string (the registry name) iff item came from a git registry clone (`~/.syllago/registries/<name>/`). Set at `scanner.go:111` and `scanner.go:129` for items discovered via `ScanWithRegistries` or `ScanRegistriesOnly`. |
| `Source` | `string` | `""` | Human-readable tag: `"library"` for Library items, `"project"` or `""` for Project Content, registry name for Registry Clone items. Set at `scanner.go:1223–1229`. For MOAT-materialized unstaged items, `Source` equals the registry name but `Path` is empty. |
| `Path` | `string` | `""` | Absolute filesystem path to the item directory or file. Always non-empty for scanner-discovered items. Empty only for MOAT-materialized items that have not been fetched yet (discriminated in `isUnstagedRegistryItem` at `library.go:501–503`). |

**Precise item categories:**

- **Library item**: `item.Library == true`. Path is under `~/.syllago/content/`. Registry is typically `""` (unless explicitly tainted).
- **Project Content** (shared): `item.Library == false && item.Registry == ""`. Path is under the project root.
- **Registry Clone item**: `item.Registry != ""`. Subdivided: if `item.Path == "" && item.Source != ""` (i.e. `isUnstagedRegistryItem`) it is a MOAT-materialized item with no on-disk files yet; otherwise it is a standard git-registry clone item with `item.Path` pointing into the registry checkout.

The `IsBuiltin()` method at `types.go:135–145` checks `item.Meta.Tags` for the `"builtin"` tag; builtin items are a subset of Library items.

---

## Q3: How are the current action buttons rendered in metapanel.go? What is the full conditional block that decides which buttons appear, including isUnstagedRegistryItem?

Buttons are built at `cli/internal/tui/metapanel.go:229–240`:

```go
var btns []string
if data.canInstall {
    btns = append(btns, zone.Mark("meta-install", activeButtonStyle.Render("[i] Install")))
}
if data.installed != "--" {
    btns = append(btns, zone.Mark("meta-uninstall", activeButtonStyle.Render("[x] Uninstall")))
}
if item.Library || item.Registry == "" {
    btns = append(btns, zone.Mark("meta-remove", activeButtonStyle.Render("[d] Remove")))
}
btns = append(btns, zone.Mark("meta-edit", activeButtonStyle.Render("[e] Edit")))
```

- `[i] Install` — shown when `data.canInstall` is true. `canInstall` is computed in `computeMetaPanelData` at `metapanel.go:55–63`:
  ```go
  canInstall := false
  if item.Library || item.Registry == "" || isUnstagedRegistryItem(&item) {
      for _, prov := range providers {
          if installer.CheckStatus(item, prov, repoRoot) != installer.StatusInstalled {
              canInstall = true
              break
          }
      }
  }
  ```
  Registry Clone items that are not `isUnstagedRegistryItem` never set `canInstall = true`.

- `[x] Uninstall` — shown when `data.installed != "--"`, i.e. at least one provider abbreviation was appended (`metapanel.go:232–234`).

- `[d] Remove` — shown when `item.Library || item.Registry == ""` (`metapanel.go:235–237`). That is, Library items and Project Content items show Remove; Registry Clone items do not.

- `[e] Edit` — always shown (`metapanel.go:238–239`).

`isUnstagedRegistryItem` is defined at `library.go:501–503`:
```go
func isUnstagedRegistryItem(item *catalog.ContentItem) bool {
    return item != nil && item.Path == "" && item.Source != ""
}
```

---

## Q4: How does list_cmd.go currently filter and output items — what flags exist, how is per-row output formatted, and where is the output loop?

Source: `cli/cmd/syllago/list.go`.

**Flags** (`list.go:34–38`):
- `--source` (string, default `"all"`) — accepted values: `library`, `shared`, `registry`, `builtin`, `all`.
- `--type` (string, default `""`) — single content type filter (e.g. `skills`, `rules`).

**Filtering flow** (`list.go:60–111`):
1. `moat.LoadAndScan` is called to produce the catalog.
2. Outer loop iterates `catalog.AllContentTypes()`, skipping types that don't match `--type` if set.
3. Inner loop iterates `cat.ByType(ct)`, calling `filterBySource(item, sourceFilter)` per item (`helpers.go:153–170`). `filterBySource` dispatches on the source string:
   - `"library"`: `item.Library`
   - `"shared"`: `!item.Library && item.Registry == "" && !item.IsBuiltin()`
   - `"registry"`: `item.Registry != ""`
   - `"builtin"`: `item.IsBuiltin()`
   - `"all"` or default: `true`

**Per-row output** (`list.go:131–143`):
```go
for i, group := range result.Groups {
    if i > 0 {
        fmt.Fprintln(output.Writer)
    }
    fmt.Fprintf(output.Writer, "%s (%d)\n", group.Type, group.Count)
    for _, item := range group.Items {
        glyph := trustGlyph(item.Trust)
        fmt.Fprintf(output.Writer, "  %-2s %-18s [%-8s] %s\n",
            glyph, item.Name, item.Source, item.Description)
    }
}
```
Format: 2-char trust glyph column (`✓`/`R`/blank), 18-char name column, 8-char source tag in brackets, description.

**JSON output** (`list.go:121–124`): if `output.JSON` is true, marshals a `listResult` struct (`list.go:41–58`) with `Groups []listGroup`, each with `Type`, `Count`, `Items []listItem`. `listItem` has `Name`, `Source`, `Description`, `Trust`, `TrustTier`, `Revoked`.

---

## Q5: What flags does add_cmd.go currently accept, and how does the install step (if any) get triggered today — what is the entry point into installer.Install from the add pipeline?

**Flags** declared at `cli/cmd/syllago/add_cmd.go:52–73`:
- `--from` (string array, required) — provider slug(s) or monolithic file path(s)
- `--split` (string) — splitter heuristic for monolithic mode
- `--all` (bool) — add all discovered content
- `--dry-run` (bool)
- `--exclude` (string array) — skip hooks/MCP by name
- `--scope` (string, default `"all"`) — settings scope for hooks/MCP
- `-f`/`--force` (bool) — overwrite without prompting
- `--base-dir` (string) — override base directory
- `--no-input` (bool) — disable interactive prompts
- `--name` (string) — display name for hooks/MCP metadata
- `--source-registry` (string, hidden) — registry name for taint tracking
- `--source-visibility` (string, hidden) — source registry visibility
- `--trusted-root` (string) — path to Sigstore trusted_root.json

**Install step**: `add_cmd.go` does NOT trigger `installer.Install`. The add pipeline only writes content into the Library (`~/.syllago/content/`). The call chain is:

`runAdd` → `add.AddItems` (`add_cmd.go:304`) → `writeItem` (`add/add.go:206`) — which writes files to `globalDir/<type>/<provider>/<name>/` (or `globalDir/<type>/<name>/` for universal types). `installer.Install` is never called from the add pipeline.

The install step requires a separate `syllago install --to <provider>` CLI command or the TUI install wizard's `doInstallCmd` (`actions.go:829–876`) which calls `installer.Install` at `actions.go:863`.

---

## Q6: In registry_cmd.go and actions.go, what is the full success path after a registry add — where does control return and what output is emitted?

**CLI path** (`cli/cmd/syllago/registry_cmd.go`):

`registryAddCmd.RunE` at `registry_cmd.go:53–231`:
1. Resolves alias, signing profile, prints security banner.
2. Calls `registryops.AddRegistry(cmd.Context(), opts)` (`registry_cmd.go:138`).
3. On success: prints `"Visibility: public|private"` and/or `"MOAT compliance detected via registry.yaml."` (`registry_cmd.go:148–155`).
4. Prints `"Added registry: %s\n"` with `outcome.Registry.Name` (`registry_cmd.go:157`).
5. If `outcome.Registry.IsMOAT()`: optionally chains `syncMOATRegistry` (`registry_cmd.go:165–194`).
6. Prompts sandbox allowlist question (`registry_cmd.go:197–227`).
7. Returns `nil`.

**TUI path** (`cli/internal/tui/actions.go`):

`doRegistryAddCmd` (`actions.go:430–450`) dispatches `registryops.AddRegistry` in a `tea.Cmd`. On success it returns `registryAddDoneMsg{name: outcome.Registry.Name, isMOAT: outcome.Registry.IsMOAT()}`.

`handleRegistryAddDone` (`actions.go:483–502`):
- If `msg.isMOAT`: keeps `registryOpInProgress=true`, pushes toast `"Added <name> — verifying signing identity..."`, chains `doMOATSyncCmd`.
- If not MOAT: sets `registryOpInProgress = false`, pushes toast `"Added registry: <name>"`, calls `rescanCatalog()`.

---

## Q7: How are INSTALL_002 and ITEM_001 errors currently constructed and emitted — what is the call site, what does the suggestion field currently contain, and are there any callers that branch on the error code?

**INSTALL_002 (`ErrInstallItemNotFound`)** — call sites:

1. `cli/cmd/syllago/install_cmd.go:308` — in item-lookup loop when no match found by name:
   - Message: `"no item named %q found in your library"` (with `args[0]`)
   - Suggestion: `"Hint: syllago list --type "+hint` (hint defaults to `"skills"` if no type filter set)

2. `cli/cmd/syllago/install_cmd.go:552` — second lookup path (batch install variant):
   - Message: `"no item named %q found in your library"` (with `args[0]`)
   - Suggestion: `"Hint: syllago list --type "+hint`

3. `cli/cmd/syllago/install_cmd_append.go:88–93` — when no rule found for append:
   - Message: `"no rule named %q found in your library"` (with `args[0]`)
   - Suggestion: `"Hint: syllago list --type rules"`

4. `cli/cmd/syllago/install_moat.go:268–273` — when manifest does not list item by name:
   - Message: `"registry %q does not list an item named %q in its manifest"`
   - Suggestion: `` "Run `syllago registry items <name>` to see available content." ``

5. `cli/cmd/syllago/uninstall_cmd.go:106–108` — when item not found for uninstall:
   - Message: `"no item named %q found in your library"` (with `name`)
   - Suggestion: `"Hint: syllago list    (show all library items)"`

6. `cli/cmd/syllago/uninstall_cmd.go:256–261` — when library rule not found for D7 append uninstall:
   - Message: `"library rule %q not found for uninstall"` (with `name`)
   - Suggestion: `"Library rules required for D7 exact-match uninstall live under ~/.syllago/content/rules/<source>/<name>/"`
   - Uses `NewStructuredErrorDetail` (has `Details` field)

No callers in the non-test codebase branch on `ErrInstallItemNotFound` / `"INSTALL_002"` by code value. The errors are returned up the cobra chain and terminate the command.

**ITEM_001 (`ErrItemNotFound`)** — call sites (non-test, non-add-pipeline):

1. `cli/cmd/syllago/remove_cmd.go:90` — item not found in library for remove:
   - Message: `"no item named %q found in your library"` (with `name`)
   - Suggestion: `"Run 'syllago list' to show all library items"`

2. `cli/cmd/syllago/convert_cmd.go:144` — item not found in library for convert:
   - Message: `"no item named %q in your library"` (with `name`)
   - Suggestion: `"Run 'syllago list' to see all library items"`

3. `cli/cmd/syllago/convert_cmd.go:154` — content file not locatable for item:
   - Message: `"cannot locate content file for %s"` (with `name`)
   - Suggestion: `"Ensure the item has a primary content file"`

4. `cli/cmd/syllago/edit_cmd.go:93` — item not found for edit.
5. `cli/cmd/syllago/share_cmd.go:166` — item not found for share.
6. `cli/cmd/syllago/compat_cmd.go:69` — item not found for compat.
7. `cli/cmd/syllago/inspect.go:456` — content file not locatable.
8. `cli/cmd/syllago/add_cmd.go:287`, `add_cmd.go:1209`, `add_cmd.go:1263` — item not found by name/type filter during add.
9. `cli/cmd/syllago/add_cmd_monolithic.go:67` — monolithic add item not found.

No callers in the non-test codebase branch on `ErrItemNotFound` / `"ITEM_001"` by code value. All are terminal errors returned up the cobra chain.

---

## Q8: How does syllago currently read and write ~/.syllago/config.json — what struct/package owns it, and how are boolean flags persisted and read back?

**Owning package**: `cli/internal/config` (`config.go`).

**Struct**: `config.Config` at `config.go:260–268`:
```go
type Config struct {
    Providers         []string                      `json:"providers"`
    ContentRoot       string                        `json:"content_root,omitempty"`
    Registries        []Registry                    `json:"registries,omitempty"`
    AllowedRegistries []string                      `json:"allowed_registries,omitempty"`
    Preferences       map[string]string             `json:"preferences,omitempty"`
    Sandbox           SandboxConfig                 `json:"sandbox,omitempty"`
    ProviderPaths     map[string]ProviderPathConfig `json:"provider_paths,omitempty"`
}
```

`Config` has **no boolean fields**. All flags and toggles that need persistence are currently stored as string values in `Preferences map[string]string` (the map is keyed by arbitrary string keys, values are strings).

**Read path**: `LoadGlobal()` at `config.go:367–374` → `LoadFromPath()` at `config.go:376–391` → `os.ReadFile` + `json.Unmarshal`. Returns `&Config{}` (empty, not error) when the file does not exist (`fs.ErrNotExist` case).

**Write path**: `SaveGlobal(cfg *Config)` at `config.go:393–422`. Uses atomic write: `json.MarshalIndent` → write to `target.tmp.<random>` → `os.Rename` to `target`. Directory created with `os.MkdirAll(dir, 0755)` if absent.

**Project-level config**: `Load(projectRoot string)` at `config.go:293–306` and `Save(projectRoot string, cfg *Config)` at `config.go:308–334` — same JSON shape, stored at `<projectRoot>/.syllago/config.json`.

**Global dir path**: `config.GlobalDirPath()` at `config.go:346–355` returns `~/.syllago/` (or `GlobalDirOverride` in tests). File path is `~/.syllago/config.json`.

**No boolean flags exist on `Config`**. Telemetry consent state is owned by the `telemetry` package (separate from `config`), and its persistence mechanism is separate from `config.json`.

---

## Cross-cutting observations

- `catalog.ContentItem.Source` (`types.go:79`) is a human-readable tag (`"project"`, `"global"`, `"library"`, or registry name) distinct from and overlapping with `Library bool` and `Registry string`. The three fields are set independently: `Source` is set by `ScanWithGlobalAndRegistries` (`scanner.go:1220–1228`), while `Library` is set only by that function for global items (`scanner.go:1268`). A Library item has `Library=true`, `Source="global"`, `Registry=""`.

- The `tableModel` inside `libraryModel` receives the full `catalog.Items` slice including non-Library items. Whether non-Library items actually appear in the table depends on how `tableModel.SetItems` filters or presents them — that filtering, if any, lives in `table.go`, not in `library.go` or `app.go`.

- `list.go` uses `filterBySource` with a default case of `return item.Library` when source is not one of the recognized values — an unknown `--source` value silently falls back to library-only behavior (`helpers.go:168`).

- `add.AddItems` writes to `~/.syllago/content/` but never calls `installer.Install`. The install action is a subsequent, separate step — there is no "add and install in one shot" path in the CLI or the add wizard's `add.AddItems` call chain.

- `config.Config` has no boolean fields. The `Preferences map[string]string` is the only current mechanism for persisting arbitrary string-valued user settings in `config.json`.

RESEARCH_COMPLETE
