// Package doctor runs syllago health checks used by both the CLI and TUI.
package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// CheckStatus is the outcome of a single health check.
type CheckStatus string

const (
	CheckOK   CheckStatus = "ok"
	CheckWarn CheckStatus = "warn"
	CheckErr  CheckStatus = "error"
)

// CheckResult is one doctor check's output.
type CheckResult struct {
	Name    string      `json:"name"`
	Status  CheckStatus `json:"status"`
	Message string      `json:"message"`
	Details []string    `json:"details,omitempty"`
}

// Result is the full output of a Run call.
type Result struct {
	Checks  []CheckResult `json:"checks"`
	Summary string        `json:"summary"`
}

// Run executes all doctor checks and returns the aggregate result.
// Pass an empty projectRoot to skip project-specific checks (symlinks,
// integrity, orphans).
func Run(projectRoot string) Result {
	var checks []CheckResult
	checks = append(checks, CheckLibrary())
	checks = append(checks, CheckConfigWith(projectRoot))
	checks = append(checks, CheckProviders())
	if projectRoot != "" {
		checks = append(checks, CheckSymlinks(projectRoot))
		checks = append(checks, CheckContentDrift(projectRoot))
		checks = append(checks, CheckOrphans(projectRoot))
	}
	checks = append(checks, CheckRegistriesWith(projectRoot))
	checks = append(checks, CheckNamingQuality(projectRoot))

	warns, errs := 0, 0
	for _, c := range checks {
		switch c.Status {
		case CheckWarn:
			warns++
		case CheckErr:
			errs++
		}
	}

	summary := "All checks passed"
	if errs > 0 {
		summary = fmt.Sprintf("%d error(s), %d warning(s)", errs, warns)
	} else if warns > 0 {
		summary = fmt.Sprintf("%d warning(s)", warns)
	}

	return Result{Checks: checks, Summary: summary}
}

// CheckLibrary verifies the syllago content library directory exists.
func CheckLibrary() CheckResult {
	dir := catalog.GlobalContentDir()
	if dir == "" {
		return CheckResult{Name: "library", Status: CheckErr, Message: "Library: cannot determine home directory"}
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return CheckResult{
			Name:    "library",
			Status:  CheckErr,
			Message: fmt.Sprintf("Library: %s not found", dir),
			Details: []string{"Run 'syllago init' to create your library"},
		}
	}

	count := 0
	for _, ct := range catalog.AllContentTypes() {
		typeDir := filepath.Join(dir, string(ct))
		entries, err := os.ReadDir(typeDir)
		if err == nil {
			count += len(entries)
		}
	}
	return CheckResult{Name: "library", Status: CheckOK, Message: fmt.Sprintf("Library: %s (%d items)", dir, count)}
}

// CheckConfigWith verifies the global and project syllago configs are loadable.
func CheckConfigWith(projectRoot string) CheckResult {
	globalCfg, gErr := config.LoadGlobal()
	projectCfg, pErr := config.Load(projectRoot)

	if gErr != nil && pErr != nil {
		return CheckResult{Name: "config", Status: CheckErr, Message: "Config: failed to load", Details: []string{gErr.Error()}}
	}

	var parts []string
	if globalCfg != nil {
		parts = append(parts, "global")
	}
	if projectCfg != nil && projectRoot != "" {
		parts = append(parts, "project")
	}
	if len(parts) == 0 {
		return CheckResult{
			Name:    "config",
			Status:  CheckWarn,
			Message: "Config: no config files found",
			Details: []string{"Run 'syllago init' or create ~/.syllago/config.json"},
		}
	}

	return CheckResult{Name: "config", Status: CheckOK, Message: fmt.Sprintf("Config: %s loaded", JoinWords(parts))}
}

// CheckProviders verifies at least one AI provider is detected.
func CheckProviders() CheckResult {
	detected := provider.DetectProviders()
	found, notFound := 0, 0
	var missing []string
	for _, p := range detected {
		if p.Detected {
			found++
		} else {
			notFound++
			missing = append(missing, p.Slug)
		}
	}

	if found == 0 {
		return CheckResult{Name: "providers", Status: CheckWarn, Message: "Providers: none detected", Details: missing}
	}
	msg := fmt.Sprintf("Providers: %d detected", found)
	if notFound > 0 {
		msg += fmt.Sprintf(", %d not found", notFound)
	}
	return CheckResult{Name: "providers", Status: CheckOK, Message: msg}
}

// CheckSymlinks verifies installed symlinks are not broken.
func CheckSymlinks(projectRoot string) CheckResult {
	inst, err := installer.LoadInstalled(projectRoot)
	if err != nil {
		return CheckResult{Name: "symlinks", Status: CheckWarn, Message: "Symlinks: could not load installed.json"}
	}

	if len(inst.Symlinks) == 0 {
		return CheckResult{Name: "symlinks", Status: CheckOK, Message: "Symlinks: none installed"}
	}

	broken := 0
	var details []string
	for _, s := range inst.Symlinks {
		if _, err := os.Stat(s.Target); err != nil {
			broken++
			details = append(details, fmt.Sprintf("broken: %s -> %s", s.Path, s.Target))
		}
	}

	if broken > 0 {
		return CheckResult{
			Name:    "symlinks",
			Status:  CheckWarn,
			Message: fmt.Sprintf("Symlinks: %d of %d broken", broken, len(inst.Symlinks)),
			Details: details,
		}
	}
	return CheckResult{Name: "symlinks", Status: CheckOK, Message: fmt.Sprintf("Symlinks: %d valid", len(inst.Symlinks))}
}

// CheckContentDrift verifies installed content has not drifted from the library.
func CheckContentDrift(projectRoot string) CheckResult {
	drifted, err := installer.VerifyIntegrity(projectRoot)
	if err != nil {
		return CheckResult{Name: "integrity", Status: CheckWarn, Message: "Integrity: could not verify", Details: []string{err.Error()}}
	}

	if len(drifted) == 0 {
		return CheckResult{Name: "integrity", Status: CheckOK, Message: "Integrity: no content drift detected"}
	}

	var details []string
	for _, d := range drifted {
		details = append(details, fmt.Sprintf("%s: %s (%s)", d.Type, d.Name, d.Status))
	}
	return CheckResult{
		Name:    "integrity",
		Status:  CheckWarn,
		Message: fmt.Sprintf("Integrity: %d item(s) modified since install", len(drifted)),
		Details: details,
	}
}

// CheckOrphans verifies all tracked installs still have matching content on disk.
func CheckOrphans(projectRoot string) CheckResult {
	inst, err := installer.LoadInstalled(projectRoot)
	if err != nil {
		return CheckResult{Name: "orphans", Status: CheckWarn, Message: "Orphans: could not load installed.json"}
	}

	libDir := catalog.GlobalContentDir()
	if libDir == "" {
		return CheckResult{Name: "orphans", Status: CheckOK, Message: "Orphans: skipped (no library)"}
	}

	orphaned := 0
	var details []string
	for _, s := range inst.Symlinks {
		if _, err := os.Stat(s.Target); err != nil {
			orphaned++
			details = append(details, filepath.Base(s.Path))
		}
	}

	detected := provider.DetectProviders()
	var mergeOrphans []installer.OrphanEntry
	var mergeErr error
	if len(inst.Hooks) > 0 || len(inst.MCP) > 0 {
		mergeOrphans, mergeErr = installer.CheckOrphanedMerges(projectRoot, detected)
	}
	if mergeErr == nil {
		for _, o := range mergeOrphans {
			orphaned++
			if o.Type == "hook" {
				details = append(details, fmt.Sprintf("%s: untracked hook in hooks.%s (provider: %s)", o.Type, o.Key, o.Provider))
			} else {
				details = append(details, fmt.Sprintf("%s: untracked server %q (provider: %s)", o.Type, o.Key, o.Provider))
			}
		}
	}

	if orphaned > 0 {
		return CheckResult{
			Name:    "orphans",
			Status:  CheckWarn,
			Message: fmt.Sprintf("Orphans: %d installed item(s) missing from disk or untracked", orphaned),
			Details: details,
		}
	}
	return CheckResult{Name: "orphans", Status: CheckOK, Message: "Orphans: all installed content accounted for"}
}

// CheckRegistriesWith verifies all configured registries are reachable.
func CheckRegistriesWith(projectRoot string) CheckResult {
	globalCfg, _ := config.LoadGlobal()
	projectCfg, _ := config.Load(projectRoot)
	merged := config.Merge(globalCfg, projectCfg)

	if len(merged.Registries) == 0 {
		return CheckResult{Name: "registries", Status: CheckOK, Message: "Registries: none configured"}
	}

	pub, priv, unknown := 0, 0, 0
	for _, r := range merged.Registries {
		switch r.Visibility {
		case "public":
			pub++
		case "private":
			priv++
		default:
			unknown++
		}
	}

	var parts []string
	if pub > 0 {
		parts = append(parts, fmt.Sprintf("%d public", pub))
	}
	if priv > 0 {
		parts = append(parts, fmt.Sprintf("%d private", priv))
	}
	if unknown > 0 {
		parts = append(parts, fmt.Sprintf("%d unknown", unknown))
	}

	return CheckResult{
		Name:    "registries",
		Status:  CheckOK,
		Message: fmt.Sprintf("Registries: %d configured (%s)", len(merged.Registries), JoinWords(parts)),
	}
}

// CheckNamingQuality verifies hooks and MCP items have human-readable display names.
func CheckNamingQuality(projectRoot string) CheckResult {
	scan, err := moat.LoadAndScan(projectRoot, projectRoot, time.Now())
	if err != nil {
		return CheckResult{Name: "naming", Status: CheckWarn, Message: "Naming: could not scan content", Details: []string{err.Error()}}
	}
	cat := scan.Catalog

	var unnamed int
	var details []string
	for _, item := range cat.Items {
		if item.Type != catalog.Hooks && item.Type != catalog.MCP {
			continue
		}
		if item.DisplayName == "" || item.DisplayName == item.Name {
			unnamed++
			details = append(details, fmt.Sprintf("%s %s: no display name", item.Type, item.Name))
		}
	}

	if unnamed > 0 {
		return CheckResult{
			Name:    "naming",
			Status:  CheckWarn,
			Message: fmt.Sprintf("Naming: %d hooks/MCP items have no display name", unnamed),
			Details: details,
		}
	}
	return CheckResult{Name: "naming", Status: CheckOK, Message: "Naming: all hooks/MCP items have display names"}
}

// JoinWords joins a list of words with ", " separators.
func JoinWords(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	default:
		result := parts[0]
		for i := 1; i < len(parts); i++ {
			result += ", " + parts[i]
		}
		return result
	}
}
