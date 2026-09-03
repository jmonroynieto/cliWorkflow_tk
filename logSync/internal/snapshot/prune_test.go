package snapshot

import (
	"os"
	"path/filepath"
	"testing"
)

// vaultWith builds a real local directory holding the named files, and
// records a baseline for each, so the store and the "vault" agree to start
// with.
func vaultWith(t *testing.T, names ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, n := range names {
		p := filepath.Join(root, n)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("content of "+n+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Write(p, []byte("baseline of "+n+"\n")); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func statesByPath(t *testing.T, root string) map[string]State {
	t.Helper()
	entries, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]State{}
	for _, e := range entries {
		out[e.LocalPath] = e.State
	}
	return out
}

func TestList_EmptyStoreIsNotAnError(t *testing.T) {
	withIsolatedState(t)
	entries, err := List("")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected no entries, got %d", len(entries))
	}
}

// Locate must not bring the store into existence: an inspection command
// that creates what it reports on makes "there is no store" unobservable.
func TestLocate_DoesNotCreateTheStore(t *testing.T) {
	withIsolatedState(t)
	dir, exists, err := Locate()
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("a fresh state home should not already hold a store")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("Locate created %s as a side effect", dir)
	}
}

// A deleted note leaves a baseline whose directory is still there. That is
// the case a sync round may clean up on its own.
func TestState_DeletedFileWithLivingDirectoryIsStale(t *testing.T) {
	withIsolatedState(t)
	root := vaultWith(t, "notes/keep.md", "notes/gone.md")
	if err := os.Remove(filepath.Join(root, "notes/gone.md")); err != nil {
		t.Fatal(err)
	}
	states := statesByPath(t, root)
	if got := states[filepath.Join(root, "notes/keep.md")]; got != Live {
		t.Errorf("surviving note should be live, got %s", got)
	}
	if got := states[filepath.Join(root, "notes/gone.md")]; got != Stale {
		t.Errorf("deleted note should be stale, got %s", got)
	}
}

// A vault on a mount that is not up looks exactly like a deleted one. The
// whole point of the parent-directory test is that this case never gets
// cleaned up without being asked for.
func TestState_MissingDirectoryIsOrphanedNotStale(t *testing.T) {
	withIsolatedState(t)
	root := vaultWith(t, "notes/a.md", "notes/b.md")
	if err := os.RemoveAll(filepath.Join(root, "notes")); err != nil {
		t.Fatal(err)
	}
	for p, st := range statesByPath(t, root) {
		if st != Orphaned {
			t.Errorf("%s: expected orphaned, got %s", p, st)
		}
	}
}

// Inside a root that is present, both a deleted note and a deleted folder
// are deletions, and both are prunable. What survives is whatever the round
// still knows about.
func TestPruneUnder_RemovesDeletedNotesAndDeletedFoldersAlike(t *testing.T) {
	withIsolatedState(t)
	root := vaultWith(t, "notes/gone.md", "notes/kept.md", "archive/old.md")
	if err := os.Remove(filepath.Join(root, "notes/gone.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, "archive")); err != nil {
		t.Fatal(err)
	}

	keep := map[string]struct{}{filepath.Join(root, "notes/kept.md"): {}}
	removed, err := PruneUnder(root, keep)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 2 {
		t.Fatalf("expected the deleted note and the deleted folder's baseline pruned, got %v", removed)
	}
	states := statesByPath(t, root)
	if len(states) != 1 {
		t.Errorf("only the kept note should still have a baseline, got %v", states)
	}
	if states[filepath.Join(root, "notes/kept.md")] != Live {
		t.Error("the surviving note lost its baseline")
	}
}

// The keep set is what stops a prune from eating baselines for files that
// are merely absent from one side this round.
func TestPruneUnder_KeepSetProtectsAnAbsentButKnownFile(t *testing.T) {
	withIsolatedState(t)
	root := vaultWith(t, "notes/on-phone-only.md")
	if err := os.Remove(filepath.Join(root, "notes/on-phone-only.md")); err != nil {
		t.Fatal(err)
	}
	keep := map[string]struct{}{filepath.Join(root, "notes/on-phone-only.md"): {}}
	removed, err := PruneUnder(root, keep)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 0 {
		t.Errorf("a file the round still knows about must keep its baseline, got %v", removed)
	}
}

func TestPruneUnder_StaysInsideItsOwnRoot(t *testing.T) {
	withIsolatedState(t)
	mine := vaultWith(t, "a.md")
	theirs := vaultWith(t, "a.md")
	if err := os.Remove(filepath.Join(mine, "a.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(theirs, "a.md")); err != nil {
		t.Fatal(err)
	}
	removed, err := PruneUnder(mine, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 {
		t.Fatalf("expected one removal inside the given root, got %d", len(removed))
	}
	if states := statesByPath(t, theirs); len(states) != 1 {
		t.Error("a prune reached into a vault it was not given")
	}
}

// Removing the last baseline in a directory has to take the directory too,
// or the store grows in inodes exactly as fast as it used to grow in files.
func TestRemove_TakesEmptiedDirectoriesWithIt(t *testing.T) {
	withIsolatedState(t)
	root := vaultWith(t, "deep/nested/only.md")
	if err := os.Remove(filepath.Join(root, "deep/nested/only.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := PruneUnder(root, nil); err != nil {
		t.Fatal(err)
	}
	storeDir, _, err := Locate()
	if err != nil {
		t.Fatal(err)
	}
	leftover := filepath.Join(storeDir, root, "deep")
	if _, err := os.Stat(leftover); !os.IsNotExist(err) {
		t.Errorf("emptied directory %s was left behind", leftover)
	}
	if _, err := os.Stat(storeDir); err != nil {
		t.Errorf("the store root itself must survive being emptied: %v", err)
	}
}

// Baselines written by the older build used scope-relative keys, which map
// to local paths that were never real. They need no separate migration —
// the same prune reaches them.
func TestList_OldRelativeKeysSurfaceAsRemovable(t *testing.T) {
	withIsolatedState(t)
	storeDir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	// What the old layout wrote: a key relative to whatever was being synced.
	if err := os.WriteFile(filepath.Join(storeDir, "journal.md"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(storeDir, "journal"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(storeDir, "journal", "2026_08.md"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := List("")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected both old-layout entries listed, got %d", len(entries))
	}
	for _, e := range entries {
		if e.State == Live {
			t.Errorf("%s: an old relative key cannot describe a live file", e.LocalPath)
		}
	}
}

// Reorganising a vault — deleting a folder and moving its notes — is
// ordinary. Anchoring the "deleted, or disappeared?" question at each
// file's own parent called all of those baselines orphaned, so they could
// only be cleared by hand and accumulated with every reorganisation.
func TestStaleUnder_ADeletedFolderInsideALivingRootIsPrunable(t *testing.T) {
	withIsolatedState(t)
	root := vaultWith(t, "keep.md", "old-folder/a.md", "old-folder/b.md")
	if err := os.RemoveAll(filepath.Join(root, "old-folder")); err != nil {
		t.Fatal(err)
	}
	dead, err := StaleUnder(root, map[string]struct{}{filepath.Join(root, "keep.md"): {}})
	if err != nil {
		t.Fatal(err)
	}
	if len(dead) != 2 {
		t.Fatalf("expected both baselines under the deleted folder to be prunable, got %d", len(dead))
	}
}

// The case the anchor exists to protect: the vault itself is not there.
func TestStaleUnder_AMissingRootPrunesNothing(t *testing.T) {
	withIsolatedState(t)
	root := vaultWith(t, "notes/a.md", "notes/b.md")
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	dead, err := StaleUnder(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(dead) != 0 {
		t.Errorf("an absent vault root must prune nothing, got %d", len(dead))
	}
	removed, err := PruneUnder(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 0 {
		t.Errorf("PruneUnder must agree, got %d", len(removed))
	}
}
