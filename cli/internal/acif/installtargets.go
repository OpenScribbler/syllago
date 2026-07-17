package acif

import (
	_ "embed"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Entry-point resolution over the install-target matrix
// ([ACIF-INSTALL] §6–§9, §11; PROTOCOL op 4.14).

// InstallEntryPointsPathEnv points at a refreshed copy of the
// conformance/install-entry-points.yaml export. The embedded copy is the
// vendored fallback ([ACIF-INSTALL] §12, refresh-over-vendored).
const InstallEntryPointsPathEnv = "ACIF_INSTALL_ENTRY_POINTS"

//go:embed install-entry-points.yaml
var vendoredInstallEntryPoints []byte

// InstallEntry is one entry-point row ([ACIF-INSTALL] §6). JSON tags cover
// the PROTOCOL §4.14 `entry` override; YAML tags cover the export rows.
type InstallEntry struct {
	Scope        string `yaml:"scope" json:"scope"`
	PathTemplate string `yaml:"path_template" json:"path_template"`
	Layout       string `yaml:"layout" json:"layout"`
	Status       string `yaml:"status" json:"status"`
}

// InstallTarget is one resolved entry point (PROTOCOL §4.14 result row).
type InstallTarget struct {
	Scope       string `json:"scope"`
	Path        string `json:"path"`
	Layout      string `json:"layout"`
	Status      string `json:"status"`
	WriteTarget bool   `json:"write_target"`
}

// InstallResolveInput carries the PROTOCOL §4.14 inputs. Entry, when
// non-nil, overrides the matrix with a single fully-formed row.
type InstallResolveInput struct {
	Provider    string
	ContentType string
	ContentName string
	HomeDir     string
	ProjectRoot string
	Scope       string
	Entry       *InstallEntry
}

type installMatrix map[string]map[string][]InstallEntry

var (
	installMatrixMu     sync.RWMutex
	installMatrixLoaded bool
	installMatrixVal    installMatrix
	installMatrixErr    error
)

func parseInstallMatrix(data []byte) (installMatrix, error) {
	var doc struct {
		InstallEntryPoints installMatrix `yaml:"install_entry_points"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse install-entry-points export: %w", err)
	}
	if doc.InstallEntryPoints == nil {
		return nil, fmt.Errorf("parse install-entry-points export: missing install_entry_points")
	}
	return doc.InstallEntryPoints, nil
}

func loadVendoredInstallMatrix() (installMatrix, error) {
	installMatrixMu.RLock()
	if installMatrixLoaded {
		matrix, err := installMatrixVal, installMatrixErr
		installMatrixMu.RUnlock()
		return matrix, err
	}
	installMatrixMu.RUnlock()

	matrix, err := parseInstallMatrix(vendoredInstallEntryPoints)

	installMatrixMu.Lock()
	if !installMatrixLoaded {
		installMatrixVal = matrix
		installMatrixErr = err
		installMatrixLoaded = true
	}
	matrix, err = installMatrixVal, installMatrixErr
	installMatrixMu.Unlock()

	return matrix, err
}

func swapInstallMatrix(matrix installMatrix) {
	installMatrixMu.Lock()
	installMatrixVal = matrix
	installMatrixErr = nil
	installMatrixLoaded = true
	installMatrixMu.Unlock()
}

func loadInstallMatrix() (installMatrix, error) {
	if path := os.Getenv(InstallEntryPointsPathEnv); path != "" {
		refreshed, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		return parseInstallMatrix(refreshed)
	}
	return loadVendoredInstallMatrix()
}

// InstallEntryRows returns the matrix rows for a provider/content-type pair.
// The ACIF_INSTALL_ENTRY_POINTS override has the same precedence here as it
// does in ResolveInstallTargets.
func InstallEntryRows(provider, contentType string) ([]InstallEntry, error) {
	matrix, err := loadInstallMatrix()
	if err != nil {
		return nil, err
	}
	rows := matrix[provider][contentType]
	if len(rows) == 0 {
		return nil, nil
	}
	return append([]InstallEntry(nil), rows...), nil
}

// installPlaceholderRe matches placeholder tokens in a path template. The
// token set is closed (§8.1): `<content-name>` is the only member in 0.1.
var installPlaceholderRe = regexp.MustCompile(`<[^<>/]+>`)

func unrecognizedInstallToken(template string) string {
	for _, token := range installPlaceholderRe.FindAllString(template, -1) {
		if token != "<content-name>" {
			return token
		}
	}
	return ""
}

// resolveInstallPath anchors and substitutes one row (§8.3). Resolution is
// byte-exact string assembly: no separator normalization, no Clean — a
// trailing slash on a directory_of_files template survives.
func resolveInstallPath(row InstallEntry, contentName, homeDir, projectRoot string) string {
	path := row.PathTemplate
	switch {
	case strings.HasPrefix(path, "~/"):
		path = homeDir + path[1:]
	case row.Scope == "managed":
		// Managed templates are absolute and resolve verbatim.
	default:
		path = projectRoot + "/" + path
	}
	return strings.ReplaceAll(path, "<content-name>", contentName)
}

// ResolveInstallTargets resolves the ordered entry-point rows for an
// invocation per [ACIF-INSTALL] §6–§9 and the §11 disposition lanes. The
// returned error is a *RejectError for every spec-minted refusal.
func ResolveInstallTargets(in InstallResolveInput) ([]InstallTarget, []Diagnostic, error) {
	// §8.2 validity predicate — rejected before any path is formed. A
	// content name carries no separators, so the whole name is the only
	// segment substitution can produce.
	name := in.ContentName
	if name == "" || name == "." || name == ".." ||
		strings.ContainsAny(name, "/\\") || strings.ContainsRune(name, 0) {
		return nil, nil, &RejectError{ID: "acif.install.content_name_invalid"}
	}

	var rows []InstallEntry
	if in.Entry != nil {
		rows = []InstallEntry{*in.Entry}
	} else {
		matrix, err := loadInstallMatrix()
		if err != nil {
			return nil, nil, err
		}
		rows = matrix[in.Provider][in.ContentType]
	}

	// §6: absence is meaningful — no rows means refuse, never guess.
	if len(rows) == 0 {
		return nil, nil, &RejectError{ID: "acif.install.no_entry_point"}
	}

	if in.Scope != "" {
		var filtered []InstallEntry
		var available []string
		seen := map[string]bool{}
		for _, row := range rows {
			if row.Scope == in.Scope {
				filtered = append(filtered, row)
			} else if !seen[row.Scope] {
				seen[row.Scope] = true
				available = append(available, row.Scope)
			}
		}
		if len(filtered) == 0 {
			// PROTOCOL Appendix A pins available_scopes as sorted.
			sort.Strings(available)
			return nil, nil, &RejectError{
				ID: "acif.install.scope_unavailable",
				Diagnostics: []Diagnostic{{
					ID:     "acif.install.scope_unavailable",
					Params: map[string]any{"available_scopes": available},
				}},
			}
		}
		rows = filtered
	}

	// §6: the first status-current row per scope is the write target.
	writeIndex := map[string]int{}
	for i, row := range rows {
		if row.Status == "current" {
			if _, ok := writeIndex[row.Scope]; !ok {
				writeIndex[row.Scope] = i
			}
		}
	}

	targets := make([]InstallTarget, 0, len(rows))
	var diags []Diagnostic
	for i, row := range rows {
		isWrite := false
		if j, ok := writeIndex[row.Scope]; ok && j == i {
			isWrite = true
		}
		// §8.1 placeholder totality: a token outside the closed grammar
		// refuses the write direction; a read/discovery row cannot yield a
		// resolved path either, so it is disclosed and skipped (§11).
		if token := unrecognizedInstallToken(row.PathTemplate); token != "" {
			diag := Diagnostic{
				ID:     "acif.install.placeholder_unrecognized",
				Params: map[string]any{"token": token},
			}
			if isWrite {
				return nil, nil, &RejectError{
					ID:          "acif.install.placeholder_unrecognized",
					Diagnostics: []Diagnostic{diag},
				}
			}
			diags = append(diags, diag)
			continue
		}
		targets = append(targets, InstallTarget{
			Scope:       row.Scope,
			Path:        resolveInstallPath(row, name, in.HomeDir, in.ProjectRoot),
			Layout:      row.Layout,
			Status:      row.Status,
			WriteTarget: isWrite,
		})
	}

	// §11 supersession lane: a scope whose rows exist but include no
	// status-current row resolves through a superseded location — warn,
	// with the targets still returned.
	warned := map[string]bool{}
	for _, row := range rows {
		if _, ok := writeIndex[row.Scope]; !ok && !warned[row.Scope] {
			warned[row.Scope] = true
			diags = append(diags, Diagnostic{
				ID:     "acif.install.entry_point_superseded",
				Params: map[string]any{"scope": row.Scope},
			})
		}
	}

	return targets, diags, nil
}
