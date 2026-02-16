package detectors

import "testing"

func TestAllDetectorsCount(t *testing.T) {
	all := AllDetectors()
	// 5 fact + 24 surprise = 29 total
	if len(all) != 29 {
		t.Errorf("expected 29 detectors, got %d", len(all))
	}
}

func TestAllDetectorsUniqueNames(t *testing.T) {
	all := AllDetectors()
	seen := make(map[string]bool)
	for _, d := range all {
		name := d.Name()
		if seen[name] {
			t.Errorf("duplicate detector name: %s", name)
		}
		seen[name] = true
	}
}
