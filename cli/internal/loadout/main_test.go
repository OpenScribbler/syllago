package loadout

import (
	"fmt"
	"os"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// TestMain points the global content dir at an empty directory, so the
// installer's check for servers placed under the legacy root never reads
// the real ~/.syllago.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "loadout-global-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	catalog.GlobalContentDirOverride = dir
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
