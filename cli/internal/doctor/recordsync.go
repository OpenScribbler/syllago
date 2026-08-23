package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// BackfillEntry is one healthy library-backed provider link with no
// matching install record.
type BackfillEntry struct {
	Coord       installstore.Coord
	LibraryPath string
	Placement   installstore.PlacementInput
}

// BackfillPlan classifies healthy syllago-owned provider links against the
// install-record store.
type BackfillPlan struct {
	Entries   []BackfillEntry         // untracked library-backed links to record
	Legacy    []installer.ScannedLink // healthy links targeting outside the library root (registry checkout / MOAT cache)
	Unmatched []installer.ScannedLink // links inside the library root with no matching catalog item
	Tracked   int                     // healthy links already recorded
}

// PlanRecordBackfill classifies healthy syllago-owned provider links against
// the install-record store.
func PlanRecordBackfill(links []installer.ScannedLink, store *installstore.Store, libraryRoot string, items []catalog.ContentItem) BackfillPlan {
	var plan BackfillPlan
	for _, link := range links {
		if link.Class == installer.LinkBroken {
			continue
		}
		if !recordSyncPathWithinRoot(link.Target, libraryRoot) {
			plan.Legacy = append(plan.Legacy, link)
			continue
		}

		item, ok := recordSyncItemForLink(link, items)
		if !ok {
			plan.Unmatched = append(plan.Unmatched, link)
			continue
		}

		coord := installstore.Coord{
			Registry: recordSyncRegistryFor(item),
			Type:     string(item.Type),
			Name:     item.Name,
		}
		if recordSyncPlacementTracked(store.Find(coord), link) {
			plan.Tracked++
			continue
		}

		plan.Entries = append(plan.Entries, BackfillEntry{
			Coord:       coord,
			LibraryPath: item.Path,
			Placement: installstore.PlacementInput{
				Provider:  link.Provider,
				Mechanism: installstore.MechanismSymlink,
				Path:      link.Path,
			},
		})
	}
	return plan
}

// CheckInstallRecords reconciles healthy provider links against the
// install-record store: reality-based complement to record-based checks.
func CheckInstallRecords() CheckResult {
	home, err := os.UserHomeDir()
	if err != nil {
		return installRecordsGatherWarn(err)
	}
	links := installer.ScanProviderLinks(provider.AllProviders, home, SyllagoOwnedRoots(home))

	storePath, err := installstore.DefaultPath()
	if err != nil {
		return installRecordsGatherWarn(err)
	}
	store, err := installstore.Load(storePath)
	if err != nil {
		return installRecordsGatherWarn(err)
	}

	items, err := recordSyncLibraryItems()
	if err != nil {
		return installRecordsGatherWarn(err)
	}
	return checkInstallRecordsAt(links, store, catalog.GlobalContentDir(), items)
}

func checkInstallRecordsAt(links []installer.ScannedLink, store *installstore.Store, libraryRoot string, items []catalog.ContentItem) CheckResult {
	plan := PlanRecordBackfill(links, store, libraryRoot, items)
	if len(plan.Entries) == 0 && len(plan.Legacy) == 0 && len(plan.Unmatched) == 0 {
		if plan.Tracked == 0 {
			return CheckResult{Name: "install-records", Status: CheckOK, Message: "Install records: no provider links to track"}
		}
		return CheckResult{Name: "install-records", Status: CheckOK, Message: fmt.Sprintf("Install records: %d link(s) tracked", plan.Tracked)}
	}

	details := make([]string, 0, len(plan.Entries)+len(plan.Legacy)+len(plan.Unmatched)+1)
	for _, entry := range plan.Entries {
		details = append(details, fmt.Sprintf("untracked: %s %s/%s", entry.Placement.Provider, entry.Coord.Type, entry.Coord.Name))
	}
	for _, link := range plan.Legacy {
		details = append(details, fmt.Sprintf("legacy: %s -> %s", link.Path, link.Target))
	}
	for _, link := range plan.Unmatched {
		details = append(details, fmt.Sprintf("unknown: %s -> %s", link.Path, link.Target))
	}
	if len(plan.Entries) > 0 {
		details = append(details, "Run 'syllago doctor --fix' to backfill install records")
	}

	message := fmt.Sprintf("Install records: %d untracked link(s)", len(plan.Entries))
	if len(plan.Entries) == 0 {
		message = fmt.Sprintf("Install records: %d link(s) not reconciled", len(plan.Legacy)+len(plan.Unmatched))
	}
	return CheckResult{Name: "install-records", Status: CheckWarn, Message: message, Details: details}
}

func installRecordsGatherWarn(err error) CheckResult {
	return CheckResult{
		Name:    "install-records",
		Status:  CheckWarn,
		Message: "Install records: could not reconcile",
		Details: []string{err.Error()},
	}
}

func recordSyncLibraryItems() ([]catalog.ContentItem, error) {
	emptyProjectRoot, err := os.MkdirTemp("", "syllago-doctor-records-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(emptyProjectRoot) }()

	cat, err := catalog.ScanWithGlobalAndRegistries(emptyProjectRoot, emptyProjectRoot, nil)
	if err != nil {
		return nil, fmt.Errorf("scanning library: %w", err)
	}

	var items []catalog.ContentItem
	for _, item := range cat.Items {
		if item.Source == "global" {
			items = append(items, item)
		}
	}
	return items, nil
}

func recordSyncItemForLink(link installer.ScannedLink, items []catalog.ContentItem) (catalog.ContentItem, bool) {
	for _, item := range items {
		if item.Type != link.ContentType {
			continue
		}
		if link.Target == item.Path || link.Target == installer.SourcePathFor(item) || recordSyncPathWithinRoot(link.Target, item.Path) {
			return item, true
		}
	}
	return catalog.ContentItem{}, false
}

func recordSyncPlacementTracked(rec *installstore.Record, link installer.ScannedLink) bool {
	if rec == nil {
		return false
	}
	for _, placement := range rec.Placements {
		if placement.Provider == link.Provider &&
			placement.Mechanism == installstore.MechanismSymlink &&
			placement.Path == link.Path {
			return true
		}
	}
	return false
}

func recordSyncRegistryFor(item catalog.ContentItem) string {
	registry := item.Registry
	if registry == "" && item.Meta != nil && item.Meta.SourceType == "registry" && item.Meta.SourceRegistry != "" {
		registry = item.Meta.SourceRegistry
	}
	return registry
}

func recordSyncPathWithinRoot(path, root string) bool {
	if path == "" || root == "" {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}
