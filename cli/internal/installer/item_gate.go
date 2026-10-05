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
// The registry's attestations vouch only for the hash it lists. A copy
// staged at another hash, or one the registry no longer lists, is checked
// for revocations and for the listing's privacy declaration, and skips
// the tier floor, since no attestation of its own hash is at hand. A copy
// whose registry has no cached manifest is still refused when the
// lockfile has archived a revocation of its hash.
//
// ok is false for an item no MOAT registry or lockfile accounts for,
// which installs ungated.
func CheckItem(item catalog.ContentItem, in *moat.GateInputs, lf *moat.Lockfile, session *moat.Session, minTier moat.TrustTier) (ItemGate, bool) {
	regName, hash := item.Registry, ""
	if regName == "" && item.Meta != nil {
		regName, hash = item.Meta.SourceRegistry, item.Meta.SourceHash
	}
	if regName == "" {
		return ItemGate{}, false
	}
	var registryURL string
	if in != nil {
		registryURL = in.ManifestURIs[regName]
	}

	var listed *moat.ContentEntry
	var ok bool
	if in.HasRegistry(regName) {
		listed, ok = moat.FindTypedEntry(in.Manifests[regName], item.Name, item.Type)
	}
	var entry moat.ContentEntry
	switch {
	case ok && (hash == "" || hash == listed.ContentHash):
		entry = *listed
	case ok:
		entry = moat.ContentEntry{Name: listed.Name, Type: listed.Type, ContentHash: hash, PrivateRepo: listed.PrivateRepo}
		minTier = moat.TrustTierUnsigned
	case hash != "" && (in.HasRegistry(regName) || lf != nil && lf.IsRevoked(hash)):
		entry = moat.ContentEntry{Name: item.Name, ContentHash: hash}
		minTier = moat.TrustTierUnsigned
	default:
		return ItemGate{}, false
	}

	var revSet *moat.RevocationSet
	if in != nil {
		revSet = in.RevSet
	}
	gate := PreInstallCheck(&entry, registryURL, lf, revSet, session, minTier)
	return ItemGate{GateBlock: gate, Entry: &entry, RegistryURL: registryURL}, true
}
