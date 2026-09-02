package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jmonroynieto/cliWorkflow_tk/logSync/internal/snapshot"
	"github.com/jmonroynieto/cliWorkflow_tk/logSync/internal/synctree"
)

// A file the device listed but would not hand over is absent from the
// staged tree, which is indistinguishable from a file deleted on the phone
// unless the failure is carried forward. Left uncorrected it means a
// transient read error gets read as a deletion — and with a tracking repo,
// a prompt asking whether to respect a deletion that never happened.
func TestReport_StagingFailureIsNotTreatedAsAPhoneSideDeletion(t *testing.T) {
	s := &session{
		localDir:    t.TempDir(),
		stageFailed: map[string]struct{}{"unreadable-on-phone.md": {}},
	}
	c := synctree.Classification{
		LocalOnly: []string{"genuinely-new.md", "unreadable-on-phone.md"},
	}
	got := s.report(c)
	if len(got.LocalOnly) != 1 || got.LocalOnly[0] != "genuinely-new.md" {
		t.Errorf("expected only the genuinely new file left in LocalOnly, got %v", got.LocalOnly)
	}
}

func TestReport_LeavesLocalOnlyAloneWhenNothingFailedToStage(t *testing.T) {
	s := &session{localDir: t.TempDir()}
	c := synctree.Classification{LocalOnly: []string{"a.md", "b.md"}}
	if got := s.report(c); len(got.LocalOnly) != 2 {
		t.Errorf("expected both paths kept, got %v", got.LocalOnly)
	}
}

// The keep set has to include paths whose phone-side state is unknown, or a
// staging failure costs the file its merge baseline as well.
func TestPruneSnapshots_KeepsBaselinesForPathsThatFailedToStage(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	local := t.TempDir()
	s := &session{
		localDir:    local,
		stageFailed: map[string]struct{}{"flaky.md": {}},
	}
	// The file exists locally but neither walk reported it this round.
	if err := os.WriteFile(filepath.Join(local, "flaky.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := snapshot.Write(filepath.Join(local, "flaky.md"), []byte("baseline\n")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(local, "flaky.md")); err != nil {
		t.Fatal(err)
	}

	s.pruneSnapshots(synctree.Classification{}, true)

	if _, found, err := snapshot.Read(filepath.Join(local, "flaky.md")); err != nil {
		t.Fatal(err)
	} else if !found {
		t.Error("a path whose phone-side state is unknown lost its baseline")
	}
}

func TestPruneSnapshots_PreviewRemovesNothing(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	local := t.TempDir()
	s := &session{localDir: local}
	if err := snapshot.Write(filepath.Join(local, "gone.md"), []byte("baseline\n")); err != nil {
		t.Fatal(err)
	}

	s.pruneSnapshots(synctree.Classification{}, false)

	if _, found, err := snapshot.Read(filepath.Join(local, "gone.md")); err != nil {
		t.Fatal(err)
	} else if !found {
		t.Error("a preview must not remove anything")
	}

	s.pruneSnapshots(synctree.Classification{}, true)

	if _, found, err := snapshot.Read(filepath.Join(local, "gone.md")); err != nil {
		t.Fatal(err)
	} else if found {
		t.Error("a committed run should have dropped the dead baseline")
	}
}
