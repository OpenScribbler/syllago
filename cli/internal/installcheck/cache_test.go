package installcheck

import "testing"

// TestScan_MtimeCacheSkipsRead verifies that a repeat Scan, given a target
// file whose mtime+size are unchanged, does not call readFile (the filesystem
// hook indirected in scan.go).
func TestScan_MtimeCacheSkipsRead(t *testing.T) {
	// Cannot run in parallel: mutates the package-private readFile hook.
	body := []byte("# Cache me\n\nBody.\n")
	inst, library, _ := seedRuleAndInstall(t, body)

	origReadFile := readFile
	t.Cleanup(func() { readFile = origReadFile })

	var reads int
	readFile = func(name string) ([]byte, error) {
		reads++
		return origReadFile(name)
	}

	// First scan populates the cache (one read expected).
	_ = Scan(inst, library)
	firstReads := reads
	if firstReads == 0 {
		t.Fatal("first scan performed zero reads; expected at least one")
	}
	// Second scan, same mtime+size — expect zero additional reads.
	_ = Scan(inst, library)
	if reads != firstReads {
		t.Errorf("second scan performed %d reads; expected cache hit (still %d)", reads-firstReads, firstReads)
	}

	// Sanity: cache invalidation makes the next scan re-read.
	for _, r := range inst.RuleAppends {
		InvalidateCache(r.TargetFile)
	}
	_ = Scan(inst, library)
	if reads == firstReads {
		t.Errorf("scan after InvalidateCache did not re-read; expected >%d reads, got %d", firstReads, reads)
	}

}
