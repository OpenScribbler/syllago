package main

import rollbackpkg "github.com/OpenScribbler/syllago/cli/internal/rollback"

func shortSHA(sha string) string {
	return rollbackpkg.ShortSHA(sha)
}
