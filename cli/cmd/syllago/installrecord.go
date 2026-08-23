package main

import (
	"fmt"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// recordInstallBookkeeping best-effort-records a successful install. Never
// fails the install: any error is reported as a warning on stderr.
func recordInstallBookkeeping(item catalog.ContentItem, provSlug string, pl installer.Placement) {
	storePath, err := installstore.DefaultPath()
	if err != nil {
		warnInstallRecord(err)
		return
	}
	if err := installstore.RecordInstall(storePath, installRecordCoord(item), item.Path, installRecordPlacement(provSlug, pl), time.Now()); err != nil {
		warnInstallRecord(err)
	}
}

// recordMOATInstallBookkeeping best-effort-records a successful MOAT install
// with provenance. Never fails the install: any error is reported as a warning.
func recordMOATInstallBookkeeping(item catalog.ContentItem, provSlug string, pl installer.Placement, moatProv *installstore.MOATProvenance) {
	storePath, err := installstore.DefaultPath()
	if err != nil {
		warnInstallRecord(err)
		return
	}
	if err := installstore.RecordInstallMOAT(storePath, installRecordCoord(item), item.Path, installRecordPlacement(provSlug, pl), moatProv, time.Now()); err != nil {
		warnInstallRecord(err)
	}
}

// recordUninstallBookkeeping mirrors recordInstallBookkeeping for uninstalls.
func recordUninstallBookkeeping(item catalog.ContentItem, provSlug string, pl installer.Placement) {
	storePath, err := installstore.DefaultPath()
	if err != nil {
		warnInstallRecord(err)
		return
	}
	if err := installstore.RecordUninstall(storePath, installRecordCoord(item), installRecordPlacement(provSlug, pl), time.Now()); err != nil {
		warnInstallRecord(err)
	}
}

// forgetInstallRecord drops the whole record after a library removal.
func forgetInstallRecord(item catalog.ContentItem) {
	storePath, err := installstore.DefaultPath()
	if err != nil {
		warnInstallRecord(err)
		return
	}
	if err := installstore.ForgetRecord(storePath, installRecordCoord(item)); err != nil {
		warnInstallRecord(err)
	}
}

func installRecordCoord(item catalog.ContentItem) installstore.Coord {
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

func installRecordPlacement(provSlug string, pl installer.Placement) installstore.PlacementInput {
	return installstore.PlacementInput{
		Provider:  provSlug,
		Mechanism: installstore.Mechanism(pl.Mechanism),
		Path:      pl.Path,
		Keys:      pl.Keys,
	}
}

func warnInstallRecord(err error) {
	fmt.Fprintf(output.ErrWriter, "warning: could not record install state: %v\n", err)
}
