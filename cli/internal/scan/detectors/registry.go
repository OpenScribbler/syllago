package detectors

import "github.com/holdenhewett/romanesco/cli/internal/scan"

// AllDetectors returns every registered detector (fact + surprise).
func AllDetectors() []scan.Detector {
	return []scan.Detector{
		// Tier 1 fact detectors
		TechStack{},
		Dependencies{},
		BuildCommands{},
		DirectoryStructure{},
		ProjectMetadata{},

		// Cross-language surprise detectors (1-12)
		CompetingFrameworks{},
		ModuleConflict{},
		MigrationInProgress{},
		WrapperBypass{},
		LockFileConflict{},
		TestConvention{},
		DeprecatedPattern{},
		PathAliasGap{},
		VersionMismatch{},
		VersionConstraint{},
		EnvConvention{},
		LinterExtraction{},

		// Go-specific (13-16)
		GoInternal{},
		GoNilInterface{},
		GoCGO{},
		GoReplace{},

		// Python-specific (17-19)
		PythonAsync{},
		PythonLayout{},
		PythonNamespace{},

		// Rust-specific (20-22)
		RustFeatures{},
		RustUnsafe{},
		RustAsyncRuntime{},

		// JS/TS-specific (23-24)
		TSStrictness{},
		MonorepoStructure{},
	}
}
