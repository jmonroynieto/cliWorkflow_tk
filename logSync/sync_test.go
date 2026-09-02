package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmonroynieto/cliWorkflow_tk/logSync/internal/merge"
	"github.com/jmonroynieto/cliWorkflow_tk/logSync/internal/snapshot"
)

func TestPreviewContent_FrontmatterAndParagraph(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.md")
	content := "---\ndateUpdated: 2026-08-01\ntitle: x\n---\nfirst paragraph line one\nfirst paragraph line two\n\nsecond paragraph should be cut\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got := previewContent(path)
	if !strings.Contains(got, "dateUpdated: 2026-08-01") {
		t.Errorf("expected full frontmatter in preview, got: %q", got)
	}
	if !strings.Contains(got, "first paragraph line one") {
		t.Errorf("expected first paragraph body in preview, got: %q", got)
	}
	if strings.Contains(got, "second paragraph should be cut") {
		t.Errorf("expected preview to stop before the second paragraph, got: %q", got)
	}
}

func TestPreviewContent_NoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.md")
	if err := os.WriteFile(path, []byte("just body text\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := previewContent(path)
	if !strings.Contains(got, "just body text") {
		t.Errorf("expected body text in preview, got: %q", got)
	}
}

func TestResolveAncestor_NeitherSourceFallsBackToNotFound(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	_, found, err := resolveAncestor("", t.TempDir(), "journal/2026_08.md")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Error("expected no ancestor when neither git nor snapshot has one")
	}
}

func TestResolveAncestor_PrefersGitOverSnapshot(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	repo := t.TempDir()
	runGit(t, repo, "init", "-q", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(repo, "a.md"), []byte("git version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "a.md")
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "init")
	if err := snapshot.Write(filepath.Join(repo, "a.md"), []byte("snapshot version\n")); err != nil {
		t.Fatal(err)
	}

	content, found, err := resolveAncestor(repo, repo, "a.md")
	if err != nil {
		t.Fatal(err)
	}
	if !found || string(content) != "git version\n" {
		t.Errorf("expected git ancestor to win, got found=%v content=%q", found, content)
	}
}

func TestResolveAncestor_FallsBackToSnapshotWhenGitHasNone(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	repo := t.TempDir()
	runGit(t, repo, "init", "-q", "-b", "main")
	if err := snapshot.Write(filepath.Join(repo, "journal/2026_08.md"), []byte("last converged state\n")); err != nil {
		t.Fatal(err)
	}

	content, found, err := resolveAncestor(repo, repo, "journal/2026_08.md")
	if err != nil {
		t.Fatal(err)
	}
	if !found || string(content) != "last converged state\n" {
		t.Errorf("expected snapshot ancestor, got found=%v content=%q", found, content)
	}
}

// End-to-end proof that the snapshot fallback fixes the exact defect
// the bash tool had for a gitignored merge scope (where
// merge.Ancestor alone can never help it): editing the same entry
// independently on both devices, across two sync rounds, converges to one
// edited line instead of duplicating it — using only resolveAncestor +
// merge.Into, the same machinery runSync actually calls.
func TestResolveAncestorAndMerge_MergeScopeConvergesAcrossRoundsWithoutDuplication(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	dir := t.TempDir()
	local := filepath.Join(dir, "local.md")
	phone := filepath.Join(dir, "phone.md")
	rel := "local.md"

	// Round 1: both sides start identical (e.g. after an initial push) —
	// record that as the converged baseline, exactly as runSync does after
	// a successful StageOut. The two edits below are separated by an
	// unchanged line ("line three") — diff3 needs at least one unchanged
	// anchor between two independent edits to tell them apart; two edits on
	// literally adjacent lines are a known, separate limitation (see
	// TestResolveAncestorAndMerge_AdjacentEditsWithNoBufferStillConflict
	// below), not what this test is checking.
	base := "# Log\nline one\nline two\nline three\nline four\n"
	if err := os.WriteFile(local, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := snapshot.Write(filepath.Join(dir, rel), []byte(base)); err != nil {
		t.Fatal(err)
	}

	// Round 2: local edits line two; phone (independently) edits line four.
	edited := "# Log\nline one\nline two EDITED LOCALLY\nline three\nline four\n"
	if err := os.WriteFile(local, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(phone, []byte("# Log\nline one\nline two\nline three\nline four EDITED ON PHONE\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ancestor, hasAncestor, err := resolveAncestor("", dir, rel)
	if err != nil {
		t.Fatal(err)
	}
	if !hasAncestor {
		t.Fatal("expected the round-1 snapshot to serve as this round's ancestor")
	}

	res, err := merge.Into(local, ancestor, hasAncestor, phone, local)
	if err != nil {
		t.Fatal(err)
	}
	if res.Conflicts != 0 {
		t.Fatalf("expected a clean 3-way merge (independent, non-overlapping edits), got %d conflicts", res.Conflicts)
	}
	merged, _ := os.ReadFile(local)
	if strings.Count(string(merged), "line two") != 1 {
		t.Errorf("local edit duplicated instead of merged cleanly: %q", merged)
	}
	if strings.Count(string(merged), "line four") != 1 {
		t.Errorf("phone edit duplicated instead of merged cleanly: %q", merged)
	}
	if !strings.Contains(string(merged), "line two EDITED LOCALLY") || !strings.Contains(string(merged), "line four EDITED ON PHONE") {
		t.Errorf("both independent edits should survive: %q", merged)
	}
}

// Two edits on literally adjacent lines, with no unchanged line between
// them, are a known, general limitation of line-based 3-way merging (not
// specific to logSync, --union, or the snapshot ancestor): diff3 has no
// way to tell "two independent single-line edits" apart from "one edit
// spanning both lines" without at least one unchanged anchor line between
// them. This documents that boundary rather than pretending it doesn't
// exist — with --union it degrades to duplication instead of a marked
// conflict, same as the empty-base case, just for a smaller region.
func TestResolveAncestorAndMerge_AdjacentEditsWithNoBufferStillConflict(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	dir := t.TempDir()
	local := filepath.Join(dir, "local.md")
	phone := filepath.Join(dir, "phone.md")
	rel := "local.md"

	base := "# Log\nline one\nline two\nline three\n"
	if err := os.WriteFile(local, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := snapshot.Write(filepath.Join(dir, rel), []byte(base)); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(local, []byte("# Log\nline one\nline two EDITED LOCALLY\nline three\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(phone, []byte("# Log\nline one\nline two\nline three EDITED ON PHONE\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ancestor, hasAncestor, err := resolveAncestor("", dir, rel)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := merge.Into(local, ancestor, hasAncestor, phone, local); err != nil {
		t.Fatal(err)
	}
	merged, _ := os.ReadFile(local)
	if strings.Count(string(merged), "line two") == 1 && strings.Count(string(merged), "line three") == 1 {
		t.Skip("git's diff3 alignment improved enough to separate zero-buffer adjacent edits — limitation no longer applies")
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestPreviewContent_LongBodyIsCapped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.md")
	long := strings.Repeat("x", 2000)
	if err := os.WriteFile(path, []byte(long), 0o644); err != nil {
		t.Fatal(err)
	}
	got := previewContent(path)
	if len(got) >= len(long) {
		t.Errorf("expected preview to be capped well below %d bytes, got %d", len(long), len(got))
	}
	// The trailing newline matters: the caller prints a separator line
	// straight after this, and without it the "..." and the dashes ran
	// together on one line.
	if !strings.HasSuffix(got, "...\n") {
		t.Errorf("expected capped preview to end with an ellipsis marker on its own line, got: %q", got[len(got)-10:])
	}
}

func TestPreviewContent_AlwaysEndsWithANewline(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"no-trailing-newline.md": "---\ntitle: t\n---\n\nbody with no final newline",
		"short.md":               "just a line\n",
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := previewContent(path); !strings.HasSuffix(got, "\n") {
			t.Errorf("%s: preview must end with a newline so the separator starts its own line, got %q", name, got)
		}
	}
}

// gitRel is what makes the tracking repo usable at all when local_dir sits
// inside it rather than being it — the shipped layout.
func TestGitRel(t *testing.T) {
	for _, tc := range []struct {
		name           string
		repo, localDir string
		rel            string
		want           string
		wantOK         bool
	}{
		{"local_dir nested in the repo", "/vault", "/vault/journal", "entry.md", "journal/entry.md", true},
		{"nested, with a subdirectory", "/vault", "/vault/journal", "archive/2026-07.md", "journal/archive/2026-07.md", true},
		{"local_dir is the repo", "/vault", "/vault", "Notes/a.md", "Notes/a.md", true},
		{"local_dir outside the repo", "/vault", "/elsewhere", "a.md", "", false},
		{"no tracking repo configured", "", "/vault/journal", "a.md", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := gitRel(tc.repo, tc.localDir, tc.rel)
			if ok != tc.wantOK || got != tc.want {
				t.Errorf("gitRel(%q, %q, %q) = %q, %v; want %q, %v", tc.repo, tc.localDir, tc.rel, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

// With local_dir nested inside the tracking repo, the git ancestor has to
// resolve — this is the lookup that used to ask for "journal.md" when the
// repo held "journal/entry.md" and therefore always missed.
func TestResolveAncestor_FindsGitAncestorWhenLocalDirIsNestedInTheRepo(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	repo := t.TempDir()
	mergeScope := filepath.Join(repo, "journal")
	if err := os.MkdirAll(mergeScope, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "init", "-q", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(mergeScope, "entry.md"), []byte("committed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "init")

	content, found, err := resolveAncestor(repo, mergeScope, "entry.md")
	if err != nil {
		t.Fatal(err)
	}
	if !found || string(content) != "committed\n" {
		t.Errorf("expected the git ancestor at journal/entry.md, got found=%v content=%q", found, content)
	}
}

// /dev/null is a character device, so the obvious os.ModeCharDevice check
// called it a terminal and `sync --commit < /dev/null` prompted into a
// stream that could never answer.
func TestIsTerminal_DevNullIsNotATerminal(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if isTerminal(f) {
		t.Error("/dev/null must not count as a terminal")
	}
}

func TestIsTerminal_RegularFileIsNotATerminal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "answers")
	if err := os.WriteFile(path, []byte("y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if isTerminal(f) {
		t.Error("a regular file must not count as a terminal")
	}
}
