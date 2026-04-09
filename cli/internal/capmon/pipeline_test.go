package capmon_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestRunPipeline_InvalidStage(t *testing.T) {
	opts := capmon.PipelineOptions{
		Stage:     "invalid-stage",
		CacheRoot: t.TempDir(),
	}
	exitClass, err := capmon.RunPipeline(context.Background(), opts)
	if err == nil {
		t.Error("expected error for invalid stage")
	}
	if exitClass != capmon.ExitFatal {
		t.Errorf("expected ExitFatal (%d), got %d", capmon.ExitFatal, exitClass)
	}
}

func TestRunPipeline_AllStages_DryRun(t *testing.T) {
	cacheDir := t.TempDir()
	opts := capmon.PipelineOptions{
		DryRun:             true,
		CacheRoot:          cacheDir,
		SourceManifestsDir: t.TempDir(),
		CapabilitiesDir:    t.TempDir(),
	}
	exitClass, err := capmon.RunPipeline(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunPipeline dry-run: %v", err)
	}
	if exitClass != capmon.ExitClean {
		t.Errorf("expected ExitClean (%d), got %d", capmon.ExitClean, exitClass)
	}
}

func TestRunPipeline_FetchExtractStage(t *testing.T) {
	opts := capmon.PipelineOptions{
		Stage:              "fetch-extract",
		CacheRoot:          t.TempDir(),
		SourceManifestsDir: t.TempDir(),
		CapabilitiesDir:    t.TempDir(),
	}
	exitClass, err := capmon.RunPipeline(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunPipeline fetch-extract: %v", err)
	}
	if exitClass != capmon.ExitClean {
		t.Errorf("expected ExitClean (%d), got %d", capmon.ExitClean, exitClass)
	}
}

func TestRunPipeline_ReportStage(t *testing.T) {
	opts := capmon.PipelineOptions{
		Stage:              "report",
		DryRun:             true,
		CacheRoot:          t.TempDir(),
		SourceManifestsDir: t.TempDir(),
		CapabilitiesDir:    t.TempDir(),
	}
	exitClass, err := capmon.RunPipeline(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunPipeline report: %v", err)
	}
	if exitClass != capmon.ExitClean {
		t.Errorf("expected ExitClean (%d), got %d", capmon.ExitClean, exitClass)
	}
}

// TestRunPipeline_ReportNoDryRun exercises stage 4 (runStage4Review) by running
// the report stage without dry-run mode and without a pause sentinel.
func TestRunPipeline_ReportNoDryRun(t *testing.T) {
	// Ensure no .capmon-pause file in the working directory
	origDir, _ := os.Getwd()
	workDir := t.TempDir()
	os.Chdir(workDir)
	t.Cleanup(func() { os.Chdir(origDir) })

	opts := capmon.PipelineOptions{
		Stage:              "report",
		DryRun:             false, // stage 4 runs
		CacheRoot:          t.TempDir(),
		SourceManifestsDir: t.TempDir(),
		CapabilitiesDir:    t.TempDir(),
	}
	exitClass, err := capmon.RunPipeline(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunPipeline report (no dry-run): %v", err)
	}
	if exitClass != capmon.ExitClean {
		t.Errorf("expected ExitClean (%d), got %d", capmon.ExitClean, exitClass)
	}
}

func TestRunPipeline_PauseSentinel(t *testing.T) {
	cacheDir := t.TempDir()
	// Create .capmon-pause in a temp work dir
	workDir := t.TempDir()
	pauseFile := filepath.Join(workDir, ".capmon-pause")
	if err := os.WriteFile(pauseFile, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	// Override working directory for the sentinel check
	origDir, _ := os.Getwd()
	os.Chdir(workDir)
	t.Cleanup(func() { os.Chdir(origDir) })

	opts := capmon.PipelineOptions{
		Stage:              "report",
		CacheRoot:          cacheDir,
		SourceManifestsDir: t.TempDir(),
		CapabilitiesDir:    t.TempDir(),
	}
	exitClass, _ := capmon.RunPipeline(context.Background(), opts)
	if exitClass != capmon.ExitPaused {
		t.Errorf("expected ExitPaused (%d), got %d", capmon.ExitPaused, exitClass)
	}
}
