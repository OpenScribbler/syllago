package drift

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

const BaselineFileName = "baseline.json"

// Baseline represents a serialized scan snapshot.
type Baseline struct {
	ProjectName string           `json:"projectName"`
	Sections    []BaselineSection `json:"sections"`
}

// BaselineSection is a section with its content hash for efficient diffing.
type BaselineSection struct {
	Category string `json:"category"`
	Title    string `json:"title"`
	Origin   string `json:"origin"`
	Hash     string `json:"hash"`
	Content  string `json:"content,omitempty"`
}

// SaveBaseline writes a ContextDocument as a baseline snapshot.
func SaveBaseline(nescoDir string, doc model.ContextDocument) error {
	b := BaselineFromDocument(doc)

	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(nescoDir, 0755); err != nil {
		return err
	}

	path := filepath.Join(nescoDir, BaselineFileName)
	return os.WriteFile(path, data, 0644)
}

// LoadBaseline reads a baseline snapshot from disk.
func LoadBaseline(nescoDir string) (*Baseline, error) {
	path := filepath.Join(nescoDir, BaselineFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no baseline found: %w", err)
	}

	var b Baseline
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// BaselineExists checks if a baseline file exists.
func BaselineExists(nescoDir string) bool {
	_, err := os.Stat(filepath.Join(nescoDir, BaselineFileName))
	return err == nil
}

// BaselineFromDocument creates a Baseline from a ContextDocument without writing to disk.
func BaselineFromDocument(doc model.ContextDocument) *Baseline {
	b := &Baseline{ProjectName: doc.ProjectName}
	for _, s := range doc.Sections {
		if s.SectionOrigin() == model.OriginHuman {
			continue // only track auto-maintained sections
		}
		content, _ := json.Marshal(s)
		hash := sha256.Sum256(content)
		b.Sections = append(b.Sections, BaselineSection{
			Category: string(s.SectionCategory()),
			Title:    s.SectionTitle(),
			Origin:   string(s.SectionOrigin()),
			Hash:     hex.EncodeToString(hash[:]),
			Content:  string(content),
		})
	}
	return b
}
