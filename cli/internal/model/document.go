package model

import "time"

// ContextDocument is the canonical representation of scanned codebase context.
// Produced by detectors, consumed by emitters, tracked by drift.
type ContextDocument struct {
	ProjectName string    `json:"projectName"`
	ScanTime    time.Time `json:"scanTime"`
	Sections    []Section `json:"sections"`
}

// Category identifies the semantic purpose of a section.
type Category string

const (
	CatTechStack    Category = "tech-stack"
	CatDependencies Category = "dependencies"
	CatBuildCommands Category = "build-commands"
	CatDirStructure Category = "directory-structure"
	CatProjectMeta  Category = "project-metadata"
	CatConventions  Category = "conventions"
	CatSurprise     Category = "surprise"
	CatCurated      Category = "curated"
)

// Origin distinguishes auto-maintained from human-authored content.
type Origin string

const (
	OriginAuto  Origin = "auto"
	OriginHuman Origin = "human"
)
