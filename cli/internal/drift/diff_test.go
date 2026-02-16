package drift

import "testing"

func TestDiffDetectsChanges(t *testing.T) {
	baseline := &Baseline{
		Sections: []BaselineSection{
			{Category: "tech-stack", Title: "Tech Stack", Hash: "aaa"},
			{Category: "dependencies", Title: "Dependencies", Hash: "bbb"},
		},
	}
	current := &Baseline{
		Sections: []BaselineSection{
			{Category: "tech-stack", Title: "Tech Stack", Hash: "aaa"}, // unchanged
			{Category: "dependencies", Title: "Dependencies", Hash: "ccc"}, // changed
			{Category: "surprise", Title: "New surprise", Hash: "ddd"}, // new
		},
	}

	report := Diff(baseline, current)
	if report.Clean {
		t.Error("report should not be clean")
	}
	if len(report.Changed) != 1 {
		t.Errorf("expected 1 changed, got %d", len(report.Changed))
	}
	if len(report.New) != 1 {
		t.Errorf("expected 1 new, got %d", len(report.New))
	}
}

func TestDiffDetectsRemoved(t *testing.T) {
	baseline := &Baseline{
		Sections: []BaselineSection{
			{Category: "tech-stack", Title: "Tech Stack", Hash: "aaa"},
		},
	}
	current := &Baseline{
		Sections: []BaselineSection{},
	}

	report := Diff(baseline, current)
	if len(report.Removed) != 1 {
		t.Errorf("expected 1 removed, got %d", len(report.Removed))
	}
}

func TestDiffClean(t *testing.T) {
	baseline := &Baseline{
		Sections: []BaselineSection{
			{Category: "tech-stack", Title: "Tech Stack", Hash: "aaa"},
		},
	}

	report := Diff(baseline, baseline)
	if !report.Clean {
		t.Error("identical baselines should be clean")
	}
}
