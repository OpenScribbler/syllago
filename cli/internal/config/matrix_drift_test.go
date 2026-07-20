package config

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/acif"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

var knownMismatches = map[string]struct{}{
	"amp/rules":        {}, // tracked in syllago-t63g5
	"codex/rules":      {}, // tracked in syllago-t63g5
	"gemini-cli/rules": {}, // tracked in syllago-t63g5
	"opencode/rules":   {}, // tracked in syllago-t63g5
	"cline/rules":      {}, // tracked in syllago-t63g5
	"roo-code/rules":   {}, // tracked in syllago-t63g5
	// pi/hooks: the matrix row records pi's native contract (one
	// <content-name>.ts extension per hook); ADR-0020 Phase 1b routes pi hook
	// installs through the adapter into a single syllago-owned
	// syllago-hooks.ts, so the provider table returns the merge sentinel.
	// Expressing a tool-owned consolidated file needs an ACIF layout-enum
	// addition (Class C) — tracked in syllago-t63g5.
	"pi/hooks": {}, // tracked in syllago-t63g5
}

func TestInstallMatrixMatchesLegacyProviderTables(t *testing.T) {
	unsetACIFInstallEntryPointsEnv(t)

	if len(knownMismatches) != 7 {
		t.Fatalf("knownMismatches must contain exactly seven tracked entries, got %d", len(knownMismatches))
	}

	contentTypes := []catalog.ContentType{
		catalog.Skills,
		catalog.Agents,
		catalog.MCP,
		catalog.Rules,
		catalog.Hooks,
		catalog.Commands,
	}

	const home = "/home/u"
	var gaps []string
	var mismatches []string
	reproducedKnown := make(map[string]bool, len(knownMismatches))
	for _, prov := range provider.AllProviders {
		for _, ct := range contentTypes {
			if prov.SupportsType == nil || !prov.SupportsType(ct) {
				continue
			}
			legacy := prov.InstallDir(home, ct)
			matrix, ok, err := rawMatrixInstallDir(prov.Slug, ct, home)
			if err != nil {
				t.Fatalf("matrix rows for %s/%s: %v", prov.Slug, ct, err)
			}
			if !ok {
				gaps = append(gaps, fmt.Sprintf("  %s × %s", prov.Slug, ct))
				continue
			}
			if legacy != matrix {
				key := fmt.Sprintf("%s/%s", prov.Slug, ct)
				if _, ok := knownMismatches[key]; ok {
					reproducedKnown[key] = true
					continue
				}
				mismatches = append(mismatches, fmt.Sprintf(
					"%s %s: legacy InstallDir=%q, matrixInstallDir=%q; reconcile via an ACIF row-data amendment or a provider-table fix — never a local divergence",
					prov.Slug, ct, legacy, matrix,
				))
			}
		}
	}

	sort.Strings(gaps)
	if len(gaps) > 0 {
		t.Logf("ACIF install-entry-point GAPs:\n%s", strings.Join(gaps, "\n"))
	}

	sort.Strings(mismatches)
	for _, mismatch := range mismatches {
		t.Error(mismatch)
	}

	var resolved []string
	for key := range knownMismatches {
		if !reproducedKnown[key] {
			resolved = append(resolved, key)
		}
	}
	sort.Strings(resolved)
	for _, key := range resolved {
		t.Errorf("%s resolved — remove from knownMismatches", key)
	}
}

func rawMatrixInstallDir(providerSlug string, ct catalog.ContentType, homeDir string) (string, bool, error) {
	contentType := acifContentType(ct)
	if contentType == "" {
		return "", false, nil
	}
	rows, err := acif.InstallEntryRows(providerSlug, contentType)
	if err != nil {
		return "", false, err
	}
	for _, row := range rows {
		if row.Status != "current" || row.Scope != "user" {
			continue
		}
		if row.Layout == "merged_into_shared_file" {
			return provider.JSONMergeSentinel, true, nil
		}
		dir, ok := matrixTemplateDir(row.PathTemplate, homeDir)
		if !ok {
			return "", true, nil
		}
		return dir, true, nil
	}
	return "", false, nil
}
