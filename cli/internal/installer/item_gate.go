package installer

import (
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
)

// ItemGate is the trust gate's answer for one catalog item, with the
// registry URL and entry it was checked against. Entry carries the hash
// the item holds, so a confirmation recorded against Entry.ContentHash
// covers exactly the content that will be installed.
type ItemGate struct {
	GateBlock
	Entry       *moat.ContentEntry
	RegistryURL string
}

// CheckItem runs PreInstallCheck for an item that came from a MOAT
// registry, reading the registry's cached manifest from in. A registry
// item names its registry in Registry. A Library copy names it in its
// metadata, with the hash it was staged at, and is checked at that hash:
// a revocation of the version the Library holds blocks it even after the
// registry has published a newer one.
//
// A copy whose registry no longer lists it is checked for revocations
// only, since no entry remains to give it a trust tier or a privacy
// declaration.
//
// ok is false for an item no MOAT registry in in accounts for, which
// installs ungated.
func CheckItem(item catalog.ContentItem, in *moat.GateInputs, lf *moat.Lockfile, session *moat.Session, minTier moat.TrustTier) (ItemGate, bool) {
	if in == nil {
		return ItemGate{}, false
	}
	regName, hash := item.Registry, ""
	if regName == "" && item.Meta != nil {
		regName, hash = item.Meta.SourceRegistry, item.Meta.SourceHash
	}
	if regName == "" || !in.HasRegistry(regName) {
		return ItemGate{}, false
	}
	registryURL := in.ManifestURIs[regName]

	listed, ok := moat.FindContentEntry(in.Manifests[regName], item.Name)
	var entry moat.ContentEntry
	switch {
	case ok:
		entry = *listed
		if hash != "" {
			entry.ContentHash = hash
		}
	case hash != "":
		entry = moat.ContentEntry{Name: item.Name, ContentHash: hash}
		minTier = moat.TrustTierUnsigned
	default:
		return ItemGate{}, false
	}

	gate := PreInstallCheck(&entry, registryURL, lf, in.RevSet, session, minTier)
	return ItemGate{GateBlock: gate, Entry: &entry, RegistryURL: registryURL}, true
}
