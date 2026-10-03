package main

import (
	"errors"
	"fmt"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/lifecycle"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/rollback"
)

// addRespectingPins adds items to the Library through the lifecycle
// Overwrite, so an add never writes over a pinned Library item. It reports
// each pinned item and leaves it out, and fails when every item is pinned.
// A replaced item's install record keeps the version it replaced.
func addRespectingPins(items []add.DiscoveryItem, opts add.AddOptions, globalDir string, canon add.Canonicalizer, ver string) ([]add.AddResult, error) {
	dest := func(t add.DiscoveryItem) string { return add.DestDir(t.Type, opts.Provider, t.Name, globalDir) }
	dests := make([]lifecycle.Destination, 0, len(items))
	for _, item := range items {
		dests = append(dests, lifecycle.Destination{Type: item.Type, Name: item.Name, Path: dest(item)})
	}
	var results []add.AddResult
	req := lifecycle.OverwriteRequest{
		Destinations: dests,
		Write: func(approved []lifecycle.Destination) ([]lifecycle.Written, error) {
			ok := make(map[string]bool, len(approved))
			for _, d := range approved {
				ok[d.Path] = true
			}
			var write []add.DiscoveryItem
			for _, item := range items {
				if ok[dest(item)] {
					write = append(write, item)
				}
			}
			results = add.AddItems(write, opts, globalDir, canon, ver)
			written := make([]lifecycle.Written, 0, len(approved))
			for _, d := range approved {
				written = append(written, lifecycle.Written{Path: d.Path, SourceSHA: opts.SourceSHA})
			}
			return written, nil
		},
	}
	m := lifecycle.New()
	out, err := m.Overwrite(req)
	var decision *lifecycle.DecisionRequired
	if errors.As(err, &decision) {
		pinned, _ := decision.Context.([]lifecycle.PinnedDestination)
		req.Decisions.Overwrite = make(map[string]bool, len(pinned))
		for _, p := range pinned {
			req.Decisions.Overwrite[p.Path] = false
			if p.SourceSHA != "" {
				fmt.Fprintf(output.ErrWriter, "  pinned %s/%s — holding at %s; unpin to update: syllago unpin %s\n", p.Type, p.Name, rollback.ShortSHA(p.SourceSHA), p.Name)
			} else {
				fmt.Fprintf(output.ErrWriter, "  pinned %s/%s; unpin to update: syllago unpin %s\n", p.Type, p.Name, p.Name)
			}
		}
		if len(pinned) == len(dests) {
			return nil, output.NewStructuredError(
				output.ErrInstallConflict,
				"all requested items are pinned",
				"Run 'syllago unpin <name>' first to unpin the items you want to update",
			)
		}
		out, err = m.Overwrite(req)
	}
	var structured output.StructuredError
	if errors.As(err, &structured) {
		return nil, err
	}
	if err != nil {
		return nil, output.NewStructuredErrorDetail(output.ErrInstallNotWritable, "adding to the library", "Check permissions on ~/.syllago and that installs.json is valid JSON", err.Error())
	}
	printLifecycleWarnings(output.ErrWriter, out)
	return results, nil
}
