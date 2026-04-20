package moat

// Operator-provided trusted root loader per ADR 0007 slice 2d.
//
// Enterprise/air-gapped deployments may pin a specific Sigstore trusted_root.json
// per registry (via config.Registry.TrustedRoot) or per invocation (via the
// --trusted-root CLI flag). This is the loading path for that override.
//
// The override is NOT staleness-checked: the operator has taken responsibility
// for refreshing the root, and the bundled 90/180/365-day cliff encodes
// staleness for the specific bundled root that ships with the binary — not
// for an external file whose provenance syllago does not control. A fresh
// override is always treated as Status=Fresh; load failures become errors,
// not Missing/Corrupt status codes, because silent fall-back to the bundled
// root would be a trust downgrade the operator did not authorize.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

// TrustedRootFromPath loads a Sigstore trusted_root.json from a filesystem
// path and returns a TrustedRootInfo with Source=path. Returns an error when
// the file is unreadable or not valid JSON.
//
// The 'now' parameter is accepted for signature symmetry with
// BundledTrustedRoot, but staleness is intentionally not computed — override
// roots are the operator's responsibility.
func TrustedRootFromPath(path string, now time.Time) (TrustedRootInfo, error) {
	if path == "" {
		return TrustedRootInfo{}, errors.New("trusted-root override path is empty")
	}

	bytes, readErr := os.ReadFile(path)
	if readErr != nil {
		return TrustedRootInfo{}, fmt.Errorf("reading trusted-root override %s: %w", path, readErr)
	}

	// Minimal shape check: must be valid JSON. Full schema validation is
	// deferred to sigstore-go at verification time — enforcing it here
	// would duplicate that logic and brittle against upstream schema
	// additions.
	var probe map[string]any
	if unmarshalErr := json.Unmarshal(bytes, &probe); unmarshalErr != nil {
		return TrustedRootInfo{}, fmt.Errorf("parsing trusted-root override %s: %w", path, unmarshalErr)
	}

	return TrustedRootInfo{
		Source:    TrustedRootSourcePathFlag,
		IssuedAt:  time.Time{},
		AgeDays:   0,
		CliffDate: time.Time{},
		Status:    TrustedRootStatusFresh,
		Bytes:     bytes,
	}, nil
}
