package drift

// DriftReport describes what changed between baseline and current scan.
type DriftReport struct {
	Changed []DriftItem `json:"changed"`
	New     []DriftItem `json:"new"`
	Removed []DriftItem `json:"removed"`
	Clean   bool        `json:"clean"`
}

// DriftItem represents a single changed/new/removed section.
type DriftItem struct {
	Category string `json:"category"`
	Title    string `json:"title"`
}

// Diff compares a current scan against a baseline snapshot.
func Diff(baseline *Baseline, current *Baseline) DriftReport {
	report := DriftReport{}

	baseMap := make(map[string]BaselineSection)
	for _, s := range baseline.Sections {
		key := s.Category + ":" + s.Title
		baseMap[key] = s
	}

	currMap := make(map[string]BaselineSection)
	for _, s := range current.Sections {
		key := s.Category + ":" + s.Title
		currMap[key] = s
	}

	// Check for changed and new
	for key, curr := range currMap {
		base, exists := baseMap[key]
		if !exists {
			report.New = append(report.New, DriftItem{
				Category: curr.Category,
				Title:    curr.Title,
			})
		} else if base.Hash != curr.Hash {
			report.Changed = append(report.Changed, DriftItem{
				Category: curr.Category,
				Title:    curr.Title,
			})
		}
	}

	// Check for removed
	for key, base := range baseMap {
		if _, exists := currMap[key]; !exists {
			report.Removed = append(report.Removed, DriftItem{
				Category: base.Category,
				Title:    base.Title,
			})
		}
	}

	report.Clean = len(report.Changed) == 0 && len(report.New) == 0 && len(report.Removed) == 0
	return report
}
