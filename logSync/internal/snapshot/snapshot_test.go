package snapshot

import (
	"os"
	"path/filepath"
	"testing"
)

func withIsolatedState(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
}

// vaultRoot gives a test its own real directory to key snapshots against.
// The store keys on a file's absolute path, so a test needs one — but it
// should be a path that exists on the machine running the test, not an
// invented home directory standing in for somebody's.
func vaultRoot(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func TestReadMissingReturnsNotFoundNoError(t *testing.T) {
	withIsolatedState(t)
	root := vaultRoot(t)
	_, found, err := Read(filepath.Join(root, "journal", "2026_08.md"))
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Error("expected found=false for a path never written")
	}
}

func TestWriteThenReadRoundTrips(t *testing.T) {
	withIsolatedState(t)
	root := vaultRoot(t)
	note := filepath.Join(root, "journal", "2026_08.md")
	want := []byte("# Log\n- shared\n")
	if err := Write(note, want); err != nil {
		t.Fatal(err)
	}
	got, found, err := Read(note)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected found=true after Write")
	}
	if string(got) != string(want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWriteOverwritesPreviousSnapshot(t *testing.T) {
	withIsolatedState(t)
	note := filepath.Join(vaultRoot(t), "a.md")
	if err := Write(note, []byte("v1")); err != nil {
		t.Fatal(err)
	}
	if err := Write(note, []byte("v2")); err != nil {
		t.Fatal(err)
	}
	got, _, err := Read(note)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "v2" {
		t.Errorf("got %q, want v2", got)
	}
}

func TestDirIsIsolatedFromRealVaultContent(t *testing.T) {
	withIsolatedState(t)
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(filepath.Dir(dir)) != "logSync" {
		t.Errorf("expected snapshot dir under a logSync-named directory, got %s", dir)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Errorf("expected %s to be a directory", dir)
	}
}

func TestReadRejectsRelativeKey(t *testing.T) {
	withIsolatedState(t)
	if _, _, err := Read("journal/2026_08.md"); err == nil {
		t.Error("expected a relative key to be refused, since it cannot identify a file across vaults or scopes")
	}
}

// The same note reached from two scopes — vault scope, where notesync sees
// it as "journal/entry.md" under the vault root, and merge scope, where
// sync sees it as "entry.md" under the journal directory — has to land on
// one snapshot. Otherwise seeding a device with notesync leaves sync with
// no ancestor and its first merge duplicates whatever the two sides shared.
func TestSameFileFromTwoScopesSharesOneSnapshot(t *testing.T) {
	withIsolatedState(t)
	root := vaultRoot(t)
	vaultScope := filepath.Join(root, "journal/entry.md")
	mergeScope := filepath.Join(root, "journal", "entry.md")

	if err := Write(vaultScope, []byte("seeded by notesync\n")); err != nil {
		t.Fatal(err)
	}
	got, found, err := Read(mergeScope)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("merge scope could not see the ancestor vault scope recorded")
	}
	if string(got) != "seeded by notesync\n" {
		t.Errorf("got %q", got)
	}
}

// Two vaults holding a note at the same relative path must not share one
// snapshot; before keys became absolute they silently overwrote each other.
func TestDifferentVaultsDoNotShareASnapshot(t *testing.T) {
	withIsolatedState(t)
	inA := filepath.Join(vaultRoot(t), "notes/idea.md")
	inB := filepath.Join(vaultRoot(t), "notes/idea.md")
	if err := Write(inA, []byte("from A\n")); err != nil {
		t.Fatal(err)
	}
	if err := Write(inB, []byte("from B\n")); err != nil {
		t.Fatal(err)
	}
	a, _, err := Read(inA)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != "from A\n" {
		t.Errorf("vault A's snapshot was overwritten by vault B: got %q", a)
	}
}
