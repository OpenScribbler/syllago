package main

import (
	"github.com/OpenScribbler/syllago/cli/internal/acif"
)

// Falling back to the vendored matrix is normal operation per
// [ACIF-INSTALL] §12 (refresh-over-vendored), so a failed refresh is
// deliberately silent — offline installs must not warn on every run.
var refreshInstallEntryPointsForInstall = func() {
	_ = acif.RefreshInstallEntryPoints(nil)
}
