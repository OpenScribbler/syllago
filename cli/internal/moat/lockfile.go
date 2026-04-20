package moat

// MOAT lockfile (spec §Lockfile, v0.6.0) — `.syllago/moat-lockfile.json`.
//
// The lockfile is a per-project trust ledger recording the full attestation
// bundle and signed payload for every installed MOAT-sourced content item,
// plus the revocation hard-block list and per-registry fetch timestamps
// used for 72-hour staleness enforcement (G-9). It is distinct from
// `installed.json` (mechanism tracking for uninstall) and intentionally so —
// they serve different audiences: uninstall logic reads `installed.json`;
// `moat-verify` offline mode reads the lockfile.
//
// Interoperability is normative: a lockfile written by syllago MUST be
// readable by any other conforming client, and vice versa. This file
// follows the exact field names, casing, and shape documented in
// `moat-spec.md` §Lockfile. Do not add Syllago-only keys to this top-level
// structure — the MAY-extend allowance in the spec covers `entries[]`
// sub-fields but not the core shape.
//
// Pre-write verification (spec §Lockfile field notes):
//
//	Before an entry is added, `sha256(signed_payload)` MUST equal the
//	`data.hash.value` field of the Rekor entry at `rekor_log_index`. If
//	this check fails, the entry MUST NOT be written and the install
//	MUST be aborted. AddEntry performs this check — callers who bypass
//	AddEntry are violating the spec.
//
// Upgrade path (spec §Lockfile):
//
//	A lockfile written before v0.6.0 has no `registries` key. Load
//	initializes an empty map in that case so callers can write the
//	`fetched_at` timestamp on the next successful manifest fetch
//	without the lockfile going through an explicit migration.
//
// See ADR 0007 G-7.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// LockfileSchemaVersion is the only `moat_lockfile_version` value this
// client accepts. Unknown versions are rejected — a future bump will add
// explicit grace-period handling similar to attestation payload versioning
// (see G-14).
const LockfileSchemaVersion = 1

// LockfileRelPath is the lockfile's location relative to a project root.
// The file sits next to `installed.json` in `.syllago/` so both sources of
// truth travel with the project.
const LockfileRelPath = ".syllago/moat-lockfile.json"

// Trust tier label strings used in lockfile entries[].trust_tier. These
// match the spec's closed set exactly — do NOT reuse catalog.TrustTier's
// labels ("Dual-Attested") which are presentation-only. The wire format
// uses uppercase with hyphens.
const (
	LockTrustTierDualAttested = "DUAL-ATTESTED"
	LockTrustTierSigned       = "SIGNED"
	LockTrustTierUnsigned     = "UNSIGNED"
)

// TrustTierLabel maps a moat.TrustTier (computed from manifest fields) to
// the normative lockfile label. Used by the install flow when building a
// LockEntry so there is one place where the wire label is derived.
func TrustTierLabel(t TrustTier) string {
	switch t {
	case TrustTierDualAttested:
		return LockTrustTierDualAttested
	case TrustTierSigned:
		return LockTrustTierSigned
	case TrustTierUnsigned:
		return LockTrustTierUnsigned
	}
	return ""
}

// RegistryLockState is the per-registry state held in lockfile.registries.
// Keyed at the top level by manifest URL; the lockfile never stores the
// URL inside the struct because the map key is authoritative.
type RegistryLockState struct {
	// FetchedAt is the client's last successful manifest fetch for this
	// registry (RFC 3339 UTC). Used for 72-hour staleness enforcement per
	// G-9. A failed fetch MUST NOT update this field — the clock runs
	// from the last *successful* fetch only.
	FetchedAt time.Time `json:"fetched_at"`
}

// LockEntry is one row in lockfile.entries[]. Field names and JSON tags
// are load-bearing for cross-client interoperability — see spec §Lockfile.
type LockEntry struct {
	Name        string    `json:"name"`
	Type        string    `json:"type"` // skill|agent|rules|command
	Registry    string    `json:"registry"`
	ContentHash string    `json:"content_hash"`
	TrustTier   string    `json:"trust_tier"` // DUAL-ATTESTED|SIGNED|UNSIGNED
	AttestedAt  time.Time `json:"attested_at"`
	PinnedAt    time.Time `json:"pinned_at"`

	// AttestationBundle holds the full cosign bundle as captured at install
	// time. json.RawMessage preserves the exact bytes — re-marshaling would
	// reorder keys and break cosign's offline verification. For UNSIGNED
	// entries this is the four-byte literal `null` (see nullRaw below).
	AttestationBundle json.RawMessage `json:"attestation_bundle"`

	// SignedPayload is the verbatim bytes passed to `cosign sign-blob` at
	// attestation time, stored as a string (not a decoded JSON object) so
	// that cosign verify-blob --offline receives the exact bytes the
	// signature covers. Nil means "UNSIGNED entry" and serializes to
	// JSON null via the *string pointer.
	SignedPayload *string `json:"signed_payload"`
}

// nullRaw is the JSON literal `null` used as the default for
// AttestationBundle on UNSIGNED entries. Using a package-level value
// avoids re-allocating the same 4 bytes for every UNSIGNED entry.
var nullRaw = json.RawMessage("null")

// NullAttestationBundle returns the RawMessage representing JSON null —
// the value to use when constructing an UNSIGNED LockEntry.
func NullAttestationBundle() json.RawMessage { return nullRaw }

// Lockfile is the full on-disk structure. Load/Save handle the file I/O;
// AddEntry is the only mutation path for entries[] because it enforces
// the pre-write hash invariant.
type Lockfile struct {
	Version       int                          `json:"moat_lockfile_version"`
	Registries    map[string]RegistryLockState `json:"registries"`
	Entries       []LockEntry                  `json:"entries"`
	RevokedHashes []string                     `json:"revoked_hashes"`
}

// NewLockfile returns a freshly initialized lockfile at the current schema
// version with non-nil slices and map. Non-nil zero values matter: JSON
// marshalling a nil slice yields `null`, but the spec requires `[]` —
// "`revoked_hashes` (REQUIRED) Array of hard-blocked content hash strings;
// empty array if none."
func NewLockfile() *Lockfile {
	return &Lockfile{
		Version:       LockfileSchemaVersion,
		Registries:    map[string]RegistryLockState{},
		Entries:       []LockEntry{},
		RevokedHashes: []string{},
	}
}

// LockfilePath returns the absolute lockfile path under the given project
// root. The caller is responsible for ensuring projectRoot exists.
func LockfilePath(projectRoot string) string {
	return filepath.Join(projectRoot, LockfileRelPath)
}

// LoadLockfile reads and parses a lockfile from disk. A missing file is
// NOT an error — returns a fresh NewLockfile() so callers can write to it
// directly on first install. This is the expected path for new projects.
//
// Non-missing I/O errors and malformed JSON return an error — the caller
// must not silently fall back to an empty lockfile on corruption because
// that would erase prior `revoked_hashes` entries, which the spec forbids.
func LoadLockfile(path string) (*Lockfile, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NewLockfile(), nil
		}
		return nil, fmt.Errorf("opening lockfile %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("reading lockfile %s: %w", path, err)
	}
	return ParseLockfile(data)
}

// ParseLockfile decodes lockfile bytes and applies v0.6.0 upgrade
// normalization (missing `registries` key gets initialized to an empty
// map). Exposed separately from LoadLockfile so tests can round-trip
// lockfile bytes without touching the filesystem.
func ParseLockfile(data []byte) (*Lockfile, error) {
	var lf Lockfile
	if err := json.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("lockfile json: %w", err)
	}

	if lf.Version == 0 {
		return nil, errors.New("lockfile missing required field: moat_lockfile_version")
	}
	if lf.Version != LockfileSchemaVersion {
		return nil, fmt.Errorf("unsupported moat_lockfile_version %d: only %d is supported",
			lf.Version, LockfileSchemaVersion)
	}

	// v0.6.0 upgrade path: pre-0.6.0 lockfiles lack the `registries` key.
	// Initialize it so the next successful fetch writes `fetched_at`
	// without needing an explicit migration step. Spec §Lockfile: "If a
	// conforming client reads a lockfile without the `registries` key
	// (upgrade from a pre-staleness lockfile), it SHOULD initialize the
	// key and set `fetched_at` to the current time on the next successful
	// manifest fetch."
	if lf.Registries == nil {
		lf.Registries = map[string]RegistryLockState{}
	}

	if lf.Entries == nil {
		lf.Entries = []LockEntry{}
	}
	if lf.RevokedHashes == nil {
		lf.RevokedHashes = []string{}
	}

	return &lf, nil
}

// Save atomically writes the lockfile to disk. Writes to a sibling temp
// file then renames — this avoids leaving a half-written lockfile if the
// process dies mid-write (which would silently drop revoked_hashes
// entries, a spec violation).
//
// The parent directory is created if missing (mode 0755) so callers
// don't need to pre-create `.syllago/`.
func (l *Lockfile) Save(path string) error {
	if l.Version == 0 {
		l.Version = LockfileSchemaVersion
	}
	if l.Registries == nil {
		l.Registries = map[string]RegistryLockState{}
	}
	if l.Entries == nil {
		l.Entries = []LockEntry{}
	}
	if l.RevokedHashes == nil {
		l.RevokedHashes = []string{}
	}

	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling lockfile: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating lockfile dir: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "moat-lockfile-*.json.tmp")
	if err != nil {
		return fmt.Errorf("creating temp lockfile: %w", err)
	}
	tmpPath := tmp.Name()
	// On any failure after CreateTemp, best-effort remove the temp file.
	// Rename on success makes the temp path gone, so a post-success
	// Remove is a no-op.
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing temp lockfile: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("syncing temp lockfile: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp lockfile: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming lockfile into place: %w", err)
	}
	return nil
}

// SetRegistryFetchedAt records a successful manifest fetch for the given
// registry URL. Callers MUST only invoke this after verification succeeds
// — the staleness clock runs from the last *successful* fetch and must
// not be reset by failed attempts (spec §Freshness Guarantee).
func (l *Lockfile) SetRegistryFetchedAt(registryURL string, t time.Time) {
	if l.Registries == nil {
		l.Registries = map[string]RegistryLockState{}
	}
	l.Registries[registryURL] = RegistryLockState{FetchedAt: t.UTC()}
}

// IsRevoked reports whether a content hash is in the hard-block list.
func (l *Lockfile) IsRevoked(contentHash string) bool {
	for _, h := range l.RevokedHashes {
		if h == contentHash {
			return true
		}
	}
	return false
}

// AddRevokedHash appends a hash to revoked_hashes if not already present.
// De-duplication is safe because the spec says "entries MUST NOT be
// silently removed" — this function never removes, and adding a duplicate
// would bloat the file without changing semantics.
func (l *Lockfile) AddRevokedHash(contentHash string) {
	if l.IsRevoked(contentHash) {
		return
	}
	l.RevokedHashes = append(l.RevokedHashes, contentHash)
}

// ErrSignedPayloadHashMismatch is returned when AddEntry's pre-write check
// detects that sha256(signed_payload) does not match the expected Rekor
// data.hash.value. Callers MUST propagate this upward and abort the
// install — silent fall-through is a spec violation.
var ErrSignedPayloadHashMismatch = errors.New("signed_payload hash does not match Rekor data.hash.value")

// AddEntry validates and appends a LockEntry to entries[].
//
// For SIGNED and DUAL-ATTESTED entries, the caller MUST supply
// `expectedDataHashValue` — the Rekor entry's `spec.data.hash.value`
// field at rekor_log_index. AddEntry verifies:
//
//	sha256(entry.SignedPayload) == expectedDataHashValue (hex)
//
// If the hash does not match, the entry is NOT appended and
// ErrSignedPayloadHashMismatch is returned. This enforces the
// §Lockfile requirement: "Before storing, conforming clients MUST
// confirm that sha256(signed_payload.encode('utf-8')) equals the
// data.hash.value field of the Rekor entry at rekor_log_index. If
// this check fails, the entry MUST NOT be written to the lockfile
// and the install MUST be aborted."
//
// For UNSIGNED entries (trust_tier=UNSIGNED, signed_payload=nil,
// attestation_bundle=null), expectedDataHashValue is ignored — there
// is no Rekor entry to verify against.
//
// Side effect on success: the caller should Save() the lockfile; this
// function does not write to disk.
func (l *Lockfile) AddEntry(entry LockEntry, expectedDataHashValue string) error {
	switch entry.TrustTier {
	case LockTrustTierSigned, LockTrustTierDualAttested:
		if entry.SignedPayload == nil {
			return fmt.Errorf("trust_tier=%s requires signed_payload", entry.TrustTier)
		}
		if len(entry.AttestationBundle) == 0 || jsonIsNull(entry.AttestationBundle) {
			return fmt.Errorf("trust_tier=%s requires attestation_bundle", entry.TrustTier)
		}
		if expectedDataHashValue == "" {
			return errors.New("expectedDataHashValue required for SIGNED/DUAL-ATTESTED entries")
		}
		digest := sha256.Sum256([]byte(*entry.SignedPayload))
		got := hex.EncodeToString(digest[:])
		if got != expectedDataHashValue {
			return fmt.Errorf("%w: sha256=%s rekor=%s", ErrSignedPayloadHashMismatch, got, expectedDataHashValue)
		}
	case LockTrustTierUnsigned:
		// UNSIGNED: bundle and payload MUST be JSON null. Accept a nil
		// AttestationBundle RawMessage as equivalent to null.
		if entry.SignedPayload != nil {
			return errors.New("UNSIGNED entry must have nil signed_payload")
		}
		if len(entry.AttestationBundle) > 0 && !jsonIsNull(entry.AttestationBundle) {
			return errors.New("UNSIGNED entry must have null attestation_bundle")
		}
		if len(entry.AttestationBundle) == 0 {
			entry.AttestationBundle = nullRaw
		}
	default:
		return fmt.Errorf("unknown trust_tier %q: expected %s, %s, or %s",
			entry.TrustTier, LockTrustTierDualAttested, LockTrustTierSigned, LockTrustTierUnsigned)
	}

	if entry.Name == "" || entry.Type == "" || entry.Registry == "" || entry.ContentHash == "" {
		return errors.New("lock entry missing required field: name, type, registry, or content_hash")
	}

	l.Entries = append(l.Entries, entry)
	return nil
}

// jsonIsNull reports whether raw holds the JSON literal `null`. Whitespace
// around the literal (e.g. " null ") is accepted because json.RawMessage
// preserves exact bytes and some serializers include spacing.
func jsonIsNull(raw json.RawMessage) bool {
	for _, b := range raw {
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			continue
		}
		// First non-whitespace byte must be 'n' for "null"; anything else
		// is not null.
		return b == 'n' && string(trimJSONWS(raw)) == "null"
	}
	return false
}

// trimJSONWS strips surrounding JSON whitespace (space/tab/newline/CR)
// from a RawMessage. Used only by jsonIsNull.
func trimJSONWS(raw json.RawMessage) []byte {
	start := 0
	end := len(raw)
	for start < end {
		b := raw[start]
		if b != ' ' && b != '\t' && b != '\n' && b != '\r' {
			break
		}
		start++
	}
	for end > start {
		b := raw[end-1]
		if b != ' ' && b != '\t' && b != '\n' && b != '\r' {
			break
		}
		end--
	}
	return raw[start:end]
}
