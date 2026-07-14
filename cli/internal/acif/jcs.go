package acif

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	jsoncanonicalizer "github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
)

// CanonicalJSON returns the RFC 8785 (JCS) serialization of the JSON text in raw.
func CanonicalJSON(raw json.RawMessage) ([]byte, error) {
	return jsoncanonicalizer.Transform([]byte(raw))
}

// MetadataHash computes [ACIF-PUBLISHER] §6: lowercase-hex SHA-256 over
// JCS(publisher_section) followed by one LF (0x0A).
func MetadataHash(publisherSection json.RawMessage) (hashHex string, canonicalBytes []byte, err error) {
	canonicalBytes, err = CanonicalJSON(publisherSection)
	if err != nil {
		return "", nil, err
	}
	h := sha256.New()
	_, _ = h.Write(canonicalBytes)
	_, _ = h.Write([]byte{'\n'})
	return hex.EncodeToString(h.Sum(nil)), canonicalBytes, nil
}
