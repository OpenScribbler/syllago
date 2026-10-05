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
