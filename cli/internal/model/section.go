package model

// Section is the interface satisfied by both typed and text sections.
type Section interface {
	SectionCategory() Category
	SectionOrigin() Origin
	SectionTitle() string
}

// TextSection holds freeform content (surprises, curated, heuristics).
type TextSection struct {
	Category Category `json:"category"`
	Origin   Origin   `json:"origin"`
	Title    string   `json:"title"`
	Body     string   `json:"body"`
	Source   string   `json:"source,omitempty"` // detector name that produced this
}

func (s TextSection) SectionCategory() Category { return s.Category }
func (s TextSection) SectionOrigin() Origin     { return s.Origin }
func (s TextSection) SectionTitle() string      { return s.Title }

// TechStackSection holds parsed tech stack facts.
type TechStackSection struct {
	Origin           Origin            `json:"origin"`
	Title            string            `json:"title"`
	Language         string            `json:"language"`
	LanguageVersion  string            `json:"languageVersion"`
	Framework        string            `json:"framework,omitempty"`
	FrameworkVersion string            `json:"frameworkVersion,omitempty"`
	Runtime          string            `json:"runtime,omitempty"`
	RuntimeVersion   string            `json:"runtimeVersion,omitempty"`
	Extra            map[string]string `json:"extra,omitempty"`
}

func (s TechStackSection) SectionCategory() Category { return CatTechStack }
func (s TechStackSection) SectionOrigin() Origin     { return s.Origin }
func (s TechStackSection) SectionTitle() string      { return s.Title }

// DependencySection holds grouped dependency information.
type DependencySection struct {
	Origin Origin            `json:"origin"`
	Title  string            `json:"title"`
	Groups []DependencyGroup `json:"groups"`
}

// DependencyGroup is a named collection of dependencies (e.g., "production", "dev").
type DependencyGroup struct {
	Category string       `json:"category"`
	Items    []Dependency `json:"items"`
}

// Dependency is a single named dependency with its version.
type Dependency struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (s DependencySection) SectionCategory() Category { return CatDependencies }
func (s DependencySection) SectionOrigin() Origin     { return s.Origin }
func (s DependencySection) SectionTitle() string      { return s.Title }

// BuildCommandSection holds build/task runner commands.
type BuildCommandSection struct {
	Origin   Origin         `json:"origin"`
	Title    string         `json:"title"`
	Commands []BuildCommand `json:"commands"`
}

// BuildCommand is a single build/task command extracted from a project file.
type BuildCommand struct {
	Name    string `json:"name"`    // e.g., "build", "test", "lint"
	Command string `json:"command"` // actual command text
	Source  string `json:"source"`  // file it came from (Makefile, package.json, etc.)
}

func (s BuildCommandSection) SectionCategory() Category { return CatBuildCommands }
func (s BuildCommandSection) SectionOrigin() Origin     { return s.Origin }
func (s BuildCommandSection) SectionTitle() string      { return s.Title }

// DirectoryStructureSection holds project layout information.
type DirectoryStructureSection struct {
	Origin  Origin           `json:"origin"`
	Title   string           `json:"title"`
	Entries []DirectoryEntry `json:"entries"`
}

// DirectoryEntry describes a single directory in the project layout.
type DirectoryEntry struct {
	Path        string `json:"path"`
	Description string `json:"description,omitempty"`
	Convention  string `json:"convention,omitempty"` // e.g., "source", "test", "config", "build"
}

func (s DirectoryStructureSection) SectionCategory() Category { return CatDirStructure }
func (s DirectoryStructureSection) SectionOrigin() Origin     { return s.Origin }
func (s DirectoryStructureSection) SectionTitle() string      { return s.Title }

// ProjectMetadataSection holds project-level facts.
type ProjectMetadataSection struct {
	Origin      Origin `json:"origin"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	License     string `json:"license,omitempty"`
	CI          string `json:"ci,omitempty"` // e.g., "GitHub Actions", "GitLab CI"
}

func (s ProjectMetadataSection) SectionCategory() Category { return CatProjectMeta }
func (s ProjectMetadataSection) SectionOrigin() Origin     { return s.Origin }
func (s ProjectMetadataSection) SectionTitle() string      { return s.Title }
