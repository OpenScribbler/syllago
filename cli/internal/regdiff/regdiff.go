package regdiff

// Kind classifies how an item changed between two registry states.
type Kind string

const (
	KindAdded    Kind = "added"
	KindModified Kind = "modified"
	KindRemoved  Kind = "removed"
)

// ItemChange is one content item's change between two registry states.
type ItemChange struct {
	Type  string // content type dir/slug, e.g. "skills", or MOAT type "skill"
	Name  string // item name
	Kind  Kind
	Paths []string // changed file paths relative to repo root (git registries); nil for MOAT
}

// Diff is the full change set for one registry between two refs.
type Diff struct {
	Registry   string       // registry name
	OldRef     string       // old git SHA or old manifest UpdatedAt (RFC3339); "" when no baseline
	NewRef     string       // new git SHA or new manifest UpdatedAt (RFC3339)
	UpToDate   bool         // true when OldRef == NewRef (nothing to diff)
	Changes    []ItemChange // sorted by (Type, Name)
	OtherPaths []string     // changed paths not attributable to any item (git only), sorted
}

// ItemRef locates one item inside a git registry checkout. Callers build
// this from their catalog scan of the CURRENT (new) checkout.
type ItemRef struct {
	Type string // content type, e.g. "skills"
	Name string
	Dir  string // item directory relative to repo root, e.g. "skills/my-skill"
}
