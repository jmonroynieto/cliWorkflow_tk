package merge

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Ported from the bash tool's test suite: merging local against phone when
// each side has added a line the other does not have keeps both. That is
// the join-on-distinct-lines property the union operator has, and it is
// the one case where an empty base costs nothing.
func TestInto_EmptyBase_UnionKeepsBothSides(t *testing.T) {
	dir := t.TempDir()
	local := filepath.Join(dir, "local.md")
	phone := filepath.Join(dir, "phone.md")
	dest := filepath.Join(dir, "dest.md")
	writeFile(t, local, "shared\nlocal-only\n")
	writeFile(t, phone, "shared\nphone-only\n")

	res, err := Into(local, nil, false, phone, dest)
	if err != nil {
		t.Fatal(err)
	}
	if res.UsedBase {
		t.Error("expected empty-base union, got UsedBase=true")
	}
	got, _ := os.ReadFile(dest)
	for _, want := range []string{"shared", "local-only", "phone-only"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("merged output missing %q; got %q", want, got)
		}
	}
}

// Idempotence: merging a file against itself changes
// nothing (re-running a merge with no new changes must not duplicate lines).
func TestInto_EmptyBase_Idempotent(t *testing.T) {
	dir := t.TempDir()
	local := filepath.Join(dir, "local.md")
	phone := filepath.Join(dir, "phone.md")
	dest := filepath.Join(dir, "dest.md")
	writeFile(t, local, "- shared\n")
	writeFile(t, phone, "- shared\n")

	if _, err := Into(local, nil, false, phone, dest); err != nil {
		t.Fatal(err)
	}
	out1, _ := os.ReadFile(dest)

	// Re-merge dest against itself (as both "local" and "phone" would be
	// once both sides have converged) and confirm no duplication.
	if _, err := Into(dest, nil, false, dest, dest); err != nil {
		t.Fatal(err)
	}
	out2, _ := os.ReadFile(dest)
	if string(out1) != string(out2) {
		t.Errorf("merge not idempotent: %q != %q", out1, out2)
	}
	if n := strings.Count(string(out2), "- shared"); n != 1 {
		t.Errorf("expected exactly 1 occurrence of '- shared', got %d: %q", n, out2)
	}
}

// This is the case that was already known to be broken for the
// empty-base union: editing the same shared line on both sides re-emits it
// as a duplicate instead of resolving to one edited line, because an empty
// base makes every hunk a conflict. A real ancestor fixes it: git
// merge-file can then see that neither side touched the *other* hunks and
// resolve the one both sides changed as a real 3-way merge.
func TestInto_WithAncestor_FixesDuplicateReemission(t *testing.T) {
	dir := t.TempDir()
	local := filepath.Join(dir, "local.md")
	phone := filepath.Join(dir, "phone.md")
	dest := filepath.Join(dir, "dest.md")
	ancestor := "line one\nline two\nline three\n"
	// Local edits line two; phone is untouched from the ancestor.
	writeFile(t, local, "line one\nline two EDITED\nline three\n")
	writeFile(t, phone, ancestor)

	res, err := Into(local, []byte(ancestor), true, phone, dest)
	if err != nil {
		t.Fatal(err)
	}
	if res.Conflicts != 0 {
		t.Errorf("expected a clean 3-way merge, got %d conflicts", res.Conflicts)
	}
	got, _ := os.ReadFile(dest)
	if n := strings.Count(string(got), "line two"); n != 1 {
		t.Errorf("expected exactly 1 occurrence of the edited line, got %d: %q", n, got)
	}
	if !strings.Contains(string(got), "line two EDITED") {
		t.Errorf("expected the local edit to survive: %q", got)
	}
}

func TestAncestor_NotFoundReturnsFalseNotError(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "git", "init", "-q", "-b", "main")

	_, found, err := Ancestor(dir, "never-committed.md")
	if err != nil {
		t.Fatalf("expected no error for an untracked path, got: %v", err)
	}
	if found {
		t.Error("expected found=false for a path with no commit history")
	}
}

func TestAncestor_FindsCommittedContent(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "git", "init", "-q", "-b", "main")
	run(t, dir, "git", "config", "user.email", "test@example.com")
	run(t, dir, "git", "config", "user.name", "test")
	writeFile(t, filepath.Join(dir, "a.md"), "committed content\n")
	run(t, dir, "git", "add", "a.md")
	run(t, dir, "git", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "init")

	content, found, err := Ancestor(dir, "a.md")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected found=true")
	}
	if string(content) != "committed content\n" {
		t.Errorf("got %q", content)
	}
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out)
	}
}

// `git show HEAD:<path>` exits 0 and prints the whole commit object when
// <path> contains a `[`, because git reads the bracket as a wildcard
// pathspec rather than a missing file. That made an untracked note named
// e.g. "[draft] plan.md" report found=true with a commit dump standing in
// for its content — which then fed both the merge base and the
// tracked-or-not test behind the apparent-deletion prompt.
func TestAncestor_UntrackedPathWithBracketIsNotFound(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "git", "init", "-q", "-b", "main")
	run(t, dir, "git", "config", "user.email", "test@example.com")
	run(t, dir, "git", "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(dir, "tracked.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "git", "add", "-A")
	run(t, dir, "git", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "init")

	for _, name := range []string{"[draft] plan.md", "a[b.md", "note [1].md"} {
		content, found, err := Ancestor(dir, name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if found {
			t.Errorf("%s: untracked path reported as tracked, with %q as its content", name, content)
		}
	}
}

// The same names, once actually committed, still have to come back.
func TestAncestor_TrackedPathWithBracketRoundTrips(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "git", "init", "-q", "-b", "main")
	run(t, dir, "git", "config", "user.email", "test@example.com")
	run(t, dir, "git", "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(dir, "[draft] plan.md"), []byte("real content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "git", "add", "-A")
	run(t, dir, "git", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "init")

	content, found, err := Ancestor(dir, "[draft] plan.md")
	if err != nil {
		t.Fatal(err)
	}
	if !found || string(content) != "real content\n" {
		t.Errorf("got found=%v content=%q", found, content)
	}
}

// A directory name is not a blob; it must read as "no ancestor" rather
// than an error that aborts the whole round.
func TestAncestor_DirectoryIsNotFoundNotAnError(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "git", "init", "-q", "-b", "main")
	run(t, dir, "git", "config", "user.email", "test@example.com")
	run(t, dir, "git", "config", "user.name", "test")
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "a.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "git", "add", "-A")
	run(t, dir, "git", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "init")

	if _, found, err := Ancestor(dir, "sub"); err != nil || found {
		t.Errorf("expected found=false, nil error for a directory; got found=%v err=%v", found, err)
	}
}
