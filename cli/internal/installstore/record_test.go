package installstore

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeContentFixture(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	writeFile(t, path, []byte("# "+name+"\n"))
	return path
}

func assertStoreFileMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("store file exists or stat failed: %v", err)
	}
}

func TestRecordInstallCreatesStoreAndKeylessPlacement(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	storePath := filepath.Join(dir, "state", "installs.json")
	libraryPath := writeContentFixture(t, dir, "skill.md")
	coord := testCoord("core", "skills", "writer")
	now := fixedTime(1)

	err := RecordInstall(storePath, coord, libraryPath, PlacementInput{
		Provider:  "codex",
		Mechanism: MechanismSymlink,
		Path:      "/tmp/home/.codex/skills/writer",
	}, now)
	if err != nil {
		t.Fatalf("RecordInstall: %v", err)
	}

	s := mustLoadStore(t, storePath)
	rec := s.Find(coord)
	if rec == nil {
		t.Fatal("record missing")
	}
	if rec.Coord != coord {
		t.Fatalf("Coord = %#v, want %#v", rec.Coord, coord)
	}
	if rec.LibraryPath != libraryPath {
		t.Fatalf("LibraryPath = %s, want %s", rec.LibraryPath, libraryPath)
	}
	if rec.ContentHash == "" {
		t.Fatal("ContentHash is empty")
	}
	if rec.InstalledAt != now || rec.UpdatedAt != now {
		t.Fatalf("times = installed %v updated %v, want %v", rec.InstalledAt, rec.UpdatedAt, now)
	}
	if len(rec.Placements) != 1 {
		t.Fatalf("Placements length = %d, want 1", len(rec.Placements))
	}
	gotPlacement := rec.Placements[0]
	wantPlacement := Placement{
		Provider:    "codex",
		Mechanism:   MechanismSymlink,
		Path:        "/tmp/home/.codex/skills/writer",
		InstalledAt: now,
	}
	if gotPlacement != wantPlacement {
		t.Fatalf("placement = %#v, want %#v", gotPlacement, wantPlacement)
	}
}

func TestRecordInstallKeyedPlacementReplacesIdentity(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	storePath := filepath.Join(dir, "installs.json")
	libraryPath := writeContentFixture(t, dir, "hook.json")
	coord := testCoord("core", "hooks", "audit")
	first := fixedTime(2)
	second := fixedTime(3)
	input := PlacementInput{
		Provider:  "claude-code",
		Mechanism: MechanismHookMerge,
		Path:      "/tmp/home/.claude/settings.json",
		Keys:      []string{"hooks.PreToolUse"},
	}

	if err := RecordInstall(storePath, coord, libraryPath, input, first); err != nil {
		t.Fatalf("RecordInstall first: %v", err)
	}
	if err := RecordInstall(storePath, coord, libraryPath, input, second); err != nil {
		t.Fatalf("RecordInstall second: %v", err)
	}

	rec := mustLoadStore(t, storePath).Find(coord)
	if rec == nil {
		t.Fatal("record missing")
	}
	if len(rec.Placements) != 1 {
		t.Fatalf("Placements length = %d, want 1", len(rec.Placements))
	}
	got := rec.Placements[0]
	if got.Key != "hooks.PreToolUse" {
		t.Fatalf("Key = %q, want hooks.PreToolUse", got.Key)
	}
	if got.InstalledAt != second {
		t.Fatalf("InstalledAt = %v, want replacement time %v", got.InstalledAt, second)
	}
}

func TestRecordInstallSecondProviderPreservesInstallTimeAndBumpsUpdatedAt(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	storePath := filepath.Join(dir, "installs.json")
	libraryPath := writeContentFixture(t, dir, "writer.md")
	coord := testCoord("core", "skills", "writer")
	first := fixedTime(4)
	second := fixedTime(5)

	if err := RecordInstall(storePath, coord, libraryPath, PlacementInput{
		Provider:  "codex",
		Mechanism: MechanismSymlink,
		Path:      "/tmp/codex/writer",
	}, first); err != nil {
		t.Fatalf("RecordInstall first: %v", err)
	}
	if err := RecordInstall(storePath, coord, libraryPath, PlacementInput{
		Provider:  "claude-code",
		Mechanism: MechanismSymlink,
		Path:      "/tmp/claude/writer",
	}, second); err != nil {
		t.Fatalf("RecordInstall second: %v", err)
	}

	rec := mustLoadStore(t, storePath).Find(coord)
	if rec == nil {
		t.Fatal("record missing")
	}
	if rec.InstalledAt != first {
		t.Fatalf("InstalledAt = %v, want preserved %v", rec.InstalledAt, first)
	}
	if rec.UpdatedAt != second {
		t.Fatalf("UpdatedAt = %v, want %v", rec.UpdatedAt, second)
	}
	if got := providersOf(rec.Placements); !reflect.DeepEqual(got, []string{"claude-code", "codex"}) {
		t.Fatalf("providers = %#v, want claude-code/codex", got)
	}
}

func TestRecordInstallHashFailureLeavesStoreUntouched(t *testing.T) {
	t.Parallel()

	storePath := filepath.Join(t.TempDir(), "installs.json")
	err := RecordInstall(storePath, testCoord("core", "skills", "missing"), filepath.Join(t.TempDir(), "missing.md"), PlacementInput{
		Provider:  "codex",
		Mechanism: MechanismSymlink,
		Path:      "/tmp/codex/missing",
	}, fixedTime(6))
	if err == nil {
		t.Fatal("RecordInstall missing libraryPath returned nil error")
	}
	assertStoreFileMissing(t, storePath)
}

func TestRecordUninstallRemovesPlacementAndPrunesEmptyRecord(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	storePath := filepath.Join(dir, "installs.json")
	libraryPath := writeContentFixture(t, dir, "writer.md")
	coord := testCoord("core", "skills", "writer")

	codex := PlacementInput{Provider: "codex", Mechanism: MechanismSymlink, Path: "/tmp/codex/writer"}
	claude := PlacementInput{Provider: "claude-code", Mechanism: MechanismSymlink, Path: "/tmp/claude/writer"}
	if err := RecordInstall(storePath, coord, libraryPath, codex, fixedTime(7)); err != nil {
		t.Fatalf("RecordInstall codex: %v", err)
	}
	if err := RecordInstall(storePath, coord, libraryPath, claude, fixedTime(8)); err != nil {
		t.Fatalf("RecordInstall claude: %v", err)
	}

	if err := RecordUninstall(storePath, coord, codex, fixedTime(9)); err != nil {
		t.Fatalf("RecordUninstall first: %v", err)
	}
	rec := mustLoadStore(t, storePath).Find(coord)
	if rec == nil {
		t.Fatal("record pruned too early")
	}
	if got := providersOf(rec.Placements); !reflect.DeepEqual(got, []string{"claude-code"}) {
		t.Fatalf("providers after first uninstall = %#v, want claude-code", got)
	}

	if err := RecordUninstall(storePath, coord, claude, fixedTime(10)); err != nil {
		t.Fatalf("RecordUninstall last: %v", err)
	}
	if rec := mustLoadStore(t, storePath).Find(coord); rec != nil {
		t.Fatalf("record after last placement removal = %#v, want nil", rec)
	}
}

func TestRecordUninstallNoOpsDoNotCreateOrRewriteStore(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	input := PlacementInput{Provider: "codex", Mechanism: MechanismSymlink, Path: "/tmp/codex/writer"}

	missingStore := filepath.Join(dir, "missing", "installs.json")
	if err := RecordUninstall(missingStore, testCoord("core", "skills", "writer"), input, fixedTime(11)); err != nil {
		t.Fatalf("RecordUninstall missing store: %v", err)
	}
	assertStoreFileMissing(t, missingStore)

	storePath := filepath.Join(dir, "installs.json")
	s := mustLoadStore(t, storePath)
	s.Upsert(testRecord(testCoord("core", "skills", "other"), fixedTime(12)))
	mustSaveStore(t, s)
	before := mustReadFile(t, storePath)

	if err := RecordUninstall(storePath, testCoord("core", "skills", "writer"), input, fixedTime(13)); err != nil {
		t.Fatalf("RecordUninstall missing record: %v", err)
	}
	if after := mustReadFile(t, storePath); !reflect.DeepEqual(after, before) {
		t.Fatalf("missing-record no-op rewrote store\nbefore:\n%s\nafter:\n%s", before, after)
	}

	if err := RecordUninstall(storePath, testCoord("core", "skills", "other"), input, fixedTime(14)); err != nil {
		t.Fatalf("RecordUninstall missing placement: %v", err)
	}
	if after := mustReadFile(t, storePath); !reflect.DeepEqual(after, before) {
		t.Fatalf("missing-placement no-op rewrote store\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestForgetRecordRemovesRecordAndNoOpsWhenMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	storePath := filepath.Join(dir, "installs.json")
	libraryPath := writeContentFixture(t, dir, "writer.md")
	coord := testCoord("core", "skills", "writer")
	input := PlacementInput{Provider: "codex", Mechanism: MechanismSymlink, Path: "/tmp/codex/writer"}
	if err := RecordInstall(storePath, coord, libraryPath, input, fixedTime(15)); err != nil {
		t.Fatalf("RecordInstall: %v", err)
	}

	if err := ForgetRecord(storePath, coord); err != nil {
		t.Fatalf("ForgetRecord present: %v", err)
	}
	if rec := mustLoadStore(t, storePath).Find(coord); rec != nil {
		t.Fatalf("record after ForgetRecord = %#v, want nil", rec)
	}
	before := mustReadFile(t, storePath)
	if err := ForgetRecord(storePath, coord); err != nil {
		t.Fatalf("ForgetRecord missing: %v", err)
	}
	if after := mustReadFile(t, storePath); !reflect.DeepEqual(after, before) {
		t.Fatalf("missing ForgetRecord rewrote store\nbefore:\n%s\nafter:\n%s", before, after)
	}

	missingStore := filepath.Join(dir, "none", "installs.json")
	if err := ForgetRecord(missingStore, coord); err != nil {
		t.Fatalf("ForgetRecord missing store: %v", err)
	}
	assertStoreFileMissing(t, missingStore)
}

func TestRecordInstallPreservesMOAT(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	storePath := filepath.Join(dir, "installs.json")
	libraryPath := writeContentFixture(t, dir, "writer.md")
	coord := testCoord("core", "skills", "writer")
	moat := &MOATProvenance{
		ManifestURI: "https://example.com/moat.json",
		SourceURI:   "https://example.com/writer.tgz",
		TrustTier:   "SIGNED",
		AttestedAt:  fixedTime(16),
	}

	s := mustLoadStore(t, storePath)
	rec := testRecord(coord, fixedTime(17))
	rec.MOAT = moat
	s.Upsert(rec)
	mustSaveStore(t, s)

	if err := RecordInstall(storePath, coord, libraryPath, PlacementInput{
		Provider:  "codex",
		Mechanism: MechanismSymlink,
		Path:      "/tmp/codex/writer",
	}, fixedTime(18)); err != nil {
		t.Fatalf("RecordInstall: %v", err)
	}
	got := mustLoadStore(t, storePath).Find(coord)
	if got == nil {
		t.Fatal("record missing")
	}
	if !reflect.DeepEqual(got.MOAT, moat) {
		t.Fatalf("MOAT = %#v, want %#v", got.MOAT, moat)
	}
}

func TestRecordInstallMOATSetsProvenanceOnFreshRecord(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	storePath := filepath.Join(dir, "installs.json")
	libraryPath := writeContentFixture(t, dir, "writer.md")
	coord := testCoord("core", "skills", "writer")
	moat := &MOATProvenance{
		ManifestURI: "https://example.com/moat.json",
		SourceURI:   "https://example.com/writer.tgz",
		TrustTier:   "DUAL-ATTESTED",
		AttestedAt:  fixedTime(19),
	}

	if err := RecordInstallMOAT(storePath, coord, libraryPath, PlacementInput{
		Provider:  "codex",
		Mechanism: MechanismSymlink,
		Path:      "/tmp/codex/writer",
	}, moat, fixedTime(20)); err != nil {
		t.Fatalf("RecordInstallMOAT: %v", err)
	}

	got := mustLoadStore(t, storePath).Find(coord)
	if got == nil {
		t.Fatal("record missing")
	}
	if !reflect.DeepEqual(got.MOAT, moat) {
		t.Fatalf("MOAT = %#v, want %#v", got.MOAT, moat)
	}
}

func TestRecordInstallMOATNilPreservesExistingProvenance(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	storePath := filepath.Join(dir, "installs.json")
	libraryPath := writeContentFixture(t, dir, "writer.md")
	coord := testCoord("core", "skills", "writer")
	moat := &MOATProvenance{
		ManifestURI: "https://example.com/moat.json",
		SourceURI:   "https://example.com/writer.tgz",
		TrustTier:   "SIGNED",
		AttestedAt:  fixedTime(21),
	}

	if err := RecordInstallMOAT(storePath, coord, libraryPath, PlacementInput{
		Provider:  "codex",
		Mechanism: MechanismSymlink,
		Path:      "/tmp/codex/writer",
	}, moat, fixedTime(22)); err != nil {
		t.Fatalf("seed RecordInstallMOAT: %v", err)
	}
	if err := RecordInstallMOAT(storePath, coord, libraryPath, PlacementInput{
		Provider:  "claude-code",
		Mechanism: MechanismSymlink,
		Path:      "/tmp/claude/writer",
	}, nil, fixedTime(23)); err != nil {
		t.Fatalf("RecordInstallMOAT nil provenance: %v", err)
	}

	got := mustLoadStore(t, storePath).Find(coord)
	if got == nil {
		t.Fatal("record missing")
	}
	if !reflect.DeepEqual(got.MOAT, moat) {
		t.Fatalf("MOAT = %#v, want preserved %#v", got.MOAT, moat)
	}
}

func TestRecordInstallMOATReplacesExistingProvenance(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	storePath := filepath.Join(dir, "installs.json")
	libraryPath := writeContentFixture(t, dir, "writer.md")
	coord := testCoord("core", "skills", "writer")
	oldMOAT := &MOATProvenance{
		ManifestURI: "https://example.com/old.json",
		SourceURI:   "https://example.com/old.tgz",
		TrustTier:   "SIGNED",
		AttestedAt:  fixedTime(24),
	}
	newMOAT := &MOATProvenance{
		ManifestURI: "https://example.com/new.json",
		SourceURI:   "https://example.com/new.tgz",
		TrustTier:   "UNSIGNED",
		AttestedAt:  fixedTime(25),
	}

	if err := RecordInstallMOAT(storePath, coord, libraryPath, PlacementInput{
		Provider:  "codex",
		Mechanism: MechanismSymlink,
		Path:      "/tmp/codex/writer",
	}, oldMOAT, fixedTime(26)); err != nil {
		t.Fatalf("seed RecordInstallMOAT: %v", err)
	}
	if err := RecordInstallMOAT(storePath, coord, libraryPath, PlacementInput{
		Provider:  "codex",
		Mechanism: MechanismSymlink,
		Path:      "/tmp/codex/writer",
	}, newMOAT, fixedTime(27)); err != nil {
		t.Fatalf("RecordInstallMOAT replace: %v", err)
	}

	got := mustLoadStore(t, storePath).Find(coord)
	if got == nil {
		t.Fatal("record missing")
	}
	if !reflect.DeepEqual(got.MOAT, newMOAT) {
		t.Fatalf("MOAT = %#v, want replacement %#v", got.MOAT, newMOAT)
	}
}

func providersOf(placements []Placement) []string {
	out := make([]string, 0, len(placements))
	for _, placement := range placements {
		out = append(out, placement.Provider)
	}
	return out
}
