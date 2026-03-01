package loadout

import (
	"errors"
	"time"

	"github.com/OpenScribbler/nesco/cli/internal/snapshot"
)

// staleThreshold is how old a --try snapshot can be before we consider it stale.
// 24 hours is a simple heuristic: any --try snapshot older than this almost certainly
// didn't auto-revert via the SessionEnd hook (e.g., user killed the process, or the
// hook failed silently).
const staleThreshold = 24 * time.Hour

// StaleInfo describes a stale --try snapshot that was not cleaned up.
type StaleInfo struct {
	LoadoutName string
	CreatedAt   time.Time
}

// CheckStaleSnapshot returns stale info if a --try snapshot exists that hasn't
// been cleaned up. "Stale" means: mode is "try" AND snapshot age exceeds 24 hours.
//
// Returns nil if:
//   - No snapshot exists
//   - The snapshot is in "keep" mode (permanent, not expected to auto-revert)
//   - The snapshot is recent (less than 24 hours old -- still within expected session lifetime)
func CheckStaleSnapshot(projectRoot string) (*StaleInfo, error) {
	manifest, _, err := snapshot.Load(projectRoot)
	if errors.Is(err, snapshot.ErrNoSnapshot) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Only "try" mode snapshots can be stale
	if manifest.Mode != "try" {
		return nil, nil
	}

	// Check age
	age := time.Since(manifest.CreatedAt)
	if age < staleThreshold {
		return nil, nil
	}

	return &StaleInfo{
		LoadoutName: manifest.LoadoutName,
		CreatedAt:   manifest.CreatedAt,
	}, nil
}
