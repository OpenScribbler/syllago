//go:build !cgo

// Package extract_typescript provides TypeScript source extraction for capmon.
// The full implementation requires CGO (go-tree-sitter).
// When CGO is disabled, the extractor is not registered; sources with format "typescript"
// are skipped with a warning (ErrNoExtractor) during Stage 2.
package extract_typescript
