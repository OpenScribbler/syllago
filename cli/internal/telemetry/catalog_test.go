package telemetry

import (
	"testing"
)

// TestEventCatalog_CapmonFetchProperties verifies that the telemetry catalog
// registers capmon_fetch in the expected property Commands lists and that
// source_count and fetch_errors PropertyDef entries exist.
func TestEventCatalog_CapmonFetchProperties(t *testing.T) {
	// Build name → PropertyDef index across all events.
	propIndex := make(map[string]PropertyDef)
	for _, ev := range EventCatalog() {
		for _, p := range ev.Properties {
			propIndex[p.Name] = p
		}
	}

	containsCapmonFetch := func(cmds []string) bool {
		for _, c := range cmds {
			if c == "capmon_fetch" {
				return true
			}
		}
		return false
	}

	// capmon_fetch must be in provider.Commands.
	if prop, ok := propIndex["provider"]; !ok {
		t.Fatal("catalog missing 'provider' property")
	} else if !containsCapmonFetch(prop.Commands) {
		t.Errorf("capmon_fetch not in provider.Commands; got: %v", prop.Commands)
	}

	// capmon_fetch must be in dry_run.Commands.
	if prop, ok := propIndex["dry_run"]; !ok {
		t.Fatal("catalog missing 'dry_run' property")
	} else if !containsCapmonFetch(prop.Commands) {
		t.Errorf("capmon_fetch not in dry_run.Commands; got: %v", prop.Commands)
	}

	// source_count must exist as a registered property.
	if _, ok := propIndex["source_count"]; !ok {
		t.Error("catalog missing 'source_count' property — add it to command_executed properties")
	}

	// fetch_errors must exist as a registered property.
	if _, ok := propIndex["fetch_errors"]; !ok {
		t.Error("catalog missing 'fetch_errors' property — add it to command_executed properties")
	}
}
