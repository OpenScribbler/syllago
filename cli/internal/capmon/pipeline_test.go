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
