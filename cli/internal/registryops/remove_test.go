package registryops

import (
	"errors"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/config"
)

// stubRemoveFn swaps the orchestrator's clone-deletion seam for the duration
// of the test. fn receives the registry name and returns the error to
// surface to the orchestrator.
func stubRemoveFn(t *testing.T, fn func(name string) error) {
	t.Helper()
	orig := RemoveFn
	RemoveFn = fn
	t.Cleanup(func() { RemoveFn = orig })
}

// withGlobalDir points config.LoadGlobal/SaveGlobal at a tempdir and seeds
// it with the given registries. Returns nothing — the side effect is the
// reset cleanup.
func withGlobalDir(t *testing.T, regs []config.Registry) {
	t.Helper()
	dir := t.TempDir()
	orig := config.GlobalDirOverride
	config.GlobalDirOverride = dir
	t.Cleanup(func() { config.GlobalDirOverride = orig })

	if len(regs) > 0 {
		if err := config.SaveGlobal(&config.Config{Registries: regs}); err != nil {
			t.Fatalf("seed config.SaveGlobal: %v", err)
		}
	}
}

func TestRemoveRegistry_PrunesNamedEntry(t *testing.T) {
	withGlobalDir(t, []config.Registry{
		{Name: "keep", URL: "https://example.com/keep.git"},
		{Name: "drop", URL: "https://example.com/drop.git"},
	})
	stubRemoveFn(t, func(name string) error { return nil })

	out, err := RemoveRegistry("drop")
	if err != nil {
		t.Fatalf("RemoveRegistry: %v", err)
	}
	if out.Name != "drop" {
		t.Errorf("out.Name = %q, want %q", out.Name, "drop")
	}
	if out.CloneRemoveErr != nil {
		t.Errorf("CloneRemoveErr = %v, want nil", out.CloneRemoveErr)
	}

	cfg, _ := config.LoadGlobal()
	if len(cfg.Registries) != 1 || cfg.Registries[0].Name != "keep" {
		t.Errorf("post-remove registries = %+v, want [keep]", cfg.Registries)
	}
}

func TestRemoveRegistry_NotFound(t *testing.T) {
	withGlobalDir(t, []config.Registry{
		{Name: "only", URL: "https://example.com/only.git"},
	})
	stubRemoveFn(t, func(name string) error {
		t.Fatalf("RemoveFn must not be called when name not found, was called with %q", name)
		return nil
	})

	_, err := RemoveRegistry("ghost")
	if !errors.Is(err, ErrRemoveNotFound) {
		t.Fatalf("err = %v, want errors.Is(ErrRemoveNotFound)", err)
	}

	// Config must be untouched on not-found — no partial write.
	cfg, _ := config.LoadGlobal()
	if len(cfg.Registries) != 1 || cfg.Registries[0].Name != "only" {
		t.Errorf("config mutated on not-found: %+v", cfg.Registries)
	}
}

func TestRemoveRegistry_CloneFailureIsSoft(t *testing.T) {
	withGlobalDir(t, []config.Registry{
		{Name: "drop", URL: "https://example.com/drop.git"},
	})
	cloneErr := errors.New("permission denied")
	stubRemoveFn(t, func(name string) error { return cloneErr })

	out, err := RemoveRegistry("drop")
	if err != nil {
		t.Fatalf("RemoveRegistry: %v", err)
	}
	if out.CloneRemoveErr == nil || !errors.Is(out.CloneRemoveErr, cloneErr) {
		t.Errorf("CloneRemoveErr = %v, want wraps %v", out.CloneRemoveErr, cloneErr)
	}

	// Config save still happened — the registry must be gone.
	cfg, _ := config.LoadGlobal()
	if len(cfg.Registries) != 0 {
		t.Errorf("registry still in config after soft-fail clone: %+v", cfg.Registries)
	}
}
