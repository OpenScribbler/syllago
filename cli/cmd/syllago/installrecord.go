package main

import (
	"fmt"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// recordAddUpdateBookkeeping rotates install records for items that were
// force-overwritten in the library, capturing the one-step rollback point.
func recordAddUpdateBookkeeping(results []add.AddResult, regName, sourceSHA string) {
	for _, r := range results {
		if r.Status != add.AddStatusUpdated {
			continue
		}
		storePath, err := installstore.DefaultPath()
		if err != nil {
			warnInstallRecord(err)
			continue
		}
		coord := installstore.Coord{Registry: regName, Type: string(r.Type), Name: r.Name}
		store, err := installstore.Load(storePath)
		if err != nil {
			warnInstallRecord(err)
			continue
		}
		rec := store.Find(coord)
		if rec == nil {
			continue
		}
		if err := installstore.RecordUpdate(storePath, coord, rec.LibraryPath, sourceSHA, "", time.Now()); err != nil {
			warnInstallRecord(err)
		}
	}
}

func warnInstallRecord(err error) {
	fmt.Fprintf(output.ErrWriter, "warning: could not record install state: %v\n", err)
}
