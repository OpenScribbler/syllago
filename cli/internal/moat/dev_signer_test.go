package moat

import (
	"testing"
)

// TestSignManifestDev_HappyPath verifies that SignManifestDev produces a
// matched (bundleJSON, trustedRootJSON) pair that round-trips through
// VerifyManifest without error.
func TestSignManifestDev_HappyPath(t *testing.T) {
	t.Parallel()

	manifestBytes := []byte(`{"_version":1,"registry":"test","content":[]}`)
	bundleJSON, trustedRootJSON, profile, err := SignManifestDev(manifestBytes)
	if err != nil {
		t.Fatalf("SignManifestDev failed: %v", err)
	}
	if len(bundleJSON) == 0 {
		t.Fatal("bundleJSON is empty")
	}
	if len(trustedRootJSON) == 0 {
		t.Fatal("trustedRootJSON is empty")
	}
	if profile.Issuer != DevSigningIssuer {
		t.Errorf("unexpected issuer: got %q, want %q", profile.Issuer, DevSigningIssuer)
	}
	if profile.Subject != DevSigningSubject {
		t.Errorf("unexpected subject: got %q, want %q", profile.Subject, DevSigningSubject)
	}

	// Round-trip verification: the bundle must verify against the same manifest
	// bytes using the dev trusted root.
	_, err = VerifyManifest(manifestBytes, bundleJSON, &profile, trustedRootJSON)
	if err != nil {
		t.Fatalf("VerifyManifest round-trip failed: %v", err)
	}
}

// TestSignManifestDev_DifferentManifestFails verifies that a bundle produced
// for one manifest does NOT verify against a different manifest's bytes.
func TestSignManifestDev_DifferentManifestFails(t *testing.T) {
	t.Parallel()

	manifestBytes := []byte(`{"_version":1,"registry":"test","content":[]}`)
	bundleJSON, trustedRootJSON, profile, err := SignManifestDev(manifestBytes)
	if err != nil {
		t.Fatalf("SignManifestDev failed: %v", err)
	}

	tampered := []byte(`{"_version":1,"registry":"attacker","content":[]}`)
	_, err = VerifyManifest(tampered, bundleJSON, &profile, trustedRootJSON)
	if err == nil {
		t.Fatal("VerifyManifest must reject a different manifest, got nil error")
	}
}

// TestSignManifestDev_WrongProfileFails verifies that the wrong OIDC profile
// is rejected by VerifyManifest.
func TestSignManifestDev_WrongProfileFails(t *testing.T) {
	t.Parallel()

	manifestBytes := []byte(`{"_version":1,"registry":"test","content":[]}`)
	bundleJSON, trustedRootJSON, _, err := SignManifestDev(manifestBytes)
	if err != nil {
		t.Fatalf("SignManifestDev failed: %v", err)
	}

	wrongProfile := SigningProfile{
		Issuer:  "https://token.actions.githubusercontent.com",
		Subject: "https://github.com/attacker/evil/.github/workflows/publish.yml@refs/heads/main",
	}
	_, err = VerifyManifest(manifestBytes, bundleJSON, &wrongProfile, trustedRootJSON)
	if err == nil {
		t.Fatal("VerifyManifest must reject wrong identity, got nil error")
	}
}
