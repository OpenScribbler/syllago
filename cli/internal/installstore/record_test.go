package installstore

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeContentFixture(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	writeFile(t, path, []byte("# "+name+"\n"))
	return path
}

func mustHashContent(t *testing.T, path string) string {
	t.Helper()
	hash, err := HashContent(path)
	if err != nil {
		t.Fatalf("HashContent(%s): %v", path, err)
	}
	return hash
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

func TestRecordInstallMetaSourceSHAAndPreservesExistingState(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	storePath := filepath.Join(dir, "installs.json")
	libraryPath := writeContentFixture(t, dir, "writer.md")
	coord := testCoord("core", "skills", "writer")
	input := PlacementInput{Provider: "codex", Mechanism: MechanismSymlink, Path: "/tmp/codex/writer"}

	if err := RecordInstallMeta(storePath, coord, libraryPath, input, InstallMeta{SourceSHA: "sha-new"}, fixedTime(28)); err != nil {
		t.Fatalf("RecordInstallMeta fresh: %v", err)
	}
	rec := mustLoadStore(t, storePath).Find(coord)
	if rec == nil {
		t.Fatal("record missing")
	}
	if rec.SourceSHA != "sha-new" {
		t.Fatalf("SourceSHA = %q, want sha-new", rec.SourceSHA)
	}

	pinnedAt := fixedTime(29)
	prev := &PreviousVersion{
		SourceSHA:   "sha-prev",
		ContentHash: rec.ContentHash,
		CopyPath:    filepath.Join(dir, "rollback", "writer"),
		ReplacedAt:  fixedTime(30),
	}
	s := mustLoadStore(t, storePath)
	rec = s.Find(coord)
	rec.SourceSHA = "sha-existing"
	rec.Pinned = true
	rec.PinnedAt = pinnedAt
	rec.Previous = prev
	mustSaveStore(t, s)

	if err := RecordInstallMeta(storePath, coord, libraryPath, PlacementInput{
		Provider:  "claude-code",
		Mechanism: MechanismSymlink,
		Path:      "/tmp/claude/writer",
	}, InstallMeta{}, fixedTime(31)); err != nil {
		t.Fatalf("RecordInstallMeta reinstall: %v", err)
	}

	got := mustLoadStore(t, storePath).Find(coord)
	if got == nil {
		t.Fatal("record missing after reinstall")
	}
	if got.SourceSHA != "sha-existing" {
		t.Fatalf("SourceSHA = %q, want preserved sha-existing", got.SourceSHA)
	}
	if !got.Pinned || got.PinnedAt != pinnedAt {
		t.Fatalf("pin state = pinned %v at %v, want true at %v", got.Pinned, got.PinnedAt, pinnedAt)
	}
	if !reflect.DeepEqual(got.Previous, prev) {
		t.Fatalf("Previous = %#v, want preserved %#v", got.Previous, prev)
	}
}

func TestRecordInstallMetaHashFailureLeavesStoreUntouched(t *testing.T) {
	t.Parallel()

	storePath := filepath.Join(t.TempDir(), "installs.json")
	err := RecordInstallMeta(storePath, testCoord("core", "skills", "missing"), filepath.Join(t.TempDir(), "missing.md"), PlacementInput{
		Provider:  "codex",
		Mechanism: MechanismSymlink,
		Path:      "/tmp/codex/missing",
	}, InstallMeta{SourceSHA: "sha-new"}, fixedTime(32))
	if err == nil {
		t.Fatal("RecordInstallMeta missing libraryPath returned nil error")
	}
	assertStoreFileMissing(t, storePath)
}

func TestSetPinnedSetsClearsAndNoOps(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	storePath := filepath.Join(dir, "installs.json")
	libraryPath := writeContentFixture(t, dir, "writer.md")
	coord := testCoord("core", "skills", "writer")
	input := PlacementInput{Provider: "codex", Mechanism: MechanismSymlink, Path: "/tmp/codex/writer"}
	if err := RecordInstall(storePath, coord, libraryPath, input, fixedTime(33)); err != nil {
		t.Fatalf("RecordInstall: %v", err)
	}

	pinnedAt := fixedTime(34)
	if err := SetPinned(storePath, coord, true, pinnedAt); err != nil {
		t.Fatalf("SetPinned pin: %v", err)
	}
	rec := mustLoadStore(t, storePath).Find(coord)
	if rec == nil {
		t.Fatal("record missing")
	}
	if !rec.Pinned || rec.PinnedAt != pinnedAt || rec.UpdatedAt != pinnedAt {
		t.Fatalf("pin fields = pinned %v at %v updated %v, want true at/updated %v", rec.Pinned, rec.PinnedAt, rec.UpdatedAt, pinnedAt)
	}

	pinnedBytes := mustReadFile(t, storePath)
	if err := SetPinned(storePath, coord, true, fixedTime(35)); err != nil {
		t.Fatalf("SetPinned already pinned: %v", err)
	}
	if after := mustReadFile(t, storePath); !reflect.DeepEqual(after, pinnedBytes) {
		t.Fatalf("already-pinned no-op rewrote store\nbefore:\n%s\nafter:\n%s", pinnedBytes, after)
	}

	unpinnedAt := fixedTime(36)
	if err := SetPinned(storePath, coord, false, unpinnedAt); err != nil {
		t.Fatalf("SetPinned unpin: %v", err)
	}
	rec = mustLoadStore(t, storePath).Find(coord)
	if rec == nil {
		t.Fatal("record missing after unpin")
	}
	if rec.Pinned || !rec.PinnedAt.IsZero() || rec.UpdatedAt != unpinnedAt {
		t.Fatalf("unpin fields = pinned %v at %v updated %v, want false zero updated %v", rec.Pinned, rec.PinnedAt, rec.UpdatedAt, unpinnedAt)
	}
}

func TestSetPinnedMissingStoreAndRecordErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	coord := testCoord("core", "skills", "writer")
	missingStore := filepath.Join(dir, "missing", "installs.json")
	if err := SetPinned(missingStore, coord, true, fixedTime(37)); err == nil {
		t.Fatal("SetPinned missing store returned nil error")
	}
	assertStoreFileMissing(t, missingStore)

	storePath := filepath.Join(dir, "installs.json")
	s := mustLoadStore(t, storePath)
	s.Upsert(testRecord(testCoord("core", "skills", "other"), fixedTime(38)))
	mustSaveStore(t, s)
	if err := SetPinned(storePath, coord, true, fixedTime(39)); err == nil {
		t.Fatal("SetPinned missing record returned nil error")
	}
}

func TestRecordUpdateRotatesPreviousAndRehashes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	storePath := filepath.Join(dir, "installs.json")
	libraryPath := filepath.Join(dir, "writer.md")
	writeFile(t, libraryPath, []byte("# writer\n"))
	coord := testCoord("core", "skills", "writer")
	input := PlacementInput{Provider: "codex", Mechanism: MechanismSymlink, Path: "/tmp/codex/writer"}
	if err := RecordInstallMeta(storePath, coord, libraryPath, input, InstallMeta{SourceSHA: "sha-old"}, fixedTime(40)); err != nil {
		t.Fatalf("RecordInstallMeta: %v", err)
	}
	oldHash := mustLoadStore(t, storePath).Find(coord).ContentHash

	writeFile(t, libraryPath, []byte("# writer updated\n"))
	firstUpdate := fixedTime(41)
	if err := RecordUpdate(storePath, coord, libraryPath, "sha-new", filepath.Join(dir, "prev1"), firstUpdate); err != nil {
		t.Fatalf("RecordUpdate first: %v", err)
	}
	rec := mustLoadStore(t, storePath).Find(coord)
	if rec == nil {
		t.Fatal("record missing")
	}
	firstUpdateHash := mustHashContent(t, libraryPath)
	if rec.SourceSHA != "sha-new" || rec.ContentHash != firstUpdateHash || rec.UpdatedAt != firstUpdate {
		t.Fatalf("updated record = %#v, want source sha-new hash %s updated %v", rec, firstUpdateHash, firstUpdate)
	}
	wantPrev := &PreviousVersion{
		SourceSHA:   "sha-old",
		ContentHash: oldHash,
		CopyPath:    filepath.Join(dir, "prev1"),
		ReplacedAt:  firstUpdate,
	}
	if !reflect.DeepEqual(rec.Previous, wantPrev) {
		t.Fatalf("Previous after first update = %#v, want %#v", rec.Previous, wantPrev)
	}

	writeFile(t, libraryPath, []byte("# writer updated again\n"))
	secondUpdate := fixedTime(42)
	if err := RecordUpdate(storePath, coord, libraryPath, "sha-newer", filepath.Join(dir, "prev2"), secondUpdate); err != nil {
		t.Fatalf("RecordUpdate second: %v", err)
	}
	rec = mustLoadStore(t, storePath).Find(coord)
	if rec == nil {
		t.Fatal("record missing after second update")
	}
	wantPrev = &PreviousVersion{
		SourceSHA:   "sha-new",
		ContentHash: firstUpdateHash,
		CopyPath:    filepath.Join(dir, "prev2"),
		ReplacedAt:  secondUpdate,
	}
	if !reflect.DeepEqual(rec.Previous, wantPrev) {
		t.Fatalf("Previous after second update = %#v, want %#v", rec.Previous, wantPrev)
	}
	if rec.SourceSHA != "sha-newer" || rec.ContentHash != mustHashContent(t, libraryPath) {
		t.Fatalf("record after second update = %#v", rec)
	}
}

func TestRecordUpdateErrPinnedAndMissingErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	coord := testCoord("core", "skills", "writer")
	missingStore := filepath.Join(dir, "missing", "installs.json")
	if err := RecordUpdate(missingStore, coord, filepath.Join(dir, "writer.md"), "sha-new", "", fixedTime(43)); err == nil {
		t.Fatal("RecordUpdate missing store returned nil error")
	}

	storePath := filepath.Join(dir, "installs.json")
	s := mustLoadStore(t, storePath)
	s.Upsert(testRecord(testCoord("core", "skills", "other"), fixedTime(44)))
	mustSaveStore(t, s)
	if err := RecordUpdate(storePath, coord, filepath.Join(dir, "writer.md"), "sha-new", "", fixedTime(45)); err == nil {
		t.Fatal("RecordUpdate missing record returned nil error")
	}

	libraryPath := writeContentFixture(t, dir, "writer.md")
	if err := RecordInstallMeta(storePath, coord, libraryPath, PlacementInput{
		Provider:  "codex",
		Mechanism: MechanismSymlink,
		Path:      "/tmp/codex/writer",
	}, InstallMeta{SourceSHA: "sha-old"}, fixedTime(46)); err != nil {
		t.Fatalf("RecordInstallMeta: %v", err)
	}
	if err := SetPinned(storePath, coord, true, fixedTime(47)); err != nil {
		t.Fatalf("SetPinned: %v", err)
	}
	beforePinned := mustReadFile(t, storePath)
	err := RecordUpdate(storePath, coord, libraryPath, "sha-new", "", fixedTime(48))
	if !errors.Is(err, ErrPinned) {
		t.Fatalf("RecordUpdate pinned error = %v, want errors.Is ErrPinned", err)
	}
	if after := mustReadFile(t, storePath); !reflect.DeepEqual(after, beforePinned) {
		t.Fatalf("pinned update rewrote store\nbefore:\n%s\nafter:\n%s", beforePinned, after)
	}
}

func TestRecordUpdateHashFailureWritesNothing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	storePath := filepath.Join(dir, "installs.json")
	libraryPath := writeContentFixture(t, dir, "writer.md")
	coord := testCoord("core", "skills", "writer")
	if err := RecordInstallMeta(storePath, coord, libraryPath, PlacementInput{
		Provider:  "codex",
		Mechanism: MechanismSymlink,
		Path:      "/tmp/codex/writer",
	}, InstallMeta{SourceSHA: "sha-old"}, fixedTime(49)); err != nil {
		t.Fatalf("RecordInstallMeta: %v", err)
	}
	before := mustReadFile(t, storePath)

	err := RecordUpdate(storePath, coord, filepath.Join(dir, "missing.md"), "sha-new", filepath.Join(dir, "prev"), fixedTime(50))
	if err == nil {
		t.Fatal("RecordUpdate missing libraryPath returned nil error")
	}
	if after := mustReadFile(t, storePath); !reflect.DeepEqual(after, before) {
		t.Fatalf("hash failure rewrote store\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestRecordRollbackRestoresSourceSHAAndClearsPrevious(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	storePath := filepath.Join(dir, "installs.json")
	libraryPath := filepath.Join(dir, "writer.md")
	oldContent := []byte("# writer\n")
	writeFile(t, libraryPath, oldContent)
	coord := testCoord("core", "skills", "writer")
	if err := RecordInstallMeta(storePath, coord, libraryPath, PlacementInput{
		Provider:  "codex",
		Mechanism: MechanismSymlink,
		Path:      "/tmp/codex/writer",
	}, InstallMeta{SourceSHA: "sha-old"}, fixedTime(51)); err != nil {
		t.Fatalf("RecordInstallMeta: %v", err)
	}

	writeFile(t, libraryPath, []byte("# writer updated\n"))
	if err := RecordUpdate(storePath, coord, libraryPath, "sha-new", filepath.Join(dir, "prev"), fixedTime(52)); err != nil {
		t.Fatalf("RecordUpdate: %v", err)
	}

	writeFile(t, libraryPath, oldContent)
	rolledAt := fixedTime(53)
	if err := RecordRollback(storePath, coord, libraryPath, rolledAt); err != nil {
		t.Fatalf("RecordRollback: %v", err)
	}
	rec := mustLoadStore(t, storePath).Find(coord)
	if rec == nil {
		t.Fatal("record missing")
	}
	if rec.SourceSHA != "sha-old" {
		t.Fatalf("SourceSHA = %q, want sha-old", rec.SourceSHA)
	}
	if rec.ContentHash != mustHashContent(t, libraryPath) {
		t.Fatalf("ContentHash = %q, want current restored hash", rec.ContentHash)
	}
	if rec.Previous != nil {
		t.Fatalf("Previous = %#v, want nil", rec.Previous)
	}
	if rec.UpdatedAt != rolledAt {
		t.Fatalf("UpdatedAt = %v, want %v", rec.UpdatedAt, rolledAt)
	}
}

func TestRecordRollbackErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	coord := testCoord("core", "skills", "writer")
	missingStore := filepath.Join(dir, "missing", "installs.json")
	if err := RecordRollback(missingStore, coord, filepath.Join(dir, "writer.md"), fixedTime(54)); err == nil {
		t.Fatal("RecordRollback missing store returned nil error")
	}

	storePath := filepath.Join(dir, "installs.json")
	libraryPath := writeContentFixture(t, dir, "writer.md")
	s := mustLoadStore(t, storePath)
	s.Upsert(testRecord(testCoord("core", "skills", "other"), fixedTime(55)))
	mustSaveStore(t, s)
	if err := RecordRollback(storePath, coord, libraryPath, fixedTime(56)); err == nil {
		t.Fatal("RecordRollback missing record returned nil error")
	}

	if err := RecordInstallMeta(storePath, coord, libraryPath, PlacementInput{
		Provider:  "codex",
		Mechanism: MechanismSymlink,
		Path:      "/tmp/codex/writer",
	}, InstallMeta{SourceSHA: "sha-old"}, fixedTime(57)); err != nil {
		t.Fatalf("RecordInstallMeta: %v", err)
	}
	err := RecordRollback(storePath, coord, libraryPath, fixedTime(58))
	if err == nil || !strings.Contains(err.Error(), "no rollback data") {
		t.Fatalf("RecordRollback no previous error = %v, want no rollback data", err)
	}

	s = mustLoadStore(t, storePath)
	rec := s.Find(coord)
	rec.Previous = &PreviousVersion{
		SourceSHA:   "sha-prev",
		ContentHash: rec.ContentHash,
		CopyPath:    filepath.Join(dir, "prev"),
		ReplacedAt:  fixedTime(59),
	}
	mustSaveStore(t, s)
	before := mustReadFile(t, storePath)
	err = RecordRollback(storePath, coord, filepath.Join(dir, "gone.md"), fixedTime(60))
	if err == nil {
		t.Fatal("RecordRollback missing libraryPath returned nil error")
	}
	if after := mustReadFile(t, storePath); !reflect.DeepEqual(after, before) {
		t.Fatalf("rollback hash failure rewrote store\nbefore:\n%s\nafter:\n%s", before, after)
	}
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
