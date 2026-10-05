package installer

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
)

func TestCheckItem(t *testing.T) {
	const (
		regName = "moat-reg"
		regURL  = "https://registry.example.com/manifest.json"
	)
	oldHash := "sha256:" + strings.Repeat("a", 64)
	newHash := "sha256:" + strings.Repeat("b", 64)
	idx := int64(7)

	libraryCopy := func(name, hash string) catalog.ContentItem {
		return catalog.ContentItem{
			Name:    name,
			Type:    catalog.Skills,
			Library: true,
			Meta:    &metadata.Meta{SourceRegistry: regName, SourceHash: hash},
		}
	}

	tests := []struct {
		name        string
		item        catalog.ContentItem
		revoked     string // hash the registry revokes, if any
		lockRevoked string // hash the lockfile revokes, if any
		minTier     moat.TrustTier
		wantOK      bool
		wantGate    MOATGateDecision
		wantHash    string
	}{
		{
			name:     "registry item is checked at the manifest's hash",
			item:     catalog.ContentItem{Name: "listed", Type: catalog.Skills, Registry: regName},
			wantOK:   true,
			wantGate: MOATGateProceed,
			wantHash: newHash,
		},
		{
			name:     "Library copy of the current version proceeds",
			item:     libraryCopy("listed", newHash),
			wantOK:   true,
			wantGate: MOATGateProceed,
			wantHash: newHash,
		},
		{
			name:     "Library copy of a revoked older version is blocked",
			item:     libraryCopy("listed", oldHash),
			revoked:  oldHash,
			wantOK:   true,
			wantGate: MOATGateHardBlock,
			wantHash: oldHash,
		},
		{
			name:     "a revocation of the newer version spares the older copy",
			item:     libraryCopy("listed", oldHash),
			revoked:  newHash,
			wantOK:   true,
			wantGate: MOATGateProceed,
			wantHash: oldHash,
		},
		{
			name:        "lockfile revocation of the copy's hash is blocked",
			item:        libraryCopy("listed", oldHash),
			lockRevoked: oldHash,
			wantOK:      true,
			wantGate:    MOATGateHardBlock,
			wantHash:    oldHash,
		},
		{
			name:     "Library copy is held to the policy floor",
			item:     libraryCopy("listed", newHash),
			minTier:  moat.TrustTierDualAttested,
			wantOK:   true,
			wantGate: MOATGateTierBelowPolicy,
			wantHash: newHash,
		},
		{
			name:     "unlisted copy with a revoked hash is blocked",
			item:     libraryCopy("delisted", oldHash),
			revoked:  oldHash,
			wantOK:   true,
			wantGate: MOATGateHardBlock,
			wantHash: oldHash,
		},
		{
			name:     "unlisted copy is checked for revocations only",
			item:     libraryCopy("delisted", oldHash),
			minTier:  moat.TrustTierDualAttested,
			wantOK:   true,
			wantGate: MOATGateProceed,
			wantHash: oldHash,
		},
		{
			name: "unlisted copy without a hash bypasses",
			item: libraryCopy("delisted", ""),
		},
		{
			name: "copy from a registry the inputs do not hold bypasses",
			item: catalog.ContentItem{Name: "listed", Library: true, Meta: &metadata.Meta{SourceRegistry: "git-reg", SourceHash: oldHash}},
		},
		{
			name: "local content bypasses",
			item: catalog.ContentItem{Name: "listed", Library: true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			manifest := &moat.Manifest{
				ManifestURI: regURL,
				Content: []moat.ContentEntry{{
					Name:          "listed",
					Type:          "skill",
					ContentHash:   newHash,
					RekorLogIndex: &idx,
				}},
			}
			if tc.revoked != "" {
				manifest.Revocations = []moat.Revocation{{
					ContentHash: tc.revoked,
					Reason:      "compromised",
					DetailsURL:  "https://example.com/recall",
					Source:      moat.RevocationSourceRegistry,
				}}
			}
			in := &moat.GateInputs{
				RevSet:       moat.NewRevocationSet(),
				Manifests:    map[string]*moat.Manifest{regName: manifest},
				ManifestURIs: map[string]string{regName: regURL},
			}
			in.RevSet.AddFromManifest(manifest, regURL)
			lf := moat.NewLockfile()
			if tc.lockRevoked != "" {
				lf.RevokedHashes = append(lf.RevokedHashes, tc.lockRevoked)
			}
			minTier := tc.minTier
			if minTier == 0 {
				minTier = moat.TrustTierUnsigned
			}

			got, ok := CheckItem(tc.item, in, lf, moat.NewSession(), minTier)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if got.Decision != tc.wantGate {
				t.Errorf("decision = %v, want %v", got.Decision, tc.wantGate)
			}
			if got.Entry.ContentHash != tc.wantHash {
				t.Errorf("checked hash = %s, want %s", got.Entry.ContentHash, tc.wantHash)
			}
			if got.RegistryURL != regURL {
				t.Errorf("RegistryURL = %q, want %q", got.RegistryURL, regURL)
			}
		})
	}
}

func TestCheckItem_NilInputsBypass(t *testing.T) {
	item := catalog.ContentItem{Name: "listed", Registry: "moat-reg"}
	if _, ok := CheckItem(item, nil, nil, nil, moat.TrustTierUnsigned); ok {
		t.Fatal("CheckItem with nil inputs: ok = true, want false")
	}
}

// A manifest may list a skill and a rule under one name; a copy is
// checked against the entry of its own type.
func TestCheckItem_MatchesEntryOfItsType(t *testing.T) {
	const regName, regURL = "moat-reg", "https://registry.example.com/manifest.json"
	skillHash := "sha256:" + strings.Repeat("a", 64)
	ruleHash := "sha256:" + strings.Repeat("b", 64)
	manifest := &moat.Manifest{ManifestURI: regURL, Content: []moat.ContentEntry{
		{Name: "shared", Type: "skill", ContentHash: skillHash},
		{Name: "shared", Type: "rules", ContentHash: ruleHash, PrivateRepo: true},
	}}
	in := &moat.GateInputs{
		RevSet:       moat.NewRevocationSet(),
		Manifests:    map[string]*moat.Manifest{regName: manifest},
		ManifestURIs: map[string]string{regName: regURL},
	}
	item := catalog.ContentItem{
		Name:    "shared",
		Type:    catalog.Rules,
		Library: true,
		Meta:    &metadata.Meta{SourceRegistry: regName, SourceHash: ruleHash},
	}

	got, ok := CheckItem(item, in, moat.NewLockfile(), moat.NewSession(), moat.TrustTierUnsigned)
	if !ok {
		t.Fatal("ok = false, want the rule's entry")
	}
	if got.Decision != MOATGatePrivatePrompt {
		t.Errorf("decision = %v, want %v from the private rule entry", got.Decision, MOATGatePrivatePrompt)
	}
}

// A copy whose registry has no cached manifest is still refused when the
// lockfile has archived a revocation of its hash.
func TestCheckItem_LockfileRevocationWithoutManifest(t *testing.T) {
	hash := "sha256:" + strings.Repeat("a", 64)
	item := catalog.ContentItem{
		Name:    "copy",
		Type:    catalog.Skills,
		Library: true,
		Meta:    &metadata.Meta{SourceRegistry: "uncached-reg", SourceHash: hash},
	}
	lf := moat.NewLockfile()
	lf.RevokedHashes = append(lf.RevokedHashes, hash)

	for name, in := range map[string]*moat.GateInputs{
		"no inputs":       nil,
		"registry absent": {RevSet: moat.NewRevocationSet(), Manifests: map[string]*moat.Manifest{}, ManifestURIs: map[string]string{}},
	} {
		t.Run(name, func(t *testing.T) {
			got, ok := CheckItem(item, in, lf, moat.NewSession(), moat.TrustTierUnsigned)
			if !ok || got.Decision != MOATGateHardBlock {
				t.Errorf("ok = %v, decision = %v; want a hard block", ok, got.Decision)
			}
		})
	}

	if _, ok := CheckItem(item, nil, moat.NewLockfile(), moat.NewSession(), moat.TrustTierUnsigned); ok {
		t.Error("an unrevoked copy without a cached manifest was gated; want it to bypass")
	}
}

// An older copy is not credited with the attestation of the version its
// registry lists now.
func TestCheckItem_OlderCopyHasNoBorrowedAttestation(t *testing.T) {
	const regName, regURL = "moat-reg", "https://registry.example.com/manifest.json"
	idx := int64(7)
	manifest := &moat.Manifest{ManifestURI: regURL, Content: []moat.ContentEntry{{
		Name:          "listed",
		Type:          "skill",
		ContentHash:   "sha256:" + strings.Repeat("b", 64),
		RekorLogIndex: &idx,
	}}}
	in := &moat.GateInputs{
		RevSet:       moat.NewRevocationSet(),
		Manifests:    map[string]*moat.Manifest{regName: manifest},
		ManifestURIs: map[string]string{regName: regURL},
	}
	item := catalog.ContentItem{
		Name:    "listed",
		Type:    catalog.Skills,
		Library: true,
		Meta:    &metadata.Meta{SourceRegistry: regName, SourceHash: "sha256:" + strings.Repeat("a", 64)},
	}

	got, ok := CheckItem(item, in, moat.NewLockfile(), moat.NewSession(), moat.TrustTierUnsigned)
	if !ok {
		t.Fatal("ok = false, want the older copy gated")
	}
	if tier := got.Entry.TrustTier(); tier != moat.TrustTierUnsigned {
		t.Errorf("older copy's tier = %v, want %v", tier, moat.TrustTierUnsigned)
	}
}
