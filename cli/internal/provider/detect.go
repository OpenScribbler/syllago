package provider

import (
	"os"
)

// DetectProviders checks the filesystem for installed AI coding tools
// and returns a slice of Providers with Detected set appropriately.
func DetectProviders() []Provider {
	home, err := os.UserHomeDir()
	if err != nil {
		return AllProviders // return all as not detected
	}

	var result []Provider
	for _, p := range AllProviders {
		detected := p // copy
		if p.Detect != nil {
			detected.Detected = p.Detect(home)
		}
		result = append(result, detected)
	}
	return result
}
