package acif

// Diagnostic is the ACIF structured diagnostic (adapter protocol §3.1).
type Diagnostic struct {
	ID     string         `json:"id"`
	Params map[string]any `json:"params,omitempty"`
}

// RejectError carries a spec-minted acif.* error identifier.
type RejectError struct {
	ID     string
	Detail string
}

func (e *RejectError) Error() string {
	if e == nil {
		return ""
	}
	if e.Detail == "" {
		return e.ID
	}
	return e.ID + ": " + e.Detail
}

const (
	ErrBodySymlink       = "acif.body.symlink"
	ErrBodyPathCollision = "acif.body.path_collision"
	ErrBodyEmpty         = "acif.body.empty"
)
