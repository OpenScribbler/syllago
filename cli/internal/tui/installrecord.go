package tui

import (
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
)

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
