package capmon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
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

// runStage1Fetch fetches all source URLs from provider source manifests and writes
// content to the cache. Skips unchanged content (hash comparison). Records per-source
// errors but continues — a single bad URL does not abort the provider.
func runStage1Fetch(ctx context.Context, opts PipelineOptions, manifest *RunManifest) error {
	manifests, err := LoadAllSourceManifests(opts.SourceManifestsDir)
	if err != nil {
		return fmt.Errorf("load source manifests: %w", err)
	}

	for _, m := range manifests {
		if opts.ProviderFilter != "" && m.Slug != opts.ProviderFilter {
			continue
		}
		status := manifest.Providers[m.Slug]
		status.Slug = m.Slug

		for ctName, ct := range m.ContentTypes {
			for i, src := range ct.Sources {
				if err := ValidateSourceURL(src.URL); err != nil {
					status.Errors = append(status.Errors, fmt.Sprintf("%s.%d: SSRF rejected: %v", ctName, i, err))
					continue
				}
				sourceID := fmt.Sprintf("%s.%d", ctName, i)
				entry, fetchErr := FetchSource(ctx, opts.CacheRoot, m.Slug, sourceID, src.URL)
				if fetchErr != nil {
					status.Errors = append(status.Errors, fmt.Sprintf("%s: %v", sourceID, fetchErr))
					continue
				}
				// Patch meta with format and URL for stage 2
				entry.Meta.Format = src.Format
				entry.Meta.SourceURL = src.URL
				if err := WriteCacheMeta(opts.CacheRoot, *entry); err != nil {
					status.Errors = append(status.Errors, fmt.Sprintf("%s meta: %v", sourceID, err))
				}
				status.SourcesFetched++
			}
		}
		manifest.Providers[m.Slug] = status
	}
	return nil
}

// runStage2Extract reads each cached source, runs the appropriate extractor,
// and writes extracted.json alongside raw.bin in the cache entry directory.
func runStage2Extract(ctx context.Context, opts PipelineOptions, manifest *RunManifest) error {
	manifests, err := LoadAllSourceManifests(opts.SourceManifestsDir)
	if err != nil {
		return fmt.Errorf("load source manifests: %w", err)
	}

	for _, m := range manifests {
		if opts.ProviderFilter != "" && m.Slug != opts.ProviderFilter {
			continue
		}
		status := manifest.Providers[m.Slug]
		status.Slug = m.Slug

		for ctName, ct := range m.ContentTypes {
			for i, src := range ct.Sources {
				sourceID := fmt.Sprintf("%s.%d", ctName, i)
				if !IsCached(opts.CacheRoot, m.Slug, sourceID) {
					continue
				}
				entry, err := ReadCacheEntry(opts.CacheRoot, m.Slug, sourceID)
				if err != nil {
					status.Errors = append(status.Errors, fmt.Sprintf("%s read: %v", sourceID, err))
					continue
				}
				format := entry.Meta.Format
				if format == "" {
					format = src.Format
				}
				result, err := Extract(ctx, format, entry.Raw, src.Selector)
				if err != nil {
					var noExt *ErrNoExtractor
					if errors.As(err, &noExt) {
						status.Warnings = append(status.Warnings, fmt.Sprintf("%s: format %q has no extractor (skipped)", sourceID, format))
						status.SourcesSkipped++
					} else {
						status.Errors = append(status.Errors, fmt.Sprintf("%s extract(%s): %v", sourceID, format, err))
					}
					continue
				}
				// Write extracted.json into the cache entry directory
				outPath := filepath.Join(opts.CacheRoot, m.Slug, sourceID, "extracted.json")
				data, _ := json.MarshalIndent(result, "", "  ")
				if err := os.WriteFile(outPath, data, 0644); err != nil {
					status.Errors = append(status.Errors, fmt.Sprintf("%s write extracted: %v", sourceID, err))
					continue
				}
				status.SourcesExtracted++
			}
		}
		manifest.Providers[m.Slug] = status
	}
	return nil
}

// runStage3Diff loads extracted.json files from the cache and compares them
// against the current provider capability YAMLs in CapabilitiesDir.
// Diff results are recorded in the manifest for Stage 4 to act on.
func runStage3Diff(_ context.Context, opts PipelineOptions, manifest *RunManifest) error {
	for slug, status := range manifest.Providers {
		if status.SourcesExtracted == 0 {
			continue
		}
		// Collect all extracted field values for this provider
		combined := make(map[string]string)
		cacheProviderDir := filepath.Join(opts.CacheRoot, slug)
		entries, err := os.ReadDir(cacheProviderDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			extPath := filepath.Join(cacheProviderDir, e.Name(), "extracted.json")
			data, err := os.ReadFile(extPath)
			if err != nil {
				continue
			}
			var src ExtractedSource
			if err := json.Unmarshal(data, &src); err != nil {
				continue
			}
			for k, fv := range src.Fields {
				combined[k] = fv.Value
			}
		}

		// Compare against capability YAML if it exists
		capsPath := filepath.Join(opts.CapabilitiesDir, slug+".yaml")
		if _, err := os.Stat(capsPath); os.IsNotExist(err) {
			// No baseline yet — record as new provider, no diff
			status.NeedsBaseline = true
			manifest.Providers[slug] = status
			continue
		}
		caps, err := loadCurrentFields(capsPath)
		if err != nil {
			status.Errors = append(status.Errors, fmt.Sprintf("load caps: %v", err))
			manifest.Providers[slug] = status
			continue
		}
		diff := DiffProviderCapabilities(slug, manifest.RunID, &ExtractedSource{Fields: toFieldValues(combined)}, caps)
		if len(diff.Changes) > 0 {
			status.HasDrift = true
			status.Diff = &diff
		}
		manifest.Providers[slug] = status
	}
	return nil
}

// runStage4Review creates PRs and issues for providers with drift or failures.
func runStage4Review(ctx context.Context, opts PipelineOptions, manifest *RunManifest) error {
	for slug, status := range manifest.Providers {
		if status.HasDrift && status.Diff != nil {
			existing, found, err := DeduplicatePR(ctx, slug)
			if err == nil && found {
				_ = existing // PR already open, skip
				continue
			}
			if _, err := CreateDriftPR(ctx, slug, manifest.RunID, *status.Diff); err != nil {
				status.Errors = append(status.Errors, fmt.Sprintf("create PR: %v", err))
				manifest.Providers[slug] = status
			}
		}
	}
	return nil
}

// loadCurrentFields reads a capability YAML and returns a flat field→value map.
// Keys are dot-delimited paths (e.g. "content_types.hooks.events.before_tool_execute.native_name").
func loadCurrentFields(capsPath string) (map[string]string, error) {
	data, err := os.ReadFile(capsPath)
	if err != nil {
		return nil, err
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse YAML %s: %w", capsPath, err)
	}
	result := make(map[string]string)
	flattenInterface("", raw, result)
	return result, nil
}

func flattenInterface(prefix string, v interface{}, out map[string]string) {
	switch val := v.(type) {
	case map[string]interface{}:
		for k, child := range val {
			key := k
			if prefix != "" {
				key = prefix + "." + k
			}
			flattenInterface(key, child, out)
		}
	case string:
		out[prefix] = val
	case bool:
		if val {
			out[prefix] = "true"
		} else {
			out[prefix] = "false"
		}
	case float64:
		out[prefix] = fmt.Sprintf("%g", val)
	case int:
		// yaml.v3 unmarshals integers as int (not float64)
		out[prefix] = fmt.Sprintf("%d", val)
	}
}

func toFieldValues(m map[string]string) map[string]FieldValue {
	out := make(map[string]FieldValue, len(m))
	for k, v := range m {
		out[k] = FieldValue{Value: v, ValueHash: SHA256Hex([]byte(v))}
	}
	return out
}
