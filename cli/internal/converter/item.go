package converter

import (
	"errors"
	"fmt"
	"os"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

var (
	// ErrNotConvertible means the item's content type has no format
	// conversion.
	ErrNotConvertible = errors.New("content type has no format conversion")
	// ErrNoContentFile means the item's directory holds no content file.
	ErrNoContentFile = errors.New("no content file found")
	// ErrUnreadable means the content could not be read as its source
	// provider's format.
	ErrUnreadable = errors.New("content could not be read as its source format")
	// ErrRender means the content was read but the target's format could
	// not be written.
	ErrRender = errors.New("rendering failed")
)

// Conversion is a library item converted to one provider's format.
type Conversion struct {
	Result
	// From is the provider whose format the content was read as; "" means
	// syllago's canonical format.
	From string
	// Source is the item's content file as read.
	Source []byte
}

// ConvertItem converts a library item to the target provider's format.
// from overrides the item's source provider; when it is "", the provider
// recorded when the item was added wins over the provider directory the
// item sits in. A target that cannot hold the content returns a
// Conversion whose Content is nil, with the warnings that say why.
func ConvertItem(item catalog.ContentItem, to provider.Provider, from string) (*Conversion, error) {
	conv := For(item.Type)
	if conv == nil && item.Type != catalog.Hooks {
		return nil, fmt.Errorf("%w: %s", ErrNotConvertible, item.Type.Label())
	}
	contentFile := ResolveContentFile(item)
	if contentFile == "" {
		return nil, fmt.Errorf("%w for %s", ErrNoContentFile, item.Name)
	}
	raw, err := os.ReadFile(contentFile)
	if err != nil {
		return nil, err
	}
	if from == "" && item.Meta != nil {
		from = item.Meta.SourceProvider
	}
	if from == "" {
		from = item.Provider
	}
	c := &Conversion{From: from, Source: raw}

	if item.Type == catalog.Hooks {
		rendered, err := ConvertHooks(raw, from, to.Slug)
		if errors.Is(err, ErrNoHookEncoder) {
			return nil, err
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrUnreadable, err)
		}
		c.Result = *rendered
		return c, nil
	}

	canonical, err := conv.Canonicalize(raw, from)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnreadable, err)
	}
	rendered, err := conv.Render(canonical.Content, to)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRender, err)
	}
	c.Result = *rendered
	return c, nil
}

// ProviderCompat says whether one provider can take an item, and what
// converting the item to that provider's format loses.
type ProviderCompat struct {
	Provider  provider.Provider
	Supported bool
	Warnings  []string
}

// CompatReport converts an item to every known provider's format and
// reports the result for each. An error means the item itself cannot be
// read; a provider that cannot take the item is a row with Supported
// false.
func CompatReport(item catalog.ContentItem) ([]ProviderCompat, error) {
	var report []ProviderCompat
	for _, prov := range provider.AllProviders {
		row := ProviderCompat{Provider: prov}
		if prov.SupportsType == nil || !prov.SupportsType(item.Type) {
			row.Warnings = []string{item.Type.Label() + " not supported"}
			report = append(report, row)
			continue
		}
		c, err := ConvertItem(item, prov, "")
		switch {
		case errors.Is(err, ErrNotConvertible):
			// The type installs as is, so a provider that holds it takes it.
			row.Supported = true
		case errors.Is(err, ErrNoHookEncoder):
			row.Warnings = []string{"syllago cannot write hooks for " + prov.Name}
		case errors.Is(err, ErrRender):
			row.Warnings = []string{err.Error()}
		case err != nil:
			return nil, err
		case c.Content == nil:
			row.Warnings = c.Warnings
			if len(row.Warnings) == 0 {
				row.Warnings = []string{"conversion produced no output"}
			}
		default:
			row.Supported = true
			row.Warnings = c.Warnings
		}
		report = append(report, row)
	}
	return report, nil
}
