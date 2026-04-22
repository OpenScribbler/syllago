package moat

// Load-and-scan orchestration (ADR 0007 Phase 2c, bead syllago-nmjrm).
//
// Every live-catalog consumer (TUI rescan, `syllago list`, `syllago
// inspect`, `syllago doctor`) needs the same preamble: merge configs
// from the three sources, enumerate cloned registries, load the MOAT
// lockfile, and run ScanAndEnrich with a current timestamp. LoadAndScan
// collapses that preamble into one call so each caller can treat
// "catalog with trust state" as a single operation rather than
// reconstructing the pipeline locally.
//
// Import direction note: this file is the first in the moat package to
// depend on internal/registry. That direction is valid — registry has
// never imported moat and nothing here forces the reverse — but if a
// future change pulls moat types into registry (e.g. a verify-during-
// clone flow), LoadAndScan should move up a layer rather than letting
// the cycle form.

import (
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

// ScanResult bundles the outputs of LoadAndScan. Callers that only need
// the display catalog (CLI commands) read Catalog and drop the rest;
// callers that dispatch installs (TUI) read every field so the gate
// sees the same snapshot the user just saw.
type ScanResult struct {
	Catalog         *catalog.Catalog
	Lockfile        *Lockfile
	GateInputs      *GateInputs
	RegistrySources []catalog.RegistrySource
	Config          *config.Config
}

// LoadAndScan performs the full config-load + registry-enumerate +
// lockfile-load + scan + enrich pipeline and returns a ScanResult.
//
// Error policy mirrors ScanAndEnrich: only a fundamental catalog-scan
// failure returns a non-nil error. Per-registry enrichment problems
// (missing cache, unparseable manifest) attach to ScanResult.Catalog
// via cat.Warnings so the caller can surface them without aborting.
//
// MUST be called off the event loop in a tea.Cmd when invoked from the
// TUI — the embedded I/O (file reads, sigstore verification on first
// call per process) would violate .claude/rules/tui-elm.md rule #2.
func LoadAndScan(root, projectRoot string, now time.Time) (*ScanResult, error) {
	globalCfg, _ := config.LoadGlobal()
	projectCfg, _ := config.Load(projectRoot)
	contentCfg, _ := config.Load(root)
	merged := config.Merge(globalCfg, config.Merge(contentCfg, projectCfg))

	var regSources []catalog.RegistrySource
	for _, r := range merged.Registries {
		if registry.IsCloned(r.Name) {
			dir, _ := registry.CloneDir(r.Name)
			regSources = append(regSources, catalog.RegistrySource{Name: r.Name, Path: dir})
		}
	}

	cacheDir, _ := config.GlobalDirPath()
	lf, _ := LoadLockfile(LockfilePath(projectRoot))

	cat, err := ScanAndEnrich(merged, root, projectRoot, regSources, lf, cacheDir, now)
	if err != nil {
		return nil, err
	}

	return &ScanResult{
		Catalog:         cat,
		Lockfile:        lf,
		GateInputs:      BuildGateInputs(merged, cacheDir),
		RegistrySources: regSources,
		Config:          merged,
	}, nil
}
