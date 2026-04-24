# MOAT Auto-Detection — Implementation Plan

**Goal:** Registries that are MOAT-compliant are automatically detected and verified without requiring `--moat` or `--signing-identity` flags from the operator.

**Architecture:**
1. Bundled allowlist gains a `manifest_uri` field per entry. For well-known registries (syllago-meta-registry), `registry add` auto-sets `type: moat`, `manifest_uri`, and `signing_profile` — no flags needed, no TOFU.
2. `registry.yaml` gains a `manifest_uri` field. Any registry can self-declare MOAT compliance. Syllago detects it at `registry add` time and sets `type: moat` + `manifest_uri`; first sync requires `--yes` (TOFU, per spec).
3. `registry sync` upgrades existing plain git registries that match the allowlist or self-declare via `registry.yaml`. Only two registry types exist: `git` (default/empty) and `moat` — these are the only values `r.IsGit()` and `r.IsMOAT()` key on.
4. Meta-registry bootstrap: add `moat.yml` (publisher action), `moat-registry.yml` (registry action), and `registry.yml` (MOAT operator config) to `syllago-meta-registry`.
5. `registry list` shows a TRUST column: `moat` when type is MOAT + synced, `pending` when MOAT but not yet synced, `─` for git registries.

**Spec constraints (moat-spec.md):**
- Detection mechanism: bundled allowlist = syllago's pre-established signing identity (equivalent to Registry Index for known registries)
- TOFU: exit non-zero for unknown registries; allowlist-matched registries are pre-trusted (no TOFU)
- `manifest_uri` in the manifest is for substitution-attack detection; clients discover URI via allowlist or self-declaration
- `registry.yaml manifest_uri` is a syllago extension (not spec-normative) enabling self-declaration

**Tech Stack:** Go 1.23, cobra, bubbletea, moat package, `signing_identities.json` (embed)

**Note on repos:** syllago (`/home/hhewett/.local/src/syllago`) and syllago-meta-registry (`/home/hhewett/.local/src/syllago-meta-registry`) are separate git repos. Tasks 1–6 and 8–9 target syllago. Task 7 targets syllago-meta-registry. Commits are separate.

---

## Task 1: Add ManifestURI to allowlist entry struct and JSON

**Files:**
- Modify: `cli/internal/moat/signing_identities_loader.go`
- Modify: `cli/internal/moat/signing_identities.json`

**Depends on:** nothing

### Success Criteria
- `grep -q '"manifest_uri"' cli/internal/moat/signing_identities.json` → pass — JSON has manifest_uri field
- `cd cli && go build ./internal/moat/...` → pass — compiles with AllowlistEntry type
- `cd cli && go test ./internal/moat/ -run "TestLookupSigningIdentity|TestParseSigningIdentities" -v` → pass — all existing tests pass after updating field access paths in Step 6 (confirms compilation and field-renaming correctness; `ManifestURI` population is verified in Task 8 via `TestLookupSigningIdentity_MetaRegistryHasManifestURI`)

---

### Step 1: Change signingIdentityEntry to include ManifestURI

In `cli/internal/moat/signing_identities_loader.go`, change the entry struct:

```go
// signingIdentityEntry is the on-disk JSON shape of a single allowlist entry.
type signingIdentityEntry struct {
	RegistryURL string                `json:"registry_url"`
	Description string                `json:"description,omitempty"`
	ManifestURI string                `json:"manifest_uri,omitempty"` // HTTPS URL of the MOAT registry manifest; empty = allowlist predates this field
	Profile     config.SigningProfile `json:"profile"`
}
```

### Step 2: Introduce AllowlistEntry as the lookup return type

Replace the package-level vars and index type:

```go
// AllowlistEntry is the result of a successful LookupSigningIdentity call.
// ManifestURI may be empty for allowlist entries that predate the field —
// callers must guard with `entry.ManifestURI != ""` before using it.
type AllowlistEntry struct {
	Profile     *config.SigningProfile
	ManifestURI string
}

var (
	identityIndex     map[string]*AllowlistEntry
	identityIndexOnce sync.Once
)
```

### Step 3: Update LookupSigningIdentity to return AllowlistEntry

```go
// LookupSigningIdentity reports the known-good signing profile and manifest URI
// for the given registry URL, if one is bundled. ManifestURI may be empty for
// entries added before the manifest_uri field existed — callers guard with != "".
//
// Returns (nil, false) when no match is found.
func LookupSigningIdentity(registryURL string) (*AllowlistEntry, bool) {
	idx := loadSigningIdentityIndex()
	key := normalizeRegistryURL(registryURL)
	if key == "" {
		return nil, false
	}
	e, ok := idx[key]
	if !ok {
		return nil, false
	}
	clone := AllowlistEntry{ManifestURI: e.ManifestURI}
	profileClone := *e.Profile
	clone.Profile = &profileClone
	return &clone, true
}
```

### Step 4: Update parseSigningIdentities to build map[string]*AllowlistEntry

Replace the `idx := make(...)` block in `parseSigningIdentities`:

```go
idx := make(map[string]*AllowlistEntry, len(doc.Identities))
for i, e := range doc.Identities {
	key := normalizeRegistryURL(e.RegistryURL)
	// ... all existing validation unchanged ...
	p := e.Profile
	idx[key] = &AllowlistEntry{
		Profile:     &p,
		ManifestURI: e.ManifestURI,
	}
}
return idx, nil
```

Also update `loadSigningIdentityIndex` return type to `map[string]*AllowlistEntry`.

### Step 5: Update signing_identities.json

Update subject_regex to match both `main` and `master` branches. The GitHub OIDC subject includes the ref at workflow execution time: `moat.yml` fires on `main` (reference workflow default), but the existing entry was written with `master`. Both must match to cover all deployments. Add `manifest_uri`:

```json
{
  "_version": 1,
  "description": "Bundled allowlist of known-good MOAT signing identities keyed by registry URL. Entries here skip the TOFU prompt and CLI --signing-identity flags; a match at `syllago registry add` time auto-pins the profile. Inclusion criteria: the registry is published by a well-known operator, its signing workflow is publicly inspectable, and its GitHub OIDC numeric IDs are recorded here so repo-transfer forgery is blocked.",
  "identities": [
    {
      "registry_url": "https://github.com/OpenScribbler/syllago-meta-registry",
      "description": "syllago official meta-registry (OpenScribbler/syllago-meta-registry)",
      "manifest_uri": "https://raw.githubusercontent.com/OpenScribbler/syllago-meta-registry/moat-registry/registry.json",
      "profile": {
        "issuer": "https://token.actions.githubusercontent.com",
        "subject_regex": "^https://github\\.com/OpenScribbler/syllago-meta-registry/\\.github/workflows/(moat|moat-registry)\\.yml@refs/heads/(main|master)$",
        "profile_version": 1,
        "repository_id": "1193220959",
        "repository_owner_id": "263775997"
      }
    }
  ]
}
```

### Step 6: Update existing tests in signing_identities_loader_test.go for new return type, then run

`LookupSigningIdentity` now returns `*AllowlistEntry` instead of `*config.SigningProfile`. Two existing tests access profile fields directly on the returned pointer and must be updated:

**`TestLookupSigningIdentity` (~line 39-46):** Change `p.Issuer` → `p.Profile.Issuer` and `p.SubjectRegex` → `p.Profile.SubjectRegex`.

**`TestLookupSigningIdentity_ResultIsCopy` (~line 126-138):** Change `p1.Issuer` → `p1.Profile.Issuer` and `p2.Issuer` → `p2.Profile.Issuer` throughout. The mutation test `p1.Issuer = "..."` becomes `p1.Profile.Issuer = "..."`.

```bash
cd cli && go test ./internal/moat/ -run TestParseSigningIdentities -v
cd cli && go test ./internal/moat/ -run TestLookupSigningIdentity -v
```

---

## Task 2: Propagate ManifestURI through signingResolution

**Files:**
- Modify: `cli/cmd/syllago/registry_signing.go`

**Depends on:** Task 1

### Success Criteria
- `grep -q "ManifestURI string" cli/cmd/syllago/registry_signing.go` → pass — field added to signingResolution
- `cd cli && go build ./cmd/syllago/` → pass — single callsite in registry_cmd.go compiles with new field
- `cd cli && go test ./cmd/syllago/ -run TestResolveSigningProfile` → pass — allowlist case populates ManifestURI

---

### Step 1: Add ManifestURI to signingResolution

```go
// signingResolution is the output of resolveSigningProfile: either a
// populated profile + source label + manifest URI, or a (nil, "") pair
// meaning the caller should continue in legacy git-mode.
type signingResolution struct {
	Profile     *config.SigningProfile
	ManifestURI string // non-empty only for "allowlist" source; empty for "flags" and legacy git
	Source      string // "allowlist", "flags", or "" when legacy git
}
```

### Step 2: Update the allowlist lookup callsite in resolveSigningProfile

The existing code at line ~81:
```go
allowlistProfile, hasAllowlistEntry := moat.LookupSigningIdentity(gitURL)
```
becomes:
```go
allowlistEntry, hasAllowlistEntry := moat.LookupSigningIdentity(gitURL)
```

Then update Case 3 (allowlist match) to populate ManifestURI:
```go
// Case 3: no --signing-identity, but allowlist has a match → auto-pin.
if hasAllowlistEntry {
	return &signingResolution{
		Profile:     allowlistEntry.Profile,
		ManifestURI: allowlistEntry.ManifestURI, // may be empty for pre-manifest_uri entries
		Source:      "allowlist",
	}, nil
}
```

Case 1 (no MOAT intent) and Case 2 (flags) leave ManifestURI empty. For flag-sourced profiles, the manifest URI is not known until after the first sync — the manifest itself declares it, and the client verifies it matches.

The single callsite that reads `signingResolution` is in `registry_cmd.go` at ~line 211 — Task 3 shows exactly how ManifestURI is consumed there.

### Step 3: Fix existing callsites in registry_signing_test.go

`moat.LookupSigningIdentity` now returns `*AllowlistEntry`. Two tests in `registry_signing_test.go` call it and use the result directly as `*config.SigningProfile` — this is a type error after Task 1.

**`TestDescribeProfileSource_ProducesHumanText` (~line 257):**
```go
// Before:
p, _ := moat.LookupSigningIdentity(metaRegistryURL)
if p == nil { t.Fatal(...) }
allowlistMsg := describeProfileSource(&signingResolution{Profile: p, Source: "allowlist"}, ...)
flagsMsg := describeProfileSource(&signingResolution{Profile: p, Source: "flags"}, ...)

// After:
entry, _ := moat.LookupSigningIdentity(metaRegistryURL)
if entry == nil { t.Fatal("meta-registry missing from allowlist") }
allowlistMsg := describeProfileSource(&signingResolution{Profile: entry.Profile, Source: "allowlist"}, ...)
flagsMsg := describeProfileSource(&signingResolution{Profile: entry.Profile, Source: "flags"}, ...)
```

**`TestDescribeProfileSource_UnknownSourceIsSilent` (~line 276):**
```go
// Before:
p, _ := moat.LookupSigningIdentity(metaRegistryURL)
if msg := describeProfileSource(&signingResolution{Profile: p, Source: "future-source"}, ...); ...

// After:
entry, _ := moat.LookupSigningIdentity(metaRegistryURL)
if msg := describeProfileSource(&signingResolution{Profile: entry.Profile, Source: "future-source"}, ...); ...
```

---

## Task 3: Set ManifestURI in registry add (allowlist path)

**Files:**
- Modify: `cli/cmd/syllago/registry_cmd.go` lines ~211-214

**Depends on:** Task 2

### Success Criteria
- `grep -A2 "newRegistry.Type = config.RegistryTypeMOAT" cli/cmd/syllago/registry_cmd.go | grep -q "newRegistry.ManifestURI ="` → pass — assignment present immediately after Type assignment
- `cd cli && go build ./cmd/syllago/` → pass

---

### Step 1: Set ManifestURI alongside Type and SigningProfile

Change the existing block in registry_cmd.go RunE at ~line 211:

```go
// Before (existing):
if signing != nil && signing.Profile != nil {
	newRegistry.Type = config.RegistryTypeMOAT
	newRegistry.SigningProfile = signing.Profile
}

// After:
if signing != nil && signing.Profile != nil {
	newRegistry.Type = config.RegistryTypeMOAT
	newRegistry.SigningProfile = signing.Profile
	newRegistry.ManifestURI = signing.ManifestURI // from allowlist; empty for flag-sourced profiles
}
```

For flag-sourced profiles (`Source == "flags"`), `ManifestURI` is left empty. The registry still functions as MOAT — the manifest URI is discovered from the manifest itself on first sync and stored at that point.

---

## Task 4: Auto-detect MOAT from registry.yaml at registry add time

**Files:**
- Modify: `cli/internal/registry/registry.go` (Manifest struct, ~line 231)
- Modify: `cli/cmd/syllago/registry_cmd.go` (post-clone block, after ~line 220)

**Depends on:** Task 3

### Success Criteria
- `grep -q 'ManifestURI.*yaml:"manifest_uri' cli/internal/registry/registry.go` → pass — field added with correct yaml tag
- `cd cli && go test ./internal/registry/ -run TestLoadManifest` → pass — existing manifest tests still pass
- `grep -q "MOAT compliance detected" cli/cmd/syllago/registry_cmd.go` → pass — user-facing message present in source
- `cd cli && go build ./cmd/syllago/` → pass — self-declaration block compiles (full behavioral test is TestRegistryAutoMOAT_RegistryYAML_SetsManifestURI written in Task 8)

---

### Step 1: Add ManifestURI to registry.Manifest struct

In `cli/internal/registry/registry.go` at the Manifest struct (~line 231):

```go
type Manifest struct {
	Name              string         `yaml:"name"`
	Description       string         `yaml:"description,omitempty"`
	Maintainers       []string       `yaml:"maintainers,omitempty"`
	Version           string         `yaml:"version,omitempty"`
	MinSyllagoVersion string         `yaml:"min_syllago_version,omitempty"`
	Items             []ManifestItem `yaml:"items,omitempty"`
	Visibility        string         `yaml:"visibility,omitempty"`
	ManifestURI       string         `yaml:"manifest_uri,omitempty"` // MOAT self-declaration; syllago extension, not spec-normative
}
```

### Step 2: Add MOAT self-declaration detection in registry add

The registry add control flow in `registry_cmd.go` is:
1. ~line 80: `signing, err := resolveSigningProfile(gitURL, rawSigningFlags)` — signing may be nil-profile (git mode) or have Profile (MOAT)
2. ~line 141: `registry.Clone(gitURL, name, refFlag)` — clone
3. ~line 146-192: content scan, visibility probe, manifest load
4. ~line 204-220: build `newRegistry`, set Type+SigningProfile+ManifestURI if signing.Profile != nil

Insert the self-declaration block AFTER line 220 (after `cfg.Registries = append(...)` would be too late; insert before it, after newRegistry is built):

```go
// MOAT self-declaration: if registry.yaml declares manifest_uri and the allowlist
// didn't already provide a signing profile, mark as MOAT. First sync will require
// --yes (TOFU — spec §Registry Trust: manually-added registry signing identity is
// accepted from the manifest on first fetch).
//
// signing.Profile == nil covers two cases: (a) no MOAT intent at all (git mode),
// (b) --moat was passed but no allowlist match and no --signing-identity (which
// would have hard-failed earlier in resolveSigningProfile). So reaching here with
// signing.Profile == nil means we're in pure git mode and should check self-declaration.
if signing == nil || signing.Profile == nil {
	if manifest, _ := registry.LoadManifestFromDir(dir); manifest != nil && manifest.ManifestURI != "" {
		newRegistry.Type = config.RegistryTypeMOAT
		newRegistry.ManifestURI = manifest.ManifestURI
		fmt.Fprintf(output.Writer, "MOAT compliance detected via registry.yaml. Run `syllago registry sync --yes %s` to verify and pin the signing identity.\n", name)
	}
}
```

Place this block immediately before `cfg.Registries = append(cfg.Registries, newRegistry)`.

---

## Task 5: Auto-upgrade existing plain registries on registry sync

**Files:**
- Modify: `cli/cmd/syllago/registry_cmd.go` (registry sync RunE, single-registry and all-registry loops)

**Depends on:** Task 4

### Success Criteria
- `cd cli && go build ./cmd/syllago/` → pass
- `grep -q "Auto-upgraded" cli/cmd/syllago/registry_cmd.go` → pass — upgrade message present
- `cd cli && go test ./cmd/syllago/ -run "TestRegistrySync_EmptyConfig|TestRegistrySync_NameNotFound|TestRegistrySync_NameNotCloned" -v` → pass — existing sync error-path tests unbroken (these tests are parallel-safe: `tryUpgradeToMOAT` calls `registry.CloneDir`/`registry.IsCloned`/`registry.LoadManifestFromDir` directly, not `cloneFn`, so introducing the `cloneFn` seam in Task 8 does not affect these tests)

---

### Step 1: Add tryUpgradeToMOAT helper function

Add a new unexported function in `registry_cmd.go`:

```go
// tryUpgradeToMOAT checks if a git-type registry should be upgraded to MOAT.
// Precedence: bundled allowlist (pre-trusted, no TOFU) > registry.yaml self-declaration (TOFU on first sync).
// Mutates r in place and saves cfg when an upgrade occurs. Returns true if upgraded.
//
// Precondition: r.IsGit() must be true before calling.
// CloneDir may return an error if the registry is configured but not cloned yet —
// in that case registry.yaml self-declaration is skipped (can't read from a non-existent clone).
func tryUpgradeToMOAT(r *config.Registry, cfg *config.Config, cfgRoot string, out io.Writer) (bool, error) {
	// 1. Allowlist check — pre-trusted, no TOFU on first sync.
	if entry, ok := moat.LookupSigningIdentity(r.URL); ok && entry.ManifestURI != "" {
		r.Type = config.RegistryTypeMOAT
		r.ManifestURI = entry.ManifestURI
		if r.SigningProfile == nil {
			r.SigningProfile = entry.Profile
		}
		fmt.Fprintf(out, "Auto-upgraded %s to MOAT (allowlist match).\n", r.Name)
		if err := config.Save(cfgRoot, cfg); err != nil {
			return false, fmt.Errorf("saving upgraded registry config: %w", err)
		}
		return true, nil
	}
	// 2. registry.yaml self-declaration — TOFU, requires --yes on first sync.
	cloneDir, err := registry.CloneDir(r.Name)
	if err != nil || !registry.IsCloned(r.Name) {
		return false, nil // not cloned yet, can't read registry.yaml
	}
	if manifest, _ := registry.LoadManifestFromDir(cloneDir); manifest != nil && manifest.ManifestURI != "" {
		r.Type = config.RegistryTypeMOAT
		r.ManifestURI = manifest.ManifestURI
		fmt.Fprintf(out, "Auto-upgraded %s to MOAT (registry.yaml manifest_uri). Run `syllago registry sync --yes %s` to pin the signing identity.\n", r.Name, r.Name)
		if err := config.Save(cfgRoot, cfg); err != nil {
			return false, fmt.Errorf("saving upgraded registry config: %w", err)
		}
		return true, nil
	}
	return false, nil
}
```

### Step 2: Call tryUpgradeToMOAT inside the sync loop before the git/MOAT branch

In the sync loop (~line 451), immediately before `if !r.IsMOAT() { /* git pull */ }`:

```go
if r.IsGit() {
	if _, err := tryUpgradeToMOAT(r, cfg, root, output.Writer); err != nil {
		return err
	}
}
// r may now be MOAT after upgrade; fall through to the IsMOAT() branch.
```

This runs for every git registry on every sync. The allowlist lookup is a local map read (O(1), no I/O). The registry.yaml read is a single file read only when the allowlist misses and the clone exists. The cost is negligible.

---

## Task 6: Add TRUST column to registry list output

**Files:**
- Modify: `cli/cmd/syllago/registry_cmd.go` (registryListCmd RunE, ~lines 332-373)

**Depends on:** Task 5

### Success Criteria
- `grep -q '"TRUST"' cli/cmd/syllago/registry_cmd.go` → pass — TRUST column header string present in source (binary rebuild happens in Task 9)
- `cd cli && go build ./cmd/syllago/` → pass
- **Verified in Task 8:** `cd cli && go test ./cmd/syllago/ -run TestRegistryList_TrustColumn -v` → pass — test asserts "TRUST" header and "moat" value appear for a MOAT registry with `LastFetchedAt` set

---

### Step 1: Add TrustTier to registryListItem

The `registryListItem` struct (find it near the `registrySyncCmd` definition or as a local in the RunE):

```go
type registryListItem struct {
	Name      string             `json:"name"`
	Status    string             `json:"status"`
	URL       string             `json:"url"`
	Ref       string             `json:"ref"`
	Manifest  *registry.Manifest `json:"manifest,omitempty"`
	IsMOAT    bool               `json:"is_moat"`
	TrustTier string             `json:"trust_tier,omitempty"` // "moat", "pending", or "" for git registries
}
```

The tier value is derived from the config record alone — no catalog enrichment needed. `moat` = type is MOAT and has been synced at least once (`LastFetchedAt != nil`). `pending` = type is MOAT but never synced. Full trust tier (SIGNED/DUAL-ATTESTED/UNSIGNED) requires catalog enrichment; that is surfaced in the TUI and is out of scope for the CLI list command.

### Step 2: Populate TrustTier in the item-building loop

Replace the existing `items = append(items, registryListItem{...})` block:

```go
tier := ""
if r.IsMOAT() {
	if r.LastFetchedAt != nil {
		tier = "moat"
	} else {
		tier = "pending"
	}
}
items = append(items, registryListItem{
	Name:      r.Name,
	Status:    status,
	URL:       r.URL,
	Ref:       ref,
	Manifest:  manifest,
	IsMOAT:    r.IsMOAT(),
	TrustTier: tier,
})
```

### Step 3: Add TRUST column to text output

Replace the header and row format strings:

```go
fmt.Fprintf(output.Writer, "%-20s  %-8s  %-8s  %-9s  %s\n", "NAME", "STATUS", "VERSION", "TRUST", "URL / DESCRIPTION")
fmt.Fprintf(output.Writer, "%-20s  %-8s  %-8s  %-9s  %s\n",
	strings.Repeat("─", 20), strings.Repeat("─", 8),
	strings.Repeat("─", 8), strings.Repeat("─", 9), strings.Repeat("─", 40))
for _, item := range items {
	version := "─"
	if item.Manifest != nil && item.Manifest.Version != "" {
		version = item.Manifest.Version
	}
	trust := "─"
	if item.TrustTier != "" {
		trust = item.TrustTier
	}
	fmt.Fprintf(output.Writer, "%-20s  %-8s  %-8s  %-9s  %s\n",
		truncateStr(item.Name, 20), item.Status, version, trust, item.URL)
	if item.Manifest != nil && item.Manifest.Description != "" {
		fmt.Fprintf(output.Writer, "  %s\n", item.Manifest.Description)
	}
}
```

---

## Task 7: Bootstrap syllago-meta-registry with MOAT workflows

**Files (all in `/home/hhewett/.local/src/syllago-meta-registry/`):**
- Create: `.github/workflows/moat.yml`
- Create: `.github/workflows/moat-registry.yml`
- Create: `registry.yml` (MOAT operator config — distinct from `registry.yaml` which is the content metadata)
- Modify: `registry.yaml`

**Depends on:** Task 1 (to know the manifest_uri value)

Note: this is the `syllago-meta-registry` repo, NOT the `syllago` repo. Commands run from `/home/hhewett/.local/src/syllago-meta-registry/`.

### Success Criteria
- `test -f /home/hhewett/.local/src/syllago-meta-registry/.github/workflows/moat.yml` → pass
- `test -f /home/hhewett/.local/src/syllago-meta-registry/.github/workflows/moat-registry.yml` → pass
- `test -f /home/hhewett/.local/src/syllago-meta-registry/registry.yml` → pass
- `grep -q "manifest_uri" /home/hhewett/.local/src/syllago-meta-registry/registry.yaml` → pass
- `python3 -c "import yaml; yaml.safe_load(open('/home/hhewett/.local/src/syllago-meta-registry/.github/workflows/moat.yml'))"` → exit 0 — moat.yml is valid YAML
- `python3 -c "import yaml; yaml.safe_load(open('/home/hhewett/.local/src/syllago-meta-registry/.github/workflows/moat-registry.yml'))"` → exit 0 — moat-registry.yml is valid YAML
- `cd /home/hhewett/.local/src/syllago-meta-registry && python3 -c "import yaml; d=yaml.safe_load(open('registry.yml')); assert d.get('schema_version')==1; assert 'registry' in d; assert d['registry'].get('manifest_uri')"` → pass — valid MOAT registry config

---

### Step 1: Create .github/workflows/moat.yml

Copy the reference publisher action verbatim:
```bash
mkdir -p /home/hhewett/.local/src/syllago-meta-registry/.github/workflows
cp /home/hhewett/.local/src/moat/reference/moat.yml \
   /home/hhewett/.local/src/syllago-meta-registry/.github/workflows/moat.yml
```

The reference action discovers content from canonical directories (`skills/`, `agents/`, `rules/`, `commands/`). The meta-registry also has `hooks/`, `mcp/`, and `loadouts/` directories — these are syllago-specific content types that fall outside the MOAT canonical type_map. The action will emit "Undiscovered content" warnings for them, which is acceptable: they are not attested by the publisher action, and their trust tier will be UNSIGNED. This is out of scope for this plan; a follow-up task can add them to `moat.yml` (the content discovery override, not the workflow) when syllago-specific MOAT content type support is added.

### Step 2: Create .github/workflows/moat-registry.yml

Copy the reference registry action verbatim:
```bash
cp /home/hhewett/.local/src/moat/reference/moat-registry.yml \
   /home/hhewett/.local/src/syllago-meta-registry/.github/workflows/moat-registry.yml
```

The registry action reads `registry.yml` from the repo root (Step 3 below). No customization needed.

### Step 3: Create registry.yml (MOAT operator config)

This file configures the MOAT registry action. It is distinct from `registry.yaml` (the syllago content registry metadata). The single source entry is the meta-registry itself — it is a self-publishing registry.

**Cross-reference:** The `manifest_uri` value here must exactly match the `manifest_uri` set in Task 1's `signing_identities.json` entry and in Task 7 Step 4's `registry.yaml`. All three must stay in sync — they all point to the same signed manifest file produced by `moat-registry.yml`.

```yaml
schema_version: 1

registry:
  name: syllago-meta-registry
  operator: OpenScribbler
  manifest_uri: https://raw.githubusercontent.com/OpenScribbler/syllago-meta-registry/moat-registry/registry.json

sources:
  - uri: https://github.com/OpenScribbler/syllago-meta-registry
```

The `moat-registry.yml` action will clone this URI, read `moat-attestation.json` from the `moat-attestation` branch (produced by `moat.yml`), determine trust tiers, sign the manifest, and push `registry.json` + `registry.json.sigstore` to the `moat-registry` branch. The `manifest_uri` above is the stable URL of that `registry.json`.

### Step 4: Update registry.yaml to add manifest_uri

```yaml
name: syllago-meta-registry
description: Official syllago meta-registry — skills, agents, loadouts, and content for syllago usage and management
version: 1.0.0
maintainers: [OpenScribbler]
visibility: public
manifest_uri: https://raw.githubusercontent.com/OpenScribbler/syllago-meta-registry/moat-registry/registry.json
```

This enables future users who add the meta-registry without the bundled allowlist (e.g., after a fresh allowlist that hasn't been refreshed) to auto-detect MOAT via the self-declaration path in Task 4.

---

## Task 8: Tests for auto-detection changes

**Files:**
- Modify: `cli/internal/moat/signing_identities_loader_test.go`
- Modify: `cli/cmd/syllago/registry_signing_test.go`
- Create: `cli/cmd/syllago/registry_cmd_moat_autodetect_test.go`

**Depends on:** Tasks 1–6

### Success Criteria
- `cd cli && go test ./internal/moat/ -run TestLookupSigningIdentity -v` → pass — AllowlistEntry.ManifestURI non-empty for meta-registry
- `cd cli && go test ./cmd/syllago/ -run TestResolveSigningProfile_AllowlistIncludesManifestURI -v` → pass
- `cd cli && go test ./cmd/syllago/ -run TestRegistryAutoMOAT -v` → pass — all subtests green
- `cd cli && go test ./internal/moat/ -coverprofile=moat.out && go tool cover -func=moat.out | tail -1` → total ≥ 80%
- `cd cli && go test ./cmd/syllago/ -coverprofile=cmd.out && go tool cover -func=cmd.out | tail -1` → total ≥ 80%

---

### Step 1: Update signing_identities_loader_test.go

Add to the existing test file:

```go
func TestLookupSigningIdentity_MetaRegistryHasManifestURI(t *testing.T) {
	entry, ok := LookupSigningIdentity("https://github.com/OpenScribbler/syllago-meta-registry")
	if !ok {
		t.Fatal("expected allowlist match for meta-registry URL")
	}
	if entry.ManifestURI == "" {
		t.Error("expected non-empty ManifestURI for meta-registry allowlist entry")
	}
	wantPrefix := "https://raw.githubusercontent.com/OpenScribbler/syllago-meta-registry/"
	if !strings.HasPrefix(entry.ManifestURI, wantPrefix) {
		t.Errorf("ManifestURI %q does not start with expected prefix %q", entry.ManifestURI, wantPrefix)
	}
	if entry.Profile == nil {
		t.Error("expected non-nil Profile")
	}
}

func TestParseSigningIdentities_ManifestURIOptional(t *testing.T) {
	// Entries without manifest_uri must still parse successfully (back-compat).
	data := []byte(`{
		"_version": 1,
		"identities": [{
			"registry_url": "https://github.com/example/repo",
			"profile": {
				"issuer": "https://token.actions.githubusercontent.com",
				"subject": "https://github.com/example/repo/.github/workflows/moat.yml@refs/heads/main",
				"profile_version": 1,
				"repository_id": "12345",
				"repository_owner_id": "67890"
			}
		}]
	}`)
	idx, err := parseSigningIdentities(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entry := idx[normalizeRegistryURL("https://github.com/example/repo")]
	if entry == nil {
		t.Fatal("expected entry in index")
	}
	if entry.ManifestURI != "" {
		t.Errorf("expected empty ManifestURI for entry without manifest_uri, got %q", entry.ManifestURI)
	}
}
```

### Step 2: Update registry_signing_test.go

Add to the existing test file (the seam pattern this project uses is function-level overrides for I/O and package-level globals for provider/config — the test for resolveSigningProfile calls the function directly since the allowlist is the real bundled one):

```go
func TestResolveSigningProfile_AllowlistIncludesManifestURI(t *testing.T) {
	res, err := resolveSigningProfile(
		"https://github.com/OpenScribbler/syllago-meta-registry",
		signingFlagSet{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil resolution")
	}
	if res.Source != "allowlist" {
		t.Errorf("expected Source=allowlist, got %q", res.Source)
	}
	if res.ManifestURI == "" {
		t.Error("expected non-empty ManifestURI from allowlist resolution")
	}
	if res.Profile == nil {
		t.Error("expected non-nil Profile")
	}
}

func TestResolveSigningProfile_FlagsLeaveManifestURIEmpty(t *testing.T) {
	res, err := resolveSigningProfile(
		"https://github.com/example/unknown-registry",
		signingFlagSet{
			UserRequestedMOAT: true,
			Identity:          "https://github.com/example/unknown/.github/workflows/moat.yml@refs/heads/main",
			Issuer:            moat.GitHubActionsIssuer,
			RepositoryID:      "111",
			RepositoryOwnerID: "222",
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Source != "flags" {
		t.Errorf("expected Source=flags, got %q", res.Source)
	}
	if res.ManifestURI != "" {
		t.Errorf("expected empty ManifestURI for flag-sourced profile, got %q", res.ManifestURI)
	}
}
```

### Step 3: Add cloneFn seam to registry_cmd.go, then create registry_cmd_moat_autodetect_test.go

**Why a new seam?** `registry.Clone` runs `git clone` and has no existing override hook. The pattern this codebase uses (e.g., `moatSyncFn = moat.Sync` in `registry_sync_moat.go`) is a package-level function var in the `cmd/syllago` package — NOT in the internal package. This keeps the internal package clean and the seam local to the command layer where the expensive operation is called.

**Add to `registry_cmd.go`** (near the `moatSyncFn` declaration at the top of the file):

```go
// cloneFn is a package-level seam so tests can stub git clone without network access.
// Overriding this in tests must mirror what registry.Clone does: create the clone dir
// with a valid registry layout at registry.CloneDir(name).
var cloneFn = func(url, name, ref string) error {
    return registry.Clone(url, name, ref)
}
```

**Replace the `registry.Clone(...)` call in the add command** (line ~141):

```go
// Before:
if err := registry.Clone(gitURL, name, refFlag); err != nil {

// After:
if err := cloneFn(gitURL, name, refFlag); err != nil {
```

**Create `cli/cmd/syllago/registry_cmd_moat_autodetect_test.go`:**

```go
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

// stubClone overrides cloneFn to create a fake clone dir at registry.CloneDir(name)
// containing a registry.yaml with the given content. Restores on t.Cleanup.
func stubClone(t *testing.T, yamlContent string) {
	t.Helper()
	orig := cloneFn
	cloneFn = func(url, name, ref string) error {
		cloneDir, err := registry.CloneDir(name)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(cloneDir, 0755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(cloneDir, "registry.yaml"), []byte(yamlContent), 0644)
	}
	t.Cleanup(func() { cloneFn = orig })
}

func TestRegistryAutoMOAT_AllowlistURL_SetsManifestURI(t *testing.T) {
	// No t.Parallel — swaps package-level cloneFn and registry.OverrideProbeForTest.
	// registry add for the meta-registry URL must auto-set type=moat + ManifestURI
	// from the bundled allowlist — no --moat flag required.
	root := withRegistryProjectAndCache(t, nil, &config.Config{})
	output.SetForTest(t)
	overrideProbe(t, func(url string) (string, error) { return "public", nil })

	// Fake clone: minimal registry.yaml with no manifest_uri — allowlist provides it.
	stubClone(t, "name: syllago-meta-registry\nversion: \"1.0\"\n")

	err := registryAddCmd.RunE(registryAddCmd, []string{"https://github.com/OpenScribbler/syllago-meta-registry"})
	if err != nil {
		t.Fatalf("registry add failed: %v", err)
	}

	got, err := config.Load(root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(got.Registries) != 1 {
		t.Fatalf("expected 1 registry, got %d", len(got.Registries))
	}
	r := got.Registries[0]
	if r.Type != config.RegistryTypeMOAT {
		t.Errorf("expected type=moat, got %q", r.Type)
	}
	if r.ManifestURI == "" {
		t.Error("expected non-empty ManifestURI from allowlist auto-detection")
	}
	wantPrefix := "https://raw.githubusercontent.com/OpenScribbler/syllago-meta-registry/"
	if !strings.HasPrefix(r.ManifestURI, wantPrefix) {
		t.Errorf("ManifestURI %q does not start with %q", r.ManifestURI, wantPrefix)
	}
}

func TestRegistryAutoMOAT_RegistryYAML_SetsManifestURI(t *testing.T) {
	// No t.Parallel — swaps package-level cloneFn and registry.OverrideProbeForTest.
	// registry add for a non-allowlisted URL that declares manifest_uri in registry.yaml
	// must auto-set type=moat + ManifestURI from that self-declaration.
	root := withRegistryProjectAndCache(t, nil, &config.Config{})
	output.SetForTest(t)
	overrideProbe(t, func(url string) (string, error) { return "public", nil })

	const testURL = "https://github.com/example/non-allowlisted-registry"
	const wantManifestURI = "https://raw.githubusercontent.com/example/non-allowlisted-registry/moat-registry/registry.json"

	stubClone(t, "name: non-allowlisted-registry\nversion: \"1.0\"\nmanifest_uri: "+wantManifestURI+"\n")

	err := registryAddCmd.RunE(registryAddCmd, []string{testURL})
	if err != nil {
		t.Fatalf("registry add failed: %v", err)
	}

	got, err := config.Load(root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(got.Registries) != 1 {
		t.Fatalf("expected 1 registry, got %d", len(got.Registries))
	}
	r := got.Registries[0]
	if r.Type != config.RegistryTypeMOAT {
		t.Errorf("expected type=moat, got %q", r.Type)
	}
	if r.ManifestURI != wantManifestURI {
		t.Errorf("expected ManifestURI %q, got %q", wantManifestURI, r.ManifestURI)
	}
}

func TestRegistryList_TrustColumn(t *testing.T) {
	// registry list must show the TRUST column header and "moat" for a synced MOAT registry.
	now := time.Now()
	cfg := &config.Config{
		Registries: []config.Registry{
			{
				Name:         "example/moat-reg",
				URL:          "https://github.com/example/moat-reg",
				Type:         config.RegistryTypeMOAT,
				ManifestURI:  "https://raw.githubusercontent.com/example/moat-reg/moat-registry/registry.json",
				LastFetchedAt: &now,
			},
			{
				Name: "example/plain-reg",
				URL:  "https://github.com/example/plain-reg",
			},
		},
	}
	withRegistryProjectAndCache(t, nil, cfg)
	stdout, _ := output.SetForTest(t)

	if err := registryListCmd.RunE(registryListCmd, nil); err != nil {
		t.Fatalf("registryListCmd.RunE: %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, "TRUST") {
		t.Errorf("expected TRUST column header in output, got:\n%s", got)
	}
	if !strings.Contains(got, "moat") {
		t.Errorf("expected 'moat' in TRUST column for MOAT registry, got:\n%s", got)
	}
}
```

---

## Task 9: Build, format, and commit

**Files:** All modified Go files in `cli/` (syllago repo) and bootstrap files in `syllago-meta-registry` repo.

**Depends on:** Tasks 1–8

### Success Criteria
- `cd /home/hhewett/.local/src/syllago/cli && make fmt` → pass — no unformatted files
- `cd /home/hhewett/.local/src/syllago/cli && make build` → pass — binary compiles
- `cd /home/hhewett/.local/src/syllago/cli && make test` → pass — all tests green
- `cd /home/hhewett/.local/src/syllago/cli && go test ./internal/moat/ -coverprofile=moat.out && go tool cover -func=moat.out | tail -1` → line shows ≥ 80%
- `cd /home/hhewett/.local/src/syllago/cli && go test ./cmd/syllago/ -coverprofile=cmd.out && go tool cover -func=cmd.out | tail -1` → line shows ≥ 80%
- `cd /home/hhewett/.local/src/syllago-meta-registry && git push` → pass — bootstrap files pushed to GitHub
- After GitHub Actions complete: `syllago registry sync OpenScribbler/syllago-meta-registry --yes` (from a fresh clone) → exits 0 and prints "Synced: ... (tofu-accepted, ...)"

---

### Step 1: Format, build, and test the syllago repo

```bash
cd /home/hhewett/.local/src/syllago/cli
make fmt
make build
make test
```

If any golden files changed due to registry list column width change: regenerate them.
```bash
cd /home/hhewett/.local/src/syllago/cli && go test ./internal/tui/ -update-golden
git diff cli/internal/tui/testdata/  # review each change
```

### Step 2: Commit syllago Go changes

```bash
cd /home/hhewett/.local/src/syllago
git add cli/internal/moat/signing_identities_loader.go \
        cli/internal/moat/signing_identities.json \
        cli/cmd/syllago/registry_signing.go \
        cli/cmd/syllago/registry_cmd.go \
        cli/internal/registry/registry.go \
        cli/internal/moat/signing_identities_loader_test.go \
        cli/cmd/syllago/registry_signing_test.go \
        cli/cmd/syllago/registry_cmd_moat_autodetect_test.go
git commit -m "feat(moat): auto-detect MOAT compliance from allowlist and registry.yaml"
git push
```

### Step 3: Commit syllago-meta-registry bootstrap files

This is the second repo — separate git history.

```bash
cd /home/hhewett/.local/src/syllago-meta-registry
git add .github/workflows/moat.yml \
        .github/workflows/moat-registry.yml \
        registry.yml \
        registry.yaml
git commit -m "feat(moat): add MOAT publisher + registry actions and operator config"
git push
```

### Step 4: Verify GitHub Actions complete

After push, two workflows run on the `moat.yml` trigger:
1. `moat.yml` — discovers content, signs items, pushes `moat-attestation.json` to `moat-attestation` branch. Must complete before step 2.
2. `moat-registry.yml` — runs on schedule (daily) or manual trigger. Reads `registry.yml`, clones the source, reads `moat-attestation.json`, signs the manifest, pushes `registry.json` + `registry.json.sigstore` to `moat-registry` branch.

Manual trigger for `moat-registry.yml` immediately after push (don't wait for the daily cron). Use `gh run list` to capture the run ID before watching, so you watch the specific triggered run and not an unrelated concurrent run:
```bash
gh workflow run moat-registry.yml --repo OpenScribbler/syllago-meta-registry
gh run list --repo OpenScribbler/syllago-meta-registry --workflow moat-registry.yml --limit 1 --json databaseId --jq '.[0].databaseId' | xargs gh run watch --repo OpenScribbler/syllago-meta-registry
```

**Success criteria:**
- `gh workflow run moat-registry.yml --repo OpenScribbler/syllago-meta-registry` → exit 0 (workflow dispatch accepted)
- Watch the specific triggered run (not the most recent run, which may be a different workflow): `gh run list --repo OpenScribbler/syllago-meta-registry --workflow moat-registry.yml --limit 1 --json databaseId --jq '.[0].databaseId' | xargs gh run watch --repo OpenScribbler/syllago-meta-registry` → exits 0 when the run completes successfully

### Step 5: Smoke test the full auto-detection flow

After both Actions complete:
```bash
# Upgrade the existing plain registry in config:
syllago registry sync OpenScribbler/syllago-meta-registry --yes
# Expected output: "Auto-upgraded OpenScribbler/syllago-meta-registry to MOAT (allowlist match)."
#                  "Synced: OpenScribbler/syllago-meta-registry (tofu-accepted, ...)"

syllago registry list
# Expected: OpenScribbler/syllago-meta-registry row shows "moat" in TRUST column
```

**Success criteria:**
- `syllago registry sync OpenScribbler/syllago-meta-registry --yes 2>&1 | grep -q "Auto-upgraded"` → pass — upgrade message printed
- `syllago registry list 2>&1 | grep -q "moat"` → pass — trust tier displayed in TRUST column
