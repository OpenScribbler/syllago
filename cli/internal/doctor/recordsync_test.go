package doctor

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

func TestPlanRecordBackfill(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	libraryRoot := filepath.Join(tmp, "content")
	skillPath := filepath.Join(libraryRoot, "skills", "dir-skill")
	agentPath := filepath.Join(libraryRoot, "agents", "helper")
	registrySkillPath := filepath.Join(libraryRoot, "skills", "registry-skill")
	nestedSkillPath := filepath.Join(libraryRoot, "skills", "nested-skill")

	skill := catalog.ContentItem{Name: "dir-skill", Type: catalog.Skills, Path: skillPath}
	agent := catalog.ContentItem{Name: "helper", Type: catalog.Agents, Path: agentPath}
	registrySkill := catalog.ContentItem{
		Name: "registry-skill",
		Type: catalog.Skills,
		Path: registrySkillPath,
		Meta: &metadata.Meta{SourceType: "registry", SourceRegistry: "acme/tools"},
	}
	nestedSkill := catalog.ContentItem{Name: "nested-skill", Type: catalog.Skills, Path: nestedSkillPath}
	items := []catalog.ContentItem{skill, agent, registrySkill, nestedSkill}

	skillLink := doctorBackfillLink("claude-code", catalog.Skills, filepath.Join(tmp, "provider", "skills", "dir-skill"), skillPath, installer.LinkHealthy)
	agentLink := doctorBackfillLink("claude-code", catalog.Agents, filepath.Join(tmp, "provider", "agents", "helper.md"), filepath.Join(agentPath, "AGENT.md"), installer.LinkHealthy)
	registryLink := doctorBackfillLink("codex", catalog.Skills, filepath.Join(tmp, "codex", "skills", "registry-skill"), registrySkillPath, installer.LinkHealthy)
	nestedLink := doctorBackfillLink("zed", catalog.Skills, filepath.Join(tmp, "zed", "skills", "nested-skill"), filepath.Join(nestedSkillPath, "docs", "README.md"), installer.LinkHealthy)
	legacyLink := doctorBackfillLink("claude-code", catalog.Skills, filepath.Join(tmp, "provider", "skills", "legacy"), filepath.Join(tmp, "registries", "core", "skills", "legacy"), installer.LinkHealthy)
	unmatchedLink := doctorBackfillLink("claude-code", catalog.Skills, filepath.Join(tmp, "provider", "skills", "unknown"), filepath.Join(libraryRoot, "skills", "unknown"), installer.LinkHealthy)
	brokenLink := doctorBackfillLink("claude-code", catalog.Skills, filepath.Join(tmp, "provider", "skills", "broken"), filepath.Join(libraryRoot, "skills", "broken"), installer.LinkBroken)

	skillCoord := installstore.Coord{Type: string(catalog.Skills), Name: "dir-skill"}
	agentCoord := installstore.Coord{Type: string(catalog.Agents), Name: "helper"}
	registryCoord := installstore.Coord{Registry: "acme/tools", Type: string(catalog.Skills), Name: "registry-skill"}
	nestedCoord := installstore.Coord{Type: string(catalog.Skills), Name: "nested-skill"}

	tests := []struct {
		name          string
		links         []installer.ScannedLink
		store         *installstore.Store
		wantEntries   []BackfillEntry
		wantLegacy    []installer.ScannedLink
		wantUnmatched []installer.ScannedLink
		wantTracked   int
	}{
		{
			name:  "fresh store records every healthy library link",
			links: []installer.ScannedLink{skillLink, agentLink, registryLink},
			store: &installstore.Store{Version: installstore.CurrentVersion},
			wantEntries: []BackfillEntry{
				doctorBackfillEntry(skillCoord, skillPath, skillLink),
				doctorBackfillEntry(agentCoord, agentPath, agentLink),
				doctorBackfillEntry(registryCoord, registrySkillPath, registryLink),
			},
		},
		{
			name:  "already recorded placement is tracked",
			links: []installer.ScannedLink{skillLink},
			store: doctorBackfillStore(skillCoord, installstore.Placement{
				Provider:    skillLink.Provider,
				Mechanism:   installstore.MechanismSymlink,
				Path:        skillLink.Path,
				InstalledAt: doctorBackfillTime(),
			}),
			wantTracked: 1,
		},
		{
			name:  "same coord different placement still needs entry",
			links: []installer.ScannedLink{skillLink},
			store: doctorBackfillStore(skillCoord, installstore.Placement{
				Provider:    "codex",
				Mechanism:   installstore.MechanismSymlink,
				Path:        filepath.Join(tmp, "elsewhere", "dir-skill"),
				InstalledAt: doctorBackfillTime(),
			}),
			wantEntries: []BackfillEntry{doctorBackfillEntry(skillCoord, skillPath, skillLink)},
		},
		{
			name:        "legacy direct link is skipped",
			links:       []installer.ScannedLink{legacyLink},
			store:       &installstore.Store{Version: installstore.CurrentVersion},
			wantLegacy:  []installer.ScannedLink{legacyLink},
			wantTracked: 0,
		},
		{
			name:          "library link without item is unmatched",
			links:         []installer.ScannedLink{unmatchedLink},
			store:         &installstore.Store{Version: installstore.CurrentVersion},
			wantUnmatched: []installer.ScannedLink{unmatchedLink},
		},
		{
			name:  "nested target under item path matches",
			links: []installer.ScannedLink{nestedLink},
			store: &installstore.Store{Version: installstore.CurrentVersion},
			wantEntries: []BackfillEntry{
				doctorBackfillEntry(nestedCoord, nestedSkillPath, nestedLink),
			},
		},
		{
			name:  "broken links are ignored",
			links: []installer.ScannedLink{brokenLink},
			store: &installstore.Store{Version: installstore.CurrentVersion},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := PlanRecordBackfill(tt.links, tt.store, libraryRoot, items)
			if !reflect.DeepEqual(plan.Entries, tt.wantEntries) {
				t.Fatalf("Entries = %#v, want %#v", plan.Entries, tt.wantEntries)
			}
			if !reflect.DeepEqual(plan.Legacy, tt.wantLegacy) {
				t.Fatalf("Legacy = %#v, want %#v", plan.Legacy, tt.wantLegacy)
			}
			if !reflect.DeepEqual(plan.Unmatched, tt.wantUnmatched) {
				t.Fatalf("Unmatched = %#v, want %#v", plan.Unmatched, tt.wantUnmatched)
			}
			if plan.Tracked != tt.wantTracked {
				t.Fatalf("Tracked = %d, want %d", plan.Tracked, tt.wantTracked)
			}
		})
	}
}

func TestCheckInstallRecordsAt(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	libraryRoot := filepath.Join(tmp, "content")
	skillPath := filepath.Join(libraryRoot, "skills", "dir-skill")
	skill := catalog.ContentItem{Name: "dir-skill", Type: catalog.Skills, Path: skillPath}
	skillCoord := installstore.Coord{Type: string(catalog.Skills), Name: "dir-skill"}
	skillLink := doctorBackfillLink("claude-code", catalog.Skills, filepath.Join(tmp, "provider", "skills", "dir-skill"), skillPath, installer.LinkHealthy)
	legacyLink := doctorBackfillLink("claude-code", catalog.Skills, filepath.Join(tmp, "provider", "skills", "legacy"), filepath.Join(tmp, "registries", "core", "skills", "legacy"), installer.LinkHealthy)
	unmatchedLink := doctorBackfillLink("codex", catalog.Skills, filepath.Join(tmp, "codex", "skills", "unknown"), filepath.Join(libraryRoot, "skills", "unknown"), installer.LinkHealthy)
	brokenLink := doctorBackfillLink("claude-code", catalog.Skills, filepath.Join(tmp, "provider", "skills", "broken"), filepath.Join(libraryRoot, "skills", "broken"), installer.LinkBroken)

	tests := []struct {
		name             string
		links            []installer.ScannedLink
		store            *installstore.Store
		wantStatus       CheckStatus
		wantMessage      string
		wantDetails      []string
		wantHintPresent  bool
		wantHintExcluded bool
	}{
		{
			name:        "no links",
			store:       &installstore.Store{Version: installstore.CurrentVersion},
			wantStatus:  CheckOK,
			wantMessage: "Install records: no provider links to track",
		},
		{
			name:        "broken links do not count",
			links:       []installer.ScannedLink{brokenLink},
			store:       &installstore.Store{Version: installstore.CurrentVersion},
			wantStatus:  CheckOK,
			wantMessage: "Install records: no provider links to track",
		},
		{
			name:  "tracked link is clean",
			links: []installer.ScannedLink{skillLink},
			store: doctorBackfillStore(skillCoord, installstore.Placement{
				Provider:    skillLink.Provider,
				Mechanism:   installstore.MechanismSymlink,
				Path:        skillLink.Path,
				InstalledAt: doctorBackfillTime(),
			}),
			wantStatus:  CheckOK,
			wantMessage: "Install records: 1 link(s) tracked",
		},
		{
			name:        "untracked library link warns with fix hint",
			links:       []installer.ScannedLink{skillLink},
			store:       &installstore.Store{Version: installstore.CurrentVersion},
			wantStatus:  CheckWarn,
			wantMessage: "Install records: 1 untracked link(s)",
			wantDetails: []string{
				"untracked: claude-code skills/dir-skill",
				"Run 'syllago doctor --fix' to backfill install records",
			},
			wantHintPresent: true,
		},
		{
			name:        "legacy and unknown without entries warn without fix hint",
			links:       []installer.ScannedLink{legacyLink, unmatchedLink},
			store:       &installstore.Store{Version: installstore.CurrentVersion},
			wantStatus:  CheckWarn,
			wantMessage: "Install records: 2 link(s) not reconciled",
			wantDetails: []string{
				"legacy: " + legacyLink.Path + " -> " + legacyLink.Target,
				"unknown: " + unmatchedLink.Path + " -> " + unmatchedLink.Target,
			},
			wantHintExcluded: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkInstallRecordsAt(tt.links, tt.store, libraryRoot, []catalog.ContentItem{skill})
			if result.Name != "install-records" {
				t.Fatalf("Name = %q, want install-records", result.Name)
			}
			if result.Status != tt.wantStatus {
				t.Fatalf("Status = %s, want %s: %#v", result.Status, tt.wantStatus, result)
			}
			if result.Message != tt.wantMessage {
				t.Fatalf("Message = %q, want %q", result.Message, tt.wantMessage)
			}
			if tt.wantDetails != nil && !reflect.DeepEqual(result.Details, tt.wantDetails) {
				t.Fatalf("Details = %#v, want %#v", result.Details, tt.wantDetails)
			}
			hasHint := doctorDetailsContain(result.Details, "Run 'syllago doctor --fix' to backfill install records")
			if tt.wantHintPresent && !hasHint {
				t.Fatalf("Details = %#v, want doctor --fix hint", result.Details)
			}
			if tt.wantHintExcluded && hasHint {
				t.Fatalf("Details = %#v, did not want doctor --fix hint", result.Details)
			}
		})
	}
}

func doctorBackfillLink(provider string, ct catalog.ContentType, path, target string, class installer.LinkClass) installer.ScannedLink {
	return installer.ScannedLink{
		Provider:    provider,
		ContentType: ct,
		Path:        path,
		Target:      target,
		Class:       class,
	}
}

func doctorBackfillEntry(coord installstore.Coord, libraryPath string, link installer.ScannedLink) BackfillEntry {
	return BackfillEntry{
		Coord:       coord,
		LibraryPath: libraryPath,
		Placement: installstore.PlacementInput{
			Provider:  link.Provider,
			Mechanism: installstore.MechanismSymlink,
			Path:      link.Path,
		},
	}
}

func doctorBackfillStore(coord installstore.Coord, placement installstore.Placement) *installstore.Store {
	return &installstore.Store{
		Version: installstore.CurrentVersion,
		Records: []installstore.Record{
			{
				Coord:       coord,
				ContentHash: "sha256:test",
				LibraryPath: "/tmp/library",
				InstalledAt: doctorBackfillTime(),
				UpdatedAt:   doctorBackfillTime(),
				Placements:  []installstore.Placement{placement},
			},
		},
	}
}

func doctorBackfillTime() time.Time {
	return time.Date(2026, 8, 22, 9, 0, 0, 0, time.UTC)
}

func doctorDetailsContain(details []string, needle string) bool {
	for _, detail := range details {
		if strings.Contains(detail, needle) {
			return true
		}
	}
	return false
}
