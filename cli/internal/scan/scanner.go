package scan

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

// Detector is the interface implemented by all fact and surprise detectors.
type Detector interface {
	Name() string
	Detect(root string) ([]model.Section, error)
}

// ScanResult holds the output of a complete scan.
type ScanResult struct {
	Document model.ContextDocument `json:"document"`
	Warnings []Warning             `json:"warnings,omitempty"`
	Duration time.Duration         `json:"duration"`
}

// Warning records a non-fatal detector issue (timeout, panic, error).
type Warning struct {
	Detector string `json:"detector"`
	Message  string `json:"message"`
}

// DefaultTimeout is the per-detector timeout.
const DefaultTimeout = 5 * time.Second

// Scanner runs detectors and assembles results.
type Scanner struct {
	Detectors []Detector
	Timeout   time.Duration
}

// NewScanner creates a scanner with the given detectors and default timeout.
func NewScanner(detectors ...Detector) *Scanner {
	return &Scanner{
		Detectors: detectors,
		Timeout:   DefaultTimeout,
	}
}

// Run executes all detectors in parallel and assembles a ContextDocument.
func (s *Scanner) Run(projectRoot string) ScanResult {
	start := time.Now()

	type detectorResult struct {
		name     string
		sections []model.Section
		warning  *Warning
	}

	results := make(chan detectorResult, len(s.Detectors))
	var wg sync.WaitGroup

	for _, d := range s.Detectors {
		wg.Add(1)
		go func(det Detector) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), s.Timeout)
			defer cancel()

			doneCh := make(chan detectorResult, 1)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						doneCh <- detectorResult{
							name:    det.Name(),
							warning: &Warning{Detector: det.Name(), Message: fmt.Sprintf("panic: %v", r)},
						}
					}
				}()
				sections, err := det.Detect(projectRoot)
				if err != nil {
					doneCh <- detectorResult{
						name:    det.Name(),
						warning: &Warning{Detector: det.Name(), Message: err.Error()},
					}
					return
				}
				doneCh <- detectorResult{name: det.Name(), sections: sections}
			}()

			select {
			case r := <-doneCh:
				results <- r
			case <-ctx.Done():
				results <- detectorResult{
					name:    det.Name(),
					warning: &Warning{Detector: det.Name(), Message: "timeout after " + s.Timeout.String()},
				}
			}
		}(d)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var allSections []model.Section
	var warnings []Warning
	for r := range results {
		allSections = append(allSections, r.sections...)
		if r.warning != nil {
			warnings = append(warnings, *r.warning)
		}
	}

	sort.Slice(allSections, func(i, j int) bool {
		return categoryOrder(allSections[i].SectionCategory()) < categoryOrder(allSections[j].SectionCategory())
	})

	return ScanResult{
		Document: model.ContextDocument{
			ProjectName: projectRoot,
			ScanTime:    start,
			Sections:    allSections,
		},
		Warnings: warnings,
		Duration: time.Since(start),
	}
}

// categoryOrder defines the section ordering in emitted output.
func categoryOrder(c model.Category) int {
	switch c {
	case model.CatTechStack:
		return 0
	case model.CatDependencies:
		return 1
	case model.CatBuildCommands:
		return 2
	case model.CatDirStructure:
		return 3
	case model.CatProjectMeta:
		return 4
	case model.CatConventions:
		return 5
	case model.CatSurprise:
		return 6
	case model.CatCurated:
		return 7
	default:
		return 99
	}
}
