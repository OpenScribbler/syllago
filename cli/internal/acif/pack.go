package acif

import "github.com/google/uuid"

// DerivePackID implements [ACIF-PUBLISHER] §9.4 over already-canonical inputs:
// UUIDv5(namespace, canonicalRepositoryURL + "\n" + canonicalDisplayName).
func DerivePackID(namespace uuid.UUID, repositoryURL, displayName string) uuid.UUID {
	return uuid.NewSHA1(namespace, []byte(repositoryURL+"\n"+displayName))
}

type PackResolution struct {
	Resolution string
	MemberOf   string
	Install    string
}

func ResolvePack(declaredPackID, inferredPackID string, knownPackIDs []string) PackResolution {
	if declaredPackID != "" {
		for _, known := range knownPackIDs {
			if declaredPackID == known {
				return PackResolution{
					Resolution: "declared",
					MemberOf:   declaredPackID,
					Install:    "proceed",
				}
			}
		}
		return PackResolution{
			Resolution: "unresolved",
			Install:    "refuse-unless-operator-opt-in",
		}
	}
	if inferredPackID != "" {
		return PackResolution{
			Resolution: "inferred",
			MemberOf:   inferredPackID,
			Install:    "proceed",
		}
	}
	return PackResolution{
		Resolution: "none",
		Install:    "proceed",
	}
}
