package synctree

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestClassify(t *testing.T) {
	local := t.TempDir()
	phone := t.TempDir()

	write(t, local, "local-only.md", "a")
	write(t, phone, "phone-only.md", "b")
	write(t, local, "identical.md", "same")
	write(t, phone, "identical.md", "same")
	write(t, local, "differ.md", "local version")
	write(t, phone, "differ.md", "phone version")
	// .git content must never surface in either tree.
	write(t, local, ".git/HEAD", "ref: refs/heads/main")

	c, err := Classify(local, phone, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}

	assertContains(t, "LocalOnly", c.LocalOnly, "local-only.md")
	assertContains(t, "PhoneOnly", c.PhoneOnly, "phone-only.md")
	assertContains(t, "Identical", c.Identical, "identical.md")
	assertContains(t, "Differ", c.Differ, "differ.md")
	for _, list := range [][]string{c.LocalOnly, c.PhoneOnly, c.Differ, c.Identical} {
		for _, rel := range list {
			if rel == ".git/HEAD" || filepath.Base(rel) == "HEAD" && filepath.Dir(rel) == ".git" {
				t.Errorf(".git content leaked into classification: %v", list)
			}
		}
	}
}

func TestClassify_IgnorePatternMakesDifferenceInvisible(t *testing.T) {
	local := t.TempDir()
	phone := t.TempDir()
	write(t, local, "a.md", "dateUpdated: 2026-08-01\nbody\n")
	write(t, phone, "a.md", "dateUpdated: 2026-08-09\nbody\n")

	c, err := Classify(local, phone, []string{`^[+-]dateUpdated:`}, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, "Identical", c.Identical, "a.md")
	assertNotContains(t, "Differ", c.Differ, "a.md")
}

func TestClassify_UnreadableFileReported(t *testing.T) {
	local := t.TempDir()
	phone := t.TempDir()
	write(t, local, "secret.md", "x")
	write(t, phone, "secret.md", "y")
	if err := os.Chmod(filepath.Join(local, "secret.md"), 0o000); err != nil {
		t.Skipf("cannot make file unreadable in this environment: %v", err)
	}
	defer os.Chmod(filepath.Join(local, "secret.md"), 0o644)

	c, err := Classify(local, phone, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, "Unreadable", c.Unreadable, "secret.md")
	assertNotContains(t, "Differ", c.Differ, "secret.md")
}

func TestClassify_ExcludePathHidesRepoPlumbing(t *testing.T) {
	local := t.TempDir()
	phone := t.TempDir()
	write(t, local, "note.md", "a")
	write(t, phone, "note.md", "a")
	write(t, local, ".gitconfig", "[core]\n")
	write(t, local, ".githooks/pre-commit", "#!/bin/sh\n")

	c, err := Classify(local, phone, nil, []string{".gitconfig", ".githooks"}, false)
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, "Identical", c.Identical, "note.md")
	for _, list := range [][]string{c.LocalOnly, c.PhoneOnly, c.Differ, c.Identical, c.Unreadable} {
		assertNotContains(t, "any", list, ".gitconfig")
		assertNotContains(t, "any", list, ".githooks/pre-commit")
	}
}

func TestClassify_SymlinkExcludedByDefault(t *testing.T) {
	local := t.TempDir()
	phone := t.TempDir()
	target := filepath.Join(t.TempDir(), "attachment.pdf")
	if err := os.WriteFile(target, []byte("pdf bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(local, "attachment.pdf")); err != nil {
		t.Skipf("cannot create symlink in this environment: %v", err)
	}

	c, err := Classify(local, phone, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, list := range [][]string{c.LocalOnly, c.PhoneOnly, c.Differ, c.Identical} {
		assertNotContains(t, "any", list, "attachment.pdf")
	}
	assertContains(t, "LocalSymlinks", c.LocalSymlinks, "attachment.pdf")
}

func TestClassify_SymlinkIncludedWithFlag(t *testing.T) {
	local := t.TempDir()
	phone := t.TempDir()
	target := filepath.Join(t.TempDir(), "attachment.pdf")
	if err := os.WriteFile(target, []byte("pdf bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(local, "attachment.pdf")); err != nil {
		t.Skipf("cannot create symlink in this environment: %v", err)
	}

	c, err := Classify(local, phone, nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, "LocalOnly", c.LocalOnly, "attachment.pdf")
	assertContains(t, "LocalSymlinks", c.LocalSymlinks, "attachment.pdf")
}

func TestClassify_BrokenSymlinkNeverIncluded(t *testing.T) {
	local := t.TempDir()
	phone := t.TempDir()
	if err := os.Symlink(filepath.Join(t.TempDir(), "gone.pdf"), filepath.Join(local, "attachment.pdf")); err != nil {
		t.Skipf("cannot create symlink in this environment: %v", err)
	}

	c, err := Classify(local, phone, nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, list := range [][]string{c.LocalOnly, c.PhoneOnly, c.Differ, c.Identical} {
		assertNotContains(t, "any", list, "attachment.pdf")
	}
	assertContains(t, "LocalSymlinks", c.LocalSymlinks, "attachment.pdf")
}

func TestClassify_PhoneContentAtSymlinkPathIsProtectedNotPhoneOnly(t *testing.T) {
	local := t.TempDir()
	phone := t.TempDir()
	target := filepath.Join(t.TempDir(), "attachment.pdf")
	if err := os.WriteFile(target, []byte("pdf bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(local, "attachment.pdf")); err != nil {
		t.Skipf("cannot create symlink in this environment: %v", err)
	}
	// The phone has content at this path (e.g. from a prior --with-symlinks
	// push); a plain run (includeSymlinks=false) must not offer to copy it
	// in over the local symlink.
	write(t, phone, "attachment.pdf", "stale phone copy")

	c, err := Classify(local, phone, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	assertNotContains(t, "PhoneOnly", c.PhoneOnly, "attachment.pdf")
	assertContains(t, "ProtectedSymlinkOnPhone", c.ProtectedSymlinkOnPhone, "attachment.pdf")
}

func TestLock_SecondAcquireFails(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "logSync.lock")

	release, err := Lock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	if _, err := Lock(lockPath); err == nil {
		t.Error("expected second Lock() to fail while the first is held")
	}
}

func TestLock_ReleasedThenReacquirable(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "logSync.lock")

	release, err := Lock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}

	release2, err := Lock(lockPath)
	if err != nil {
		t.Fatalf("expected to reacquire after release, got: %v", err)
	}
	release2()
}

func assertContains(t *testing.T, label string, list []string, want string) {
	t.Helper()
	for _, v := range list {
		if v == want {
			return
		}
	}
	t.Errorf("%s: expected %q in %v", label, want, list)
}

func assertNotContains(t *testing.T, label string, list []string, unwanted string) {
	t.Helper()
	for _, v := range list {
		if v == unwanted {
			t.Errorf("%s: did not expect %q in %v", label, unwanted, list)
		}
	}
}

// An unreadable file that exists on one side only has to be reported as
// unreadable during a preview, not announced as sendable and then failed
// at commit — a dry run is the one place the plan must be trustworthy.
func TestClassify_OneSidedUnreadableFileIsNotPlannedForTransfer(t *testing.T) {
	local, phone := t.TempDir(), t.TempDir()
	path := filepath.Join(local, "secret.md")
	if err := os.WriteFile(path, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	if os.Geteuid() == 0 {
		t.Skip("running as root, which bypasses the permission bits this test relies on")
	}

	c, err := Classify(local, phone, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.LocalOnly) != 0 {
		t.Errorf("unreadable file planned for transfer: %v", c.LocalOnly)
	}
	if len(c.Unreadable) != 1 || c.Unreadable[0] != "secret.md" {
		t.Errorf("expected secret.md reported unreadable, got %v", c.Unreadable)
	}
}

// exclude_path used to be an exact, root-anchored string compare, so a
// `.trash` rule covered only a `.trash` at the vault root and there was no
// way to express the `**/.trash/` the vault's own .gitignore already used.
func TestExcluder(t *testing.T) {
	for _, tc := range []struct {
		name     string
		patterns []string
		path     string
		want     bool
	}{
		{"bare name at the root", []string{".trash"}, ".trash", true},
		{"bare name nested", []string{".trash"}, "Notes/2026/.trash", true},
		{"bare name does not match a prefix", []string{".trash"}, ".trashcan", false},
		{"glob within one segment", []string{"*.tmp"}, "Notes/scratch.tmp", true},
		{"glob does not cross a slash", []string{"*.tmp"}, "Notes/a/b.tmp", true},
		{"anchored path at the root", []string{".obsidian/workspace.json"}, ".obsidian/workspace.json", true},
		{"anchored path is not matched deeper", []string{".obsidian/workspace.json"}, "sub/.obsidian/workspace.json", false},
		{"double star makes it match deeper", []string{"**/.obsidian/workspace.json"}, "sub/.obsidian/workspace.json", true},
		{"double star still matches at the root", []string{"**/.obsidian/workspace.json"}, ".obsidian/workspace.json", true},
		{"trailing slash is ignored", []string{".githooks/"}, "a/.githooks", true},
		{"unrelated path", []string{".trash", "*.tmp"}, "Notes/keep.md", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, err := newExcluder(tc.patterns)
			if err != nil {
				t.Fatal(err)
			}
			if got := e.match(tc.path); got != tc.want {
				t.Errorf("match(%q) with %v = %v, want %v", tc.path, tc.patterns, got, tc.want)
			}
		})
	}
}

func TestExcluder_RejectsAMalformedPattern(t *testing.T) {
	if _, err := newExcluder([]string{"[unclosed"}); err == nil {
		t.Error("a pattern that can never match should be refused, not silently ignored")
	}
}

// A nested .trash is exactly the case the old exact match let through.
func TestWalkFiles_ExcludesANestedDirectoryByBareName(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"Notes/keep.md", "Notes/.trash/deleted.md", ".trash/other.md"} {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files, _, _, err := WalkFiles(root, []string{".trash"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := files["Notes/keep.md"]; !ok {
		t.Error("an ordinary note was excluded")
	}
	if len(files) != 1 {
		t.Errorf("expected only the ordinary note, got %v", files)
	}
}

// Walked has to cover every role, or a prune built from it deletes the
// baseline of anything the classification did not put in a transfer bucket.
func TestClassify_WalkedCoversSymlinksAndUnreadableFiles(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root, which bypasses the permission bits this test relies on")
	}
	local, phone := t.TempDir(), t.TempDir()
	target := filepath.Join(t.TempDir(), "attachment.bin")
	if err := os.WriteFile(target, []byte("payload\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(local, "linked.bin")); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(local, "secret.md")
	if err := os.WriteFile(secret, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(phone, "secret.md"), []byte("y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(secret, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(secret, 0o644) })

	// A plain run: symlinks are not syncable content, but they still exist.
	c, err := Classify(local, phone, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	walked := map[string]bool{}
	for _, rel := range c.Walked {
		walked[rel] = true
	}
	if !walked["linked.bin"] {
		t.Error("a local symlink must count as walked, or a plain run prunes its baseline")
	}
	if !walked["secret.md"] {
		t.Error("an unreadable file must count as walked")
	}
}

func TestClassify_BrokenSymlinkIsReportedWhenSymlinksAreRequested(t *testing.T) {
	local, phone := t.TempDir(), t.TempDir()
	if err := os.Symlink(filepath.Join(local, "nowhere.bin"), filepath.Join(local, "dangling.bin")); err != nil {
		t.Fatal(err)
	}
	c, err := Classify(local, phone, nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.BrokenSymlinks) != 1 || c.BrokenSymlinks[0] != "dangling.bin" {
		t.Errorf("expected the dangling link reported, got %v", c.BrokenSymlinks)
	}
	if len(c.LocalOnly) != 0 {
		t.Errorf("a link with no target is not sendable content, got %v", c.LocalOnly)
	}
}

// A vault's images, fonts and PDFs must be identified as non-text so the
// caller can keep them away from a line merge — a union of two PNGs is not
// a PNG.
func TestClassify_BinaryContentIsFlaggedAndStillCountedAsDiffering(t *testing.T) {
	local, phone := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(local, "image.png"), []byte{0x89, 'P', 'N', 'G', 0x00, 0x01}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(phone, "image.png"), []byte{0x89, 'P', 'N', 'G', 0x00, 0x02}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(local, "note.md"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(phone, "note.md"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := Classify(local, phone, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Binary) != 1 || c.Binary[0] != "image.png" {
		t.Errorf("expected the png flagged as binary, got %v", c.Binary)
	}
	if len(c.Differ) != 2 {
		t.Errorf("both files differ and both should be reported: %v", c.Differ)
	}
}

// A `.git` that is a file, not a directory, is a gitdir pointer left by a
// submodule or a linked worktree. An Obsidian vault that versions
// .obsidian/ as a submodule has exactly one, holding a path that means
// nothing on any other machine.
func TestWalkFiles_SkipsAGitdirPointerFile(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".obsidian"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".obsidian", ".git"), []byte("gitdir: ../.git/modules/.obsidian\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".obsidian", "app.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, _, _, err := WalkFiles(root, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, leaked := files[".obsidian/.git"]; leaked {
		t.Error("a gitdir pointer must never be treated as vault content")
	}
	if _, ok := files[".obsidian/app.json"]; !ok {
		t.Error("ordinary .obsidian config should still sync")
	}
}
