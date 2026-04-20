// Package moat — revocation enforcement.
//
// MOAT registry manifests carry a revocations[] list. Revocation handling is a
// two-tier contract (ADR 0007 G-8, spec §Revocation Mechanism):
//
//   - source=registry  → HARD-BLOCK. Refuse to install or load. Non-zero exit.
//   - source=publisher → WARN-ONCE-PER-SESSION. Require explicit confirmation
//     (interactive Y/n; non-interactive exits 1). Once confirmed for a given
//     (issuing-registry, content_hash) pair, suppress further warnings for
//     that pair until the session ends.
//
// This module provides the primitives. Wiring — registry sync, install flow,
// CheckStatus integration — lands in the Phase 2 merge (syllago-dsqjz).
//
// Forward-compat: unknown `reason` values are carried through verbatim. The
// closed-set validation of reason strings lives in manifest parsing (see
// manifest.go); this layer never re-validates. A future registry publishing a
// new reason string still flows through the enforcement path.
package moat

import "sort"

// RevocationStatus is the effective enforcement decision for a content hash.
type RevocationStatus int

const (
	// RevStatusNone means the hash is not revoked by any known manifest.
	RevStatusNone RevocationStatus = iota

	// RevStatusRegistryBlock means a registry-source revocation applies —
	// the caller MUST refuse to install or load the content and exit non-zero.
	RevStatusRegistryBlock

	// RevStatusPublisherWarn means a publisher-source revocation applies —
	// the caller MUST warn the user and require explicit confirmation on the
	// first occurrence per session. Non-interactive callers MUST exit non-zero.
	RevStatusPublisherWarn
)

// String returns a short human label for diagnostics.
func (s RevocationStatus) String() string {
	switch s {
	case RevStatusNone:
		return "none"
	case RevStatusRegistryBlock:
		return "registry-block"
	case RevStatusPublisherWarn:
		return "publisher-warn"
	default:
		return "unknown"
	}
}

// RevocationRecord is a single revocation observation enriched with the
// issuing-registry URL (so UIs can surface where the revocation came from) and
// the computed enforcement Status.
//
// Reason is carried verbatim from the manifest — callers MUST treat unknown
// values as opaque and present them to the user as-is. Do not re-validate here.
type RevocationRecord struct {
	ContentHash        string
	Reason             string
	DetailsURL         string
	Source             string // "registry" or "publisher" (after EffectiveSource())
	IssuingRegistryURL string // URL of the manifest's home registry
	Status             RevocationStatus
}

// RevocationSet is an index of revocations keyed by content_hash, aggregated
// across one or more registry manifests. A single hash may map to multiple
// records if more than one manifest revokes it.
type RevocationSet struct {
	byHash map[string][]RevocationRecord
}

// NewRevocationSet creates an empty set.
func NewRevocationSet() *RevocationSet {
	return &RevocationSet{byHash: make(map[string][]RevocationRecord)}
}

// AddFromManifest indexes every revocation in m under its content_hash.
// registryURL is the URL the manifest was fetched from — surfaced as
// `IssuingRegistryURL` on each resulting record. A nil manifest is a no-op.
//
// Source classification uses Revocation.EffectiveSource() so an absent `source`
// field defaults to "registry" per spec. Unknown source values fall through to
// RevStatusRegistryBlock as the safer default.
func (s *RevocationSet) AddFromManifest(m *Manifest, registryURL string) {
	if s == nil || m == nil {
		return
	}
	for i := range m.Revocations {
		r := &m.Revocations[i]
		src := r.EffectiveSource()
		var status RevocationStatus
		switch src {
		case RevocationSourcePublisher:
			status = RevStatusPublisherWarn
		case RevocationSourceRegistry:
			status = RevStatusRegistryBlock
		default:
			status = RevStatusRegistryBlock
		}
		rec := RevocationRecord{
			ContentHash:        r.ContentHash,
			Reason:             r.Reason,
			DetailsURL:         r.DetailsURL,
			Source:             src,
			IssuingRegistryURL: registryURL,
			Status:             status,
		}
		s.byHash[r.ContentHash] = append(s.byHash[r.ContentHash], rec)
	}
}

// Lookup returns every record matching contentHash (nil if none).
func (s *RevocationSet) Lookup(contentHash string) []RevocationRecord {
	if s == nil {
		return nil
	}
	return s.byHash[contentHash]
}

// Len reports the number of distinct revoked content hashes.
func (s *RevocationSet) Len() int {
	if s == nil {
		return 0
	}
	return len(s.byHash)
}

// Session tracks per-session suppression state for publisher-source
// revocations. Once the user confirms acceptance of a publisher revocation
// for (issuing-registry, content_hash), subsequent checks for the same pair
// must not re-prompt in the same session.
//
// Registry-source revocations never interact with Session — they always
// block.
type Session struct {
	confirmed map[string]struct{}
}

// NewSession creates a fresh session tracker.
func NewSession() *Session {
	return &Session{confirmed: make(map[string]struct{})}
}

// ShouldWarn reports whether the caller should emit a warning + confirmation
// prompt for a publisher revocation. Returns true on first call; false after
// MarkConfirmed for the same (registry, hash) pair.
//
// A nil Session always returns true (no suppression).
func (sess *Session) ShouldWarn(issuingRegistryURL, contentHash string) bool {
	if sess == nil {
		return true
	}
	_, ok := sess.confirmed[sessionKey(issuingRegistryURL, contentHash)]
	return !ok
}

// MarkConfirmed records that the user has acknowledged the publisher
// revocation for (registry, hash). Safe to call multiple times.
func (sess *Session) MarkConfirmed(issuingRegistryURL, contentHash string) {
	if sess == nil {
		return
	}
	sess.confirmed[sessionKey(issuingRegistryURL, contentHash)] = struct{}{}
}

func sessionKey(registry, hash string) string {
	return registry + "|" + hash
}

// CheckLockfile enumerates every (lockfile entry, revocation record) pair
// where the entry's content_hash is revoked. The result is deterministic:
// sorted by (issuing-registry, content_hash, source). Duplicate entries
// within the same (registry, hash, source) are collapsed.
//
// Callers use this at registry sync time to decide what to block or warn
// about. A nil lockfile or nil set yields nil.
func CheckLockfile(lf *Lockfile, set *RevocationSet) []RevocationRecord {
	if lf == nil || set == nil {
		return nil
	}
	var out []RevocationRecord
	seen := make(map[string]bool)
	for _, entry := range lf.Entries {
		for _, r := range set.Lookup(entry.ContentHash) {
			k := r.IssuingRegistryURL + "|" + r.ContentHash + "|" + r.Source
			if seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IssuingRegistryURL != out[j].IssuingRegistryURL {
			return out[i].IssuingRegistryURL < out[j].IssuingRegistryURL
		}
		if out[i].ContentHash != out[j].ContentHash {
			return out[i].ContentHash < out[j].ContentHash
		}
		return out[i].Source < out[j].Source
	})
	return out
}
