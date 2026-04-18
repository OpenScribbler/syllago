package registry

// RegistryClient is the abstraction for reading from a remote content
// registry. Every registry operation goes through this interface so git vs
// MOAT conditionals never spread across the codebase (AD-6, Panel C8).
//
// Phase 1 ships with GitClient only. Phase 2 adds MOATClient (manifest
// fetch + signature verify + hash-checked content fetch). Both implementations
// live in this package; callers construct via Open.
//
// Design note: lifecycle operations (Clone to create a local cache, Remove
// to delete it) stay as package-level functions. They are git-specific today
// and the MOAT equivalent — fetching and verifying a manifest — is a very
// different operation. Putting them on this interface would force one side
// to stub out nonsense. Sync, Items, FetchContent, Type, Trust are the
// operations that meaningfully have two implementations.

import (
	"context"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// RegistryClient abstracts access to a content registry. Implementations
// must be safe for concurrent use by multiple goroutines.
type RegistryClient interface {
	// Sync brings the local view of the registry up-to-date with the remote.
	// Git: runs git pull --ff-only in the clone directory.
	// MOAT: conditional-GET the manifest with ETag, verify signature,
	// update the cached manifest+ETag on success.
	//
	// Must honor ctx cancellation — long-running network calls MUST exit
	// promptly when the caller cancels.
	Sync(ctx context.Context) error

	// Items returns every content item currently visible in this registry.
	// Git: scans the clone directory for registry.yaml or content-type dirs.
	// MOAT: returns one item per manifest.content[] entry.
	//
	// This must be cheap and pure — no network I/O, no disk writes. The
	// underlying cache is populated by Sync.
	Items() []catalog.ContentItem

	// FetchContent materializes the bytes for the given item at dest.
	// Git: copies from the already-local clone directory.
	// MOAT: downloads item.source_uri and verifies the SHA matches
	// content_hash from the manifest — a mismatch is a HARD failure.
	//
	// dest is an absolute path to a directory that will contain the
	// content files. The directory must not exist or must be empty.
	FetchContent(ctx context.Context, item catalog.ContentItem, dest string) error

	// Type returns the backend identifier. Stable values: "git", "moat".
	// Callers use this for display and telemetry only — behavioral
	// conditionals belong inside the implementations, not at call sites.
	Type() string

	// Trust returns verification metadata for this registry. Returns nil
	// when the backend performs no cryptographic verification. GitClient
	// always returns nil; MOATClient returns a populated struct after
	// Sync has succeeded at least once.
	Trust() *TrustMetadata
}

// TrustMetadata describes verification state for a MOAT-backed registry.
// Git registries return nil from RegistryClient.Trust — absence of signal
// is the correct representation per C9 (UI collapses to "Unverified" without
// a badge, avoiding false negative connotations).
type TrustMetadata struct {
	// SigningProfile identifies the issuer+subject pair that signed the
	// registry manifest. Must match the registry_signing_profile the user
	// approved at registry-add time (TOFU), or sync fails closed.
	SigningProfile SigningProfile

	// LastVerifiedAt records when signature verification last succeeded.
	// Used by the freshness check (spec §Staleness) — stale beyond the
	// configured threshold requires a fresh Sync before install.
	LastVerifiedAt time.Time

	// ManifestETag is the ETag from the most recent successful fetch.
	// Passed as If-None-Match on the next Sync to let the server return
	// 304 Not Modified when nothing has changed (C7 bandwidth saver).
	ManifestETag string
}

// SigningProfile is the issuer/subject tuple identifying a signer.
// Parallel shape to moat.SigningProfile — kept here so the registry
// interface does not force a registry→moat import in Phase 1 (only
// GitClient exists; nothing pulls in moat types yet). Phase 2 reconciles
// the two types once MOATClient is written.
type SigningProfile struct {
	Issuer  string
	Subject string
}

// Type identifiers returned by RegistryClient.Type. Use these constants
// rather than string literals so typos fail at compile time.
const (
	TypeGit  = "git"
	TypeMOAT = "moat"
)

// Open returns a RegistryClient for the named registry.
//
// Phase 1: every registry is git-backed, so Open always returns a GitClient
// pointed at the clone directory. The name must correspond to a registry
// that has already been added via Clone; Open does not create new clones.
//
// Phase 2: this factory will dispatch on the registry type recorded in
// config.Registry (git vs moat) and return the appropriate client. The
// signature will gain a type parameter; existing callers break at that
// point by design — the point of this interface is to surface the choice.
func Open(name string) (RegistryClient, error) {
	dir, err := CloneDir(name)
	if err != nil {
		return nil, err
	}
	return NewGitClient(name, dir), nil
}
