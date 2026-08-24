package tui

import (
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
)

func recordTUIInstallBookkeeping(item catalog.ContentItem, provSlug string, pl installer.Placement) {
	storePath, err := installstore.DefaultPath()
	if err != nil {
		return
	}
	// The TUI has no stable stderr surface mid-render; install-state
	// bookkeeping is best-effort and must not disturb the Elm loop.
	_ = installstore.RecordInstallMeta(storePath, tuiInstallRecordCoord(item), item.Path, tuiInstallRecordPlacement(provSlug, pl), installstore.InstallMeta{
		SourceSHA: tuiInstallRecordSourceSHA(item),
	}, time.Now())
}

func recordTUIMOATInstallBookkeeping(item catalog.ContentItem, provSlug string, pl installer.Placement, moatProv *installstore.MOATProvenance) {
	storePath, err := installstore.DefaultPath()
	if err != nil {
		return
	}
	// The TUI has no stable stderr surface mid-render; install-state
	// bookkeeping is best-effort and must not disturb the Elm loop.
	_ = installstore.RecordInstallMeta(storePath, tuiInstallRecordCoord(item), item.Path, tuiInstallRecordPlacement(provSlug, pl), installstore.InstallMeta{
		MOAT:      moatProv,
		SourceSHA: tuiInstallRecordSourceSHA(item),
	}, time.Now())
}

func recordTUIAddUpdateBookkeeping(regName, contentType, name, libraryFallbackPath, sourceSHA string) {
	storePath, err := installstore.DefaultPath()
	if err != nil {
		return
	}
	store, err := installstore.Load(storePath)
	if err != nil {
		return
	}
	coord := installstore.Coord{Registry: regName, Type: contentType, Name: name}
	rec := store.Find(coord)
	if rec == nil {
		return
	}
	libraryPath := rec.LibraryPath
	if libraryPath == "" {
		libraryPath = libraryFallbackPath
	}
	// The TUI has no stable stderr surface mid-render; install-state
	// bookkeeping is best-effort and must not disturb the Elm loop.
	_ = installstore.RecordUpdate(storePath, coord, libraryPath, sourceSHA, "", time.Now())
}

func recordTUIMOATUpdateBookkeeping(item catalog.ContentItem, prevCopyPath string) {
	storePath, err := installstore.DefaultPath()
	if err != nil {
		return
	}
	store, err := installstore.Load(storePath)
	if err != nil {
		return
	}
	coord := tuiInstallRecordCoord(item)
	rec := store.Find(coord)
	if rec == nil {
		return
	}
	// The TUI has no stable stderr surface mid-render; install-state
	// bookkeeping is best-effort and must not disturb the Elm loop.
	_ = installstore.RecordUpdate(storePath, coord, rec.LibraryPath, "", prevCopyPath, time.Now())
}

func recordTUIUninstallBookkeeping(item catalog.ContentItem, provSlug string, pl installer.Placement) {
	storePath, err := installstore.DefaultPath()
	if err != nil {
		return
	}
	// The TUI has no stable stderr surface mid-render; install-state
	// bookkeeping is best-effort and must not disturb the Elm loop.
	_ = installstore.RecordUninstall(storePath, tuiInstallRecordCoord(item), tuiInstallRecordPlacement(provSlug, pl), time.Now())
}

func forgetTUIInstallRecord(item catalog.ContentItem) {
	storePath, err := installstore.DefaultPath()
	if err != nil {
		return
	}
	// The TUI has no stable stderr surface mid-render; install-state
	// bookkeeping is best-effort and must not disturb the Elm loop.
	_ = installstore.ForgetRecord(storePath, tuiInstallRecordCoord(item))
}

func tuiInstallRecordCoord(item catalog.ContentItem) installstore.Coord {
	registry := item.Registry
	if registry == "" && item.Meta != nil && item.Meta.SourceType == "registry" && item.Meta.SourceRegistry != "" {
		registry = item.Meta.SourceRegistry
	}
	return installstore.Coord{
		Registry: registry,
		Type:     string(item.Type),
		Name:     item.Name,
	}
}

func tuiInstallRecordPlacement(provSlug string, pl installer.Placement) installstore.PlacementInput {
	return installstore.PlacementInput{
		Provider:  provSlug,
		Mechanism: installstore.Mechanism(pl.Mechanism),
		Path:      pl.Path,
		Keys:      pl.Keys,
	}
}

func tuiInstallRecordSourceSHA(item catalog.ContentItem) string {
	if item.Meta == nil {
		return ""
	}
	return item.Meta.SourceSHA
}
