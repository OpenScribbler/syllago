package installstore

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
)

func fixedTime(hour int) time.Time {
	return time.Date(2026, 8, 22, hour, 0, 0, 0, time.UTC)
}

func testCoord(registry, typ, name string) Coord {
	return Coord{Registry: registry, Type: typ, Name: name}
}

func testRecord(c Coord, installedAt time.Time) Record {
	return Record{
		Coord:       c,
		ContentHash: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		LibraryPath: filepath.ToSlash(filepath.Join(c.Type, c.Name)),
		InstalledAt: installedAt,
		UpdatedAt:   installedAt.Add(time.Hour),
	}
}

func mustLoadStore(t *testing.T, path string) *Store {
	t.Helper()
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load(%s): %v", path, err)
	}
	return s
}

func mustSaveStore(t *testing.T, s *Store) {
	t.Helper()
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	return data
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func TestLoad(t *testing.T) {
	t.Parallel()

	t.Run("missing file returns empty store that can save", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "nested", "installs.json")

		s, err := Load(path)
		if err != nil {
			t.Fatalf("Load missing: %v", err)
		}
		if s.Version != CurrentVersion {
			t.Fatalf("Version = %d, want %d", s.Version, CurrentVersion)
		}
		if len(s.Records) != 0 {
			t.Fatalf("Records length = %d, want 0", len(s.Records))
		}

		if err := s.Save(); err != nil {
			t.Fatalf("Save after missing Load: %v", err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat(%s): %v", path, err)
		}
		if got := info.Mode().Perm(); got != 0o644 {
			t.Fatalf("saved mode = %o, want 0644", got)
		}
	})

	t.Run("corrupt json returns error", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "installs.json")
		writeFile(t, path, []byte(`{"version":`))

		_, err := Load(path)
		if err == nil {
			t.Fatal("Load corrupt JSON returned nil error")
		}
	})

	t.Run("newer version returns error mentioning both versions", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "installs.json")
		writeFile(t, path, []byte(`{"version":99,"records":[]}`))

		_, err := Load(path)
		if err == nil {
			t.Fatal("Load newer version returned nil error")
		}
		msg := err.Error()
		if !strings.Contains(msg, "99") || !strings.Contains(msg, "1") {
			t.Fatalf("error = %q, want both version numbers", msg)
		}
	})
}

func TestStoreRoundTrip(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "state", "installs.json")
	s := mustLoadStore(t, path)
	local := testRecord(testCoord("", "rules", "local-style"), fixedTime(9))
	local.ContentHash = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	local.Placements = []Placement{{
		Provider:    "codex",
		Mechanism:   MechanismRuleAppend,
		Path:        "/tmp/project/AGENTS.md",
		Key:         "syllago:local-style",
		Scope:       "/tmp/project",
		InstalledAt: fixedTime(10),
	}}

	moatRec := testRecord(testCoord("core", "skills", "reviewer"), fixedTime(11))
	moatRec.MOAT = &MOATProvenance{
		ManifestURI: "https://example.com/moat-manifest.json",
		SourceURI:   "https://example.com/reviewer.tar.gz",
		TrustTier:   "SIGNED",
		AttestedAt:  fixedTime(8),
	}
	moatRec.Placements = []Placement{
		{
			Provider:    "claude",
			Mechanism:   MechanismSymlink,
			Path:        "/tmp/home/.claude/skills/reviewer",
			InstalledAt: fixedTime(12),
		},
		{
			Provider:    "codex",
			Mechanism:   MechanismHookMerge,
			Path:        "/tmp/home/.codex/settings.json",
			Key:         "PreToolUse",
			InstalledAt: fixedTime(13),
		},
	}
	s.Records = []Record{local, moatRec}

	mustSaveStore(t, s)
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load round-trip: %v", err)
	}
	if !reflect.DeepEqual(s, loaded) {
		t.Fatalf("round-trip mismatch\n got: %#v\nwant: %#v", loaded, s)
	}
}

func TestStoreRoundTripPinRollbackFieldsDeterministic(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "state", "installs.json")
	coord := testCoord("core", "skills", "writer")
	rec := testRecord(coord, fixedTime(1))
	rec.SourceSHA = "abc123"
	rec.Pinned = true
	rec.PinnedAt = fixedTime(2)
	rec.Previous = &PreviousVersion{
		SourceSHA:   "def456",
		ContentHash: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		CopyPath:    filepath.Join(t.TempDir(), "rollback", "writer"),
		ReplacedAt:  fixedTime(3),
	}

	s := mustLoadStore(t, path)
	s.Records = []Record{rec}
	mustSaveStore(t, s)
	firstBytes := mustReadFile(t, path)

	loaded := mustLoadStore(t, path)
	got := loaded.Find(coord)
	if got == nil {
		t.Fatal("record missing after load")
	}
	if !reflect.DeepEqual(got, &rec) {
		t.Fatalf("new fields round-trip mismatch\n got: %#v\nwant: %#v", got, &rec)
	}

	mustSaveStore(t, loaded)
	if secondBytes := mustReadFile(t, path); !bytes.Equal(secondBytes, firstBytes) {
		t.Fatalf("second save changed bytes\nfirst:\n%s\nsecond:\n%s", firstBytes, secondBytes)
	}
}

func TestLoadOldStoreCompatNewFieldsZero(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "installs.json")
	writeFile(t, path, []byte(`{
  "version": 1,
  "records": [
    {
      "registry": "core",
      "type": "skills",
      "name": "writer",
      "content_hash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "library_path": "skills/writer",
      "installed_at": "2026-08-22T01:00:00Z",
      "updated_at": "2026-08-22T02:00:00Z"
    }
  ]
}`))

	s := mustLoadStore(t, path)
	rec := s.Find(testCoord("core", "skills", "writer"))
	if rec == nil {
		t.Fatal("record missing")
	}
	if rec.SourceSHA != "" {
		t.Fatalf("SourceSHA = %q, want empty", rec.SourceSHA)
	}
	if rec.Pinned {
		t.Fatal("Pinned = true, want false")
	}
	if !rec.PinnedAt.IsZero() {
		t.Fatalf("PinnedAt = %v, want zero", rec.PinnedAt)
	}
	if rec.Previous != nil {
		t.Fatalf("Previous = %#v, want nil", rec.Previous)
	}
}

func TestStoreSaveDeterministic(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.json")
	pathB := filepath.Join(dir, "b.json")

	coordA := testCoord("zeta", "skills", "z")
	coordB := testCoord("", "rules", "local")
	coordC := testCoord("alpha", "commands", "build")

	recA := testRecord(coordA, fixedTime(1))
	recA.Placements = []Placement{
		{Provider: "codex", Mechanism: MechanismMCPMerge, Path: "/settings.json", Key: "server-b", InstalledAt: fixedTime(4)},
		{Provider: "claude", Mechanism: MechanismSymlink, Path: "/skills/z", InstalledAt: fixedTime(3)},
		{Provider: "codex", Mechanism: MechanismMCPMerge, Path: "/settings.json", Key: "server-a", InstalledAt: fixedTime(2)},
	}
	recB := testRecord(coordB, fixedTime(5))
	recC := testRecord(coordC, fixedTime(6))

	first := mustLoadStore(t, pathA)
	first.Records = []Record{recA, recC, recB}
	mustSaveStore(t, first)

	second := mustLoadStore(t, pathB)
	recAAlt := recA
	recAAlt.Placements = []Placement{recA.Placements[2], recA.Placements[0], recA.Placements[1]}
	second.Records = []Record{recB, recAAlt, recC}
	mustSaveStore(t, second)

	if a, b := mustReadFile(t, pathA), mustReadFile(t, pathB); !bytes.Equal(a, b) {
		t.Fatalf("saved bytes differ\nA:\n%s\nB:\n%s", a, b)
	}
}

func TestStoreSaveDefensiveBranches(t *testing.T) {
	t.Parallel()

	var nilStore *Store
	if err := nilStore.Save(); err == nil {
		t.Fatal("nil Store Save returned nil error")
	}

	if err := (&Store{Version: CurrentVersion}).Save(); err == nil {
		t.Fatal("empty-path Store Save returned nil error")
	}

	path := filepath.Join(t.TempDir(), "installs.json")
	s := mustLoadStore(t, path)
	s.Version = 0
	s.Records = nil
	mustSaveStore(t, s)

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load saved zero-version store: %v", err)
	}
	if loaded.Version != CurrentVersion {
		t.Fatalf("Version = %d, want %d", loaded.Version, CurrentVersion)
	}
	if len(loaded.Records) != 0 {
		t.Fatalf("Records length = %d, want 0", len(loaded.Records))
	}
}

func TestStoreUpsert(t *testing.T) {
	t.Parallel()

	c := testCoord("core", "skills", "writer")
	s := &Store{Version: CurrentVersion}

	inserted := testRecord(c, fixedTime(1))
	s.Upsert(inserted)
	if got := s.Find(c); got == nil || got.ContentHash != inserted.ContentHash {
		t.Fatalf("fresh insert not found: %#v", got)
	}

	replacement := testRecord(c, time.Time{})
	replacement.ContentHash = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	replacement.UpdatedAt = fixedTime(4)
	s.Upsert(replacement)
	got := s.Find(c)
	if got == nil {
		t.Fatal("replacement missing")
	}
	if got.InstalledAt != inserted.InstalledAt {
		t.Fatalf("InstalledAt = %v, want preserved %v", got.InstalledAt, inserted.InstalledAt)
	}
	if got.ContentHash != replacement.ContentHash {
		t.Fatalf("ContentHash = %s, want %s", got.ContentHash, replacement.ContentHash)
	}

	override := testRecord(c, fixedTime(8))
	s.Upsert(override)
	got = s.Find(c)
	if got == nil {
		t.Fatal("override missing")
	}
	if got.InstalledAt != override.InstalledAt {
		t.Fatalf("InstalledAt = %v, want override %v", got.InstalledAt, override.InstalledAt)
	}
}

func TestStoreFindAndRemove(t *testing.T) {
	t.Parallel()

	c := testCoord("core", "skills", "writer")
	missing := testCoord("core", "skills", "missing")
	s := &Store{Version: CurrentVersion, Records: []Record{testRecord(c, fixedTime(1))}}

	got := s.Find(c)
	if got == nil {
		t.Fatal("Find present returned nil")
	}
	got.ContentHash = "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	if s.Records[0].ContentHash != got.ContentHash {
		t.Fatal("Find did not return a pointer into the store slice")
	}
	if got := s.Find(missing); got != nil {
		t.Fatalf("Find missing = %#v, want nil", got)
	}

	if !s.Remove(c) {
		t.Fatal("Remove present = false, want true")
	}
	if len(s.Records) != 0 {
		t.Fatalf("Records length after remove = %d, want 0", len(s.Records))
	}
	if s.Remove(missing) {
		t.Fatal("Remove missing = true, want false")
	}
}

func TestStoreAddPlacement(t *testing.T) {
	t.Parallel()

	c := testCoord("core", "skills", "writer")
	missing := testCoord("core", "skills", "missing")
	s := &Store{Version: CurrentVersion, Records: []Record{testRecord(c, fixedTime(1))}}

	err := s.AddPlacement(missing, Placement{Provider: "codex", Mechanism: MechanismSymlink, Path: "/x"})
	if err == nil {
		t.Fatal("AddPlacement missing record returned nil error")
	}
	for _, want := range []string{"core", "skills", "missing"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("missing-record error = %q, want substring %q", err.Error(), want)
		}
	}

	p := Placement{
		Provider:    "codex",
		Mechanism:   MechanismHookMerge,
		Path:        "/tmp/settings.json",
		Key:         "PreToolUse",
		InstalledAt: fixedTime(2),
	}
	if err := s.AddPlacement(c, p); err != nil {
		t.Fatalf("AddPlacement first: %v", err)
	}
	replacement := p
	replacement.Scope = "/tmp/project"
	replacement.InstalledAt = fixedTime(3)
	if err := s.AddPlacement(c, replacement); err != nil {
		t.Fatalf("AddPlacement replacement: %v", err)
	}
	rec := s.Find(c)
	if rec == nil {
		t.Fatal("record missing after placement add")
	}
	if len(rec.Placements) != 1 {
		t.Fatalf("Placements length = %d, want 1", len(rec.Placements))
	}
	if rec.Placements[0].Scope != replacement.Scope || rec.Placements[0].InstalledAt != replacement.InstalledAt {
		t.Fatalf("placement was not replaced: %#v", rec.Placements[0])
	}

	distinctKey := replacement
	distinctKey.Key = "PostToolUse"
	if err := s.AddPlacement(c, distinctKey); err != nil {
		t.Fatalf("AddPlacement distinct key: %v", err)
	}
	if len(rec.Placements) != 2 {
		t.Fatalf("Placements length after distinct key = %d, want 2", len(rec.Placements))
	}
}

func TestStoreRemovePlacement(t *testing.T) {
	t.Parallel()

	c := testCoord("core", "skills", "writer")
	p1 := Placement{Provider: "codex", Mechanism: MechanismHookMerge, Path: "/tmp/settings.json", Key: "PreToolUse", InstalledAt: fixedTime(2)}
	p2 := Placement{Provider: "codex", Mechanism: MechanismHookMerge, Path: "/tmp/settings.json", Key: "PostToolUse", InstalledAt: fixedTime(3)}
	s := &Store{Version: CurrentVersion, Records: []Record{testRecord(c, fixedTime(1))}}
	s.Records[0].Placements = []Placement{p1, p2}

	if !s.RemovePlacement(c, p1.Provider, p1.Mechanism, p1.Path, p1.Key) {
		t.Fatal("RemovePlacement present = false, want true")
	}
	if len(s.Records[0].Placements) != 1 || s.Records[0].Placements[0].Key != p2.Key {
		t.Fatalf("placements after first remove = %#v", s.Records[0].Placements)
	}
	if s.RemovePlacement(c, p1.Provider, p1.Mechanism, p1.Path, p1.Key) {
		t.Fatal("RemovePlacement absent = true, want false")
	}
	if !s.RemovePlacement(c, p2.Provider, p2.Mechanism, p2.Path, p2.Key) {
		t.Fatal("RemovePlacement last = false, want true")
	}
	if rec := s.Find(c); rec == nil {
		t.Fatal("record was removed with last placement")
	} else if len(rec.Placements) != 0 {
		t.Fatalf("placements length = %d, want 0", len(rec.Placements))
	}
	if s.RemovePlacement(testCoord("core", "skills", "missing"), p2.Provider, p2.Mechanism, p2.Path, p2.Key) {
		t.Fatal("RemovePlacement missing record = true, want false")
	}
}

func TestStoreQueries(t *testing.T) {
	t.Parallel()

	local := testRecord(testCoord("", "rules", "local"), fixedTime(1))
	local.Placements = []Placement{{Provider: "codex", Mechanism: MechanismRuleAppend, Path: "/tmp/AGENTS.md"}}
	core := testRecord(testCoord("core", "skills", "writer"), fixedTime(2))
	core.MOAT = &MOATProvenance{
		ManifestURI: "https://example.com/moat.json",
		SourceURI:   "https://example.com/writer.tgz",
		TrustTier:   "SIGNED",
	}
	core.Placements = []Placement{
		{Provider: "codex", Mechanism: MechanismSymlink, Path: "/tmp/codex/writer"},
		{Provider: "claude", Mechanism: MechanismSymlink, Path: "/tmp/claude/writer"},
	}
	other := testRecord(testCoord("other", "commands", "build"), fixedTime(3))
	other.Placements = []Placement{{Provider: "claude", Mechanism: MechanismSymlink, Path: "/tmp/claude/build"}}
	unplaced := testRecord(testCoord("core", "rules", "empty"), fixedTime(4))
	s := &Store{Version: CurrentVersion, Records: []Record{local, core, other, unplaced}}

	cases := []struct {
		name string
		got  []Record
		want []Coord
	}{
		{"by provider codex", s.ByProvider("codex"), []Coord{local.Coord, core.Coord}},
		{"by provider missing", s.ByProvider("missing"), nil},
		{"by registry core", s.ByRegistry("core"), []Coord{core.Coord, unplaced.Coord}},
		{"by registry local", s.ByRegistry(""), []Coord{local.Coord}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := coordsOf(tc.got); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("coords = %#v, want %#v", got, tc.want)
			}
		})
	}

	results := s.ByRegistry("core")
	if len(results) != 2 {
		t.Fatalf("ByRegistry(core) length = %d, want 2", len(results))
	}
	results[0].MOAT.TrustTier = "MUTATED"
	results[0].Placements[0].Path = "/mutated"
	if s.Records[1].MOAT.TrustTier != "SIGNED" {
		t.Fatalf("ByRegistry returned aliased MOAT pointer")
	}
	if s.Records[1].Placements[0].Path == "/mutated" {
		t.Fatalf("ByRegistry returned aliased placements slice")
	}
}

func TestDefaultPathHonorsGlobalDirOverride(t *testing.T) {
	prev := config.GlobalDirOverride
	config.GlobalDirOverride = t.TempDir()
	t.Cleanup(func() {
		config.GlobalDirOverride = prev
	})

	want := filepath.Join(config.GlobalDirOverride, "installs.json")
	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error: %v", err)
	}
	if got != want {
		t.Fatalf("DefaultPath() = %s, want %s", got, want)
	}
}

func coordsOf(records []Record) []Coord {
	if len(records) == 0 {
		return nil
	}
	coords := make([]Coord, 0, len(records))
	for _, rec := range records {
		coords = append(coords, rec.Coord)
	}
	return coords
}
