package capmon

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"
)

// PipelineOptions controls which stages run and pipeline behavior.
type PipelineOptions struct {
	// ProviderFilter limits execution to a single provider slug. Empty = all providers.
	ProviderFilter string
	// Stage controls which pipeline stages run.
	// "": all stages (1-4)
	// "fetch-extract": stages 1-2 only
	// "report": stages 3-4 only
	Stage string
	// DryRun prevents Stage 4 from creating PRs/issues; writes report to w instead.
	DryRun bool
	// CacheRoot is the path to .capmon-cache/. Defaults to ".capmon-cache".
	CacheRoot string
	// SourceManifestsDir is the path to docs/provider-sources/. Defaults to "docs/provider-sources".
	SourceManifestsDir string
	// CapabilitiesDir is the path to docs/provider-capabilities/. Defaults to "docs/provider-capabilities".
	CapabilitiesDir string
}

// RunPipeline executes the capmon pipeline with the given options.
// Returns the exit class (0-5) and any fatal error.
func RunPipeline(ctx context.Context, opts PipelineOptions) (exitClass int, err error) {
	if opts.CacheRoot == "" {
		opts.CacheRoot = ".capmon-cache"
	}
	if opts.SourceManifestsDir == "" {
		opts.SourceManifestsDir = "docs/provider-sources"
	}
	if opts.CapabilitiesDir == "" {
		opts.CapabilitiesDir = "docs/provider-capabilities"
	}

	// Validate stage value
	switch opts.Stage {
	case "", "fetch-extract", "report":
		// valid
	default:
		return ExitFatal, fmt.Errorf("invalid --stage %q: must be 'fetch-extract' or 'report'", opts.Stage)
	}

	manifest := RunManifest{
		RunID:     generateRunID(),
		StartedAt: time.Now().UTC(),
		Providers: make(map[string]ProviderStatus),
	}

	runFetchExtract := opts.Stage == "" || opts.Stage == "fetch-extract"
	runReport := opts.Stage == "" || opts.Stage == "report"

	// Stage 1: Fetch
	if runFetchExtract {
		if err := runStage1Fetch(ctx, opts, &manifest); err != nil {
			manifest.ExitClass = ExitInfrastructureFailure
			WriteRunManifest(opts.CacheRoot, manifest) //nolint:errcheck
			return ExitInfrastructureFailure, err
		}
	}

	// Stage 2: Extract
	if runFetchExtract {
		if err := runStage2Extract(ctx, opts, &manifest); err != nil {
			manifest.ExitClass = ExitPartialFailure
			WriteRunManifest(opts.CacheRoot, manifest) //nolint:errcheck
			return ExitPartialFailure, err
		}
	}

	// Stage 3: Diff
	if runReport {
		if err := runStage3Diff(ctx, opts, &manifest); err != nil {
			manifest.ExitClass = ExitPartialFailure
			WriteRunManifest(opts.CacheRoot, manifest) //nolint:errcheck
			return ExitPartialFailure, err
		}
	}

	// Stage 4: Review/PR (skipped if paused or dry-run)
	if runReport {
		paused := false
		if _, statErr := os.Stat(".capmon-pause"); statErr == nil {
			paused = true
		}
		if !paused && !opts.DryRun {
			if err := runStage4Review(ctx, opts, &manifest); err != nil {
				manifest.ExitClass = ExitPartialFailure
				WriteRunManifest(opts.CacheRoot, manifest) //nolint:errcheck
				return ExitPartialFailure, err
			}
			if manifest.ExitClass == ExitClean {
				manifest.ExitClass = ExitClean
			}
		} else if paused {
			manifest.ExitClass = ExitPaused
		}
	}

	manifest.FinishedAt = time.Now().UTC()
	WriteRunManifest(opts.CacheRoot, manifest) //nolint:errcheck
	return manifest.ExitClass, nil
}

// generateRunID returns a short random run identifier.
func generateRunID() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// Stub implementations — filled in Phase 9.
func runStage1Fetch(_ context.Context, _ PipelineOptions, _ *RunManifest) error {
	return nil
}

func runStage2Extract(_ context.Context, _ PipelineOptions, _ *RunManifest) error {
	return nil
}

func runStage3Diff(_ context.Context, _ PipelineOptions, _ *RunManifest) error {
	return nil
}

func runStage4Review(_ context.Context, _ PipelineOptions, _ *RunManifest) error {
	return nil
}
