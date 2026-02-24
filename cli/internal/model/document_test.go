package model

import "testing"

func TestSectionInterface(t *testing.T) {
	t.Parallel()
	sections := []Section{
		TextSection{Category: CatSurprise, Origin: OriginAuto, Title: "Competing test frameworks", Body: "Jest and Vitest both present"},
		TechStackSection{Origin: OriginAuto, Title: "Tech Stack", Language: "TypeScript", LanguageVersion: "5.3"},
		BuildCommandSection{Origin: OriginAuto, Title: "Build Commands", Commands: []BuildCommand{{Name: "test", Command: "npm test", Source: "package.json"}}},
		DependencySection{Origin: OriginAuto, Title: "Dependencies", Groups: []DependencyGroup{{Category: "production", Items: []Dependency{{Name: "react", Version: "18.0"}}}}},
		DirectoryStructureSection{Origin: OriginAuto, Title: "Directory Structure", Entries: []DirectoryEntry{{Path: "src/", Convention: "source"}}},
		ProjectMetadataSection{Origin: OriginAuto, Title: "Project Metadata", License: "MIT"},
	}

	expected := []Category{CatSurprise, CatTechStack, CatBuildCommands, CatDependencies, CatDirStructure, CatProjectMeta}
	for i, s := range sections {
		if s.SectionCategory() != expected[i] {
			t.Errorf("section %d: category = %q, want %q", i, s.SectionCategory(), expected[i])
		}
		if s.SectionOrigin() != OriginAuto {
			t.Errorf("section %d: origin = %q, want %q", i, s.SectionOrigin(), OriginAuto)
		}
	}
}

func TestContextDocumentSections(t *testing.T) {
	t.Parallel()
	doc := ContextDocument{
		ProjectName: "test-project",
		Sections: []Section{
			TechStackSection{Origin: OriginAuto, Title: "Tech Stack", Language: "Go", LanguageVersion: "1.25"},
			TextSection{Category: CatSurprise, Origin: OriginAuto, Title: "Internal package imported externally"},
			TextSection{Category: CatCurated, Origin: OriginHuman, Title: "Architecture notes"},
		},
	}

	if len(doc.Sections) != 3 {
		t.Fatalf("got %d sections, want 3", len(doc.Sections))
	}
	if doc.Sections[2].SectionOrigin() != OriginHuman {
		t.Errorf("curated section origin = %q, want %q", doc.Sections[2].SectionOrigin(), OriginHuman)
	}
}
