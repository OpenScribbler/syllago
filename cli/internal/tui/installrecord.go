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
	_ = installstore.RecordInstall(storePath, tuiInstallRecordCoord(item), item.Path, tuiInstallRecordPlacement(provSlug, pl), time.Now())
}

func recordTUIMOATInstallBookkeeping(item catalog.ContentItem, provSlug string, pl installer.Placement, moatProv *installstore.MOATProvenance) {
	storePath, err := installstore.DefaultPath()
	if err != nil {
		return
	}
	// The TUI has no stable stderr surface mid-render; install-state
	// bookkeeping is best-effort and must not disturb the Elm loop.
	_ = installstore.RecordInstallMOAT(storePath, tuiInstallRecordCoord(item), item.Path, tuiInstallRecordPlacement(provSlug, pl), moatProv, time.Now())
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
	return installstore.Coord{
		Registry: item.Registry,
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
